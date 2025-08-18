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

package main

import (
	"flag"
	"fmt"
	"os"

	deviceplugin "kunpeng.huawei.com/kunpeng-cloud-computing/pkg/kae-device-plugin/device-plugin"
	kaeplugin "kunpeng.huawei.com/kunpeng-cloud-computing/pkg/kae-device-plugin/kae-plugin"

	"k8s.io/klog/v2"
)

const (
	namespace = "kae.kunpeng.com"
)

func main() {
	kernelVfDrivers := flag.String("kernel-vf-drivers", "hisi_hpre", "Comma separated VF Device Driver of the KAE Devices in the system. Devices supported: hisi_hpre,hisi_zip,hisi_sec2")
	flag.Parse()

	plugin, err := kaeplugin.NewDevicePlugin(*kernelVfDrivers)
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}

	manager := deviceplugin.NewManager(namespace, plugin)
	klog.V(1).Infof("KAE device plugin started")

	manager.Run()
}
