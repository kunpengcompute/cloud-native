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
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

type noopCPUQoSSetter struct{}

func (s noopCPUQoSSetter) SetPodCPUQoSLevel(_ context.Context, _ *corev1.Pod, _ string) error {
	return nil
}

func TestResolvePodGroup(t *testing.T) {
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{PodQoSGroupLabelKey: "group-b"}}}
	group, ok := resolvePodGroup(pod)
	if !ok || group != "group-b" {
		t.Fatalf("resolvePodGroup() = (%q, %v), want (group-b, true)", group, ok)
	}
}

func TestNewPodBindingReconciler(t *testing.T) {
	scheme := runtime.NewScheme()
	cl := fake.NewClientBuilder().WithScheme(scheme).Build()

	r := NewPodBindingReconciler(cl, scheme, true)
	if r.Client == nil || r.NodeIdentity == nil || r.MPAMBinder == nil || r.CPUQoSSetter == nil {
		t.Fatalf("constructor did not initialize required fields: %#v", r)
	}
	if len(r.Actions) == 0 {
		t.Fatalf("constructor should initialize default actions")
	}
}

func TestPodBindingReconcilerShouldProcessPod(t *testing.T) {
	r := &PodBindingReconciler{
		NodeIdentity: DefaultNodeIdentity{nodeName: "node-a"},
		CPUQoSSetter: noopCPUQoSSetter{},
		Actions: []PodAction{
			SetDynamicGroupLabelAction{},
			BindResctrlGroupAction{},
		},
	}

	t.Run("process when node matches and label exists", func(t *testing.T) {
		pod := &corev1.Pod{}
		pod.Spec.NodeName = "node-a"
		pod.Status.Phase = corev1.PodRunning
		pod.Labels = map[string]string{PodQoSGroupLabelKey: "group-a"}
		if !r.shouldProcessPod(pod) {
			t.Fatalf("shouldProcessPod() = false, want true")
		}
	})

	t.Run("skip when node does not match", func(t *testing.T) {
		pod := &corev1.Pod{}
		pod.Spec.NodeName = "node-b"
		pod.Status.Phase = corev1.PodRunning
		pod.Labels = map[string]string{PodQoSGroupLabelKey: "group-a"}
		if r.shouldProcessPod(pod) {
			t.Fatalf("shouldProcessPod() = true, want false")
		}
	})

	t.Run("process when offline label exists", func(t *testing.T) {
		pod := &corev1.Pod{}
		pod.Spec.NodeName = "node-a"
		pod.Status.Phase = corev1.PodRunning
		pod.Labels = map[string]string{WorkloadClassLabelKey: WorkloadClassOffline}
		if !r.shouldProcessPod(pod) {
			t.Fatalf("shouldProcessPod() = false, want true")
		}
	})
}

func TestPodBindingReconcilerEnsureOfflineGroupLabel(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("add corev1 scheme failed: %v", err)
	}

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "offline-a",
			UID:       types.UID("uid-a"),
			Labels: map[string]string{
				WorkloadClassLabelKey: WorkloadClassOffline,
			},
		},
		Spec: corev1.PodSpec{NodeName: "node-a"},
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(pod).Build()
	r := &PodBindingReconciler{Client: cl}

	changed, err := r.ensureOfflineGroupLabel(context.Background(), pod)
	if err != nil {
		t.Fatalf("ensureOfflineGroupLabel() unexpected error: %v", err)
	}
	if !changed {
		t.Fatalf("ensureOfflineGroupLabel() changed=false, want true")
	}
	if got := pod.Labels[PodQoSGroupLabelKey]; got != "qos-dynamic-offline-node-a" {
		t.Fatalf("group label=%q, want qos-dynamic-offline-node-a", got)
	}
}
