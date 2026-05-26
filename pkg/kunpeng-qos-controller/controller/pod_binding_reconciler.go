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
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	qosv1alpha1 "kunpeng.huawei.com/kunpeng-cloud-computing/api/kunpeng-qos-controller/v1alpha1"
	"kunpeng.huawei.com/kunpeng-cloud-computing/pkg/kunpeng-qos-controller/util"
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
	MPAMBinder           PodProcessBinder
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
		MPAMBinder:           MPAMPodBinder{},
		CPUQoSSetter:         MPAMPodBinder{},
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
	return MPAMPodBinder{}
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
		For(
			&corev1.Pod{},
			builder.WithPredicates(predicate.NewPredicateFuncs(func(obj client.Object) bool {
				pod, ok := obj.(*corev1.Pod)
				if !ok {
					return false
				}
				return r.shouldProcessPod(pod)
			})),
		).
		Watches(
			&qosv1alpha1.QoSPolicy{},
			handler.EnqueueRequestsFromMapFunc(r.mapQoSPolicyToPods),
			builder.WithPredicates(predicate.Funcs{
				CreateFunc: func(_ event.CreateEvent) bool {
					return true
				},
				UpdateFunc: func(e event.UpdateEvent) bool {
					oldObj, oldOK := e.ObjectOld.(*qosv1alpha1.QoSPolicy)
					newObj, newOK := e.ObjectNew.(*qosv1alpha1.QoSPolicy)
					if !oldOK || !newOK || oldObj == nil || newObj == nil {
						return false
					}
					// Only react to cpu qos-level changes on policy update.
					return oldObj.Spec.CPU.QoSLevel != newObj.Spec.CPU.QoSLevel
				},
				DeleteFunc: func(_ event.DeleteEvent) bool {
					return false
				},
				GenericFunc: func(_ event.GenericEvent) bool {
					return false
				},
			}),
		).
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

func isdynamicWorkload(pod *corev1.Pod) bool {
	if pod == nil || len(pod.Labels) == 0 {
		return false
	}
	return pod.Labels[WorkloadClassLabelKey] == WorkloadClassOffline
}

func (r *PodBindingReconciler) ensureOfflineGroupLabel(ctx context.Context, pod *corev1.Pod) (bool, error) {
	if pod == nil {
		return false, nil
	}
	if !isdynamicWorkload(pod) {
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

func (r *PodBindingReconciler) mapQoSPolicyToPods(ctx context.Context, obj client.Object) []reconcile.Request {
	policy, ok := obj.(*qosv1alpha1.QoSPolicy)
	if !ok || policy == nil || policy.Name == "" {
		klog.V(4).Infof("skip QoSPolicy-to-Pod mapping for invalid object: %#v", obj)
		return nil
	}
	klog.V(2).Infof(
		"map QoSPolicy %s to pods on node %s, selector=%v",
		policy.Name, r.nodeIdentity().NodeName(), policy.Spec.NodeSelector,
	)
	nodeLabels, err := r.nodeIdentity().NodeLabels(ctx, r.Client)
	if err != nil {
		klog.Warningf("get labels for node %s failed when handling QoSPolicy %s: %v", r.nodeIdentity().NodeName(), policy.Name, err)
		return nil
	}
	if !util.MatchNodeSelector(nodeLabels, policy.Spec.NodeSelector) {
		klog.V(2).Infof(
			"skip QoSPolicy %s mapping on node %s: nodeSelector mismatch, nodeLabels=%v selector=%v",
			policy.Name, r.nodeIdentity().NodeName(), nodeLabels, policy.Spec.NodeSelector,
		)
		return nil
	}

	var pods corev1.PodList
	if err := r.List(ctx, &pods, client.MatchingLabels{PodQoSGroupLabelKey: policy.Name}); err != nil {
		klog.Warningf("list pods for QoSPolicy %s failed: %v", policy.Name, err)
		return nil
	}
	klog.V(2).Infof("listed %d pods with label %s=%s", len(pods.Items), PodQoSGroupLabelKey, policy.Name)

	requests := make([]reconcile.Request, 0, len(pods.Items))
	for i := range pods.Items {
		pod := &pods.Items[i]
		if !r.shouldProcessPod(pod) {
			klog.V(4).Infof(
				"skip pod %s/%s for QoSPolicy %s mapping: shouldProcessPod=false (node=%s phase=%s labels=%v)",
				pod.Namespace, pod.Name, policy.Name, pod.Spec.NodeName, pod.Status.Phase, pod.Labels,
			)
			continue
		}
		klog.V(3).Infof("enqueue pod %s/%s for QoSPolicy %s update", pod.Namespace, pod.Name, policy.Name)
		requests = append(requests, reconcile.Request{
			NamespacedName: types.NamespacedName{
				Namespace: pod.Namespace,
				Name:      pod.Name,
			},
		})
	}
	klog.V(2).Infof("mapped QoSPolicy %s to %d pod reconcile requests", policy.Name, len(requests))
	return requests
}

func clientKeyForPolicy(name string) types.NamespacedName {
	// Policy is cluster-scoped, only name is used.
	return types.NamespacedName{Name: name}
}
