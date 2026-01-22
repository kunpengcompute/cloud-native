// Copyright 2019 Intel Corporation. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cache

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"kunpeng.huawei.com/kunpeng-cloud-computing/api/kunpeng-tap/policy-manager/v1alpha1"
)

var _ = Describe("Container", func() {
	var (
		containerInstance *container
	)

	BeforeEach(func() {
		containerInstance = &container{
			ID:    "test-container-id",
			PodID: "test-pod-id",
			Name:  "test-container",
			cache: &cache{
				Pods: map[string]*pod{
					"test-pod-id": {
						ID: "test-pod-id",
						containers: map[string]string{
							"test-container": "test-container-id",
						},
					},
				},
				Containers: map[string]*container{
					"test-container-id": containerInstance,
				},
			},
			LinuxReq: &v1alpha1.LinuxContainerResources{},
		}
	})

	Describe("fromDockerRunRequest", func() {
		It("should create a container from a ContainerResourceHookRequest", func() {
			// Setup test data
			containerID := "test-container-id"
			podID := "test-pod-id"

			// Call fromDockerRunRequest
			err := containerInstance.fromDockerRunRequest(&v1alpha1.ContainerResourceHookRequest{
				ContainerMeta: &v1alpha1.ContainerMetadata{
					Id:   containerID,
					Name: "test-container",
				},
				PodMeta: &v1alpha1.PodSandboxMetadata{
					Id: podID,
				},
			})
			Expect(err).NotTo(HaveOccurred())

			// Verify container properties
			Expect(containerInstance.ID).To(Equal(containerID))
			Expect(containerInstance.PodID).To(Equal(podID))
			Expect(containerInstance.Name).To(Equal("test-container"))
		})
	})

	Describe("GetPod", func() {
		It("should return the pod associated with the container", func() {
			// Setup test data
			podID := "test-pod-id"
			containerInstance.PodID = podID
			containerInstance.cache = &cache{
				Pods: map[string]*pod{
					podID: {
						ID: podID,
					},
				},
			}

			// Call GetPod
			pod, found := containerInstance.GetPod()

			// Verify the pod was found
			Expect(found).To(BeTrue())
			Expect(pod).NotTo(BeNil())
			Expect(pod.GetID()).To(Equal(podID))
		})
	})

	Describe("Modifying Functions", func() {
		BeforeEach(func() {
			// Setup a test container with Linux resources
			linuxResources := &v1alpha1.LinuxContainerResources{}
			containerInstance.LinuxReq = linuxResources
		})

		It("should set Linux resources", func() {
			resources := &v1alpha1.LinuxContainerResources{
				CpuPeriod: 1000,
				CpuQuota:  2000,
			}

			// Set the resources
			containerInstance.SetLinuxResources(resources)

			// Verify the resources are set
			Expect(containerInstance.GetLinuxResources()).To(Equal(resources))
		})

		It("should set CPU period", func() {
			containerInstance.SetCPUPeriod(1000)

			// Verify the CPU period is set
			Expect(containerInstance.GetCPUPeriod()).To(Equal(int64(1000)))
		})

		It("should set CPU quota", func() {
			containerInstance.SetCPUQuota(2000)

			// Verify the CPU quota is set
			Expect(containerInstance.GetCPUQuota()).To(Equal(int64(2000)))
		})

		It("should set CPU shares", func() {
			containerInstance.SetCPUShares(512)

			// Verify the CPU shares are set
			Expect(containerInstance.GetCPUShares()).To(Equal(int64(512)))
		})

		It("should set memory limit", func() {
			containerInstance.SetMemoryLimit(4096)

			// Verify the memory limit is set
			Expect(containerInstance.GetMemoryLimit()).To(Equal(int64(4096)))
		})

		It("should set OOM score adjustment", func() {
			containerInstance.SetOomScoreAdj(100)

			// Verify the OOM score adjustment is set
			Expect(containerInstance.GetOomScoreAdj()).To(Equal(int64(100)))
		})

		It("should set cpuset CPUs", func() {
			containerInstance.SetCpusetCpus("0-3")

			// Verify the cpuset CPUs are set
			Expect(containerInstance.GetCpusetCpus()).To(Equal("0-3"))
		})

		It("should set cpuset mems", func() {
			containerInstance.SetCpusetMems("0")

			// Verify the cpuset mems are set
			Expect(containerInstance.GetCpusetMems()).To(Equal("0"))
		})
	})

	Describe("Pending and Deletion Functions", func() {
		BeforeEach(func() {
			// Setup a test container
			containerInstance.ID = "test-container-id"
		})

		It("should clear pending state for a specific controller", func() {
			// Setup pending state tracking
			pendingControllers := []string{"CRI"}
			containerInstance.pending = make(map[string]struct{}, len(pendingControllers))
			for _, controller := range pendingControllers {
				containerInstance.pending[controller] = struct{}{}
			}

			// Verify the container has pending changes
			Expect(containerInstance.HasPending("CRI")).To(BeTrue())

			// Clear pending changes
			containerInstance.ClearPending("CRI")

			// Verify the pending state is cleared
			Expect(containerInstance.HasPending("CRI")).To(BeFalse())
		})
	})
})
