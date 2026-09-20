/*
 * Copyright (c) 2026 Huawei Technology corp.
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
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/utils/cpuset"
	"kunpeng.huawei.com/kunpeng-cloud-computing/pkg/kunpeng-tap/policy"
)

func TestAllocateRejectsMemoryOvercommitWithoutMutatingCPU(t *testing.T) {
	supply := newTestSupply(1024, 512)
	supply.grantedShared = 250
	supply.grantedCPUByRequest = 250
	supply.grantedCPUByLimit = 500
	request := newTestRequest(500, 1000, 513)

	grant, err := supply.Allocate(request, true)
	if err == nil {
		t.Fatal("expected memory capacity error")
	}
	if grant != nil {
		t.Fatalf("expected no grant, got %v", grant)
	}
	if !strings.Contains(err.Error(), "memory request 513 KB exceeds allocatable 512 KB on node test-node") {
		t.Fatalf("unexpected error: %v", err)
	}
	if supply.grantedShared != 250 || supply.grantedCPUByRequest != 250 || supply.grantedCPUByLimit != 500 {
		t.Fatalf("CPU state changed after rejected allocation: shared=%d request=%d limit=%d",
			supply.grantedShared, supply.grantedCPUByRequest, supply.grantedCPUByLimit)
	}
	if supply.grantedMemory != 512 {
		t.Fatalf("memory state changed after rejected allocation: got %d KB, want 512 KB", supply.grantedMemory)
	}
}

func TestAllocateAcceptsMemoryAtCapacityBoundary(t *testing.T) {
	supply := newTestSupply(1024, 256)
	request := newTestRequest(500, 1000, 768)

	grant, err := supply.Allocate(request, true)
	if err != nil {
		t.Fatalf("allocation failed: %v", err)
	}
	if grant.AllocatedMemory() != 768 {
		t.Fatalf("allocated memory = %d KB, want 768 KB", grant.AllocatedMemory())
	}
	if supply.grantedMemory != 1024 || supply.AllocatableMemory() != 0 {
		t.Fatalf("memory state after allocation: granted=%d KB allocatable=%d KB",
			supply.grantedMemory, supply.AllocatableMemory())
	}
	if supply.grantedShared != 500 || supply.grantedCPUByRequest != 500 || supply.grantedCPUByLimit != 1000 {
		t.Fatalf("unexpected CPU state: shared=%d request=%d limit=%d",
			supply.grantedShared, supply.grantedCPUByRequest, supply.grantedCPUByLimit)
	}
}

func TestAllocateAllowsMemoryOvercommitWhenCapacityCheckDisabled(t *testing.T) {
	supply := newTestSupply(512, 256)
	request := newTestRequest(500, 1000, 512)

	grant, err := supply.Allocate(request, false)
	if err != nil {
		t.Fatalf("allocation failed with memory capacity check disabled: %v", err)
	}
	if grant.AllocatedMemory() != 512 || supply.grantedMemory != 768 {
		t.Fatalf("unexpected memory state: grant=%d KB supply=%d KB",
			grant.AllocatedMemory(), supply.grantedMemory)
	}
	if supply.AllocatableMemory() != 0 {
		t.Fatalf("allocatable memory = %d KB, want 0 KB", supply.AllocatableMemory())
	}
}

func TestAllocateCPUFailureDoesNotMutateMemory(t *testing.T) {
	supply := newTestSupply(1024, 128)
	request := newTestRequest(2500, 2500, 256)

	grant, err := supply.Allocate(request, true)
	if err == nil {
		t.Fatal("expected CPU capacity error")
	}
	if grant != nil {
		t.Fatalf("expected no grant, got %v", grant)
	}
	if supply.grantedMemory != 128 {
		t.Fatalf("memory state changed after rejected CPU allocation: got %d KB, want 128 KB", supply.grantedMemory)
	}
}

func newTestSupply(memoryTotal, grantedMemory uint64) *supply {
	return &supply{
		node:          &baseNode{name: "test-node", kind: NumaNode},
		sharable:      cpuset.New(0, 1),
		memoryTotal:   memoryTotal,
		grantedMemory: grantedMemory,
	}
}

func newTestRequest(cpuRequest, cpuLimit, memoryLimitKB int64) Request {
	cpuRequestQuantity := resource.NewMilliQuantity(cpuRequest, resource.DecimalSI)
	cpuLimitQuantity := resource.NewMilliQuantity(cpuLimit, resource.DecimalSI)
	memoryLimitQuantity := resource.NewQuantity(memoryLimitKB*1024, resource.BinarySI)

	return newRequest(policy.ContainerContext{
		Request: policy.ContainerRequest{
			Resources: &policy.Resources{
				EstimatedRequirements: &corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU: *cpuRequestQuantity,
					},
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    *cpuLimitQuantity,
						corev1.ResourceMemory: *memoryLimitQuantity,
					},
				},
			},
		},
	})
}
