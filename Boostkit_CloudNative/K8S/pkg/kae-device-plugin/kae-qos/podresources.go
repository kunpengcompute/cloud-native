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
	"google.golang.org/grpc/credentials/insecure"
	"k8s.io/klog"
	podresourcesapi "k8s.io/kubelet/pkg/apis/podresources/v1"
)

const (
	podResourcesPath = "/var/lib/kubelet/pod-resources"
	kubeletSock      = "kubelet.sock"
)

var supportDeivce map[string]struct{} = map[string]struct{}{
	"kae.kunpeng.com/hisi_hpre": {},
	"kae.kunpeng.com/hisi_zip":  {},
	"kae.kunpeng.com/hisi_sec2": {},
}

// resourceName -> deviceIds
type ResourceInfo map[string][]string

type podResourceManager struct {
	// podUID -> resourceName -> deviceIds
	// podUID in this struct is <namespace/podName>
	podResources   map[string]ResourceInfo
	resourcesMutex sync.Mutex
	client         podresourcesapi.PodResourcesListerClient
	syncTicker     *time.Ticker
	syncDone       chan bool
}

func NewPodResourceManager(syncPeriod time.Duration) (*podResourceManager, error) {
	conn, err := grpc.NewClient(filepath.Join("unix://", podResourcesPath, kubeletSock), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to create grpc client: %w", err)
	}

	client := podresourcesapi.NewPodResourcesListerClient(conn)
	return &podResourceManager{
		syncTicker:   time.NewTicker(syncPeriod),
		syncDone:     make(chan bool, 1),
		podResources: make(map[string]ResourceInfo),
		client:       client,
	}, nil
}

func (pm *podResourceManager) updatePodResources(waitTime time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), waitTime)
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

func (pm *podResourceManager) run(timeout time.Duration) {
	defer pm.syncTicker.Stop()

	for {
		err := pm.updatePodResources(timeout)
		if err != nil {
			klog.Errorf("Failed to update pod resources: %+v", err)
		}

		select {
		case <-pm.syncDone:
			return
		case <-pm.syncTicker.C:
		}
	}
}

func (pm *podResourceManager) getDeviceIds(podUid, resourceName string) []string {
	pm.resourcesMutex.Lock()
	defer pm.resourcesMutex.Unlock()

	if _, ok := pm.podResources[podUid]; !ok {
		return nil
	}

	return pm.podResources[podUid][resourceName]
}

func isSupportDevice(resourceName string) bool {
	_, ok := supportDeivce[resourceName]
	return ok
}
