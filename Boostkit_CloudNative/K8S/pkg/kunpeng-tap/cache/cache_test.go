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
	criv1 "k8s.io/cri-api/pkg/apis/runtime/v1"

	"kunpeng.huawei.com/kunpeng-cloud-computing/api/kunpeng-tap/policy-manager/v1alpha1"
)

var _ = Describe("Cache", Ordered, func() {
	var (
		testCache Cache
	)
	BeforeEach(func() {
		// Setup a new fake cache instance before each test
		testCache, _ = NewCache(Options{
			CacheDir: "/tmp/cache",
		})
	})

	AfterEach(func() {
		// Clean up after each test
	})

	Describe("Pod Operations", func() {
		Describe("GetPods", func() {
			var podID1, podID2 string

			BeforeEach(func() {})

			It("should return all pods in the cache", func() {
				// Setup test pods
				podID1 = "test-pod-id-1"
				podID2 = "test-pod-id-2"
				testCache.(*cache).Pods[podID1] = &pod{ID: podID1}
				testCache.(*cache).Pods[podID2] = &pod{ID: podID2}

				// Get all pods
				pods := testCache.GetPods()

				// Verify both pods are returned
				Expect(pods).To(HaveLen(2))
				podIDs := []string{pods[0].GetID(), pods[1].GetID()}
				Expect(podIDs).To(ContainElements(podID1, podID2))
			})
			It("should return an empty list when no pods are in the cache", func() {
				// Get all pods
				pods := testCache.GetPods()

				// Verify no pods are returned
				Expect(pods).To(BeEmpty())
			})
		})

		Describe("InsertPod", func() {
			It("should correctly insert a pod into the cache", func() {
				// Setup a test pod
				podID := "test-pod-id"

				// Create a pod sandbox hook request for testing
				podInfo := &v1alpha1.PodSandboxHookRequest{
					PodMeta: &v1alpha1.PodSandboxMetadata{
						Name:      "test-pod",
						Uid:       podID,
						Namespace: "default",
					},
					Labels: map[string]string{
						"app": "test",
					},
					Annotations: map[string]string{
						"annotation-key": "annotation-value",
					},
					CgroupParent: "/kubepods.slice/kubepods-test.slice",
				}

				// Insert the pod into the cache
				pod, err := testCache.InsertPod(podID, podInfo, nil)
				Expect(err).NotTo(HaveOccurred())
				Expect(pod).NotTo(BeNil())
				Expect(pod.GetID()).To(Equal(podID))
				Expect(pod.GetUID()).To(Equal(podInfo.PodMeta.Uid))
				Expect(pod.GetName()).To(Equal(podInfo.PodMeta.Name))
				Expect(pod.GetNamespace()).To(Equal(podInfo.PodMeta.Namespace))
				Expect(pod.GetLabels()).To(Equal(podInfo.Labels))
				Expect(pod.GetAnnotations()).To(Equal(podInfo.Annotations))
				Expect(pod.GetCgroupParentDir()).To(Equal(podInfo.CgroupParent))
			})

			It("should return an error when inserting a pod with an invalid ID", func() {
				// Setup a test pod with an invalid ID
				podID := ""
				// Create a pod sandbox hook request for testing
				podInfo := &v1alpha1.PodSandboxHookRequest{
					PodMeta: &v1alpha1.PodSandboxMetadata{
						Name:      "test-pod",
						Uid:       podID,
						Namespace: "default",
					},
					Labels: map[string]string{
						"app": "test",
					},
					Annotations: map[string]string{
						"annotation-key": "annotation-value",
					},
					CgroupParent: "/kubepods.slice/kubepods-test.slice",
				}

				// Insert the pod into the cache
				pod, err := testCache.InsertPod(podID, podInfo, nil)
				Expect(err).To(HaveOccurred())
				Expect(pod).To(BeNil())
			})
		})

		Describe("LookupPod", func() {
			It("should correctly lookup a pod by ID", func() {
				// Setup and insert a test pod
				podID := "test-pod-id"
				testCache.(*cache).Pods[podID] = &pod{
					ID:           podID,
					UID:          podID,
					Name:         "test-pod",
					Namespace:    "default",
					Labels:       map[string]string{"app": "test"},
					Annotations:  map[string]string{"annotation-key": "annotation-value"},
					CgroupParent: "/kubepods.slice/kubepods-test.slice",
				}

				// Look up the pod
				pod, found := testCache.LookupPod(podID)

				// Verify the pod was found
				Expect(found).To(BeTrue())
				Expect(pod).NotTo(BeNil())
				Expect(pod.GetID()).To(Equal(podID))
				Expect(pod.GetName()).To(Equal("test-pod"))
				Expect(pod.GetNamespace()).To(Equal("default"))
				Expect(pod.GetLabels()).To(HaveKeyWithValue("app", "test"))
				Expect(pod.GetAnnotations()).To(HaveKeyWithValue("annotation-key", "annotation-value"))
				Expect(pod.GetCgroupParentDir()).To(Equal("/kubepods.slice/kubepods-test.slice"))
			})

			It("should return false when looking up a non-existent pod", func() {
				// Look up a non-existent pod
				_, found := testCache.LookupPod("non-existent-pod")

				// Verify the pod was not found
				Expect(found).To(BeFalse())
			})
		})

		Describe("DeletePod", func() {
			It("should delete a pod from the cache", func() {
				// Setup and insert a test pod
				podID := "test-pod-id"
				podInfo := &v1alpha1.PodSandboxHookRequest{
					PodMeta: &v1alpha1.PodSandboxMetadata{
						Name:      "test-pod",
						Uid:       podID,
						Namespace: "default",
					},
				}
				_, err := testCache.InsertPod(podID, podInfo, nil)
				Expect(err).NotTo(HaveOccurred())

				// Delete the pod
				deletedPod := testCache.DeletePod(podID)

				// Verify the pod was deleted
				Expect(deletedPod).NotTo(BeNil())
				Expect(deletedPod.GetID()).To(Equal(podID))

				// Verify the pod is no longer in the cache
				_, found := testCache.LookupPod(podID)
				Expect(found).To(BeFalse())
			})
		})
	})

	Describe("Container Operations", func() {
		Describe("Container Lookup", func() {
			var (
				podID       string
				containerID string
			)

			BeforeEach(func() {
				// Setup a test pod and container
				podID = "test-pod-id"
				containerID = "test-container-id"

				// Insert the pod into the cache
				testCache.(*cache).Pods[podID] = &pod{
					cache:        testCache.(*cache),
					ID:           podID,
					UID:          podID,
					Name:         "test-pod",
					Namespace:    "default",
					Labels:       map[string]string{"app": "test"},
					Annotations:  map[string]string{"annotation-key": "annotation-value"},
					CgroupParent: "/kubepods.slice/kubepods-test.slice",
				}

				// Insert the container into the cache
				testCache.(*cache).Containers[containerID] = &container{
					cache: testCache.(*cache),
					ID:    containerID,
					PodID: podID,
					Name:  "test-container",
				}
			})

			It("should correctly lookup a container by ID", func() {
				// Look up the container
				container, found := testCache.LookupContainer(containerID)

				// Verify the container was found
				Expect(found).To(BeTrue())
				Expect(container).NotTo(BeNil())

				// Verify container properties
				Expect(container.GetID()).To(Equal(containerID))
				Expect(container.GetPodID()).To(Equal(podID))
				Expect(container.GetName()).To(Equal("test-container"))
			})

			It("should return false when looking up a non-existent container", func() {
				// Look up a non-existent container
				_, found := testCache.LookupContainer("non-existent-container")

				// Verify the container was not found
				Expect(found).To(BeFalse())
			})
		})

		Describe("Container Management", func() {
			var (
				podID       string
				containerID string
			)

			BeforeEach(func() {
				// Setup a test pod and container
				podID = "test-pod-id"
				containerID = "test-container-id"

				testCache.(*cache).Pods[podID] = &pod{
					cache:        testCache.(*cache),
					ID:           podID,
					UID:          podID,
					Name:         "test-pod",
					Namespace:    "default",
					Labels:       map[string]string{"app": "test"},
					Annotations:  map[string]string{"annotation-key": "annotation-value"},
					CgroupParent: "/kubepods.slice/kubepods-test.slice",
				}

				testCache.(*cache).Containers[containerID] = &container{
					cache: testCache.(*cache),
					ID:    containerID,
					PodID: podID,
					Name:  "test-container",
				}
			})

			It("should delete a container from the cache", func() {
				// Delete the container
				deletedContainer := testCache.DeleteContainer(containerID)

				// Verify the container was deleted
				Expect(deletedContainer).NotTo(BeNil())
				Expect(deletedContainer.GetID()).To(Equal(containerID))

				// Verify the container is no longer in the cache
				_, found := testCache.LookupContainer(containerID)
				Expect(found).To(BeFalse())
			})

			It("should get the pod associated with a container", func() {
				// Look up the container
				container, found := testCache.LookupContainer(containerID)
				Expect(found).To(BeTrue())

				// Get the pod associated with the container
				pod, found := container.GetPod()

				// Verify the pod was found
				Expect(found).To(BeTrue())
				Expect(pod).NotTo(BeNil())
				Expect(pod.GetID()).To(Equal(podID))
			})
		})
	})

	Describe("UpdateContainerID", func() {
		var (
			containerID string
		)

		BeforeEach(func() {
			// Setup a test container
			containerID = "test-container-id"
			testCache.(*cache).Containers[containerID] = &container{
				cache: testCache.(*cache),
				ID:    containerID,
			}
		})

		It("should update the container's runtime ID", func() {
			// Create a mock response to update the container ID
			response := &criv1.CreateContainerResponse{
				ContainerId: "new-runtime-id",
			}

			// Update the container ID
			updatedContainer, err := testCache.UpdateContainerID(containerID, response)
			Expect(err).NotTo(HaveOccurred())

			// Verify the container ID was updated
			Expect(updatedContainer.GetID()).To(Equal("new-runtime-id"))
		})
	})

	Describe("GetContainers", func() {
		var containerID1, containerID2 string

		BeforeEach(func() {
			// Setup test containers
			containerID1 = "test-container-id-1"
			containerID2 = "test-container-id-2"
			testCache.(*cache).Containers[containerID1] = &container{
				cache: testCache.(*cache),
				ID:    containerID1,
				PodID: "test-pod-id-1",
			}
			testCache.(*cache).Containers[containerID2] = &container{
				cache: testCache.(*cache),
				ID:    containerID2,
				PodID: "test-pod-id-2",
			}
		})

		It("should return all containers in the cache", func() {
			// Get all containers
			containers := testCache.GetContainers()

			// Verify both containers are returned
			Expect(containers).To(HaveLen(2))
			containerIDs := []string{containers[0].GetID(), containers[1].GetID()}
			Expect(containerIDs).To(ContainElements(containerID1, containerID2))
		})
	})
})
