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
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/containerd/nri/pkg/api"

	"kunpeng.huawei.com/kunpeng-cloud-computing/pkg/kunpeng-tap-pcore-biding/topology"
)

func TestDefaultConfig(t *testing.T) {
	got := DefaultConfig()
	if got.SocketPath != "/var/run/nri/nri.sock" || got.ScanInterval != 10*time.Second || got.CgroupRoot != "" || got.DryRun {
		t.Fatalf("unexpected default scalar config: %+v", got)
	}
	if !reflect.DeepEqual(got.Namespaces, []string{"default"}) {
		t.Fatalf("Namespaces = %v, want [default]", got.Namespaces)
	}
	if !reflect.DeepEqual(got.RuntimeClasses, []string{"kata"}) {
		t.Fatalf("RuntimeClasses = %v, want [kata]", got.RuntimeClasses)
	}
}

func TestNew(t *testing.T) {
	if _, err := New(DefaultConfig(), nil); err == nil {
		t.Fatal("New() with no sibling pairs succeeded, want error")
	}

	cfg := DefaultConfig()
	cfg.SocketPath = ""
	agent, err := New(cfg, []topology.SiblingPair{{CPU0: 0, CPU1: 1}})
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	if agent.stub == nil || agent.reconcileCh == nil || agent.pods == nil {
		t.Fatalf("New() returned an incompletely initialized agent: %+v", agent)
	}
	if _, ok := agent.namespaces["default"]; !ok {
		t.Fatal("default namespace is missing")
	}
	if _, ok := agent.runtimeClasses["kata"]; !ok {
		t.Fatal("default runtime class is missing")
	}
}

func TestConfigure(t *testing.T) {
	agent := &Agent{mask: api.EventMask(42)}
	got, err := agent.Configure(context.Background(), "", "", "")
	if err != nil {
		t.Fatalf("Configure() failed: %v", err)
	}
	if got != agent.mask {
		t.Fatalf("Configure() mask = %v, want %v", got, agent.mask)
	}
}

func TestPodLifecycleCallbacks(t *testing.T) {
	agent := &Agent{
		pods:        map[string]podInfo{"stale": {id: "stale"}},
		reconcileCh: make(chan struct{}, 1),
	}
	pod1 := testPodSandbox("pod-1", "first")
	pod2 := testPodSandbox("pod-2", "second")

	updates, err := agent.Synchronize(context.Background(), []*api.PodSandbox{pod1, nil}, nil)
	if err != nil || updates != nil {
		t.Fatalf("Synchronize() = (%v, %v), want (nil, nil)", updates, err)
	}
	if got := agent.listPods(); len(got) != 1 || got[0].id != pod1.Id {
		t.Fatalf("pods after Synchronize() = %+v, want only %q", got, pod1.Id)
	}
	assertReconcileRequested(t, agent)

	if err := agent.RunPodSandbox(context.Background(), pod2); err != nil {
		t.Fatalf("RunPodSandbox() failed: %v", err)
	}
	if got := agent.listPods(); len(got) != 2 {
		t.Fatalf("pod count after RunPodSandbox() = %d, want 2", len(got))
	}
	assertReconcileRequested(t, agent)

	if err := agent.StopPodSandbox(context.Background(), pod1); err != nil {
		t.Fatalf("StopPodSandbox() failed: %v", err)
	}
	if got := agent.listPods(); len(got) != 1 || got[0].id != pod2.Id {
		t.Fatalf("pods after StopPodSandbox() = %+v, want only %q", got, pod2.Id)
	}
	assertReconcileRequested(t, agent)

	if err := agent.RemovePodSandbox(context.Background(), pod2); err != nil {
		t.Fatalf("RemovePodSandbox() failed: %v", err)
	}
	if got := agent.listPods(); len(got) != 0 {
		t.Fatalf("pods after RemovePodSandbox() = %+v, want empty", got)
	}
	assertReconcileRequested(t, agent)
}

func TestNilPodUpdatesAreIgnored(t *testing.T) {
	agent := &Agent{pods: map[string]podInfo{}, reconcileCh: make(chan struct{}, 1)}
	agent.upsertPod(nil)
	agent.removePod(nil)
	if len(agent.pods) != 0 {
		t.Fatalf("nil pod update changed pod map: %+v", agent.pods)
	}
}

func TestRequestReconcileCoalescesNotifications(t *testing.T) {
	agent := &Agent{reconcileCh: make(chan struct{}, 1)}
	agent.requestReconcile()
	agent.requestReconcile()
	if got := len(agent.reconcileCh); got != 1 {
		t.Fatalf("queued reconcile notifications = %d, want 1", got)
	}
}

func TestListPodsSortsDeterministically(t *testing.T) {
	agent := &Agent{pods: map[string]podInfo{
		"3": {id: "3", namespace: "z", name: "pod"},
		"2": {id: "2", namespace: "a", name: "pod"},
		"1": {id: "1", namespace: "a", name: "pod"},
	}}
	got := agent.listPods()
	want := []string{"1", "2", "3"}
	ids := make([]string, 0, len(got))
	for _, pod := range got {
		ids = append(ids, pod.id)
	}
	if !reflect.DeepEqual(ids, want) {
		t.Fatalf("listPods() ids = %v, want %v", ids, want)
	}
}

func TestConvertPodUsesFallbackResources(t *testing.T) {
	pod := &api.PodSandbox{
		Id:             "sandbox-id",
		Name:           "test-pod",
		Namespace:      "default",
		RuntimeHandler: "kata-clh",
		Linux: &api.LinuxPodSandbox{
			CgroupParent: "/kubepods/test",
			Resources: &api.LinuxResources{Cpu: &api.LinuxCPU{
				Quota:  api.Int64(200000),
				Period: api.UInt64(100000),
			}},
		},
	}
	got := convertPod(pod)
	if got.id != pod.Id || got.cgroupPath != pod.Linux.CgroupParent || got.runtimeClass != pod.RuntimeHandler {
		t.Fatalf("convertPod() identity fields = %+v", got)
	}
	if !got.cpuLimitKnown || got.cpuQuota != 200000 || got.cpuPeriod != 100000 {
		t.Fatalf("convertPod() CPU fields = %+v", got)
	}
}

func TestRunReturnsStubError(t *testing.T) {
	wantErr := errors.New("stub failed")
	stub := newFakeStub(wantErr)
	agent := &Agent{
		stub:           stub,
		cfg:            Config{ScanInterval: time.Hour},
		pods:           map[string]podInfo{},
		reconcileCh:    make(chan struct{}, 1),
		namespaces:     map[string]struct{}{},
		runtimeClasses: map[string]struct{}{},
	}
	if err := agent.Run(context.Background()); !errors.Is(err, wantErr) {
		t.Fatalf("Run() error = %v, want %v", err, wantErr)
	}
}

func TestRunStopsStubWhenContextIsCancelled(t *testing.T) {
	stub := newFakeStub(nil)
	agent := &Agent{
		stub:           stub,
		cfg:            Config{ScanInterval: time.Hour},
		pods:           map[string]podInfo{},
		reconcileCh:    make(chan struct{}, 1),
		namespaces:     map[string]struct{}{},
		runtimeClasses: map[string]struct{}{},
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- agent.Run(ctx)
	}()
	<-stub.started
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run() error = %v, want nil", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Run() did not return after context cancellation")
	}
	select {
	case <-stub.stopped:
	default:
		t.Fatal("Run() did not stop the NRI stub")
	}
}

func TestParseCpusetMount(t *testing.T) {
	t.Run("cgroup v1 cpuset", func(t *testing.T) {
		line := "36 25 0:32 / /sys/fs/cgroup/cpuset rw - cgroup cgroup rw,cpuset"
		got, ok := parseCpusetMount(line)
		if !ok || got != "/sys/fs/cgroup/cpuset" {
			t.Fatalf("parseCpusetMount() = (%q, %v)", got, ok)
		}
	})

	t.Run("cgroup v1 without cpuset", func(t *testing.T) {
		line := "36 25 0:32 / /sys/fs/cgroup/cpu rw - cgroup cgroup rw,cpu"
		if got, ok := parseCpusetMount(line); ok || got != "/sys/fs/cgroup/cpu" {
			t.Fatalf("parseCpusetMount() = (%q, %v), want (%q, false)", got, ok, "/sys/fs/cgroup/cpu")
		}
	})

	t.Run("cgroup v2 controllers", func(t *testing.T) {
		root := t.TempDir()
		if err := os.WriteFile(filepath.Join(root, "cgroup.controllers"), []byte("cpu cpuset io"), 0o644); err != nil {
			t.Fatalf("write controllers: %v", err)
		}
		line := "36 25 0:32 / " + root + " rw - cgroup2 cgroup rw"
		got, ok := parseCpusetMount(line)
		if !ok || got != root {
			t.Fatalf("parseCpusetMount() = (%q, %v), want (%q, true)", got, ok, root)
		}
	})

	for _, line := range []string{"", "malformed", "1 2 3 4 - cgroup cgroup rw,cpuset"} {
		if got, ok := parseCpusetMount(line); ok || got != "" {
			t.Fatalf("parseCpusetMount(%q) = (%q, %v), want empty false", line, got, ok)
		}
	}
}

func TestMountHelpers(t *testing.T) {
	if !hasMountOption("rw,cpuset,cpu", "cpuset") {
		t.Fatal("hasMountOption() did not find cpuset")
	}
	if hasMountOption("rw,cpu", "cpuset") {
		t.Fatal("hasMountOption() unexpectedly found cpuset")
	}
	if got := unescapeMountPath(`/sys/fs/cgroup/a\040b\011c\012d`); got != "/sys/fs/cgroup/a b\tc\nd" {
		t.Fatalf("unescapeMountPath() = %q", got)
	}
	if hasCgroup2Controller(t.TempDir(), "cpuset") {
		t.Fatal("hasCgroup2Controller() succeeded without cgroup.controllers")
	}
}

func testPodSandbox(id, name string) *api.PodSandbox {
	return &api.PodSandbox{
		Id:             id,
		Name:           name,
		Namespace:      "default",
		RuntimeHandler: "kata-clh",
	}
}

func assertReconcileRequested(t *testing.T, agent *Agent) {
	t.Helper()
	select {
	case <-agent.reconcileCh:
	default:
		t.Fatal("reconcile was not requested")
	}
}

type fakeStub struct {
	runErr   error
	started  chan struct{}
	stopped  chan struct{}
	stopOnce sync.Once
}

func newFakeStub(runErr error) *fakeStub {
	return &fakeStub{
		runErr:  runErr,
		started: make(chan struct{}),
		stopped: make(chan struct{}),
	}
}

func (s *fakeStub) Run(context.Context) error {
	close(s.started)
	if s.runErr != nil {
		return s.runErr
	}
	<-s.stopped
	return nil
}

func (s *fakeStub) Start(context.Context) error { return nil }

func (s *fakeStub) Stop() {
	s.stopOnce.Do(func() {
		close(s.stopped)
	})
}

func (s *fakeStub) Wait() {}

func (s *fakeStub) UpdateContainers(updates []*api.ContainerUpdate) ([]*api.ContainerUpdate, error) {
	return updates, nil
}

func (s *fakeStub) RegistrationTimeout() time.Duration { return 0 }

func (s *fakeStub) RequestTimeout() time.Duration { return 0 }
