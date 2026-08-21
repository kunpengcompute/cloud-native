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
	"fmt"
	"reflect"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	qosv1alpha1 "kunpeng.huawei.com/kunpeng-cloud-computing/api/kunpeng-qos-controller/v1alpha1"
)

const (
	DefaultNodeSelectorKey = "kubernetes.io/hostname"

	defaultDynamicMBStep     = int32(10)
	defaultDynamicL3WaysStep = int32(1)
	defaultDynamicL3MaxStep  = int32(10)
)

// DynamicPolicyUpdater updates per-node dynamic QoSPolicy CR.
// QoSPolicyDynamicUpdater is the default implementation.
type DynamicPolicyUpdater interface {
	ApplyReasons(ctx context.Context, nodeName string, reasons []InterferenceReason) error
}

// QoSPolicyDynamicUpdater upserts one per-node dynamic QoSPolicy CR.
type QoSPolicyDynamicUpdater struct {
	Client client.Client

	// Per-update tuning step. Keep small to avoid large oscillation.
	MBStep     int32
	L3WaysStep int32
	L3MaxStep  int32
}

// NewQoSPolicyDynamicUpdater creates updater with default tuning steps.
func NewQoSPolicyDynamicUpdater(c client.Client) *QoSPolicyDynamicUpdater {
	return &QoSPolicyDynamicUpdater{
		Client:     c,
		MBStep:     defaultDynamicMBStep,
		L3WaysStep: defaultDynamicL3WaysStep,
		L3MaxStep:  defaultDynamicL3MaxStep,
	}
}

// ApplyReasons ensures and updates one dynamic QoSPolicy for given node.
func (u *QoSPolicyDynamicUpdater) ApplyReasons(
	ctx context.Context,
	nodeName string,
	reasons []InterferenceReason,
) error {
	if u.Client == nil {
		return fmt.Errorf("client must not be nil")
	}
	if strings.TrimSpace(nodeName) == "" {
		return fmt.Errorf("node name must not be empty")
	}

	reasons, _ = normalizeInterferenceReasons(reasons, false)
	if len(reasons) == 0 {
		return nil
	}

	name := dynamicPolicyName(nodeName)

	var current qosv1alpha1.QoSPolicy
	err := u.Client.Get(ctx, types.NamespacedName{Name: name}, &current)
	if err != nil {
		if apierrors.IsNotFound(err) {
			spec := defaultDynamicPolicySpec(nodeName)
			u.applyReasonsToSpec(&spec, reasons, u.MBStep, u.L3WaysStep, u.L3MaxStep)
			newObj := &qosv1alpha1.QoSPolicy{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "qos.kunpeng.huawei.com/v1alpha1",
					Kind:       "QoSPolicy",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: name,
				},
				Spec: spec,
			}
			return u.Client.Create(ctx, newObj)
		}
		return err
	}

	desired := current.Spec
	if desired.NodeSelector == nil {
		desired.NodeSelector = map[string]string{}
	}
	desired.NodeSelector[DefaultNodeSelectorKey] = nodeName
	if desired.L3.Ways < 1 {
		// Ensure valid baseline for current CRD validation.
		desired.L3.Ways = 1
	}
	u.applyReasonsToSpec(&desired, reasons, u.MBStep, u.L3WaysStep, u.L3MaxStep)
	if reflect.DeepEqual(current.Spec, desired) {
		return nil
	}
	current.Spec = desired
	return u.Client.Update(ctx, &current)
}

func dynamicPolicyName(nodeName string) string {
	name := strings.ToLower(strings.TrimSpace(nodeName))
	name = strings.ReplaceAll(name, ".", "-")
	name = strings.ReplaceAll(name, "_", "-")
	if name == "" {
		name = "unknown-node"
	}
	return "qos-dynamic-offline-" + name
}

func defaultDynamicPolicySpec(nodeName string) qosv1alpha1.QoSPolicySpec {
	return qosv1alpha1.QoSPolicySpec{
		NodeSelector: map[string]string{
			DefaultNodeSelectorKey: nodeName,
		},
		MB: qosv1alpha1.MBPolicy{
			HDL: 1,
			PRI: 3,
			MIN: 0,
			MAX: 100,
		},
		L3: qosv1alpha1.L3Policy{
			PRI:  0,
			MIN:  0,
			MAX:  100,
			Ways: 4,
		},
		CPU: qosv1alpha1.CPUPolicy{
			QoSLevel: 0,
		},
	}
}

func (u *QoSPolicyDynamicUpdater) applyReasonsToSpec(
	spec *qosv1alpha1.QoSPolicySpec,
	reasons []InterferenceReason,
	mbStep int32,
	l3WaysStep int32,
	l3MaxStep int32,
) {
	for _, reason := range reasons {
		switch reason {
		case InterferenceReasonMB:
			spec.MB.MAX = maxInt32(spec.MB.MAX-mbStep, 1)
		case InterferenceReasonL3:
			spec.L3.Ways = maxInt32(spec.L3.Ways-l3WaysStep, 1)
			spec.L3.MAX = maxInt32(spec.L3.MAX-l3MaxStep, 1)
		case InterferenceReasonCPU:
			// CPU interference maps to cpu.qos_level control through QoSPolicy.
			spec.CPU.QoSLevel = -1
		}
	}
}

func maxInt32(v int32, min int32) int32 {
	if v < min {
		return min
	}
	return v
}
