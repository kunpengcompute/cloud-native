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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	qosv1alpha1 "kunpeng.huawei.com/kunpeng-cloud-computing/api/kunpeng-qos-controller/v1alpha1"
)

func TestQoSPolicyDynamicUpdaterApplyReasonUnknown(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := qosv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("add scheme failed: %v", err)
	}

	cl := fake.NewClientBuilder().WithScheme(scheme).Build()
	updater := NewQoSPolicyDynamicUpdater(cl)
	if err := updater.ApplyReason(context.Background(), "node-a", InterferenceReasonUnknown); err == nil {
		t.Fatalf("ApplyReason() should fail on unknown reason")
	}
}

func TestQoSPolicyDynamicUpdaterApplyReasonCreate(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := qosv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("add scheme failed: %v", err)
	}

	cl := fake.NewClientBuilder().WithScheme(scheme).Build()
	updater := NewQoSPolicyDynamicUpdater(cl)
	nodeName := "node-a"

	if err := updater.ApplyReason(context.Background(), nodeName, InterferenceReasonL3); err != nil {
		t.Fatalf("ApplyReason() unexpected error: %v", err)
	}

	name := dynamicPolicyName(nodeName)
	var got qosv1alpha1.QoSPolicy
	if err := cl.Get(context.Background(), types.NamespacedName{Name: name}, &got); err != nil {
		t.Fatalf("get created policy failed: %v", err)
	}
	if got.Spec.NodeSelector[DefaultNodeSelectorKey] != nodeName {
		t.Fatalf("unexpected node selector: %v", got.Spec.NodeSelector)
	}
	if got.Spec.L3.Ways != 3 {
		t.Fatalf("unexpected l3 ways after one l3 step: %d", got.Spec.L3.Ways)
	}
	if got.Spec.L3.MAX != 90 {
		t.Fatalf("unexpected l3 max after one l3 step: %d", got.Spec.L3.MAX)
	}
}

func TestQoSPolicyDynamicUpdaterApplyReasonUpdate(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := qosv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("add scheme failed: %v", err)
	}

	nodeName := "node-a"
	name := dynamicPolicyName(nodeName)
	existing := &qosv1alpha1.QoSPolicy{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "qos.kunpeng.huawei.com/v1alpha1",
			Kind:       "QoSPolicy",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: qosv1alpha1.QoSPolicySpec{
			NodeSelector: map[string]string{
				DefaultNodeSelectorKey: nodeName,
			},
			MB: qosv1alpha1.MBPolicy{HDL: 1, PRI: 3, MIN: 0, MAX: 100},
			L3: qosv1alpha1.L3Policy{PRI: 0, MIN: 0, MAX: 100, Ways: 4},
		},
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(existing).Build()
	updater := NewQoSPolicyDynamicUpdater(cl)

	if err := updater.ApplyReason(context.Background(), nodeName, InterferenceReasonMB); err != nil {
		t.Fatalf("ApplyReason() unexpected error: %v", err)
	}

	var got qosv1alpha1.QoSPolicy
	if err := cl.Get(context.Background(), types.NamespacedName{Name: name}, &got); err != nil {
		t.Fatalf("get updated policy failed: %v", err)
	}
	if got.Spec.MB.MAX != 90 {
		t.Fatalf("expected MB.MAX=90 for one mb step, got %d", got.Spec.MB.MAX)
	}
	if got.Spec.L3.Ways != 4 {
		t.Fatalf("expected l3 ways unchanged for mb reason, got %d", got.Spec.L3.Ways)
	}
}

func TestQoSPolicyDynamicUpdaterApplyReasonNoChange(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := qosv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("add scheme failed: %v", err)
	}

	nodeName := "node-a"
	name := dynamicPolicyName(nodeName)
	initial := &qosv1alpha1.QoSPolicy{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "qos.kunpeng.huawei.com/v1alpha1",
			Kind:       "QoSPolicy",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: qosv1alpha1.QoSPolicySpec{
			NodeSelector: map[string]string{
				DefaultNodeSelectorKey: nodeName,
			},
			MB: qosv1alpha1.MBPolicy{HDL: 1, PRI: 3, MIN: 0, MAX: 100},
			L3: qosv1alpha1.L3Policy{PRI: 0, MIN: 0, MAX: 1, Ways: 1},
		},
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(initial).Build()
	updater := NewQoSPolicyDynamicUpdater(cl)

	var before qosv1alpha1.QoSPolicy
	if err := cl.Get(context.Background(), types.NamespacedName{Name: name}, &before); err != nil {
		t.Fatalf("get before failed: %v", err)
	}

	if err := updater.ApplyReason(context.Background(), nodeName, InterferenceReasonL3); err != nil {
		t.Fatalf("ApplyReason() unexpected error: %v", err)
	}

	var after qosv1alpha1.QoSPolicy
	if err := cl.Get(context.Background(), types.NamespacedName{Name: name}, &after); err != nil {
		t.Fatalf("get after failed: %v", err)
	}

	// No-op path should not change resourceVersion in fake client.
	if before.ResourceVersion != "" && before.ResourceVersion != after.ResourceVersion {
		t.Fatalf("expected no update when desired state unchanged, before rv=%s after rv=%s", before.ResourceVersion, after.ResourceVersion)
	}
}
