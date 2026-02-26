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
	"sync"

	v1 "k8s.io/api/core/v1"
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

	cache                 cache.Cache
	sys                   system.System
	allowed               cpuset.CPUSet
	isolated              cpuset.CPUSet
	nodes                 map[string]Node
	pools                 []Node
	root                  Node
	nodeCnt               int
	depth                 int
	allocations           allocations             // container pool assignmentss
	enableMemoryTopology  bool                    // 是否启用内存拓扑感知
	resourcePriority      string                  // 资源分配优先级策略
	enableClusterAffinity bool                    // 是否启用 cluster 亲和
	metricsManager        *TopologyMetricsManager // 资源树监控管理器
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

// CountColocation returns the number of containers already allocated to the specified node.
// Implements the ScoreContext interface.
func (p *TopologyAwarePolicy) CountColocation(nodeID int) int {
	count := 0
	p.allocations.grants.Range(func(_, grantVal interface{}) bool {
		grant, ok := grantVal.(Grant)
		if !ok {
			klog.ErrorS(nil, "Invalid grant type in allocations", "grantVal", grantVal)
			return true
		}
		if grant.GetNode().NodeID() == nodeID {
			count++
		}
		return true
	})
	return count
}

// CountNodeGPUs returns the number of GPUs attached to the specified NUMA node.
// Implements the ScoreContext interface.
func (p *TopologyAwarePolicy) CountNodeGPUs(numaID system.ID) int {
	return len(p.sys.NodeGPUs(numaID))
}

// Ensure TopologyAwarePolicy implements ScoreContext.
var _ ScoreContext = &TopologyAwarePolicy{}

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
		BasePolicy:            *policy.NewBasePolicy(PolicyName, PolicyDescription),
		sys:                   sys,
		cache:                 c,
		enableMemoryTopology:  opts.EnableMemoryTopology,
		resourcePriority:      opts.ResourcePriority,
		enableClusterAffinity: opts.EnableClusterAffinity,
	}
	p.allocations = p.newAllocations()

	// 如果启用了 cluster 亲和，检查机器是否支持 Cluster 特性并验证 cluster 拓扑
	if p.enableClusterAffinity {
		if sys.SupportsClusterFeature() {
			// Cluster 拓扑已经在 System.Discover() 中被发现
			// 这里只需要验证是否有可用的 clusters
			clusterIDs := sys.ClusterIDs()
			if len(clusterIDs) > 0 {
				klog.InfoS("Cluster affinity enabled",
					"numClusters", len(clusterIDs))
			} else {
				klog.InfoS("Cluster affinity enabled but no clusters found, feature disabled")
				p.enableClusterAffinity = false
			}
		} else {
			klog.InfoS("Cluster affinity enabled but machine doesn't support cluster feature, feature disabled")
			p.enableClusterAffinity = false
		}
	}

	if err := p.initialize(); err != nil {
		klog.ErrorS(err, "Failed to initialize topology-aware policy")
		return nil
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
		// Step 1: 先过滤有足够容量的池 (Filter-First)
		filteredPools := p.filterPoolsByCapacity(request)
		if len(filteredPools) == 0 {
			return nil, fmt.Errorf("failed to allocate cpu: no pool with sufficient capacity for container %s",
				containerCtx.Request.PodMeta.Name)
		}

		// Step 2: 计算其他资源的亲和性
		affinity := p.calculatePoolAffinities(request)

		// Step 3: 对过滤后的池进行排序 (Then-Sort)
		score, sortedPools := p.sortPoolsByScore(request, affinity, filteredPools)
		if len(sortedPools) == 0 {
			return nil, fmt.Errorf("failed to allocate cpu: no suitable pool found for container %s",
				containerCtx.Request.PodMeta.Name)
		}

		for _, n := range sortedPools {
			klog.V(5).InfoS("node fitting for container", "node", n.Name(), "score", score[n.NodeID()].String())
		}

		// Step 4: 选择最优的资源池 (使用 GPU-first 和深度优先逻辑)
		pool = p.findBestAvailablePool(request, sortedPools)
		if pool == nil {
			return nil, fmt.Errorf("failed to allocate cpu: no suitable pool found for container %s",
				containerCtx.Request.PodMeta.Name)
		}
		klog.V(4).InfoS("Selected best pool from filtered and sorted pools",
			"pool", pool.Name(),
			"filteredCount", len(filteredPools),
			"score", score[pool.NodeID()].String())
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

// saveAllocations 将当前的资源分配状态保存到 cache 中
func (p *TopologyAwarePolicy) saveAllocations() {
	if p.cache == nil {
		return
	}
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
