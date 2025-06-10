// Copyright (c) Huawei Technologies Co., Ltd. 2023. All rights reserved.
// rubik licensed under the Mulan PSL v2.
// You can use this software according to the terms and conditions of the Mulan PSL v2.
// You may obtain a copy of Mulan PSL v2 at:
//     http://license.coscl.org.cn/MulanPSL2
// THIS SOFTWARE IS PROVIDED ON AN "AS IS" BASIS, WITHOUT WARRANTIES OF ANY KIND, EITHER EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO NON-INFRINGEMENT, MERCHANTABILITY OR FIT FOR A PARTICULAR
// PURPOSE.
// See the Mulan PSL v2 for more details.
// Author: Jiaqi Yang
// Create: 2023-01-05
// Description: This file defines podInfo

// Package typedef defines core struct and methods for k8s-mpam-controller
package typedef

import (
	"k8s.io/klog"

	"kunpeng.huawei.com/kunpeng-cloud-computing/pkg/k8s-mpam-controller/util"
)

const (
	// offlinekey is used for k8s-mpam-controll.yaml to tag a pod is offline pod
	OfflineKey = "kunpeng.com/offline"
)

// PodInfo represents pod
type PodInfo struct {
	Hierarchy
	Name            string
	UID             string
	Namespace       string
	IDContainersMap map[string]*ContainerInfo
	Annotations     map[string]string
}

// ContainerInfo represents container
type ContainerInfo struct {
	Hierarchy
	Name string
	ID   string
}

// NewPodInfo creates the PodInfo instance
func NewPodInfo(pod *RawPod) *PodInfo {
	return &PodInfo{
		Name:            pod.Name,
		Namespace:       pod.Namespace,
		UID:             pod.ID(),
		Hierarchy:       Hierarchy{Path: pod.CgroupPath()},
		IDContainersMap: pod.ExtractContainerInfos(),
		Annotations:     pod.DeepCopy().Annotations,
	}
}

// NewContainerInfo creates the ContainerInfo instance
func NewContainerInfo(con *RawContainer, podCgroupPath string) *ContainerInfo {
	conInfo := &ContainerInfo{}
	conInfo.ID = con.status.ContainerID
	conInfo.Name = con.status.Name
	cgroupPath, err := util.GetContainerCgroupParentDirByID(podCgroupPath, con.status.ContainerID)
	if err != nil {
		klog.Errorf("get container: %s cgroup path failed, err: %s", con.status.Name, err)
	}
	conInfo.Hierarchy = Hierarchy{Path: cgroupPath}
	return conInfo
}

// DeepCopy returns deepcopy object
func (pod *PodInfo) DeepCopy() *PodInfo {
	if pod == nil {
		return nil
	}

	var copypod = *pod
	// nil is different from empty value in golang
	if pod.IDContainersMap != nil {
		contMap := make(map[string]*ContainerInfo)
		for id, cont := range pod.IDContainersMap {
			contMap[id] = cont.DeepCopy()
		}
		copypod.IDContainersMap = contMap
	}

	if pod.Annotations != nil {
		annoMap := make(map[string]string)
		for k, v := range pod.Annotations {
			annoMap[k] = v
		}
		copypod.Annotations = annoMap
	}

	return &copypod
}

// DeepCopy returns deepcopy object.
func (cont *ContainerInfo) DeepCopy() *ContainerInfo {
	copyObject := *cont
	return &copyObject
}

// Offline is used to determine whether the pod is offline
func (pod *PodInfo) Offline() bool {
	var anno string

	if pod.Annotations != nil {
		anno = pod.Annotations[OfflineKey]
	}

	// Annotations have a higher priority than labels
	return anno == "true"
}

// Online is used to determine whether the pod is online
func (pod *PodInfo) Online() bool {
	return !pod.Offline()
}
