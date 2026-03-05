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

	"k8s.io/klog/v2"
	"kunpeng.huawei.com/kunpeng-cloud-computing/cmd/kunpeng-tap/proxy/options"
	"kunpeng.huawei.com/kunpeng-cloud-computing/pkg/kunpeng-tap/sysfs/system"
)

// filterPoolsByCapacity filters pools that have sufficient capacity for the request.
// This is the first step in the filter-first-then-sort allocation strategy.
func (p *TopologyAwarePolicy) filterPoolsByCapacity(request Request) []Node {
	var filteredPools []Node
	var rootPool Node

	for _, pool := range p.pools {
		// Track root separately - it should only be used as a last resort
		// when no sub-node (cluster/numa/socket) has sufficient capacity.
		if pool == p.root {
			rootPool = pool
			continue
		}
		if p.hasCapacity(request, pool) {
			filteredPools = append(filteredPools, pool)
		} else {
			klog.V(3).InfoS("Pool rejected by capacity filter",
				"pool", pool.Name(),
				"kind", pool.Kind(),
				"depth", pool.RootDistance(),
				"request", request.String())
		}
	}

	// Fall back to root only when no sub-node can accommodate the request.
	// This handles large containers that exceed any single socket's capacity.
	if len(filteredPools) == 0 && rootPool != nil {
		filteredPools = append(filteredPools, rootPool)
		klog.InfoS("WARNING: No sub-node has sufficient capacity, falling back to root",
			"request", request.String(),
			"cpuRequest", request.CPURequest(),
			"cpuLimit", request.CPULimit(),
			"memoryLimit", request.MemoryLimit())
	}

	klog.V(3).InfoS("Filtered pools by capacity",
		"totalPools", len(p.pools),
		"filteredPools", len(filteredPools),
		"request", request.String())

	return filteredPools
}

// findBestAvailablePool finds the best available pool for the request.
func (p *TopologyAwarePolicy) findBestAvailablePool(request Request, pools []Node) Node {
	// GPU-First策略下的特殊处理
	if p.resourcePriority == options.ResourcePriorityGPUFirst && request.HasGPURequest() {
		if pool := p.findGPUAffinityPool(request, pools); pool != nil {
			return pool
		}
		klog.V(4).InfoS("GPU-First: No GPU NUMA pools with sufficient capacity found, falling back to regular allocation")
	}

	// 常规处理逻辑
	return p.findRegularPool(request, pools)
}

// findGPUAffinityPool finds a pool with GPU affinity for GPU requests
// Strategy:
// 1. Determine NUMA nodes for requested GPUs
// 2. Set high priority for those NUMA nodes and their children
// 3. Use existing sorting and selection logic to pick optimal pool
func (p *TopologyAwarePolicy) findGPUAffinityPool(request Request, _ []Node) Node {
	// Get NUMA nodes for requested GPU devices
	gpuNUMANodes := p.getRequestedGPUNUMANodes(request)
	if len(gpuNUMANodes) == 0 {
		return nil
	}

	// Calculate affinity weights for GPU NUMA nodes
	affinity := p.calculateGPUNUMAAffinity(gpuNUMANodes)

	// Filter pools first, then sort
	filteredPools := p.filterPoolsByCapacity(request)
	if len(filteredPools) == 0 {
		return nil
	}

	// Use existing sort logic on filtered pools
	score, sortedPools := p.sortPoolsByScore(request, affinity, filteredPools)
	if len(sortedPools) == 0 {
		return nil
	}

	// Find first pool from sorted pools that is in GPU NUMA nodes
	for _, pool := range sortedPools {
		// Verify selected pool is in GPU NUMA nodes or their children
		poolNUMAIDs := pool.FreeResource().GetNode().GetNUMAIDs()
		if p.isPoolInGPUNUMANodes(poolNUMAIDs, gpuNUMANodes) {
			klog.V(4).InfoS("GPU-First: Found optimal pool in GPU NUMA node",
				"pool", pool.Name(),
				"poolNUMAIDs", poolNUMAIDs,
				"gpuNUMANodes", gpuNUMANodes,
				"score", score[pool.NodeID()].String())
			return pool
		}
	}

	// GPU NUMA nodes and children don't have sufficient resources
	klog.V(4).InfoS("GPU-First: GPU NUMA node and its children do not have sufficient capacity, falling back to CPU-first allocation")
	return nil
}

// getRequestedGPUNUMANodes gets the set of NUMA nodes for requested GPUs.
func (p *TopologyAwarePolicy) getRequestedGPUNUMANodes(request Request) map[system.ID]bool {
	requestedGPUs := request.GetRequestedGPUDevices()
	if len(requestedGPUs) == 0 {
		klog.V(4).InfoS("GPU-First: No specific GPU requested")
		return nil
	}

	gpuNUMANodes := make(map[system.ID]bool)
	for _, gpuID := range p.sys.GPUIDs() {
		gpu := p.sys.GPU(gpuID)
		if gpu == nil {
			continue
		}

		gpuIDStr := fmt.Sprintf("%d", gpuID)
		for _, requestedID := range requestedGPUs {
			if gpuIDStr == requestedID {
				numaNodeID := gpu.NodeID()
				gpuNUMANodes[numaNodeID] = true
				klog.V(4).InfoS("GPU-First: Found requested GPU on NUMA node",
					"gpuID", gpuID,
					"numaNode", numaNodeID)
			}
		}
	}

	if len(gpuNUMANodes) == 0 {
		klog.V(4).InfoS("GPU-First: No matching GPU found")
	}

	return gpuNUMANodes
}

// calculateGPUNUMAAffinity calculates affinity weights for GPU NUMA nodes.
func (p *TopologyAwarePolicy) calculateGPUNUMAAffinity(gpuNUMANodes map[system.ID]bool) map[int]int32 {
	affinity := make(map[int]int32)

	// Set high affinity weight for GPU NUMA nodes (weight 1000 to prioritize in sorting)
	for numaID := range gpuNUMANodes {
		affinity[int(numaID)] = 1000
		klog.V(4).InfoS("GPU-First: Set high affinity for GPU NUMA node",
			"numaNode", numaID,
			"affinity", 1000)
	}

	return affinity
}

// isPoolInGPUNUMANodes checks if pool is in GPU NUMA nodes or their children.
func (p *TopologyAwarePolicy) isPoolInGPUNUMANodes(poolNUMAIDs []system.ID, gpuNUMANodes map[system.ID]bool) bool {
	// Check if any of pool's NUMA IDs are in GPU NUMA nodes
	for _, numaID := range poolNUMAIDs {
		if gpuNUMANodes[numaID] {
			return true
		}
	}
	return false
}

// findRegularPool finds the best pool using regular allocation logic.
func (p *TopologyAwarePolicy) findRegularPool(request Request, pools []Node) Node {
	if len(pools) == 0 {
		klog.ErrorS(nil, "No pools available for allocation")
		return nil
	}

	// Among pools with sufficient capacity, select the one with greatest depth (prefer finer granularity)
	var bestPool Node
	maxDepth := -1

	for _, pool := range pools {
		if p.hasCapacity(request, pool) {
			depth := pool.RootDistance()
			if depth > maxDepth {
				bestPool = pool
				maxDepth = depth
			}
		}
	}

	if bestPool == nil {
		klog.ErrorS(nil, "No pool with sufficient capacity found",
			"request", request.String(),
			"poolsEvaluated", len(pools))
		return nil
	}

	klog.V(4).InfoS("Selected pool",
		"pool", bestPool.Name(),
		"capacity", bestPool.FreeResource().GetScore(request, p).SharedCapacity(),
		"depth", bestPool.RootDistance())
	return bestPool
}

// hasCapacity checks if a pool has sufficient capacity for the request.
func (p *TopologyAwarePolicy) hasCapacity(request Request, pool Node) bool {
	if !p.checkCapacityByRequest(request, pool) {
		return false
	}
	// Only check memory capacity when memory topology awareness is enabled.
	// When disabled, NUMA nodes may report memoryTotal=0, which would
	// incorrectly reject all pools.
	if p.enableMemoryTopology && !p.checkMemoryCapacity(request, pool) {
		return false
	}
	return true
}

// checkCapacityByRequest checks if pool has sufficient capacity for request.
// Uses request value for capacity checking, consistent with actual allocation logic.
func (p *TopologyAwarePolicy) checkCapacityByRequest(request Request, pool Node) bool {
	origPool := pool
	// Traverse up to root, checking capacity at each level
	for pool != nil && pool != p.root {
		freeResource := pool.FreeResource()
		if freeResource == nil {
			klog.ErrorS(nil, "Pool has nil FreeResource", "pool", pool.Name())
			return false
		}

		score := freeResource.GetScore(request, p)
		if score == nil {
			klog.ErrorS(nil, "FreeResource returned nil score", "pool", pool.Name(), "request", request.String())
			return false
		}

		// Use SharedCapacityByRequest for capacity checking (based on request value)
		if score.SharedCapacityByRequest() < 0 {
			klog.V(3).InfoS("CPU capacity check failed",
				"candidatePool", origPool.Name(),
				"failedAtLevel", pool.Name(),
				"totalSharedCPU", freeResource.AllocatableSharedCPU()+freeResource.GrantedShared(),
				"grantedCPU", freeResource.GrantedShared(),
				"grantedCPUByRequest", freeResource.GrantedCPUByRequest(),
				"allocatableCPU", freeResource.AllocatableSharedCPU(),
				"sharedCapacityByRequest", score.SharedCapacityByRequest(),
				"cpuRequest", request.CPURequest(),
				"cpuLimit", request.CPULimit())
			return false
		}

		pool = pool.Parent()
	}
	return true
}

// checkMemoryCapacity checks if the pool has enough memory capacity for the request.
func (p *TopologyAwarePolicy) checkMemoryCapacity(request Request, pool Node) bool {
	memoryLimitKB := uint64(request.MemoryLimit() / 1024)
	if memoryLimitKB == 0 {
		return true // No memory requirement
	}
	origPool := pool
	// Traverse up to root, checking memory capacity at each level
	for pool != nil && pool != p.root {
		freeResource := pool.FreeResource()
		if freeResource == nil {
			return false
		}
		score := freeResource.GetScore(request, p)
		if score == nil {
			return false
		}
		if score.MemoryCapacity() < memoryLimitKB {
			klog.V(3).InfoS("Memory capacity check failed",
				"candidatePool", origPool.Name(),
				"failedAtLevel", pool.Name(),
				"availableMemoryKB", score.MemoryCapacity(),
				"requiredMemoryKB", memoryLimitKB)
			return false
		}
		pool = pool.Parent()
	}
	return true
}

// calculateSpecificGPUAffinity calculates affinity for specific GPU devices.
func (p *TopologyAwarePolicy) calculateSpecificGPUAffinity(request Request) map[int]int32 {
	affinity := make(map[int]int32)
	numaWeights := make(map[int]int)
	// Iterate through all GPU devices
	for _, gpuID := range p.sys.GPUIDs() {
		gpu := p.sys.GPU(gpuID)
		if gpu == nil {
			continue
		}

		gpuIDStr := fmt.Sprintf("%d", gpuID)
		for _, requestedID := range request.GetRequestedGPUDevices() {
			if gpuIDStr == requestedID {
				numaNodeID := int(gpu.NodeID())
				numaWeights[numaNodeID]++
				klog.V(4).InfoS("Matched requested GPU to NUMA node",
					"gpuID", gpuID,
					"numaNode", numaNodeID)
			}
		}
	}
	// Set affinity based on weights
	for numaID, weight := range numaWeights {
		affinity[numaID] += int32(weight * 100)
		klog.V(3).InfoS("Added GPU affinity for NUMA node",
			"numaNode", numaID,
			"weight", weight*100)
	}

	return affinity
}

// calculatePoolAffinities calculates pool affinities for a request.
func (p *TopologyAwarePolicy) calculatePoolAffinities(request Request) map[int]int32 {
	// Create mapping to store affinity weights for each NUMA node
	affinity := make(map[int]int32)
	containerCtx := request.GetContext()

	// If container requests GPU resources, add affinity for NUMA nodes with GPUs
	if !request.HasGPURequest() {
		return affinity
	}

	klog.V(3).InfoS("Container requests GPU resources",
		"pod", containerCtx.Request.PodMeta.Name,
		"container", containerCtx.Request.ContainerMeta.Name,
		"requestedDevices", request.GetRequestedGPUDevices())

	// GPU-first strategy: calculate NUMA node affinity only for requested GPUs
	// If no specific GPU request, return empty affinity map
	if len(request.GetRequestedGPUDevices()) > 0 {
		return p.calculateSpecificGPUAffinity(request)
	}

	return affinity
}
