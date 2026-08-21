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
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	qosv1alpha1 "kunpeng.huawei.com/kunpeng-cloud-computing/api/kunpeng-qos-controller/v1alpha1"
)

type operationCountingClient struct {
	client.Client
	getCalls    int
	createCalls int
	updateCalls int
}

func (c *operationCountingClient) Get(
	ctx context.Context,
	key client.ObjectKey,
	obj client.Object,
	opts ...client.GetOption,
) error {
	c.getCalls++
	return c.Client.Get(ctx, key, obj, opts...)
}

func (c *operationCountingClient) Create(
	ctx context.Context,
	obj client.Object,
	opts ...client.CreateOption,
) error {
	c.createCalls++
	return c.Client.Create(ctx, obj, opts...)
}

func (c *operationCountingClient) Update(
	ctx context.Context,
	obj client.Object,
	opts ...client.UpdateOption,
) error {
	c.updateCalls++
	return c.Client.Update(ctx, obj, opts...)
}

func TestQoSPolicyDynamicUpdaterApplyReasonsNoActionableReason(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := qosv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("add scheme failed: %v", err)
	}

	baseClient := fake.NewClientBuilder().WithScheme(scheme).Build()
	cl := &operationCountingClient{Client: baseClient}
	updater := NewQoSPolicyDynamicUpdater(cl)
	if err := updater.ApplyReasons(context.Background(), "node-a", []InterferenceReason{
		InterferenceReasonNone,
		InterferenceReason("unknown"),
	}); err != nil {
		t.Fatalf("ApplyReasons() unexpected error: %v", err)
	}
	if cl.getCalls != 0 || cl.createCalls != 0 || cl.updateCalls != 0 {
		t.Fatalf(
			"expected no Kubernetes API access, got get=%d create=%d update=%d",
			cl.getCalls,
			cl.createCalls,
			cl.updateCalls,
		)
	}
}

func TestQoSPolicyDynamicUpdaterApplyReasonsCreate(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := qosv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("add scheme failed: %v", err)
	}

	baseClient := fake.NewClientBuilder().WithScheme(scheme).Build()
	cl := &operationCountingClient{Client: baseClient}
	updater := NewQoSPolicyDynamicUpdater(cl)
	nodeName := "node-a"

	if err := updater.ApplyReasons(context.Background(), nodeName, []InterferenceReason{
		InterferenceReasonL3,
		InterferenceReasonMB,
		InterferenceReasonCPU,
		InterferenceReasonL3,
		InterferenceReasonNone,
	}); err != nil {
		t.Fatalf("ApplyReasons() unexpected error: %v", err)
	}

	name := dynamicPolicyName(nodeName)
	var got qosv1alpha1.QoSPolicy
	if err := baseClient.Get(context.Background(), types.NamespacedName{Name: name}, &got); err != nil {
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
	if got.Spec.MB.MAX != 90 {
		t.Fatalf("unexpected mb max after one mb step: %d", got.Spec.MB.MAX)
	}
	if got.Spec.CPU.QoSLevel != -1 {
		t.Fatalf("unexpected cpu qos level: %d", got.Spec.CPU.QoSLevel)
	}
	if cl.getCalls != 1 || cl.createCalls != 1 || cl.updateCalls != 0 {
		t.Fatalf(
			"expected one get and create, got get=%d create=%d update=%d",
			cl.getCalls,
			cl.createCalls,
			cl.updateCalls,
		)
	}
}

func TestQoSPolicyDynamicUpdaterApplyReasonsUpdate(t *testing.T) {
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
	baseClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(existing).Build()
	cl := &operationCountingClient{Client: baseClient}
	updater := NewQoSPolicyDynamicUpdater(cl)

	if err := updater.ApplyReasons(context.Background(), nodeName, []InterferenceReason{
		InterferenceReason(" MB "),
		InterferenceReasonL3,
		InterferenceReasonCPU,
		InterferenceReasonMB,
	}); err != nil {
		t.Fatalf("ApplyReasons() unexpected error: %v", err)
	}

	var got qosv1alpha1.QoSPolicy
	if err := baseClient.Get(context.Background(), types.NamespacedName{Name: name}, &got); err != nil {
		t.Fatalf("get updated policy failed: %v", err)
	}
	if got.Spec.MB.MAX != 90 {
		t.Fatalf("expected MB.MAX=90 for one mb step, got %d", got.Spec.MB.MAX)
	}
	if got.Spec.L3.Ways != 3 || got.Spec.L3.MAX != 90 {
		t.Fatalf("expected one l3 step, got ways=%d max=%d", got.Spec.L3.Ways, got.Spec.L3.MAX)
	}
	if got.Spec.CPU.QoSLevel != -1 {
		t.Fatalf("expected cpu qos level=-1, got %d", got.Spec.CPU.QoSLevel)
	}
	if cl.getCalls != 1 || cl.createCalls != 0 || cl.updateCalls != 1 {
		t.Fatalf(
			"expected one get and update, got get=%d create=%d update=%d",
			cl.getCalls,
			cl.createCalls,
			cl.updateCalls,
		)
	}
}

func TestQoSPolicyDynamicUpdaterApplyReasonsNoChange(t *testing.T) {
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
			MB:  qosv1alpha1.MBPolicy{HDL: 1, PRI: 3, MIN: 0, MAX: 1},
			L3:  qosv1alpha1.L3Policy{PRI: 0, MIN: 0, MAX: 1, Ways: 1},
			CPU: qosv1alpha1.CPUPolicy{QoSLevel: -1},
		},
	}
	baseClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(initial).Build()
	cl := &operationCountingClient{Client: baseClient}
	updater := NewQoSPolicyDynamicUpdater(cl)

	if err := updater.ApplyReasons(context.Background(), nodeName, []InterferenceReason{
		InterferenceReasonCPU,
		InterferenceReasonMB,
		InterferenceReasonL3,
	}); err != nil {
		t.Fatalf("ApplyReasons() unexpected error: %v", err)
	}
	if cl.getCalls != 1 || cl.createCalls != 0 || cl.updateCalls != 0 {
		t.Fatalf(
			"expected one get and no write, got get=%d create=%d update=%d",
			cl.getCalls,
			cl.createCalls,
			cl.updateCalls,
		)
	}
}
