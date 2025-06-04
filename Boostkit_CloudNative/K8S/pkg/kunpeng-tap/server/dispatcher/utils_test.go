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

package dispatcher_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1"
	"kunpeng.huawei.com/kunpeng-cloud-computing/api/kunpeng-tap/policy-manager/v1alpha1"
	"kunpeng.huawei.com/kunpeng-cloud-computing/cmd/kunpeng-tap/proxy/options"
	"kunpeng.huawei.com/kunpeng-cloud-computing/pkg/kunpeng-tap/policy"
	"kunpeng.huawei.com/kunpeng-cloud-computing/pkg/kunpeng-tap/server"
	"kunpeng.huawei.com/kunpeng-cloud-computing/pkg/kunpeng-tap/server/dispatcher"
)

var _ = Describe("Utils", func() {
	Describe("TransferToCRIContainerEnvs", func() {
		It("should convert map to []*runtimeapi.KeyValue", func() {
			envs := map[string]string{"key1": "value1", "key2": "value2"}
			result := dispatcher.TransferToCRIContainerEnvs(envs)

			Expect(result).To(HaveLen(2))
			Expect(result[0].Key).To(Equal("key1"))
			Expect(result[0].Value).To(Equal("value1"))
			Expect(result[1].Key).To(Equal("key2"))
			Expect(result[1].Value).To(Equal("value2"))
		})

		It("should return an empty slice if input is nil", func() {
			result := dispatcher.TransferToCRIContainerEnvs(nil)
			Expect(result).To(BeEmpty())
		})

		It("should handle an empty map", func() {
			envs := map[string]string{}
			result := dispatcher.TransferToCRIContainerEnvs(envs)
			Expect(result).To(BeEmpty())
		})
	})

	Describe("TransferCRIContainerEnvsToMap", func() {
		It("should convert []*runtimeapi.KeyValue to map", func() {
			envs := []*runtimeapi.KeyValue{
				{Key: "key1", Value: "value1"},
				{Key: "key2", Value: "value2"},
			}
			result := dispatcher.TransferCRIContainerEnvsToMap(envs)

			Expect(result).To(HaveLen(2))
			Expect(result["key1"]).To(Equal("value1"))
			Expect(result["key2"]).To(Equal("value2"))
		})

		It("should return an empty map if input is nil", func() {
			result := dispatcher.TransferCRIContainerEnvsToMap(nil)
			Expect(result).To(BeEmpty())
		})

		It("should handle an empty slice", func() {
			envs := []*runtimeapi.KeyValue{}
			result := dispatcher.TransferCRIContainerEnvsToMap(envs)
			Expect(result).To(BeEmpty())
		})
	})

	Describe("TransferToCRIResources", func() {
		It("should convert v1alpha1.LinuxContainerResources to runtimeapi.LinuxContainerResources", func() {
			input := &v1alpha1.LinuxContainerResources{
				CpuPeriod:          1000,
				CpuQuota:           2000,
				CpuShares:          512,
				MemoryLimitInBytes: 1024,
				OomScoreAdj:        10,
				CpusetCpus:         "0-3",
				CpusetMems:         "0",
				Unified:            map[string]string{"key": "value"},
				HugepageLimits: []*v1alpha1.HugepageLimit{
					{PageSize: "2MB", Limit: 2048},
				},
				MemorySwapLimitInBytes: 2048,
			}

			result := dispatcher.TransferToCRIResources(input)

			Expect(result.CpuPeriod).To(Equal(int64(1000)))
			Expect(result.CpuQuota).To(Equal(int64(2000)))
			Expect(result.CpuShares).To(Equal(int64(512)))
			Expect(result.MemoryLimitInBytes).To(Equal(int64(1024)))
			Expect(result.OomScoreAdj).To(Equal(int64(10)))
			Expect(result.CpusetCpus).To(Equal("0-3"))
			Expect(result.CpusetMems).To(Equal("0"))
			Expect(result.Unified).To(HaveKeyWithValue("key", "value"))
			Expect(result.HugepageLimits).To(HaveLen(1))
			Expect(result.HugepageLimits[0].PageSize).To(Equal("2MB"))
			Expect(result.HugepageLimits[0].Limit).To(Equal(uint64(2048)))
			Expect(result.MemorySwapLimitInBytes).To(Equal(int64(2048)))
		})

		It("should handle nil input", func() {
			result := dispatcher.TransferToCRIResources(nil)
			Expect(result).To(BeNil())
		})
	})

	Describe("TransferToHookResources", func() {
		It("should convert runtimeapi.LinuxContainerResources to v1alpha1.LinuxContainerResources", func() {
			input := &runtimeapi.LinuxContainerResources{
				CpuPeriod:          1000,
				CpuQuota:           2000,
				CpuShares:          512,
				MemoryLimitInBytes: 1024,
				OomScoreAdj:        10,
				CpusetCpus:         "0-3",
				CpusetMems:         "0",
				Unified:            map[string]string{"key": "value"},
				HugepageLimits: []*runtimeapi.HugepageLimit{
					{PageSize: "2MB", Limit: 2048},
				},
				MemorySwapLimitInBytes: 2048,
			}

			result := dispatcher.TransferToHookResources(input)

			Expect(result.CpuPeriod).To(Equal(int64(1000)))
			Expect(result.CpuQuota).To(Equal(int64(2000)))
			Expect(result.CpuShares).To(Equal(int64(512)))
			Expect(result.MemoryLimitInBytes).To(Equal(int64(1024)))
			Expect(result.OomScoreAdj).To(Equal(int64(10)))
			Expect(result.CpusetCpus).To(Equal("0-3"))
			Expect(result.CpusetMems).To(Equal("0"))
			Expect(result.Unified).To(HaveKeyWithValue("key", "value"))
			Expect(result.HugepageLimits).To(HaveLen(1))
			Expect(result.HugepageLimits[0].PageSize).To(Equal("2MB"))
			Expect(result.HugepageLimits[0].Limit).To(Equal(uint64(2048)))
			Expect(result.MemorySwapLimitInBytes).To(Equal(int64(2048)))
		})

		It("should handle nil input", func() {
			result := dispatcher.TransferToHookResources(nil)
			Expect(result).To(BeNil())
		})
	})

	Describe("GetResourceTypeFromHookType", func() {
		It("should return the correct RuntimeResourceType for a given HookType", func() {
			Expect(dispatcher.GetResourceTypeFromHookType(policy.PreRunPodSandbox)).To(Equal(server.RuntimePodResource))
			Expect(dispatcher.GetResourceTypeFromHookType(policy.PreCreateContainer)).To(Equal(server.RuntimeContainerResource))
			Expect(dispatcher.GetResourceTypeFromHookType(policy.HookType("unknown"))).To(Equal(server.RuntimeNoopResource))
		})

		It("should handle an empty HookType", func() {
			Expect(dispatcher.GetResourceTypeFromHookType(policy.HookType(""))).To(Equal(server.RuntimeNoopResource))
		})
	})

	Describe("SkipTAPHandle", func() {
		It("should return true if the SkipTAPLabel is present", func() {
			labels := map[string]string{options.SkipTAPLabel: "true"}
			Expect(dispatcher.SkipTAPHandle(labels)).To(BeTrue())
		})

		It("should return false if the SkipTAPLabel is not present", func() {
			labels := map[string]string{"otherLabel": "value"}
			Expect(dispatcher.SkipTAPHandle(labels)).To(BeFalse())
		})

		It("should handle an empty labels map", func() {
			labels := map[string]string{}
			Expect(dispatcher.SkipTAPHandle(labels)).To(BeFalse())
		})

		It("should handle nil labels map", func() {
			Expect(dispatcher.SkipTAPHandle(nil)).To(BeFalse())
		})
	})
})
