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

package dynamiccontrol

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestLocalOnlinePodSourceListOnlinePodCgroups(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("add corev1 scheme failed: %v", err)
	}

	pods := []runtime.Object{
		newPod("default", "online-a", "uid-a", "node-a", corev1.PodRunning, corev1.PodQOSBurstable,
			map[string]string{WorkloadClassLabelKey: WorkloadClassOnline}),
		newPod("default", "online-b-other-node", "uid-b", "node-b", corev1.PodRunning, corev1.PodQOSBurstable,
			map[string]string{WorkloadClassLabelKey: WorkloadClassOnline}),
		newPod("default", "offline-a", "uid-c", "node-a", corev1.PodRunning, corev1.PodQOSBurstable,
			map[string]string{WorkloadClassLabelKey: WorkloadClassOffline}),
		newPod("default", "pending-online", "uid-d", "node-a", corev1.PodPending, corev1.PodQOSBurstable,
			map[string]string{WorkloadClassLabelKey: WorkloadClassOnline}),
		newPod("kube-system", "system-online", "uid-e", "node-a", corev1.PodRunning, corev1.PodQOSBurstable,
			map[string]string{WorkloadClassLabelKey: WorkloadClassOnline}),
	}

	cl := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(pods...).Build()
	source := &LocalOnlinePodSource{Client: cl}

	got, err := source.ListOnlinePodCgroups(context.Background(), "node-a")
	if err != nil {
		t.Fatalf("ListOnlinePodCgroups() unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 online pods on node-a, got %d: %+v", len(got), got)
	}
	if got[0].Namespace != "default" || got[0].Name != "online-a" || got[0].UID != "uid-a" {
		t.Fatalf("unexpected pod selected: %+v", got[0])
	}
	if got[1].Namespace != "kube-system" || got[1].Name != "system-online" || got[1].UID != "uid-e" {
		t.Fatalf("unexpected pod selected: %+v", got[1])
	}
	for _, item := range got {
		if item.CgroupPath == "" {
			t.Fatalf("cgroup path should not be empty: %+v", item)
		}
	}
}

func TestLocalOnlinePodSourceEmptyNodeName(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("add corev1 scheme failed: %v", err)
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).Build()
	source := &LocalOnlinePodSource{Client: cl}
	if _, err := source.ListOnlinePodCgroups(context.Background(), ""); err == nil {
		t.Fatalf("expected error for empty node name")
	}
}

func newPod(
	namespace, name, uid, node string,
	phase corev1.PodPhase,
	qos corev1.PodQOSClass,
	labels map[string]string,
) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
			UID:       types.UID(uid),
			Labels:    labels,
		},
		Spec: corev1.PodSpec{
			NodeName: node,
		},
		Status: corev1.PodStatus{
			Phase:    phase,
			QOSClass: qos,
		},
	}
}
