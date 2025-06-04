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
	allocations          allocations // container pool assignmentss
	enableMemoryTopology bool        // 是否启用内存拓扑感知
}

// Name returns the name of this policy.
func (p *TopologyAwarePolicy) Name() string {
	return PolicyName
}

func (p *TopologyAwarePolicy) Description() string {
	return PolicyDescription
}

// NewTopologyAwarePolicy creates a new topology-aware policy
func NewTopologyAwarePolicy(c cache.Cache, opts *policy.PolicyOptions) policy.Policy {
	sys, err := system.NewSystem("")
	if err != nil {
		klog.ErrorS(err, "Failed to discover system resources")
		return nil
	}
	p := &TopologyAwarePolicy{
		BasePolicy:           *policy.NewBasePolicy(PolicyName, PolicyDescription),
		sys:                  sys,
		cache:                c,
		enableMemoryTopology: opts.EnableMemoryTopology,
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

	return p
}

// 实现 PreCreateContainerHook 方法
func (p *TopologyAwarePolicy) PreCreateContainerHook(ctx policy.HookContext) (*policy.Allocation, error) {
	containerCtx, ok := ctx.(*policy.ContainerContext)
	if !ok {
		klog.ErrorS(nil, "Invalid context type for PreCreateContainerHook")
		return nil, fmt.Errorf("invalid context type: expected *policy.ContainerContext")
	}

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
	klog.V(5).InfoS("AllocateResources", "container request", containerCtx.Request)

	grant, err := p.allocatePool(containerCtx)
	if err != nil {
		return err
	}

	p.applyGrant(grant)
	klog.InfoS("Allocated resources for container", "container request", containerCtx.Request, "grant", grant)
	return nil
}

func (p *TopologyAwarePolicy) allocatePool(containerCtx policy.ContainerContext) (Grant, error) {
	request := newRequest(containerCtx)
	var pool Node

	if containerCtx.Request.PodMeta.Namespace == "kube-system" {
		// 系统容器直接分配到根节点
		klog.V(5).InfoS("Allocating system container to root pool", "container request", containerCtx.Request)
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

		pool = pools[0]
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

	p.saveAllocations()
	return grant, nil
}

func (p *TopologyAwarePolicy) calculatePoolAffinities(request Request) map[int]int32 {
	// 创建一个映射来存储每个NUMA节点的权重
	affinity := make(map[int]int32)
	containerCtx := request.GetContext()

	// 如果容器请求了GPU资源，增加对有GPU的NUMA节点的亲和性
	if request.HasGPURequest() {
		klog.V(3).InfoS("Container requests GPU resources",
			"pod", containerCtx.Request.PodMeta.Name,
			"container", containerCtx.Request.ContainerMeta.Name,
			"requestedDevices", request.GetRequestedGPUDevices())

		// 如果有特定的GPU设备请求，尝试找到这些设备所在的NUMA节点
		if len(request.GetRequestedGPUDevices()) > 0 {
			// 创建一个映射来存储每个NUMA节点的权重
			numaWeights := make(map[int]int)

			// 遍历所有GPU设备
			for _, gpuID := range p.sys.GPUIDs() {
				gpu := p.sys.GPU(gpuID)
				if gpu == nil {
					continue
				}

				// 将GPU ID转换为字符串以进行比较
				gpuIDStr := fmt.Sprintf("%d", gpuID)

				// 检查这个GPU是否是请求的设备之一
				for _, requestedID := range request.GetRequestedGPUDevices() {
					if gpuIDStr == requestedID {
						// 找到匹配的GPU，增加其NUMA节点的权重
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
		} else {
			// 如果没有特定的GPU设备请求，则增加所有有GPU的NUMA节点的亲和性
			for _, nodeID := range p.sys.NodeIDs() {
				gpuCount := len(p.sys.NodeGPUs(nodeID))
				if gpuCount > 0 {
					// 为每个GPU增加一定的亲和性权重
					affinity[int(nodeID)] += int32(gpuCount * 100)
					klog.V(3).InfoS("Added general GPU affinity for NUMA node",
						"numaNode", nodeID,
						"gpuCount", gpuCount,
						"weight", gpuCount*100)
				}
			}
		}
	}

	return affinity
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

// Compare two pools by scores for allocation preference.
func (p *TopologyAwarePolicy) compare(request Request, scores map[int]Score, affinity map[int]int32, i int, j int) bool {
	//
	// 比较计分的算法如下：
	// 1. 如果节点在树中较低，则获胜
	// 2. 如果是共享分配，则较少共置容器获胜
	// 3. 如果请求包含GPU，则GPU亲和性权重更高的节点获胜
	// 4. id 更小的获胜

	node1, node2 := p.pools[i], p.pools[j]
	depth1, depth2 := node1.RootDistance(), node2.RootDistance()
	id1, id2 := node1.NodeID(), node2.NodeID()
	score1, score2 := scores[id1], scores[id2]

	shared1 := score1.SharedCapacity()
	shared2 := score2.SharedCapacity()

	// 不满足资源要求的节点失败
	if node1.FreeResource().GetScore(request).SharedCapacity() >= 0 &&
		node2.FreeResource().GetScore(request).SharedCapacity() < 0 {
		klog.V(5).InfoS("Lower shared capacity wins", "winner", node1.Name(), "failed", node2.Name())
		return true
	}

	if node1.FreeResource().GetScore(request).SharedCapacity() < 0 &&
		node2.FreeResource().GetScore(request).SharedCapacity() >= 0 {
		klog.V(5).InfoS("Lower shared capacity wins", "winner", node2.Name(), "failed", node1.Name())
		return false
	}

	// 层次更低的节点获胜
	if depth1 > depth2 {
		klog.V(5).InfoS("Lower depth wins", "winner", node1.Name(), "failed", node2.Name())
		return true
	}
	if depth2 > depth1 {
		klog.V(5).InfoS("Lower depth wins", "winner", node2.Name(), "failed", node1.Name())
		return false
	}

	klog.V(5).InfoS("Depth is a TIE", "node1", node1.Name(), "node2", node2.Name())

	// 更多共享容量的节点获胜
	if shared1 > shared2 {
		return true
	}
	if shared2 > shared1 {
		return false
	}
	klog.V(5).InfoS("Shared capacity is a TIE", "node1", node1.Name(), "node2", node2.Name())

	// 较少共置容器的节点获胜
	if score1.Colocated() < score2.Colocated() {
		klog.V(5).InfoS("Fewer colocated containers wins", "winner", node1.Name(), "failed", node2.Name())
		return true
	}
	if score2.Colocated() < score1.Colocated() {
		klog.V(5).InfoS("Fewer colocated containers wins", "winner", node2.Name(), "failed", node1.Name())
		return false
	}

	// 如果请求包含GPU，则考虑GPU亲和性和GPU数量
	if request.HasGPURequest() {
		// 获取两个节点的 NUMA IDs
		numaIDs1 := node1.FreeResource().GetNode().GetNUMAIDs()
		numaIDs2 := node2.FreeResource().GetNode().GetNUMAIDs()
		klog.V(5).InfoS("GPU affinity compare", "node1", node1.Name(), "node2", node2.Name(), "numaIDs1", numaIDs1, "numaIDs2", numaIDs2)
		// 计算每个节点的总亲和性权重
		var affinity1, affinity2 int32
		for _, numaID := range numaIDs1 {
			affinity1 += affinity[int(numaID)]
		}
		for _, numaID := range numaIDs2 {
			affinity2 += affinity[int(numaID)]
		}

		klog.V(5).InfoS("GPU affinity compare", "node1", node1.Name(), "node2", node2.Name(), "affinity1", affinity1, "affinity2", affinity2)

		if affinity1 > affinity2 {
			klog.V(5).InfoS("GPU affinity wins",
				"winner", node1.Name(),
				"numaIDs", numaIDs1,
				"affinity", affinity1)
			return true
		}
		if affinity2 > affinity1 {
			klog.V(5).InfoS("GPU affinity wins",
				"winner", node2.Name(),
				"numaIDs", numaIDs2,
				"affinity", affinity2)
			return false
		}
	}
	klog.V(5).InfoS("GPU affinity is a TIE", "node1", node1.Name(), "node2", node2.Name())
	// id 更小的节点获胜
	return id1 < id2
}

func (p *TopologyAwarePolicy) PostStopContainerHook(ctx policy.HookContext) (*policy.Allocation, error) {
	klog.V(5).InfoS("PostStopContainerHook", "container request", ctx)
	containerCtx, ok := ctx.(*policy.ContainerContext)
	if !ok {
		klog.ErrorS(nil, "Invalid context type for PostStopContainerHook")
		return nil, nil
	}

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
	klog.V(5).InfoS("ReleaseResources", "container request", containerCtx.Request)

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
	grant.Release()

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

	// 遍历所有的 grants 并保存
	// p.allocations.grants.Range(func(key, value interface{}) bool {
	// 	grant := value.(Grant)
	// 	containerCtx := grant.GetContext()

	// 	// 更新容器的资源分配信息
	// 	if container, ok := p.cache.LookupContainer(containerCtx.Request.ContainerMeta.ID); ok {
	// 		// 设置 CPU 集
	// 		cpus := grant.SharedCPUSet().String()
	// 		container.SetCpusetCpus(cpus)

	// 		// 设置内存节点
	// 		if p.enableMemoryTopology {
	// 			memset := grant.Memset().String()
	// 			container.SetCpusetMems(memset)
	// 		}
	// 	}
	// 	return true
	// })
}

// restoreAllocations 从 cache 中恢复资源分配状态
func (p *TopologyAwarePolicy) restoreAllocations() error {
	if p.cache == nil {
		return nil
	}

	// 遍历所有容器
	for _, container := range p.cache.GetContainers() {
		// 获取容器的 CPU 集和内存节点信息
		cpus := container.GetCpusetCpus()

		if cpus == "" {
			continue
		}

		// 查找对应的节点
		var targetNode Node
		for _, node := range p.pools {
			if node.FreeResource().SharableCPUs().String() == cpus {
				targetNode = node
				break
			}
		}

		if targetNode == nil {
			klog.Warningf("Node not found when restoring grant for container %s", container.GetID())
			continue
		}

		// 获取 pod 信息
		pod, ok := container.GetPod()
		if !ok {
			klog.Warningf("Pod not found for container %s", container.GetID())
			continue
		}

		// 创建新的 grant
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

		// 设置分配的资源
		// 从容器的 CPU limits 获取分配的 CPU 数量
		if resources := container.GetResourceRequirements(); resources.Limits != nil {
			if cpuLimit := resources.Limits.Cpu(); cpuLimit != nil {
				grant.SetAllocatedCPU(int(cpuLimit.MilliValue()))
			}
		}

		// 将 grant 添加到 allocations 中
		gid := pod.GetUID() + ":" + container.GetName()
		p.allocations.grants.Store(gid, grant)
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
			klog.V(5).InfoS("Created pool for ", "name", socket.Name())
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
		socket := sockets[socketID]
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
