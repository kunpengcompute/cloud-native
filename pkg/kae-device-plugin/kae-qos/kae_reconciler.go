/*
 * Copyright (c) 2025 Huawei Technology corp.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package kaeqos

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

const (
	finalizerName = "kae.kunpeng.com/finalizer"
)

type KaeQosReconciler struct {
	QosManager *QosManager
	client.Client
}

func (r *KaeQosReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	pod := &corev1.Pod{}
	if err := r.Get(ctx, req.NamespacedName, pod); err != nil {
		if apierrors.IsNotFound(err) {
			// Pod has already been deleted from the cluster
			return ctrl.Result{}, nil
		}
		// Other errors (e.g. network, API server issues)
		return ctrl.Result{}, err
	}

	if pod.DeletionTimestamp.IsZero() {
		// Ensure the Pod has a finalizer to perform cleanup before deletion
		if !controllerutil.ContainsFinalizer(pod, finalizerName) {
			controllerutil.AddFinalizer(pod, finalizerName)
			if err := r.Update(ctx, pod); err != nil {
				return ctrl.Result{}, err
			}

			return ctrl.Result{}, nil
		}

		// Get pod resources
		if err := r.QosManager.client.fetchPodResources(pod.Namespace, pod.Name); err != nil {
			return ctrl.Result{}, fmt.Errorf("get pod : %s resources failed: %v", req.NamespacedName, err)
		}

		// Call QosManager to update Pod's device QoS
		if err := r.QosManager.updateQos(pod); err != nil {
			return ctrl.Result{}, fmt.Errorf("update pod : %s qos failed: %v", req.NamespacedName, err)
		}

		return ctrl.Result{}, nil
	}

	if controllerutil.ContainsFinalizer(pod, finalizerName) {
		// Get pod resources
		if err := r.QosManager.client.fetchPodResources(pod.Namespace, pod.Name); err != nil {
			return ctrl.Result{}, fmt.Errorf("get pod : %s resources failed: %v", req.NamespacedName, err)
		}

		// Execute recovery logic, restore device qos
		if err := r.QosManager.restoreQos(pod); err != nil {
			return ctrl.Result{}, fmt.Errorf("restore pod : %s qos failed: %v", req.NamespacedName, err)
		}

		// Remove finalizer to allow Kubernetes to actually delete the Pod
		controllerutil.RemoveFinalizer(pod, finalizerName)
		if err := r.Update(ctx, pod); err != nil {
			return ctrl.Result{}, err
		}

	}

	return ctrl.Result{}, nil
}

func (r *KaeQosReconciler) SetupWithManager(mgr ctrl.Manager, nodeName string) error {
	// Only reconcile pods running on the specified node
	nodePredicate := predicate.NewPredicateFuncs(func(obj client.Object) bool {
		pod, ok := obj.(*corev1.Pod)
		if !ok {
			return false
		}

		if pod.Spec.NodeName != nodeName || pod.Status.Phase != corev1.PodRunning {
			return false
		}

		return isKAEDeviceRequested(pod)
	})

	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Pod{}).
		WithEventFilter(nodePredicate).
		Complete(r)
}

func isKAEDeviceRequested(pod *corev1.Pod) bool {
	for _, container := range pod.Spec.Containers {
		for resourceName := range container.Resources.Limits {
			if isSupportDevice(string(resourceName)) {
				return true
			}
		}
	}

	return false
}
