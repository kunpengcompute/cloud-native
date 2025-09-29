/*
 * Copyright 2022 The Koordinator Authors.
 * Modifications Copyright 2025 Huawei Technology corp.
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

package dispatcher

import (
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1"

	"kunpeng.huawei.com/kunpeng-cloud-computing/api/kunpeng-tap/policy-manager/v1alpha1"
	"kunpeng.huawei.com/kunpeng-cloud-computing/cmd/kunpeng-tap/proxy/options"
	"kunpeng.huawei.com/kunpeng-cloud-computing/pkg/kunpeng-tap/policy"
	"kunpeng.huawei.com/kunpeng-cloud-computing/pkg/kunpeng-tap/server"
)

func TransferToCRIContainerEnvs(envs map[string]string) []*runtimeapi.KeyValue {
	var res []*runtimeapi.KeyValue
	if envs == nil {
		return res
	}
	for key, val := range envs {
		res = append(res, &runtimeapi.KeyValue{
			Key:   key,
			Value: val,
		})
	}
	return res
}
func TransferCRIContainerEnvsToMap(envs []*runtimeapi.KeyValue) map[string]string {
	res := make(map[string]string)
	if envs == nil {
		return res
	}
	for _, item := range envs {
		res[item.GetKey()] = item.GetValue()
	}
	return res
}

func TransferToCRIResources(r *v1alpha1.LinuxContainerResources) *runtimeapi.LinuxContainerResources {
	if r == nil {
		return nil
	}

	linuxResources := &runtimeapi.LinuxContainerResources{
		CpuPeriod:              r.GetCpuPeriod(),
		CpuQuota:               r.GetCpuQuota(),
		CpuShares:              r.GetCpuShares(),
		MemoryLimitInBytes:     r.GetMemoryLimitInBytes(),
		OomScoreAdj:            r.GetOomScoreAdj(),
		CpusetCpus:             r.GetCpusetCpus(),
		CpusetMems:             r.GetCpusetMems(),
		Unified:                r.GetUnified(),
		MemorySwapLimitInBytes: r.GetMemorySwapLimitInBytes(),
	}

	for _, item := range r.GetHugepageLimits() {
		linuxResources.HugepageLimits = append(linuxResources.HugepageLimits, &runtimeapi.HugepageLimit{
			PageSize: item.GetPageSize(),
			Limit:    item.GetLimit(),
		})
	}
	return linuxResources
}

func TransferToHookResources(r *runtimeapi.LinuxContainerResources) *v1alpha1.LinuxContainerResources {
	if r == nil {
		return nil
	}

	linuxResources := &v1alpha1.LinuxContainerResources{
		CpuPeriod:              r.GetCpuPeriod(),
		CpuQuota:               r.GetCpuQuota(),
		CpuShares:              r.GetCpuShares(),
		MemoryLimitInBytes:     r.GetMemoryLimitInBytes(),
		OomScoreAdj:            r.GetOomScoreAdj(),
		CpusetCpus:             r.GetCpusetCpus(),
		CpusetMems:             r.GetCpusetMems(),
		Unified:                r.GetUnified(),
		MemorySwapLimitInBytes: r.GetMemorySwapLimitInBytes(),
	}

	for _, item := range r.GetHugepageLimits() {
		linuxResources.HugepageLimits = append(linuxResources.HugepageLimits, &v1alpha1.HugepageLimit{
			PageSize: item.GetPageSize(),
			Limit:    item.GetLimit(),
		})
	}
	return linuxResources
}

func GetResourceTypeFromHookType(hookType policy.HookType) server.RuntimeResourceType {
	switch hookType {
	case policy.PreRunPodSandbox, policy.PostStopPodSandbox, policy.PreRemovePodSandbox:
		return server.RuntimePodResource
	case policy.PreCreateContainer, policy.PreStartContainer, policy.PostStartContainer,
		policy.PostStopContainer, policy.PreRemoveContainer, policy.PreUpdateContainerResources:
		return server.RuntimeContainerResource
	default:
		return server.RuntimeNoopResource
	}
}

func SkipTAPHandle(labels map[string]string) bool {
	if _, ok := labels[options.SkipTAPLabel]; ok {
		return true
	}
	return false
}
