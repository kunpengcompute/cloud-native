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
	"os"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const nodeNameEnvKey = "NODE_NAME"

// DefaultNodeIdentity is a Kubernetes-backed implementation of NodeIdentity.
type DefaultNodeIdentity struct {
	nodeName string
}

// NewDefaultNodeIdentity resolves current node name from NODE_NAME only.
func NewDefaultNodeIdentity() DefaultNodeIdentity {
	return DefaultNodeIdentity{
		nodeName: resolveNodeName(os.Getenv(nodeNameEnvKey)),
	}
}

// NodeName returns current node name.
func (n DefaultNodeIdentity) NodeName() string {
	return n.nodeName
}

// NodeLabels fetches current node labels from apiserver.
func (n DefaultNodeIdentity) NodeLabels(ctx context.Context, c client.Client) (map[string]string, error) {
	if c == nil {
		return nil, fmt.Errorf("client must not be nil")
	}
	if n.nodeName == "" {
		return nil, fmt.Errorf("node name is empty")
	}

	var node corev1.Node
	if err := c.Get(ctx, types.NamespacedName{Name: n.nodeName}, &node); err != nil {
		return nil, err
	}

	labels := make(map[string]string, len(node.Labels))
	for k, v := range node.Labels {
		labels[k] = v
	}
	return labels, nil
}

func resolveNodeName(envNodeName string) string {
	return strings.ToLower(strings.TrimSpace(envNodeName))
}
