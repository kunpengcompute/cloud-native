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
	"k8s.io/utils/cpuset"
	"kunpeng.huawei.com/kunpeng-cloud-computing/pkg/kunpeng-tap/cache"
	"kunpeng.huawei.com/kunpeng-cloud-computing/pkg/kunpeng-tap/policy"
)

// findNodeByCPUSet finds the best matching node for a given cpuset string.
// Strategy: find the deepest node whose CPU set contains the container's cpuset.
func (p *TopologyAwarePolicy) findNodeByCPUSet(cpuSetStr string) Node {
	containerCPUs, err := cpuset.Parse(cpuSetStr)
	if err != nil {
		klog.ErrorS(err, "Failed to parse cpuset", "cpuset", cpuSetStr)
		return nil
	}

	// Find the deepest (most precise) node whose CPU set contains the container's cpuset
	var bestMatch Node
	for _, node := range p.pools {
		nodeCPUs := node.FreeResource().SharableCPUs()
		// Check if container's cpuset is a subset of node's cpuset
		if containerCPUs.IsSubsetOf(nodeCPUs) {
			// Select the node with greatest depth (more precise match)
			if bestMatch == nil || node.RootDistance() > bestMatch.RootDistance() {
				bestMatch = node
			}
		}
	}

	if bestMatch != nil {
		klog.V(5).InfoS("Found matching node for cpuset",
			"cpuset", cpuSetStr,
			"node", bestMatch.Name(),
			"depth", bestMatch.RootDistance())
	}

	return bestMatch
}

// createGrantFromContainer creates a grant from container info.
func (p *TopologyAwarePolicy) createGrantFromContainer(container cache.Container, targetNode Node) (Grant, string, error) {
	pod, ok := container.GetPod()
	if !ok {
		return nil, "", fmt.Errorf("pod not found for container %s", container.GetID())
	}

	containerCtx := policy.ContainerContext{
		Request: policy.ContainerRequest{
			ContainerMeta: policy.ContainerMeta{
				ID:   container.GetID(),
				Name: container.GetName(),
			},
			PodMeta: policy.PodMeta{
				UID: pod.GetUID(),
			},
		},
	}
	grant := newGrant(targetNode, containerCtx, false, 0)

	// Set allocated CPU and memory from container resource requirements
	resources := container.GetResourceRequirements()

	// For containerd/docker mode, GetResourceRequirements() may return empty values
	// because container.Resources is copied from pod.Resources which is not properly set.
	// In this case, we need to estimate resources from LinuxContainerResources.
	cpuRequestMilli := int64(0)
	cpuLimitMilli := int64(0)
	memoryBytes := int64(0)

	// First try to get from ResourceRequirements
	if resources.Requests != nil {
		if cpuRequest := resources.Requests.Cpu(); cpuRequest != nil && cpuRequest.MilliValue() > 0 {
			cpuRequestMilli = cpuRequest.MilliValue()
		}
		if memRequest := resources.Requests.Memory(); memRequest != nil && memRequest.Value() > 0 {
			memoryBytes = memRequest.Value()
		}
	}
	if resources.Limits != nil {
		if cpuLimit := resources.Limits.Cpu(); cpuLimit != nil && cpuLimit.MilliValue() > 0 {
			cpuLimitMilli = cpuLimit.MilliValue()
		}
	}

	// If ResourceRequirements is empty, estimate from LinuxContainerResources
	if cpuRequestMilli == 0 || cpuLimitMilli == 0 {
		linuxRes := container.GetLinuxResources()
		if linuxRes != nil {
			// Calculate CPU request from CpuShares (shares to milliCPU conversion)
			// CpuShares of 1024 = 1000 milliCPU (1 CPU)
			if cpuRequestMilli == 0 && linuxRes.CpuShares > 0 {
				cpuRequestMilli = int64(float64(linuxRes.CpuShares*1000)/float64(1024) + 0.5)
			}
			// Calculate CPU limit from CpuQuota and CpuPeriod
			if cpuLimitMilli == 0 && linuxRes.CpuQuota > 0 && linuxRes.CpuPeriod > 0 {
				cpuLimitMilli = int64(float64(linuxRes.CpuQuota*1000)/float64(linuxRes.CpuPeriod) + 0.5)
			}
			// Get memory limit
			if memoryBytes == 0 && linuxRes.MemoryLimitInBytes > 0 {
				memoryBytes = linuxRes.MemoryLimitInBytes
			}
		}
	}

	// Set CPU request
	if cpuRequestMilli > 0 {
		grant.SetAllocatedCPU(int(cpuRequestMilli))
		grant.SetAllocatedCPUByRequest(int(cpuRequestMilli))
	}

	// Set CPU limit
	if cpuLimitMilli > 0 {
		grant.SetAllocatedCPUByLimit(int(cpuLimitMilli))
	}

	// Set memory (convert bytes to KB)
	if memoryBytes > 0 {
		grant.SetAllocatedMemory(uint64(memoryBytes / 1024))
	}

	klog.V(5).InfoS("Created grant from container",
		"containerID", container.GetID(),
		"cpuRequestMilli", cpuRequestMilli,
		"cpuLimitMilli", cpuLimitMilli,
		"memoryBytes", memoryBytes)

	gid := pod.GetUID() + ":" + container.GetName()
	return grant, gid, nil
}

// updateNodeResourceUsage updates the resource usage for a node and propagates to parent nodes.
func (p *TopologyAwarePolicy) updateNodeResourceUsage(node Node, grant Grant) {
	if node == nil || node.IsNil() {
		return
	}

	// Update the target node's resource usage
	if s, ok := node.FreeResource().(*supply); ok {
		s.grantedShared += grant.AllocatedCPUs()
		s.grantedCPUByRequest += grant.AllocatedCPUByRequest()
		s.grantedCPUByLimit += grant.AllocatedCPUByLimit()
		s.grantedMemory += grant.AllocatedMemory()

		klog.V(5).InfoS("Updated node resource usage",
			"node", node.Name(),
			"grantedShared", s.grantedShared,
			"grantedCPUByRequest", s.grantedCPUByRequest,
			"grantedCPUByLimit", s.grantedCPUByLimit,
			"grantedMemory", s.grantedMemory)
	}

	// Propagate to parent nodes
	p.propagateResourceUsageToParent(grant)
}

// rebuildContainerAllocation rebuilds allocation for a single container.
func (p *TopologyAwarePolicy) rebuildContainerAllocation(container cache.Container) error {
	cpus := container.GetCpusetCpus()
	if cpus == "" {
		return nil // Skip containers without CPU allocation
	}

	// Check if grant already exists
	pod, ok := container.GetPod()
	if !ok {
		return fmt.Errorf("pod not found for container %s", container.GetID())
	}
	gid := pod.GetUID() + ":" + container.GetName()
	if _, exists := p.allocations.grants.Load(gid); exists {
		klog.V(5).InfoS("Grant already exists, skipping rebuild",
			"containerID", container.GetID(),
			"gid", gid)
		return nil
	}

	// Find matching node
	targetNode := p.findNodeByCPUSet(cpus)
	if targetNode == nil {
		klog.Warningf("Node not found when rebuilding allocation for container %s with cpuset %s",
			container.GetID(), cpus)
		return nil
	}

	// Create grant
	grant, gid, err := p.createGrantFromContainer(container, targetNode)
	if err != nil {
		return fmt.Errorf("failed to create grant: %w", err)
	}

	// Update node resource usage
	p.updateNodeResourceUsage(targetNode, grant)

	// Store grant
	p.allocations.grants.Store(gid, grant)

	klog.InfoS("Rebuilt allocation for container",
		"containerID", container.GetID(),
		"containerName", container.GetName(),
		"cpuset", cpus,
		"node", targetNode.Name(),
		"allocatedCPUs", grant.AllocatedCPUs(),
		"allocatedMemory", grant.AllocatedMemory())

	return nil
}

// RebuildAllocationsFromCache implements StateSynchronizer interface.
// This method is called after NRI Synchronize to rebuild allocation state from cached containers.
func (p *TopologyAwarePolicy) RebuildAllocationsFromCache() error {
	if p.cache == nil {
		klog.InfoS("Cache is nil, skipping allocation rebuild")
		return nil
	}

	containers := p.cache.GetContainers()
	klog.InfoS("Rebuilding allocations from cache", "containerCount", len(containers))

	// Log available pools for debugging
	for _, node := range p.pools {
		supply := node.FreeResource()
		if supply != nil {
			klog.V(4).InfoS("Pool available for rebuild",
				"node", node.Name(),
				"sharableCPUs", supply.SharableCPUs().String(),
				"depth", node.RootDistance())
		}
	}

	rebuiltCount := 0
	skippedNoCpuset := 0
	for _, container := range containers {
		cpuset := container.GetCpusetCpus()
		if cpuset == "" {
			skippedNoCpuset++
			continue
		}
		if err := p.rebuildContainerAllocation(container); err != nil {
			klog.ErrorS(err, "Failed to rebuild allocation for container",
				"containerID", container.GetID(),
				"containerName", container.GetName(),
				"cpuset", cpuset)
		} else {
			rebuiltCount++
		}
	}

	klog.InfoS("Completed rebuilding allocations from cache",
		"totalContainers", len(containers),
		"rebuiltAllocations", rebuiltCount,
		"skippedNoCpuset", skippedNoCpuset)

	// Log node resource usage after rebuild for verification
	for _, node := range p.pools {
		supply := node.FreeResource()
		if supply != nil && supply.GrantedShared() > 0 {
			klog.InfoS("Node resource usage after rebuild",
				"node", node.Name(),
				"grantedCPU", supply.GrantedShared(),
				"grantedCPUByRequest", supply.GrantedCPUByRequest(),
				"grantedCPUByLimit", supply.GrantedCPUByLimit(),
				"grantedMemory", supply.GrantedMemory(),
				"allocatableSharedCPU", supply.AllocatableSharedCPU())
		}
	}

	// Update metrics after rebuild
	p.updateAllMetrics()

	return nil
}
