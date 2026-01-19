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

package numa_aware

import (
	"math"
	"strconv"

	v1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
	"kunpeng.huawei.com/kunpeng-cloud-computing/pkg/kunpeng-tap/cache"
	policy "kunpeng.huawei.com/kunpeng-cloud-computing/pkg/kunpeng-tap/policy"
)

const (
	// PolicyName is the name used to identify this policy
	PolicyName = "numa-aware"

	// PolicyDescription describes the policy functionality
	PolicyDescription = "Policy for NUMA-aware container resource allocation"
)

type NumaAwarePolicy struct {
	policy.BasePolicy
	cache cache.Cache
}

// NewNumaAwarePolicy creates a new NUMA-aware policy
func NewNumaAwarePolicy(cache cache.Cache) policy.Policy {
	return &NumaAwarePolicy{
		BasePolicy: *policy.NewBasePolicy(PolicyName, PolicyDescription),
		cache:      cache,
	}
}

// Name returns the policy name
func (p *NumaAwarePolicy) Name() string {
	return PolicyName
}

// Description returns the policy description
func (p *NumaAwarePolicy) Description() string {
	return PolicyDescription
}

// SetCache sets the shared cache for the policy
func (p *NumaAwarePolicy) SetCache(c cache.Cache) {
	p.cache = c
}

// GetCache returns the cache for the policy
func (p *NumaAwarePolicy) GetCache() cache.Cache {
	return p.cache
}

// PreCreateContainerHook implements NUMA-aware CPU set allocation
func (p *NumaAwarePolicy) PreCreateContainerHook(ctx policy.HookContext) (*policy.Allocation, error) {
	containerCtx, ok := ctx.(*policy.ContainerContext)
	if !ok {
		klog.ErrorS(nil, "Invalid context type for PreCreateContainerHook")
		return nil, nil
	}

	request := containerCtx.Request

	if request.Resources == nil {
		klog.V(0).InfoS("Resources is nil")
		return nil, nil
	}

	resourceReq, resourceLimit := request.Resources.GetRequests(), request.Resources.GetLimits()
	if resourceReq == nil || resourceLimit == nil {
		klog.V(0).InfoS("Resource requirements or limits are nil")
		return nil, nil
	}

	qos := policy.ParseCgroupForQOSClass(request.CgroupParent)

	var alloc *policy.Allocation

	switch qos {
	case v1.PodQOSGuaranteed, v1.PodQOSBurstable:
		// Modify cpuset range for Guaranteed and Burstable pods
		alloc = p.AllocateCPUSet(resourceReq, resourceLimit)
	case v1.PodQOSBestEffort:
		// Don't modify anything for BestEffort pods
	}

	return alloc, nil
}

func (p *NumaAwarePolicy) AllocateCPUSet(request, limit *v1.ResourceList) *policy.Allocation {
	alloc := policy.NewAllocation()

	if request == nil || limit == nil {
		klog.V(0).InfoS("Request or limit is nil")
		return nil
	}

	reqCpu, limitCpu := request.Cpu().AsApproximateFloat64(), limit.Cpu().AsApproximateFloat64()
	klog.V(5).InfoS("reqCpu and limitCpu", "reqCpu", reqCpu, "limitCpu", limitCpu)
	nodeResources := p.cache.GetNodeResources()

	klog.V(5).InfoS("nodeResources", "resources", nodeResources)

	// Check if there are any node resources available
	if len(nodeResources) == 0 {
		klog.V(0).InfoS("No node resources available")
		return nil
	}

	// 按照请求资源选择节点
	// Fix: 默认 CPU 中所有 NUMA 的cpu数量一致
	cpuTotalInNode := nodeResources[0].CpuTotal

	// 超出一个节点，暂不处理
	if reqCpu > cpuTotalInNode || limitCpu > cpuTotalInNode {
		return nil
	}
	// 选择出 CPU 已分配量最低的节点
	var preferedNode int = -1
	used := math.MaxFloat64

	for i, v := range nodeResources {
		// req超出节点总CPU数量，不可亲和
		if reqCpu+v.CpuUsedByRequest > cpuTotalInNode {
			continue
		}

		if v.CpuUsed < used {
			preferedNode = i
			used = v.CpuUsed
		}
	}

	klog.V(5).InfoS("Selected preferred node", "nodeId", preferedNode)

	// Check if no suitable node was found
	if preferedNode == -1 {
		klog.V(0).InfoS("No suitable node found for CPU allocation")
		return nil
	}

	// TODO: 设置方法修改从系统信息直接获取，check 超线程等情况
	alloc.SetCPUSetCpus(strconv.Itoa(preferedNode*int(cpuTotalInNode)) + "-" + strconv.Itoa((preferedNode+1)*int(cpuTotalInNode)-1))

	klog.V(5).InfoS("Allocated cpuset", "cpuset", alloc.Resources.CpusetCpus)

	return alloc
}
