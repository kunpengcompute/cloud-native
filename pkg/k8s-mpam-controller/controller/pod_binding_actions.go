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
	"fmt"
	"strconv"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/klog/v2"

	qosv1alpha1 "kunpeng.huawei.com/kunpeng-cloud-computing/api/k8s-mpam-controller/v1alpha1"
)

// PodAction is one composable behavior unit for PodBindingReconciler.
// Reconciler runs actions in sequence and each action decides if it applies.
type PodAction interface {
	Name() string
	Match(pod *corev1.Pod) bool
	Apply(ctx context.Context, r *PodBindingReconciler, pod *corev1.Pod) error
}

// DefaultPodActions builds the default action pipeline for PodBindingReconciler.
// When dynamic control is disabled, offline-group auto labeling is skipped.
func DefaultPodActions(enableDynamicControl bool) []PodAction {
	actions := make([]PodAction, 0, 2)
	if enableDynamicControl {
		actions = append(actions, SetDynamicGroupLabelAction{})
	}
	actions = append(actions, BindResctrlGroupAction{})
	actions = append(actions, SetCPUQoSAction{})
	return actions
}

// SetDynamicGroupLabelAction sets QoS group label for offline pods.
type SetDynamicGroupLabelAction struct{}

func (a SetDynamicGroupLabelAction) Name() string {
	return "set-dynamic-group-label"
}

func (a SetDynamicGroupLabelAction) Match(pod *corev1.Pod) bool {
	return isdynamicWorkload(pod)
}

func (a SetDynamicGroupLabelAction) Apply(ctx context.Context, r *PodBindingReconciler, pod *corev1.Pod) error {
	changed, err := r.ensureOfflineGroupLabel(ctx, pod)
	if err != nil {
		return err
	}
	if !changed {
		return nil
	}
	groupName, _ := resolvePodGroup(pod)
	klog.V(1).Infof(
		"set Pod %s/%s(uid=%s) label %s=%s for offline workload",
		pod.Namespace, pod.Name, pod.UID, PodQoSGroupLabelKey, groupName,
	)
	return nil
}

// BindResctrlGroupAction binds pod processes into target resctrl group.
type BindResctrlGroupAction struct{}

func (a BindResctrlGroupAction) Name() string {
	return "bind-resctrl-group"
}

// SetCPUQoSAction applies cpu.qos_level based on group-named policy.
type SetCPUQoSAction struct{}

func (a SetCPUQoSAction) Name() string {
	return "set-cpu-qos"
}

func (a SetCPUQoSAction) Match(pod *corev1.Pod) bool {
	_, ok := resolvePodGroup(pod)
	return ok
}

func (a SetCPUQoSAction) Apply(ctx context.Context, r *PodBindingReconciler, pod *corev1.Pod) error {
	groupName, ok := resolvePodGroup(pod)
	if !ok {
		return nil
	}
	var policy qosv1alpha1.QoSPolicy
	if err := r.Get(ctx, clientKeyForPolicy(groupName), &policy); err != nil {
		if apierrors.IsNotFound(err) {
			// Policy may be deleting or not created yet; skip and wait for next reconcile.
			return nil
		}
		return err
	}
	level := strconv.FormatInt(int64(policy.Spec.CPU.QoSLevel), 10)
	if err := r.cpuQoSSetter().SetPodCPUQoSLevel(ctx, pod, level); err != nil {
		return err
	}
	klog.V(1).Infof(
		"set Pod %s/%s(uid=%s) cpu.qos_level=%s from policy/group=%s",
		pod.Namespace, pod.Name, pod.UID, level, groupName,
	)
	return nil
}

func (a BindResctrlGroupAction) Match(pod *corev1.Pod) bool {
	_, ok := resolvePodGroup(pod)
	return ok
}

func (a BindResctrlGroupAction) Apply(ctx context.Context, r *PodBindingReconciler, pod *corev1.Pod) error {
	groupName, ok := resolvePodGroup(pod)
	if !ok {
		return nil
	}
	if err := r.MPAMBinder.BindPodToGroup(ctx, pod, groupName); err != nil {
		return err
	}
	klog.V(1).Infof(
		"bound Pod %s/%s(uid=%s) to resctrl group=%s",
		pod.Namespace, pod.Name, pod.UID, groupName,
	)
	return nil
}

func runPodActions(ctx context.Context, r *PodBindingReconciler, pod *corev1.Pod) error {
	for _, action := range r.Actions {
		if action == nil || !action.Match(pod) {
			continue
		}
		if err := action.Apply(ctx, r, pod); err != nil {
			return fmt.Errorf("pod action %s failed: %w", action.Name(), err)
		}
	}
	return nil
}
