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

package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// MPAMPolicySpec defines the desired state for MPAM settings.
type MPAMPolicySpec struct {
	// MB groups memory-bandwidth related policy items.
	// +kubebuilder:default:={}
	// +optional
	MB MBPolicy `json:"mb,omitempty"`

	// L3 groups cache related policy items.
	// +kubebuilder:default:={}
	// +optional
	L3 L3Policy `json:"l3,omitempty"`

	// NodeSelector selects which nodes this policy applies to.
	// If omitted, the controller should decide default behavior (typically all nodes).
	// +optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`
}

// MBPolicy defines memory-bandwidth related controls.
type MBPolicy struct {
	// HDL can only be 0 or 1. Default is 1.
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=1
	// +kubebuilder:default:=1
	// +optional
	HDL int32 `json:"hdl,omitempty"`

	// PRI range is [0, 7]. Default is 3.
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=7
	// +kubebuilder:default:=3
	// +optional
	PRI int32 `json:"pri,omitempty"`

	// MIN range is [0, 100]. Default is 0.
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=100
	// +kubebuilder:default:=0
	// +optional
	MIN int32 `json:"min,omitempty"`

	// MAX range is [0, 100]. Default is 100.
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=100
	// +kubebuilder:default:=100
	// +optional
	MAX int32 `json:"max,omitempty"`
}

// L3Policy defines cache related controls.
type L3Policy struct {
	// PRI range is [0, 3]. Default is 0.
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=3
	// +kubebuilder:default:=0
	// +optional
	PRI int32 `json:"pri,omitempty"`

	// MIN range is [0, 100]. Default is 0.
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=100
	// +kubebuilder:default:=0
	// +optional
	MIN int32 `json:"min,omitempty"`

	// MAX range is [0, 100]. Default is 100.
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=100
	// +kubebuilder:default:=100
	// +optional
	MAX int32 `json:"max,omitempty"`

	// Ways is the number of cache ways to allocate. Minimum is 1.
	// Machine-specific upper bound should be checked by the controller on each node.
	// +kubebuilder:validation:Minimum=1
	// +optional
	Ways int32 `json:"ways,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:path=mpampolicies,scope=Cluster,shortName=mpam
// +kubebuilder:printcolumn:name="MBPRI",type="integer",JSONPath=".spec.mb.pri"
// +kubebuilder:printcolumn:name="L3WAYS",type="integer",JSONPath=".spec.l3.ways"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"

// MPAMPolicy is the Schema for MPAM policy CRD.
type MPAMPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec MPAMPolicySpec `json:"spec,omitempty"`
}

// +kubebuilder:object:root=true

// MPAMPolicyList contains a list of MPAMPolicy.
type MPAMPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MPAMPolicy `json:"items"`
}
