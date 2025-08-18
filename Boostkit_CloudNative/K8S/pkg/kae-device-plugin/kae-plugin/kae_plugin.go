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
	"strings"
	"time"

	"github.com/pkg/errors"
	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"

	dpapi "kunpeng.huawei.com/kunpeng-cloud-computing/pkg/kae-device-plugin/device-plugin"
)

const (
	// Period of device scans.
	scanPeriod = 5 * time.Second
)

// KAE PCI VF Device ID -> kernel KAE VF device driver mappings.
var kaeDeviceDriver = map[string]string{
	"a259": "hisi_hpre",
	"a251": "hisi_zip",
	"a256": "hisi_sec2",
}

// DevicePlugin represents KAE plugin.
type DevicePlugin struct {
	scanTicker *time.Ticker
	scanDone   chan bool
	drivers    []string
}

// NewDevicePlugin returns new instance of vfio based QAT plugin.
func NewDevicePlugin(drivers string) (*DevicePlugin, error) {

	kernelDrivers := strings.Split(drivers, ",")
	for _, driver := range kernelDrivers {
		if !isValidKernelDriver(driver) {
			return nil, errors.Errorf("wrong kernel VF driver: %s is not support", driver)
		}
	}

	return &DevicePlugin{
		drivers:    kernelDrivers,
		scanTicker: time.NewTicker(scanPeriod),
		scanDone:   make(chan bool, 1),
	}, nil
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
	for _, driver := range dp.drivers {
		vfDevices, err := dp.getVfDevices(driver)
		if err != nil {
			return nil, err
		}
		for _, vfBdf := range vfDevices {
			health, err := dp.getDeivceHealth(vfBdf)
			if err != nil {
				return nil, err
			}

			// get the name of this device in /dev, such as hisi_hpre-10
			deviceName, err := dp.getDeviceName(vfBdf)
			if err != nil {
				return nil, err
			}

			envs := dp.getEnvs(vfBdf)

			devinfo := dpapi.NewDeviceInfo(health, dp.getDeviceSpecs(deviceName), dp.getMounts(deviceName), envs, nil)

			devTree.AddDevice(driver, vfBdf, devinfo)
		}

	}

	return devTree, nil
}

func (dp *DevicePlugin) getVfDevices(driver string) ([]string, error) {
	return nil, nil
}

func (dp *DevicePlugin) getDeivceHealth(bdf string) (string, error) {
	return "", nil
}

func (dp *DevicePlugin) getDeviceName(bdf string) (string, error) {
	return "", nil
}

func (dp *DevicePlugin) getDeviceSpecs(deviceName string) []pluginapi.DeviceSpec {
	return nil
}

func (dp *DevicePlugin) getMounts(deviceName string) []pluginapi.Mount {
	return nil
}

func (dp *DevicePlugin) getEnvs(bdf string) map[string]string {
	return nil
}
