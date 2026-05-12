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
	"testing"

	"kunpeng.huawei.com/kunpeng-cloud-computing/pkg/kunpeng-qos-controller/util"
)

func TestMatchNodeSelector(t *testing.T) {
	tests := []struct {
		name       string
		nodeLabels map[string]string
		selector   map[string]string
		want       bool
	}{
		{
			name:       "empty selector matches all",
			nodeLabels: map[string]string{"kubernetes.io/hostname": "node-a"},
			selector:   map[string]string{},
			want:       true,
		},
		{
			name:       "single key matches",
			nodeLabels: map[string]string{"qos.kunpeng.huawei.com/enabled": "true"},
			selector:   map[string]string{"qos.kunpeng.huawei.com/enabled": "true"},
			want:       true,
		},
		{
			name:       "single key mismatch",
			nodeLabels: map[string]string{"qos.kunpeng.huawei.com/enabled": "false"},
			selector:   map[string]string{"qos.kunpeng.huawei.com/enabled": "true"},
			want:       false,
		},
		{
			name: "multi keys all match",
			nodeLabels: map[string]string{
				"qos.kunpeng.huawei.com/enabled": "true",
				"kubernetes.io/hostname":         "node-a",
				"topology.kubernetes.io/zone":    "zone-a",
			},
			selector: map[string]string{
				"qos.kunpeng.huawei.com/enabled": "true",
				"topology.kubernetes.io/zone":    "zone-a",
			},
			want: true,
		},
		{
			name: "multi keys one mismatch",
			nodeLabels: map[string]string{
				"qos.kunpeng.huawei.com/enabled": "true",
				"kubernetes.io/hostname":         "node-a",
				"topology.kubernetes.io/zone":    "zone-b",
			},
			selector: map[string]string{
				"qos.kunpeng.huawei.com/enabled": "true",
				"topology.kubernetes.io/zone":    "zone-a",
			},
			want: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			got := util.MatchNodeSelector(tt.nodeLabels, tt.selector)
			if got != tt.want {
				t.Fatalf("MatchNodeSelector() = %v, want %v", got, tt.want)
			}
		})
	}
}
