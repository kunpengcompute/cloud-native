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

package numa_aware

import (
	"github.com/docker/docker/client"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	criv1 "k8s.io/cri-api/pkg/apis/runtime/v1"
	numa_aware_policy "kunpeng.huawei.com/kunpeng-cloud-computing/pkg/kunpeng-tap/policy/numa-aware"

	"kunpeng.huawei.com/kunpeng-cloud-computing/pkg/kunpeng-tap/cache"
	"kunpeng.huawei.com/kunpeng-cloud-computing/pkg/kunpeng-tap/policy"
)

const (
	// Test constants for cgroup paths
	cgroupGuaranteed = "/kubepods/guaranteed"
	cgroupBurstable  = "/kubepods/burstable"
	cgroupBestEffort = "/kubepods/besteffort"
)

// MockCache implements cache.Cache interface for testing
type MockCache struct {
	nodeResources []cache.NumaNodeResources
}

// MockInvalidContext implements HookContext but is not ContainerContext
type MockInvalidContext struct{}

func (m *MockInvalidContext) FromProxy(req interface{}) {
	// Do nothing
}

func (m *MockCache) InsertPod(id string, msg interface{}, status *cache.PodStatus) (cache.Pod, error) {
	return nil, nil
}

func (m *MockCache) DeletePod(id string) cache.Pod {
	return nil
}

func (m *MockCache) LookupPod(id string) (cache.Pod, bool) {
	return nil, false
}

func (m *MockCache) InsertContainer(containerId string, msg interface{}) (cache.Container, error) {
	return nil, nil
}

func (m *MockCache) UpdateContainerID(cacheID string, msg interface{}) (cache.Container, error) {
	return nil, nil
}

func (m *MockCache) DeleteContainer(id string) cache.Container {
	return nil
}

func (m *MockCache) LookupContainer(id string) (cache.Container, bool) {
	return nil, false
}

func (m *MockCache) GetPendingContainers() []cache.Container {
	return nil
}

func (m *MockCache) GetPods() []cache.Pod {
	return nil
}

func (m *MockCache) GetContainers() []cache.Container {
	return nil
}

func (m *MockCache) GetContainerCacheIds() []string {
	return nil
}

func (m *MockCache) GetContainerIds() []string {
	return nil
}

func (m *MockCache) RefreshPods(msg *criv1.ListPodSandboxResponse, status map[string]*cache.PodStatus) ([]cache.Pod, []cache.Pod, []cache.Container) {
	return nil, nil, nil
}

func (m *MockCache) RefreshContainers(msg *criv1.ListContainersResponse) ([]cache.Container, []cache.Container) {
	return nil, nil
}

func (m *MockCache) GetNodeResources() []cache.NumaNodeResources {
	return m.nodeResources
}

func (m *MockCache) ValidateCachedContainers(containerIds []string) []string {
	return nil
}

func (m *MockCache) CleanupStaleContainers(staleContainerIds []string) int {
	return 0
}

func (m *MockCache) LoadStoreDocker(dockerClient client.CommonAPIClient, cgroupDriver string) error {
	return nil
}

func (m *MockCache) LoadStoreContainerd(backendRuntimeServiceClient criv1.RuntimeServiceClient) error {
	return nil
}

// Helper function to create resource list
func createResourceList(cpu, memory string) *v1.ResourceList {
	resources := v1.ResourceList{}
	if cpu != "" {
		resources[v1.ResourceCPU] = resource.MustParse(cpu)
	}
	if memory != "" {
		resources[v1.ResourceMemory] = resource.MustParse(memory)
	}
	return &resources
}

// Helper function to create container context
func createContainerContext(cgroupParent string, reqCPU, limitCPU, reqMem, limitMem string) *policy.ContainerContext {
	reqResources := createResourceList(reqCPU, reqMem)
	limitResources := createResourceList(limitCPU, limitMem)

	ctx := &policy.ContainerContext{
		Request: policy.ContainerRequest{
			CgroupParent: cgroupParent,
			Resources: &policy.Resources{
				EstimatedRequirements: &v1.ResourceRequirements{
					Requests: *reqResources,
					Limits:   *limitResources,
				},
			},
		},
	}
	return ctx
}

var _ = Describe("NumaAwarePolicy", func() {
	var mockCache *MockCache
	var policy *numa_aware_policy.NumaAwarePolicy

	BeforeEach(func() {
		mockCache = &MockCache{}
	})

	Describe("NewNumaAwarePolicy", func() {
		It("should create a new NUMA-aware policy", func() {
			policy := numa_aware_policy.NewNumaAwarePolicy(mockCache)

			Expect(policy).NotTo(BeNil())
			Expect(policy.Name()).To(Equal(numa_aware_policy.PolicyName))
			Expect(policy.Description()).To(Equal(numa_aware_policy.PolicyDescription))
		})
	})

	Describe("Name", func() {
		It("should return the correct policy name", func() {
			policy := &numa_aware_policy.NumaAwarePolicy{}
			Expect(policy.Name()).To(Equal(numa_aware_policy.PolicyName))
		})
	})

	Describe("Description", func() {
		It("should return the correct policy description", func() {
			policy := &numa_aware_policy.NumaAwarePolicy{}
			Expect(policy.Description()).To(Equal(numa_aware_policy.PolicyDescription))
		})
	})

	Describe("SetCache", func() {
		It("should set the cache correctly", func() {
			policy := &numa_aware_policy.NumaAwarePolicy{}

			policy.SetCache(mockCache)
			Expect(policy.GetCache()).To(Equal(mockCache))
		})
	})

	Describe("PreCreateContainerHook", func() {
		BeforeEach(func() {
			policy = numa_aware_policy.NewNumaAwarePolicy(mockCache).(*numa_aware_policy.NumaAwarePolicy)
		})

		Context("with invalid context", func() {
			It("should handle nil context", func() {
				alloc, err := policy.PreCreateContainerHook(nil)
				Expect(err).NotTo(HaveOccurred())
				Expect(alloc).To(BeNil())
			})

			It("should handle wrong context type", func() {
				wrongCtx := &MockInvalidContext{}
				alloc, err := policy.PreCreateContainerHook(wrongCtx)
				Expect(err).NotTo(HaveOccurred())
				Expect(alloc).To(BeNil())
			})
		})

		Context("with nil resources", func() {
			It("should handle nil resources without panic", func() {
				ctx := createContainerContext(cgroupGuaranteed, "", "", "", "")
				ctx.Request.Resources = nil // Set resources to nil to test the nil check

				alloc, err := policy.PreCreateContainerHook(ctx)
				Expect(err).NotTo(HaveOccurred())
				Expect(alloc).To(BeNil())
			})

			It("should handle empty resources (no EstimatedRequirements)", func() {
				emptyCtx := createContainerContext(cgroupGuaranteed, "", "", "", "")
				emptyCtx.Request.Resources.EstimatedRequirements = nil

				alloc, err := policy.PreCreateContainerHook(emptyCtx)
				Expect(err).NotTo(HaveOccurred())
				Expect(alloc).To(BeNil())
			})
		})

		Context("with different QOS classes", func() {
			It("should not allocate for BestEffort QOS", func() {
				ctx := createContainerContext(cgroupBestEffort, "100m", "200m", "128Mi", "256Mi")

				alloc, err := policy.PreCreateContainerHook(ctx)
				Expect(err).NotTo(HaveOccurred())
				Expect(alloc).To(BeNil())
			})

			It("should allocate CPU set for Guaranteed QOS", func() {
				// Setup mock cache with node resources
				mockCache.nodeResources = []cache.NumaNodeResources{
					{
						CpuTotal:         4.0,
						CpuUsed:          1.0,
						CpuUsedByRequest: 0.5,
						CpuFree:          3.0,
					},
					{
						CpuTotal:         4.0,
						CpuUsed:          2.0,
						CpuUsedByRequest: 1.0,
						CpuFree:          2.0,
					},
				}
				policy = numa_aware_policy.NewNumaAwarePolicy(mockCache).(*numa_aware_policy.NumaAwarePolicy)

				ctx := createContainerContext(cgroupGuaranteed, "1000m", "1000m", "1Gi", "1Gi")

				alloc, err := policy.PreCreateContainerHook(ctx)
				Expect(err).NotTo(HaveOccurred())
				Expect(alloc).NotTo(BeNil())
				Expect(alloc.Resources.CpusetCpus).To(Equal("0-3")) // Should allocate to node 0 (less used)
			})

			It("should allocate CPU set for Burstable QOS to node with less usage", func() {
				// Setup mock cache with node resources
				mockCache.nodeResources = []cache.NumaNodeResources{
					{
						CpuTotal:         4.0,
						CpuUsed:          3.0,
						CpuUsedByRequest: 2.5,
						CpuFree:          1.0,
					},
					{
						CpuTotal:         4.0,
						CpuUsed:          1.5,
						CpuUsedByRequest: 1.0,
						CpuFree:          2.5,
					},
				}
				policy = numa_aware_policy.NewNumaAwarePolicy(mockCache).(*numa_aware_policy.NumaAwarePolicy)

				ctx := createContainerContext(cgroupBurstable, "500m", "1000m", "512Mi", "1Gi")

				alloc, err := policy.PreCreateContainerHook(ctx)
				Expect(err).NotTo(HaveOccurred())
				Expect(alloc).NotTo(BeNil())
				Expect(alloc.Resources.CpusetCpus).To(Equal("4-7")) // Should allocate to node 1 (less used)
			})
		})

		Context("with resource constraints", func() {
			It("should return nil when CPU request exceeds single node capacity", func() {
				mockCache.nodeResources = []cache.NumaNodeResources{
					{
						CpuTotal:         4.0,
						CpuUsed:          1.0,
						CpuUsedByRequest: 0.5,
						CpuFree:          3.0,
					},
				}
				policy = numa_aware_policy.NewNumaAwarePolicy(mockCache).(*numa_aware_policy.NumaAwarePolicy)

				ctx := createContainerContext(cgroupGuaranteed, "5000m", "5000m", "1Gi", "1Gi")

				alloc, err := policy.PreCreateContainerHook(ctx)
				Expect(err).NotTo(HaveOccurred())
				Expect(alloc).To(BeNil()) // Should return nil when CPU exceeds node capacity
			})

			It("should return nil when no node has enough capacity", func() {
				mockCache.nodeResources = []cache.NumaNodeResources{
					{
						CpuTotal:         4.0,
						CpuUsed:          3.5,
						CpuUsedByRequest: 3.8, // Almost full by request
						CpuFree:          0.5,
					},
					{
						CpuTotal:         4.0,
						CpuUsed:          3.0,
						CpuUsedByRequest: 3.9, // Almost full by request
						CpuFree:          1.0,
					},
				}
				policy = numa_aware_policy.NewNumaAwarePolicy(mockCache).(*numa_aware_policy.NumaAwarePolicy)

				ctx := createContainerContext(cgroupGuaranteed, "500m", "500m", "512Mi", "512Mi")

				alloc, err := policy.PreCreateContainerHook(ctx)
				Expect(err).NotTo(HaveOccurred())
				Expect(alloc).To(BeNil()) // Should return nil when no node has enough capacity
			})
		})
	})

	Describe("AllocateCPUSet", func() {
		BeforeEach(func() {
			policy = numa_aware_policy.NewNumaAwarePolicy(mockCache).(*numa_aware_policy.NumaAwarePolicy)
		})

		Context("with nil inputs", func() {
			It("should return nil with nil request", func() {
				alloc := policy.AllocateCPUSet(nil, createResourceList("1000m", "1Gi"))
				Expect(alloc).To(BeNil())
			})

			It("should return nil with nil limit", func() {
				alloc := policy.AllocateCPUSet(createResourceList("1000m", "1Gi"), nil)
				Expect(alloc).To(BeNil())
			})

			It("should return nil with both nil", func() {
				alloc := policy.AllocateCPUSet(nil, nil)
				Expect(alloc).To(BeNil())
			})
		})

		Context("with empty node resources", func() {
			It("should return nil when no nodes are available", func() {
				mockCache.nodeResources = []cache.NumaNodeResources{} // Empty node resources
				policy = numa_aware_policy.NewNumaAwarePolicy(mockCache).(*numa_aware_policy.NumaAwarePolicy)

				request := createResourceList("1000m", "1Gi")
				limit := createResourceList("1000m", "1Gi")

				alloc := policy.AllocateCPUSet(request, limit)
				Expect(alloc).To(BeNil()) // Should return nil when no nodes available
			})
		})
	})
})
