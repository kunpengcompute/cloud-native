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
	"strings"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

const (
	// PodQoSGroupLabelKey is the label key for pod-to-group binding.
	PodQoSGroupLabelKey = "qos.kunpeng.huawei.com/group"
	// WorkloadClassLabelKey marks workload class for pod-level behavior.
	WorkloadClassLabelKey = "qos.kunpeng.huawei.com/workload-class"
	// WorkloadClassOffline marks offline workloads.
	WorkloadClassOffline = "offline"
)

// PodProcessBinder binds pod processes into target resctrl group.
// Concrete cgroup/tasks traversal should be provided by a later implementation.
type PodProcessBinder interface {
	BindPodToGroup(ctx context.Context, pod *corev1.Pod, groupName string) error
}

// PodCPUQoSSetter sets cpu qos level for all pod containers.
type PodCPUQoSSetter interface {
	SetPodCPUQoSLevel(ctx context.Context, pod *corev1.Pod, level string) error
}

// PodBindingReconciler watches pods and binds pod processes to target resctrl group.
type PodBindingReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	NodeIdentity         NodeIdentity
	Binder               PodProcessBinder
	CPUQoSSetter         PodCPUQoSSetter
	EnableDynamicControl bool
	Actions              []PodAction
}

// NewPodBindingReconciler builds a PodBindingReconciler with defaults.
func NewPodBindingReconciler(
	k8sClient client.Client,
	scheme *runtime.Scheme,
	enableDynamicControl bool,
) *PodBindingReconciler {
	reconciler := &PodBindingReconciler{
		Client:               k8sClient,
		Scheme:               scheme,
		NodeIdentity:         NewDefaultNodeIdentity(),
		Binder:               LocalPodProcessBinder{},
		CPUQoSSetter:         LocalPodProcessBinder{},
		EnableDynamicControl: enableDynamicControl,
	}
	reconciler.Actions = DefaultPodActions(enableDynamicControl)
	return reconciler
}

func (r *PodBindingReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	klog.V(4).Infof("reconcile Pod %s/%s for QoS binding on node %s", req.Namespace, req.Name, r.nodeIdentity().NodeName())

	var pod corev1.Pod
	if err := r.Get(ctx, req.NamespacedName, &pod); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if !pod.DeletionTimestamp.IsZero() {
		// Pod deletion does not need explicit unbind in V1. Process exit will release tasks.
		return ctrl.Result{}, nil
	}

	if !r.shouldProcessPod(&pod) {
		return ctrl.Result{}, nil
	}

	if err := runPodActions(ctx, r, &pod); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *PodBindingReconciler) nodeIdentity() NodeIdentity {
	if r.NodeIdentity != nil {
		return r.NodeIdentity
	}
	return NewDefaultNodeIdentity()
}

func (r *PodBindingReconciler) cpuQoSSetter() PodCPUQoSSetter {
	if r.CPUQoSSetter != nil {
		return r.CPUQoSSetter
	}
	return LocalPodProcessBinder{}
}

func resolvePodGroup(pod *corev1.Pod) (string, bool) {
	if pod == nil {
		return "", false
	}
	if v := pod.Labels[PodQoSGroupLabelKey]; v != "" {
		return v, true
	}
	return "", false
}

func (r *PodBindingReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Pod{}).
		WithEventFilter(predicate.NewPredicateFuncs(func(obj client.Object) bool {
			pod, ok := obj.(*corev1.Pod)
			if !ok {
				return false
			}
			return r.shouldProcessPod(pod)
		})).
		Complete(r)
}

func (r *PodBindingReconciler) shouldProcessPod(pod *corev1.Pod) bool {
	if pod == nil {
		return false
	}
	if pod.Spec.NodeName == "" || pod.Spec.NodeName != r.nodeIdentity().NodeName() {
		return false
	}
	if pod.Status.Phase != corev1.PodRunning {
		return false
	}
	for _, action := range r.Actions {
		if action != nil && action.Match(pod) {
			return true
		}
	}
	return false
}

func isOfflineWorkload(pod *corev1.Pod) bool {
	if pod == nil || len(pod.Labels) == 0 {
		return false
	}
	return pod.Labels[WorkloadClassLabelKey] == WorkloadClassOffline
}

func (r *PodBindingReconciler) ensureOfflineGroupLabel(ctx context.Context, pod *corev1.Pod) (bool, error) {
	if pod == nil {
		return false, nil
	}
	if !isOfflineWorkload(pod) {
		return false, nil
	}
	nodeName := strings.TrimSpace(pod.Spec.NodeName)
	if nodeName == "" {
		return false, fmt.Errorf("pod %s/%s node name must not be empty", pod.Namespace, pod.Name)
	}

	desiredGroup := dynamicOfflineGroupName(nodeName)
	if pod.Labels != nil && pod.Labels[PodQoSGroupLabelKey] == desiredGroup {
		return false, nil
	}

	if pod.Labels == nil {
		pod.Labels = map[string]string{}
	}
	pod.Labels[PodQoSGroupLabelKey] = desiredGroup
	if err := r.Update(ctx, pod); err != nil {
		return false, err
	}
	return true, nil
}

func dynamicOfflineGroupName(nodeName string) string {
	name := strings.ToLower(strings.TrimSpace(nodeName))
	name = strings.ReplaceAll(name, ".", "-")
	name = strings.ReplaceAll(name, "_", "-")
	if name == "" {
		name = "unknown-node"
	}
	return "qos-dynamic-offline-" + name
}

func clientKeyForPolicy(name string) types.NamespacedName {
	// Policy is cluster-scoped, only name is used.
	return types.NamespacedName{Name: name}
}
