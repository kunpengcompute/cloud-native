// Copyright (c) Huawei Technologies Co., Ltd. 2023. All rights reserved.
// rubik licensed under the Mulan PSL v2.
// You can use this software according to the terms and conditions of the Mulan PSL v2.
// You may obtain a copy of Mulan PSL v2 at:
//
//	http://license.coscl.org.cn/MulanPSL2
//
// THIS SOFTWARE IS PROVIDED ON AN "AS IS" BASIS, WITHOUT WARRANTIES OF ANY KIND, EITHER EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO NON-INFRINGEMENT, MERCHANTABILITY OR FIT FOR A PARTICULAR
// PURPOSE.
// See the Mulan PSL v2 for more details.
// Author: Jiaqi Yang
// Create: 2023-01-05
// Description: This file defines apiinformer which interact with kubernetes apiserver
package informer

import (
	"context"
	"fmt"
	"os"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog"

	"kunpeng.huawei.com/kunpeng-cloud-computing/pkg/k8s-mpam-controller/typedef"
)

// APIServerInformer interacts with k8s api server and forward data to the internal
type APIServerInformer struct {
	podManager *PodManager
	client     *kubernetes.Clientset
	nodeName   string
}

// NewAPIServerInformer creates an PIServerInformer instance
func NewAPIServerInformer(client *kubernetes.Clientset, podManager *PodManager) (*APIServerInformer, error) {
	informer := &APIServerInformer{
		podManager: podManager,
		client:     client,
	}

	// filter pods on current nodes
	nodeName := os.Getenv("NODE_NAME")
	if nodeName == "" {
		return nil, fmt.Errorf("unable to get node name from environment variable")
	}
	informer.nodeName = nodeName

	return informer, nil
}

// Start starts and enables PIServerInformer
func (informer *APIServerInformer) Start(ctx context.Context) error {
	const specNodeNameField = "spec.nodeName"
	// set options to return only pods on the current node.
	var fieldSelector = fields.OneTermEqualSelector(specNodeNameField, informer.nodeName).String()
	informer.listFunc(fieldSelector)
	informer.watchFunc(ctx, fieldSelector)
	return nil
}

func (informer *APIServerInformer) listFunc(fieldSelector string) {
	pods, err := informer.client.CoreV1().Pods("").List(context.Background(),
		metav1.ListOptions{FieldSelector: fieldSelector})
	if err != nil {
		klog.Errorf("failed to get pod list from APIServer informer: %v", err)
		return
	}
	informer.podManager.handleListEvent(typedef.RAWPODSYNCALL, pods.Items)
}

func (informer *APIServerInformer) watchFunc(ctx context.Context, fieldSelector string) {
	const reSyncTime = 30
	kubeInformerFactory := informers.NewSharedInformerFactoryWithOptions(informer.client,
		time.Duration(reSyncTime)*time.Second,
		informers.WithTweakListOptions(func(options *metav1.ListOptions) {
			options.FieldSelector = fieldSelector
		}))
	kubeInformerFactory.Core().V1().Pods().Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    informer.AddFunc,
		UpdateFunc: informer.UpdateFunc,
		DeleteFunc: informer.DeleteFunc,
	})
	kubeInformerFactory.Start(ctx.Done())
}

// AddFunc handles the raw pod increase event
func (informer *APIServerInformer) AddFunc(obj interface{}) {
	informer.podManager.handleWatchEvent(typedef.RAWPODADD, obj)
}

// UpdateFunc handles the raw pod update event
func (informer *APIServerInformer) UpdateFunc(oldObj, newObj interface{}) {
	informer.podManager.handleWatchEvent(typedef.RAWPODUPDATE, newObj)
}

// DeleteFunc handles the raw pod deletion event
func (informer *APIServerInformer) DeleteFunc(obj interface{}) {
	informer.podManager.handleWatchEvent(typedef.RAWPODDELETE, obj)
}
