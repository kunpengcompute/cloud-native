/*
Copyright (c) Huawei Technologies Co., Ltd. 2023-2026. All rights reserved.

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

package controller

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"

	"kunpeng.huawei.com/kunpeng-cloud-computing/pkg/k8s-mpam-controller/util"
)

const (
	defaultCgroupCPURoot  = "/sys/fs/cgroup/cpu"
	defaultTasksFileName  = "tasks"
	defaultCPUQoSFileName = "cpu.qos_level"
)

// LocalPodProcessBinder binds pod processes to target resctrl group by scanning
// cgroup tasks under local node filesystem.
type LocalPodProcessBinder struct {
	CgroupRoot    string
	CgroupCPURoot string
	ResctrlRoot   string
	TasksFile     string
}

func (b LocalPodProcessBinder) BindPodToGroup(ctx context.Context, pod *corev1.Pod, groupName string) error {
	_ = ctx
	if pod == nil {
		return fmt.Errorf("pod must not be nil")
	}

	podUID := string(pod.UID)
	if podUID == "" {
		return fmt.Errorf("pod uid must not be empty")
	}

	targetTasks := filepath.Join(b.resctrlRoot(), groupName, b.tasksFile())
	if _, err := os.Stat(targetTasks); err != nil {
		return fmt.Errorf("target resctrl tasks file not ready: %w", err)
	}

	taskFiles, err := b.resolvePodTaskFiles(pod)
	if err != nil {
		return err
	}
	if len(taskFiles) == 0 {
		return fmt.Errorf("no cgroup tasks files found for pod uid %s", podUID)
	}

	pids, err := readPIDs(taskFiles)
	if err != nil {
		return err
	}
	if len(pids) == 0 {
		return fmt.Errorf("no pids found for pod uid %s", podUID)
	}

	for _, pid := range pids {
		if err := writePID(targetTasks, pid); err != nil {
			return err
		}
	}
	klog.V(1).Infof(
		"pod %s/%s(uid=%s) wrote %d pids to group %s tasks %s",
		pod.Namespace, pod.Name, pod.UID, len(pids), groupName, targetTasks,
	)
	return nil
}

func (b LocalPodProcessBinder) resctrlRoot() string {
	if b.ResctrlRoot != "" {
		return b.ResctrlRoot
	}
	return defaultResctrlRoot
}

func (b LocalPodProcessBinder) cgroupCPURoot() string {
	if b.CgroupCPURoot != "" {
		return b.CgroupCPURoot
	}
	return defaultCgroupCPURoot
}

func (b LocalPodProcessBinder) tasksFile() string {
	if b.TasksFile != "" {
		return b.TasksFile
	}
	return defaultTasksFileName
}

func (b LocalPodProcessBinder) SetPodCPUQoSLevel(ctx context.Context, pod *corev1.Pod, level string) error {
	_ = ctx
	if pod == nil {
		return fmt.Errorf("pod must not be nil")
	}
	if strings.TrimSpace(level) == "" {
		return fmt.Errorf("cpu qos level must not be empty")
	}
	if string(pod.UID) == "" {
		return fmt.Errorf("pod uid must not be empty")
	}

	targets, err := b.resolvePodCPUQoSFiles(pod)
	if err != nil {
		return err
	}
	if len(targets) == 0 {
		return fmt.Errorf("no cpu.qos_level files found for pod uid %s", pod.UID)
	}

	for _, target := range targets {
		current, err := os.ReadFile(target)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				// Container may have exited during reconciliation; ignore and continue.
				continue
			}
			return fmt.Errorf("read cpu qos file %s failed: %w", target, err)
		}
		// If the current level is the same as the desired level, skip writing.
		if strings.TrimSpace(string(current)) == level {
			continue
		}
		// Kernel rejects reverse transition in our target environment.
		// Keep desired=0 from downgrading already-set -1 to avoid continuous errors.
		if strings.TrimSpace(string(current)) == "-1" && level == "0" {
			klog.V(1).Infof(
				"skip cpu.qos_level rollback for pod %s/%s container file %s: current=-1 desired=0",
				pod.Namespace, pod.Name, target,
			)
			continue
		}

		if err := os.WriteFile(target, []byte(level), 0o600); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				// Container may have exited during reconciliation; ignore and continue.
				continue
			}
			return fmt.Errorf("write %s to %s failed: %w", level, target, err)
		}
	}
	return nil
}

func (b LocalPodProcessBinder) resolvePodTaskFiles(pod *corev1.Pod) ([]string, error) {
	podParentDir := util.GetPodCgroupParentDir(pod)
	containerIDs := collectContainerIDs(pod)
	if len(containerIDs) == 0 {
		return nil, fmt.Errorf("pod has no container ids in status")
	}

	out := make([]string, 0, len(containerIDs))
	for _, cid := range containerIDs {
		containerParentDir, err := util.GetContainerCgroupParentDirByID(podParentDir, cid)
		if err != nil {
			return nil, fmt.Errorf("resolve container cgroup path for %s failed: %w", cid, err)
		}
		out = append(out, filepath.Join(b.cgroupCPURoot(), containerParentDir, b.tasksFile()))
	}
	return out, nil
}

func (b LocalPodProcessBinder) resolvePodCPUQoSFiles(pod *corev1.Pod) ([]string, error) {
	podParentDir := util.GetPodCgroupParentDir(pod)
	containerIDs := collectContainerIDs(pod)
	if len(containerIDs) == 0 {
		return nil, fmt.Errorf("pod has no container ids in status")
	}

	out := make([]string, 0, len(containerIDs))
	for _, cid := range containerIDs {
		containerParentDir, err := util.GetContainerCgroupParentDirByID(podParentDir, cid)
		if err != nil {
			return nil, fmt.Errorf("resolve container cgroup path for %s failed: %w", cid, err)
		}
		out = append(out, filepath.Join(b.cgroupCPURoot(), containerParentDir, defaultCPUQoSFileName))
	}
	return out, nil
}

func collectContainerIDs(pod *corev1.Pod) []string {
	out := make([]string, 0, len(pod.Status.ContainerStatuses))
	for _, st := range pod.Status.ContainerStatuses {
		if strings.TrimSpace(st.ContainerID) == "" {
			continue
		}
		out = append(out, st.ContainerID)
	}

	return out
}

func readPIDs(taskFiles []string) ([]string, error) {
	out := make([]string, 0)
	for _, tasks := range taskFiles {
		data, err := os.ReadFile(tasks)
		if err != nil {
			return nil, fmt.Errorf("read tasks file %s failed: %w", tasks, err)
		}
		for row := range strings.SplitSeq(string(data), "\n") {
			pid := strings.TrimSpace(row)
			if pid == "" {
				continue
			}
			out = append(out, pid)
		}
	}
	return out, nil
}

func writePID(targetTasksPath, pid string) error {
	f, err := os.OpenFile(targetTasksPath, os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("open target tasks file %s failed: %w", targetTasksPath, err)
	}
	defer f.Close()

	if _, err := f.WriteString(pid); err != nil {
		if errors.Is(err, syscall.ESRCH) {
			return nil
		}
		return fmt.Errorf("write pid %s to %s failed: %w", pid, targetTasksPath, err)
	}

	return nil
}
