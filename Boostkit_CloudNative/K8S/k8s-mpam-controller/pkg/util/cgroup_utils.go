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
	CgroupRootDir = "/sys/fs/cgroup"
	PerfEventDir  = "perf_event"
	CpuDir        = "cpu"
	CgroupProc    = "cgroup.proc"
)

func GetPodCgroupParentDir(pod *corev1.Pod) string {
	qosClass := pod.Status.QOSClass

	return filepath.Join(
		CgroupPathFormatter.ParentDir,
		CgroupPathFormatter.QOSDirFn(qosClass),
		CgroupPathFormatter.PodDirFn(qosClass, string(pod.UID)),
	)
}

func GetPodCgroupPerfEventParentDir(cgroupDir string) string {
	return CgroupRootDir + "/" + PerfEventDir + "/" + cgroupDir
}

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

func GetCgroupPathFromSubSysAndFile(cgroupDir string, subsys string, file string) string {
	return CgroupRootDir + "/" + subsys + "/" + cgroupDir + "/" + file
}
