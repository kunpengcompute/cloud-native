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
	"k8s.io/klog/v2"
)

// propagateResourceToParent is a unified method for propagating resource updates to parent nodes.
// isAllocation: true for allocation (add resources), false for release (subtract resources)
func (p *TopologyAwarePolicy) propagateResourceToParent(grant Grant, isAllocation bool) {
	currentNode := grant.GetNode()
	if currentNode == nil || currentNode.IsNil() {
		if isAllocation {
			klog.V(4).InfoS("Current node is nil, skipping resource propagation")
		} else {
			klog.V(4).InfoS("Current node is nil, skipping resource release propagation")
		}
		return
	}

	action := "usage"
	if !isAllocation {
		action = "release"
	}

	klog.V(4).InfoS("Propagating resource "+action+" to parent nodes",
		"currentNode", currentNode.Name(),
		"allocatedCPUs", grant.AllocatedCPUs(),
		"allocatedCPUByRequest", grant.AllocatedCPUByRequest(),
		"allocatedCPUByLimit", grant.AllocatedCPUByLimit(),
		"allocatedMemory", grant.AllocatedMemory(),
		"isAllocation", isAllocation)

	// Start from current node's parent and propagate upward
	parent := currentNode.Parent()
	for parent != nil && !parent.IsNil() {
		// Update parent node's resource usage
		p.updateParentResource(parent, grant, isAllocation)

		parentSupply := parent.FreeResource()
		if parentSupply != nil {
			klog.V(4).InfoS("Updated parent resource "+action,
				"parent", parent.Name(),
				"grantedShared", parentSupply.GrantedShared(),
				"grantedSharedRequest", parentSupply.GrantedCPUByRequest(),
				"grantedSharedLimit", parentSupply.GrantedCPUByLimit(),
				"grantedMemory", parentSupply.GrantedMemory())
		}

		// Continue upward
		parent = parent.Parent()
	}
}

// updateParentResource updates parent node's resource usage based on grant info.
// isAllocation: true means add resources (allocation), false means subtract (release)
func (p *TopologyAwarePolicy) updateParentResource(parent Node, grant Grant, isAllocation bool) {
	if parent == nil || parent.IsNil() {
		return
	}

	// Get parent node's resource supply
	parentSupply := parent.FreeResource()
	if parentSupply == nil {
		klog.V(4).InfoS("Parent supply is nil, skipping update", "parent", parent.Name())
		return
	}

	// Update parent node's resource usage based on grant info
	if supply, ok := parentSupply.(*supply); ok {
		if isAllocation {
			// Add grant's allocated CPU resources
			supply.grantedShared += grant.AllocatedCPUs()
			supply.grantedCPUByRequest += grant.AllocatedCPUByRequest()
			supply.grantedCPUByLimit += grant.AllocatedCPUByLimit()
			supply.grantedMemory += grant.AllocatedMemory()
		} else {
			// Subtract grant's allocated CPU resources
			supply.grantedShared -= grant.AllocatedCPUs()
			supply.grantedCPUByRequest -= grant.AllocatedCPUByRequest()
			supply.grantedCPUByLimit -= grant.AllocatedCPUByLimit()
			supply.grantedMemory -= grant.AllocatedMemory()

			// Ensure values don't go below 0
			if supply.grantedShared < 0 {
				supply.grantedShared = 0
			}
			if supply.grantedCPUByRequest < 0 {
				supply.grantedCPUByRequest = 0
			}
			if supply.grantedCPUByLimit < 0 {
				supply.grantedCPUByLimit = 0
			}
			if supply.grantedMemory < 0 {
				supply.grantedMemory = 0
			}
		}

		klog.V(5).InfoS("Updated parent resource",
			"parent", parent.Name(),
			"isAllocation", isAllocation,
			"grantedShared", supply.grantedShared,
			"grantedCPUByRequest", supply.grantedCPUByRequest,
			"grantedCPUByLimit", supply.grantedCPUByLimit,
			"grantedMemory", supply.grantedMemory,
			"grantAllocatedCPUs", grant.AllocatedCPUs(),
			"grantAllocatedCPUByRequest", grant.AllocatedCPUByRequest(),
			"grantAllocatedCPUByLimit", grant.AllocatedCPUByLimit(),
			"grantAllocatedMemory", grant.AllocatedMemory())
	}
}

// propagateResourceUsageToParent propagates resource allocation to parent nodes.
// This is a wrapper for backward compatibility.
func (p *TopologyAwarePolicy) propagateResourceUsageToParent(grant Grant) {
	p.propagateResourceToParent(grant, true)
}

// propagateResourceReleaseToParent propagates resource release to parent nodes.
// This is a wrapper for backward compatibility.
func (p *TopologyAwarePolicy) propagateResourceReleaseToParent(grant Grant) {
	p.propagateResourceToParent(grant, false)
}
