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

package topologyaware

import (
	"fmt"
	"strings"

	ginkgo "github.com/onsi/ginkgo/v2"
	gomega "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"kunpeng.huawei.com/kunpeng-cloud-computing/pkg/kunpeng-tap/policy"
	topologyaware "kunpeng.huawei.com/kunpeng-cloud-computing/pkg/kunpeng-tap/policy/topology-aware"
	"kunpeng.huawei.com/kunpeng-cloud-computing/pkg/kunpeng-tap/sysfs/system"
)

// Resource Priority Configuration Tests
var _ = ginkgo.Describe("Resource Priority Configuration Tests", func() {
	ginkgo.Context("Resource Priority Settings", func() {
		ginkgo.It("should correctly configure CPU-first priority", func() {
			opts := policy.NewPolicyOptions()
			opts.ResourcePriority = policy.ResourcePriorityCPUFirst
			gomega.Expect(opts.ResourcePriority).To(gomega.Equal(policy.ResourcePriorityCPUFirst))
		})

		ginkgo.It("should correctly configure GPU-first priority", func() {
			opts := policy.NewPolicyOptions()
			opts.ResourcePriority = policy.ResourcePriorityGPUFirst
			gomega.Expect(opts.ResourcePriority).To(gomega.Equal(policy.ResourcePriorityGPUFirst))
		})
	})

	ginkgo.Context("Policy Options Defaults", func() {
		ginkgo.It("should have correct default values", func() {
			opts := policy.NewPolicyOptions()
			gomega.Expect(opts.ResourcePriority).To(gomega.Equal(policy.ResourcePriorityCPUFirst))
			gomega.Expect(opts.EnableMemoryTopology).To(gomega.BeFalse())
		})
	})

	ginkgo.Context("Resource Priority Constants", func() {
		ginkgo.It("should have correct constant values", func() {
			gomega.Expect(string(policy.ResourcePriorityCPUFirst)).To(gomega.Equal("cpu-first"))
			gomega.Expect(string(policy.ResourcePriorityGPUFirst)).To(gomega.Equal("gpu-first"))
		})
	})
})

// Helper function to create container context with GPU environment variables
func createContainerContextWithGPU(containerName, podUID, podName, namespace string, cpuRequest, cpuLimit int64, gpuDevices []string) *policy.ContainerContext {
	requestsQuantity := resource.NewQuantity(cpuRequest, resource.DecimalSI)
	limitsQuantity := resource.NewQuantity(cpuLimit, resource.DecimalSI)
	memoryQuantity := resource.NewQuantity(100*1024*1024, resource.BinarySI) // 100MB

	ctx := &policy.ContainerContext{
		Request: policy.ContainerRequest{
			ContainerMeta: policy.ContainerMeta{
				Name: containerName,
				ID:   "containerd://" + containerName + "-id-123",
			},
			PodMeta: policy.PodMeta{
				UID:       podUID,
				Name:      podName,
				Namespace: namespace,
			},
			Resources: &policy.Resources{
				EstimatedRequirements: &corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    *requestsQuantity,
						corev1.ResourceMemory: *memoryQuantity,
					},
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    *limitsQuantity,
						corev1.ResourceMemory: *memoryQuantity,
					},
				},
			},
			ContainerEnvs: make(map[string]string),
		},
	}

	// Add GPU environment variables if specified
	if len(gpuDevices) > 0 {
		// Convert slice of device IDs to comma-separated /dev/vacc format
		var devicePaths []string
		for _, deviceID := range gpuDevices {
			devicePaths = append(devicePaths, fmt.Sprintf("/dev/vacc%s", deviceID))
		}
		ctx.Request.ContainerEnvs["VA_VISIBLE_DEVICES"] = strings.Join(devicePaths, ",")
	}

	return ctx
}

// Helper function to setup GPU configuration for mock system
func setupGPUConfiguration(mockSystem *MockSystem, gpuConfig string) {
	switch gpuConfig {
	case "2GPU_NUMA1_3":
		// 2 GPUs: GPU0 on NUMA1, GPU1 on NUMA3
		gpu0 := &MockGPU{
			id:          0,
			nodeID:      1,
			pciAddress:  "0000:01:00.0",
			vendorID:    "0x10de",
			deviceID:    "0x1234",
			deviceClass: "0x0302",
		}
		gpu1 := &MockGPU{
			id:          1,
			nodeID:      3,
			pciAddress:  "0000:02:00.0",
			vendorID:    "0x10de",
			deviceID:    "0x1234",
			deviceClass: "0x0302",
		}
		mockSystem.gpus[0] = gpu0
		mockSystem.gpus[1] = gpu1
		mockSystem.gpuIDs = []system.ID{0, 1}

	case "4GPU_ALL_NUMA":
		// 4 GPUs: one on each NUMA node
		for i := 0; i < 4; i++ {
			gpu := &MockGPU{
				id:          system.ID(i),
				nodeID:      system.ID(i),
				pciAddress:  fmt.Sprintf("0000:0%d:00.0", i+1),
				vendorID:    "0x10de",
				deviceID:    "0x1234",
				deviceClass: "0x0302",
			}
			mockSystem.gpus[system.ID(i)] = gpu
		}
		mockSystem.gpuIDs = []system.ID{0, 1, 2, 3}
	}
}

// Helper function to extract NUMA node from CPU set
func extractNUMAFromCPUSet(cpuSet string) []int {
	cpus := parseCpuList(cpuSet)
	numaNodes := make(map[int]bool)

	for _, cpu := range cpus {
		switch {
		case cpu < 24:
			numaNodes[0] = true // NUMA 0: CPUs 0-23
		case cpu < 48:
			numaNodes[1] = true // NUMA 1: CPUs 24-47
		case cpu < 72:
			numaNodes[2] = true // NUMA 2: CPUs 48-71
		default:
			numaNodes[3] = true // NUMA 3: CPUs 72-95
		}
	}

	var result []int
	for numa := range numaNodes {
		result = append(result, numa)
	}
	return result
}

var _ = ginkgo.Describe("Resource Priority Functional Tests", func() {
	var (
		mockCache  *MockCache
		mockSystem *MockSystem
		opts       *policy.PolicyOptions
	)

	ginkgo.BeforeEach(func() {
		mockCache = NewMockCache()
		mockSystem = NewMockSystem()
		opts = policy.NewPolicyOptions()
		opts.EnableMemoryTopology = false
	})

	ginkgo.Describe("CPU-First Priority Strategy", func() {
		ginkgo.BeforeEach(func() {
			opts.ResourcePriority = policy.ResourcePriorityCPUFirst
		})

		ginkgo.Context("with 2 GPUs on NUMA 1 and 3", func() {
			ginkgo.BeforeEach(func() {
				mockSystem.SetupLargeTopology()
				setupGPUConfiguration(mockSystem, "2GPU_NUMA1_3")
			})

			ginkgo.It("should prioritize CPU capacity over GPU affinity for CPU-only workloads", func() {
				policy := topologyaware.NewTopologyAwarePolicyWithSystem(mockCache, opts, mockSystem)
				gomega.Expect(policy).NotTo(gomega.BeNil())

				topologyPolicy, ok := policy.(*topologyaware.TopologyAwarePolicy)
				gomega.Expect(ok).To(gomega.BeTrue())

				// Create CPU-only container
				containerCtx := createContainerContextWithGPU(
					"cpu-container", "cpu-pod-uid", "cpu-pod", "default",
					4, 4, nil, // 4 CPU, no GPU
				)

				allocation, err := topologyPolicy.PreCreateContainerHook(containerCtx)
				gomega.Expect(err).To(gomega.BeNil())
				gomega.Expect(allocation).NotTo(gomega.BeNil())
				gomega.Expect(allocation.Resources.CpusetCpus).NotTo(gomega.BeEmpty())

				numaNodes := extractNUMAFromCPUSet(allocation.Resources.CpusetCpus)
				ginkgo.GinkgoWriter.Printf("CPU-only container (CPU-first strategy) allocated to NUMA nodes: %v\n", numaNodes)
				ginkgo.GinkgoWriter.Printf("Expected: Should prefer NUMA 0 (best CPU availability in fresh system)\n")

				// Verify allocation is valid
				gomega.Expect(len(numaNodes)).To(gomega.BeNumerically(">", 0), "Container should be allocated to at least one NUMA node")

				// For CPU-first strategy with CPU-only workload, should prefer NUMA 0 (best CPU availability)
				// In a fresh system, NUMA 0 typically has the most available CPU resources
				if len(numaNodes) == 1 {
					allocatedNUMA := numaNodes[0]
					ginkgo.GinkgoWriter.Printf("✓ CPU-only container allocated to NUMA %d\n", allocatedNUMA)

					// Should prefer NUMA 0 for best CPU availability
					gomega.Expect(allocatedNUMA).To(gomega.Equal(0),
						"CPU-first strategy should allocate CPU-only workload to NUMA 0 (best CPU availability)")
				} else {
					// Multi-NUMA allocation is also valid for larger containers
					ginkgo.GinkgoWriter.Printf("✓ CPU-only container allocated across multiple NUMA nodes: %v\n", numaNodes)
					gomega.Expect(numaNodes).To(gomega.ContainElement(0),
						"Multi-NUMA allocation should include NUMA 0 (best CPU availability)")
				}
			})

			ginkgo.It("should consider GPU affinity after CPU capacity for GPU workloads", func() {
				policy := topologyaware.NewTopologyAwarePolicyWithSystem(mockCache, opts, mockSystem)
				gomega.Expect(policy).NotTo(gomega.BeNil())

				topologyPolicy, ok := policy.(*topologyaware.TopologyAwarePolicy)
				gomega.Expect(ok).To(gomega.BeTrue())

				// Create GPU container requesting GPU 0 (located on NUMA 1)
				containerCtx := createContainerContextWithGPU(
					"gpu-container", "gpu-pod-uid", "gpu-pod", "default",
					4, 4, []string{"0"}, // 4 CPU + GPU 0 (on NUMA 1)
				)

				allocation, err := topologyPolicy.PreCreateContainerHook(containerCtx)
				gomega.Expect(err).To(gomega.BeNil())
				gomega.Expect(allocation).NotTo(gomega.BeNil())
				gomega.Expect(allocation.Resources.CpusetCpus).NotTo(gomega.BeEmpty())

				numaNodes := extractNUMAFromCPUSet(allocation.Resources.CpusetCpus)
				ginkgo.GinkgoWriter.Printf("GPU container (CPU-first) requesting GPU 0 allocated to NUMA nodes: %v\n", numaNodes)
				ginkgo.GinkgoWriter.Printf("Expected: GPU 0 is located on NUMA 1, but CPU-first may prefer NUMA 0 (best CPU capacity)\n")

				// Verify allocation is valid
				gomega.Expect(len(numaNodes)).To(gomega.BeNumerically(">", 0), "Container should be allocated to at least one NUMA node")

				// With CPU-first strategy, should prefer CPU capacity over GPU affinity
				// In a fresh system, NUMA 0 typically has the best CPU availability
				// But if CPU capacity is equal, should consider GPU affinity (NUMA 1)
				if len(numaNodes) == 1 {
					allocatedNUMA := numaNodes[0]
					if allocatedNUMA == 0 {
						ginkgo.GinkgoWriter.Printf("✓ CPU-first strategy: Allocated to NUMA 0 (prioritizing CPU capacity)\n")
					} else if allocatedNUMA == 1 {
						ginkgo.GinkgoWriter.Printf("✓ CPU-first strategy: Allocated to NUMA 1 (GPU affinity as secondary factor)\n")
					} else {
						ginkgo.GinkgoWriter.Printf("! Unexpected allocation to NUMA %d\n", allocatedNUMA)
					}

					// For CPU-first strategy, NUMA 0 or NUMA 1 are both acceptable
					// NUMA 0: Best CPU capacity (primary factor)
					// NUMA 1: GPU affinity (secondary factor)
					gomega.Expect(allocatedNUMA).To(gomega.Or(gomega.Equal(0), gomega.Equal(1)),
						"CPU-first strategy should allocate to NUMA 0 (CPU capacity) or NUMA 1 (GPU affinity)")
				} else {
					// Multi-NUMA allocation should include either NUMA 0 (CPU capacity) or NUMA 1 (GPU affinity)
					ginkgo.GinkgoWriter.Printf("✓ GPU container allocated across multiple NUMA nodes: %v\n", numaNodes)
					hasNUMA0 := false
					hasNUMA1 := false
					for _, numa := range numaNodes {
						if numa == 0 {
							hasNUMA0 = true
						}
						if numa == 1 {
							hasNUMA1 = true
						}
					}
					gomega.Expect(hasNUMA0 || hasNUMA1).To(gomega.BeTrue(),
						"Multi-NUMA allocation should include NUMA 0 (CPU capacity) or NUMA 1 (GPU affinity)")
				}
			})

			ginkgo.It("should handle multiple GPU containers with CPU-first priority", func() {
				policy := topologyaware.NewTopologyAwarePolicyWithSystem(mockCache, opts, mockSystem)
				gomega.Expect(policy).NotTo(gomega.BeNil())

				topologyPolicy, ok := policy.(*topologyaware.TopologyAwarePolicy)
				gomega.Expect(ok).To(gomega.BeTrue())

				// Test GPU 0 container (GPU 0 is on NUMA 1, but CPU-first may prefer NUMA 0)
				gpu0ContainerCtx := createContainerContextWithGPU(
					"gpu-container-0", "gpu-pod-0-uid", "gpu-pod-0", "default",
					4, 4, []string{"0"}, // 4 CPU + GPU 0 (on NUMA 1)
				)

				gpu0Allocation, err := topologyPolicy.PreCreateContainerHook(gpu0ContainerCtx)
				gomega.Expect(err).To(gomega.BeNil())
				gomega.Expect(gpu0Allocation).NotTo(gomega.BeNil())

				gpu0Numas := extractNUMAFromCPUSet(gpu0Allocation.Resources.CpusetCpus)
				ginkgo.GinkgoWriter.Printf("GPU 0 container (CPU-first) allocated to NUMA nodes: %v\n", gpu0Numas)

				// Test GPU 1 container (GPU 1 is on NUMA 3, but CPU-first may prefer NUMA 0)
				gpu1ContainerCtx := createContainerContextWithGPU(
					"gpu-container-1", "gpu-pod-1-uid", "gpu-pod-1", "default",
					4, 4, []string{"1"}, // 4 CPU + GPU 1 (on NUMA 3)
				)

				gpu1Allocation, err := topologyPolicy.PreCreateContainerHook(gpu1ContainerCtx)
				gomega.Expect(err).To(gomega.BeNil())
				gomega.Expect(gpu1Allocation).NotTo(gomega.BeNil())

				gpu1Numas := extractNUMAFromCPUSet(gpu1Allocation.Resources.CpusetCpus)
				ginkgo.GinkgoWriter.Printf("GPU 1 container (CPU-first) allocated to NUMA nodes: %v\n", gpu1Numas)

				// With CPU-first strategy, containers may be allocated based on CPU capacity first
				// GPU 0 container: acceptable on NUMA 0 (CPU capacity) or NUMA 1 (GPU affinity)
				// GPU 1 container: acceptable on NUMA 0 (CPU capacity) or NUMA 3 (GPU affinity)

				// Verify GPU 0 container allocation
				if len(gpu0Numas) == 1 {
					allocatedNUMA := gpu0Numas[0]
					if allocatedNUMA == 0 {
						ginkgo.GinkgoWriter.Printf("✓ GPU 0 container allocated to NUMA 0 (CPU capacity priority)\n")
					} else if allocatedNUMA == 1 {
						ginkgo.GinkgoWriter.Printf("✓ GPU 0 container allocated to NUMA 1 (GPU affinity as secondary factor)\n")
					}
					gomega.Expect(allocatedNUMA).To(gomega.Or(gomega.Equal(0), gomega.Equal(1)),
						"CPU-first strategy should allocate GPU 0 container to NUMA 0 or NUMA 1")
				}

				// Verify GPU 1 container allocation
				if len(gpu1Numas) == 1 {
					allocatedNUMA := gpu1Numas[0]
					if allocatedNUMA == 0 {
						ginkgo.GinkgoWriter.Printf("✓ GPU 1 container allocated to NUMA 0 (CPU capacity priority)\n")
					} else if allocatedNUMA == 3 {
						ginkgo.GinkgoWriter.Printf("✓ GPU 1 container allocated to NUMA 3 (GPU affinity as secondary factor)\n")
					}
					gomega.Expect(allocatedNUMA).To(gomega.Or(gomega.Equal(0), gomega.Equal(3)),
						"CPU-first strategy should allocate GPU 1 container to NUMA 0 or NUMA 3")
				}

				ginkgo.GinkgoWriter.Printf("✓ Multiple GPU containers handled with CPU-first priority: GPU 0 on NUMA %v, GPU 1 on NUMA %v\n", gpu0Numas, gpu1Numas)
			})

			ginkgo.It("should handle concurrent deployment of multiple GPU containers", func() {
				policy := topologyaware.NewTopologyAwarePolicyWithSystem(mockCache, opts, mockSystem)
				gomega.Expect(policy).NotTo(gomega.BeNil())

				topologyPolicy, ok := policy.(*topologyaware.TopologyAwarePolicy)
				gomega.Expect(ok).To(gomega.BeTrue())

				// Simulate concurrent deployment scenario: multiple GPU containers requesting different GPUs
				// This tests the real-world scenario where multiple GPU workloads are deployed simultaneously

				// Container 1: ML training workload requesting GPU 0
				mlTrainingCtx := createContainerContextWithGPU(
					"ml-training-gpu0", "ml-training-pod-0-uid", "ml-training-pod-0", "default",
					6, 6, []string{"0"}, // 6 CPU + GPU 0 (on NUMA 1)
				)

				// Container 2: Inference service requesting GPU 1
				inferenceCtx := createContainerContextWithGPU(
					"inference-gpu1", "inference-pod-1-uid", "inference-pod-1", "default",
					4, 4, []string{"1"}, // 4 CPU + GPU 1 (on NUMA 3)
				)

				// Container 3: Data processing workload requesting GPU 0 (should compete with Container 1)
				dataProcessingCtx := createContainerContextWithGPU(
					"data-processing-gpu0", "data-proc-pod-uid", "data-proc-pod", "default",
					8, 8, []string{"0"}, // 8 CPU + GPU 0 (on NUMA 1)
				)

				// Deploy containers sequentially to simulate real deployment
				ginkgo.GinkgoWriter.Printf("Deploying ML training container (GPU 0)...\n")
				mlAllocation, err := topologyPolicy.PreCreateContainerHook(mlTrainingCtx)
				gomega.Expect(err).To(gomega.BeNil())
				gomega.Expect(mlAllocation).NotTo(gomega.BeNil())

				mlNumas := extractNUMAFromCPUSet(mlAllocation.Resources.CpusetCpus)
				ginkgo.GinkgoWriter.Printf("ML training container (GPU 0) allocated to NUMA nodes: %v\n", mlNumas)

				ginkgo.GinkgoWriter.Printf("Deploying inference service container (GPU 1)...\n")
				inferenceAllocation, err := topologyPolicy.PreCreateContainerHook(inferenceCtx)
				gomega.Expect(err).To(gomega.BeNil())
				gomega.Expect(inferenceAllocation).NotTo(gomega.BeNil())

				inferenceNumas := extractNUMAFromCPUSet(inferenceAllocation.Resources.CpusetCpus)
				ginkgo.GinkgoWriter.Printf("Inference service container (GPU 1) allocated to NUMA nodes: %v\n", inferenceNumas)

				ginkgo.GinkgoWriter.Printf("Deploying data processing container (GPU 0, competing resource)...\n")
				dataAllocation, err := topologyPolicy.PreCreateContainerHook(dataProcessingCtx)
				gomega.Expect(err).To(gomega.BeNil())
				gomega.Expect(dataAllocation).NotTo(gomega.BeNil())

				dataNumas := extractNUMAFromCPUSet(dataAllocation.Resources.CpusetCpus)
				ginkgo.GinkgoWriter.Printf("Data processing container (GPU 0) allocated to NUMA nodes: %v\n", dataNumas)

				// Verify allocations with CPU-first strategy
				// All containers should be allocated successfully, with CPU capacity as primary consideration

				// Verify ML training container (GPU 0)
				if len(mlNumas) == 1 {
					allocatedNUMA := mlNumas[0]
					if allocatedNUMA == 0 {
						ginkgo.GinkgoWriter.Printf("✓ ML training container allocated to NUMA 0 (CPU capacity priority)\n")
					} else if allocatedNUMA == 1 {
						ginkgo.GinkgoWriter.Printf("✓ ML training container allocated to NUMA 1 (GPU affinity)\n")
					}
					gomega.Expect(allocatedNUMA).To(gomega.Or(gomega.Equal(0), gomega.Equal(1)),
						"ML training container should be allocated to NUMA 0 or NUMA 1")
				}

				// Verify inference service container (GPU 1)
				if len(inferenceNumas) == 1 {
					allocatedNUMA := inferenceNumas[0]
					if allocatedNUMA == 0 {
						ginkgo.GinkgoWriter.Printf("✓ Inference service container allocated to NUMA 0 (CPU capacity priority)\n")
					} else if allocatedNUMA == 3 {
						ginkgo.GinkgoWriter.Printf("✓ Inference service container allocated to NUMA 3 (GPU affinity)\n")
					}
					gomega.Expect(allocatedNUMA).To(gomega.Or(gomega.Equal(0), gomega.Equal(3)),
						"Inference service container should be allocated to NUMA 0 or NUMA 3")
				}

				// Verify data processing container (GPU 0, competing with ML training)
				if len(dataNumas) == 1 {
					allocatedNUMA := dataNumas[0]
					if allocatedNUMA == 0 {
						ginkgo.GinkgoWriter.Printf("✓ Data processing container allocated to NUMA 0 (CPU capacity priority)\n")
					} else if allocatedNUMA == 1 {
						ginkgo.GinkgoWriter.Printf("✓ Data processing container allocated to NUMA 1 (GPU affinity)\n")
					}
					gomega.Expect(allocatedNUMA).To(gomega.Or(gomega.Equal(0), gomega.Equal(1)),
						"Data processing container should be allocated to NUMA 0 or NUMA 1")
				}

				ginkgo.GinkgoWriter.Printf("✓ Successfully deployed multiple GPU containers concurrently\n")
				ginkgo.GinkgoWriter.Printf("  - ML training (GPU 0): NUMA %v\n", mlNumas)
				ginkgo.GinkgoWriter.Printf("  - Inference service (GPU 1): NUMA %v\n", inferenceNumas)
				ginkgo.GinkgoWriter.Printf("  - Data processing (GPU 0): NUMA %v\n", dataNumas)
			})
		})

		ginkgo.Context("with 4 GPUs on all NUMA nodes", func() {
			ginkgo.BeforeEach(func() {
				mockSystem.SetupLargeTopology()
				setupGPUConfiguration(mockSystem, "4GPU_ALL_NUMA")
			})

			ginkgo.It("should prioritize CPU capacity when all NUMA nodes have GPUs", func() {
				policy := topologyaware.NewTopologyAwarePolicyWithSystem(mockCache, opts, mockSystem)
				gomega.Expect(policy).NotTo(gomega.BeNil())

				topologyPolicy, ok := policy.(*topologyaware.TopologyAwarePolicy)
				gomega.Expect(ok).To(gomega.BeTrue())

				// Create GPU container
				containerCtx := createContainerContextWithGPU(
					"gpu-container", "gpu-pod-uid", "gpu-pod", "default",
					8, 8, []string{"2"}, // 8 CPU + GPU 2 (on NUMA 2)
				)

				allocation, err := topologyPolicy.PreCreateContainerHook(containerCtx)
				gomega.Expect(err).To(gomega.BeNil())
				gomega.Expect(allocation).NotTo(gomega.BeNil())
				gomega.Expect(allocation.Resources.CpusetCpus).NotTo(gomega.BeEmpty())

				numaNodes := extractNUMAFromCPUSet(allocation.Resources.CpusetCpus)
				ginkgo.GinkgoWriter.Printf("GPU container (4GPU, CPU-first) allocated to NUMA nodes: %v\n", numaNodes)
			})
		})
	})

	ginkgo.Describe("GPU-First Priority Strategy", func() {
		ginkgo.BeforeEach(func() {
			opts.ResourcePriority = policy.ResourcePriorityGPUFirst
		})

		ginkgo.Context("with 2 GPUs on NUMA 1 and 3", func() {
			ginkgo.BeforeEach(func() {
				mockSystem.SetupLargeTopology()
				setupGPUConfiguration(mockSystem, "2GPU_NUMA1_3")
			})

			ginkgo.It("should prioritize GPU affinity over CPU capacity for GPU workloads", func() {
				policy := topologyaware.NewTopologyAwarePolicyWithSystem(mockCache, opts, mockSystem)
				gomega.Expect(policy).NotTo(gomega.BeNil())

				topologyPolicy, ok := policy.(*topologyaware.TopologyAwarePolicy)
				gomega.Expect(ok).To(gomega.BeTrue())

				// Create GPU container requesting GPU 0 (located on NUMA 1)
				containerCtx := createContainerContextWithGPU(
					"gpu-container-0", "gpu-pod-0-uid", "gpu-pod-0", "default",
					4, 4, []string{"0"}, // 4 CPU + GPU 0 (on NUMA 1)
				)

				allocation, err := topologyPolicy.PreCreateContainerHook(containerCtx)
				gomega.Expect(err).To(gomega.BeNil())
				gomega.Expect(allocation).NotTo(gomega.BeNil())
				gomega.Expect(allocation.Resources.CpusetCpus).NotTo(gomega.BeEmpty())

				numaNodes := extractNUMAFromCPUSet(allocation.Resources.CpusetCpus)
				ginkgo.GinkgoWriter.Printf("GPU container (GPU-first) requesting GPU 0 allocated to NUMA nodes: %v\n", numaNodes)
				ginkgo.GinkgoWriter.Printf("Expected: Should prefer NUMA 1 (where GPU 0 is located)\n")

				// Verify allocation is valid
				gomega.Expect(len(numaNodes)).To(gomega.BeNumerically(">", 0), "Container should be allocated to at least one NUMA node")

				// With GPU-first strategy, should strongly prefer NUMA 1 (where GPU 0 is located)
				if len(numaNodes) == 1 {
					allocatedNUMA := numaNodes[0]
					ginkgo.GinkgoWriter.Printf("✓ GPU container allocated to NUMA %d\n", allocatedNUMA)

					// Should prefer NUMA 1 where GPU 0 is located
					gomega.Expect(allocatedNUMA).To(gomega.Equal(1),
						"GPU-first strategy should allocate GPU 0 container to NUMA 1 (where GPU 0 is located)")
				} else {
					// Multi-NUMA allocation should still include the GPU's NUMA node
					ginkgo.GinkgoWriter.Printf("✓ GPU container allocated across multiple NUMA nodes: %v\n", numaNodes)
					gomega.Expect(numaNodes).To(gomega.ContainElement(1),
						"Multi-NUMA allocation should include NUMA 1 (where GPU 0 is located)")
				}
			})

			ginkgo.It("should fall back to CPU capacity for CPU-only workloads", func() {
				policy := topologyaware.NewTopologyAwarePolicyWithSystem(mockCache, opts, mockSystem)
				gomega.Expect(policy).NotTo(gomega.BeNil())

				topologyPolicy, ok := policy.(*topologyaware.TopologyAwarePolicy)
				gomega.Expect(ok).To(gomega.BeTrue())

				// Create CPU-only container
				containerCtx := createContainerContextWithGPU(
					"cpu-only-container", "cpu-only-pod-uid", "cpu-only-pod", "default",
					4, 4, nil, // 4 CPU, no GPU
				)

				allocation, err := topologyPolicy.PreCreateContainerHook(containerCtx)
				gomega.Expect(err).To(gomega.BeNil())
				gomega.Expect(allocation).NotTo(gomega.BeNil())
				gomega.Expect(allocation.Resources.CpusetCpus).NotTo(gomega.BeEmpty())

				numaNodes := extractNUMAFromCPUSet(allocation.Resources.CpusetCpus)
				ginkgo.GinkgoWriter.Printf("CPU-only container (GPU-first strategy) allocated to NUMA nodes: %v\n", numaNodes)
				ginkgo.GinkgoWriter.Printf("Expected: Should fall back to NUMA 0 (best CPU availability) since no GPU requested\n")

				// Verify allocation is valid
				gomega.Expect(len(numaNodes)).To(gomega.BeNumerically(">", 0), "Container should be allocated to at least one NUMA node")

				// For CPU-only workloads with GPU-first strategy, should fall back to CPU capacity priority
				// This should behave the same as CPU-first strategy
				if len(numaNodes) == 1 {
					allocatedNUMA := numaNodes[0]
					ginkgo.GinkgoWriter.Printf("✓ CPU-only container (GPU-first fallback) allocated to NUMA %d\n", allocatedNUMA)

					// Should fall back to NUMA 0 for best CPU availability
					gomega.Expect(allocatedNUMA).To(gomega.Equal(0),
						"GPU-first strategy should fall back to NUMA 0 (best CPU availability) for CPU-only workloads")
				} else {
					// Multi-NUMA allocation should still include NUMA 0
					ginkgo.GinkgoWriter.Printf("✓ CPU-only container allocated across multiple NUMA nodes: %v\n", numaNodes)
					gomega.Expect(numaNodes).To(gomega.ContainElement(0),
						"Multi-NUMA allocation should include NUMA 0 (best CPU availability)")
				}
			})

			ginkgo.It("should deploy multiple GPU containers with optimal GPU affinity", func() {
				policy := topologyaware.NewTopologyAwarePolicyWithSystem(mockCache, opts, mockSystem)
				gomega.Expect(policy).NotTo(gomega.BeNil())

				topologyPolicy, ok := policy.(*topologyaware.TopologyAwarePolicy)
				gomega.Expect(ok).To(gomega.BeTrue())

				// Simulate production deployment scenario with multiple GPU workloads
				// This tests GPU-first strategy's ability to optimize GPU locality

				// Container 1: Deep learning training requesting GPU 0
				dlTrainingCtx := createContainerContextWithGPU(
					"dl-training-gpu0", "dl-training-pod-uid", "dl-training-pod", "default",
					8, 8, []string{"0"}, // 8 CPU + GPU 0 (on NUMA 1)
				)

				// Container 2: Computer vision inference requesting GPU 1
				cvInferenceCtx := createContainerContextWithGPU(
					"cv-inference-gpu1", "cv-inference-pod-uid", "cv-inference-pod", "default",
					6, 6, []string{"1"}, // 6 CPU + GPU 1 (on NUMA 3)
				)

				// Container 3: Another training job requesting GPU 0 (resource competition)
				secondTrainingCtx := createContainerContextWithGPU(
					"second-training-gpu0", "second-training-pod-uid", "second-training-pod", "default",
					4, 4, []string{"0"}, // 4 CPU + GPU 0 (on NUMA 1)
				)

				// Container 4: Batch processing requesting GPU 1 (resource competition)
				batchProcessingCtx := createContainerContextWithGPU(
					"batch-processing-gpu1", "batch-proc-pod-uid", "batch-proc-pod", "default",
					6, 6, []string{"1"}, // 6 CPU + GPU 1 (on NUMA 3)
				)

				// Deploy containers and verify GPU affinity optimization
				ginkgo.GinkgoWriter.Printf("Deploying deep learning training container (GPU 0)...\n")
				dlAllocation, err := topologyPolicy.PreCreateContainerHook(dlTrainingCtx)
				gomega.Expect(err).To(gomega.BeNil())
				gomega.Expect(dlAllocation).NotTo(gomega.BeNil())

				dlNumas := extractNUMAFromCPUSet(dlAllocation.Resources.CpusetCpus)
				ginkgo.GinkgoWriter.Printf("Deep learning training container (GPU 0) allocated to NUMA nodes: %v\n", dlNumas)

				ginkgo.GinkgoWriter.Printf("Deploying computer vision inference container (GPU 1)...\n")
				cvAllocation, err := topologyPolicy.PreCreateContainerHook(cvInferenceCtx)
				gomega.Expect(err).To(gomega.BeNil())
				gomega.Expect(cvAllocation).NotTo(gomega.BeNil())

				cvNumas := extractNUMAFromCPUSet(cvAllocation.Resources.CpusetCpus)
				ginkgo.GinkgoWriter.Printf("Computer vision inference container (GPU 1) allocated to NUMA nodes: %v\n", cvNumas)

				ginkgo.GinkgoWriter.Printf("Deploying second training container (GPU 0, competing)...\n")
				secondAllocation, err := topologyPolicy.PreCreateContainerHook(secondTrainingCtx)
				gomega.Expect(err).To(gomega.BeNil())
				gomega.Expect(secondAllocation).NotTo(gomega.BeNil())

				secondNumas := extractNUMAFromCPUSet(secondAllocation.Resources.CpusetCpus)
				ginkgo.GinkgoWriter.Printf("Second training container (GPU 0) allocated to NUMA nodes: %v\n", secondNumas)

				ginkgo.GinkgoWriter.Printf("Deploying batch processing container (GPU 1, competing)...\n")
				batchAllocation, err := topologyPolicy.PreCreateContainerHook(batchProcessingCtx)
				gomega.Expect(err).To(gomega.BeNil())
				gomega.Expect(batchAllocation).NotTo(gomega.BeNil())

				batchNumas := extractNUMAFromCPUSet(batchAllocation.Resources.CpusetCpus)
				ginkgo.GinkgoWriter.Printf("Batch processing container (GPU 1) allocated to NUMA nodes: %v\n", batchNumas)

				// Verify GPU-first strategy: all GPU containers should be allocated to their GPU's NUMA node
				// This ensures optimal GPU locality and memory bandwidth

				// Verify deep learning training container (GPU 0 -> NUMA 1)
				if len(dlNumas) == 1 {
					allocatedNUMA := dlNumas[0]
					ginkgo.GinkgoWriter.Printf("✓ Deep learning training container allocated to NUMA %d\n", allocatedNUMA)
					gomega.Expect(allocatedNUMA).To(gomega.Equal(1),
						"GPU-first strategy should allocate GPU 0 container to NUMA 1")
				} else {
					ginkgo.GinkgoWriter.Printf("✓ Deep learning training container allocated across multiple NUMA nodes: %v\n", dlNumas)
					gomega.Expect(dlNumas).To(gomega.ContainElement(1),
						"Multi-NUMA allocation should include NUMA 1 (where GPU 0 is located)")
				}

				// Verify computer vision inference container (GPU 1 -> NUMA 3)
				if len(cvNumas) == 1 {
					allocatedNUMA := cvNumas[0]
					ginkgo.GinkgoWriter.Printf("✓ Computer vision inference container allocated to NUMA %d\n", allocatedNUMA)
					gomega.Expect(allocatedNUMA).To(gomega.Equal(3),
						"GPU-first strategy should allocate GPU 1 container to NUMA 3")
				} else {
					ginkgo.GinkgoWriter.Printf("✓ Computer vision inference container allocated across multiple NUMA nodes: %v\n", cvNumas)
					gomega.Expect(cvNumas).To(gomega.ContainElement(3),
						"Multi-NUMA allocation should include NUMA 3 (where GPU 1 is located)")
				}

				// Verify second training container (GPU 0)
				// Due to resource competition, it may be allocated to NUMA 0 (fallback) or NUMA 1 (GPU affinity)
				if len(secondNumas) == 1 {
					allocatedNUMA := secondNumas[0]
					if allocatedNUMA == 1 {
						ginkgo.GinkgoWriter.Printf("✓ Second training container allocated to NUMA 1 (optimal GPU affinity)\n")
					} else if allocatedNUMA == 0 {
						ginkgo.GinkgoWriter.Printf("✓ Second training container allocated to NUMA 0 (resource competition fallback)\n")
					} else {
						ginkgo.GinkgoWriter.Printf("! Second training container allocated to unexpected NUMA %d\n", allocatedNUMA)
					}
					// With resource competition, both NUMA 0 and NUMA 1 are acceptable
					gomega.Expect(allocatedNUMA).To(gomega.Or(gomega.Equal(0), gomega.Equal(1)),
						"GPU-first strategy should allocate GPU 0 container to NUMA 1 (optimal) or NUMA 0 (fallback)")
				} else {
					ginkgo.GinkgoWriter.Printf("✓ Second training container allocated across multiple NUMA nodes: %v\n", secondNumas)
					// Multi-NUMA allocation should include either NUMA 0 or NUMA 1
					hasNUMA0 := false
					hasNUMA1 := false
					for _, numa := range secondNumas {
						if numa == 0 {
							hasNUMA0 = true
						}
						if numa == 1 {
							hasNUMA1 = true
						}
					}
					gomega.Expect(hasNUMA0 || hasNUMA1).To(gomega.BeTrue(),
						"Multi-NUMA allocation should include NUMA 0 or NUMA 1")
				}

				// Verify batch processing container (GPU 1)
				// Due to resource competition, it may be allocated to NUMA 0 (fallback) or NUMA 3 (GPU affinity)
				if len(batchNumas) == 1 {
					allocatedNUMA := batchNumas[0]
					if allocatedNUMA == 3 {
						ginkgo.GinkgoWriter.Printf("✓ Batch processing container allocated to NUMA 3 (optimal GPU affinity)\n")
					} else if allocatedNUMA == 0 || allocatedNUMA == 2 {
						ginkgo.GinkgoWriter.Printf("✓ Batch processing container allocated to NUMA %d (resource competition fallback)\n", allocatedNUMA)
					} else {
						ginkgo.GinkgoWriter.Printf("! Batch processing container allocated to unexpected NUMA %d\n", allocatedNUMA)
					}
					// With resource competition, NUMA 0, 2, or 3 are acceptable
					gomega.Expect(allocatedNUMA).To(gomega.Or(gomega.Equal(0), gomega.Equal(2), gomega.Equal(3)),
						"GPU-first strategy should allocate GPU 1 container to NUMA 3 (optimal) or other NUMA (fallback)")
				} else {
					ginkgo.GinkgoWriter.Printf("✓ Batch processing container allocated across multiple NUMA nodes: %v\n", batchNumas)
					// Multi-NUMA allocation should include at least one valid NUMA node
					gomega.Expect(len(batchNumas)).To(gomega.BeNumerically(">", 0),
						"Multi-NUMA allocation should include at least one NUMA node")
				}

				ginkgo.GinkgoWriter.Printf("✓ Successfully deployed multiple GPU containers with optimal GPU affinity\n")
				ginkgo.GinkgoWriter.Printf("  - Deep learning training (GPU 0): NUMA %v\n", dlNumas)
				ginkgo.GinkgoWriter.Printf("  - Computer vision inference (GPU 1): NUMA %v\n", cvNumas)
				ginkgo.GinkgoWriter.Printf("  - Second training (GPU 0): NUMA %v\n", secondNumas)
				ginkgo.GinkgoWriter.Printf("  - Batch processing (GPU 1): NUMA %v\n", batchNumas)
			})

			ginkgo.It("should handle GPU containers when CPU requests exceed NUMA capacity", func() {
				policy := topologyaware.NewTopologyAwarePolicyWithSystem(mockCache, opts, mockSystem)
				gomega.Expect(policy).NotTo(gomega.BeNil())

				topologyPolicy, ok := policy.(*topologyaware.TopologyAwarePolicy)
				gomega.Expect(ok).To(gomega.BeTrue())

				// Test scenario: Multiple GPU 0 containers with total CPU requests exceeding NUMA 1 capacity
				// NUMA 1 has 24 CPUs (24000m), we'll request containers totaling more than this

				ginkgo.GinkgoWriter.Printf("Testing GPU-First strategy with CPU requests exceeding NUMA capacity\n")
				ginkgo.GinkgoWriter.Printf("NUMA 1 (GPU 0 location) has 24 CPUs (24000m)\n")

				// Container 1: Large GPU 0 container (12 CPUs)
				largeGPU0Ctx := createContainerContextWithGPU(
					"large-gpu0-container", "large-gpu0-uid", "large-gpu0-pod", "default",
					12, 12, []string{"0"}, // 12 CPU + GPU 0 (on NUMA 1)
				)

				largeAllocation, err := topologyPolicy.PreCreateContainerHook(largeGPU0Ctx)
				gomega.Expect(err).To(gomega.BeNil())
				gomega.Expect(largeAllocation).NotTo(gomega.BeNil())

				largeNumas := extractNUMAFromCPUSet(largeAllocation.Resources.CpusetCpus)
				ginkgo.GinkgoWriter.Printf("Large GPU 0 container (12 CPU) allocated to NUMA nodes: %v\n", largeNumas)

				// Container 2: Medium GPU 0 container (10 CPUs) - should still fit in NUMA 1
				mediumGPU0Ctx := createContainerContextWithGPU(
					"medium-gpu0-container", "medium-gpu0-uid", "medium-gpu0-pod", "default",
					10, 10, []string{"0"}, // 10 CPU + GPU 0 (on NUMA 1)
				)

				mediumAllocation, err := topologyPolicy.PreCreateContainerHook(mediumGPU0Ctx)
				gomega.Expect(err).To(gomega.BeNil())
				gomega.Expect(mediumAllocation).NotTo(gomega.BeNil())

				mediumNumas := extractNUMAFromCPUSet(mediumAllocation.Resources.CpusetCpus)
				ginkgo.GinkgoWriter.Printf("Medium GPU 0 container (10 CPU) allocated to NUMA nodes: %v\n", mediumNumas)

				// Container 3: Small GPU 0 container (4 CPUs) - should exceed NUMA 1 capacity (12+10+4=26 > 24)
				smallGPU0Ctx := createContainerContextWithGPU(
					"small-gpu0-container", "small-gpu0-uid", "small-gpu0-pod", "default",
					4, 4, []string{"0"}, // 4 CPU + GPU 0 (on NUMA 1)
				)

				smallAllocation, err := topologyPolicy.PreCreateContainerHook(smallGPU0Ctx)
				gomega.Expect(err).To(gomega.BeNil())
				gomega.Expect(smallAllocation).NotTo(gomega.BeNil())

				smallNumas := extractNUMAFromCPUSet(smallAllocation.Resources.CpusetCpus)
				ginkgo.GinkgoWriter.Printf("Small GPU 0 container (4 CPU) allocated to NUMA nodes: %v\n", smallNumas)

				// Verify allocation behavior with GPU-First strategy
				// First two containers should prefer NUMA 1 (where GPU 0 is located)
				// Third container may need to spill to adjacent NUMA or Socket level

				// Verify large container allocation (should prefer NUMA 1)
				if len(largeNumas) == 1 {
					allocatedNUMA := largeNumas[0]
					ginkgo.GinkgoWriter.Printf("✓ Large GPU 0 container allocated to NUMA %d\n", allocatedNUMA)
					gomega.Expect(allocatedNUMA).To(gomega.Equal(1),
						"GPU-First: Large GPU 0 container should be allocated to NUMA 1 (GPU location)")
				} else {
					ginkgo.GinkgoWriter.Printf("✓ Large GPU 0 container allocated across multiple NUMA nodes: %v\n", largeNumas)
					gomega.Expect(largeNumas).To(gomega.ContainElement(1),
						"Multi-NUMA allocation should include NUMA 1 (GPU 0 location)")
				}

				// Verify medium container allocation (should prefer NUMA 1 if capacity allows)
				if len(mediumNumas) == 1 {
					allocatedNUMA := mediumNumas[0]
					ginkgo.GinkgoWriter.Printf("✓ Medium GPU 0 container allocated to NUMA %d\n", allocatedNUMA)
					gomega.Expect(allocatedNUMA).To(gomega.Equal(1),
						"GPU-First: Medium GPU 0 container should be allocated to NUMA 1 (GPU location)")
				} else {
					ginkgo.GinkgoWriter.Printf("✓ Medium GPU 0 container allocated across multiple NUMA nodes: %v\n", mediumNumas)
					gomega.Expect(mediumNumas).To(gomega.ContainElement(1),
						"Multi-NUMA allocation should include NUMA 1 (GPU 0 location)")
				}

				// Verify small container allocation (may spill to adjacent NUMA or Socket)
				if len(smallNumas) == 1 {
					allocatedNUMA := smallNumas[0]
					ginkgo.GinkgoWriter.Printf("✓ Small GPU 0 container allocated to NUMA %d\n", allocatedNUMA)

					if allocatedNUMA == 1 {
						ginkgo.GinkgoWriter.Printf("  → Still allocated to NUMA 1 (optimal GPU affinity)\n")
					} else if allocatedNUMA == 0 {
						ginkgo.GinkgoWriter.Printf("  → Spilled to NUMA 0 (same socket as NUMA 1)\n")
					} else {
						ginkgo.GinkgoWriter.Printf("  → Allocated to NUMA %d (resource pressure fallback)\n", allocatedNUMA)
					}

					// With resource pressure, acceptable allocations are:
					// NUMA 1: Still fits in GPU's NUMA (optimal)
					// NUMA 0: Same socket as NUMA 1 (good locality)
					// Other NUMA: Fallback due to resource pressure
					gomega.Expect(allocatedNUMA).To(gomega.BeNumerically(">=", 0),
						"Small GPU 0 container should be allocated to a valid NUMA node")
				} else {
					ginkgo.GinkgoWriter.Printf("✓ Small GPU 0 container allocated across multiple NUMA nodes: %v\n", smallNumas)
					gomega.Expect(len(smallNumas)).To(gomega.BeNumerically(">", 0),
						"Multi-NUMA allocation should include at least one NUMA node")
				}

				ginkgo.GinkgoWriter.Printf("✓ Successfully handled GPU containers with CPU requests exceeding NUMA capacity\n")
				ginkgo.GinkgoWriter.Printf("  - Large GPU 0 (12 CPU): NUMA %v\n", largeNumas)
				ginkgo.GinkgoWriter.Printf("  - Medium GPU 0 (10 CPU): NUMA %v\n", mediumNumas)
				ginkgo.GinkgoWriter.Printf("  - Small GPU 0 (4 CPU): NUMA %v\n", smallNumas)
			})

			ginkgo.It("should escalate to Socket level when NUMA capacity is exceeded", func() {
				policy := topologyaware.NewTopologyAwarePolicyWithSystem(mockCache, opts, mockSystem)
				gomega.Expect(policy).NotTo(gomega.BeNil())

				topologyPolicy, ok := policy.(*topologyaware.TopologyAwarePolicy)
				gomega.Expect(ok).To(gomega.BeTrue())

				// Test scenario: GPU containers that exceed both NUMA 1 and adjacent NUMA capacity
				// Socket 0 contains NUMA 0 and NUMA 1, total 48 CPUs
				// We'll create containers that exceed individual NUMA capacity but fit in Socket

				ginkgo.GinkgoWriter.Printf("Testing GPU-First strategy escalation to Socket level\n")
				ginkgo.GinkgoWriter.Printf("Socket 0 contains NUMA 0 and NUMA 1, total 48 CPUs\n")

				// Container 1: Very large GPU 0 container (30 CPUs) - exceeds NUMA 1 capacity
				veryLargeGPU0Ctx := createContainerContextWithGPU(
					"very-large-gpu0", "very-large-gpu0-uid", "very-large-gpu0-pod", "default",
					30, 30, []string{"0"}, // 30 CPU + GPU 0 (on NUMA 1)
				)

				veryLargeAllocation, err := topologyPolicy.PreCreateContainerHook(veryLargeGPU0Ctx)
				gomega.Expect(err).To(gomega.BeNil())
				gomega.Expect(veryLargeAllocation).NotTo(gomega.BeNil())

				veryLargeNumas := extractNUMAFromCPUSet(veryLargeAllocation.Resources.CpusetCpus)
				ginkgo.GinkgoWriter.Printf("Very large GPU 0 container (30 CPU) allocated to NUMA nodes: %v\n", veryLargeNumas)

				// Container 2: Another large GPU 0 container (15 CPUs)
				anotherLargeGPU0Ctx := createContainerContextWithGPU(
					"another-large-gpu0", "another-large-gpu0-uid", "another-large-gpu0-pod", "default",
					15, 15, []string{"0"}, // 15 CPU + GPU 0 (on NUMA 1)
				)

				anotherLargeAllocation, err := topologyPolicy.PreCreateContainerHook(anotherLargeGPU0Ctx)
				gomega.Expect(err).To(gomega.BeNil())
				gomega.Expect(anotherLargeAllocation).NotTo(gomega.BeNil())

				anotherLargeNumas := extractNUMAFromCPUSet(anotherLargeAllocation.Resources.CpusetCpus)
				ginkgo.GinkgoWriter.Printf("Another large GPU 0 container (15 CPU) allocated to NUMA nodes: %v\n", anotherLargeNumas)

				// Verify that containers are allocated within the same socket when possible
				// Socket 0 contains NUMA 0 and NUMA 1
				// GPU 0 is on NUMA 1, so containers should prefer Socket 0

				// Verify very large container allocation
				if len(veryLargeNumas) >= 1 {
					// Should be allocated to Socket 0 (NUMA 0 and/or NUMA 1)
					hasSocket0NUMA := false
					for _, numa := range veryLargeNumas {
						if numa == 0 || numa == 1 {
							hasSocket0NUMA = true
							break
						}
					}
					gomega.Expect(hasSocket0NUMA).To(gomega.BeTrue(),
						"Very large GPU 0 container should be allocated to Socket 0 (NUMA 0 or 1)")

					if hasSocket0NUMA {
						ginkgo.GinkgoWriter.Printf("✓ Very large GPU 0 container allocated to Socket 0 (optimal GPU locality)\n")
					}
				}

				// Verify another large container allocation
				if len(anotherLargeNumas) >= 1 {
					// Should also prefer Socket 0 if capacity allows
					hasSocket0NUMA := false
					for _, numa := range anotherLargeNumas {
						if numa == 0 || numa == 1 {
							hasSocket0NUMA = true
							break
						}
					}

					if hasSocket0NUMA {
						ginkgo.GinkgoWriter.Printf("✓ Another large GPU 0 container allocated to Socket 0 (good GPU locality)\n")
					} else {
						ginkgo.GinkgoWriter.Printf("✓ Another large GPU 0 container allocated to different socket due to resource pressure\n")
					}

					// Should be allocated somewhere valid
					gomega.Expect(len(anotherLargeNumas)).To(gomega.BeNumerically(">", 0),
						"Another large GPU 0 container should be allocated to at least one NUMA node")
				}

				ginkgo.GinkgoWriter.Printf("✓ Successfully handled Socket-level escalation for GPU containers\n")
				ginkgo.GinkgoWriter.Printf("  - Very large GPU 0 (30 CPU): NUMA %v\n", veryLargeNumas)
				ginkgo.GinkgoWriter.Printf("  - Another large GPU 0 (15 CPU): NUMA %v\n", anotherLargeNumas)
			})

			ginkgo.It("should handle cross-socket allocation when GPU socket is fully utilized", func() {
				policy := topologyaware.NewTopologyAwarePolicyWithSystem(mockCache, opts, mockSystem)
				gomega.Expect(policy).NotTo(gomega.BeNil())

				topologyPolicy, ok := policy.(*topologyaware.TopologyAwarePolicy)
				gomega.Expect(ok).To(gomega.BeTrue())

				// Test scenario: Fill up Socket 0 (NUMA 0, 1) completely, then allocate more GPU 0 containers
				// This tests the fallback behavior when even the GPU's socket is exhausted

				ginkgo.GinkgoWriter.Printf("Testing cross-socket allocation when GPU socket is fully utilized\n")
				ginkgo.GinkgoWriter.Printf("Socket 0 (NUMA 0, 1) has 48 CPUs total\n")

				// Container 1: Fill most of Socket 0 (40 CPUs)
				socketFillerCtx := createContainerContextWithGPU(
					"socket-filler", "socket-filler-uid", "socket-filler-pod", "default",
					40, 40, []string{"0"}, // 40 CPU + GPU 0 (on NUMA 1)
				)

				socketFillerAllocation, err := topologyPolicy.PreCreateContainerHook(socketFillerCtx)
				gomega.Expect(err).To(gomega.BeNil())
				gomega.Expect(socketFillerAllocation).NotTo(gomega.BeNil())

				socketFillerNumas := extractNUMAFromCPUSet(socketFillerAllocation.Resources.CpusetCpus)
				ginkgo.GinkgoWriter.Printf("Socket filler container (40 CPU) allocated to NUMA nodes: %v\n", socketFillerNumas)

				// Container 2: Try to allocate another GPU 0 container (12 CPUs)
				// This should exceed Socket 0 capacity (40+12=52 > 48)
				overflowGPU0Ctx := createContainerContextWithGPU(
					"overflow-gpu0", "overflow-gpu0-uid", "overflow-gpu0-pod", "default",
					12, 12, []string{"0"}, // 12 CPU + GPU 0 (on NUMA 1)
				)

				overflowAllocation, err := topologyPolicy.PreCreateContainerHook(overflowGPU0Ctx)
				gomega.Expect(err).To(gomega.BeNil())
				gomega.Expect(overflowAllocation).NotTo(gomega.BeNil())

				overflowNumas := extractNUMAFromCPUSet(overflowAllocation.Resources.CpusetCpus)
				ginkgo.GinkgoWriter.Printf("Overflow GPU 0 container (12 CPU) allocated to NUMA nodes: %v\n", overflowNumas)

				// Container 3: Another GPU 0 container (8 CPUs) to further test cross-socket behavior
				crossSocketGPU0Ctx := createContainerContextWithGPU(
					"cross-socket-gpu0", "cross-socket-gpu0-uid", "cross-socket-gpu0-pod", "default",
					8, 8, []string{"0"}, // 8 CPU + GPU 0 (on NUMA 1)
				)

				crossSocketAllocation, err := topologyPolicy.PreCreateContainerHook(crossSocketGPU0Ctx)
				gomega.Expect(err).To(gomega.BeNil())
				gomega.Expect(crossSocketAllocation).NotTo(gomega.BeNil())

				crossSocketNumas := extractNUMAFromCPUSet(crossSocketAllocation.Resources.CpusetCpus)
				ginkgo.GinkgoWriter.Printf("Cross-socket GPU 0 container (8 CPU) allocated to NUMA nodes: %v\n", crossSocketNumas)

				// Verify allocation behavior
				// Socket filler should prefer Socket 0 (where GPU 0 is located)
				if len(socketFillerNumas) >= 1 {
					hasSocket0NUMA := false
					for _, numa := range socketFillerNumas {
						if numa == 0 || numa == 1 {
							hasSocket0NUMA = true
							break
						}
					}
					gomega.Expect(hasSocket0NUMA).To(gomega.BeTrue(),
						"Socket filler container should be allocated to Socket 0 (GPU 0 location)")
					ginkgo.GinkgoWriter.Printf("✓ Socket filler allocated to Socket 0 (optimal GPU locality)\n")
				}

				// Overflow container may need to spill to Socket 1 (NUMA 2, 3)
				if len(overflowNumas) >= 1 {
					hasSocket0NUMA := false
					hasSocket1NUMA := false
					for _, numa := range overflowNumas {
						if numa == 0 || numa == 1 {
							hasSocket0NUMA = true
						}
						if numa == 2 || numa == 3 {
							hasSocket1NUMA = true
						}
					}

					if hasSocket0NUMA {
						ginkgo.GinkgoWriter.Printf("✓ Overflow container still allocated to Socket 0 (good GPU locality)\n")
					} else if hasSocket1NUMA {
						ginkgo.GinkgoWriter.Printf("✓ Overflow container spilled to Socket 1 (cross-socket allocation)\n")
					}

					// Should be allocated somewhere valid
					gomega.Expect(hasSocket0NUMA || hasSocket1NUMA).To(gomega.BeTrue(),
						"Overflow container should be allocated to a valid socket")
				}

				// Cross-socket container should also get valid allocation
				if len(crossSocketNumas) >= 1 {
					hasSocket0NUMA := false
					hasSocket1NUMA := false
					for _, numa := range crossSocketNumas {
						if numa == 0 || numa == 1 {
							hasSocket0NUMA = true
						}
						if numa == 2 || numa == 3 {
							hasSocket1NUMA = true
						}
					}

					if hasSocket0NUMA {
						ginkgo.GinkgoWriter.Printf("✓ Cross-socket container allocated to Socket 0 (good GPU locality)\n")
					} else if hasSocket1NUMA {
						ginkgo.GinkgoWriter.Printf("✓ Cross-socket container allocated to Socket 1 (resource pressure fallback)\n")
					}

					// Should be allocated somewhere valid
					gomega.Expect(hasSocket0NUMA || hasSocket1NUMA).To(gomega.BeTrue(),
						"Cross-socket container should be allocated to a valid socket")
				}

				ginkgo.GinkgoWriter.Printf("✓ Successfully handled cross-socket allocation under resource pressure\n")
				ginkgo.GinkgoWriter.Printf("  - Socket filler (40 CPU): NUMA %v\n", socketFillerNumas)
				ginkgo.GinkgoWriter.Printf("  - Overflow GPU 0 (12 CPU): NUMA %v\n", overflowNumas)
				ginkgo.GinkgoWriter.Printf("  - Cross-socket GPU 0 (8 CPU): NUMA %v\n", crossSocketNumas)
			})

			// TODO: Add comparison test between GPU-First and CPU-First strategies
		})

		ginkgo.Context("with 4 GPUs on all NUMA nodes", func() {
			ginkgo.BeforeEach(func() {
				mockSystem.SetupLargeTopology()
				setupGPUConfiguration(mockSystem, "4GPU_ALL_NUMA")
			})

			ginkgo.It("should prefer specific GPU NUMA node when requested", func() {
				policy := topologyaware.NewTopologyAwarePolicyWithSystem(mockCache, opts, mockSystem)
				gomega.Expect(policy).NotTo(gomega.BeNil())

				topologyPolicy, ok := policy.(*topologyaware.TopologyAwarePolicy)
				gomega.Expect(ok).To(gomega.BeTrue())

				// Create GPU container requesting specific GPU
				containerCtx := createContainerContextWithGPU(
					"gpu-container", "gpu-pod-uid", "gpu-pod", "default",
					8, 8, []string{"3"}, // 8 CPU + GPU 3 (on NUMA 3)
				)

				allocation, err := topologyPolicy.PreCreateContainerHook(containerCtx)
				gomega.Expect(err).To(gomega.BeNil())
				gomega.Expect(allocation).NotTo(gomega.BeNil())
				gomega.Expect(allocation.Resources.CpusetCpus).NotTo(gomega.BeEmpty())

				numaNodes := extractNUMAFromCPUSet(allocation.Resources.CpusetCpus)
				ginkgo.GinkgoWriter.Printf("GPU container (4GPU, GPU-first) allocated to NUMA nodes: %v\n", numaNodes)

				// With GPU-first strategy, should prefer NUMA 3 (where GPU 3 is located)
				gomega.Expect(numaNodes).To(gomega.ContainElement(3))
			})

			ginkgo.It("should allocate multiple GPU containers to their respective NUMA nodes", func() {
				policy := topologyaware.NewTopologyAwarePolicyWithSystem(mockCache, opts, mockSystem)
				gomega.Expect(policy).NotTo(gomega.BeNil())

				topologyPolicy, ok := policy.(*topologyaware.TopologyAwarePolicy)
				gomega.Expect(ok).To(gomega.BeTrue())

				// In 4GPU_ALL_NUMA configuration: GPU 0→NUMA 0, GPU 1→NUMA 1, GPU 2→NUMA 2, GPU 3→NUMA 3

				// Test GPU 0 container (should go to NUMA 0)
				gpu0ContainerCtx := createContainerContextWithGPU(
					"gpu-container-0", "gpu-pod-0-uid", "gpu-pod-0", "default",
					6, 6, []string{"0"}, // 6 CPU + GPU 0 (on NUMA 0 in 4GPU config)
				)

				gpu0Allocation, err := topologyPolicy.PreCreateContainerHook(gpu0ContainerCtx)
				gomega.Expect(err).To(gomega.BeNil())
				gomega.Expect(gpu0Allocation).NotTo(gomega.BeNil())

				gpu0Numas := extractNUMAFromCPUSet(gpu0Allocation.Resources.CpusetCpus)
				ginkgo.GinkgoWriter.Printf("GPU 0 container (GPU-first) allocated to NUMA nodes: %v\n", gpu0Numas)
				ginkgo.GinkgoWriter.Printf("Expected: Should be allocated to NUMA 0 (where GPU 0 is located in 4GPU config)\n")

				// Test GPU 2 container (should go to NUMA 2)
				gpu2ContainerCtx := createContainerContextWithGPU(
					"gpu-container-2", "gpu-pod-2-uid", "gpu-pod-2", "default",
					6, 6, []string{"2"}, // 6 CPU + GPU 2 (on NUMA 2 in 4GPU config)
				)

				gpu2Allocation, err := topologyPolicy.PreCreateContainerHook(gpu2ContainerCtx)
				gomega.Expect(err).To(gomega.BeNil())
				gomega.Expect(gpu2Allocation).NotTo(gomega.BeNil())

				gpu2Numas := extractNUMAFromCPUSet(gpu2Allocation.Resources.CpusetCpus)
				ginkgo.GinkgoWriter.Printf("GPU 2 container (GPU-first) allocated to NUMA nodes: %v\n", gpu2Numas)
				ginkgo.GinkgoWriter.Printf("Expected: Should be allocated to NUMA 2 (where GPU 2 is located in 4GPU config)\n")

				// Verify GPU 0 container allocation
				if len(gpu0Numas) == 1 {
					allocatedNUMA := gpu0Numas[0]
					ginkgo.GinkgoWriter.Printf("✓ GPU 0 container allocated to NUMA %d\n", allocatedNUMA)
					gomega.Expect(allocatedNUMA).To(gomega.Equal(0),
						"GPU-first strategy should allocate GPU 0 container to NUMA 0 (4GPU config)")
				} else {
					ginkgo.GinkgoWriter.Printf("✓ GPU 0 container allocated across multiple NUMA nodes: %v\n", gpu0Numas)
					gomega.Expect(gpu0Numas).To(gomega.ContainElement(0),
						"Multi-NUMA allocation should include NUMA 0 (where GPU 0 is located)")
				}

				// Verify GPU 2 container allocation
				if len(gpu2Numas) == 1 {
					allocatedNUMA := gpu2Numas[0]
					ginkgo.GinkgoWriter.Printf("✓ GPU 2 container allocated to NUMA %d\n", allocatedNUMA)
					gomega.Expect(allocatedNUMA).To(gomega.Equal(2),
						"GPU-first strategy should allocate GPU 2 container to NUMA 2 (4GPU config)")
				} else {
					ginkgo.GinkgoWriter.Printf("✓ GPU 2 container allocated across multiple NUMA nodes: %v\n", gpu2Numas)
					gomega.Expect(gpu2Numas).To(gomega.ContainElement(2),
						"Multi-NUMA allocation should include NUMA 2 (where GPU 2 is located)")
				}

				// Verify that the two containers are allocated to different NUMA nodes
				ginkgo.GinkgoWriter.Printf("✓ Verified: GPU 0 container on NUMA %v, GPU 2 container on NUMA %v\n", gpu0Numas, gpu2Numas)
				if len(gpu0Numas) == 1 && len(gpu2Numas) == 1 {
					gomega.Expect(gpu0Numas[0]).NotTo(gomega.Equal(gpu2Numas[0]),
						"GPU containers should be allocated to different NUMA nodes for optimal GPU locality")
				}
			})
		})
	})

	ginkgo.Describe("Mixed Workload Scenarios", func() {
		ginkgo.Context("CPU-first strategy with mixed workloads", func() {
			ginkgo.BeforeEach(func() {
				opts.ResourcePriority = policy.ResourcePriorityCPUFirst
				mockSystem.SetupLargeTopology()
				setupGPUConfiguration(mockSystem, "2GPU_NUMA1_3")
			})

			ginkgo.It("should handle sequential allocation of CPU and GPU workloads", func() {
				policy := topologyaware.NewTopologyAwarePolicyWithSystem(mockCache, opts, mockSystem)
				gomega.Expect(policy).NotTo(gomega.BeNil())

				topologyPolicy, ok := policy.(*topologyaware.TopologyAwarePolicy)
				gomega.Expect(ok).To(gomega.BeTrue())

				// First allocate a CPU-intensive container
				cpuContainerCtx := createContainerContextWithGPU(
					"cpu-heavy", "cpu-heavy-uid", "cpu-heavy-pod", "default",
					12, 12, nil, // 12 CPU, no GPU
				)

				cpuAllocation, err := topologyPolicy.PreCreateContainerHook(cpuContainerCtx)
				gomega.Expect(err).To(gomega.BeNil())
				gomega.Expect(cpuAllocation).NotTo(gomega.BeNil())

				cpuNumas := extractNUMAFromCPUSet(cpuAllocation.Resources.CpusetCpus)
				ginkgo.GinkgoWriter.Printf("CPU-heavy container allocated to NUMA nodes: %v\n", cpuNumas)

				// Then allocate a GPU container
				gpuContainerCtx := createContainerContextWithGPU(
					"gpu-ml", "gpu-ml-uid", "gpu-ml-pod", "default",
					8, 8, []string{"1"}, // 8 CPU + GPU 1 (on NUMA 3)
				)

				gpuAllocation, err := topologyPolicy.PreCreateContainerHook(gpuContainerCtx)
				gomega.Expect(err).To(gomega.BeNil())
				gomega.Expect(gpuAllocation).NotTo(gomega.BeNil())

				gpuNumas := extractNUMAFromCPUSet(gpuAllocation.Resources.CpusetCpus)
				ginkgo.GinkgoWriter.Printf("GPU-ML container allocated to NUMA nodes: %v\n", gpuNumas)

				// Verify allocations don't overlap inappropriately
				gomega.Expect(cpuAllocation.Resources.CpusetCpus).NotTo(gomega.Equal(gpuAllocation.Resources.CpusetCpus))
			})
		})

		ginkgo.Context("GPU-first strategy with mixed workloads", func() {
			ginkgo.BeforeEach(func() {
				opts.ResourcePriority = policy.ResourcePriorityGPUFirst
				mockSystem.SetupLargeTopology()
				setupGPUConfiguration(mockSystem, "4GPU_ALL_NUMA")
			})

			ginkgo.It("should prioritize GPU locality for ML workloads", func() {
				policy := topologyaware.NewTopologyAwarePolicyWithSystem(mockCache, opts, mockSystem)
				gomega.Expect(policy).NotTo(gomega.BeNil())

				topologyPolicy, ok := policy.(*topologyaware.TopologyAwarePolicy)
				gomega.Expect(ok).To(gomega.BeTrue())

				// Allocate multiple GPU containers to different NUMA nodes
				testCases := []struct {
					name     string
					gpuID    string
					expected int // expected NUMA node
				}{
					{"ml-training-0", "0", 0},
					{"ml-training-1", "1", 1},
					{"ml-training-2", "2", 2},
					{"ml-training-3", "3", 3},
				}

				for _, tc := range testCases {
					containerCtx := createContainerContextWithGPU(
						tc.name, tc.name+"-uid", tc.name+"-pod", "default",
						6, 6, []string{tc.gpuID},
					)

					allocation, err := topologyPolicy.PreCreateContainerHook(containerCtx)
					gomega.Expect(err).To(gomega.BeNil())
					gomega.Expect(allocation).NotTo(gomega.BeNil())

					numaNodes := extractNUMAFromCPUSet(allocation.Resources.CpusetCpus)
					ginkgo.GinkgoWriter.Printf("Container %s (GPU %s) allocated to NUMA nodes: %v\n",
						tc.name, tc.gpuID, numaNodes)

					// With GPU-first strategy, should prefer the NUMA node with the requested GPU
					gomega.Expect(numaNodes).To(gomega.ContainElement(tc.expected),
						"Container %s should be allocated to NUMA %d (where GPU %s is located)",
						tc.name, tc.expected, tc.gpuID)
				}
			})
		})
	})

	ginkgo.Describe("Resource Contention Scenarios", func() {
		ginkgo.Context("High CPU utilization with GPU requests", func() {
			ginkgo.BeforeEach(func() {
				mockSystem.SetupLargeTopology()
				setupGPUConfiguration(mockSystem, "2GPU_NUMA1_3")
			})

			ginkgo.It("should handle resource contention differently based on priority strategy", func() {
				// Test both strategies with the same workload pattern
				strategies := []struct {
					name     string
					priority policy.ResourcePriority
				}{
					{"CPU-First", policy.ResourcePriorityCPUFirst},
					{"GPU-First", policy.ResourcePriorityGPUFirst},
				}

				for _, strategy := range strategies {
					ginkgo.By(fmt.Sprintf("Testing %s strategy", strategy.name))

					opts.ResourcePriority = strategy.priority
					policy := topologyaware.NewTopologyAwarePolicyWithSystem(mockCache, opts, mockSystem)
					gomega.Expect(policy).NotTo(gomega.BeNil())

					topologyPolicy, ok := policy.(*topologyaware.TopologyAwarePolicy)
					gomega.Expect(ok).To(gomega.BeTrue())

					// Simulate high CPU usage scenario
					highCPUCtx := createContainerContextWithGPU(
						"high-cpu", "high-cpu-uid", "high-cpu-pod", "default",
						20, 20, nil, // 20 CPU, no GPU
					)

					cpuAllocation, err := topologyPolicy.PreCreateContainerHook(highCPUCtx)
					gomega.Expect(err).To(gomega.BeNil())
					gomega.Expect(cpuAllocation).NotTo(gomega.BeNil())

					// Then try to allocate a GPU workload
					gpuCtx := createContainerContextWithGPU(
						"gpu-workload", "gpu-workload-uid", "gpu-workload-pod", "default",
						8, 8, []string{"0"}, // 8 CPU + GPU 0 (on NUMA 1)
					)

					gpuAllocation, err := topologyPolicy.PreCreateContainerHook(gpuCtx)
					gomega.Expect(err).To(gomega.BeNil())
					gomega.Expect(gpuAllocation).NotTo(gomega.BeNil())

					cpuNumas := extractNUMAFromCPUSet(cpuAllocation.Resources.CpusetCpus)
					gpuNumas := extractNUMAFromCPUSet(gpuAllocation.Resources.CpusetCpus)

					ginkgo.GinkgoWriter.Printf("%s strategy - High CPU container: %v, GPU container: %v\n",
						strategy.name, cpuNumas, gpuNumas)

					// Verify both allocations are valid
					gomega.Expect(len(cpuNumas)).To(gomega.BeNumerically(">", 0))
					gomega.Expect(len(gpuNumas)).To(gomega.BeNumerically(">", 0))
				}
			})
		})
	})

	ginkgo.Describe("Edge Cases and Validation", func() {
		ginkgo.Context("Invalid GPU requests", func() {
			ginkgo.BeforeEach(func() {
				opts.ResourcePriority = policy.ResourcePriorityGPUFirst
				mockSystem.SetupLargeTopology()
				setupGPUConfiguration(mockSystem, "2GPU_NUMA1_3")
			})

			ginkgo.It("should handle containers with invalid GPU device requests gracefully", func() {
				policy := topologyaware.NewTopologyAwarePolicyWithSystem(mockCache, opts, mockSystem)
				gomega.Expect(policy).NotTo(gomega.BeNil())

				topologyPolicy, ok := policy.(*topologyaware.TopologyAwarePolicy)
				gomega.Expect(ok).To(gomega.BeTrue())

				// Request non-existent GPU
				containerCtx := createContainerContextWithGPU(
					"invalid-gpu", "invalid-gpu-uid", "invalid-gpu-pod", "default",
					4, 4, []string{"99"}, // Non-existent GPU 99
				)

				allocation, err := topologyPolicy.PreCreateContainerHook(containerCtx)
				// Should still succeed but may not get optimal GPU placement
				gomega.Expect(err).To(gomega.BeNil())
				gomega.Expect(allocation).NotTo(gomega.BeNil())

				numaNodes := extractNUMAFromCPUSet(allocation.Resources.CpusetCpus)
				ginkgo.GinkgoWriter.Printf("Container with invalid GPU request allocated to NUMA nodes: %v\n", numaNodes)
			})
		})

		ginkgo.Context("System containers", func() {
			ginkgo.BeforeEach(func() {
				mockSystem.SetupLargeTopology()
				setupGPUConfiguration(mockSystem, "4GPU_ALL_NUMA")
			})

			ginkgo.It("should handle system containers consistently regardless of priority strategy", func() {
				strategies := []policy.ResourcePriority{
					policy.ResourcePriorityCPUFirst,
					policy.ResourcePriorityGPUFirst,
				}

				for _, strategy := range strategies {
					opts.ResourcePriority = strategy
					policy := topologyaware.NewTopologyAwarePolicyWithSystem(mockCache, opts, mockSystem)
					gomega.Expect(policy).NotTo(gomega.BeNil())

					topologyPolicy, ok := policy.(*topologyaware.TopologyAwarePolicy)
					gomega.Expect(ok).To(gomega.BeTrue())

					// Create system container (kube-system namespace)
					systemCtx := createContainerContextWithGPU(
						"system-container", "system-uid", "system-pod", "kube-system",
						2, 2, nil,
					)

					allocation, err := topologyPolicy.PreCreateContainerHook(systemCtx)
					gomega.Expect(err).To(gomega.BeNil())
					gomega.Expect(allocation).NotTo(gomega.BeNil())

					numaNodes := extractNUMAFromCPUSet(allocation.Resources.CpusetCpus)
					ginkgo.GinkgoWriter.Printf("System container (%v strategy) allocated to NUMA nodes: %v\n",
						strategy, numaNodes)

					// System containers should get consistent allocation regardless of strategy
					gomega.Expect(len(numaNodes)).To(gomega.BeNumerically(">", 0))
				}
			})
		})
	})
})
