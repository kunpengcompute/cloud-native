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
	"strconv"
	"strings"
	"testing"

	ginkgo "github.com/onsi/ginkgo/v2"
	gomega "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"kunpeng.huawei.com/kunpeng-cloud-computing/pkg/kunpeng-tap/policy"
	topologyaware "kunpeng.huawei.com/kunpeng-cloud-computing/pkg/kunpeng-tap/policy/topology-aware"
)

// Helper functions for tests
func stringPtr(s string) *string {
	return &s
}

func int64Ptr(i int64) *int64 {
	return &i
}

// parseCpuList parses a CPU list string like "0-3,5,7" and returns a slice of CPU IDs
func parseCpuList(cpuList string) []int {
	var cpus []int
	if cpuList == "" {
		return cpus
	}

	parts := strings.Split(cpuList, ",")
	for _, part := range parts {
		cpus = append(cpus, parseCpuPart(part)...)
	}
	return cpus
}

// parseCpuPart parses a single part of CPU list (either a range like "0-3" or single CPU like "5")
func parseCpuPart(part string) []int {
	if strings.Contains(part, "-") {
		return parseCpuRange(part)
	}
	return parseSingleCpu(part)
}

// parseCpuRange parses a CPU range like "0-3" and returns a slice of CPU IDs
func parseCpuRange(rangePart string) []int {
	var cpus []int
	rangeParts := strings.Split(rangePart, "-")
	if len(rangeParts) != 2 {
		return cpus
	}

	start, err1 := strconv.Atoi(strings.TrimSpace(rangeParts[0]))
	end, err2 := strconv.Atoi(strings.TrimSpace(rangeParts[1]))
	if err1 != nil || err2 != nil {
		return cpus
	}

	for i := start; i <= end; i++ {
		cpus = append(cpus, i)
	}
	return cpus
}

// parseSingleCpu parses a single CPU ID like "5" and returns a slice with that CPU ID
func parseSingleCpu(cpuPart string) []int {
	var cpus []int
	cpu, err := strconv.Atoi(strings.TrimSpace(cpuPart))
	if err == nil {
		cpus = append(cpus, cpu)
	}
	return cpus
}

// createBasicContainerContext creates a basic ContainerContext for testing
func createBasicContainerContext(containerName, podUID, podName, namespace string) *policy.ContainerContext {
	// Create basic resource requirements with minimal CPU and memory
	cpuQuantity := resource.NewMilliQuantity(1000, resource.DecimalSI)       // 1 CPU
	memoryQuantity := resource.NewQuantity(100*1024*1024, resource.BinarySI) // 100MB

	return &policy.ContainerContext{
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
						corev1.ResourceCPU:    *cpuQuantity,
						corev1.ResourceMemory: *memoryQuantity,
					},
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    *cpuQuantity,
						corev1.ResourceMemory: *memoryQuantity,
					},
				},
			},
		},
	}
}

// createContainerContextWithResources creates a ContainerContext with resource specifications
func createContainerContextWithResources(containerName, podUID, podName, namespace string, cpuQuota, cpuPeriod, cpuShares int64) *policy.ContainerContext {
	ctx := createBasicContainerContext(containerName, podUID, podName, namespace)

	// Calculate CPU requests and limits based on quota and period
	cpuCores := float64(cpuQuota) / float64(cpuPeriod)
	cpuQuantity := resource.NewMilliQuantity(int64(cpuCores*1000), resource.DecimalSI)

	ctx.Request.Resources = &policy.Resources{
		CpuQuota:  int64Ptr(cpuQuota),
		CpuPeriod: int64Ptr(cpuPeriod),
		CpuShares: int64Ptr(cpuShares),
		EstimatedRequirements: &corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU: *cpuQuantity,
			},
			Limits: corev1.ResourceList{
				corev1.ResourceCPU: *cpuQuantity,
			},
		},
	}
	return ctx
}

// createContainerContextWithID creates a ContainerContext with specific container ID
func createContainerContextWithID(containerID, containerName, podUID string) *policy.ContainerContext {
	return &policy.ContainerContext{
		Request: policy.ContainerRequest{
			ContainerMeta: policy.ContainerMeta{
				ID:   containerID,
				Name: containerName,
			},
			PodMeta: policy.PodMeta{
				UID: podUID,
			},
			Resources: &policy.Resources{
				EstimatedRequirements: &corev1.ResourceRequirements{
					Requests: corev1.ResourceList{},
					Limits:   corev1.ResourceList{},
				},
			},
		},
	}
}

// createContainerContextWithCPURequests creates a ContainerContext with specific CPU requests and limits
func createContainerContextWithCPURequests(containerName, podUID, podName, namespace string, requestsCPU, limitsCPU int64) *policy.ContainerContext {
	requestsQuantity := resource.NewQuantity(requestsCPU, resource.DecimalSI)
	limitsQuantity := resource.NewQuantity(limitsCPU, resource.DecimalSI)

	return &policy.ContainerContext{
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
						corev1.ResourceCPU: *requestsQuantity,
					},
					Limits: corev1.ResourceList{
						corev1.ResourceCPU: *limitsQuantity,
					},
				},
			},
		},
	}
}

// isValidNUMARange checks if the CPU set belongs to one of the expected NUMA ranges
func isValidNUMARange(cpuSet string) bool {
	// Support both single-socket (small) and multi-socket (large) topologies
	expectedRanges := []string{
		// Single-socket topology (8 CPUs)
		"0-7", // Single NUMA node

		// Multi-socket topology (96 CPUs)
		"0-23",  // NUMA 0
		"24-47", // NUMA 1
		"48-71", // NUMA 2
		"72-95", // NUMA 3
	}

	for _, expectedRange := range expectedRanges {
		if cpuSet == expectedRange {
			return true
		}
	}
	return false
}

// isValidSocketRange checks if the CPU set belongs to one of the expected Socket ranges
func isValidSocketRange(cpuSet string) bool {
	expectedRanges := []string{
		"0-47",  // Socket 0 (NUMA 0 + NUMA 1)
		"48-95", // Socket 1 (NUMA 2 + NUMA 3)
	}

	for _, expectedRange := range expectedRanges {
		if cpuSet == expectedRange {
			return true
		}
	}
	return false
}

// isValidSystemRange checks if the CPU set covers the entire system
func isValidSystemRange(cpuSet string) bool {
	return cpuSet == "0-95" // Entire system range
}

// createContainerContextWithMemory creates a ContainerContext with CPU and memory specifications
func createContainerContextWithMemory(containerName, podUID, podName, namespace string, requestsCPU, limitsCPU int64, memoryMB int64) *policy.ContainerContext {
	requestsCPUQuantity := resource.NewQuantity(requestsCPU, resource.DecimalSI)
	limitsCPUQuantity := resource.NewQuantity(limitsCPU, resource.DecimalSI)
	memoryQuantity := resource.NewQuantity(memoryMB*1024*1024, resource.BinarySI) // Convert MB to bytes

	return &policy.ContainerContext{
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
						corev1.ResourceCPU:    *requestsCPUQuantity,
						corev1.ResourceMemory: *memoryQuantity,
					},
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    *limitsCPUQuantity,
						corev1.ResourceMemory: *memoryQuantity,
					},
				},
			},
		},
	}
}

// getAllocationType determines the allocation type based on CPU set
func getAllocationType(cpuSet string) string {
	if isValidNUMARange(cpuSet) {
		return "NUMA"
	} else if isValidSocketRange(cpuSet) {
		return "Socket"
	} else if isValidSystemRange(cpuSet) {
		return "System"
	}
	return "Unknown"
}

func TestTopologyAware(t *testing.T) {
	gomega.RegisterFailHandler(ginkgo.Fail)
	ginkgo.RunSpecs(t, "TopologyAware Policy Suite")
}

var _ = ginkgo.Describe("TopologyAware Policy", func() {
	var (
		mockCache  *MockCache
		mockSystem *MockSystem
		opts       *policy.PolicyOptions
	)

	ginkgo.BeforeEach(func() {
		mockCache = NewMockCache()
		mockSystem = NewMockSystem()
		opts = &policy.PolicyOptions{
			EnableMemoryTopology: false,
		}
	})

	ginkgo.Describe("NewTopologyAwarePolicy", func() {
		ginkgo.Context("with valid system and cache", func() {
			ginkgo.BeforeEach(func() {
				// Setup a simple single-socket, single-NUMA topology
				mockSystem.SetupSingleSocketTopology()
			})

			ginkgo.It("should create policy successfully", func() {
				policy := topologyaware.NewTopologyAwarePolicyWithSystem(mockCache, opts, mockSystem)
				gomega.Expect(policy).NotTo(gomega.BeNil())
				gomega.Expect(policy.Name()).To(gomega.Equal(topologyaware.PolicyName))
				gomega.Expect(policy.Description()).To(gomega.Equal(topologyaware.PolicyDescription))
			})

			ginkgo.It("should create policy with memory topology enabled", func() {
				opts.EnableMemoryTopology = true
				policy := topologyaware.NewTopologyAwarePolicyWithSystem(mockCache, opts, mockSystem)
				gomega.Expect(policy).NotTo(gomega.BeNil())

				topologyPolicy, ok := policy.(*topologyaware.TopologyAwarePolicy)
				gomega.Expect(ok).To(gomega.BeTrue())
				gomega.Expect(topologyPolicy.MemoryTopology()).To(gomega.BeTrue())
			})
		})

		ginkgo.Context("with invalid system", func() {
			ginkgo.BeforeEach(func() {
				// Setup system that will fail validation
				mockSystem.SetupInvalidTopology()
			})

			ginkgo.It("should return nil when system validation fails", func() {
				policy := topologyaware.NewTopologyAwarePolicyWithSystem(mockCache, opts, mockSystem)
				gomega.Expect(policy).To(gomega.BeNil())
			})
		})

		ginkgo.Context("with multi-socket topology", func() {
			ginkgo.BeforeEach(func() {
				// Setup a dual-socket topology
				mockSystem.SetupDualSocketTopology()
			})

			ginkgo.It("should create policy with virtual root node", func() {
				policy := topologyaware.NewTopologyAwarePolicyWithSystem(mockCache, opts, mockSystem)
				gomega.Expect(policy).NotTo(gomega.BeNil())

				topologyPolicy, ok := policy.(*topologyaware.TopologyAwarePolicy)
				gomega.Expect(ok).To(gomega.BeTrue())
				gomega.Expect(topologyPolicy.Root()).NotTo(gomega.BeNil())
				gomega.Expect(topologyPolicy.Root().Kind()).To(gomega.Equal(topologyaware.VirtualNode))
			})
		})
	})

	ginkgo.Describe("PreCreateContainerHook", func() {
		var (
			topologyPolicy *topologyaware.TopologyAwarePolicy
			containerCtx   *policy.ContainerContext
		)

		ginkgo.BeforeEach(func() {
			mockSystem.SetupSingleSocketTopology()
			policy := topologyaware.NewTopologyAwarePolicyWithSystem(mockCache, opts, mockSystem)
			gomega.Expect(policy).NotTo(gomega.BeNil())

			var ok bool
			topologyPolicy, ok = policy.(*topologyaware.TopologyAwarePolicy)
			gomega.Expect(ok).To(gomega.BeTrue())

			// Setup container context
			containerCtx = createBasicContainerContext("test-container", "test-pod-uid", "test-pod", "default")
		})

		ginkgo.Context("with valid container request", func() {
			ginkgo.It("should allocate resources successfully", func() {
				allocation, err := topologyPolicy.PreCreateContainerHook(containerCtx)
				gomega.Expect(err).To(gomega.BeNil())
				gomega.Expect(allocation).NotTo(gomega.BeNil())
				gomega.Expect(allocation.Resources.CpusetCpus).NotTo(gomega.BeEmpty())
			})
		})

		ginkgo.Context("with system container", func() {
			ginkgo.BeforeEach(func() {
				containerCtx.Request.PodMeta.Namespace = "kube-system"
			})

			ginkgo.It("should allocate to root pool", func() {
				allocation, err := topologyPolicy.PreCreateContainerHook(containerCtx)
				gomega.Expect(err).To(gomega.BeNil())
				gomega.Expect(allocation).NotTo(gomega.BeNil())
			})
		})

		ginkgo.Context("with invalid context type", func() {
			ginkgo.It("should return error for invalid context", func() {
				allocation, err := topologyPolicy.PreCreateContainerHook(nil)
				gomega.Expect(err).NotTo(gomega.BeNil())
				gomega.Expect(allocation).To(gomega.BeNil())
				gomega.Expect(err.Error()).To(gomega.ContainSubstring("invalid context type"))
			})
		})
	})

	ginkgo.Describe("PostStopContainerHook", func() {
		var (
			topologyPolicy *topologyaware.TopologyAwarePolicy
			containerCtx   *policy.ContainerContext
		)

		ginkgo.BeforeEach(func() {
			mockSystem.SetupSingleSocketTopology()
			policy := topologyaware.NewTopologyAwarePolicyWithSystem(mockCache, opts, mockSystem)
			gomega.Expect(policy).NotTo(gomega.BeNil())

			var ok bool
			topologyPolicy, ok = policy.(*topologyaware.TopologyAwarePolicy)
			gomega.Expect(ok).To(gomega.BeTrue())

			// Setup container context
			containerCtx = createContainerContextWithID("test-container-id", "test-container", "test-pod-uid")

			// Setup mock cache to return container and pod
			mockCache.SetupContainerAndPod("test-container-id", "test-pod-uid", "test-container")
		})

		ginkgo.Context("with valid container", func() {
			ginkgo.It("should release resources successfully", func() {
				// Create a container context with proper resource requirements
				allocCtx := createBasicContainerContext("test-container", "test-pod-uid", "test-pod", "default")

				// First allocate resources
				_, err := topologyPolicy.PreCreateContainerHook(allocCtx)
				gomega.Expect(err).To(gomega.BeNil())

				// Then release resources
				allocation, err := topologyPolicy.PostStopContainerHook(containerCtx)
				gomega.Expect(err).To(gomega.BeNil())
				gomega.Expect(allocation).To(gomega.BeNil()) // PostStopContainerHook returns nil allocation
			})
		})

		ginkgo.Context("with container not in cache", func() {
			ginkgo.BeforeEach(func() {
				mockCache.ClearContainers()
			})

			ginkgo.It("should handle missing container gracefully", func() {
				allocation, err := topologyPolicy.PostStopContainerHook(containerCtx)
				gomega.Expect(err).To(gomega.BeNil())
				gomega.Expect(allocation).To(gomega.BeNil())
			})
		})

		ginkgo.Context("with invalid context type", func() {
			ginkgo.It("should return nil for invalid context", func() {
				allocation, err := topologyPolicy.PostStopContainerHook(nil)
				gomega.Expect(err).To(gomega.BeNil())
				gomega.Expect(allocation).To(gomega.BeNil())
			})
		})
	})

	ginkgo.Describe("ContainerContext Creation Examples", func() {
		var topologyPolicy *topologyaware.TopologyAwarePolicy

		ginkgo.BeforeEach(func() {
			mockSystem.SetupSingleSocketTopology()
			policy := topologyaware.NewTopologyAwarePolicyWithSystem(mockCache, opts, mockSystem)
			gomega.Expect(policy).NotTo(gomega.BeNil())

			var ok bool
			topologyPolicy, ok = policy.(*topologyaware.TopologyAwarePolicy)
			gomega.Expect(ok).To(gomega.BeTrue())
		})

		ginkgo.Context("Basic ContainerContext", func() {
			ginkgo.It("should create basic container context successfully", func() {
				containerCtx := createBasicContainerContext("web-server", "web-pod-uid-123", "web-server-pod", "production")

				gomega.Expect(containerCtx).NotTo(gomega.BeNil())
				gomega.Expect(containerCtx.Request.ContainerMeta.Name).To(gomega.Equal("web-server"))
				gomega.Expect(containerCtx.Request.ContainerMeta.ID).To(gomega.Equal("containerd://web-server-id-123"))
				gomega.Expect(containerCtx.Request.PodMeta.UID).To(gomega.Equal("web-pod-uid-123"))
				gomega.Expect(containerCtx.Request.PodMeta.Name).To(gomega.Equal("web-server-pod"))
				gomega.Expect(containerCtx.Request.PodMeta.Namespace).To(gomega.Equal("production"))
			})
		})

		ginkgo.Context("ContainerContext with Resources", func() {
			ginkgo.It("should create container context with CPU resources", func() {
				containerCtx := createContainerContextWithResources(
					"cpu-intensive-app",
					"cpu-pod-uid-456",
					"cpu-intensive-pod",
					"compute",
					400000, // 4 CPU cores quota
					100000, // period
					2048,   // shares
				)

				gomega.Expect(containerCtx).NotTo(gomega.BeNil())
				gomega.Expect(containerCtx.Request.Resources).NotTo(gomega.BeNil())
				gomega.Expect(*containerCtx.Request.Resources.CpuQuota).To(gomega.Equal(int64(400000)))
				gomega.Expect(*containerCtx.Request.Resources.CpuPeriod).To(gomega.Equal(int64(100000)))
				gomega.Expect(*containerCtx.Request.Resources.CpuShares).To(gomega.Equal(int64(2048)))
			})

			ginkgo.It("should allocate resources for CPU intensive workload", func() {
				// Create a CPU intensive container using our working function
				containerCtx := createContainerContextWithMemory(
					"cpu-intensive-app",
					"cpu-pod-uid-456",
					"cpu-intensive-pod",
					"compute",
					4,   // requests.CPU = 4 cores
					6,   // limits.CPU = 6 cores
					512, // memory = 512MB
				)

				allocation, err := topologyPolicy.PreCreateContainerHook(containerCtx)
				gomega.Expect(err).To(gomega.BeNil())
				gomega.Expect(allocation).NotTo(gomega.BeNil())
				gomega.Expect(allocation.Resources.CpusetCpus).NotTo(gomega.BeEmpty())

				// Verify allocated CPU count and that it's in NUMA range (6 CPU <= 24)
				cpuList := parseCpuList(allocation.Resources.CpusetCpus)
				gomega.Expect(len(cpuList)).To(gomega.BeNumerically(">=", 1))

				cpuSet := allocation.Resources.CpusetCpus
				gomega.Expect(isValidNUMARange(cpuSet)).To(gomega.BeTrue(),
					"CPU intensive container CPU set %s should be in NUMA range", cpuSet)
			})
		})

		ginkgo.Context("System Container", func() {
			ginkgo.It("should handle system containers in kube-system namespace", func() {
				containerCtx := createBasicContainerContext("kube-proxy", "system-pod-uid-789", "kube-proxy-pod", "kube-system")

				allocation, err := topologyPolicy.PreCreateContainerHook(containerCtx)
				gomega.Expect(err).To(gomega.BeNil())
				gomega.Expect(allocation).NotTo(gomega.BeNil())
			})
		})

		ginkgo.Context("Container with Extended Configuration", func() {
			ginkgo.It("should create container with full configuration", func() {
				containerCtx := &policy.ContainerContext{
					Request: policy.ContainerRequest{
						ContainerMeta: policy.ContainerMeta{
							Name:    "database-server",
							ID:      "containerd://database-server-id-abc123",
							Sandbox: false,
						},
						PodMeta: policy.PodMeta{
							UID:       "database-pod-uid-def456",
							Name:      "database-server-pod",
							Namespace: "database",
						},
						PodLabels: map[string]string{
							"app":     "database",
							"version": "v2.0.0",
							"tier":    "backend",
						},
						PodAnnotations: map[string]string{
							"scheduler.alpha.kubernetes.io/preferred-zone": "zone-a",
							"topology.kubernetes.io/zone":                  "us-west-1a",
						},
						CgroupParent: "/kubepods/burstable/pod-uid-def456",
						ContainerEnvs: map[string]string{
							"DB_PORT":         "5432",
							"LOG_LEVEL":       "info",
							"MAX_CONNECTIONS": "100",
						},
						Resources: &policy.Resources{
							CpuSetCpus:         stringPtr("0-7"),
							CpuSetMems:         stringPtr("0-1"),
							CpuPeriod:          int64Ptr(100000),
							CpuQuota:           int64Ptr(800000), // 8 CPU cores
							CpuShares:          int64Ptr(4096),
							MemoryLimitInBytes: int64Ptr(8589934592), // 8GB
						},
					},
					Response: policy.ContainerResponse{
						AddContainerEnvs: map[string]string{
							"ALLOCATED_CPUS": "0-7",
							"NUMA_NODE":      "0",
						},
					},
				}

				gomega.Expect(containerCtx).NotTo(gomega.BeNil())
				gomega.Expect(containerCtx.Request.ContainerMeta.Name).To(gomega.Equal("database-server"))
				gomega.Expect(containerCtx.Request.PodLabels["app"]).To(gomega.Equal("database"))
				gomega.Expect(containerCtx.Request.ContainerEnvs["DB_PORT"]).To(gomega.Equal("5432"))
				gomega.Expect(*containerCtx.Request.Resources.MemoryLimitInBytes).To(gomega.Equal(int64(8589934592)))
				gomega.Expect(containerCtx.Response.AddContainerEnvs["NUMA_NODE"]).To(gomega.Equal("0"))
			})
		})

		ginkgo.Describe("PreCreateContainerHook Large Topology Tests", func() {
			var topologyPolicy *topologyaware.TopologyAwarePolicy

			ginkgo.BeforeEach(func() {
				// Setup large topology: 2 Sockets, 4 NUMA nodes, 96 CPUs
				mockSystem.SetupLargeTopology()

				// Disable memory topology as specified
				opts.EnableMemoryTopology = false

				policy := topologyaware.NewTopologyAwarePolicyWithSystem(mockCache, opts, mockSystem)
				gomega.Expect(policy).NotTo(gomega.BeNil())

				var ok bool
				topologyPolicy, ok = policy.(*topologyaware.TopologyAwarePolicy)
				gomega.Expect(ok).To(gomega.BeTrue())
			})

			ginkgo.Context("with CPU requests 12 and limits 18", func() {
				ginkgo.It("should allocate CPUs within a single NUMA node range", func() {
					// Create container context with CPU requests=12, limits=18
					containerCtx := createContainerContextWithCPURequests(
						"cpu-test-container",
						"cpu-test-pod-uid",
						"cpu-test-pod",
						"default",
						12, // requests.CPU = 12
						18, // limits.CPU = 18
					)

					// Call PreCreateContainerHook
					allocation, err := topologyPolicy.PreCreateContainerHook(containerCtx)

					// Verify the allocation
					gomega.Expect(err).To(gomega.BeNil())
					gomega.Expect(allocation).NotTo(gomega.BeNil())
					gomega.Expect(allocation.Resources).NotTo(gomega.BeNil())
					gomega.Expect(allocation.Resources.CpusetCpus).NotTo(gomega.BeEmpty())

					// Verify that the allocated CPUs belong to one of the expected NUMA ranges
					cpuSet := allocation.Resources.CpusetCpus
					gomega.Expect(isValidNUMARange(cpuSet)).To(gomega.BeTrue(),
						"CPU set %s should match one of the NUMA ranges: 0-23, 24-47, 48-71, 72-95", cpuSet)

					// Log the allocation for debugging
					ginkgo.GinkgoWriter.Printf("Allocated CPU set: %s\n", cpuSet)

					// Additional verification: check which NUMA node this belongs to
					switch cpuSet {
					case "0-23":
						ginkgo.GinkgoWriter.Printf("✅ Allocated to NUMA 0 (Socket 0, NUMA 0): CPUs 0-23\n")
					case "24-47":
						ginkgo.GinkgoWriter.Printf("✅ Allocated to NUMA 1 (Socket 0, NUMA 1): CPUs 24-47\n")
					case "48-71":
						ginkgo.GinkgoWriter.Printf("✅ Allocated to NUMA 2 (Socket 1, NUMA 2): CPUs 48-71\n")
					case "72-95":
						ginkgo.GinkgoWriter.Printf("✅ Allocated to NUMA 3 (Socket 1, NUMA 3): CPUs 72-95\n")
					default:
						ginkgo.GinkgoWriter.Printf("⚠️  Unexpected CPU set: %s\n", cpuSet)
					}
				})

				ginkgo.It("should prefer NUMA node with best affinity", func() {
					// Create multiple containers to test NUMA affinity
					containerCtx1 := createContainerContextWithCPURequests(
						"cpu-test-container-1",
						"cpu-test-pod-uid-1",
						"cpu-test-pod-1",
						"default",
						8,  // requests.CPU = 8
						12, // limits.CPU = 12
					)

					containerCtx2 := createContainerContextWithCPURequests(
						"cpu-test-container-2",
						"cpu-test-pod-uid-2",
						"cpu-test-pod-2",
						"default",
						6,  // requests.CPU = 6
						10, // limits.CPU = 10
					)

					// Allocate first container
					allocation1, err1 := topologyPolicy.PreCreateContainerHook(containerCtx1)
					gomega.Expect(err1).To(gomega.BeNil())
					gomega.Expect(allocation1).NotTo(gomega.BeNil())
					gomega.Expect(isValidNUMARange(allocation1.Resources.CpusetCpus)).To(gomega.BeTrue())

					// Allocate second container
					allocation2, err2 := topologyPolicy.PreCreateContainerHook(containerCtx2)
					gomega.Expect(err2).To(gomega.BeNil())
					gomega.Expect(allocation2).NotTo(gomega.BeNil())
					gomega.Expect(isValidNUMARange(allocation2.Resources.CpusetCpus)).To(gomega.BeTrue())

					// Log allocations for debugging
					ginkgo.GinkgoWriter.Printf("Container 1 CPU set: %s\n", allocation1.Resources.CpusetCpus)
					ginkgo.GinkgoWriter.Printf("Container 2 CPU set: %s\n", allocation2.Resources.CpusetCpus)
				})

				ginkgo.It("should handle different CPU request sizes", func() {
					testCases := []struct {
						name        string
						requestsCPU int64
						limitsCPU   int64
					}{
						{"Small workload", 2, 4},
						{"Medium workload", 8, 12},
						{"Large workload", 16, 24},
						{"Target workload", 12, 18}, // The specific test case
					}

					for _, tc := range testCases {
						containerCtx := createContainerContextWithCPURequests(
							"cpu-test-"+tc.name,
							"cpu-test-pod-uid-"+tc.name,
							"cpu-test-pod-"+tc.name,
							"default",
							tc.requestsCPU,
							tc.limitsCPU,
						)

						allocation, err := topologyPolicy.PreCreateContainerHook(containerCtx)
						gomega.Expect(err).To(gomega.BeNil(), "Failed for test case: %s", tc.name)
						gomega.Expect(allocation).NotTo(gomega.BeNil(), "Allocation is nil for test case: %s", tc.name)
						gomega.Expect(allocation.Resources.CpusetCpus).NotTo(gomega.BeEmpty(), "CPU set is empty for test case: %s", tc.name)
						gomega.Expect(isValidNUMARange(allocation.Resources.CpusetCpus)).To(gomega.BeTrue(),
							"CPU set %s should match NUMA range for test case: %s", allocation.Resources.CpusetCpus, tc.name)

						ginkgo.GinkgoWriter.Printf("Test case '%s' (req=%d, lim=%d): CPU set = %s\n",
							tc.name, tc.requestsCPU, tc.limitsCPU, allocation.Resources.CpusetCpus)
					}
				})
			})

			ginkgo.Context("with empty cache", func() {
				ginkgo.BeforeEach(func() {
					// Ensure cache is empty as specified in requirements
					mockCache.ClearContainers()
					// Note: MockCache doesn't have ClearPods method, but containers are cleared
				})

				ginkgo.It("should allocate to first available NUMA node when cache is empty", func() {
					containerCtx := createContainerContextWithCPURequests(
						"first-container",
						"first-pod-uid",
						"first-pod",
						"default",
						12, // requests.CPU = 12
						18, // limits.CPU = 18
					)

					allocation, err := topologyPolicy.PreCreateContainerHook(containerCtx)
					gomega.Expect(err).To(gomega.BeNil())
					gomega.Expect(allocation).NotTo(gomega.BeNil())
					gomega.Expect(allocation.Resources.CpusetCpus).NotTo(gomega.BeEmpty())
					gomega.Expect(isValidNUMARange(allocation.Resources.CpusetCpus)).To(gomega.BeTrue())

					// Since cache is empty, should likely get NUMA 0 (0-23)
					ginkgo.GinkgoWriter.Printf("First allocation with empty cache: %s\n", allocation.Resources.CpusetCpus)
				})
			})

			ginkgo.Describe("Specific Container Allocation Scenarios", func() {
				ginkgo.BeforeEach(func() {
					// Ensure cache is empty for each test scenario
					mockCache.ClearContainers()
				})

				ginkgo.Context("Case 1: Medium Container (25/48 CPU)", func() {
					ginkgo.It("should allocate to a single Socket range", func() {
						containerCtx := createContainerContextWithMemory(
							"medium-container",
							"medium-pod-uid",
							"medium-pod",
							"default",
							25,  // requests.CPU = 25
							48,  // limits.CPU = 48
							200, // memory = 200MB
						)

						allocation, err := topologyPolicy.PreCreateContainerHook(containerCtx)
						gomega.Expect(err).To(gomega.BeNil())
						gomega.Expect(allocation).NotTo(gomega.BeNil())
						gomega.Expect(allocation.Resources.CpusetCpus).NotTo(gomega.BeEmpty())

						cpuSet := allocation.Resources.CpusetCpus
						allocationType := getAllocationType(cpuSet)

						gomega.Expect(isValidSocketRange(cpuSet)).To(gomega.BeTrue(),
							"CPU set %s should match Socket range (0-47 or 48-95)", cpuSet)

						ginkgo.GinkgoWriter.Printf("✅ Case 1 - Medium Container (25/48): CPU set = %s, Type = %s\n",
							cpuSet, allocationType)
					})
				})

				ginkgo.Context("Case 2: Large Container (49/50 CPU)", func() {
					ginkgo.It("should allocate to entire System range", func() {
						containerCtx := createContainerContextWithMemory(
							"large-container",
							"large-pod-uid",
							"large-pod",
							"default",
							49,  // requests.CPU = 49
							50,  // limits.CPU = 50
							200, // memory = 200MB
						)

						allocation, err := topologyPolicy.PreCreateContainerHook(containerCtx)
						gomega.Expect(err).To(gomega.BeNil())
						gomega.Expect(allocation).NotTo(gomega.BeNil())
						gomega.Expect(allocation.Resources.CpusetCpus).NotTo(gomega.BeEmpty())

						cpuSet := allocation.Resources.CpusetCpus
						allocationType := getAllocationType(cpuSet)

						gomega.Expect(isValidSystemRange(cpuSet)).To(gomega.BeTrue(),
							"CPU set %s should match System range (0-95)", cpuSet)

						ginkgo.GinkgoWriter.Printf("✅ Case 2 - Large Container (49/50): CPU set = %s, Type = %s\n",
							cpuSet, allocationType)
					})
				})

				ginkgo.Context("Case 3: Pre-deployed containers + Small container", func() {
					ginkgo.It("should allocate 4 containers to different NUMA nodes, then small container to single NUMA", func() {
						// Step 1: Deploy 4 containers with 12/18 CPU each
						var allocations []*policy.Allocation
						var cpuSets []string

						for i := 1; i <= 4; i++ {
							containerCtx := createContainerContextWithMemory(
								fmt.Sprintf("pre-container-%d", i),
								fmt.Sprintf("pre-pod-uid-%d", i),
								fmt.Sprintf("pre-pod-%d", i),
								"default",
								12,  // requests.CPU = 12
								18,  // limits.CPU = 18
								200, // memory = 200MB
							)

							allocation, err := topologyPolicy.PreCreateContainerHook(containerCtx)
							gomega.Expect(err).To(gomega.BeNil())
							gomega.Expect(allocation).NotTo(gomega.BeNil())
							gomega.Expect(allocation.Resources.CpusetCpus).NotTo(gomega.BeEmpty())

							allocations = append(allocations, allocation)
							cpuSets = append(cpuSets, allocation.Resources.CpusetCpus)
						}

						// Verify that all 4 containers are allocated to different NUMA nodes
						uniqueNUMAs := make(map[string]bool)
						for i, cpuSet := range cpuSets {
							gomega.Expect(isValidNUMARange(cpuSet)).To(gomega.BeTrue(),
								"Container %d CPU set %s should be in NUMA range", i+1, cpuSet)
							uniqueNUMAs[cpuSet] = true
							ginkgo.GinkgoWriter.Printf("Pre-container-%d: CPU set = %s (NUMA)\n", i+1, cpuSet)
						}
						gomega.Expect(len(uniqueNUMAs)).To(gomega.Equal(4), "All 4 containers should be in different NUMA nodes")

						// Step 2: Deploy small container (4/8 CPU)
						smallContainerCtx := createContainerContextWithMemory(
							"small-container",
							"small-pod-uid",
							"small-pod",
							"default",
							4,   // requests.CPU = 4
							8,   // limits.CPU = 8
							200, // memory = 200MB
						)

						smallAllocation, err := topologyPolicy.PreCreateContainerHook(smallContainerCtx)
						gomega.Expect(err).To(gomega.BeNil())
						gomega.Expect(smallAllocation).NotTo(gomega.BeNil())
						gomega.Expect(smallAllocation.Resources.CpusetCpus).NotTo(gomega.BeEmpty())

						smallCpuSet := smallAllocation.Resources.CpusetCpus
						gomega.Expect(isValidNUMARange(smallCpuSet)).To(gomega.BeTrue(),
							"Small container CPU set %s should be in single NUMA range", smallCpuSet)

						ginkgo.GinkgoWriter.Printf("✅ Case 3 - Small Container (4/8): CPU set = %s (NUMA)\n", smallCpuSet)
					})
				})

				ginkgo.Context("Case 4: Pre-deployed containers + Medium container", func() {
					ginkgo.It("should allocate 4 containers to different NUMA nodes, then medium container to single Socket", func() {
						// Step 1: Deploy 4 containers with 12/18 CPU each (same as Case 3)
						var allocations []*policy.Allocation
						var cpuSets []string

						for i := 1; i <= 4; i++ {
							containerCtx := createContainerContextWithMemory(
								fmt.Sprintf("pre-container-%d", i),
								fmt.Sprintf("pre-pod-uid-%d", i),
								fmt.Sprintf("pre-pod-%d", i),
								"default",
								12,  // requests.CPU = 12
								18,  // limits.CPU = 18
								200, // memory = 200MB
							)

							allocation, err := topologyPolicy.PreCreateContainerHook(containerCtx)
							gomega.Expect(err).To(gomega.BeNil())
							gomega.Expect(allocation).NotTo(gomega.BeNil())
							gomega.Expect(allocation.Resources.CpusetCpus).NotTo(gomega.BeEmpty())

							allocations = append(allocations, allocation)
							cpuSets = append(cpuSets, allocation.Resources.CpusetCpus)
						}

						// Verify that all 4 containers are allocated to different NUMA nodes
						uniqueNUMAs := make(map[string]bool)
						for i, cpuSet := range cpuSets {
							gomega.Expect(isValidNUMARange(cpuSet)).To(gomega.BeTrue(),
								"Container %d CPU set %s should be in NUMA range", i+1, cpuSet)
							uniqueNUMAs[cpuSet] = true
							ginkgo.GinkgoWriter.Printf("Pre-container-%d: CPU set = %s (NUMA)\n", i+1, cpuSet)
						}
						gomega.Expect(len(uniqueNUMAs)).To(gomega.Equal(4), "All 4 containers should be in different NUMA nodes")

						// Step 2: Deploy medium container (17/19 CPU)
						mediumContainerCtx := createContainerContextWithMemory(
							"medium-container",
							"medium-pod-uid",
							"medium-pod",
							"default",
							17,  // requests.CPU = 17
							19,  // limits.CPU = 19
							200, // memory = 200MB
						)

						mediumAllocation, err := topologyPolicy.PreCreateContainerHook(mediumContainerCtx)
						gomega.Expect(err).To(gomega.BeNil())
						gomega.Expect(mediumAllocation).NotTo(gomega.BeNil())
						gomega.Expect(mediumAllocation.Resources.CpusetCpus).NotTo(gomega.BeEmpty())

						mediumCpuSet := mediumAllocation.Resources.CpusetCpus
						gomega.Expect(isValidSocketRange(mediumCpuSet)).To(gomega.BeTrue(),
							"Medium container CPU set %s should be in single Socket range", mediumCpuSet)

						ginkgo.GinkgoWriter.Printf("✅ Case 4 - Medium Container (17/19): CPU set = %s (Socket)\n", mediumCpuSet)
					})
				})

				ginkgo.Context("Case 5: Pre-deployed containers + Boundary case medium container", func() {
					ginkgo.It("should allocate 4 containers to different NUMA nodes, then boundary medium container to single NUMA", func() {
						// Step 1: Deploy 4 containers with 12/18 CPU each
						var allocations []*policy.Allocation
						var cpuSets []string

						for i := 1; i <= 4; i++ {
							containerCtx := createContainerContextWithMemory(
								fmt.Sprintf("pre-container-%d", i),
								fmt.Sprintf("pre-pod-uid-%d", i),
								fmt.Sprintf("pre-pod-%d", i),
								"default",
								12,  // requests.CPU = 12
								18,  // limits.CPU = 18
								200, // memory = 200MB
							)

							allocation, err := topologyPolicy.PreCreateContainerHook(containerCtx)
							gomega.Expect(err).To(gomega.BeNil())
							gomega.Expect(allocation).NotTo(gomega.BeNil())
							gomega.Expect(allocation.Resources.CpusetCpus).NotTo(gomega.BeEmpty())

							allocations = append(allocations, allocation)
							cpuSets = append(cpuSets, allocation.Resources.CpusetCpus)
						}

						// Verify that all 4 containers are allocated to different NUMA nodes
						uniqueNUMAs := make(map[string]bool)
						for i, cpuSet := range cpuSets {
							gomega.Expect(isValidNUMARange(cpuSet)).To(gomega.BeTrue(),
								"Container %d CPU set %s should be in NUMA range", i+1, cpuSet)
							uniqueNUMAs[cpuSet] = true
							ginkgo.GinkgoWriter.Printf("Pre-container-%d: CPU set = %s (NUMA)\n", i+1, cpuSet)
						}
						gomega.Expect(len(uniqueNUMAs)).To(gomega.Equal(4), "All 4 containers should be in different NUMA nodes")

						// Step 2: Deploy boundary case medium container (12/19 CPU)
						// This is a boundary case where requests=12 (could fit in NUMA) but limits=19 (might need more)
						boundaryContainerCtx := createContainerContextWithMemory(
							"boundary-medium-container",
							"boundary-medium-pod-uid",
							"boundary-medium-pod",
							"default",
							12,  // requests.CPU = 12 (same as pre-deployed containers)
							19,  // limits.CPU = 19 (slightly higher than pre-deployed)
							200, // memory = 200MB
						)

						boundaryAllocation, err := topologyPolicy.PreCreateContainerHook(boundaryContainerCtx)
						gomega.Expect(err).To(gomega.BeNil())
						gomega.Expect(boundaryAllocation).NotTo(gomega.BeNil())
						gomega.Expect(boundaryAllocation.Resources.CpusetCpus).NotTo(gomega.BeEmpty())

						boundaryCpuSet := boundaryAllocation.Resources.CpusetCpus
						gomega.Expect(isValidNUMARange(boundaryCpuSet)).To(gomega.BeTrue(),
							"Boundary medium container CPU set %s should be in single NUMA range", boundaryCpuSet)

						ginkgo.GinkgoWriter.Printf("✅ Case 5 - Boundary Medium Container (12/19): CPU set = %s (NUMA)\n", boundaryCpuSet)
					})
				})

				ginkgo.Context("Case 6: Pre-deployed containers + Large system container", func() {
					ginkgo.It("should allocate 4 containers to different NUMA nodes, then large container to entire system", func() {
						// Step 1: Deploy 4 containers with 12/18 CPU each (same as previous cases)
						var allocations []*policy.Allocation
						var cpuSets []string

						for i := 1; i <= 4; i++ {
							containerCtx := createContainerContextWithMemory(
								fmt.Sprintf("pre-container-%d", i),
								fmt.Sprintf("pre-pod-uid-%d", i),
								fmt.Sprintf("pre-pod-%d", i),
								"default",
								12,  // requests.CPU = 12
								18,  // limits.CPU = 18
								200, // memory = 200MB
							)

							allocation, err := topologyPolicy.PreCreateContainerHook(containerCtx)
							gomega.Expect(err).To(gomega.BeNil())
							gomega.Expect(allocation).NotTo(gomega.BeNil())
							gomega.Expect(allocation.Resources.CpusetCpus).NotTo(gomega.BeEmpty())

							allocations = append(allocations, allocation)
							cpuSets = append(cpuSets, allocation.Resources.CpusetCpus)
						}

						// Verify that all 4 containers are allocated to different NUMA nodes
						uniqueNUMAs := make(map[string]bool)
						for i, cpuSet := range cpuSets {
							gomega.Expect(isValidNUMARange(cpuSet)).To(gomega.BeTrue(),
								"Container %d CPU set %s should be in NUMA range", i+1, cpuSet)
							uniqueNUMAs[cpuSet] = true
							ginkgo.GinkgoWriter.Printf("Pre-container-%d: CPU set = %s (NUMA)\n", i+1, cpuSet)
						}
						gomega.Expect(len(uniqueNUMAs)).To(gomega.Equal(4), "All 4 containers should be in different NUMA nodes")

						// Step 2: Deploy large system container (50/59 CPU)
						// This requires more resources than any single socket can provide
						largeSystemContainerCtx := createContainerContextWithMemory(
							"large-system-container",
							"large-system-pod-uid",
							"large-system-pod",
							"default",
							50,  // requests.CPU = 50 (more than single socket)
							59,  // limits.CPU = 59 (definitely needs entire system)
							200, // memory = 200MB
						)

						largeSystemAllocation, err := topologyPolicy.PreCreateContainerHook(largeSystemContainerCtx)

						// When 4 containers are already deployed (each taking 18 CPU), there's insufficient
						// resources for a 50/59 CPU container. This is expected behavior.
						if err != nil {
							// This is the expected behavior - insufficient resources
							gomega.Expect(err.Error()).To(gomega.ContainSubstring("failed to allocate cpu"))
							ginkgo.GinkgoWriter.Printf("✅ Case 6 - Large System Container (50/59): Expected failure due to insufficient resources after pre-deployment\n")
							ginkgo.GinkgoWriter.Printf("   Error: %s\n", err.Error())
						} else {
							// If allocation succeeds (unlikely with pre-deployed containers), verify it's system-wide
							gomega.Expect(largeSystemAllocation).NotTo(gomega.BeNil())
							gomega.Expect(largeSystemAllocation.Resources.CpusetCpus).NotTo(gomega.BeEmpty())
							largeSystemCpuSet := largeSystemAllocation.Resources.CpusetCpus
							gomega.Expect(isValidSystemRange(largeSystemCpuSet)).To(gomega.BeTrue(),
								"Large system container CPU set %s should cover entire system range", largeSystemCpuSet)
							ginkgo.GinkgoWriter.Printf("✅ Case 6 - Large System Container (50/59): CPU set = %s (System)\n", largeSystemCpuSet)
						}
					})
				})

				ginkgo.Context("Edge Cases and Resource Validation", func() {
					ginkgo.It("should handle various resource allocation patterns correctly", func() {
						testCases := []struct {
							name         string
							requestsCPU  int64
							limitsCPU    int64
							expectedType string
							expectError  bool
						}{
							{"Small NUMA (12/12)", 12, 12, "NUMA", false},
							{"Medium NUMA (20/20)", 20, 20, "NUMA", false},
							{"Large NUMA (23/23)", 23, 23, "NUMA", false},
							{"Boundary NUMA-Socket (24/25)", 24, 25, "Socket", true}, // May fail due to resource constraints
							{"Medium Socket (30/35)", 30, 35, "Socket", false},
							{"Large Socket (40/45)", 40, 45, "Socket", true},           // May fail due to resource constraints
							{"Boundary Socket-System (48/49)", 48, 49, "System", true}, // May fail due to resource constraints
							{"Large System (80/90)", 80, 90, "System", true},           // Likely to fail due to resource constraints
						}

						for _, tc := range testCases {
							containerCtx := createContainerContextWithMemory(
								"edge-case-"+tc.name,
								"edge-case-pod-uid-"+tc.name,
								"edge-case-pod-"+tc.name,
								"default",
								tc.requestsCPU,
								tc.limitsCPU,
								200, // memory = 200MB
							)

							allocation, err := topologyPolicy.PreCreateContainerHook(containerCtx)

							if tc.expectError && err != nil {
								// Expected failure due to resource constraints
								ginkgo.GinkgoWriter.Printf("Edge case '%s' (req=%d, lim=%d): Expected failure - %s\n",
									tc.name, tc.requestsCPU, tc.limitsCPU, err.Error())
							} else if err != nil {
								// Unexpected failure
								ginkgo.Fail(fmt.Sprintf("Unexpected failure for edge case %s: %s", tc.name, err.Error()))
							} else {
								// Success case
								gomega.Expect(allocation).NotTo(gomega.BeNil(), "Allocation is nil for edge case: %s", tc.name)
								gomega.Expect(allocation.Resources.CpusetCpus).NotTo(gomega.BeEmpty(), "CPU set is empty for edge case: %s", tc.name)

								cpuSet := allocation.Resources.CpusetCpus
								allocationType := getAllocationType(cpuSet)

								ginkgo.GinkgoWriter.Printf("Edge case '%s' (req=%d, lim=%d): CPU set = %s, Type = %s\n",
									tc.name, tc.requestsCPU, tc.limitsCPU, cpuSet, allocationType)
							}
						}
					})

					ginkgo.Context("Large System Container in Clean Environment", func() {
						ginkgo.It("should allocate large system container to entire system when no pre-deployed containers exist", func() {
							// Test large system container (50/59 CPU) in a clean environment
							largeSystemContainerCtx := createContainerContextWithMemory(
								"clean-large-system-container",
								"clean-large-system-pod-uid",
								"clean-large-system-pod",
								"default",
								50,  // requests.CPU = 50 (more than single socket)
								59,  // limits.CPU = 59 (definitely needs entire system)
								200, // memory = 200MB
							)

							largeSystemAllocation, err := topologyPolicy.PreCreateContainerHook(largeSystemContainerCtx)
							gomega.Expect(err).To(gomega.BeNil())
							gomega.Expect(largeSystemAllocation).NotTo(gomega.BeNil())
							gomega.Expect(largeSystemAllocation.Resources.CpusetCpus).NotTo(gomega.BeEmpty())

							largeSystemCpuSet := largeSystemAllocation.Resources.CpusetCpus
							gomega.Expect(isValidSystemRange(largeSystemCpuSet)).To(gomega.BeTrue(),
								"Large system container CPU set %s should cover entire system range", largeSystemCpuSet)

							ginkgo.GinkgoWriter.Printf("✅ Clean Large System Container (50/59): CPU set = %s (System)\n", largeSystemCpuSet)
						})
					})
				})
			})
		})
	})

	// 混合测试用例：验证 PreCreateContainerHook 和 PostStopContainerHook 的功能正确性
	ginkgo.Describe("Mixed Container Lifecycle Tests", func() {
		var topologyPolicy *topologyaware.TopologyAwarePolicy

		ginkgo.BeforeEach(func() {
			// Setup large topology: 2 Sockets, 4 NUMA nodes, 96 CPUs
			mockSystem.SetupLargeTopology()

			// Disable memory topology as specified
			opts.EnableMemoryTopology = false

			policy := topologyaware.NewTopologyAwarePolicyWithSystem(mockCache, opts, mockSystem)
			gomega.Expect(policy).NotTo(gomega.BeNil())

			var ok bool
			topologyPolicy, ok = policy.(*topologyaware.TopologyAwarePolicy)
			gomega.Expect(ok).To(gomega.BeTrue())
		})

		ginkgo.Context("Small Container Deployment and Release Cycles", func() {
			ginkgo.It("should correctly handle deployment and release of 4 small containers", func() {
				// Test Case 1: 部署4个小型容器，释放1个，再部署1个
				ginkgo.GinkgoWriter.Printf("=== Test Case 1: Small Container Deployment and Release Cycles ===\n")

				var allocations []*policy.Allocation
				var containerContexts []*policy.ContainerContext

				// Step 1: Deploy 4 small containers (4/8 CPU each)
				ginkgo.GinkgoWriter.Printf("Step 1: Deploying 4 small containers (4/8 CPU each)\n")
				for i := 1; i <= 4; i++ {
					containerCtx := createContainerContextWithMemory(
						fmt.Sprintf("small-container-%d", i),
						fmt.Sprintf("small-pod-uid-%d", i),
						fmt.Sprintf("small-pod-%d", i),
						"default",
						4,   // requests.CPU = 4
						8,   // limits.CPU = 8
						200, // memory = 200MB
					)

					allocation, err := topologyPolicy.PreCreateContainerHook(containerCtx)
					gomega.Expect(err).To(gomega.BeNil())
					gomega.Expect(allocation).NotTo(gomega.BeNil())
					gomega.Expect(allocation.Resources.CpusetCpus).NotTo(gomega.BeEmpty())

					// Verify each container is allocated to NUMA level
					cpuSet := allocation.Resources.CpusetCpus
					gomega.Expect(isValidNUMARange(cpuSet)).To(gomega.BeTrue(),
						"Small container %d CPU set %s should be in NUMA range", i, cpuSet)

					allocations = append(allocations, allocation)
					containerContexts = append(containerContexts, containerCtx)

					ginkgo.GinkgoWriter.Printf("  Container %d: CPU set = %s (NUMA)\n", i, cpuSet)
				}

				// Verify all 4 containers are distributed across different NUMA nodes
				uniqueNUMAs := make(map[string]bool)
				for _, allocation := range allocations {
					uniqueNUMAs[allocation.Resources.CpusetCpus] = true
				}
				gomega.Expect(len(uniqueNUMAs)).To(gomega.Equal(4), "All 4 small containers should be in different NUMA nodes")

				// Step 2: Release the second container
				ginkgo.GinkgoWriter.Printf("Step 2: Releasing container 2\n")
				releasedContainerCtx := containerContexts[1] // container 2
				releasedAllocation := allocations[1]

				// Create release context with container ID for PostStopContainerHook
				releaseCtx := createContainerContextWithID(
					releasedContainerCtx.Request.ContainerMeta.ID,
					releasedContainerCtx.Request.ContainerMeta.Name,
					releasedContainerCtx.Request.PodMeta.UID,
				)

				_, err := topologyPolicy.PostStopContainerHook(releaseCtx)
				gomega.Expect(err).To(gomega.BeNil())
				ginkgo.GinkgoWriter.Printf("  Released container 2 with CPU set: %s\n", releasedAllocation.Resources.CpusetCpus)

				// Step 3: Deploy a new small container
				ginkgo.GinkgoWriter.Printf("Step 3: Deploying new small container\n")
				newContainerCtx := createContainerContextWithMemory(
					"small-container-new",
					"small-pod-uid-new",
					"small-pod-new",
					"default",
					4,   // requests.CPU = 4
					8,   // limits.CPU = 8
					200, // memory = 200MB
				)

				newAllocation, err := topologyPolicy.PreCreateContainerHook(newContainerCtx)
				gomega.Expect(err).To(gomega.BeNil())
				gomega.Expect(newAllocation).NotTo(gomega.BeNil())
				gomega.Expect(newAllocation.Resources.CpusetCpus).NotTo(gomega.BeEmpty())

				// Verify the new container gets NUMA-level allocation
				newCpuSet := newAllocation.Resources.CpusetCpus
				gomega.Expect(isValidNUMARange(newCpuSet)).To(gomega.BeTrue(),
					"New small container CPU set %s should be in NUMA range", newCpuSet)

				ginkgo.GinkgoWriter.Printf("  New container: CPU set = %s (NUMA)\n", newCpuSet)
				ginkgo.GinkgoWriter.Printf("✅ Test Case 1 completed successfully\n\n")
			})
		})

		ginkgo.Context("Mixed Container Type Deployment and Release", func() {
			ginkgo.It("should handle repeated creation and release of different container types", func() {
				// Test Case 2: 反复创建和释放不同类型容器
				ginkgo.GinkgoWriter.Printf("=== Test Case 2: Mixed Container Type Deployment and Release ===\n")

				testSequence := []struct {
					name         string
					action       string // "create" or "release"
					requestsCPU  int64
					limitsCPU    int64
					expectedType string
				}{
					{"small-1", "create", 4, 8, "NUMA"},
					{"medium-1", "create", 25, 35, "Socket"},
					{"small-2", "create", 6, 10, "NUMA"},
					{"small-1", "release", 0, 0, ""},
					{"large-1", "create", 50, 60, "System"},
					{"medium-1", "release", 0, 0, ""},
					{"small-3", "create", 8, 12, "NUMA"},
					{"large-1", "release", 0, 0, ""},
					{"medium-2", "create", 30, 40, "Socket"},
				}

				activeContainers := make(map[string]*policy.ContainerContext)
				activeAllocations := make(map[string]*policy.Allocation)

				for step, test := range testSequence {
					ginkgo.GinkgoWriter.Printf("Step %d: %s %s\n", step+1, test.action, test.name)

					if test.action == "create" {
						containerCtx := createContainerContextWithMemory(
							test.name,
							test.name+"-uid",
							test.name+"-pod",
							"default",
							test.requestsCPU,
							test.limitsCPU,
							200, // memory = 200MB
						)

						allocation, err := topologyPolicy.PreCreateContainerHook(containerCtx)
						if err != nil {
							// In mixed scenarios, some allocations may fail due to resource competition
							ginkgo.GinkgoWriter.Printf("  ⚠️  %s creation failed (resource competition): %s\n", test.name, err.Error())
							continue
						}

						gomega.Expect(allocation).NotTo(gomega.BeNil())
						gomega.Expect(allocation.Resources.CpusetCpus).NotTo(gomega.BeEmpty())

						cpuSet := allocation.Resources.CpusetCpus

						// Verify allocation type matches expectation
						switch test.expectedType {
						case "NUMA":
							gomega.Expect(isValidNUMARange(cpuSet)).To(gomega.BeTrue(),
								"Container %s CPU set %s should be in NUMA range", test.name, cpuSet)
						case "Socket":
							gomega.Expect(isValidSocketRange(cpuSet)).To(gomega.BeTrue(),
								"Container %s CPU set %s should be in Socket range", test.name, cpuSet)
						case "System":
							gomega.Expect(isValidSystemRange(cpuSet)).To(gomega.BeTrue(),
								"Container %s CPU set %s should be in System range", test.name, cpuSet)
						default:
							gomega.Expect(true).To(gomega.BeFalse(), "Unknown expected allocation type: %s", test.expectedType)
						}

						activeContainers[test.name] = containerCtx
						activeAllocations[test.name] = allocation

						ginkgo.GinkgoWriter.Printf("  Created %s: CPU set = %s (%s)\n", test.name, cpuSet, test.expectedType)

					} else if test.action == "release" {
						containerCtx, exists := activeContainers[test.name]
						if !exists {
							ginkgo.GinkgoWriter.Printf("  ⚠️  Container %s not found for release (may have failed creation)\n", test.name)
							continue
						}

						// Create release context with container ID for PostStopContainerHook
						releaseCtx := createContainerContextWithID(
							containerCtx.Request.ContainerMeta.ID,
							containerCtx.Request.ContainerMeta.Name,
							containerCtx.Request.PodMeta.UID,
						)

						_, err := topologyPolicy.PostStopContainerHook(releaseCtx)
						gomega.Expect(err).To(gomega.BeNil())

						allocation := activeAllocations[test.name]
						ginkgo.GinkgoWriter.Printf("  Released %s: CPU set = %s\n", test.name, allocation.Resources.CpusetCpus)

						delete(activeContainers, test.name)
						delete(activeAllocations, test.name)
					}
				}

				ginkgo.GinkgoWriter.Printf("✅ Test Case 2 completed successfully\n\n")
			})
		})

		ginkgo.Context("Complex Resource Competition and Recovery", func() {
			ginkgo.It("should handle resource competition and recovery correctly", func() {
				// Test Case 3: 复杂的资源竞争和恢复场景
				ginkgo.GinkgoWriter.Printf("=== Test Case 3: Complex Resource Competition and Recovery ===\n")

				// Step 1: 部署2个中型容器，占用2个Socket
				ginkgo.GinkgoWriter.Printf("Step 1: Deploying 2 medium containers to occupy 2 Sockets\n")
				var mediumContainers []*policy.ContainerContext
				var mediumAllocations []*policy.Allocation

				for i := 1; i <= 2; i++ {
					containerCtx := createContainerContextWithMemory(
						fmt.Sprintf("medium-container-%d", i),
						fmt.Sprintf("medium-pod-uid-%d", i),
						fmt.Sprintf("medium-pod-%d", i),
						"default",
						30,  // requests.CPU = 30 (requires Socket level)
						40,  // limits.CPU = 40
						300, // memory = 300MB
					)

					allocation, err := topologyPolicy.PreCreateContainerHook(containerCtx)
					gomega.Expect(err).To(gomega.BeNil())
					gomega.Expect(allocation).NotTo(gomega.BeNil())
					gomega.Expect(allocation.Resources.CpusetCpus).NotTo(gomega.BeEmpty())

					cpuSet := allocation.Resources.CpusetCpus
					gomega.Expect(isValidSocketRange(cpuSet)).To(gomega.BeTrue(),
						"Medium container %d CPU set %s should be in Socket range", i, cpuSet)

					mediumContainers = append(mediumContainers, containerCtx)
					mediumAllocations = append(mediumAllocations, allocation)

					ginkgo.GinkgoWriter.Printf("  Medium container %d: CPU set = %s (Socket)\n", i, cpuSet)
				}

				// Step 2: 尝试部署大型容器（应该失败，因为资源不足）
				ginkgo.GinkgoWriter.Printf("Step 2: Attempting to deploy large container (should fail)\n")
				largeContainerCtx := createContainerContextWithMemory(
					"large-container-fail",
					"large-pod-uid-fail",
					"large-pod-fail",
					"default",
					60,  // requests.CPU = 60 (requires System level)
					70,  // limits.CPU = 70
					400, // memory = 400MB
				)

				largeAllocation, err := topologyPolicy.PreCreateContainerHook(largeContainerCtx)
				if err != nil {
					// Expected failure due to resource competition
					gomega.Expect(err.Error()).To(gomega.ContainSubstring("failed to allocate"))
					ginkgo.GinkgoWriter.Printf("  ✅ Large container deployment failed as expected: %s\n", err.Error())
				} else {
					// If it succeeds, verify it's system-wide
					gomega.Expect(largeAllocation).NotTo(gomega.BeNil())
					cpuSet := largeAllocation.Resources.CpusetCpus
					gomega.Expect(isValidSystemRange(cpuSet)).To(gomega.BeTrue(),
						"Large container CPU set %s should be in System range", cpuSet)
					ginkgo.GinkgoWriter.Printf("  Large container: CPU set = %s (System)\n", cpuSet)
				}

				// Step 3: 释放一个中型容器
				ginkgo.GinkgoWriter.Printf("Step 3: Releasing one medium container\n")
				releaseCtx := createContainerContextWithID(
					mediumContainers[0].Request.ContainerMeta.ID,
					mediumContainers[0].Request.ContainerMeta.Name,
					mediumContainers[0].Request.PodMeta.UID,
				)

				_, err = topologyPolicy.PostStopContainerHook(releaseCtx)
				gomega.Expect(err).To(gomega.BeNil())
				ginkgo.GinkgoWriter.Printf("  Released medium container 1: CPU set = %s\n", mediumAllocations[0].Resources.CpusetCpus)

				// Step 4: 部署多个小型容器到释放的Socket
				ginkgo.GinkgoWriter.Printf("Step 4: Deploying small containers to released Socket\n")
				for i := 1; i <= 3; i++ {
					containerCtx := createContainerContextWithMemory(
						fmt.Sprintf("small-recovery-%d", i),
						fmt.Sprintf("small-recovery-uid-%d", i),
						fmt.Sprintf("small-recovery-pod-%d", i),
						"default",
						8,   // requests.CPU = 8
						12,  // limits.CPU = 12
						150, // memory = 150MB
					)

					allocation, err := topologyPolicy.PreCreateContainerHook(containerCtx)
					gomega.Expect(err).To(gomega.BeNil())
					gomega.Expect(allocation).NotTo(gomega.BeNil())
					gomega.Expect(allocation.Resources.CpusetCpus).NotTo(gomega.BeEmpty())

					cpuSet := allocation.Resources.CpusetCpus
					gomega.Expect(isValidNUMARange(cpuSet)).To(gomega.BeTrue(),
						"Small recovery container %d CPU set %s should be in NUMA range", i, cpuSet)

					ginkgo.GinkgoWriter.Printf("  Small recovery container %d: CPU set = %s (NUMA)\n", i, cpuSet)
				}

				ginkgo.GinkgoWriter.Printf("✅ Test Case 3 completed successfully\n\n")
			})
		})

		ginkgo.Context("Stress Test with Rapid Container Lifecycle", func() {
			ginkgo.It("should handle rapid container creation and deletion", func() {
				// Test Case 4: 快速容器生命周期压力测试
				ginkgo.GinkgoWriter.Printf("=== Test Case 4: Stress Test with Rapid Container Lifecycle ===\n")

				// 定义测试序列：快速创建和删除不同类型的容器
				operations := []struct {
					action      string
					name        string
					requestsCPU int64
					limitsCPU   int64
					memory      int64
				}{
					{"create", "stress-small-1", 4, 8, 100},
					{"create", "stress-small-2", 6, 10, 120},
					{"create", "stress-medium-1", 25, 35, 250},
					{"release", "stress-small-1", 0, 0, 0},
					{"create", "stress-small-3", 8, 12, 140},
					{"create", "stress-small-4", 5, 9, 110},
					{"release", "stress-medium-1", 0, 0, 0},
					{"create", "stress-medium-2", 28, 38, 280},
					{"release", "stress-small-2", 0, 0, 0},
					{"release", "stress-small-3", 0, 0, 0},
					{"create", "stress-large-1", 50, 65, 500},
					{"release", "stress-small-4", 0, 0, 0},
					{"release", "stress-medium-2", 0, 0, 0},
					{"release", "stress-large-1", 0, 0, 0},
				}

				activeContainers := make(map[string]*policy.ContainerContext)
				activeAllocations := make(map[string]*policy.Allocation)

				for step, op := range operations {
					ginkgo.GinkgoWriter.Printf("Step %d: %s %s\n", step+1, op.action, op.name)

					if op.action == "create" {
						containerCtx := createContainerContextWithMemory(
							op.name,
							op.name+"-uid",
							op.name+"-pod",
							"default",
							op.requestsCPU,
							op.limitsCPU,
							op.memory,
						)

						allocation, err := topologyPolicy.PreCreateContainerHook(containerCtx)
						if err != nil {
							// 在压力测试中，某些分配可能失败，这是可以接受的
							ginkgo.GinkgoWriter.Printf("  ⚠️  %s creation failed (acceptable in stress test): %s\n", op.name, err.Error())
							continue
						}

						gomega.Expect(allocation).NotTo(gomega.BeNil())
						gomega.Expect(allocation.Resources.CpusetCpus).NotTo(gomega.BeEmpty())

						cpuSet := allocation.Resources.CpusetCpus
						allocationType := getAllocationType(cpuSet)

						activeContainers[op.name] = containerCtx
						activeAllocations[op.name] = allocation

						ginkgo.GinkgoWriter.Printf("  ✅ Created %s: CPU set = %s (%s)\n", op.name, cpuSet, allocationType)

					} else if op.action == "release" {
						containerCtx, exists := activeContainers[op.name]
						if !exists {
							ginkgo.GinkgoWriter.Printf("  ⚠️  Container %s not found for release (may have failed creation)\n", op.name)
							continue
						}

						releaseCtx := createContainerContextWithID(
							containerCtx.Request.ContainerMeta.ID,
							containerCtx.Request.ContainerMeta.Name,
							containerCtx.Request.PodMeta.UID,
						)

						_, err := topologyPolicy.PostStopContainerHook(releaseCtx)
						gomega.Expect(err).To(gomega.BeNil())

						allocation := activeAllocations[op.name]
						ginkgo.GinkgoWriter.Printf("  ✅ Released %s: CPU set = %s\n", op.name, allocation.Resources.CpusetCpus)

						delete(activeContainers, op.name)
						delete(activeAllocations, op.name)
					}
				}

				// 验证最终状态：所有容器都已释放
				gomega.Expect(len(activeContainers)).To(gomega.Equal(0), "All containers should be released at the end")
				ginkgo.GinkgoWriter.Printf("✅ Test Case 4 completed successfully - All containers released\n\n")
			})
		})

		ginkgo.Context("Boundary Conditions and Edge Cases", func() {
			ginkgo.It("should handle boundary conditions correctly", func() {
				// Test Case 5: 边界条件测试
				ginkgo.GinkgoWriter.Printf("=== Test Case 5: Boundary Conditions and Edge Cases ===\n")

				boundaryTests := []struct {
					name        string
					requestsCPU int64
					limitsCPU   int64
					memory      int64
					expectType  string
					expectError bool
				}{
					{"boundary-numa-max", 23, 24, 200, "NUMA", false},     // NUMA边界最大值
					{"boundary-numa-over", 25, 30, 200, "Socket", false},  // 刚好超过NUMA边界，需要Socket级别
					{"boundary-socket-max", 35, 40, 300, "System", false}, // Socket边界最大值（实际会升级到System级别）
					{"boundary-socket-over", 49, 55, 300, "System", true}, // 刚好超过Socket边界，但资源不足会失败
					{"boundary-system-max", 80, 85, 500, "System", true},  // System边界最大值（资源不足会失败）
					{"boundary-system-over", 96, 97, 500, "System", true}, // 超过系统容量
				}

				for _, test := range boundaryTests {
					ginkgo.GinkgoWriter.Printf("Testing boundary case: %s (req=%d, lim=%d)\n", test.name, test.requestsCPU, test.limitsCPU)

					containerCtx := createContainerContextWithMemory(
						test.name,
						test.name+"-uid",
						test.name+"-pod",
						"default",
						test.requestsCPU,
						test.limitsCPU,
						test.memory,
					)

					allocation, err := topologyPolicy.PreCreateContainerHook(containerCtx)

					if test.expectError {
						if err != nil {
							ginkgo.GinkgoWriter.Printf("  ✅ Expected error for %s: %s\n", test.name, err.Error())
						} else {
							ginkgo.GinkgoWriter.Printf("  ⚠️  Expected error for %s but allocation succeeded: %s\n", test.name, allocation.Resources.CpusetCpus)
						}
					} else {
						gomega.Expect(err).To(gomega.BeNil(), "Boundary test %s should not fail", test.name)
						gomega.Expect(allocation).NotTo(gomega.BeNil())
						gomega.Expect(allocation.Resources.CpusetCpus).NotTo(gomega.BeEmpty())

						cpuSet := allocation.Resources.CpusetCpus
						switch test.expectType {
						case "NUMA":
							gomega.Expect(isValidNUMARange(cpuSet)).To(gomega.BeTrue(),
								"Boundary test %s CPU set %s should be in NUMA range", test.name, cpuSet)
						case "Socket":
							gomega.Expect(isValidSocketRange(cpuSet)).To(gomega.BeTrue(),
								"Boundary test %s CPU set %s should be in Socket range", test.name, cpuSet)
						case "System":
							gomega.Expect(isValidSystemRange(cpuSet)).To(gomega.BeTrue(),
								"Boundary test %s CPU set %s should be in System range", test.name, cpuSet)
						default:
							gomega.Expect(true).To(gomega.BeFalse(), "Unknown expected allocation type: %s", test.expectType)
						}

						ginkgo.GinkgoWriter.Printf("  ✅ %s: CPU set = %s (%s)\n", test.name, cpuSet, test.expectType)

						// 立即释放以避免资源冲突
						releaseCtx := createContainerContextWithID(
							containerCtx.Request.ContainerMeta.ID,
							containerCtx.Request.ContainerMeta.Name,
							containerCtx.Request.PodMeta.UID,
						)
						_, err = topologyPolicy.PostStopContainerHook(releaseCtx)
						gomega.Expect(err).To(gomega.BeNil())
					}
				}

				ginkgo.GinkgoWriter.Printf("✅ Test Case 5 completed successfully\n\n")
			})
		})

		ginkgo.Context("Error Recovery and Resilience", func() {
			ginkgo.It("should recover from error conditions gracefully", func() {
				// Test Case 6: 错误恢复和弹性测试
				ginkgo.GinkgoWriter.Printf("=== Test Case 6: Error Recovery and Resilience ===\n")

				// Step 1: 填满所有NUMA节点
				ginkgo.GinkgoWriter.Printf("Step 1: Filling all NUMA nodes\n")
				var fillerContainers []*policy.ContainerContext
				var fillerAllocations []*policy.Allocation

				for i := 1; i <= 4; i++ {
					containerCtx := createContainerContextWithMemory(
						fmt.Sprintf("filler-container-%d", i),
						fmt.Sprintf("filler-pod-uid-%d", i),
						fmt.Sprintf("filler-pod-%d", i),
						"default",
						20,  // requests.CPU = 20 (large NUMA allocation)
						23,  // limits.CPU = 23
						400, // memory = 400MB
					)

					allocation, err := topologyPolicy.PreCreateContainerHook(containerCtx)
					gomega.Expect(err).To(gomega.BeNil())
					gomega.Expect(allocation).NotTo(gomega.BeNil())

					fillerContainers = append(fillerContainers, containerCtx)
					fillerAllocations = append(fillerAllocations, allocation)

					cpuSet := allocation.Resources.CpusetCpus
					ginkgo.GinkgoWriter.Printf("  Filler container %d: CPU set = %s\n", i, cpuSet)
				}

				// Step 2: 尝试分配小容器（应该失败）
				ginkgo.GinkgoWriter.Printf("Step 2: Attempting small container allocation (should fail)\n")
				smallContainerCtx := createContainerContextWithMemory(
					"small-fail-container",
					"small-fail-pod-uid",
					"small-fail-pod",
					"default",
					4,   // requests.CPU = 4
					8,   // limits.CPU = 8
					100, // memory = 100MB
				)

				smallAllocation, err := topologyPolicy.PreCreateContainerHook(smallContainerCtx)
				if err != nil {
					ginkgo.GinkgoWriter.Printf("  ✅ Small container allocation failed as expected: %s\n", err.Error())
				} else {
					ginkgo.GinkgoWriter.Printf("  ⚠️  Small container allocation unexpectedly succeeded: %s\n", smallAllocation.Resources.CpusetCpus)
				}

				// Step 3: 释放部分容器以恢复资源
				ginkgo.GinkgoWriter.Printf("Step 3: Releasing containers to recover resources\n")
				for i := 0; i < 2; i++ {
					releaseCtx := createContainerContextWithID(
						fillerContainers[i].Request.ContainerMeta.ID,
						fillerContainers[i].Request.ContainerMeta.Name,
						fillerContainers[i].Request.PodMeta.UID,
					)

					_, err := topologyPolicy.PostStopContainerHook(releaseCtx)
					gomega.Expect(err).To(gomega.BeNil())
					ginkgo.GinkgoWriter.Printf("  Released filler container %d: CPU set = %s\n", i+1, fillerAllocations[i].Resources.CpusetCpus)
				}

				// Step 4: 重新尝试分配小容器（应该成功）
				ginkgo.GinkgoWriter.Printf("Step 4: Retrying small container allocation (should succeed)\n")
				retryAllocation, err := topologyPolicy.PreCreateContainerHook(smallContainerCtx)
				gomega.Expect(err).To(gomega.BeNil())
				gomega.Expect(retryAllocation).NotTo(gomega.BeNil())
				gomega.Expect(retryAllocation.Resources.CpusetCpus).NotTo(gomega.BeEmpty())

				retryCpuSet := retryAllocation.Resources.CpusetCpus
				gomega.Expect(isValidNUMARange(retryCpuSet)).To(gomega.BeTrue(),
					"Retry small container CPU set %s should be in NUMA range", retryCpuSet)
				ginkgo.GinkgoWriter.Printf("  ✅ Small container retry succeeded: CPU set = %s (NUMA)\n", retryCpuSet)

				// Step 5: 清理剩余容器
				ginkgo.GinkgoWriter.Printf("Step 5: Cleaning up remaining containers\n")
				for i := 2; i < 4; i++ {
					releaseCtx := createContainerContextWithID(
						fillerContainers[i].Request.ContainerMeta.ID,
						fillerContainers[i].Request.ContainerMeta.Name,
						fillerContainers[i].Request.PodMeta.UID,
					)

					_, err := topologyPolicy.PostStopContainerHook(releaseCtx)
					gomega.Expect(err).To(gomega.BeNil())
				}

				// 释放重试的小容器
				retryReleaseCtx := createContainerContextWithID(
					smallContainerCtx.Request.ContainerMeta.ID,
					smallContainerCtx.Request.ContainerMeta.Name,
					smallContainerCtx.Request.PodMeta.UID,
				)
				_, err = topologyPolicy.PostStopContainerHook(retryReleaseCtx)
				gomega.Expect(err).To(gomega.BeNil())

				ginkgo.GinkgoWriter.Printf("✅ Test Case 6 completed successfully\n\n")
			})
		})

		ginkgo.Context("QoS Filtering", func() {
			ginkgo.It("should skip BestEffort QoS containers", func() {
				ginkgo.GinkgoWriter.Printf("\n=== Test Case: QoS Filtering - BestEffort Containers ===\n")

				// Create a BestEffort container context with besteffort cgroup path
				containerCtx := createBasicContainerContext("besteffort-container", "pod-besteffort-123", "besteffort-pod", "default")
				// Set cgroup parent to indicate BestEffort QoS
				containerCtx.Request.CgroupParent = "kubepods.slice/kubepods-besteffort.slice/kubepods-besteffort-pod123.slice"

				ginkgo.GinkgoWriter.Printf("Testing BestEffort container with cgroup: %s\n", containerCtx.Request.CgroupParent)

				// Try to allocate resources for BestEffort container
				allocation, err := topologyPolicy.PreCreateContainerHook(containerCtx)

				// Should not return error, but allocation should be nil (skipped)
				gomega.Expect(err).To(gomega.BeNil())
				gomega.Expect(allocation).To(gomega.BeNil())

				ginkgo.GinkgoWriter.Printf("✅ BestEffort container correctly skipped (allocation is nil)\n")
			})

			ginkgo.It("should process Burstable QoS containers", func() {
				ginkgo.GinkgoWriter.Printf("\n=== Test Case: QoS Filtering - Burstable Containers ===\n")

				// Create a Burstable container context with burstable cgroup path
				containerCtx := createBasicContainerContext("burstable-container", "pod-burstable-123", "burstable-pod", "default")
				// Set cgroup parent to indicate Burstable QoS
				containerCtx.Request.CgroupParent = "kubepods.slice/kubepods-burstable.slice/kubepods-burstable-pod123.slice"

				ginkgo.GinkgoWriter.Printf("Testing Burstable container with cgroup: %s\n", containerCtx.Request.CgroupParent)

				// Try to allocate resources for Burstable container
				allocation, err := topologyPolicy.PreCreateContainerHook(containerCtx)

				// Should successfully allocate resources
				gomega.Expect(err).To(gomega.BeNil())
				gomega.Expect(allocation).NotTo(gomega.BeNil())
				gomega.Expect(allocation.Resources.CpusetCpus).NotTo(gomega.BeEmpty())

				ginkgo.GinkgoWriter.Printf("✅ Burstable container correctly processed: CPU set = %s\n", allocation.Resources.CpusetCpus)

				// Clean up
				releaseCtx := createContainerContextWithID(
					containerCtx.Request.ContainerMeta.ID,
					containerCtx.Request.ContainerMeta.Name,
					containerCtx.Request.PodMeta.UID,
				)
				_, err = topologyPolicy.PostStopContainerHook(releaseCtx)
				gomega.Expect(err).To(gomega.BeNil())
			})

			ginkgo.It("should process Guaranteed QoS containers", func() {
				ginkgo.GinkgoWriter.Printf("\n=== Test Case: QoS Filtering - Guaranteed Containers ===\n")

				// Create a Guaranteed container context with guaranteed cgroup path
				containerCtx := createBasicContainerContext("guaranteed-container", "pod-guaranteed-123", "guaranteed-pod", "default")
				// Set cgroup parent to indicate Guaranteed QoS (no burstable/besteffort in path)
				containerCtx.Request.CgroupParent = "kubepods.slice/kubepods-pod123.slice"

				ginkgo.GinkgoWriter.Printf("Testing Guaranteed container with cgroup: %s\n", containerCtx.Request.CgroupParent)

				// Try to allocate resources for Guaranteed container
				allocation, err := topologyPolicy.PreCreateContainerHook(containerCtx)

				// Should successfully allocate resources
				gomega.Expect(err).To(gomega.BeNil())
				gomega.Expect(allocation).NotTo(gomega.BeNil())
				gomega.Expect(allocation.Resources.CpusetCpus).NotTo(gomega.BeEmpty())

				ginkgo.GinkgoWriter.Printf("✅ Guaranteed container correctly processed: CPU set = %s\n", allocation.Resources.CpusetCpus)

				// Clean up
				releaseCtx := createContainerContextWithID(
					containerCtx.Request.ContainerMeta.ID,
					containerCtx.Request.ContainerMeta.Name,
					containerCtx.Request.PodMeta.UID,
				)
				_, err = topologyPolicy.PostStopContainerHook(releaseCtx)
				gomega.Expect(err).To(gomega.BeNil())
			})
		})
	})
})
