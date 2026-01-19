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
	"kunpeng.huawei.com/kunpeng-cloud-computing/pkg/kunpeng-tap/sysfs/system"
)

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
	// DieNode represents a die within a physical CPU package/socket in the system.
	DieNode NodeKind = "die"
	// SocketNode represents a physical CPU package/socket in the system.
	SocketNode NodeKind = "socket"
	// VirtualNode represents a virtual node, currently the root multi-socket setups.
	VirtualNode NodeKind = "virtual node"
)

type Node interface {
	Policy() *TopologyAwarePolicy

	Name() string

	Kind() NodeKind

	NodeID() int

	// SetNodeID sets the node id.
	SetNodeID(id int)

	// IsNil tests if this node is nil.
	IsNil() bool

	Depth() int

	IsLeafNode() bool
	// Parent returns the parent node of this node.
	Parent() Node
	// Children returns the child nodes of this node.
	Children() []Node
	// LinkParent sets the given node as the parent node, and appends this node as a its child.
	LinkParent(Node, Node)
	// AddChildren appends the nodes to the children, *WITHOUT* updating their parents.
	AddChildren([]Node)
	// Get the distance of this node from the root node.
	RootDistance() int

	DepthFirst(fn func(Node) error) error

	BreadthFirst(fn func(Node) error) error

	GetScore(Request) Score

	GrantedCPU() int

	DiscoverResource() Supply

	FreeResource() Supply

	// Dump state of the node.
	Dump(prefix string, level ...int)

	// MemoryInfo returns memory information for this node.
	MemoryInfo() (*system.MemInfo, error)

	// GetNUMAIDs returns the list of NUMA node IDs associated with this node.
	// For NUMA nodes, returns a single-element slice containing its own ID.
	// For other nodes, returns IDs of all NUMA nodes under this node.
	GetNUMAIDs() []system.ID
}

var _ Node = &socketNode{}
var _ Node = &dieNode{}
var _ Node = &numaNode{}
var _ Node = &clusterNode{}
var _ Node = &virtualNode{}

// baseNode 包含所有节点类型共享的基本实现
type baseNode struct {
	policy       *TopologyAwarePolicy
	name         string
	id           int
	kind         NodeKind
	depth        int
	parent       Node
	children     []Node
	nodeResource Supply
	freeResource Supply
}

// 初始化基本节点数据
func (n *baseNode) init(p *TopologyAwarePolicy, name string, kind NodeKind, parent Node, self Node) {
	n.policy = p
	n.name = name
	n.kind = kind
	n.parent = parent
	n.id = -1

	if !parent.IsNil() {
		n.LinkParent(parent, self)
	}
}

// Policy 返回策略指针
func (n *baseNode) Policy() *TopologyAwarePolicy {
	return n.policy
}

// IsNil 测试节点是否为空
func (n *baseNode) IsNil() bool {
	return n.kind == NilNode
}

func (n *baseNode) Name() string {
	if n.IsNil() {
		return "<nil node>"
	}
	return n.name
}

func (n *baseNode) Kind() NodeKind {
	return n.kind
}

func (n *baseNode) NodeID() int {
	if n.IsNil() {
		return -1
	}
	return n.id
}

// SetNodeID 设置节点ID
func (n *baseNode) SetNodeID(id int) {
	if n.IsNil() {
		return
	}
	n.id = id
}

func (n *baseNode) Depth() int {
	return n.depth
}

// IsLeafNode 检查此节点是否为叶节点
func (n *baseNode) IsLeafNode() bool {
	return len(n.children) == 0
}

// Parent 返回此节点的父节点
func (n *baseNode) Parent() Node {
	if n.IsNil() {
		return nil
	}
	return n.parent
}

// Children 返回此节点的子节点
func (n *baseNode) Children() []Node {
	if n.IsNil() {
		return nil
	}
	return n.children
}

// LinkParent 设置给定节点为父节点并将此节点添加到父节点的子节点中
func (n *baseNode) LinkParent(parent Node, self Node) {
	n.parent = parent
	if !parent.IsNil() {
		parent.AddChildren([]Node{self})
	}
	n.depth = parent.RootDistance() + 1
}

// AddChildren 将节点添加到子节点列表，*不*设置它们的父节点
func (n *baseNode) AddChildren(nodes []Node) {
	for _, newNode := range nodes {
		if !containsNode(n.children, newNode) {
			n.children = append(n.children, newNode)
		}
	}
}

// containsNode 检查节点是否存在于节点切片中
func containsNode(nodes []Node, target Node) bool {
	for _, node := range nodes {
		if node == target {
			return true
		}
	}
	return false
}

// RootDistance 返回此节点到根节点的距离
func (n *baseNode) RootDistance() int {
	if n.IsNil() {
		return -1
	}
	return n.depth
}

// DepthFirst 从节点开始进行深度优先遍历，在每个节点调用给定函数
func (n *baseNode) DepthFirst(fn func(Node) error) error {
	for _, c := range n.children {
		if err := c.DepthFirst(fn); err != nil {
			return err
		}
	}
	return fn(n)
}

// BreadthFirst 从节点开始进行广度优先遍历，在每个节点调用给定函数
func (n *baseNode) BreadthFirst(fn func(Node) error) error {
	if err := fn(n); err != nil {
		return err
	}
	for _, c := range n.children {
		if err := c.BreadthFirst(fn); err != nil {
			return err
		}
	}
	return nil
}

// GetScore 获取此节点对请求的评分
func (n *baseNode) GetScore(request Request) Score {
	if n.IsNil() {
		return nil
	}
	return n.FreeResource().GetScore(request)
}

// GetScore 获取此节点对请求的评分
func (n *socketNode) GetScore(request Request) Score {
	if n.IsNil() {
		return nil
	}
	return n.FreeResource().GetScore(request)
}

// GetScore 获取此节点对请求的评分
func (n *numaNode) GetScore(request Request) Score {
	if n.IsNil() {
		return nil
	}
	return n.FreeResource().GetScore(request)
}

// GetScore 获取此节点对请求的评分
func (n *dieNode) GetScore(request Request) Score {
	if n.IsNil() {
		return nil
	}
	return n.FreeResource().GetScore(request)
}

// GetScore 获取此节点对请求的评分
func (n *virtualNode) GetScore(request Request) Score {
	if n.IsNil() {
		return nil
	}
	return n.FreeResource().GetScore(request)
}

// GrantedCPU 返回此节点授予的共享CPU容量
func (n *baseNode) GrantedCPU() int {
	// 如果是叶子节点，直接返回当前节点的已授予共享CPU
	if n.IsLeafNode() {
		return n.freeResource.GrantedShared()
	}

	// 对于非叶子节点，我们应该只计算子节点的总和
	// 避免重复计算，因为子节点的资源已经包含在其自身的计算中
	granted := 0
	for _, c := range n.children {
		granted += c.GrantedCPU()
	}
	return granted
}

func (n *baseNode) FreeResource() Supply {
	return n.freeResource
}

// DiscoverResource 发现此节点可用的资源
func (n *baseNode) DiscoverResource() Supply {
	klog.V(5).InfoS("Discovering Resource available at node", "node", n.Name())
	n.nodeResource = newSupply(n, cpuset.New(), cpuset.New())
	for _, c := range n.children {
		n.nodeResource.Collect(c.DiscoverResource())
	}

	n.freeResource = n.nodeResource.Clone()
	return n.nodeResource.Clone()
}

// Dump 输出节点状态
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

// socketNode 表示物理CPU包/插槽
type socketNode struct {
	baseNode
	socketID system.ID
	sysPkg   system.CPUPackage
}

func NewSocketNode(p *TopologyAwarePolicy, id system.ID, parent Node) Node {
	n := &socketNode{
		socketID: id,
		sysPkg:   p.sys.Package(id),
	}
	n.init(p, fmt.Sprintf("socket #%v", id), SocketNode, parent, n)
	return n
}

func (n *socketNode) DiscoverResource() Supply {
	klog.V(5).InfoS("Discovering Resource available at socket Node", "node", n.Name())

	if n.IsLeafNode() {
		sockCPU := n.sysPkg.CPUSet()
		isolated := sockCPU.Intersection(n.policy.isolated)
		sharable := sockCPU.Difference(isolated)

		n.nodeResource = newSupply(n, isolated, sharable)
	} else {
		n.nodeResource = newSupply(n, cpuset.New(), cpuset.New())
		for _, c := range n.children {
			n.nodeResource.Collect(c.DiscoverResource())
		}
	}

	n.freeResource = n.nodeResource.Clone()
	return n.nodeResource.Clone()
}

// dieNode 表示物理CPU包/插槽内的一个die
type dieNode struct {
	baseNode
	dieID  system.ID
	sysPkg system.CPUPackage
}

func NewDieNode(p *TopologyAwarePolicy, id system.ID, parent Node) Node {
	socketParent, ok := parent.(*socketNode)
	if !ok {
		klog.ErrorS(nil, "Die parent must be a socket node")
		return nil
	}

	n := &dieNode{
		dieID:  id,
		sysPkg: p.sys.Package(socketParent.socketID),
	}
	n.init(p, fmt.Sprintf("die #%v/%v", socketParent.socketID, id), DieNode, parent, n)
	return n
}

func (n *dieNode) DiscoverResource() Supply {
	klog.V(5).InfoS("Discovering Resource available at die Node", "node", n.Name())
	n.nodeResource = newSupply(n, n.sysPkg.CPUSet(), cpuset.New())
	for _, c := range n.children {
		n.nodeResource.Collect(c.DiscoverResource())
	}

	n.freeResource = n.nodeResource.Clone()
	return n.nodeResource.Clone()
}

// numaNode 表示NUMA节点
type numaNode struct {
	baseNode
	numaID  system.ID
	sysNode system.Node
}

func NewNumaNode(p *TopologyAwarePolicy, id system.ID, parent Node) Node {
	n := &numaNode{
		numaID:  id,
		sysNode: p.sys.Node(id),
	}
	n.init(p, fmt.Sprintf("NUMA node #%v", id), NumaNode, parent, n)
	return n
}

func (n *numaNode) DiscoverResource() Supply {
	klog.V(5).InfoS("Discovering Resource available at numa Node", "node", n.Name())
	nodeResource := n.sysNode.CPUSet()
	isolated := nodeResource.Intersection(n.policy.isolated)
	sharable := nodeResource.Difference(isolated)

	n.nodeResource = newSupply(n, isolated, sharable)
	n.freeResource = n.nodeResource.Clone()
	return n.nodeResource.Clone()
}

// clusterNode represents a CPU cluster within a NUMA node.
// This is used for 950 model machines where each NUMA node contains multiple clusters.
type clusterNode struct {
	baseNode
	clusterID  system.ID      // Cluster ID
	numaNodeID system.ID      // Parent NUMA node ID
	cpus       cpuset.CPUSet  // CPUs in this cluster
	sysCluster system.Cluster // System cluster info
}

// NewClusterNode creates a new cluster node.
func NewClusterNode(p *TopologyAwarePolicy, clusterID system.ID, numaNodeID system.ID, cpus cpuset.CPUSet, sysCluster system.Cluster, parent Node) Node {
	n := &clusterNode{
		clusterID:  clusterID,
		numaNodeID: numaNodeID,
		cpus:       cpus,
		sysCluster: sysCluster,
	}
	n.init(p, fmt.Sprintf("cluster #%v (NUMA %v)", clusterID, numaNodeID), ClusterNode, parent, n)
	return n
}

func (n *clusterNode) DiscoverResource() Supply {
	klog.V(5).InfoS("Discovering Resource available at cluster Node", "node", n.Name(), "cpus", n.cpus)
	isolated := n.cpus.Intersection(n.policy.isolated)
	sharable := n.cpus.Difference(isolated)

	n.nodeResource = newSupply(n, isolated, sharable)
	n.freeResource = n.nodeResource.Clone()
	return n.nodeResource.Clone()
}

// GetClusterID returns the cluster ID.
func (n *clusterNode) GetClusterID() system.ID {
	return n.clusterID
}

// GetNUMANodeID returns the parent NUMA node ID.
func (n *clusterNode) GetNUMANodeID() system.ID {
	return n.numaNodeID
}

// GetCPUs returns the CPUs in this cluster.
func (n *clusterNode) GetCPUs() cpuset.CPUSet {
	return n.cpus
}

// MemoryInfo returns memory information for this cluster node.
// Clusters don't have their own memory, so we return the parent NUMA node's memory info.
func (n *clusterNode) MemoryInfo() (*system.MemInfo, error) {
	if n.parent != nil && !n.parent.IsNil() {
		return n.parent.MemoryInfo()
	}
	return nil, fmt.Errorf("cluster node has no parent")
}

// GetNUMAIDs returns the NUMA node ID for this cluster.
func (n *clusterNode) GetNUMAIDs() []system.ID {
	return []system.ID{n.numaNodeID}
}

// virtualNode 表示虚拟节点
type virtualNode struct {
	baseNode
}

func NewVirtualNode(p *TopologyAwarePolicy, name string, parent Node) Node {
	n := &virtualNode{}
	n.init(p, name, VirtualNode, parent, n)
	return n
}

// DepthFirst 从节点开始进行深度优先遍历，在每个节点调用给定函数
func (n *virtualNode) DepthFirst(fn func(Node) error) error {
	for _, c := range n.children {
		if err := c.DepthFirst(fn); err != nil {
			return err
		}
	}
	return fn(n)
}

// BreadthFirst 从节点开始进行广度优先遍历，在每个节点调用给定函数
func (n *virtualNode) BreadthFirst(fn func(Node) error) error {
	if err := fn(n); err != nil {
		return err
	}
	for _, c := range n.children {
		if err := c.BreadthFirst(fn); err != nil {
			return err
		}
	}
	return nil
}

func (n *virtualNode) DiscoverResource() Supply {
	klog.V(5).InfoS("Discovering Resource available at virtual Node", "node", n.Name())
	n.nodeResource = newSupply(n, cpuset.New(), cpuset.New())
	for _, c := range n.children {
		n.nodeResource.Collect(c.DiscoverResource())
	}

	n.freeResource = n.nodeResource.Clone()
	return n.nodeResource.Clone()
}

var nilNode Node

func init() {
	// nilNode 是一个空节点的实例
	nilNode = initNilNode()
}

// 初始化空节点
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

// indent produces an indentation string for the given level.
const (
	IndentDepth = 4
)

func indent(prefix string, level ...int) string {
	if len(level) < 1 {
		return prefix
	}

	depth := level[0] * IndentDepth
	return prefix + fmt.Sprintf("%*.*s", depth, depth, "")
}

// MemoryInfo returns memory information for this node.
func (n *baseNode) MemoryInfo() (*system.MemInfo, error) {
	// 基础节点不直接实现，由子类实现
	return nil, fmt.Errorf("not implemented for base node")
}

// MemoryInfo returns memory information for this NUMA node.
func (n *numaNode) MemoryInfo() (*system.MemInfo, error) {
	// 对于 NUMA 节点，直接返回系统 NUMA 节点的内存信息
	return n.sysNode.MemoryInfo()
}

// MemoryInfo returns memory information for this socket.
func (n *socketNode) MemoryInfo() (*system.MemInfo, error) {
	// 对于 Socket 节点，返回系统 Package 的内存信息
	result := &system.MemInfo{
		MemTotal: 0,
		MemFree:  0,
		MemUsed:  0,
		MemSet:   cpuset.New(),
	}

	// 获取 Die 中的所有 NUMA 节点
	for _, nodeID := range n.sysPkg.NodeIDs() {
		sysNode := n.policy.sys.Node(nodeID)
		if sysNode == nil {
			continue
		}

		// 获取节点的内存信息
		nodeMemInfo, err := sysNode.MemoryInfo()
		if err != nil {
			klog.ErrorS(err, "Failed to get memory info for node", "nodeID", nodeID)
			continue
		}

		// 累加内存信息
		result.MemTotal += nodeMemInfo.MemTotal
		result.MemFree += nodeMemInfo.MemFree
		result.MemUsed += nodeMemInfo.MemUsed
		result.MemSet = result.MemSet.Union(nodeMemInfo.MemSet)
	}

	return result, nil
}

// MemoryInfo returns memory information for this die.
func (n *dieNode) MemoryInfo() (*system.MemInfo, error) {
	// 对于 Die 节点，我们需要聚合其所有 NUMA 节点的内存信息
	result := &system.MemInfo{
		MemTotal: 0,
		MemFree:  0,
		MemUsed:  0,
		MemSet:   cpuset.New(),
	}

	// 获取 Die 中的所有 NUMA 节点
	for _, nodeID := range n.sysPkg.DieNodeIDs(n.id) {
		sysNode := n.policy.sys.Node(nodeID)
		if sysNode == nil {
			continue
		}

		// 获取节点的内存信息
		nodeMemInfo, err := sysNode.MemoryInfo()
		if err != nil {
			klog.ErrorS(err, "Failed to get memory info for node", "nodeID", nodeID)
			continue
		}

		// 累加内存信息
		result.MemTotal += nodeMemInfo.MemTotal
		result.MemFree += nodeMemInfo.MemFree
		result.MemUsed += nodeMemInfo.MemUsed
		result.MemSet = result.MemSet.Union(nodeMemInfo.MemSet)
	}

	return result, nil
}

// MemoryInfo returns memory information for this virtual node.
func (n *virtualNode) MemoryInfo() (*system.MemInfo, error) {
	// 对于虚拟节点，我们需要聚合其所有子节点的内存信息
	result := &system.MemInfo{
		MemTotal: 0,
		MemFree:  0,
		MemUsed:  0,
		MemSet:   cpuset.New(),
	}

	// 遍历所有子节点
	for _, child := range n.children {
		// 获取子节点的内存信息
		childMemInfo, err := child.MemoryInfo()
		if err != nil {
			klog.ErrorS(err, "Failed to get memory info for child node", "node", child.Name())
			continue
		}

		// 累加内存信息
		result.MemTotal += childMemInfo.MemTotal
		result.MemFree += childMemInfo.MemFree
		result.MemUsed += childMemInfo.MemUsed
		result.MemSet = result.MemSet.Union(childMemInfo.MemSet)
	}

	return result, nil
}

// GetNUMAIDs returns the list of NUMA node IDs associated with this node.
// For NUMA nodes, returns a single-element slice containing its own ID.
// For other nodes, returns IDs of all NUMA nodes under this node.
func (n *baseNode) GetNUMAIDs() []system.ID {
	if n.IsNil() {
		return nil
	}

	// 对于基础节点，递归收集所有子节点的 NUMA IDs
	numaIDs := make([]system.ID, 0)
	n.DepthFirst(func(child Node) error {
		if numaNode, ok := child.(*numaNode); ok {
			numaIDs = append(numaIDs, numaNode.numaID)
		}
		return nil
	})
	return numaIDs
}

// numaNode 实现
func (n *numaNode) GetNUMAIDs() []system.ID {
	// NUMA 节点只返回自己的 ID
	return []system.ID{n.numaID}
}

// socketNode 实现
func (n *socketNode) GetNUMAIDs() []system.ID {
	// Socket 节点返回其下所有 NUMA 节点的 ID
	numaIDs := make([]system.ID, 0)
	numaIDs = append(numaIDs, n.sysPkg.NodeIDs()...)
	return numaIDs
}

// dieNode 实现
func (n *dieNode) GetNUMAIDs() []system.ID {
	// Die 节点返回其下所有 NUMA 节点的 ID
	return n.sysPkg.DieNodeIDs(n.dieID)
}

// virtualNode 实现
func (n *virtualNode) GetNUMAIDs() []system.ID {
	// 虚拟节点返回其下所有 NUMA 节点的 ID
	return n.baseNode.GetNUMAIDs()
}
