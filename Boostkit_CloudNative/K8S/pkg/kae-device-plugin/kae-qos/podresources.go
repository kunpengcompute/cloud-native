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

package kaeqos

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
	podresourcesapi "k8s.io/kubelet/pkg/apis/podresources/v1"
)

const (
	podResourcesPath = "/var/lib/kubelet/pod-resources"
	kubeletSock      = "kubelet.sock"
)

var supportDeivce map[string]string = map[string]string{
	"kae.kunpeng.com/hisi_hpre": "hisi_hpre",
	"kae.kunpeng.com/hisi_zip":  "hisi_zip",
	"kae.kunpeng.com/hisi_sec2": "hisi_sec2",
}

// resourceName -> deviceIds
type ResourceInfo map[string][]string

type podResourceClient struct {
	// podUID -> resourceName -> deviceIds
	// podUID in this struct is <namespace/podName>
	podResources           map[string]ResourceInfo
	resourcesMutex         sync.Mutex
	timemout               time.Duration
	client                 podresourcesapi.PodResourcesListerClient
	podResourcesGetEnabled bool
}

func NewPodResourceClient(watiTime time.Duration) (*podResourceClient, error) {
	conn, err := grpc.NewClient(filepath.Join("unix://", podResourcesPath, kubeletSock), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to create grpc client: %w", err)
	}

	client := podresourcesapi.NewPodResourcesListerClient(conn)
	return &podResourceClient{
		timemout:               watiTime,
		podResources:           make(map[string]ResourceInfo),
		client:                 client,
		podResourcesGetEnabled: false,
	}, nil
}

func (pm *podResourceClient) init() {
	pm.podResourcesGetEnabled = pm.checkEnablePodResourcesGet()
}

// checkEnablePodResourcesGet checks if the PodResourcesGet API is enabled
// The Get method in the PodResources API is only available in newer Kubernetes versions.
// For more details, see https://kubernetes.io/zh-cn/docs/concepts/extend-kubernetes/compute-storage-net/device-plugins/
func (pm *podResourceClient) checkEnablePodResourcesGet() bool {
	ctx, cancel := context.WithTimeout(context.Background(), pm.timemout)
	defer cancel()

	// Retry when err is not "NotFount" or "Unimplemented", maybe network traffic jam
	err := retry.OnError(wait.Backoff{
		Steps:    3,
		Duration: 100 * time.Millisecond,
		Factor:   2.0,
		Jitter:   0.1,
	}, func(err error) bool {
		code := status.Code(err)
		return code != codes.NotFound && code != codes.Unimplemented
	}, func() error {
		_, err := pm.client.Get(ctx, &podresourcesapi.GetPodResourcesRequest{
			PodName:      "no-exist",
			PodNamespace: "default",
		})

		return err
	})

	if err != nil {
		// If the error is "rpc error: code = Unimplemented", feature gate is OFF
		if status.Code(err) == codes.Unimplemented {
			klog.Infof("PodResourcesGet API is not enabled, use List to update podResources")
			return false
		}

		if status.Code(err) == codes.NotFound {
			return true
		}

		return false
	}

	return true
}

func (pm *podResourceClient) fetchPodResources(namespace, name string) error {
	if pm.podResourcesGetEnabled {
		return pm.getPodResources(namespace, name)
	}

	return pm.listPodResources()
}

func (pm *podResourceClient) getPodResources(namespace, name string) error {
	ctx, cancel := context.WithTimeout(context.Background(), pm.timemout)
	defer cancel()

	resp, err := pm.client.Get(ctx, &podresourcesapi.GetPodResourcesRequest{
		PodName:      name,
		PodNamespace: namespace,
	})
	if err != nil {
		return fmt.Errorf("failed to get %s pod resources: %w", namespace+"/"+name, err)
	}

	pm.resourcesMutex.Lock()
	defer pm.resourcesMutex.Unlock()
	resourceInfo := map[string][]string{}
	for _, container := range resp.PodResources.Containers {
		getSupportedDevices(resourceInfo, container.Devices)
	}

	namespaceName := namespace + "/" + name
	pm.podResources[namespaceName] = resourceInfo

	return nil
}

func (pm *podResourceClient) listPodResources() error {
	ctx, cancel := context.WithTimeout(context.Background(), pm.timemout)
	defer cancel()

	resp, err := pm.client.List(ctx, &podresourcesapi.ListPodResourcesRequest{})
	if err != nil {
		return fmt.Errorf("failed to list pod resources: %w", err)
	}

	pm.resourcesMutex.Lock()
	defer pm.resourcesMutex.Unlock()
	for _, pr := range resp.PodResources {
		resourceInfo := map[string][]string{}
		for _, container := range pr.Containers {
			getSupportedDevices(resourceInfo, container.Devices)
		}

		namespaceName := pr.Namespace + "/" + pr.Name
		pm.podResources[namespaceName] = resourceInfo
	}

	return nil
}

func getSupportedDevices(resourceInfo map[string][]string, devices []*podresourcesapi.ContainerDevices) {
	if resourceInfo == nil {
		return
	}

	for _, device := range devices {
		if !isSupportDevice(device.ResourceName) {
			continue
		}
		resourceInfo[device.ResourceName] = append(resourceInfo[device.ResourceName], device.DeviceIds...)
	}
}

func (pm *podResourceClient) getDeviceIds(namespacedName, resourceName string) []string {
	pm.resourcesMutex.Lock()
	defer pm.resourcesMutex.Unlock()

	if _, ok := pm.podResources[namespacedName]; !ok {
		return nil
	}

	return pm.podResources[namespacedName][resourceName]
}

func isSupportDevice(resourceName string) bool {
	_, ok := supportDeivce[resourceName]
	return ok
}

func getResourceType(resourceName string) string {
	return supportDeivce[resourceName]
}
