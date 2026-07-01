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
	"sigs.k8s.io/controller-runtime/pkg/webhook"

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
	var webhookEnable bool
	var webhookPort int
	var webhookCertPath, webhookCertName, webhookCertKey string

	kernelVfDrivers := flag.String("kernel-vf-drivers", "hisi_hpre", "Comma separated VF Device Driver of the KAE Devices in the system. Devices supported: hisi_hpre,hisi_zip,hisi_sec2")

	flag.BoolVar(&enableQos, "enable-qos", false, "Enable KAE QoS")
	flag.BoolVar(&webhookEnable, "webhook-enable", false, "Enable webhook server. If enabled, the KAE Device Plugin will start a webhook server to handle admission requests for pods that request KAE devices. If disabled, users will need to manually specify the resources and environment variables in their pod specs.")
	flag.IntVar(&webhookPort, "webhook-port", 9443, "The port that the webhook server will listen on.")
	flag.StringVar(&webhookCertPath, "webhook-cert-path", "", "The directory that contains the webhook certificate.")
	flag.StringVar(&webhookCertName, "webhook-cert-name", "tls.crt", "The name of the webhook certificate file.")
	flag.StringVar(&webhookCertKey, "webhook-cert-key", "tls.key", "The name of the webhook key file.")

	// TODO(cuiyanxiang): add flag to config how webhook injects env and resources.

	flag.Parse()

	plugin, err := kaeplugin.NewDevicePlugin(*kernelVfDrivers)
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}

	if enableQos || webhookEnable {
		startControllers(enableQos, webhookEnable, webhook.Options{
			Port:     webhookPort,
			CertDir:  webhookCertPath,
			CertName: webhookCertName,
			KeyName:  webhookCertKey,
		})
	}

	kaeDevicePluginManager := deviceplugin.NewManager(namespace, plugin)
	klog.V(1).Infof("KAE device plugin started")

	kaeDevicePluginManager.Run()
}

func startControllers(enableQos bool, webhookEnable bool, webhookOption webhook.Options) {
	qosManager, err := kaeqos.NewQosManager(timeout)
	if err != nil {
		klog.Errorf("Failed to create qos manager: %v", err)
		os.Exit(1)
	}

	options := ctrl.Options{
		LeaderElection:   true,
		LeaderElectionID: "ecaf1259.kae.huawei.com",
	}

	if webhookEnable {
		options.WebhookServer = webhook.NewServer(webhookOption)
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), options)

	if err != nil {
		klog.Errorf("Failed to create manager: %v", err)
		os.Exit(1)
	}

	nodeName := os.Getenv("NODE_NAME")
	if nodeName == "" {
		klog.Errorf("NODE_NAME is empty")
		os.Exit(1)
	}

	if enableQos {
		if err := (&kaeqos.KaeQosReconciler{
			QosManager: qosManager,
			Client:     mgr.GetClient(),
		}).SetupWithManager(mgr, nodeName); err != nil {
			klog.Errorf("Failed to setup reconciler: %v", err)
			os.Exit(1)
		}
	}

	if webhookEnable {
		if err := kaePodWebhook.SetupKaePodWithManager(mgr); err != nil {
			klog.Errorf("Unable to create kae pod webhook: %v", err)
			os.Exit(1)
		}
	}

	klog.Infof("KAE QoS manager started")
	go func() {
		if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
			klog.Fatalf("Failed to start manager: %v", err)
		}
	}()
}
