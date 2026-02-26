// Copyright 2019 Intel Corporation. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
// Copyright (c) 2025 Huawei Technology corp.

package topologyaware

import (
	"fmt"

	"k8s.io/klog/v2"
	"k8s.io/utils/cpuset"
	"kunpeng.huawei.com/kunpeng-cloud-computing/pkg/kunpeng-tap/sysfs/system"
)

// socketNode represents a physical CPU package/socket.
type socketNode struct {
	baseNode
	memInfo *system.MemInfo // Memory information from CPUPackage (computed at construction)
}

// NewSocketNode creates a new socket node.
// Resources are created at construction time using the system.CPUPackage interface.
func NewSocketNode(p *TopologyAwarePolicy, id system.ID, parent Node, pkg system.CPUPackage) Node {
	// Get memory info from system.CPUPackage at construction time
	memInfo, err := pkg.MemoryInfo()
	if err != nil {
		klog.ErrorS(err, "Failed to get memory info for socket", "socketID", id)
		memInfo = &system.MemInfo{}
	}
	n := &socketNode{
		memInfo: memInfo,
	}
	n.init(fmt.Sprintf("socket #%v", id), SocketNode, parent)

	if !parent.IsNil() {
		n.LinkParent(parent, n)
	}

	// Create resources at construction time from system.CPUPackage
	totalCPU := pkg.CPUSet()
	isolated := totalCPU.Intersection(p.isolated)
	sharable := totalCPU.Difference(isolated)
	n.nodeResource = newSupply(n, isolated, sharable)
	n.freeResource = n.nodeResource.Clone()

	return n
}

// GetNUMAIDs returns the NUMA node IDs by aggregating from children.
func (n *socketNode) GetNUMAIDs() []system.ID {
	numaIDs := make([]system.ID, 0)
	for _, child := range n.children {
		numaIDs = append(numaIDs, child.GetNUMAIDs()...)
	}
	return numaIDs
}

// GetScore returns the score for this socket node.
func (n *socketNode) GetScore(request Request, ctx ScoreContext) Score {
	if n.IsNil() {
		return nil
	}
	return n.FreeResource().GetScore(request, ctx)
}

// MemoryInfo returns memory information for this socket from CPUPackage.
func (n *socketNode) MemoryInfo() (*system.MemInfo, error) {
	if n.memInfo != nil {
		return n.memInfo, nil
	}
	return nil, fmt.Errorf("no memory info for socket %s", n.name)
}

// numaNode represents a NUMA node.
// It stores its own NUMA ID and memory information.
type numaNode struct {
	baseNode
	numaNodeID system.ID       // This NUMA node's ID
	memInfo    *system.MemInfo // Memory information (computed at construction)
}

// NewNumaNode creates a new NUMA node.
// Resources are created at construction time.
func NewNumaNode(p *TopologyAwarePolicy, id system.ID, parent Node) Node {
	sysNode := p.sys.Node(id)
	// Get memory info at construction time
	memInfo, err := sysNode.MemoryInfo()
	if err != nil {
		klog.ErrorS(err, "Failed to get memory info for NUMA node", "nodeID", id)
		memInfo = &system.MemInfo{}
	}
	n := &numaNode{
		numaNodeID: id,
		memInfo:    memInfo,
	}
	n.init(fmt.Sprintf("NUMA node #%v", id), NumaNode, parent)

	if !parent.IsNil() {
		n.LinkParent(parent, n)
	}

	// Create resources at construction time
	totalCPU := sysNode.CPUSet()
	isolated := totalCPU.Intersection(p.isolated)
	sharable := totalCPU.Difference(isolated)

	n.nodeResource = newSupply(n, isolated, sharable)
	n.freeResource = n.nodeResource.Clone()

	return n
}

// GetNUMAIDs returns this NUMA node's ID.
func (n *numaNode) GetNUMAIDs() []system.ID {
	return []system.ID{n.numaNodeID}
}

// GetScore returns the score for this NUMA node.
func (n *numaNode) GetScore(request Request, ctx ScoreContext) Score {
	if n.IsNil() {
		return nil
	}
	return n.FreeResource().GetScore(request, ctx)
}

// MemoryInfo returns memory information for this NUMA node.
func (n *numaNode) MemoryInfo() (*system.MemInfo, error) {
	if n.memInfo != nil {
		return n.memInfo, nil
	}
	return nil, fmt.Errorf("no memory info for NUMA node %s", n.name)
}

// clusterNode represents a CPU cluster within a NUMA node.
// This is used for 950 model machines where each NUMA node contains multiple clusters.
// It derives NUMA IDs and memory info from its parent NUMA node.
type clusterNode struct {
	baseNode
}

// NewClusterNode creates a new cluster node.
// Resources are created at construction time.
func NewClusterNode(p *TopologyAwarePolicy, sysCluster system.Cluster, parent Node) Node {
	n := &clusterNode{}
	n.init(fmt.Sprintf("cluster #%v (NUMA %v)", sysCluster.ID(), sysCluster.NodeID()), ClusterNode, parent)

	if !parent.IsNil() {
		n.LinkParent(parent, n)
	}

	// Create resources at construction time
	totalCPU := sysCluster.CPUSet()
	isolated := totalCPU.Intersection(p.isolated)
	sharable := totalCPU.Difference(isolated)
	n.nodeResource = newSupply(n, isolated, sharable)
	n.freeResource = n.nodeResource.Clone()

	return n
}

// GetNUMAIDs returns the NUMA node IDs by getting from parent.
// Cluster nodes are always children of NUMA nodes.
func (n *clusterNode) GetNUMAIDs() []system.ID {
	if n.parent != nil && !n.parent.IsNil() {
		return n.parent.GetNUMAIDs()
	}
	return nil
}

// MemoryInfo returns memory information for this cluster node.
// Delegates to parent NUMA node.
func (n *clusterNode) MemoryInfo() (*system.MemInfo, error) {
	if n.parent != nil && !n.parent.IsNil() {
		return n.parent.MemoryInfo()
	}
	return nil, fmt.Errorf("cluster node has no parent")
}

// virtualNode represents a virtual node.
// It is the root node in multi-socket setups.
type virtualNode struct {
	baseNode
	memInfo *system.MemInfo // Memory information from System (computed at construction)
}

// NewVirtualNode creates a new virtual node.
// Resources are created at construction time using system.System.AllowedSet().
func NewVirtualNode(p *TopologyAwarePolicy, name string, parent Node) Node {
	// Get memory info from system.System at construction time
	memInfo, err := p.sys.MemoryInfo()
	if err != nil {
		klog.ErrorS(err, "Failed to get memory info for virtual node")
		memInfo = &system.MemInfo{}
	}
	n := &virtualNode{
		memInfo: memInfo,
	}
	n.init(name, VirtualNode, parent)

	if !parent.IsNil() {
		n.LinkParent(parent, n)
	}

	// Create resources at construction time
	allowed := p.sys.AllowedSet()
	isolated := allowed.Intersection(p.isolated)
	sharable := allowed.Difference(isolated)
	n.nodeResource = newSupply(n, isolated, sharable)
	n.freeResource = n.nodeResource.Clone()

	return n
}

// TotalCPU returns the aggregate CPU set from all children.
func (n *virtualNode) TotalCPU() cpuset.CPUSet {
	result := cpuset.New()
	for _, child := range n.children {
		result = result.Union(child.TotalCPU())
	}
	return result
}

// GetScore returns the score for this virtual node.
func (n *virtualNode) GetScore(request Request, ctx ScoreContext) Score {
	if n.IsNil() {
		return nil
	}
	return n.FreeResource().GetScore(request, ctx)
}

// MemoryInfo returns memory information for this virtual node from System.
func (n *virtualNode) MemoryInfo() (*system.MemInfo, error) {
	if n.memInfo != nil {
		return n.memInfo, nil
	}
	return nil, fmt.Errorf("no memory info for virtual node %s", n.name)
}

// GetNUMAIDs returns the NUMA node IDs by aggregating from children.
func (n *virtualNode) GetNUMAIDs() []system.ID {
	numaIDs := make([]system.ID, 0)
	for _, child := range n.children {
		numaIDs = append(numaIDs, child.GetNUMAIDs()...)
	}
	return numaIDs
}
