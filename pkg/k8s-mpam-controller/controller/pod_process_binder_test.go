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
	"os"
	"path/filepath"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"kunpeng.huawei.com/kunpeng-cloud-computing/pkg/k8s-mpam-controller/util"
)

func TestMPAMPodBinderBindPodToGroup(t *testing.T) {
	root := t.TempDir()
	cgroupRoot := filepath.Join(root, "cgroup")
	resctrlRoot := filepath.Join(root, "resctrl")
	podUID := "123e4567-e89b-12d3-a456-426614174000"
	groupName := "mpam-g1"

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			UID: types.UID(podUID),
		},
		Status: corev1.PodStatus{
			QOSClass: corev1.PodQOSGuaranteed,
			ContainerStatuses: []corev1.ContainerStatus{
				{ContainerID: "containerd://abcdef123456"},
			},
		},
	}

	podParent := util.GetPodCgroupParentDir(pod)
	containerParent, err := util.GetContainerCgroupParentDirByID(podParent, "containerd://abcdef123456")
	if err != nil {
		t.Fatalf("resolve container parent failed: %v", err)
	}
	taskDir := filepath.Join(cgroupRoot, containerParent)
	if err := os.MkdirAll(taskDir, 0o755); err != nil {
		t.Fatalf("mkdir tasks dir failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(taskDir, defaultTasksFileName), []byte("101\n202\n"), 0o600); err != nil {
		t.Fatalf("write cgroup tasks failed: %v", err)
	}

	targetDir := filepath.Join(resctrlRoot, groupName)
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		t.Fatalf("mkdir target dir failed: %v", err)
	}
	targetTasks := filepath.Join(targetDir, defaultTasksFileName)
	if err := os.WriteFile(targetTasks, []byte{}, 0o600); err != nil {
		t.Fatalf("create target tasks failed: %v", err)
	}

	binder := MPAMPodBinder{
		CgroupCPURoot: cgroupRoot,
		ResctrlRoot: resctrlRoot,
		TasksFile:   defaultTasksFileName,
	}
	if err := binder.BindPodToGroup(context.Background(), pod, groupName); err != nil {
		t.Fatalf("BindPodToGroup() unexpected error: %v", err)
	}

	data, err := os.ReadFile(targetTasks)
	if err != nil {
		t.Fatalf("read target tasks failed: %v", err)
	}
	// Regular file semantics differ from kernel cgroup tasks file. Here we only
	// assert that binder eventually writes pod pids and the final write is present.
	if !strings.Contains(string(data), "202") {
		t.Fatalf("target tasks should contain final written pid, got: %q", string(data))
	}
}

func TestMPAMPodBinderBindPodToGroupNoTaskFile(t *testing.T) {
	root := t.TempDir()
	binder := MPAMPodBinder{
		CgroupCPURoot: filepath.Join(root, "cgroup"),
		ResctrlRoot: filepath.Join(root, "resctrl"),
	}
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{UID: "uid-a"},
		Status: corev1.PodStatus{
			QOSClass: corev1.PodQOSGuaranteed,
			ContainerStatuses: []corev1.ContainerStatus{
				{ContainerID: "containerd://abcdef123456"},
			},
		},
	}
	if err := binder.BindPodToGroup(context.Background(), pod, "group-a"); err == nil {
		t.Fatalf("expected error when target tasks file does not exist")
	}
}

func TestMPAMPodBinderBindPodToGroupEmptyUID(t *testing.T) {
	root := t.TempDir()
	binder := MPAMPodBinder{
		CgroupCPURoot: filepath.Join(root, "cgroup"),
		ResctrlRoot: filepath.Join(root, "resctrl"),
	}
	pod := &corev1.Pod{}
	if err := binder.BindPodToGroup(context.Background(), pod, "group-a"); err == nil {
		t.Fatalf("expected error when pod uid is empty")
	}
}

func TestMPAMPodBinderSetPodCPUQoSLevel(t *testing.T) {
	root := t.TempDir()
	cgroupCPURoot := filepath.Join(root, "cpu")
	podUID := "123e4567-e89b-12d3-a456-426614174000"

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			UID: types.UID(podUID),
		},
		Status: corev1.PodStatus{
			QOSClass: corev1.PodQOSGuaranteed,
			ContainerStatuses: []corev1.ContainerStatus{
				{ContainerID: "containerd://abcdef123456"},
				{ContainerID: "containerd://deadbeef7788"},
			},
		},
	}

	podParent := util.GetPodCgroupParentDir(pod)
	containerA, err := util.GetContainerCgroupParentDirByID(podParent, "containerd://abcdef123456")
	if err != nil {
		t.Fatalf("resolve container A parent failed: %v", err)
	}
	containerB, err := util.GetContainerCgroupParentDirByID(podParent, "containerd://deadbeef7788")
	if err != nil {
		t.Fatalf("resolve container B parent failed: %v", err)
	}

	fileA := filepath.Join(cgroupCPURoot, containerA, defaultCPUQoSFileName)
	fileB := filepath.Join(cgroupCPURoot, containerB, defaultCPUQoSFileName)
	for _, f := range []string{fileA, fileB} {
		if err := os.MkdirAll(filepath.Dir(f), 0o755); err != nil {
			t.Fatalf("mkdir cpu qos dir failed: %v", err)
		}
		if err := os.WriteFile(f, []byte("0"), 0o600); err != nil {
			t.Fatalf("write initial cpu qos failed: %v", err)
		}
	}

	binder := MPAMPodBinder{CgroupCPURoot: cgroupCPURoot}
	if err := binder.SetPodCPUQoSLevel(context.Background(), pod, "-1"); err != nil {
		t.Fatalf("SetPodCPUQoSLevel() unexpected error: %v", err)
	}

	for _, f := range []string{fileA, fileB} {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("read cpu qos file failed: %v", err)
		}
		if strings.TrimSpace(string(data)) != "-1" {
			t.Fatalf("unexpected cpu.qos_level in %s: %q", f, string(data))
		}
	}
}

func TestMPAMPodBinderSetPodCPUQoSLevelIdempotentSkipWrite(t *testing.T) {
	root := t.TempDir()
	cgroupCPURoot := filepath.Join(root, "cpu")
	podUID := "123e4567-e89b-12d3-a456-426614174001"

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			UID: types.UID(podUID),
		},
		Status: corev1.PodStatus{
			QOSClass: corev1.PodQOSGuaranteed,
			ContainerStatuses: []corev1.ContainerStatus{
				{ContainerID: "containerd://abcdef123456"},
			},
		},
	}

	podParent := util.GetPodCgroupParentDir(pod)
	containerParent, err := util.GetContainerCgroupParentDirByID(podParent, "containerd://abcdef123456")
	if err != nil {
		t.Fatalf("resolve container parent failed: %v", err)
	}
	target := filepath.Join(cgroupCPURoot, containerParent, defaultCPUQoSFileName)
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatalf("mkdir cpu qos dir failed: %v", err)
	}
	if err := os.WriteFile(target, []byte("-1\n"), 0o600); err != nil {
		t.Fatalf("write initial cpu qos failed: %v", err)
	}
	// Make file read-only. If implementation still writes when value is already "-1",
	// this test would fail with permission error.
	if err := os.Chmod(target, 0o400); err != nil {
		t.Fatalf("chmod cpu qos file failed: %v", err)
	}

	binder := MPAMPodBinder{CgroupCPURoot: cgroupCPURoot}
	if err := binder.SetPodCPUQoSLevel(context.Background(), pod, "-1"); err != nil {
		t.Fatalf("SetPodCPUQoSLevel() should skip write when value unchanged, got error: %v", err)
	}
}

func TestMPAMPodBinderSetPodCPUQoSLevelSkipRollback(t *testing.T) {
	root := t.TempDir()
	cgroupCPURoot := filepath.Join(root, "cpu")
	podUID := "123e4567-e89b-12d3-a456-426614174002"

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "pod-a",
			UID:       types.UID(podUID),
		},
		Status: corev1.PodStatus{
			QOSClass: corev1.PodQOSGuaranteed,
			ContainerStatuses: []corev1.ContainerStatus{
				{ContainerID: "containerd://abcdef123456"},
			},
		},
	}

	podParent := util.GetPodCgroupParentDir(pod)
	containerParent, err := util.GetContainerCgroupParentDirByID(podParent, "containerd://abcdef123456")
	if err != nil {
		t.Fatalf("resolve container parent failed: %v", err)
	}
	target := filepath.Join(cgroupCPURoot, containerParent, defaultCPUQoSFileName)
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatalf("mkdir cpu qos dir failed: %v", err)
	}
	if err := os.WriteFile(target, []byte("-1"), 0o600); err != nil {
		t.Fatalf("write initial cpu qos failed: %v", err)
	}

	binder := MPAMPodBinder{CgroupCPURoot: cgroupCPURoot}
	if err := binder.SetPodCPUQoSLevel(context.Background(), pod, "0"); err != nil {
		t.Fatalf("SetPodCPUQoSLevel() should skip rollback without error, got: %v", err)
	}

	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read cpu qos file failed: %v", err)
	}
	if strings.TrimSpace(string(data)) != "-1" {
		t.Fatalf("cpu.qos_level should remain -1 after rollback skip, got %q", string(data))
	}
}

func TestReadPIDs(t *testing.T) {
	root := t.TempDir()
	tasksA := filepath.Join(root, "a.tasks")
	tasksB := filepath.Join(root, "b.tasks")

	if err := os.WriteFile(tasksA, []byte("101\n202\n"), 0o600); err != nil {
		t.Fatalf("write tasksA failed: %v", err)
	}
	if err := os.WriteFile(tasksB, []byte("202\n303\n"), 0o600); err != nil {
		t.Fatalf("write tasksB failed: %v", err)
	}

	pids, err := readPIDs([]string{tasksA, tasksB})
	if err != nil {
		t.Fatalf("readPIDs() unexpected error: %v", err)
	}
	if len(pids) != 4 || pids[0] != "101" || pids[1] != "202" || pids[2] != "202" || pids[3] != "303" {
		t.Fatalf("unexpected pid set: %#v", pids)
	}
}
