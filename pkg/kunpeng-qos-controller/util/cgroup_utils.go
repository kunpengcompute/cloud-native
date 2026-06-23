/*
Copyright 2022 The Koordinator Authors.

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
// Package util 实现常用的工具函数
package util

import (
	"path/filepath"

	corev1 "k8s.io/api/core/v1"
)

const (
	// cgroup root dir
	CgroupRootDir = "/sys/fs/cgroup"
	PerfEventDir  = "perf_event"
	CpuDir        = "cpu"
	CgroupProc    = "cgroup.proc"
)

// GetPodCgroupParentDir gets the cgroup parent dir for a pod
func GetPodCgroupParentDir(pod *corev1.Pod) string {
	qosClass := pod.Status.QOSClass

	return filepath.Join(
		CgroupPathFormatter.ParentDir,
		CgroupPathFormatter.QOSDirFn(qosClass),
		CgroupPathFormatter.PodDirFn(qosClass, string(pod.UID)),
	)
}

// GetContainerCgroupParentDirByID gets the CgroupParentDir for a container by container id
func GetContainerCgroupParentDirByID(podParentDir string, containerID string) (string, error) {
	_, containerDir, err := CgroupPathFormatter.ContainerDirFn(containerID)
	if err != nil {
		return "", err
	}

	return filepath.Join(
		podParentDir,
		containerDir,
	), nil
}
