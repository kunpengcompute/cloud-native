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

package cache

import (
	"reflect"
	"testing"

	"github.com/docker/docker/client"
	corev1 "k8s.io/api/core/v1"
	criv1 "k8s.io/cri-api/pkg/apis/runtime/v1"
	"kunpeng.huawei.com/kunpeng-cloud-computing/api/kunpeng-tap/policy-manager/v1alpha1"
)

func TestSharesToMilliCPU(t *testing.T) {
	type args struct {
		shares int64
	}
	tests := []struct {
		name string
		args args
		want int64
	}{
		{name: "1", args: args{shares: 64}, want: 63},
		{name: "2", args: args{shares: 0}, want: 0},
		{name: "3", args: args{shares: 50}, want: 49},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SharesToMilliCPU(tt.args.shares); got != tt.want {
				t.Errorf("SharesToMilliCPU() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestQuotaToMilliCPU(t *testing.T) {
	type args struct {
		quota  int64
		period int64
	}
	tests := []struct {
		name string
		args args
		want int64
	}{
		{name: "1", args: args{quota: 64, period: 100}, want: 640},
		{name: "2", args: args{quota: 0, period: 100}, want: 0},
		{name: "3", args: args{quota: 50, period: 100}, want: 500},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := QuotaToMilliCPU(tt.args.quota, tt.args.period); got != tt.want {
				t.Errorf("QuotaToMilliCPU() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCgroupParentToQoS(t *testing.T) {
	type args struct {
		dir string
	}
	tests := []struct {
		name string
		args args
		want corev1.PodQOSClass
	}{
		{name: "1", args: args{dir: ""}, want: corev1.PodQOSClass("")},
		{name: "2", args: args{dir: "/home/test"}, want: corev1.PodQOSClass("Guaranteed")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := cgroupParentToQoS(tt.args.dir); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("cgroupParentToQoS() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEstimateRequirements(t *testing.T) {
	type args struct {
		res          *v1alpha1.LinuxContainerResources
		cgroupParent string
	}
	tests := []struct {
		name string
		args args
		want *corev1.ResourceRequirements
	}{
		{name: "1", args: args{res: &v1alpha1.LinuxContainerResources{}, cgroupParent: ""}, want: &corev1.ResourceRequirements{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := EstimateRequirements(tt.args.res, tt.args.cgroupParent); got == nil {
				t.Errorf("EstimateRequirements() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEstimateComputeResources(t *testing.T) {
	type args struct {
		lnx *criv1.LinuxContainerResources
	}
	tests := []struct {
		name string
		args args
		want corev1.ResourceRequirements
	}{
		{name: "1", args: args{lnx: &criv1.LinuxContainerResources{}}, want: corev1.ResourceRequirements{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := estimateComputeResources(tt.args.lnx); len(got.Limits) != 0 {
				t.Errorf("estimateComputeResources() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCacheLoadStoreDocker(t *testing.T) {
	type args struct {
		dockerClient client.CommonAPIClient
		cgroupDriver string
	}
	tests := []struct {
		name    string
		cch     *cache
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.cch.LoadStoreDocker(tt.args.dockerClient, tt.args.cgroupDriver); (err != nil) != tt.wantErr {
				t.Errorf("cache.LoadStoreDocker() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
