/*
Copyright 2021.       // original copyright for the project
Copyright (c) 2025 Huawei Technology corp.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

/*
This file is developed based on the openstack.go.
It is modified to support the VF hotplugging in QEMU-KVM.
*/

package qemu

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/jaypipes/ghw"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/log"

	dputils "github.com/k8snetworkplumbingwg/sriov-network-device-plugin/pkg/utils"

	sriovnetworkv1 "github.com/k8snetworkplumbingwg/sriov-network-operator/api/v1"
	"github.com/k8snetworkplumbingwg/sriov-network-operator/pkg/consts"
	"github.com/k8snetworkplumbingwg/sriov-network-operator/pkg/host"
	"github.com/k8snetworkplumbingwg/sriov-network-operator/pkg/vars"
)

const (
	blacklistConfigMapName      = "device-info-blacklist"
	blacklistConfigMapNamespace = "sriov-network-operator"
	defaultConfigField          = "default"
	nodeSpecificExcludeField    = "exclude-from-default"
)

// QemuKvmInterface defines the interface for managing QEMU-KVM devices

//go:generate ../../../bin/mockgen -destination mock/mock_qemu.go -source qemu.go
type QemuKvmInterface interface {
	CreateQemuKvmDevicesInfo() error
	CreateQemuKvmDevicesInfoFromNodeStatus(*sriovnetworkv1.SriovNetworkNodeState)
	DiscoverSriovDevicesVirtualInQemuKvm() ([]sriovnetworkv1.InterfaceExt, error)
}

type qemuKvmContext struct {
	hostManager                 host.HostManagerInterface
	k8sClientset                kubernetes.Interface // k8sClientset for getting the Kubernetes client
	qemuKvmDevicesInfo          qemuDevicesInfo
	qemuKvmDevicesInfoBlacklist qemuDevicesInfo // blacklist of devices that should not be discovered
}

// qemuBlackListMetaDataDevice -- Black list Device structure within meta_data.json
type qemuBlackListMetaDataDevice struct {
	Type    string   `json:"type,omitempty"`
	Mac     string   `json:"mac,omitempty"`
	Bus     string   `json:"bus,omitempty"`
	Address string   `json:"address,omitempty"` // pci address
	Tags    []string `json:"tags,omitempty"`
}

// qemuBlackListMetaData -- QemuKvm black list meta_data.json format
type qemuBlackListMetaData struct {
	Devices            []qemuBlackListMetaDataDevice `json:"devices,omitempty"`
	ExcludeFromDefault []qemuBlackListMetaDataDevice `json:"exclude-from-default,omitempty"`
}

type qemuDevicesInfo map[string]*qemuDeviceInfo

type qemuDeviceInfo struct {
	MacAddress string
	NetworkID  string
}

// New() creates a new QemuKvmInterface instance with the provided host manager
func New(hostManager host.HostManagerInterface, k8sClient kubernetes.Interface) QemuKvmInterface {
	return &qemuKvmContext{
		hostManager:  hostManager,
		k8sClientset: k8sClient, // Use the default kube client getter
	}
}

// readBlacklistConfigMap() reads the blacklist ConfigMap
func readBlacklistConfigMap(k8sClient kubernetes.Interface) (metaData *qemuBlackListMetaData, err error) {
	log.Log.Info("readBlacklistConfigMap()")

	metaData = &qemuBlackListMetaData{}

	// get the ConfigMap from the specified namespace
	cm, err := k8sClient.CoreV1().ConfigMaps(blacklistConfigMapNamespace).Get(
		context.Background(),
		blacklistConfigMapName,
		metav1.GetOptions{},
	)
	if err != nil {
		return metaData, fmt.Errorf("failed to get ConfigMap: %v", err)
	}

	nodeName := vars.NodeName

	for key, data := range cm.Data {
		// only process fields that are relevant to the current node or the default config
		if key != nodeName && key != defaultConfigField {
			log.Log.Info("readConfigMap(): field not for the current node, skipping", "field", key)
			continue
		}
		var deviceList qemuBlackListMetaData
		if err := json.Unmarshal([]byte(data), &deviceList); err != nil {
			return nil, fmt.Errorf("failed to parse JSON for key %s: %v", key, err)
		}
		metaData.Devices = append(metaData.Devices, deviceList.Devices...)
		metaData.ExcludeFromDefault = append(metaData.ExcludeFromDefault, deviceList.ExcludeFromDefault...)
	}
	return metaData, nil
}

// Read the QemuKvm blackList meta_data from the configMap
func getQemuKvmBlackListDataFromConfigMap(k8sClient kubernetes.Interface) (metaData *qemuBlackListMetaData, err error) {
	metaData = &qemuBlackListMetaData{}
	log.Log.Info("getQemuKvmBlackListDataFromConfigMap()")

	metaData, err = readBlacklistConfigMap(k8sClient)
	if err != nil {
		log.Log.Error(err, "getQemuKvmBlackListDataFromConfigMap(): failed to read blackList meta_data from configMap")
	}
	return metaData, err
}

// getQemuKvmBlackListData() gets the QemuKvm BlackList Data, which contains the device info that will be exclude in DeviceInfo
func getQemuKvmBlackListData(k8sClient kubernetes.Interface) (*qemuBlackListMetaData, error) {
	log.Log.Info("getQemuKvmBlackListData()")
	blackListData, err := getQemuKvmBlackListDataFromConfigMap(k8sClient)

	if err != nil {
		log.Log.Error(err, "failed to read QemuKvm blackList meta_data")
		return blackListData, err
	}

	if blackListData == nil || len(blackListData.Devices) == 0 {
		return blackListData, fmt.Errorf("got QemuKvm blackList meta_data but it is empty")
	}
	return blackListData, nil
}

// createQemuKvmDevicesInfoBlackList creates the QemuKvm devices info blacklist
func (o *qemuKvmContext) createQemuKvmDevicesInfoBlackList() (deviceInfoBlackList qemuDevicesInfo, err error) {
	log.Log.Info("CreateQemuKvmDevicesInfoBlackList()")

	metaDataBlackList, err := getQemuKvmBlackListData(o.k8sClientset)
	deviceInfoBlackList = make(qemuDevicesInfo)

	// when unable to get the blacklist data, or the data is empty, we return an empty blacklist
	if err != nil {
		return deviceInfoBlackList, err
	}

	networkID := sriovnetworkv1.QemuKvmNetworkID.String() + ":" + "blacklist-network-id"

	// Create the blacklist device info map
	deviceInfoExcludeFromBlackList := make(qemuDevicesInfo)

	for _, device := range metaDataBlackList.ExcludeFromDefault {
		deviceInfoExcludeFromBlackList[device.Address] = &qemuDeviceInfo{MacAddress: device.Mac, NetworkID: networkID}
	}
	for _, device := range metaDataBlackList.Devices {
		// skip devices in deviceInfoExcludeFromBlackList
		if deviceInList(deviceInfoExcludeFromBlackList, device.Address) {
			log.Log.Info("createQemuKvmDevicesInfoBlackList(): device in 'exclude-from-default' for the node, skipping",
				"device", device.Address)
			continue
		}
		deviceInfoBlackList[device.Address] = &qemuDeviceInfo{MacAddress: device.Mac, NetworkID: networkID}
		log.Log.Info("createQemuKvmDevicesInfoBlackList(): add device to blacklist",
			"device", device.Address)
	}
	return deviceInfoBlackList, nil
}

// updateDeviceBlacklist() updates the device blacklist
func (o *qemuKvmContext) updateDeviceBlacklist() {
	deviceInfoBlackList, err := o.createQemuKvmDevicesInfoBlackList()
	if err != nil {
		infoString := "getQemuKvmBlackListData(): "
		// If the error is not related to parsing JSON, we consider it a non-fatal error.
		// But when the error is releated to parsing JSON, we consider it a fatal error.
		// Because it means the configMap is not in the expected format, and it will not work as expected.
		if !strings.Contains(err.Error(), "parse JSON") {
			infoString += "non-"
		}
		infoString += "fatal error getting QEMU-KVM device info blacklist. Using empty blacklist instead."
		log.Log.Error(err, infoString)
	}

	o.qemuKvmDevicesInfoBlacklist = deviceInfoBlackList
}

func getPCIDevices() ([]*ghw.PCIDevice, error) {

	pci, err := ghw.PCI()
	if err != nil {
		return nil, fmt.Errorf("error getting PCI info: %w", err)
	}

	if len(pci.Devices) == 0 {
		return nil, fmt.Errorf("could not retrieve PCI devices")
	}
	return pci.Devices, nil
}

// deviceInList checks if the device (pci) address exists in the deviceInfo list/blacklist
func deviceInList(deviceInfo qemuDevicesInfo, address string) bool {
	if len(deviceInfo) > 0 {
		_, exist := deviceInfo[address]
		return exist
	}
	return false
}

// check if the device is a network device
func notNetDevice(device *ghw.PCIDevice) bool {
	devClass, err := strconv.ParseInt(device.Class.ID, 16, 64)
	shouldSkip := false
	if err != nil {
		log.Log.Error(err, "notNetDevice(): unable to parse device class for device, skipping",
			"device", device)
		shouldSkip = true
	}
	if devClass != consts.NetClass {
		// Not network device
		shouldSkip = true
	}
	return shouldSkip
}

// CreateQemuKvmDevicesInfo create the qemuKvm device info map
func (o *qemuKvmContext) CreateQemuKvmDevicesInfo() error {
	// getqemuKvmDevicesInfo
	log.Log.Info("CreateQemuKvmDevicesInfo()")

	// always get qemuKvmDevicesInfoBlacklist first
	o.updateDeviceBlacklist()

	devicesInfo := make(qemuDevicesInfo)

	devices, err := getPCIDevices()
	if err != nil {
		return fmt.Errorf("CreateQemuKvmDevicesInfo(): %v", err)
	}

	for _, device := range devices {
		// Skip the non-network device
		if notNetDevice(device) {
			continue
		}

		// Skip the device in blacklist
		if deviceInList(o.qemuKvmDevicesInfoBlacklist, device.Address) {
			log.Log.Info("CreateQemuKvmDevicesInfo(): device in devicesInfo blacklist, skipping",
				"device", device.Address)
			continue
		}

		macAddress, name := getDeviceMacAndName(o.hostManager, device.Address)
		if macAddress == "" || name == "" {
			// we didn't manage to find a mac-address/name for the nic, skipping
			log.Log.Info("CreateQemuKvmDevicesInfo(): device mac-address/name not found, skipping",
				"device", device.Address)
			continue
		}

		// Construct the networkID from the VF.name of metadata:
		networkID := constructNetworkID(name)
		devicesInfo[device.Address] = &qemuDeviceInfo{MacAddress: macAddress, NetworkID: networkID}
	}

	o.qemuKvmDevicesInfo = devicesInfo
	return nil
}

func getDeviceMacAndName(hm host.HostManagerInterface, address string) (name, mac string) {
	macAddress := ""
	name = hm.TryToGetVirtualInterfaceName(address)
	if name != "" {
		if mac := hm.GetNetDevMac(name); mac != "" {
			macAddress = mac
		}
	}
	return macAddress, name
}

// Construct the networkID from the VF.name of metadata:
func constructNetworkID(name string) string {
	return string(sriovnetworkv1.QemuKvmNetworkID.String() + ":" + name)
}

// DiscoverSriovDevicesVirtualInQemuKvm discovers VFs on a virtual platform
func (o *qemuKvmContext) DiscoverSriovDevicesVirtualInQemuKvm() ([]sriovnetworkv1.InterfaceExt, error) {

	log.Log.V(2).Info("DiscoverSriovDevicesVirtualInQemuKvm()")

	// always get qemuKvmDevicesInfoBlacklist first
	o.updateDeviceBlacklist()

	pfList := []sriovnetworkv1.InterfaceExt{}

	devices, err := getPCIDevices()
	if err != nil {
		return nil, fmt.Errorf("DiscoverSriovDevicesVirtualInQemuKvm(): %v", err)
	}

	for _, device := range devices {
		// Skip the non-network device
		if notNetDevice(device) {
			continue
		}

		// Skip the device in blacklist
		if deviceInList(o.qemuKvmDevicesInfoBlacklist, device.Address) {
			log.Log.Info("DiscoverSriovDevicesVirtualInQemuKvm(): device in devicesInfo blacklist, skipping",
				"device", device.Address)
			continue
		}

		macAddress, name := getDeviceMacAndName(o.hostManager, device.Address)
		if macAddress == "" || name == "" {
			// we didn't manage to find a mac-address/name for the nic, skipping
			log.Log.Info("DiscoverSriovDevicesVirtualInQemuKvm(): device mac-address/name not found, skipping",
				"device", device.Address)
			continue
		}

		driver, err := dputils.GetDriverName(device.Address)
		if err != nil {
			log.Log.Error(err, "DiscoverSriovDevicesVirtualInQemuKvm(): unable to parse device driver for device, skipping",
				"device", device)
			continue
		}

		iface := o.buildInterface(device, driver, macAddress, name)

		pfList = append(pfList, iface)
	}
	return pfList, nil
}

func (o *qemuKvmContext) buildInterface(device *ghw.PCIDevice, driver, macAddress, name string) sriovnetworkv1.InterfaceExt {
	networkID := constructNetworkID(name)
	iface := sriovnetworkv1.InterfaceExt{
		PciAddress: device.Address,
		Driver:     driver,
		Vendor:     device.Vendor.ID,
		DeviceID:   device.Product.ID,
		NetFilter:  networkID,
		Mtu:        o.hostManager.GetNetdevMTU(device.Address),
		TotalVfs:   1,
		NumVfs:     1,
	}

	if name := o.hostManager.TryToGetVirtualInterfaceName(device.Address); name != "" {
		o.fillInterfaceAttributes(&iface, name, macAddress)
	}

	iface.VFs = []sriovnetworkv1.VirtualFunction{
		{
			PciAddress: device.Address,
			Driver:     driver,
			VfID:       0,
			Vendor:     iface.Vendor,
			DeviceID:   iface.DeviceID,
			Mtu:        iface.Mtu,
			Mac:        iface.Mac,
		},
	}
	return iface
}

// fill the attributes
func (o *qemuKvmContext) fillInterfaceAttributes(iface *sriovnetworkv1.InterfaceExt, name, metaMac string) {
	iface.Name = name
	iface.Mac = o.hostManager.GetNetDevMac(name)
	if iface.Mac == "" {
		iface.Mac = metaMac
	}
	iface.LinkSpeed = o.hostManager.GetNetDevLinkSpeed(name)
	iface.LinkType = o.hostManager.GetLinkType(name)
}

func (o *qemuKvmContext) CreateQemuKvmDevicesInfoFromNodeStatus(networkState *sriovnetworkv1.SriovNetworkNodeState) {
	// always get qemuKvmDevicesInfoBlacklist first
	o.updateDeviceBlacklist()

	// update devicesInfo with networkState.Status.Interfaces
	devicesInfo := make(qemuDevicesInfo)
	for _, iface := range networkState.Status.Interfaces {
		devicesInfo[iface.PciAddress] = &qemuDeviceInfo{MacAddress: iface.Mac, NetworkID: iface.NetFilter}
	}
	o.qemuKvmDevicesInfo = devicesInfo
}
