/*
 * Copyright (c) 2025 Huawei Technology corp.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package kaeplugin

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pkg/errors"
	"k8s.io/klog"
	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"

	dpapi "kunpeng.huawei.com/kunpeng-cloud-computing/pkg/kae-device-plugin/device-plugin"
)

const (
	// Period of device scans.
	scanPeriod         = 5 * time.Second
	pciDeviceDirectory = "/sys/bus/pci/devices"
	pciDriverDirectory = "/sys/bus/pci/drivers"
	uacceDirectory     = "/sys/class/uacce"
)

// KAE PCI VF Device ID -> kernel KAE VF device driver mappings.
var kaeDeviceDriver = map[string]string{
	"a259": "hisi_hpre",
	"a251": "hisi_zip",
	"a256": "hisi_sec2",
}

// DevicePlugin represents KAE plugin.
type DevicePlugin struct {
	scanTicker   *time.Ticker
	scanDone     chan bool
	pciDriverDir string
	pciDeviceDir string
	uacceDir     string
	drivers      []string
}

// NewDevicePlugin returns new instance of vfio based QAT plugin.
func NewDevicePlugin(drivers string) (*DevicePlugin, error) {

	kernelDrivers := strings.Split(drivers, ",")
	for _, driver := range kernelDrivers {
		if !isValidKernelDriver(driver) {
			return nil, errors.Errorf("wrong kernel VF driver: %s is not support", driver)
		}
	}

	return newDevicePlugin(pciDriverDirectory, pciDeviceDirectory, uacceDirectory, kernelDrivers), nil
}

func newDevicePlugin(pciDriverDir, pciDeviceDir, uacceDir string, drivers []string) *DevicePlugin {
	return &DevicePlugin{
		drivers:      drivers,
		scanTicker:   time.NewTicker(scanPeriod),
		scanDone:     make(chan bool, 1),
		pciDriverDir: pciDriverDir,
		pciDeviceDir: pciDeviceDir,
		uacceDir:     uacceDir,
	}
}

func isValidKernelDriver(driver string) bool {
	for _, kaeDriver := range kaeDeviceDriver {
		if driver == kaeDriver {
			return true
		}
	}

	return false
}

func (dp *DevicePlugin) Scan(notifier dpapi.Notifier) error {
	defer dp.scanTicker.Stop()

	for {
		devTree, err := dp.scan()
		if err != nil {
			return err
		}

		notifier.Notify(devTree)

		select {
		case <-dp.scanDone:
			return nil
		case <-dp.scanTicker.C:
		}
	}
}

func (dp *DevicePlugin) scan() (dpapi.DeviceTree, error) {
	devTree := dpapi.NewDeviceTree()
	n := 0

	for _, driver := range dp.drivers {
		vfDevices, err := dp.getVfDevices(driver)
		if err != nil {
			return nil, err
		}
		for _, vfDevice := range vfDevices {
			vfBdf := filepath.Base(vfDevice)
			health, err := dp.getDeivceHealth(vfBdf)
			if err != nil {
				return nil, err
			}

			// get the name of this device in /dev, such as hisi_hpre-10
			deviceName, err := dp.getDeviceName(vfBdf)
			if err != nil {
				return nil, err
			}

			// envs creates an environment variable for each passthrough device,
			// with a format similar to HISI_HPRE-1:0000:31:00.1.
			n = n + 1
			envs := map[string]string{
				fmt.Sprintf("%s-%d", strings.ToUpper(driver), n): vfBdf,
			}

			devinfo := dpapi.NewDeviceInfo(health, dp.getDeviceSpecs(deviceName), dp.getMounts(deviceName), envs, nil)

			devTree.AddDevice(driver, vfBdf, devinfo)
		}
	}

	return devTree, nil
}

func (dp *DevicePlugin) getVfDevices(driver string) ([]string, error) {
	driverDir := filepath.Join(dp.pciDriverDir, driver)
	pattern := filepath.Join(driverDir, "????:??:??.?")
	pciDirs := getPciDevicesWithPattern(pattern)
	vfDevices := make([]string, 0)

	for _, pciDir := range pciDirs {
		ok, err := dp.isVfPci(pciDir, driver)
		if err != nil {
			return nil, err
		}
		if ok {
			vfDevices = append(vfDevices, pciDir)
		}
	}

	return vfDevices, nil
}

func (dp *DevicePlugin) isVfPci(pciDir, driver string) (bool, error) {
	fileData, err := os.ReadFile(filepath.Join(pciDir, "device"))
	if err != nil {
		return false, err
	}

	deviceId := strings.TrimSpace(strings.TrimPrefix(string(fileData), "0x"))
	if kaeDeviceDriver[deviceId] == driver {
		return true, nil
	}
	return false, nil
}

func getPciDevicesWithPattern(pattern string) (pciDevices []string) {
	pciDevices = make([]string, 0)

	devs, err := filepath.Glob(pattern)
	if err != nil {
		klog.Warningf("bad pattern: %s", pattern)
		return
	}

	for _, devBdf := range devs {
		targetDev, err := filepath.EvalSymlinks(devBdf)
		if err != nil {
			klog.Warningf("unable to evaluate symlink: %s", devBdf)
			continue
		}

		pciDevices = append(pciDevices, targetDev)
	}

	return
}

func (dp *DevicePlugin) getDeivceHealth(bdf string) (string, error) {
	return pluginapi.Healthy, nil
}

func (dp *DevicePlugin) getDeviceName(bdf string) (string, error) {
	uacceDir := filepath.Join(dp.pciDeviceDir, bdf, "uacce")
	entries, err := os.ReadDir(uacceDir)
	if err != nil {
		return "", fmt.Errorf("failed to read uacce dir %s: %w", uacceDir, err)
	}

	if len(entries) == 0 {
		return "", fmt.Errorf("no uacce device found in %s", uacceDir)
	}

	// Only one file in /sys/bus/pci/devices/<pciAddress>/uacce/, such as hisi_hpre-10
	devName := entries[0].Name()

	return devName, nil
}

func (dp *DevicePlugin) getDeviceSpecs(deviceName string) []pluginapi.DeviceSpec {
	devSpec := &pluginapi.DeviceSpec{
		ContainerPath: filepath.Join("/dev", deviceName),
		HostPath:      filepath.Join("/dev", deviceName),
		Permissions:   "rw",
	}
	return []pluginapi.DeviceSpec{*devSpec}
}

func (dp *DevicePlugin) getMounts(deviceName string) []pluginapi.Mount {
	mounts := make([]pluginapi.Mount, 0)
	mounts = append(mounts, pluginapi.Mount{
		ContainerPath: filepath.Join(dp.uacceDir, deviceName),
		HostPath:      filepath.Join(dp.uacceDir, deviceName),
		ReadOnly:      false,
	})

	// KAE checks consistency between /sys/class/uacce and /dev at runtime.
	// If they are inconsistent, it may report "failed to check file /dev/hisi_xxx-n",
	// which is harmless; this mount is only to suppress the warning.
	mounts = append(mounts, pluginapi.Mount{
		ContainerPath: filepath.Join(dp.uacceDir),
		HostPath:      filepath.Join("/tmp", "kae-uacce", deviceName),
		ReadOnly:      false,
	})

	return mounts
}
