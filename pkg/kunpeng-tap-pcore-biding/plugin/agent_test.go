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
	"strconv"
	"testing"

	"github.com/containerd/nri/pkg/api"

	"kunpeng.huawei.com/kunpeng-cloud-computing/pkg/kunpeng-tap-pcore-biding/topology"
)

func TestMatchRequiresTwoCPULimit(t *testing.T) {
	agent := &Agent{
		namespaces:     toSet([]string{"default"}),
		runtimeClasses: toSet([]string{"kata-clh"}),
	}
	tests := []struct {
		name string
		pod  podInfo
		want bool
	}{
		{
			name: "one cpu limit",
			pod:  podInfo{namespace: "default", runtimeClass: "kata-clh", cpuQuota: 100000, cpuPeriod: 100000, cpuLimitKnown: true},
		},
		{
			name: "two cpu limit",
			pod:  podInfo{namespace: "default", runtimeClass: "kata-clh", cpuQuota: 200000, cpuPeriod: 100000, cpuLimitKnown: true},
			want: true,
		},
		{
			name: "two cpu limit with custom period",
			pod:  podInfo{namespace: "default", runtimeClass: "kata-clh", cpuQuota: 50000, cpuPeriod: 25000, cpuLimitKnown: true},
			want: true,
		},
		{
			name: "four cpu limit",
			pod:  podInfo{namespace: "default", runtimeClass: "kata-clh", cpuQuota: 400000, cpuPeriod: 100000, cpuLimitKnown: true},
		},
		{
			name: "unlimited cpu",
			pod:  podInfo{namespace: "default", runtimeClass: "kata-clh", cpuQuota: -1, cpuPeriod: 100000, cpuLimitKnown: true},
		},
		{
			name: "missing cpu resources",
			pod:  podInfo{namespace: "default", runtimeClass: "kata-clh"},
		},
		{
			name: "namespace not allowed",
			pod:  podInfo{namespace: "other", runtimeClass: "kata-clh", cpuQuota: 200000, cpuPeriod: 100000, cpuLimitKnown: true},
		},
		{
			name: "runtime class not allowed",
			pod:  podInfo{namespace: "default", runtimeClass: "runc", cpuQuota: 200000, cpuPeriod: 100000, cpuLimitKnown: true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := agent.match(tt.pod); got != tt.want {
				t.Fatalf("match() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestConvertPodUsesLimitAndIgnoresRequest(t *testing.T) {
	pod := &api.PodSandbox{
		Id:             "sandboxid",
		Name:           "test-pod",
		Namespace:      "default",
		RuntimeHandler: "kata-clh",
		Linux: &api.LinuxPodSandbox{
			PodResources: &api.LinuxResources{
				Cpu: &api.LinuxCPU{
					Shares: api.UInt64(1024),
					Quota:  api.Int64(200000),
					Period: api.UInt64(100000),
				},
			},
		},
	}

	got := convertPod(pod)
	agent := &Agent{
		namespaces:     toSet([]string{"default"}),
		runtimeClasses: toSet([]string{"kata-clh"}),
	}
	if !agent.match(got) {
		t.Fatal("Pod with a two-CPU limit should match regardless of CPU shares/request")
	}
}

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
		id:            "sandboxid",
		name:          "test-pod",
		namespace:     "default",
		runtimeClass:  "kata-clh",
		cgroupPath:    "/kubepods.slice/kubepods-poduid.slice",
		cpuQuota:      200000,
		cpuPeriod:     100000,
		cpuLimitKnown: true,
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

func TestReconcileOnlyUpdatesPodsWithTwoCPULimit(t *testing.T) {
	root := t.TempDir()
	pods := map[string]podInfo{}
	for _, cpuLimit := range []int64{1, 2, 4} {
		name := "limit-" + strconv.FormatInt(cpuLimit, 10)
		path := "/" + name
		mustWriteCpusetValue(t, filepath.Join(root, name), "0-7")
		pods[name] = podInfo{
			id:            name,
			name:          name,
			namespace:     "default",
			runtimeClass:  "kata-clh",
			cgroupPath:    path,
			cpuQuota:      cpuLimit * 100000,
			cpuPeriod:     100000,
			cpuLimitKnown: true,
		}
	}

	agent := &Agent{
		cfg:            Config{CgroupRoot: root},
		siblingPairs:   []topology.SiblingPair{{CPU0: 0, CPU1: 1}, {CPU0: 2, CPU1: 3}},
		namespaces:     toSet([]string{"default"}),
		runtimeClasses: toSet([]string{"kata-clh"}),
		pods:           pods,
	}

	if err := agent.reconcileOnce(); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	assertCpuset(t, filepath.Join(root, "limit-1"), "0-7")
	assertCpuset(t, filepath.Join(root, "limit-2"), "0,1")
	assertCpuset(t, filepath.Join(root, "limit-4"), "0-7")
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
