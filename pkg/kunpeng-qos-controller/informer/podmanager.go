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
// Create: 2023-01-12
// Description: This file defines PodManager passing and processing raw pod data
package informer

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog"

	"kunpeng.huawei.com/kunpeng-cloud-computing/pkg/kunpeng-qos-controller/typedef"
)

// ListOption is for filtering podInfo
type ListOption func(pi *typedef.PodInfo) bool

// PodManagerName is the unique identity of PodManager
const PodManagerName = "DefaultPodManager"

// PodManager manages pod cache and pushes cache change events based on external input
type PodManager struct {
	Pods *PodCache
}

// NewPodManager returns a PodManager pointer
func NewPodManager() *PodManager {
	manager := &PodManager{
		Pods: NewPodCache(),
	}
	return manager
}

// HandleEvent handles the event from publisher
func (manager *PodManager) HandleEvent(eventType typedef.EventType, event typedef.Event) {
	switch eventType {
	case typedef.RAWPODADD, typedef.RAWPODUPDATE, typedef.RAWPODDELETE:
		manager.handleWatchEvent(eventType, event)
	case typedef.RAWPODSYNCALL:
		manager.handleListEvent(eventType, event)
	default:
		klog.Infof("failed to process %s type event", eventType.String())
	}
}

// handleWatchEvent handles the watch event
func (manager *PodManager) handleWatchEvent(eventType typedef.EventType, event typedef.Event) {
	pod, err := eventToRawPod(event)
	if err != nil {
		klog.Warningf("%v", err.Error())
		return
	}

	switch eventType {
	case typedef.RAWPODADD:
		manager.addFunc(pod)
	case typedef.RAWPODUPDATE:
		manager.updateFunc(pod)
	case typedef.RAWPODDELETE:
		manager.deleteFunc(pod)
	default:
		klog.Errorf("invalid event type...")
	}
}

// handleListEvent handles the list event
func (manager *PodManager) handleListEvent(eventType typedef.EventType, event typedef.Event) {
	pods, err := eventToRawPods(event)
	if err != nil {
		klog.Errorf("%v", err.Error())
		return
	}
	switch eventType {
	case typedef.RAWPODSYNCALL:
		manager.sync(pods)
	default:
		klog.Errorf("invalid event type...")
	}
}

// sync replaces all Pod information sent over
func (manager *PodManager) sync(pods []*typedef.RawPod) {
	var newPods []*typedef.PodInfo
	for _, pod := range pods {
		if pod == nil || !pod.Running() {
			continue
		}
		newPods = append(newPods, pod.ExtractPodInfo())
	}
	manager.Pods.substitute(newPods)
}

// eventToRawPod converts the event interface to RawPod pointer
func eventToRawPod(e typedef.Event) (*typedef.RawPod, error) {
	pod, ok := e.(*corev1.Pod)
	if !ok {
		return nil, fmt.Errorf("failed to get raw pod information")
	}
	rawPod := typedef.RawPod(*pod)
	return &rawPod, nil
}

// eventToRawPods converts the event interface to RawPod pointer slice
func eventToRawPods(e typedef.Event) ([]*typedef.RawPod, error) {
	pods, ok := e.([]corev1.Pod)
	if !ok {
		return nil, fmt.Errorf("failed to get raw pod information")
	}
	toRawPodPointer := func(pod corev1.Pod) *typedef.RawPod {
		tmp := typedef.RawPod(pod)
		return &tmp
	}
	var pointerPods []*typedef.RawPod
	for _, pod := range pods {
		pointerPods = append(pointerPods, toRawPodPointer(pod))
	}
	return pointerPods, nil
}

// addFunc handles the pod add event
func (manager *PodManager) addFunc(pod *typedef.RawPod) {
	// condition 1: only add running pod
	if !pod.Running() {
		klog.Infof("pod %v is not running", pod.Name)
		return
	}
	// condition2: pod is not existed
	if manager.Pods.podExist(pod.ID()) {
		klog.Infof("pod %v has already added", pod.Name)
		return
	}
	// step1: get pod information
	podInfo := pod.ExtractPodInfo()
	if podInfo == nil {
		klog.Errorf("failed to extract information from raw pod")
		return
	}
	// step2. add pod information
	manager.tryAdd(podInfo)
}

// updateFunc handles the pod update event
func (manager *PodManager) updateFunc(pod *typedef.RawPod) {
	// step1: delete existed but not running pod
	if !pod.Running() {
		manager.tryDelete(pod.ID())
		return
	}

	// add or update information for running pod
	podInfo := pod.ExtractPodInfo()
	if podInfo == nil {
		klog.Errorf("failed to extract information from raw pod")
		return
	}
	// The calling order must be updated first and then added
	// step2: process exited and running pod
	manager.tryUpdate(podInfo)
	// step3: process not exited and running pod
	manager.tryAdd(podInfo)
}

// deleteFunc handles the pod delete event
func (manager *PodManager) deleteFunc(pod *typedef.RawPod) {
	manager.tryDelete(pod.ID())
}

// tryAdd tries to add pod info which is not added
func (manager *PodManager) tryAdd(podInfo *typedef.PodInfo) {
	// only add when pod is not existed
	if !manager.Pods.podExist(podInfo.UID) {
		manager.Pods.addPod(podInfo)
	}
}

// tryUpdate tries to update podinfo which is existed
func (manager *PodManager) tryUpdate(podInfo *typedef.PodInfo) {
	// only update when pod is existed
	if manager.Pods.podExist(podInfo.UID) {
		manager.Pods.updatePod(podInfo)
	}
}

// tryDelete tries to delete podinfo which is existed
func (manager *PodManager) tryDelete(id string) {
	// only delete when pod is existed
	oldPod := manager.Pods.getPod(id)
	if oldPod != nil {
		manager.Pods.delPod(id)
	}
}

func withOption(pi *typedef.PodInfo, opts []ListOption) bool {
	for _, opt := range opts {
		if !opt(pi) {
			return false
		}
	}
	return true
}

// ListPodsWithOptions filters and returns deep copy objects of all pods
func (manager *PodManager) ListPodsWithOptions(options ...ListOption) map[string]*typedef.PodInfo {
	// already deep copied
	allPods := manager.Pods.listPod()
	pods := make(map[string]*typedef.PodInfo, len(allPods))
	for _, pod := range allPods {
		if !withOption(pod, options) {
			continue
		}
		pods[pod.UID] = pod
	}
	return pods
}
