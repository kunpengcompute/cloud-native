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
	"time"

	deviceplugin "kunpeng.huawei.com/kunpeng-cloud-computing/pkg/kae-device-plugin/device-plugin"
	kaeplugin "kunpeng.huawei.com/kunpeng-cloud-computing/pkg/kae-device-plugin/kae-plugin"
	kaeqos "kunpeng.huawei.com/kunpeng-cloud-computing/pkg/kae-device-plugin/kae-qos"
	kaePodWebhook "kunpeng.huawei.com/kunpeng-cloud-computing/pkg/kae-device-plugin/webhook"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	controllerwebhook "sigs.k8s.io/controller-runtime/pkg/webhook"

	"k8s.io/klog/v2"
)

const (
	namespace = "kae.kunpeng.com"
	timeout   = 10 * time.Second
)

func main() {
	klog.InitFlags(nil)
	defer klog.Flush()
	ctrl.SetLogger(klog.NewKlogr())

	var enableQos bool
	kernelVfDrivers := flag.String("kernel-vf-drivers", "hisi_hpre", "Comma separated VF Device Driver of the KAE Devices in the system. Devices supported: hisi_hpre,hisi_zip,hisi_sec2")
	flag.BoolVar(&enableQos, "enable-qos", false, "Enable KAE QoS")
	webhookOptions := kaePodWebhook.NewOptions()
	webhookOptions.AddFlags(flag.CommandLine)

	flag.Parse()

	plugin, err := kaeplugin.NewDevicePlugin(*kernelVfDrivers)
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}

	if enableQos || webhookOptions.Enabled {
		controllerManager, err := newControllerManager(enableQos, webhookOptions)
		if err != nil {
			klog.Errorf("Failed to create KAE controller manager: %v", err)
			klog.Flush()
			os.Exit(1)
		}
		startControllerManager(controllerManager)
	}

	kaeDevicePluginManager := deviceplugin.NewManager(namespace, plugin)
	klog.V(1).Infof("KAE device plugin started")
	kaeDevicePluginManager.Run()
}

func newControllerManager(enableQos bool, webhookOptions kaePodWebhook.Options) (manager.Manager, error) {
	managerOptions := ctrl.Options{}
	var injectionConfig kaePodWebhook.InjectionConfig
	if webhookOptions.Enabled {
		serverOptions, config, err := webhookOptions.Build(os.Getenv("POD_NAMESPACE"))
		if err != nil {
			return nil, err
		}
		managerOptions.WebhookServer = controllerwebhook.NewServer(serverOptions)
		injectionConfig = config
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), managerOptions)
	if err != nil {
		return nil, fmt.Errorf("create controller-runtime manager: %w", err)
	}

	if enableQos {
		qosManager, err := kaeqos.NewQosManager(timeout)
		if err != nil {
			return nil, fmt.Errorf("create QoS manager: %w", err)
		}
		nodeName := os.Getenv("NODE_NAME")
		if nodeName == "" {
			return nil, fmt.Errorf("NODE_NAME must not be empty when KAE QoS is enabled")
		}
		if err := (&kaeqos.KaeQosReconciler{
			QosManager: qosManager,
			Client:     mgr.GetClient(),
		}).SetupWithManager(mgr, nodeName); err != nil {
			return nil, fmt.Errorf("setup KAE QoS reconciler: %w", err)
		}
		klog.Infof("KAE QoS manager enabled")
	}

	if webhookOptions.Enabled {
		if err := kaePodWebhook.SetupKaePodWithManager(mgr, injectionConfig); err != nil {
			return nil, fmt.Errorf("setup KAE pod webhook: %w", err)
		}
		klog.Infof("KAE admission webhook enabled on %s", webhookOptions.ListenAddr)
	}
	return mgr, nil
}

func startControllerManager(mgr manager.Manager) {
	go func() {
		if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
			klog.Errorf("KAE controller manager failed: %v", err)
			klog.Flush()
			os.Exit(1)
		}
		klog.Flush()
		os.Exit(0)
	}()
}
