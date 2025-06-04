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

package policy_test

import (
	"context"
	"path"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"kunpeng.huawei.com/kunpeng-cloud-computing/api/kunpeng-tap/policy-manager/v1alpha1"
	"kunpeng.huawei.com/kunpeng-cloud-computing/pkg/kunpeng-tap/cache"
	"kunpeng.huawei.com/kunpeng-cloud-computing/pkg/kunpeng-tap/policy"
)

var _ = Describe("Allocation", func() {
	var allocation *policy.Allocation
	BeforeEach(func() {
		allocation = policy.NewAllocation()
	})
	Describe("SetCPUSetCpus", func() {
		It("Should be pass", func() {
			allocation.SetCPUSetCpus("0-99")
			Expect(allocation.Resources.CpusetCpus).To(Equal("0-99"))
		})
	})
	Describe("Merge", func() {
		Describe("Empty allocation merge nil ContainerResourceHookResponse", func() {
			It("Should be pass", func() {
				allocation.Merge(nil)
				Expect(allocation.Resources).To(Equal(&v1alpha1.LinuxContainerResources{}))
			})
		})
		Describe("Allocation merge nil ContainerResourceHookResponse", func() {
			It("Should be pass", func() {
				allocation.Resources = &v1alpha1.LinuxContainerResources{
					CpuPeriod: 1,
				}
				allocation.Merge(nil)
			})
			It("Should be pass", func() {
				allocation.Resources = &v1alpha1.LinuxContainerResources{
					CpuPeriod: 1,
				}
				resp := &v1alpha1.ContainerResourceHookResponse{}
				allocation.Merge(resp)
				Expect(resp.ContainerResources.CpuPeriod).To(Equal(int64(1)))
			})
		})
	})
})

var _ = Describe("Hooks", Ordered, func() {
	var hookManager policy.HookManager
	BeforeEach(func() {
		proxyCache, err := cache.NewCache(cache.Options{CacheDir: path.Join(utSocketPathPrefix, "/topology_cache")})
		Expect(err).To(BeNil())
		hookManager = policy.NewPolicyManager(proxyCache)
	})
	Context("No hooks", func() {
		It("Should be pass", func() {
			resp, err := hookManager.PostStartContainerHook(context.Background(), &v1alpha1.ContainerResourceHookRequest{
				PodMeta:       &v1alpha1.PodSandboxMetadata{},
				ContainerMeta: &v1alpha1.ContainerMetadata{},
			})
			Expect(err).To(BeNil())
			Expect(resp).NotTo(BeNil())

			resp, err = hookManager.PostStopContainerHook(context.Background(), &v1alpha1.ContainerResourceHookRequest{
				PodMeta:       &v1alpha1.PodSandboxMetadata{},
				ContainerMeta: &v1alpha1.ContainerMetadata{},
			})
			Expect(err).To(BeNil())
			Expect(resp).NotTo(BeNil())

			resp, err = hookManager.PreCreateContainerHook(context.Background(), &v1alpha1.ContainerResourceHookRequest{
				PodMeta:       &v1alpha1.PodSandboxMetadata{},
				ContainerMeta: &v1alpha1.ContainerMetadata{},
			})
			Expect(err).To(BeNil())
			Expect(resp).NotTo(BeNil())

			resp, err = hookManager.PreRemoveContainerHook(context.Background(), &v1alpha1.ContainerResourceHookRequest{
				PodMeta:       &v1alpha1.PodSandboxMetadata{},
				ContainerMeta: &v1alpha1.ContainerMetadata{},
			})
			Expect(err).To(BeNil())
			Expect(resp).NotTo(BeNil())

			resp, err = hookManager.PreStartContainerHook(context.Background(), &v1alpha1.ContainerResourceHookRequest{
				PodMeta:       &v1alpha1.PodSandboxMetadata{},
				ContainerMeta: &v1alpha1.ContainerMetadata{},
			})
			Expect(err).To(BeNil())
			Expect(resp).NotTo(BeNil())

			resp, err = hookManager.PreUpdateContainerResourcesHook(context.Background(), &v1alpha1.ContainerResourceHookRequest{
				PodMeta:       &v1alpha1.PodSandboxMetadata{},
				ContainerMeta: &v1alpha1.ContainerMetadata{},
			})
			Expect(err).To(BeNil())
			Expect(resp).NotTo(BeNil())

			podResp, err := hookManager.PostStopPodSandboxHook(context.Background(), &v1alpha1.PodSandboxHookRequest{
				PodMeta: &v1alpha1.PodSandboxMetadata{},
			})
			Expect(err).To(BeNil())
			Expect(podResp).NotTo(BeNil())

			podResp, err = hookManager.PreRemovePodSandboxHook(context.Background(), &v1alpha1.PodSandboxHookRequest{
				PodMeta: &v1alpha1.PodSandboxMetadata{},
			})
			Expect(err).To(BeNil())
			Expect(podResp).NotTo(BeNil())

			podResp, err = hookManager.PreRunPodSandboxHook(context.Background(), &v1alpha1.PodSandboxHookRequest{
				PodMeta: &v1alpha1.PodSandboxMetadata{},
			})
			Expect(err).To(BeNil())
			Expect(podResp).NotTo(BeNil())
		})
	})
})
