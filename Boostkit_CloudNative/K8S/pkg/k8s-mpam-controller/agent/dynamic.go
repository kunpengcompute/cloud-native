/*
Copyright (c) Huawei Technologies Co., Ltd. 2023-2023. All rights reserved.

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
package agent

import (
	"context"

	"k8s.io/client-go/kubernetes"
	"k8s.io/klog"

	"kunpeng.huawei.com/kunpeng-cloud-computing/pkg/k8s-mpam-controller/dynamic"
	informer "kunpeng.huawei.com/kunpeng-cloud-computing/pkg/k8s-mpam-controller/informer"
)

// RunDynamic start the MPAM Dynamic Service
func RunDynamic(client *kubernetes.Clientset, ctx context.Context) {
	podManager := informer.NewPodManager()

	apiserverInformer, err := informer.NewAPIServerInformer(client, podManager)
	if err != nil {
		klog.Errorf("apiserver create failed, err: %s", err)
		return
	}

	apiserverInformer.Start(ctx)

	dynamicMpam := dynamic.NewDynCache(podManager)
	if err := dynamicMpam.PreStart(); err != nil {
		klog.Errorf("mpam dynamic prestart failed, err: %s", err)
		return
	}
	go dynamicMpam.Run(ctx)
}
