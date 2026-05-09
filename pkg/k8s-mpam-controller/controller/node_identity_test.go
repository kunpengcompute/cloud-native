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
	"os"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestResolveNodeName(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		want     string
	}{
		{
			name:     "prefer env value",
			envValue: "Node-A",
			want:     "node-a",
		},
		{
			name:     "empty env returns empty",
			envValue: " ",
			want:     "",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			got := resolveNodeName(tt.envValue)
			if got != tt.want {
				t.Fatalf("resolveNodeName() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDefaultNodeIdentityNodeNameFromEnv(t *testing.T) {
	oldVal, had := os.LookupEnv(nodeNameEnvKey)
	defer func() {
		if had {
			_ = os.Setenv(nodeNameEnvKey, oldVal)
			return
		}
		_ = os.Unsetenv(nodeNameEnvKey)
	}()

	_ = os.Setenv(nodeNameEnvKey, "NODE-X")
	identity := NewDefaultNodeIdentity()
	if identity.NodeName() != "node-x" {
		t.Fatalf("NodeName() = %q, want %q", identity.NodeName(), "node-x")
	}
}

func TestDefaultNodeIdentityNodeNameEmptyWhenEnvUnset(t *testing.T) {
	oldVal, had := os.LookupEnv(nodeNameEnvKey)
	defer func() {
		if had {
			_ = os.Setenv(nodeNameEnvKey, oldVal)
			return
		}
		_ = os.Unsetenv(nodeNameEnvKey)
	}()

	_ = os.Unsetenv(nodeNameEnvKey)
	identity := NewDefaultNodeIdentity()
	if identity.NodeName() != "" {
		t.Fatalf("NodeName() = %q, want empty", identity.NodeName())
	}
}

func TestDefaultNodeIdentityNodeLabels(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme(corev1) failed: %v", err)
	}

	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "node-a",
			Labels: map[string]string{
				"mpam.huawei.com/enabled":     "true",
				"kubernetes.io/hostname":      "node-a",
				"topology.kubernetes.io/zone": "zone-a",
			},
		},
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(node).Build()

	identity := DefaultNodeIdentity{nodeName: "node-a"}
	labels, err := identity.NodeLabels(context.Background(), cl)
	if err != nil {
		t.Fatalf("NodeLabels() unexpected error: %v", err)
	}
	if labels["mpam.huawei.com/enabled"] != "true" {
		t.Fatalf("labels mismatch, got: %#v", labels)
	}
}

func TestDefaultNodeIdentityNodeLabelsWithEmptyName(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme(corev1) failed: %v", err)
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).Build()

	identity := DefaultNodeIdentity{}
	if _, err := identity.NodeLabels(context.Background(), cl); err == nil {
		t.Fatalf("NodeLabels() expected error when node name is empty")
	}
}
