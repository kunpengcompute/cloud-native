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
	"k8s.io/utils/cpuset"
	"kunpeng.huawei.com/kunpeng-cloud-computing/pkg/kunpeng-tap/sysfs/system"
)

// NodeKind represents the type of a topology node.
type NodeKind string

const (
	// NilNode is the type of a nil node.
	NilNode NodeKind = ""
	// UnknownNode is the type of unknown node type.
	UnknownNode NodeKind = "unknown"
	// NumaNode represents a NUMA node in the system.
	NumaNode NodeKind = "numa node"
	// ClusterNode represents a CPU cluster within a NUMA node (for 950 model machines).
	ClusterNode NodeKind = "cluster"
	// SocketNode represents a physical CPU package/socket in the system.
	SocketNode NodeKind = "socket"
	// VirtualNode represents a virtual node, currently the root multi-socket setups.
	VirtualNode NodeKind = "virtual node"
)

// NodeIdentifier provides basic identity information for a topology node.
// Use this interface when you only need to identify or log node information.
type NodeIdentifier interface {
	// Name returns the human-readable name of this node.
	Name() string

	// Kind returns the type of this node.
	Kind() NodeKind

	// NodeID returns the unique identifier for this node.
	NodeID() int

	// SetNodeID sets the node id.
	SetNodeID(id int)

	// IsNil tests if this node is nil.
	IsNil() bool
}

// NodeHierarchy provides tree structure traversal and hierarchy management.
// Use this interface when you need to navigate or build the topology tree.
type NodeHierarchy interface {
	NodeIdentifier

	// Depth returns the depth of this node in the tree.
	Depth() int

	// IsLeafNode returns true if this node has no children.
	IsLeafNode() bool

	// Parent returns the parent node of this node.
	Parent() Node

	// Children returns the child nodes of this node.
	Children() []Node

	// RootDistance returns the distance of this node from the root node.
	RootDistance() int

	// LinkParent sets the given node as the parent node, and appends this node as its child.
	LinkParent(parent Node, self Node)

	// AddChildren appends the nodes to the children, *WITHOUT* updating their parents.
	AddChildren(nodes []Node)
}

// ResourceSupplier provides resource allocation and scoring capabilities.
// Use this interface for the core resource allocation logic.
type ResourceSupplier interface {
	NodeIdentifier

	// GetScore returns the score for this node given a request.
	// The ctx parameter provides external information (colocation count, GPU count)
	// that would otherwise require a circular dependency on the policy.
	GetScore(req Request, ctx ScoreContext) Score

	// GrantedCPU returns the granted shared CPU capacity for this node.
	GrantedCPU() int

	// FreeResource returns the available resource supply for this node.
	FreeResource() Supply
}

// HardwareInspector provides hardware topology information.
// Use this interface when querying memory or NUMA configuration.
type HardwareInspector interface {
	NodeIdentifier

	// TotalCPU returns the CPU set for this node.
	TotalCPU() cpuset.CPUSet

	// MemoryInfo returns memory information for this node.
	MemoryInfo() (*system.MemInfo, error)

	// GetNUMAIDs returns the list of NUMA node IDs associated with this node.
	// For NUMA nodes, returns a single-element slice containing its own ID.
	// For other nodes, returns IDs of all NUMA nodes under this node.
	GetNUMAIDs() []system.ID
}

// Node defines the complete interface for all topology nodes in the resource tree.
// It composes all specialized interfaces and adds debugging methods.
type Node interface {
	NodeHierarchy
	ResourceSupplier
	HardwareInspector

	// Dump outputs the state of this node for debugging.
	Dump(prefix string, level ...int)
}

// Ensure all node implementations satisfy the Node interface.
var _ Node = &socketNode{}
var _ Node = &numaNode{}
var _ Node = &clusterNode{}
var _ Node = &virtualNode{}

// Ensure baseNode satisfies the sub-interfaces (for compile-time verification).
var _ NodeIdentifier = &baseNode{}
var _ NodeHierarchy = &baseNode{}
var _ ResourceSupplier = &baseNode{}
var _ HardwareInspector = &baseNode{}
