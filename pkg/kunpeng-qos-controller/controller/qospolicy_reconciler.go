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

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	qosv1alpha1 "kunpeng.huawei.com/kunpeng-cloud-computing/api/kunpeng-qos-controller/v1alpha1"
	"kunpeng.huawei.com/kunpeng-cloud-computing/pkg/kunpeng-qos-controller/util"
)

const defaultQoSPolicyFinalizer = "qos.kunpeng.huawei.com/finalizer"

// NodeIdentity abstracts current DaemonSet instance's node identity and labels.
type NodeIdentity interface {
	NodeName() string
	NodeLabels(ctx context.Context, c client.Client) (map[string]string, error)
}

// QoSPolicyReconciler reconciles QoSPolicy CRs on a single node (DaemonSet instance).
type QoSPolicyReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	NodeIdentity NodeIdentity
	Resctrl      ResctrlGroupManager
}

// Reconcile handles create/update/delete of QoSPolicy for current node only.
func (r *QoSPolicyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	klog.V(4).Infof("reconcile QoSPolicy %s/%s on node %s", req.Namespace, req.Name, r.nodeIdentity().NodeName())

	var policy qosv1alpha1.QoSPolicy
	if err := r.Get(ctx, req.NamespacedName, &policy); err != nil {
		if apierrors.IsNotFound(err) {
			// Object already deleted from API server. Nothing to do here because
			// local cleanup should be handled via finalizer path before actual removal.
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	finalizer := defaultQoSPolicyFinalizer
	groupName := policy.Name

	// Deletion path: remove local resctrl group first, then release finalizer.
	if !policy.DeletionTimestamp.IsZero() {
		// Keep V1 simple: always try local cleanup. DeleteGroup should be idempotent
		// and treat "group not found" as success.
		klog.V(1).Infof("deleting resctrl group for QoSPolicy %s/%s: group=%s", policy.Namespace, policy.Name, groupName)
		if err := r.resctrl().DeleteGroup(ctx, groupName); err != nil {
			return ctrl.Result{}, err
		}

		if controllerutil.ContainsFinalizer(&policy, finalizer) {
			controllerutil.RemoveFinalizer(&policy, finalizer)
			if err := r.Update(ctx, &policy); err != nil {
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	// Ensure finalizer on normal reconcile so delete can trigger local cleanup.
	if !controllerutil.ContainsFinalizer(&policy, finalizer) {
		controllerutil.AddFinalizer(&policy, finalizer)
		if err := r.Update(ctx, &policy); err != nil {
			return ctrl.Result{}, err
		}
	}

	nodeLabels, err := r.nodeIdentity().NodeLabels(ctx, r.Client)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("get labels for node %q: %w", r.nodeIdentity().NodeName(), err)
	}

	// DaemonSet model: this reconciler applies only when current node matches selector.
	if !util.MatchNodeSelector(nodeLabels, policy.Spec.NodeSelector) {
		// Policy no longer targets this node. Best-effort local cleanup.
		klog.V(4).Infof(
			"QoSPolicy %s/%s does not match node %s, cleanup local group=%s",
			policy.Namespace, policy.Name, r.nodeIdentity().NodeName(), groupName,
		)
		if err := r.resctrl().DeleteGroup(ctx, groupName); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	if err := r.resctrl().EnsureGroup(ctx, groupName); err != nil {
		return ctrl.Result{}, err
	}
	klog.V(1).Infof("ensured resctrl group for QoSPolicy %s/%s: group=%s", policy.Namespace, policy.Name, groupName)

	cfg := translateQoSPolicySpec(policy.Spec)
	if err := r.resctrl().ApplyConfig(ctx, groupName, cfg); err != nil {
		return ctrl.Result{}, err
	}
	klog.V(1).Infof("applied resctrl config for QoSPolicy %s/%s: group=%s", policy.Namespace, policy.Name, groupName)

	// NOTE: status updates are intentionally skipped in daemonset mode to avoid
	// multi-writer conflicts on one shared CR status from all nodes.
	return ctrl.Result{}, nil
}

func (r *QoSPolicyReconciler) nodeIdentity() NodeIdentity {
	if r.NodeIdentity != nil {
		return r.NodeIdentity
	}
	return NewDefaultNodeIdentity()
}

func (r *QoSPolicyReconciler) resctrl() ResctrlGroupManager {
	if r.Resctrl != nil {
		return r.Resctrl
	}
	return LocalResctrlGroupManager{}
}

func translateQoSPolicySpec(spec qosv1alpha1.QoSPolicySpec) ResctrlConfig {
	return ResctrlConfig{
		MBHDL: spec.MB.HDL,
		MBPRI: spec.MB.PRI,
		L3PRI: spec.L3.PRI,
		MBMIN: spec.MB.MIN,
		L3MIN: spec.L3.MIN,
		L3MAX: spec.L3.MAX,
		MB:    spec.MB.MAX,
		L3:    spec.L3.Ways,
	}
}

// SetupWithManager registers controller with a focused event filter.
func (r *QoSPolicyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&qosv1alpha1.QoSPolicy{}).
		WithEventFilter(predicate.Funcs{
			CreateFunc: func(_ event.CreateEvent) bool {
				return true
			},
			UpdateFunc: func(e event.UpdateEvent) bool {
				if e.ObjectOld == nil || e.ObjectNew == nil {
					return false
				}
				// Reconcile when spec changes.
				if e.ObjectOld.GetGeneration() != e.ObjectNew.GetGeneration() {
					return true
				}
				// Reconcile exactly once when object enters deleting state.
				// This covers finalizer cleanup while filtering metadata-only updates
				// such as finalizer add/remove.
				oldDeleting := e.ObjectOld.GetDeletionTimestamp() != nil &&
					!e.ObjectOld.GetDeletionTimestamp().IsZero()
				newDeleting := e.ObjectNew.GetDeletionTimestamp() != nil &&
					!e.ObjectNew.GetDeletionTimestamp().IsZero()
				return !oldDeleting && newDeleting
			},
			DeleteFunc: func(_ event.DeleteEvent) bool {
				// Finalizer removal already runs cleanup on deletionTimestamp update.
				// Skip terminal delete event to avoid duplicate reconcile.
				return false
			},
			GenericFunc: func(_ event.GenericEvent) bool {
				return false
			},
		}).
		Complete(r)
}
