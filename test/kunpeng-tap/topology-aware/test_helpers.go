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
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"kunpeng.huawei.com/kunpeng-cloud-computing/pkg/kunpeng-tap/policy"
)

// ==================== Basic Helper Functions ====================

// stringPtr returns a pointer to the given string
func stringPtr(s string) *string {
	return &s
}

// int64Ptr returns a pointer to the given int64
func int64Ptr(i int64) *int64 {
	return &i
}

// ==================== CPU List Parsing Functions ====================

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

// ==================== Container Context Creation Functions ====================

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
