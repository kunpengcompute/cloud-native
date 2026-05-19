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

package nri

import (
	"context"

	"github.com/containerd/nri/pkg/api"
	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"

	"kunpeng.huawei.com/kunpeng-cloud-computing/api/kunpeng-tap/policy-manager/v1alpha1"
)

var _ = ginkgo.Describe("Plugin", func() {
	var (
		plugin *Agent
	)

	ginkgo.BeforeEach(func() {
		plugin = &Agent{}
	})

	ginkgo.Describe("Configure", func() {
		ginkgo.Context("when configuring the plugin", func() {
			ginkgo.BeforeEach(func() {
				plugin = &Agent{
					mask: api.MustParseEventMask("RunPodSandbox,CreateContainer,PostCreateContainer,RemoveContainer"),
				}
			})

			ginkgo.It("should configure successfully and return the correct mask", func() {
				mask, err := plugin.Configure(context.Background(), "test-config", "containerd", "1.7.0")
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(mask).To(gomega.Equal(plugin.mask))
			})
		})
	})

	ginkgo.Describe("convertToHookRequest", func() {
		var (
			pod       *api.PodSandbox
			container *api.Container
		)

		ginkgo.BeforeEach(func() {
			pod = &api.PodSandbox{
				Id:          "pod-123",
				Name:        "test-pod",
				Namespace:   "default",
				Uid:         "pod-uid-123",
				Annotations: map[string]string{"pod-annotation": "pod-value"},
				Labels:      map[string]string{"pod-label": "pod-label-value"},
			}

			container = &api.Container{
				Id:          "container-123",
				Name:        "test-container",
				Annotations: map[string]string{"container-annotation": "container-value"},
				Env:         []string{"TEST_ENV=test-value", "ANOTHER_ENV=another-value"},
			}
		})

		ginkgo.It("should convert pod and container to hook request correctly", func() {
			hookReq, err := plugin.convertToHookRequest(pod, container)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(hookReq).NotTo(gomega.BeNil())

			// Test pod metadata
			gomega.Expect(hookReq.PodMeta.Name).To(gomega.Equal("test-pod"))
			gomega.Expect(hookReq.PodMeta.Namespace).To(gomega.Equal("default"))
			gomega.Expect(hookReq.PodMeta.Uid).To(gomega.Equal("pod-uid-123"))
			gomega.Expect(hookReq.PodMeta.Id).To(gomega.Equal("pod-123"))

			// Test container metadata
			gomega.Expect(hookReq.ContainerMeta.Name).To(gomega.Equal("test-container"))
			gomega.Expect(hookReq.ContainerMeta.Id).To(gomega.Equal("container-123"))

			// Test annotations and labels
			gomega.Expect(hookReq.PodAnnotations["pod-annotation"]).To(gomega.Equal("pod-value"))
			gomega.Expect(hookReq.PodLabels["pod-label"]).To(gomega.Equal("pod-label-value"))
			gomega.Expect(hookReq.ContainerAnnotations["container-annotation"]).To(gomega.Equal("container-value"))

			// Test environment variables
			gomega.Expect(hookReq.ContainerEnvs["TEST_ENV"]).To(gomega.Equal("test-value"))
			gomega.Expect(hookReq.ContainerEnvs["ANOTHER_ENV"]).To(gomega.Equal("another-value"))
		})
	})

	ginkgo.Describe("convertToCtrAdjustment", func() {
		ginkgo.Context("when hook response contains valid data", func() {
			var hookResp *v1alpha1.ContainerResourceHookResponse

			ginkgo.BeforeEach(func() {
				hookResp = &v1alpha1.ContainerResourceHookResponse{
					ContainerResources: &v1alpha1.LinuxContainerResources{
						CpuPeriod:          100000,
						CpuQuota:           50000,
						CpuShares:          1024,
						MemoryLimitInBytes: 1073741824, // 1GB
						CpusetCpus:         "0-1",
						CpusetMems:         "0",
					},
					ContainerAnnotations: map[string]string{
						"test-annotation": "test-value",
					},
					ContainerEnvs: map[string]string{
						"TEST_ENV": "test-value",
					},
				}
			})

			ginkgo.It("should convert hook response to container adjustment correctly", func() {
				adjustment := plugin.convertToCtrAdjustment(hookResp)
				gomega.Expect(adjustment).NotTo(gomega.BeNil())
				gomega.Expect(adjustment.Annotations["test-annotation"]).To(gomega.Equal("test-value"))
				gomega.Expect(adjustment.Env).To(gomega.HaveLen(1))
			})
		})

		ginkgo.Context("when hook response is nil", func() {
			ginkgo.It("should return nil adjustment", func() {
				adjustment := plugin.convertToCtrAdjustment(nil)
				gomega.Expect(adjustment).To(gomega.BeNil())
			})
		})
	})

	ginkgo.Describe("convertEnvironmentVariables", func() {
		var envVars []string

		ginkgo.BeforeEach(func() {
			envVars = []string{
				"KEY1=value1",
				"KEY2=value2",
				"KEY_WITHOUT_VALUE",
				"KEY_WITH_EQUALS=value=with=equals", //pragma: allowlist secret
				"",
			}
		})

		ginkgo.It("should convert environment variables to map correctly", func() {
			envMap := plugin.convertEnvironmentVariables(envVars)

			gomega.Expect(envMap["KEY1"]).To(gomega.Equal("value1"))
			gomega.Expect(envMap["KEY2"]).To(gomega.Equal("value2"))
			gomega.Expect(envMap["KEY_WITHOUT_VALUE"]).To(gomega.Equal(""))
			gomega.Expect(envMap["KEY_WITH_EQUALS"]).To(gomega.Equal("value=with=equals"))
			gomega.Expect(envMap).To(gomega.HaveLen(4)) // Empty string should be ignored
		})
	})

	ginkgo.Describe("extractCgroupParent", func() {
		ginkgo.Context("when pod has Linux config with cgroup parent", func() {
			var pod *api.PodSandbox

			ginkgo.BeforeEach(func() {
				pod = &api.PodSandbox{
					Linux: &api.LinuxPodSandbox{
						CgroupParent: "/kubepods/besteffort/pod123",
					},
				}
			})

			ginkgo.It("should extract the cgroup parent correctly", func() {
				cgroupParent := plugin.extractCgroupParent(pod)
				gomega.Expect(cgroupParent).To(gomega.Equal("/kubepods/besteffort/pod123"))
			})
		})

		ginkgo.Context("when pod has no Linux config", func() {
			var podWithoutLinux *api.PodSandbox

			ginkgo.BeforeEach(func() {
				podWithoutLinux = &api.PodSandbox{}
			})

			ginkgo.It("should return empty string", func() {
				cgroupParent := plugin.extractCgroupParent(podWithoutLinux)
				gomega.Expect(cgroupParent).To(gomega.Equal(""))
			})
		})
	})
})
