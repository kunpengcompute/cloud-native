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

	"k8s.io/klog/v2"
	"kunpeng.huawei.com/kunpeng-cloud-computing/pkg/kunpeng-tap/monitoring"
)

// TopologyMetricsManager 负责以资源树为中心的监控指标管理
type TopologyMetricsManager struct {
	policy *TopologyAwarePolicy
	mu     sync.RWMutex
}

// NewTopologyMetricsManager 创建新的监控指标管理器
func NewTopologyMetricsManager(policy *TopologyAwarePolicy) *TopologyMetricsManager {
	return &TopologyMetricsManager{
		policy: policy,
	}
}

// UpdateAllMetrics 更新所有以资源树为中心的监控指标
func (m *TopologyMetricsManager) UpdateAllMetrics() {
	m.mu.Lock()
	defer m.mu.Unlock()

	klog.V(4).InfoS("Updating topology metrics")

	// 更新资源树节点容量指标
	m.updateNodeCapacityMetrics()

	// 更新资源树节点使用指标
	m.updateNodeUsageMetrics()

	// 更新容器分布指标
	m.updateContainerDistributionMetrics()

	klog.V(4).InfoS("Topology metrics updated successfully")
}

// updateNodeCapacityMetrics 更新节点容量指标
func (m *TopologyMetricsManager) updateNodeCapacityMetrics() {
	if m.policy == nil || m.policy.root == nil {
		return
	}

	for _, node := range m.policy.pools {
		if node == nil || node.IsNil() {
			continue
		}

		supply := node.FreeResource()
		if supply == nil {
			continue
		}

		// 获取节点信息
		nodeName := node.Name()
		nodeType := string(node.Kind())
		nodeID := fmt.Sprintf("%d", node.NodeID())
		hierarchyLevel := m.getHierarchyLevel(node)

		// CPU 容量指标
		monitoring.TopologyNodeCapacity.WithLabelValues(
			nodeName,
			nodeType,
			nodeID,
			hierarchyLevel,
			"cpu",
			"total_cores",
		).Set(float64(supply.SharableCPUs().Size()))

		// 内存容量指标
		memInfo, err := node.MemoryInfo()
		if err == nil && memInfo != nil {
			monitoring.TopologyNodeCapacity.WithLabelValues(
				nodeName,
				nodeType,
				nodeID,
				hierarchyLevel,
				"memory",
				"total_kb",
			).Set(float64(memInfo.MemTotal))
		}
	}
}

// updateNodeUsageMetrics 更新节点使用指标
func (m *TopologyMetricsManager) updateNodeUsageMetrics() {
	if m.policy == nil || m.policy.root == nil {
		return
	}

	for _, node := range m.policy.pools {
		if node == nil || node.IsNil() {
			continue
		}

		supply := node.FreeResource()
		if supply == nil {
			continue
		}

		// 获取节点信息
		nodeName := node.Name()
		nodeType := string(node.Kind())
		nodeID := fmt.Sprintf("%d", node.NodeID())
		hierarchyLevel := m.getHierarchyLevel(node)

		// CPU 使用指标
		monitoring.TopologyNodeUsage.WithLabelValues(
			nodeName,
			nodeType,
			nodeID,
			hierarchyLevel,
			"cpu",
			"allocated_millicores",
		).Set(float64(supply.GrantedShared()))

		monitoring.TopologyNodeUsage.WithLabelValues(
			nodeName,
			nodeType,
			nodeID,
			hierarchyLevel,
			"cpu",
			"request_millicores",
		).Set(float64(supply.GrantedCPUByRequest()))

		monitoring.TopologyNodeUsage.WithLabelValues(
			nodeName,
			nodeType,
			nodeID,
			hierarchyLevel,
			"cpu",
			"limit_millicores",
		).Set(float64(supply.GrantedCPUByLimit()))

		// 内存使用指标
		monitoring.TopologyNodeUsage.WithLabelValues(
			nodeName,
			nodeType,
			nodeID,
			hierarchyLevel,
			"memory",
			"allocated_kb",
		).Set(float64(supply.GrantedMemory()))
	}
}

// updateContainerDistributionMetrics 更新容器分布指标
func (m *TopologyMetricsManager) updateContainerDistributionMetrics() {
	if m.policy == nil {
		return
	}

	// 统计每个节点上的容器数量
	nodeContainerCount := make(map[string]int)

	// 遍历所有 grants 来统计容器分布
	m.policy.allocations.grants.Range(func(_, grantVal interface{}) bool {
		grant, ok := grantVal.(Grant)
		if !ok {
			return true
		}

		node := grant.GetNode()
		if node == nil || node.IsNil() {
			return true
		}

		nodeName := node.Name()
		nodeContainerCount[nodeName]++
		return true
	})

	// 更新容器分布指标
	for nodeName, count := range nodeContainerCount {
		// 通过 p.nodes 直接查找节点
		targetNode, exists := m.policy.nodes[nodeName]
		if !exists || targetNode == nil {
			continue
		}

		nodeType := string(targetNode.Kind())
		nodeID := fmt.Sprintf("%d", targetNode.NodeID())
		hierarchyLevel := m.getHierarchyLevel(targetNode)

		monitoring.TopologyNodeContainerCount.WithLabelValues(
			nodeName,
			nodeType,
			nodeID,
			hierarchyLevel,
			"running",
		).Set(float64(count))
	}
}

// getHierarchyLevel 获取节点的层次级别
func (m *TopologyMetricsManager) getHierarchyLevel(node Node) string {
	switch node.Kind() {
	case VirtualNode:
		return "virtual"
	case SocketNode:
		return "socket"
	case NumaNode:
		return "numa"
	case ClusterNode:
		return "cluster"
	default:
		return "unknown"
	}
}
