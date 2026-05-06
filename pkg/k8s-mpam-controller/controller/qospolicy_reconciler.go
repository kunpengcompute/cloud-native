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

	mpamv1alpha1 "kunpeng.huawei.com/kunpeng-cloud-computing/pkg/k8s-mpam-controller/api/v1alpha1"
)

const defaultQoSPolicyFinalizer = "mpam.kunpeng.huawei.com/finalizer"

// ResctrlConfig is the normalized config written to /sys/fs/resctrl by node agent.
type ResctrlConfig struct {
	MBHDL int32
	MBPRI int32
	L3PRI int32
	MBMIN int32
	L3MIN int32
	L3MAX int32
	MB    int32
	L3    int32 // Number of cache ways.
}

// NodeIdentity abstracts current DaemonSet instance's node identity and labels.
type NodeIdentity interface {
	NodeName() string
	NodeLabels(ctx context.Context, c client.Client) (map[string]string, error)
}

// NodeSelectorMatcher decides whether a policy should be applied on this node.
type NodeSelectorMatcher interface {
	Match(nodeLabels map[string]string, selector map[string]string) bool
}

// PolicyTranslator converts CRD spec into low-level resctrl configuration.
type PolicyTranslator interface {
	Translate(spec mpamv1alpha1.QoSPolicySpec) (ResctrlConfig, error)
}

// ResctrlGroupManager performs local /sys/fs/resctrl operations.
type ResctrlGroupManager interface {
	EnsureGroup(ctx context.Context, groupName string) error
	ApplyConfig(ctx context.Context, groupName string, cfg ResctrlConfig) error
	DeleteGroup(ctx context.Context, groupName string) error
}

// QoSPolicyReconciler reconciles QoSPolicy CRs on a single node (DaemonSet instance).
type QoSPolicyReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	NodeIdentity NodeIdentity
	Matcher      NodeSelectorMatcher
	Translator   PolicyTranslator
	Resctrl      ResctrlGroupManager

	FinalizerName string
}

// Reconcile handles create/update/delete of QoSPolicy for current node only.
func (r *QoSPolicyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	klog.V(4).Infof("reconcile QoSPolicy %s/%s on node %s", req.Namespace, req.Name, r.nodeIdentity().NodeName())

	var policy mpamv1alpha1.QoSPolicy
	if err := r.Get(ctx, req.NamespacedName, &policy); err != nil {
		if apierrors.IsNotFound(err) {
			// Object already deleted from API server. Nothing to do here because
			// local cleanup should be handled via finalizer path before actual removal.
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	finalizer := r.finalizerName()
	groupName := groupNameForPolicy(&policy)

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
	if !r.matcher().Match(nodeLabels, policy.Spec.NodeSelector) {
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

	cfg, err := r.translator().Translate(policy.Spec)
	if err != nil {
		return ctrl.Result{}, err
	}

	if err := r.resctrl().EnsureGroup(ctx, groupName); err != nil {
		return ctrl.Result{}, err
	}
	klog.V(1).Infof("ensured resctrl group for QoSPolicy %s/%s: group=%s", policy.Namespace, policy.Name, groupName)

	if err := r.resctrl().ApplyConfig(ctx, groupName, cfg); err != nil {
		return ctrl.Result{}, err
	}
	klog.V(1).Infof("applied resctrl config for QoSPolicy %s/%s: group=%s", policy.Namespace, policy.Name, groupName)

	// NOTE: status updates are intentionally skipped in daemonset mode to avoid
	// multi-writer conflicts on one shared CR status from all nodes.
	return ctrl.Result{}, nil
}

func (r *QoSPolicyReconciler) finalizerName() string {
	if r.FinalizerName != "" {
		return r.FinalizerName
	}
	return defaultQoSPolicyFinalizer
}

func (r *QoSPolicyReconciler) setDefaults() {
	if r.NodeIdentity == nil {
		r.NodeIdentity = NewDefaultNodeIdentity()
	}
	if r.Matcher == nil {
		r.Matcher = DefaultNodeSelectorMatcher{}
	}
	if r.Translator == nil {
		r.Translator = DefaultPolicyTranslator{}
	}
	if r.Resctrl == nil {
		r.Resctrl = LocalResctrlGroupManager{}
	}
}

func (r *QoSPolicyReconciler) validate() error {
	if r.Client == nil {
		return fmt.Errorf("client must not be nil")
	}
	return nil
}

func (r *QoSPolicyReconciler) matcher() NodeSelectorMatcher {
	if r.Matcher != nil {
		return r.Matcher
	}
	return DefaultNodeSelectorMatcher{}
}

func (r *QoSPolicyReconciler) translator() PolicyTranslator {
	if r.Translator != nil {
		return r.Translator
	}
	return DefaultPolicyTranslator{}
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

func groupNameForPolicy(policy *mpamv1alpha1.QoSPolicy) string {
	// Keep the first version simple: map CR name to resctrl group name directly.
	// A future version can add sanitization/truncation if needed.
	return policy.Name
}

// SetupWithManager registers controller with a focused event filter.
func (r *QoSPolicyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.setDefaults()
	if err := r.validate(); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&mpamv1alpha1.QoSPolicy{}).
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
