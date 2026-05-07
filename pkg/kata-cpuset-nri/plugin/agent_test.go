/*
 * Copyright (c) 2026 Huawei Technology corp.
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

package plugin

import (
	"os"
	"path/filepath"
	"testing"

	"kunpeng.huawei.com/kunpeng-cloud-computing/pkg/kata-cpuset-nri/topology"
)

func TestResolveCgroupPathRootJoin(t *testing.T) {
	root := t.TempDir()
	cgroupPath := "/kubepods.slice/kubepods-burstable.slice/kubepods-burstable-poduid.slice"
	expected := filepath.Join(root, "kubepods.slice/kubepods-burstable.slice/kubepods-burstable-poduid.slice")
	mustWriteCpuset(t, expected)

	agent := &Agent{cfg: Config{CgroupRoot: root}}
	got, err := agent.resolveCgroupPath(cgroupPath)
	if err != nil {
		t.Fatalf("resolve cgroup path: %v", err)
	}
	if got != expected {
		t.Fatalf("resolved path mismatch: got %q want %q", got, expected)
	}
}

func TestResolveCgroupPathSystemdScope(t *testing.T) {
	root := t.TempDir()
	cgroupPath := "kubepods-burstable-poduid.slice:cri-containerd:containerid"
	expected := filepath.Join(root, "kubepods.slice/kubepods-burstable.slice",
		"kubepods-burstable-poduid.slice/cri-containerd-containerid.scope")
	mustWriteCpuset(t, expected)

	agent := &Agent{cfg: Config{CgroupRoot: root}}
	got, err := agent.resolveCgroupPath(cgroupPath)
	if err != nil {
		t.Fatalf("resolve cgroup path: %v", err)
	}
	if got != expected {
		t.Fatalf("resolved path mismatch: got %q want %q", got, expected)
	}
}

func TestResolveCgroupPathFallbackFindByBase(t *testing.T) {
	root := t.TempDir()
	cgroupPath := "/kubepods-besteffort-poduid.slice"
	expected := filepath.Join(root, "kubepods.slice/kubepods-besteffort.slice/kubepods-besteffort-poduid.slice")
	mustWriteCpuset(t, expected)

	agent := &Agent{cfg: Config{CgroupRoot: root}}
	got, err := agent.resolveCgroupPath(cgroupPath)
	if err != nil {
		t.Fatalf("resolve cgroup path: %v", err)
	}
	if got != expected {
		t.Fatalf("resolved path mismatch: got %q want %q", got, expected)
	}
}

func TestResolveCgroupPathsIncludesSandboxCgroup(t *testing.T) {
	root := t.TempDir()
	pod := podInfo{
		id:         "sandboxid",
		cgroupPath: "/kubepods.slice/kubepods-poduid.slice",
	}
	parent := filepath.Join(root, "kubepods.slice/kubepods-poduid.slice")
	sandbox := filepath.Join(root, "kubepods-poduid.slice:cri-containerd:sandboxid")
	mustWriteCpuset(t, parent)
	mustWriteCpuset(t, sandbox)

	agent := &Agent{cfg: Config{CgroupRoot: root}}
	got, err := agent.resolveCgroupPaths(pod)
	if err != nil {
		t.Fatalf("resolve cgroup paths: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("resolved path count mismatch: got %d want 2 (%v)", len(got), got)
	}
	if got[0] != parent || got[1] != sandbox {
		t.Fatalf("resolved paths mismatch: got %v want [%q %q]", got, parent, sandbox)
	}
}

func TestReconcileUpdatesSandboxWhenParentAlreadyMatches(t *testing.T) {
	root := t.TempDir()
	pod := podInfo{
		id:           "sandboxid",
		name:         "test-pod",
		namespace:    "default",
		runtimeClass: "kata-clh",
		cgroupPath:   "/kubepods.slice/kubepods-poduid.slice",
	}
	parent := filepath.Join(root, "kubepods.slice/kubepods-poduid.slice")
	sandbox := filepath.Join(root, "kubepods-poduid.slice:cri-containerd:sandboxid")
	mustWriteCpusetValue(t, parent, "0-1")
	mustWriteCpusetValue(t, sandbox, "2-3")

	agent := &Agent{
		cfg:            Config{CgroupRoot: root},
		siblingPairs:   []topology.SiblingPair{{CPU0: 0, CPU1: 1}, {CPU0: 2, CPU1: 3}},
		namespaces:     toSet([]string{"default"}),
		runtimeClasses: toSet([]string{"kata-clh"}),
		pods:           map[string]podInfo{pod.id: pod},
	}

	if err := agent.reconcileOnce(); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	assertCpuset(t, parent, "0,1")
	assertCpuset(t, sandbox, "0,1")
}

func mustWriteCpuset(t *testing.T, dir string) {
	t.Helper()
	mustWriteCpusetValue(t, dir, "0-3")
}

func mustWriteCpusetValue(t *testing.T, dir, cpuset string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("create cgroup dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "cpuset.cpus"), []byte(cpuset), 0o644); err != nil {
		t.Fatalf("create cpuset file: %v", err)
	}
}

func assertCpuset(t *testing.T, dir, want string) {
	t.Helper()
	got, err := os.ReadFile(filepath.Join(dir, "cpuset.cpus"))
	if err != nil {
		t.Fatalf("read cpuset file: %v", err)
	}
	if string(got) != want {
		t.Fatalf("cpuset mismatch for %s: got %q want %q", dir, string(got), want)
	}
}
