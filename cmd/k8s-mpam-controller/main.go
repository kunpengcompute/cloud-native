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

// Package main provide main function for k8s-mpam-controller.
package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/klog/v2"
	"k8s.io/klog/v2/klogr"
	ctrl "sigs.k8s.io/controller-runtime"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	qosv1alpha1 "kunpeng.huawei.com/kunpeng-cloud-computing/api/k8s-mpam-controller/v1alpha1"
	"kunpeng.huawei.com/kunpeng-cloud-computing/pkg/k8s-mpam-controller/controller"
	"kunpeng.huawei.com/kunpeng-cloud-computing/pkg/k8s-mpam-controller/dynamiccontrol"
)

func main() {
	klog.InitFlags(nil)
	defer klog.Flush()

	if err := run(); err != nil {
		klog.Errorf("k8s-mpam-controller failed: %v", err)
		// Explicit flush before os.Exit because deferred calls are skipped on os.Exit.
		klog.Flush()
		os.Exit(1)
	}
}

func run() error {
	enableDynamicControl := flag.Bool("enable-dynamic-control", false, "enable dynamic interference control loops")
	enableMetrics := flag.Bool("enable-metrics", false, "enable metrics endpoint")
	metricsBindAddress := flag.String("metrics-bind-address", ":8080", "metrics endpoint bind address when --enable-metrics=true")
	agentAddr := flag.String("dynamic-agent-addr", "http://127.0.0.1:18080", "dynamic-control agent address, e.g. http://127.0.0.1:18080")
	publishInterval := flag.Duration("dynamic-publish-interval", 30*time.Second, "interval for publishing online pod cgroups")
	applyInterval := flag.Duration("dynamic-apply-interval", 30*time.Second, "interval for pulling interference and applying tuning decisions")
	taskTimeout := flag.Duration("dynamic-task-timeout", 10*time.Second, "timeout for one dynamic-control task execution")
	flag.Parse()
	ctrl.SetLogger(klogr.New())

	if *publishInterval <= 0 {
		return fmt.Errorf("invalid --dynamic-publish-interval: must be > 0")
	}
	if *applyInterval <= 0 {
		return fmt.Errorf("invalid --dynamic-apply-interval: must be > 0")
	}
	if *taskTimeout <= 0 {
		return fmt.Errorf("invalid --dynamic-task-timeout: must be > 0")
	}
	if *enableDynamicControl {
		if err := dynamiccontrol.ValidateAgentBaseURL(*agentAddr); err != nil {
			return fmt.Errorf("invalid --dynamic-agent-addr: %w", err)
		}
	}

	cfg := ctrl.GetConfigOrDie()
	metricsAddr := "0"
	if *enableMetrics {
		metricsAddr = *metricsBindAddress
	}

	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(qosv1alpha1.AddToScheme(scheme))

	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme:                 scheme,
		Metrics:                metricsserver.Options{BindAddress: metricsAddr},
		HealthProbeBindAddress: ":8081",
		LeaderElection:         false,
		LeaderElectionID:       "k8s-mpam-controller.kunpeng.huawei.com",
	})
	if err != nil {
		return err
	}

	nodeIdentity := controller.NewDefaultNodeIdentity()
	if nodeIdentity.NodeName() == "" {
		return fmt.Errorf("NODE_NAME is empty, please set NODE_NAME env")
	}

	resctrlMgr, err := controller.NewLocalResctrlGroupManager()
	if err != nil {
		return err
	}

	if err := (&controller.QoSPolicyReconciler{
		Client:       mgr.GetClient(),
		Scheme:       mgr.GetScheme(),
		NodeIdentity: nodeIdentity,
		Resctrl:      resctrlMgr,
	}).SetupWithManager(mgr); err != nil {
		return err
	}

	if err := controller.NewPodBindingReconciler(
		mgr.GetClient(),
		mgr.GetScheme(),
		*enableDynamicControl,
	).SetupWithManager(mgr); err != nil {
		return err
	}

	if *enableDynamicControl {
		agentClient := dynamiccontrol.NewTCPHTTPAgentClient(*agentAddr)
		agentClient.HTTPClient.Timeout = *taskTimeout

		policyUpdater := dynamiccontrol.NewQoSPolicyDynamicUpdater(mgr.GetClient())
		tuningEngine := dynamiccontrol.NewReasonDispatchTuningEngine(policyUpdater)
		dynamicCoordinator := dynamiccontrol.NewCoordinator(
			nodeIdentity,
			dynamiccontrol.NewLocalOnlinePodSource(mgr.GetClient()),
			agentClient,
			tuningEngine,
		)
		scheduler := dynamiccontrol.NewSyncScheduler(dynamicCoordinator)
		scheduler.PublishInterval = *publishInterval
		scheduler.ApplyInterval = *applyInterval
		scheduler.TaskTimeout = *taskTimeout

		if err := mgr.Add(scheduler); err != nil {
			return err
		}
		klog.Infof(
			"dynamic-control enabled, agent=%s publishInterval=%s applyInterval=%s timeout=%s",
			*agentAddr, *publishInterval, *applyInterval, *taskTimeout,
		)
	}

	klog.Info("starting k8s-mpam-controller manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		return err
	}
	return nil
}
