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
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	qosv1alpha1 "kunpeng.huawei.com/kunpeng-cloud-computing/api/kunpeng-qos-controller/v1alpha1"
)

type fixedNodeIdentity struct {
	name   string
	labels map[string]string
}

func (n fixedNodeIdentity) NodeName() string { return n.name }

func (n fixedNodeIdentity) NodeLabels(_ context.Context, _ client.Client) (map[string]string, error) {
	return n.labels, nil
}

type countingResctrlManager struct {
	ensureCalls int
	applyCalls  int
	deleteCalls int
}

func (m *countingResctrlManager) EnsureGroup(_ context.Context, _ string) error {
	m.ensureCalls++
	return nil
}

func (m *countingResctrlManager) ApplyConfig(_ context.Context, _ string, _ ResctrlConfig) error {
	m.applyCalls++
	return nil
}

func (m *countingResctrlManager) DeleteGroup(_ context.Context, _ string) error {
	m.deleteCalls++
	return nil
}

func TestReconcile_AddFinalizerAndApplyInSamePass(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := qosv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("add scheme failed: %v", err)
	}
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("add core scheme failed: %v", err)
	}

	policy := &qosv1alpha1.QoSPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: "policy-a",
		},
		Spec: qosv1alpha1.QoSPolicySpec{
			MB: qosv1alpha1.MBPolicy{HDL: 1, PRI: 3, MIN: 0, MAX: 100},
			L3: qosv1alpha1.L3Policy{PRI: 0, MIN: 0, MAX: 100, Ways: 4},
		},
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(policy).Build()
	resctrl := &countingResctrlManager{}

	r := &QoSPolicyReconciler{
		Client:       cl,
		NodeIdentity: fixedNodeIdentity{name: "node-a", labels: map[string]string{}},
		Resctrl:      resctrl,
	}

	_, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: policy.Name}})
	if err != nil {
		t.Fatalf("reconcile failed: %v", err)
	}

	var got qosv1alpha1.QoSPolicy
	if err := cl.Get(context.Background(), types.NamespacedName{Name: policy.Name}, &got); err != nil {
		t.Fatalf("get policy failed: %v", err)
	}
	if !containsString(got.Finalizers, defaultQoSPolicyFinalizer) {
		t.Fatalf("expected finalizer %q to be added, got %v", defaultQoSPolicyFinalizer, got.Finalizers)
	}
	if resctrl.ensureCalls != 1 || resctrl.applyCalls != 1 {
		t.Fatalf("first reconcile should add finalizer and apply config once, got ensure=%d apply=%d", resctrl.ensureCalls, resctrl.applyCalls)
	}
}

func containsString(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}
