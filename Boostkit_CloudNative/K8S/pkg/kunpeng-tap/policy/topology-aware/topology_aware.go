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
	"sort"
	"sync"

	"k8s.io/api/core/v1"
	"k8s.io/klog/v2"
	"k8s.io/utils/cpuset"
	"kunpeng.huawei.com/kunpeng-cloud-computing/pkg/kunpeng-tap/cache"
	"kunpeng.huawei.com/kunpeng-cloud-computing/pkg/kunpeng-tap/policy"
	"kunpeng.huawei.com/kunpeng-cloud-computing/pkg/kunpeng-tap/sysfs/system"
)

const (
	// PolicyName is the name used to activate this policy implementation.
	PolicyName = "topology-aware"
	// PolicyDescription is a short description of this policy.
	PolicyDescription = "A policy for cpu/memory/devices optimazation."
	// containerRequestLogKey is the log key for container request logging
	containerRequestLogKey = "container request"
)

// allocations is our cache for saving resource allocations in the cache.
type allocations struct {
	policy *TopologyAwarePolicy
	grants sync.Map // 使用 sync.Map 替代 map[string]Grant
}

// newAllocations returns a new initialized empty set of allocations.
func (p *TopologyAwarePolicy) newAllocations() allocations {
	return allocations{
		policy: p,
		grants: sync.Map{},
	}
}

type TopologyAwarePolicy struct {
	policy.BasePolicy
	cache                cache.Cache
	sys                  system.System
	allowed              cpuset.CPUSet
	isolated             cpuset.CPUSet
	nodes                map[string]Node
	pools                []Node
	root                 Node
	nodeCnt              int
	depth                int
	allocations          allocations             // container pool assignmentss
	enableMemoryTopology bool                    // 是否启用内存拓扑感知
	resourcePriority     policy.ResourcePriority // 资源分配优先级策略
	metricsManager       *TopologyMetricsManager // 资源树监控管理器
}

// Name returns the name of this policy.
func (p *TopologyAwarePolicy) Name() string {
	return PolicyName
}

func (p *TopologyAwarePolicy) Description() string {
	return PolicyDescription
}

func (p *TopologyAwarePolicy) Root() Node {
	return p.root
}

func (p *TopologyAwarePolicy) MemoryTopology() bool {
	return p.enableMemoryTopology
}

// NewTopologyAwarePolicy creates a new topology-aware policy
func NewTopologyAwarePolicy(c cache.Cache, opts *policy.PolicyOptions) policy.Policy {
	return NewTopologyAwarePolicyWithSystem(c, opts, nil)
}

// NewTopologyAwarePolicyWithSystem creates a new topology-aware policy with a custom system
// This function is primarily for testing purposes to allow injection of mock system
func NewTopologyAwarePolicyWithSystem(c cache.Cache, opts *policy.PolicyOptions, sys system.System) policy.Policy {
	var err error
	if sys == nil {
		sys, err = system.NewSystem("")
		if err != nil {
			klog.ErrorS(err, "Failed to discover system resources")
			return nil
		}
	}
	p := &TopologyAwarePolicy{
		BasePolicy:           *policy.NewBasePolicy(PolicyName, PolicyDescription),
		sys:                  sys,
		cache:                c,
		enableMemoryTopology: opts.EnableMemoryTopology,
		resourcePriority:     opts.ResourcePriority,
	}
	p.allocations = p.newAllocations()

	if err := p.initialize(); err != nil {
		klog.ErrorS(err, "Failed to initialize topology-aware policy")
		return nil
	}

	// 从 cache 中恢复资源分配状态
	if err := p.restoreAllocations(); err != nil {
		klog.ErrorS(err, "Failed to restore allocations from cache")
	}

	// 初始化资源树监控管理器
	p.metricsManager = NewTopologyMetricsManager(p)

	// 启动监控指标更新
	p.startMetricsMonitoring()

	return p
}

// GetAllocations 返回 allocations 的引用，用于监控
func (p *TopologyAwarePolicy) GetAllocations() *allocations {
	return &p.allocations
}

// GetPools 返回 pools 的副本，用于监控
func (p *TopologyAwarePolicy) GetPools() []Node {
	return append([]Node{}, p.pools...)
}

// GetSystem 返回系统信息，用于监控
func (p *TopologyAwarePolicy) GetSystem() system.System {
	return p.sys
}

// GetAllocationsGrants 返回 grants 的引用，用于监控
func (p *TopologyAwarePolicy) GetAllocationsGrants() *sync.Map {
	return &p.allocations.grants
}

// 实现 PreCreateContainerHook 方法
func (p *TopologyAwarePolicy) PreCreateContainerHook(ctx policy.HookContext) (*policy.Allocation, error) {
	containerCtx, ok := ctx.(*policy.ContainerContext)
	if !ok {
		klog.ErrorS(nil, "Invalid context type for PreCreateContainerHook")
		return nil, fmt.Errorf("invalid context type: expected *policy.ContainerContext")
	}

	// 解析 QoS 类型，过滤 BestEffort 类型的 Pod/Container
	qos := policy.ParseCgroupForQOSClass(containerCtx.Request.CgroupParent)
	if qos == v1.PodQOSBestEffort {
		klog.InfoS("Skip BestEffort QoS container",
			"pod", containerCtx.Request.PodMeta.Name,
			"namespace", containerCtx.Request.PodMeta.Namespace,
			"container", containerCtx.Request.ContainerMeta.Name,
			"qos", qos)
		// 不对 BestEffort 类型的容器进行资源分配
		return nil, nil
	}

	// 使用 defer 确保在函数结束时更新指标
	defer func() {
		p.updateAllMetrics()
	}()

	if err := p.AllocateResources(*containerCtx); err != nil {
		klog.ErrorS(err, "Failed to allocate resources",
			"pod", containerCtx.Request.PodMeta.Name,
			"namespace", containerCtx.Request.PodMeta.Namespace,
			"container", containerCtx.Request.ContainerMeta.Name)
		return nil, err
	}

	// 依据 Grant 的内容进行结果填充
	alloc := policy.NewAllocation()
	gid := containerCtx.Request.PodMeta.UID + ":" + containerCtx.Request.ContainerMeta.Name

	// 使用 sync.Map 的 Load 方法
	grantVal, ok := p.allocations.grants.Load(gid)
	if !ok {
		return nil, fmt.Errorf("no grant found for container %s", gid)
	}
	grant := grantVal.(Grant)

	// 设置CPU集
	cpus := grant.SharedCPUSet().String()
	alloc.SetCPUSetCpus(cpus)

	klog.V(3).InfoS("Resource allocation completed",
		"pod", containerCtx.Request.PodMeta.Name,
		"container", containerCtx.Request.ContainerMeta.Name,
		"NUMA node", grant.GetNode().Name())

	klog.V(3).InfoS("Container setting", "cpuset.cpus", cpus)

	// 设置memorySet
	if p.enableMemoryTopology {
		memset := grant.Memset().String()
		alloc.SetCPUSetMems(memset)
		klog.V(3).InfoS("Container setting", "cpuset.memory", memset)
	}
	// 查询GPU设备分布
	hasGPU, deviceIDs := checkDeviceRequest(containerCtx.Request.ContainerEnvs, knownDeviceConfigs[0])
	if hasGPU {
		klog.V(3).InfoS("Container resource", "GPU devices", deviceIDs)
	}
	return alloc, nil
}

func (p *TopologyAwarePolicy) AllocateResources(containerCtx policy.ContainerContext) error {
	klog.V(5).InfoS("AllocateResources", containerRequestLogKey, containerCtx.Request)
	// 分配资源池
	grant, err := p.allocatePool(containerCtx)
	if err != nil {
		return err
	}

	klog.InfoS("Allocated resources for container", containerRequestLogKey, containerCtx.Request, "grant", grant)
	return nil
}

func (p *TopologyAwarePolicy) allocatePool(containerCtx policy.ContainerContext) (Grant, error) {
	request := newRequest(containerCtx)
	var pool Node

	if containerCtx.Request.PodMeta.Namespace == "kube-system" {
		// 系统容器直接分配到根节点
		klog.V(5).InfoS("Allocating system container to root pool", containerRequestLogKey, containerCtx.Request)
		pool = p.root
	} else {
		// 计算其他资源的亲和性
		affinity := p.calculatePoolAffinities(request)

		// 根据请求的资源，选择最优的资源池
		score, pools := p.sortPoolsByScore(request, affinity)
		if len(pools) == 0 {
			return nil, fmt.Errorf("no suitable pool found for container %s",
				containerCtx.Request.PodMeta.Name)
		}

		for _, n := range pools {
			klog.V(5).InfoS("node fitting for container", "node", n.Name(), "score", score[n.NodeID()].String())
		}

		// 选择出最优的资源池
		pool = p.findBestAvailablePool(request, pools)
		if pool == nil {
			return nil, fmt.Errorf("no suitable pool found for container %s",
				containerCtx.Request.PodMeta.Name)
		}
	}

	supply := pool.FreeResource()
	grant, err := supply.Allocate(request)
	if err != nil {
		return nil, fmt.Errorf("failed to allocate %s from %s: %v",
			request, supply, err)
	}

	gid := containerCtx.Request.PodMeta.UID + ":" + containerCtx.Request.ContainerMeta.Name

	// 使用 sync.Map 的 Store 方法
	p.allocations.grants.Store(gid, grant)
	// 记录资源使用情况，向上层传递
	p.propagateResourceUsageToParent(grant)
	p.saveAllocations()

	return grant, nil
}

// propagateResourceUsageToParent 根据 grant 中的分配信息来增量更新上层节点的资源使用情况
func (p *TopologyAwarePolicy) propagateResourceUsageToParent(grant Grant) {
	currentNode := grant.GetNode()
	if currentNode == nil || currentNode.IsNil() {
		klog.V(4).InfoS("Current node is nil, skipping resource propagation")
		return
	}

	klog.V(4).InfoS("Propagating resource usage to parent nodes",
		"currentNode", currentNode.Name(),
		"allocatedCPUs", grant.AllocatedCPUs(),
		"allocatedCPUByRequest", grant.AllocatedCPUByRequest(),
		"allocatedCPUByLimit", grant.AllocatedCPUByLimit(),
		"allocatedMemory", grant.AllocatedMemory())

	// 从当前节点开始，逐层向上传递资源使用情况
	parent := currentNode.Parent()
	for parent != nil && !parent.IsNil() {
		// 根据 grant 中的分配信息来增量更新父节点的资源使用情况
		p.updateParentResourceUsageByGrant(parent, grant)

		parentSupply := parent.FreeResource()
		if parentSupply != nil {
			klog.V(4).InfoS("Updated parent resource usage",
				"parent", parent.Name(),
				"grantedShared", parentSupply.GrantedShared(),
				"grantedSharedRequest", parentSupply.GrantedCPUByRequest(),
				"grantedSharedLimit", parentSupply.GrantedCPUByLimit(),
				"grantedMemory", parentSupply.GrantedMemory())
		}

		// 继续向上传递
		parent = parent.Parent()
	}
}

// updateParentResourceUsageByGrant 根据 grant 中的分配信息来增量更新父节点的资源使用情况
func (p *TopologyAwarePolicy) updateParentResourceUsageByGrant(parent Node, grant Grant) {
	if parent == nil || parent.IsNil() {
		return
	}

	// 获取父节点的资源供应
	parentSupply := parent.FreeResource()
	if parentSupply == nil {
		klog.V(4).InfoS("Parent supply is nil, skipping update", "parent", parent.Name())
		return
	}

	// 根据 grant 中的分配信息来增量更新父节点的资源使用情况
	if supply, ok := parentSupply.(*supply); ok {
		// 增加 grant 中分配的 CPU 资源
		supply.grantedShared += grant.AllocatedCPUs()
		supply.grantedCPUByRequest += grant.AllocatedCPUByRequest()
		supply.grantedCPUByLimit += grant.AllocatedCPUByLimit()
		supply.grantedMemory += grant.AllocatedMemory()

		klog.V(5).InfoS("Updated parent resource usage by grant",
			"parent", parent.Name(),
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

// findBestAvailablePool finds the best available pool for the request.
func (p *TopologyAwarePolicy) findBestAvailablePool(request Request, pools []Node) Node {
	// GPU-First策略下的特殊处理
	if p.resourcePriority == policy.ResourcePriorityGPUFirst && request.HasGPURequest() {
		if pool := p.findGPUAffinityPool(request, pools); pool != nil {
			return pool
		}
		klog.V(4).InfoS("GPU-First: No GPU NUMA pools with sufficient capacity found, falling back to regular allocation")
	}

	// 常规处理逻辑
	return p.findRegularPool(request, pools)
}

// findGPUAffinityPool finds a pool with GPU affinity for GPU requests
func (p *TopologyAwarePolicy) findGPUAffinityPool(request Request, pools []Node) Node {
	// 阶段1：尝试特定GPU的NUMA节点
	specificAffinity := p.calculatePoolAffinities(request)
	if pool := p.findPoolWithAffinity(request, pools, specificAffinity, "specific GPU"); pool != nil {
		return pool
	}

	// 阶段2：尝试其他GPU的NUMA节点
	generalAffinity := p.calculateGeneralGPUAffinity()
	return p.findAlternativeGPUPool(request, pools, generalAffinity, specificAffinity)
}

// findPoolWithAffinity finds a NUMA-level pool with given affinity
func (p *TopologyAwarePolicy) findPoolWithAffinity(request Request, pools []Node, affinity map[int]int32, affinityType string) Node {
	for _, pool := range pools {
		if !p.isNUMALevelPool(pool) {
			continue
		}

		poolAffinity := p.calculatePoolAffinityValue(pool, affinity)
		if poolAffinity > 0 && p.hasCapacity(request, pool) {
			klog.V(5).InfoS("GPU-First: Found suitable NUMA pool", "bestPool", pool.Name(), "affinity", poolAffinity, "type", affinityType)
			return pool
		}
	}
	return nil
}

// findAlternativeGPUPool finds alternative GPU pools excluding specific GPU pools
func (p *TopologyAwarePolicy) findAlternativeGPUPool(request Request, pools []Node, generalAffinity, specificAffinity map[int]int32) Node {
	for _, pool := range pools {
		if !p.isNUMALevelPool(pool) {
			continue
		}

		poolAffinity := p.calculatePoolAffinityValue(pool, generalAffinity)
		hasSpecificGPU := p.calculatePoolAffinityValue(pool, specificAffinity) > 0

		if poolAffinity > 0 && !hasSpecificGPU && p.hasCapacity(request, pool) {
			klog.V(5).InfoS("GPU-First: Found suitable alternative GPU NUMA pool", "bestPool", pool.Name(), "affinity", poolAffinity)
			return pool
		}
	}
	return nil
}

// findRegularPool finds the best pool using regular allocation logic
func (p *TopologyAwarePolicy) findRegularPool(request Request, pools []Node) Node {
	bestPool := pools[0]
	for _, pool := range pools {
		if p.hasCapacity(request, pool) && pool.RootDistance() > bestPool.RootDistance() {
			bestPool = pool
		}
	}
	klog.V(5).InfoS("Best pool", "bestPool", bestPool.Name())
	return bestPool
}

// Helper functions
func (p *TopologyAwarePolicy) isNUMALevelPool(pool Node) bool {
	return len(pool.FreeResource().GetNode().GetNUMAIDs()) == 1
}

func (p *TopologyAwarePolicy) calculatePoolAffinityValue(pool Node, affinity map[int]int32) int32 {
	var poolAffinity int32
	for _, numaID := range pool.FreeResource().GetNode().GetNUMAIDs() {
		poolAffinity += affinity[int(numaID)]
	}
	return poolAffinity
}

func (p *TopologyAwarePolicy) hasCapacity(request Request, pool Node) bool {
	return checkCapacityByRequest(request, p.root, pool) && checkMemoryCapacity(request, p.root, pool)
}

// checkCapacity checks if the pool has enough capacity for the request.
func checkCapacityByRequest(request Request, root, pool Node) bool {
	// 不断向上遍历，确认是否满足容量条件
	for pool != nil && pool != root {
		freeResource := pool.FreeResource()
		if freeResource == nil {
			klog.ErrorS(nil, "Pool has nil FreeResource", "pool", pool.Name())
			return false
		}

		score := freeResource.GetScore(request)
		if score == nil {
			klog.ErrorS(nil, "FreeResource returned nil score", "pool", pool.Name(), "request", request.String())
			return false
		}

		if score.SharedCapacityByRequest() < 0 {
			return false
		}
		pool = pool.Parent()
	}
	return true
}

// checkMemoryCapacity checks if the pool has enough capacity for the request.
func checkMemoryCapacity(request Request, root, pool Node) bool {
	// 不断向上遍历，确认是否满足容量条件
	for pool != nil && pool != root {
		if pool.FreeResource().GetScore(request).MemoryCapacity() < uint64(request.MemoryLimit()/1024) {
			return false
		}
		pool = pool.Parent()
	}
	return true
}

// Helper method to calculate affinity for specific GPU devices
func (p *TopologyAwarePolicy) calculateSpecificGPUAffinity(request Request) map[int]int32 {
	affinity := make(map[int]int32)
	numaWeights := make(map[int]int)
	// 遍历所有GPU设备
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
	// 根据权重设置亲和性
	for numaID, weight := range numaWeights {
		affinity[numaID] += int32(weight * 100)
		klog.V(3).InfoS("Added GPU affinity for NUMA node",
			"numaNode", numaID,
			"weight", weight*100)
	}

	return affinity
}

// Helper method to calculate general GPU affinity for all nodes with GPUs
func (p *TopologyAwarePolicy) calculateGeneralGPUAffinity() map[int]int32 {
	affinity := make(map[int]int32)

	for _, nodeID := range p.sys.NodeIDs() {
		gpuCount := len(p.sys.NodeGPUs(nodeID))
		if gpuCount > 0 {
			affinity[int(nodeID)] += int32(gpuCount * 100)
			klog.V(3).InfoS("Added general GPU affinity for NUMA node",
				"numaNode", nodeID,
				"gpuCount", gpuCount,
				"weight", gpuCount*100)
		}
	}

	return affinity
}

func (p *TopologyAwarePolicy) calculatePoolAffinities(request Request) map[int]int32 {
	// 创建一个映射来存储每个NUMA节点的权重
	affinity := make(map[int]int32)
	containerCtx := request.GetContext()

	// 如果容器请求了GPU资源，增加对有GPU的NUMA节点的亲和性
	if !request.HasGPURequest() {
		return affinity
	}

	klog.V(3).InfoS("Container requests GPU resources",
		"pod", containerCtx.Request.PodMeta.Name,
		"container", containerCtx.Request.ContainerMeta.Name,
		"requestedDevices", request.GetRequestedGPUDevices())

	// 如果有特定的GPU设备请求，尝试找到这些设备所在的NUMA节点
	if len(request.GetRequestedGPUDevices()) > 0 {
		return p.calculateSpecificGPUAffinity(request)
	} else {
		// 如果没有特定的GPU设备请求，则增加所有有GPU的NUMA节点的亲和性
		return p.calculateGeneralGPUAffinity()
	}
}

func (p *TopologyAwarePolicy) applyGrant(grant Grant) {
	klog.V(3).InfoS("Applying grant", "grant", grant)
	containerCtx := grant.GetContext()

	// 设置CPU集
	cpus := grant.SharedCPUSet().String()
	containerCtx.Response.Resources.CpuSetCpus = &cpus

	// 设置内存节点
	node := grant.GetNode()
	if node != nil && p.enableMemoryTopology {
		// 获取节点的内存信息
		memInfo, err := node.MemoryInfo()
		if err == nil && memInfo != nil && !memInfo.MemSet.IsEmpty() {
			// 将 MemSet 转换为字符串
			mems := memInfo.MemSet.String()
			containerCtx.Response.Resources.CpuSetMems = &mems

			klog.V(3).InfoS("Set container memory nodes",
				"pod", containerCtx.Request.PodMeta.Name,
				"container", containerCtx.Request.ContainerMeta.Name,
				"memoryNodes", mems)
		} else {
			klog.V(3).InfoS("No memory nodes available for container",
				"pod", containerCtx.Request.PodMeta.Name,
				"container", containerCtx.Request.ContainerMeta.Name,
				"error", err)
		}
	} else if !p.enableMemoryTopology {
		klog.V(3).InfoS("Memory topology awareness is disabled",
			"pod", containerCtx.Request.PodMeta.Name,
			"container", containerCtx.Request.ContainerMeta.Name)
	}
}

func (p *TopologyAwarePolicy) sortPoolsByScore(req Request, affinity map[int]int32) (map[int]Score, []Node) {
	scores := make(map[int]Score, p.nodeCnt)

	p.root.DepthFirst(func(n Node) error {
		scores[n.NodeID()] = n.GetScore(req)
		return nil
	})

	sort.Slice(p.pools, func(i, j int) bool {
		return p.compare(req, scores, affinity, i, j)
	})
	// 输出 pools 信息
	for _, n := range p.pools {
		klog.V(5).InfoS("Sorted pool", "node", n.Name())
	}
	return scores, p.pools
}

// Helper method to check resource capacity comparison
func (p *TopologyAwarePolicy) compareResourceCapacity(request Request, node1, node2 Node) (bool, bool) {
	capacity1 := node1.FreeResource().GetScore(request).SharedCapacity()
	capacity2 := node2.FreeResource().GetScore(request).SharedCapacity()

	if capacity1 >= 0 && capacity2 < 0 {
		klog.V(5).InfoS("Lower shared capacity wins", "winner", node1.Name(), "failed", node2.Name())
		return true, true // node1 wins, comparison complete
	}

	if capacity1 < 0 && capacity2 >= 0 {
		klog.V(5).InfoS("Lower shared capacity wins", "winner", node2.Name(), "failed", node1.Name())
		return false, true // node2 wins, comparison complete
	}

	return false, false // tie, continue comparison
}

// Helper method to compare node depths
func (p *TopologyAwarePolicy) compareDepth(node1, node2 Node) (bool, bool) {
	depth1, depth2 := node1.RootDistance(), node2.RootDistance()

	if depth1 > depth2 {
		klog.V(5).InfoS("Lower depth wins", "winner", node1.Name(), "failed", node2.Name())
		return true, true // node1 wins, comparison complete
	}
	if depth2 > depth1 {
		klog.V(5).InfoS("Lower depth wins", "winner", node2.Name(), "failed", node1.Name())
		return false, true // node2 wins, comparison complete
	}

	klog.V(5).InfoS("Depth is a TIE", "node1", node1.Name(), "node2", node2.Name())
	return false, false // tie, continue comparison
}

// Helper method to compare shared capacity
func (p *TopologyAwarePolicy) compareSharedCapacity(score1, score2 Score, node1, node2 Node) (bool, bool) {
	shared1, shared2 := score1.SharedCapacity(), score2.SharedCapacity()

	if shared1 > shared2 {
		return true, true // node1 wins, comparison complete
	}
	if shared2 > shared1 {
		return false, true // node2 wins, comparison complete
	}

	klog.V(5).InfoS("Shared capacity is a TIE", "node1", node1.Name(), "node2", node2.Name())
	return false, false // tie, continue comparison
}

// Helper method to compare colocated containers
func (p *TopologyAwarePolicy) compareColocated(score1, score2 Score, node1, node2 Node) (bool, bool) {
	if score1.Colocated() < score2.Colocated() {
		klog.V(5).InfoS("Fewer colocated containers wins", "winner", node1.Name(), "failed", node2.Name())
		return true, true // node1 wins, comparison complete
	}
	if score2.Colocated() < score1.Colocated() {
		klog.V(5).InfoS("Fewer colocated containers wins", "winner", node2.Name(), "failed", node1.Name())
		return false, true // node2 wins, comparison complete
	}

	return false, false // tie, continue comparison
}

// Helper method to compare GPU affinity
func (p *TopologyAwarePolicy) compareGPUAffinity(request Request, affinity map[int]int32, node1, node2 Node) (bool, bool) {
	if !request.HasGPURequest() {
		return false, false // no GPU request, continue comparison
	}

	numaIDs1 := node1.FreeResource().GetNode().GetNUMAIDs()
	numaIDs2 := node2.FreeResource().GetNode().GetNUMAIDs()
	klog.V(5).InfoS("GPU affinity compare", "node1", node1.Name(), "node2", node2.Name(), "numaIDs1", numaIDs1, "numaIDs2", numaIDs2)

	var affinity1, affinity2 int32
	for _, numaID := range numaIDs1 {
		baseAffinity := affinity[int(numaID)]
		// GPU-First策略下，增强GPU亲和性权重
		if p.resourcePriority == policy.ResourcePriorityGPUFirst && baseAffinity > 0 {
			affinity1 += baseAffinity * 100 // 增强权重100倍，确保GPU亲和性优先
		} else {
			affinity1 += baseAffinity
		}
	}
	for _, numaID := range numaIDs2 {
		baseAffinity := affinity[int(numaID)]
		// GPU-First策略下，增强GPU亲和性权重
		if p.resourcePriority == policy.ResourcePriorityGPUFirst && baseAffinity > 0 {
			affinity2 += baseAffinity * 100 // 增强权重100倍，确保GPU亲和性优先
		} else {
			affinity2 += baseAffinity
		}
	}

	klog.V(5).InfoS("GPU affinity compare", "node1", node1.Name(), "node2", node2.Name(), "affinity1", affinity1, "affinity2", affinity2)

	if affinity1 > affinity2 {
		klog.V(5).InfoS("GPU affinity wins", "winner", node1.Name(), "numaIDs", numaIDs1, "affinity", affinity1)
		return true, true // node1 wins, comparison complete
	}
	if affinity2 > affinity1 {
		klog.V(5).InfoS("GPU affinity wins", "winner", node2.Name(), "numaIDs", numaIDs2, "affinity", affinity2)
		return false, true // node2 wins, comparison complete
	}

	klog.V(5).InfoS("GPU affinity is a TIE", "node1", node1.Name(), "node2", node2.Name())
	return false, false // tie, continue comparison
}

// Compare two pools by scores for allocation preference.
func (p *TopologyAwarePolicy) compare(request Request, scores map[int]Score, affinity map[int]int32, i int, j int) bool {
	//
	// 比较计分的算法如下：
	// 1. 如果节点在树中较低，则获胜
	// 2. 如果是共享分配，则较少共置容器获胜
	// 3. 根据资源优先级策略，调整CPU和GPU亲和性的比较顺序
	// 4. id 更小的获胜

	node1, node2 := p.pools[i], p.pools[j]
	id1, id2 := node1.NodeID(), node2.NodeID()
	score1, ok1 := scores[id1]
	score2, ok2 := scores[id2]
	if !ok1 || !ok2 {
		klog.ErrorS(nil, "Score not found", "node1", node1.Name(), "node2", node2.Name())
		return false
	}

	// 1. Check resource capacity
	if result, done := p.compareResourceCapacity(request, node1, node2); done {
		return result
	}

	// 2. Check depth
	if result, done := p.compareDepth(node1, node2); done {
		return result
	}

	// 3. Check shared capacity
	if result, done := p.compareSharedCapacity(score1, score2, node1, node2); done {
		return result
	}

	// 4. Check colocated containers
	if result, done := p.compareColocated(score1, score2, node1, node2); done {
		return result
	}

	// 5. Apply resource priority strategy
	if result, done := p.applyResourcePriorityStrategy(request, affinity, node1, node2); done {
		return result
	}

	// 6. Final tie-breaker: smaller ID wins
	return id1 < id2
}

// applyResourcePriorityStrategy applies the configured resource priority strategy
func (p *TopologyAwarePolicy) applyResourcePriorityStrategy(request Request, affinity map[int]int32, node1, node2 Node) (bool, bool) {
	switch p.resourcePriority {
	case policy.ResourcePriorityGPUFirst:
		// GPU优先策略：先比较GPU亲和性，再比较CPU容量
		if result, done := p.compareGPUAffinity(request, affinity, node1, node2); done {
			return result, true
		}
		// GPU亲和性相同时，比较CPU容量
		if result, done := p.compareCPUCapacity(node1, node2); done {
			return result, true
		}
	case policy.ResourcePriorityCPUFirst:
		fallthrough
	default:
		// CPU优先（默认）：先比较CPU容量，再比较GPU亲和性
		if result, done := p.compareCPUCapacity(node1, node2); done {
			return result, true
		}
		// CPU容量相同时，比较GPU亲和性
		if result, done := p.compareGPUAffinity(request, affinity, node1, node2); done {
			return result, true
		}
	}

	return false, false // 继续其他比较
}

// compareCPUCapacity compares CPU capacity between two nodes
func (p *TopologyAwarePolicy) compareCPUCapacity(node1, node2 Node) (bool, bool) {
	capacity1 := node1.FreeResource().AllocatableSharedCPU()
	capacity2 := node2.FreeResource().AllocatableSharedCPU()

	if capacity1 > capacity2 {
		klog.V(5).InfoS("Higher CPU capacity wins", "winner", node1.Name(), "capacity", capacity1, "failed", node2.Name(), "capacity", capacity2)
		return true, true // node1 wins, comparison complete
	}
	if capacity2 > capacity1 {
		klog.V(5).InfoS("Higher CPU capacity wins", "winner", node2.Name(), "capacity", capacity2, "failed", node1.Name(), "capacity", capacity1)
		return false, true // node2 wins, comparison complete
	}

	klog.V(5).InfoS("CPU capacity is a TIE", "node1", node1.Name(), "node2", node2.Name(), "capacity", capacity1)
	return false, false // tie, continue comparison
}

func (p *TopologyAwarePolicy) PostStopContainerHook(ctx policy.HookContext) (*policy.Allocation, error) {
	klog.V(5).InfoS("PostStopContainerHook", containerRequestLogKey, ctx)
	containerCtx, ok := ctx.(*policy.ContainerContext)
	if !ok {
		klog.ErrorS(nil, "Invalid context type for PostStopContainerHook")
		return nil, nil
	}

	// 使用 defer 确保在函数结束时更新指标
	defer func() {
		p.updateAllMetrics()
	}()

	// 在 StopContainer 请求中，我们只有 containerID
	if containerCtx.Request.ContainerMeta.ID == "" {
		klog.ErrorS(nil, "Container ID is empty in PostStopContainerHook")
		return nil, nil
	}

	// 从 cache 中查找容器信息
	container, ok := p.cache.LookupContainer(containerCtx.Request.ContainerMeta.ID)
	if !ok {
		klog.Warningf("Container not found in cache: %s", containerCtx.Request.ContainerMeta.ID)
		return nil, nil
	}

	// 获取 pod 信息
	pod, ok := container.GetPod()
	if !ok {
		klog.Warningf("Pod not found for container: %s", containerCtx.Request.ContainerMeta.ID)
		return nil, nil
	}

	// 构建完整的 ContainerContext
	fullContainerCtx := policy.ContainerContext{
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

	if err := p.ReleaseResources(fullContainerCtx); err != nil {
		klog.ErrorS(err, "Failed to release resources",
			"containerID", containerCtx.Request.ContainerMeta.ID)
		return nil, err
	}

	return nil, nil
}

func (p *TopologyAwarePolicy) ReleaseResources(containerCtx policy.ContainerContext) error {
	klog.V(5).InfoS("ReleaseResources", containerRequestLogKey, containerCtx.Request)

	// 确保有必要的容器和 pod 信息
	if containerCtx.Request.ContainerMeta.ID == "" || containerCtx.Request.PodMeta.UID == "" {
		return fmt.Errorf("missing required container or pod information")
	}

	if grant, found := p.releasePool(containerCtx); found {
		klog.InfoS("Released resources for container",
			"containerID", containerCtx.Request.ContainerMeta.ID,
			"podUID", containerCtx.Request.PodMeta.UID,
			"grant", grant)
	}

	return nil
}

func (p *TopologyAwarePolicy) releasePool(containerCtx policy.ContainerContext) (Grant, bool) {
	// 确保有必要的容器和 pod 信息
	if containerCtx.Request.ContainerMeta.ID == "" || containerCtx.Request.PodMeta.UID == "" {
		klog.Warningf("Missing required container or pod information")
		return nil, false
	}

	gid := containerCtx.Request.PodMeta.UID + ":" + containerCtx.Request.ContainerMeta.Name

	// 使用 sync.Map 的 Load 方法
	grantVal, ok := p.allocations.grants.Load(gid)
	if !ok {
		klog.V(5).InfoS("No grant found for container",
			"containerID", containerCtx.Request.ContainerMeta.ID,
			"gid", gid)
		return nil, false
	}

	grant := grantVal.(Grant)

	// 在释放资源前，先减少上层节点的资源使用情况
	p.propagateResourceReleaseToParent(grant)

	grant.Release()
	klog.V(5).InfoS("Released resources and propagated release to parent nodes", "grant", grant)

	// 使用 sync.Map 的 Delete 方法
	p.allocations.grants.Delete(gid)
	klog.V(5).InfoS("Deleted grant for container",
		"gid", gid,
		"containerID", containerCtx.Request.ContainerMeta.ID)
	p.saveAllocations()

	return grant, true
}

// propagateResourceReleaseToParent 根据 grant 中的分配信息来减少上层节点的资源使用情况
func (p *TopologyAwarePolicy) propagateResourceReleaseToParent(grant Grant) {
	currentNode := grant.GetNode()
	if currentNode == nil || currentNode.IsNil() {
		klog.V(4).InfoS("Current node is nil, skipping resource release propagation")
		return
	}

	klog.V(4).InfoS("Propagating resource release to parent nodes",
		"currentNode", currentNode.Name(),
		"allocatedCPUs", grant.AllocatedCPUs(),
		"allocatedCPUByRequest", grant.AllocatedCPUByRequest(),
		"allocatedCPUByLimit", grant.AllocatedCPUByLimit(),
		"allocatedMemory", grant.AllocatedMemory())

	// 从当前节点开始，逐层向上减少资源使用情况
	parent := currentNode.Parent()
	for parent != nil && !parent.IsNil() {
		// 根据 grant 中的分配信息来减少父节点的资源使用情况
		p.updateParentResourceUsageByRelease(parent, grant)

		parentSupply := parent.FreeResource()
		if parentSupply != nil {
			klog.V(4).InfoS("Updated parent resource usage after release",
				"parent", parent.Name(),
				"grantedShared", parentSupply.GrantedShared(),
				"grantedSharedRequest", parentSupply.GrantedCPUByRequest(),
				"grantedSharedLimit", parentSupply.GrantedCPUByLimit(),
				"grantedMemory", parentSupply.GrantedMemory())
		}

		// 继续向上传递
		parent = parent.Parent()
	}
}

// updateParentResourceUsageByRelease 根据 grant 中的分配信息来减少父节点的资源使用情况
func (p *TopologyAwarePolicy) updateParentResourceUsageByRelease(parent Node, grant Grant) {
	if parent == nil || parent.IsNil() {
		return
	}

	// 获取父节点的资源供应
	parentSupply := parent.FreeResource()
	if parentSupply == nil {
		klog.V(4).InfoS("Parent supply is nil, skipping release update", "parent", parent.Name())
		return
	}

	// 根据 grant 中的分配信息来减少父节点的资源使用情况
	if supply, ok := parentSupply.(*supply); ok {
		// 减少 grant 中分配的 CPU 资源
		supply.grantedShared -= grant.AllocatedCPUs()
		supply.grantedCPUByRequest -= grant.AllocatedCPUByRequest()
		supply.grantedCPUByLimit -= grant.AllocatedCPUByLimit()
		supply.grantedMemory -= grant.AllocatedMemory()

		// 确保值不会小于 0
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

		klog.V(5).InfoS("Updated parent resource usage by release",
			"parent", parent.Name(),
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

// saveAllocations 将当前的资源分配状态保存到 cache 中
func (p *TopologyAwarePolicy) saveAllocations() {
	if p.cache == nil {
		return
	}
}

// Helper method to find target node for container restoration
func (p *TopologyAwarePolicy) findTargetNodeForRestore(cpus string) Node {
	for _, node := range p.pools {
		if node.FreeResource().SharableCPUs().String() == cpus {
			return node
		}
	}
	return nil
}

// Helper method to create grant from container info
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

	// Set allocated CPU from container limits
	if resources := container.GetResourceRequirements(); resources.Limits != nil {
		if cpuLimit := resources.Limits.Cpu(); cpuLimit != nil {
			grant.SetAllocatedCPU(int(cpuLimit.MilliValue()))
		}
	}

	gid := pod.GetUID() + ":" + container.GetName()
	return grant, gid, nil
}

// Helper method to process a single container for restoration
func (p *TopologyAwarePolicy) processContainerForRestore(container cache.Container) error {
	cpus := container.GetCpusetCpus()
	if cpus == "" {
		return nil // Skip containers without CPU allocation
	}

	targetNode := p.findTargetNodeForRestore(cpus)
	if targetNode == nil {
		klog.Warningf("Node not found when restoring grant for container %s", container.GetID())
		return nil
	}

	grant, gid, err := p.createGrantFromContainer(container, targetNode)
	if err != nil {
		klog.Warningf("Failed to create grant for container %s: %v", container.GetID(), err)
		return nil
	}
	p.allocations.grants.Store(gid, grant)
	return nil
}

// restoreAllocations 从 cache 中恢复资源分配状态
func (p *TopologyAwarePolicy) restoreAllocations() error {
	if p.cache == nil {
		return nil
	}

	// 遍历所有容器
	for _, container := range p.cache.GetContainers() {
		if err := p.processContainerForRestore(container); err != nil {
			klog.ErrorS(err, "Failed to process container for restore", "containerID", container.GetID())
		}
	}

	return nil
}

func (p *TopologyAwarePolicy) initialize() error {
	klog.Info("Initializing topology-aware policy")
	p.nodes = nil
	p.pools = nil
	p.root = nil
	p.nodeCnt = 0
	p.depth = 0

	// 检测是否系统的资源限制
	if err := p.checkConstraints(); err != nil {
		return err
	}
	if err := p.buildResourcePoolsByTopology(); err != nil {
		return err
	}

	klog.Info("Topology-aware policy initialized")
	return nil
}

func (p *TopologyAwarePolicy) checkConstraints() error {
	// 默认设置为系统的资源参数值
	p.allowed = p.sys.AllowedSet()

	return nil
}

// createSocketNodes creates socket nodes and returns a map of socket ID to Node
func (p *TopologyAwarePolicy) createSocketNodes() map[system.ID]Node {
	sockets := make(map[system.ID]Node)

	for _, socketID := range p.sys.PackageIDs() {
		var socket Node

		if p.root != nil {
			socket = NewSocketNode(p, socketID, p.root)
			klog.V(5).InfoS("Created pool for ", "name", p.root.Name()+"/"+socket.Name())
		} else {
			// 单 Socket 时，直接设置为根节点
			socket = NewSocketNode(p, socketID, nilNode)
			p.root = socket
			klog.V(5).InfoS("Created single socket pool for ", "name", socket.Name())
		}

		p.nodes[socket.Name()] = socket
		sockets[socketID] = socket
	}

	return sockets
}

// validateDieStructure checks if die structure is valid
func (p *TopologyAwarePolicy) validateDieStructure() bool {
	hasDieTopology := false
	for _, socketID := range p.sys.PackageIDs() {
		pkg := p.sys.Package(socketID)
		dieIDs := pkg.DieIDs()

		if len(dieIDs) > 0 {
			hasDieTopology = true
			// 检查每个 die 的合法性
			for _, dieID := range dieIDs {
				// 检查 die ID 是否合法
				if dieID < 0 {
					klog.V(4).InfoS("Invalid die ID found",
						"socketID", socketID,
						"dieID", dieID)
					return false
				}

				// 检查该 die 是否有关联的 NUMA 节点
				if len(pkg.DieNodeIDs(dieID)) == 0 {
					klog.V(4).InfoS("Die has no associated NUMA nodes",
						"socketID", socketID,
						"dieID", dieID)
					return false
				}
			}
		}
	}

	if !hasDieTopology {
		klog.V(4).Info("No valid die topology detected")
		return false
	}

	klog.V(3).Info("Valid die topology detected")
	return true
}

// createDieNodes creates die nodes and returns a map of NUMA ID to its parent die Node
func (p *TopologyAwarePolicy) createDieNodes(sockets map[system.ID]Node) map[system.ID]Node {
	dies := make(map[system.ID]Node)

	// 检查是否存在合法的 die 层级
	if !p.validateDieStructure() {
		klog.V(4).Info("No valid die topology detected, skipping die node creation")
		return dies
	}

	// 遍历 Socket 节点，创建 die 节点
	for _, socketID := range p.sys.PackageIDs() {
		socket, ok := sockets[socketID]
		if !ok {
			klog.Warningf("Socket not found: socketID=%d", socketID)
			continue
		}

		pkg := p.sys.Package(socketID)

		for _, dieID := range pkg.DieIDs() {
			if len(pkg.DieNodeIDs(dieID)) > 0 {
				die := NewDieNode(p, dieID, socket)
				p.nodes[die.Name()] = die
				// 记录 die 节点下属的 NUMA 节点映射关系
				for _, numaID := range pkg.DieNodeIDs(dieID) {
					dies[numaID] = die
				}
				klog.V(5).InfoS("Created die node",
					"socketID", socketID,
					"dieID", dieID,
					"numaNodes", pkg.DieNodeIDs(dieID))
			}
		}
	}

	return dies
}

// createNumaNodes creates NUMA nodes and returns a map of NUMA ID to Node
func (p *TopologyAwarePolicy) createNumaNodes(sockets map[system.ID]Node, dies map[system.ID]Node) {
	for _, numaNodeID := range p.sys.NodeIDs() {
		var parent Node

		// 根据是否有 die 层级决定 NUMA 节点的父节点
		if len(dies) > 0 {
			if dieNode, exists := dies[numaNodeID]; exists {
				parent = dieNode
			} else {
				parent = sockets[p.sys.Node(numaNodeID).PackageID()]
			}
		} else {
			parent = sockets[p.sys.Node(numaNodeID).PackageID()]
		}

		numaNode := NewNumaNode(p, numaNodeID, parent)
		p.nodes[numaNode.Name()] = numaNode

		klog.V(5).InfoS("Created NUMA node",
			"numaNodeID", numaNodeID,
			"parent", parent.Name())
	}
}

// validateSocketStructure checks if socket structure is valid
func (p *TopologyAwarePolicy) validateSocketStructure() bool {
	if len(p.sys.PackageIDs()) == 0 {
		klog.Warning("No valid socket (package) found in system")
		return false
	}

	for _, socketID := range p.sys.PackageIDs() {
		if socketID < 0 {
			klog.Warningf("Invalid socket ID detected: socketID=%d", socketID)
			return false
		}
		pkg := p.sys.Package(socketID)
		if len(pkg.NodeIDs()) == 0 {
			klog.Warningf("Socket has no associated NUMA nodes: socketID=%d", socketID)
			return false
		}
	}
	return true
}

// buildResourcePoolsByTopology builds a hierarchical tree of pools based on HW topology.
func (p *TopologyAwarePolicy) buildResourcePoolsByTopology() error {
	if err := p.checkHWTopology(); err != nil {
		return err
	}
	klog.Info("Building pools by topology")

	p.nodes = make(map[string]Node)

	// 验证 Socket 结构
	hasValidSocketStructure := p.validateSocketStructure()
	if !hasValidSocketStructure {
		return fmt.Errorf("invalid socket structure detected")
	}

	// 多 Socket 时，创建虚拟根节点
	if len(p.sys.PackageIDs()) > 1 {
		p.root = NewVirtualNode(p, "root", nilNode)
		p.nodes[p.root.Name()] = p.root
		klog.V(5).InfoS("Created pool for ", "name", p.root.Name())
	} else {
		klog.V(5).InfoS("Omitted pool for virtual root (single-socket system)")
	}

	// 创建 Socket 节点
	sockets := p.createSocketNodes()

	// 创建 Die 节点
	dies := p.createDieNodes(sockets)
	if len(dies) > 0 {
		klog.Info("Die topology is valid, die nodes created")
	} else {
		klog.Info("No valid die topology detected, skipping die nodes")
	}

	// 创建 NUMA 节点
	p.createNumaNodes(sockets, dies)

	// 深度遍历资源树，计算深度和资源量，赋值 NUMA 节点
	p.pools = make([]Node, 0)
	err := p.root.DepthFirst(func(n Node) error {
		p.pools = append(p.pools, n)
		n.SetNodeID(p.nodeCnt)
		p.nodeCnt++

		if p.depth < n.Depth() {
			p.depth = n.Depth()
		}
		n.DiscoverResource()
		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to build resource pools: %v", err)
	}

	klog.InfoS("Topology pool setup completed",
		"socketCount", len(sockets),
		"dieCount", len(dies),
		"numaCount", len(p.sys.NodeIDs()))

	p.root.Dump("Topology pool setup completed")

	return nil
}

func (p *TopologyAwarePolicy) checkHWTopology() error {
	klog.Info("Checking hardware topology")

	if err := p.sys.ValidateTopology(); err != nil {
		klog.ErrorS(err, "Hardware topology check failed")
		return err
	}

	klog.Info("Hardware topology check passed")
	return nil
}

// startMetricsMonitoring 启动监控指标更新
func (p *TopologyAwarePolicy) startMetricsMonitoring() {
	klog.InfoS("Topology-aware policy monitoring enabled",
		"policy", p.Name(),
		"description", p.Description())

	// 立即执行一次指标更新
	if p.metricsManager != nil {
		p.metricsManager.UpdateAllMetrics()
	}
}

// updateAllMetrics 更新所有监控指标
func (p *TopologyAwarePolicy) updateAllMetrics() {
	// 使用新的监控管理器更新指标
	if p.metricsManager != nil {
		p.metricsManager.UpdateAllMetrics()
	}
}
