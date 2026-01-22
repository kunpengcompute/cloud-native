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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	v1 "k8s.io/api/core/v1"
	"kunpeng.huawei.com/kunpeng-cloud-computing/api/kunpeng-tap/policy-manager/v1alpha1"
)

var _ = Describe("Pod", func() {
	var (
		podInstance *pod
	)

	BeforeEach(func() {
		// Create a new fake pod instance
		cacheInstance := &cache{
			Pods:       map[string]*pod{},
			Containers: map[string]*container{},
		}
		podInstance = &pod{
			ID:        "test-uid",
			Name:      "test-pod",
			Namespace: "default",
			cache:     cacheInstance,
		}
	})

	Describe("FromRunRequest", func() {
		It("should create a pod from a RunPodSandboxRequest", func() {
			// Not Implemented
		})
	})

	Describe("FromListResponse", func() {
		It("should create a pod from a PodSandbox list response", func() {
			// Not Implemented
		})
	})

	Describe("FromDockerRunRequest", func() {
		It("should create a pod from a PodSandboxHookRequest", func() {
			// Setup test data
			podID := "test-uid"
			podName := "test-pod"
			namespace := "default"
			cgroupParent := "/kubepods.slice/kubepod-test.slice"
			labels := map[string]string{"app": "test"}
			annotations := map[string]string{"annotation-key": "annotation-value"}

			// Create a sandbox hook request
			podRequest := &v1alpha1.PodSandboxHookRequest{
				PodMeta: &v1alpha1.PodSandboxMetadata{
					Uid:       podID,
					Name:      podName,
					Namespace: namespace,
				},
				Labels:       labels,
				Annotations:  annotations,
				CgroupParent: cgroupParent,
			}

			// Call fromDockerRunRequest
			err := podInstance.fromDockerRunRequest(podRequest)
			Expect(err).NotTo(HaveOccurred())

			// Verify pod properties
			Expect(podInstance.GetUID()).To(Equal(podID))
			Expect(podInstance.GetName()).To(Equal(podName))
			Expect(podInstance.GetNamespace()).To(Equal(namespace))
			Expect(podInstance.GetLabels()).To(HaveKeyWithValue("app", "test"))
			Expect(podInstance.GetAnnotations()).To(HaveKeyWithValue("annotation-key", "annotation-value"))
			Expect(podInstance.GetCgroupParentDir()).To(Equal("/kubepods.slice/kubepod-test.slice"))
		})
	})

	// TODO:
	Describe("GetContainers", func() {

		BeforeEach(func() {
			podInstance.containers = map[string]string{
				"test-container":   "test-container-id",
				"test-container-2": "test-container-id-2",
			}
			podInstance.cache.Containers["test-container-id"] = &container{
				ID:    "test-container-id",
				PodID: podInstance.ID,
				Name:  "test-container",
				cache: podInstance.cache,
			}
			podInstance.cache.Containers["test-container-id-2"] = &container{
				ID:    "test-container-id-2",
				PodID: podInstance.ID,
				Name:  "test-container-2",
				cache: podInstance.cache,
			}
		})

		It("should return the normal containers of a pod", func() {
			// Get the actual container instances from the cache
			expectedContainer1 := podInstance.cache.Containers["test-container-id"]
			expectedContainer2 := podInstance.cache.Containers["test-container-id-2"]

			// Call GetContainers
			containers := podInstance.GetContainers()

			// Verify the containers
			Expect(containers).To(HaveLen(2))
			Expect(containers).To(ContainElement(expectedContainer1))
			Expect(containers).To(ContainElement(expectedContainer2))

		})
	})

	Describe("GetContainer", func() {
		BeforeEach(func() {
			podInstance.cache.Containers["test-container-id"] = &container{
				ID:    "test-container-id",
				Name:  "test-container",
				PodID: podInstance.ID,
				cache: podInstance.cache,
			}
			podInstance.containers = map[string]string{
				"test-container": "test-container-id",
			}
		})

		It("should return a container by its name", func() {
			// Setup test data
			containerName := "test-container"

			// Call GetContainer
			container, found := podInstance.GetContainer(containerName)

			// Verify the container
			Expect(found).To(BeTrue())
			Expect(container).NotTo(BeNil())
			Expect(container.GetID()).To(Equal("test-container-id"))
		})
	})

	Describe("GetQOSClass", func() {
		It("should return the correct QOS class for the pod", func() {
			// Setup test data
			podInstance.QOSClass = v1.PodQOSGuaranteed

			// Call GetQOSClass
			qosClass := podInstance.GetQOSClass()

			// Verify QOS class
			Expect(qosClass).To(Equal(v1.PodQOSGuaranteed))
		})
	})

	Describe("GetPodResourceRequirements", func() {
		It("should return the resource requirements for the pod", func() {
			// Setup test data
			resourceReq := v1.ResourceRequirements{}
			podInstance.Resources = resourceReq

			// Call GetPodResourceRequirements
			req := podInstance.GetPodResourceRequirements()

			// Verify resource requirements
			Expect(req).To(Equal(resourceReq))
		})
	})

	Describe("GetLinuxResources", func() {
		It("should return the Linux resources for the pod", func() {
			// Setup test data
			linuxResources := &v1alpha1.LinuxContainerResources{
				CpuPeriod: 1000,
				CpuQuota:  2000,
			}
			podInstance.LinuxReq = linuxResources

			// Call GetLinuxResources
			resources := podInstance.GetLinuxResources()

			// Verify Linux resources
			Expect(resources).To(Equal(linuxResources))
		})
	})
})
