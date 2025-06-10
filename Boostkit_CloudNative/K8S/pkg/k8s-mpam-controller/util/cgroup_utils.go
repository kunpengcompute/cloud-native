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
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

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

// GetPodCgroupPerfEventParentDir gets the perfevent subsystem path for a cgroup
func GetPodCgroupPerfEventParentDir(cgroupDir string) string {
	return CgroupRootDir + "/" + PerfEventDir + "/" + cgroupDir
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

// GetPIDsInContainer gets the pid of all process running in container
func GetPIDsInContainer(podParentDir string, containerID string) ([]uint32, error) {
	containerCgroupPath, err := GetContainerCgroupParentDirByID(podParentDir, containerID)
	if err != nil {
		return nil, err
	}
	containerCgroupProcPath := CgroupRootDir + "/" + containerCgroupPath + "/" + CgroupProc
	rawContent, err := os.ReadFile(containerCgroupProcPath)
	if err != nil {
		return nil, err
	}

	return ParseCgroupProcs(string(rawContent))
}

// ParseCgroupProcs parses the content in cgroup.procs.
// pattern: `7742\n10971\n11049\n11051...`
func ParseCgroupProcs(content string) ([]uint32, error) {
	pidStrs := strings.Fields(strings.TrimSpace(content))
	pids := make([]uint32, len(pidStrs))
	for i := 0; i < len(pidStrs); i++ {
		p, err := strconv.ParseUint(pidStrs[i], 10, 32)
		if err != nil {
			return nil, fmt.Errorf("failed to parse row %s into pid, err: %w", pidStrs[i], err)
		}
		pids[i] = uint32(p)
	}
	return pids, nil
}

// GetCgroupPathFromSubSysAndFile gets the cgroup subsystem path for a cgroup path
func GetCgroupPathFromSubSysAndFile(cgroupDir string, subsys string, file string) string {
	return CgroupRootDir + "/" + subsys + "/" + cgroupDir + "/" + file
}
