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

package system

import (
	"sort"

	"k8s.io/klog/v2"
	"k8s.io/utils/cpuset"
)

var _ CPUPackage = &cpuPackage{}

type cpuPackage struct {
	id       ID                   // package id
	cpus     cpuset.CPUSet        // CPUs in this package
	nodes    cpuset.CPUSet        // nodes in this package
	dies     cpuset.CPUSet        // dies in this package
	dieCPUs  map[ID]cpuset.CPUSet // CPUs per die
	dieNodes map[ID]cpuset.CPUSet // NUMA nodes per die
}

// Package methods
func (p *cpuPackage) ID() ID {
	return p.id
}

func (p *cpuPackage) CPUSet() cpuset.CPUSet {
	return p.cpus
}

func (p *cpuPackage) DieIDs() []ID {
	return p.dies.List()
}

func (p *cpuPackage) NodeIDs() []ID {
	return p.nodes.List()
}

func (p *cpuPackage) DieNodeIDs(dieID ID) []ID {
	return p.dieNodes[dieID].List()
}

func (p *cpuPackage) AddCPU(id ID) {
	p.cpus = p.cpus.Union(cpuset.New(int(id)))
}

func (p *cpuPackage) AddNode(id ID) {
	p.nodes = p.nodes.Union(cpuset.New(int(id)))
}

func (p *cpuPackage) AddDie(id ID) {
	// 创建一个新的 CPUSet，包含现有的 dies 和新的 die
	p.dies = p.dies.Union(cpuset.New(int(id)))
}

func (p *cpuPackage) AddCPUToDie(dieID ID, cpuID ID) {
	if p.dieCPUs == nil {
		p.dieCPUs = make(map[ID]cpuset.CPUSet)
	}
	if _, exists := p.dieCPUs[dieID]; !exists {
		p.dieCPUs[dieID] = cpuset.New()
	}
	// 使用 Union 来添加新的 CPU
	p.dieCPUs[dieID] = p.dieCPUs[dieID].Union(cpuset.New(int(cpuID)))
}

func (p *cpuPackage) AddNodeToDie(dieID ID, nodeID ID) {
	if p.dieNodes == nil {
		p.dieNodes = make(map[ID]cpuset.CPUSet)
	}
	if _, exists := p.dieNodes[dieID]; !exists {
		p.dieNodes[dieID] = cpuset.New()
	}
	// 使用 Union 来添加新的 node
	p.dieNodes[dieID] = p.dieNodes[dieID].Union(cpuset.New(int(nodeID)))
}

func (p *cpuPackage) DieCPUSet(dieID ID) cpuset.CPUSet {
	if cpus, ok := p.dieCPUs[dieID]; ok {
		return cpus
	}
	return cpuset.New()
}

func (p *cpuPackage) GetSortedNodes() []ID {
	ns := p.nodes.List()
	sort.Ints(ns)
	return ns
}

// MemoryInfo returns the aggregated memory information for all nodes in this package.
func (p *cpuPackage) MemoryInfo() (*MemInfo, error) {
	// 创建一个新的 MemInfo 结构体来存储聚合结果
	result := &MemInfo{
		MemTotal: 0,
		MemFree:  0,
		MemUsed:  0,
		MemSet:   cpuset.New(),
	}

	// 获取系统实例
	sys := &system{
		packages: map[ID]CPUPackage{p.id: p},
	}

	// 遍历包中的所有 NUMA 节点
	for _, nodeID := range p.NodeIDs() {
		node := sys.Node(nodeID)
		if node == nil {
			continue
		}

		// 获取节点的内存信息
		nodeMemInfo, err := node.MemoryInfo()
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
