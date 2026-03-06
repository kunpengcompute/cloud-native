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

// baseNode contains the shared implementation for all node types.
// It only contains tree structure and resource fields.
// Node-specific data (numaNodeID, memInfo, etc.) is stored in each node type.
type baseNode struct {
	name         string
	id           int
	kind         NodeKind
	depth        int
	parent       Node
	children     []Node
	nodeResource Supply
	freeResource Supply
}

// init initializes the basic node data.
// After calling init, the concrete constructor must call LinkParent if parent is not nil.
func (n *baseNode) init(name string, kind NodeKind, parent Node) {
	n.name = name
	n.kind = kind
	n.parent = parent
	n.id = -1
}

// TotalCPU returns the CPU set for this node.
// Base implementation derives from nodeResource. Node types can override if needed.
func (n *baseNode) TotalCPU() cpuset.CPUSet {
	if n.nodeResource != nil {
		return n.nodeResource.TotalCPUs()
	}
	return cpuset.New()
}

// IsNil tests if the node is nil.
func (n *baseNode) IsNil() bool {
	return n.kind == NilNode
}

// Name returns the node name.
func (n *baseNode) Name() string {
	if n.IsNil() {
		return "<nil node>"
	}
	return n.name
}

// Kind returns the node kind.
func (n *baseNode) Kind() NodeKind {
	return n.kind
}

// NodeID returns the node ID.
func (n *baseNode) NodeID() int {
	if n.IsNil() {
		return -1
	}
	return n.id
}

// SetNodeID sets the node ID.
func (n *baseNode) SetNodeID(id int) {
	if n.IsNil() {
		return
	}
	n.id = id
}

// Depth returns the node depth.
func (n *baseNode) Depth() int {
	return n.depth
}

// IsLeafNode checks if this node is a leaf node.
func (n *baseNode) IsLeafNode() bool {
	return len(n.children) == 0
}

// Parent returns the parent node.
func (n *baseNode) Parent() Node {
	if n.IsNil() {
		return nil
	}
	return n.parent
}

// Children returns the child nodes.
func (n *baseNode) Children() []Node {
	if n.IsNil() {
		return nil
	}
	return n.children
}

// LinkParent sets the given node as the parent and adds this node as a child.
func (n *baseNode) LinkParent(parent Node, self Node) {
	n.parent = parent
	if !parent.IsNil() {
		parent.AddChildren([]Node{self})
	}
	n.depth = parent.RootDistance() + 1
}

// AddChildren appends nodes to the children list, *WITHOUT* updating their parents.
func (n *baseNode) AddChildren(nodes []Node) {
	for _, newNode := range nodes {
		if !containsNode(n.children, newNode) {
			n.children = append(n.children, newNode)
		}
	}
}

// containsNode checks if a node exists in the node slice.
func containsNode(nodes []Node, target Node) bool {
	for _, node := range nodes {
		if node == target {
			return true
		}
	}
	return false
}

// RootDistance returns the distance from this node to the root.
func (n *baseNode) RootDistance() int {
	if n.IsNil() {
		return -1
	}
	return n.depth
}

// GetScore returns the score for this node against a request.
// The ctx parameter provides external information (colocation count, GPU count).
func (n *baseNode) GetScore(request Request, ctx ScoreContext) Score {
	if n.IsNil() {
		return nil
	}
	return n.FreeResource().GetScore(request, ctx)
}

// GrantedCPU returns the granted shared CPU capacity for this node.
func (n *baseNode) GrantedCPU() int {
	if n.IsLeafNode() {
		return n.freeResource.GrantedShared()
	}

	granted := 0
	for _, c := range n.children {
		granted += c.GrantedCPU()
	}
	return granted
}

// FreeResource returns the free resource supply.
func (n *baseNode) FreeResource() Supply {
	return n.freeResource
}

// Dump outputs the node state.
func (n *baseNode) Dump(prefix string, level ...int) {
	if !klog.V(5).Enabled() {
		return
	}

	lvl := 0
	if len(level) > 0 {
		lvl = level[0]
	}
	idt := indent(prefix, lvl)

	klog.V(5).InfoS("Discovering Resource available at base Node", "level", idt, "node", n.Name())

	if n.nodeResource != nil {
		klog.V(5).InfoS("Node Resource", "level", idt, "node", n.Name(), "resource", n.nodeResource)
	}

	if n.freeResource != nil {
		klog.V(5).InfoS("Node Free Resource", "level", idt, "node", n.Name(), "resource", n.freeResource)
	}

	if !n.Parent().IsNil() {
		klog.V(5).InfoS("Parent", "level", idt, "node", n.Name(), "parent", n.Parent().Name())
	}

	if len(n.children) > 0 {
		klog.V(5).InfoS("Children", "level", idt, "node", n.Name(), "children", n.children)
		for _, c := range n.children {
			c.Dump(prefix, lvl+1)
		}
	}
}

// MemoryInfo returns memory information for this node (base implementation).
func (n *baseNode) MemoryInfo() (*system.MemInfo, error) {
	return nil, fmt.Errorf("not implemented for base node")
}

// GetNUMAIDs returns the list of NUMA node IDs associated with this node.
// Base implementation returns nil. Node types (numaNode, socketNode, etc.) override this.
func (n *baseNode) GetNUMAIDs() []system.ID {
	return nil
}

// nilNode is a singleton nil node instance.
var nilNode Node

func init() {
	nilNode = initNilNode()
}

// initNilNode creates the nil node singleton.
func initNilNode() Node {
	n := &baseNode{
		name:     "<nil node>",
		id:       -1,
		kind:     NilNode,
		depth:    -1,
		children: nil,
	}
	n.parent = n
	return n
}

// IndentDepth is the number of spaces per indent level.
const IndentDepth = 4

// indent produces an indentation string for the given level.
func indent(prefix string, level ...int) string {
	if len(level) < 1 {
		return prefix
	}

	depth := level[0] * IndentDepth
	return prefix + fmt.Sprintf("%*.*s", depth, depth, "")
}
