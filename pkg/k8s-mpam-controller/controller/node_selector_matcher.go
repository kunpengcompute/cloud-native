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

// DefaultNodeSelectorMatcher matches node labels with Kubernetes nodeSelector semantics:
// all selector entries must match exactly, and an empty selector matches all nodes.
type DefaultNodeSelectorMatcher struct{}

// Match returns true when selector is empty or all selector key/value pairs are present
// and equal in nodeLabels.
func (m DefaultNodeSelectorMatcher) Match(nodeLabels map[string]string, selector map[string]string) bool {
	if len(selector) == 0 {
		return true
	}

	if len(nodeLabels) == 0 {
		return false
	}

	for key, expected := range selector {
		actual, found := nodeLabels[key]
		if !found || actual != expected {
			return false
		}
	}

	return true
}
