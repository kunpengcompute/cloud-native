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

	"k8s.io/utils/cpuset"
	"kunpeng.huawei.com/kunpeng-cloud-computing/pkg/kunpeng-tap/policy"
)

// Grant represents a resource allocation grant to a container.
type Grant interface {
	GetContext() policy.ContainerContext
	// GetNode returns the Node this grant is allocated to.
	GetNode() Node
	// String returns a printable representation of this grant.
	String() string
	// AllocatedCPUs returns the amount of milli-CPU allocated.
	AllocatedCPUs() int
	// AllocatedCPUByRequest returns the amount of milli-CPU allocated by request.
	AllocatedCPUByRequest() int
	// AllocatedCPUByLimit returns the amount of milli-CPU allocated by limit.
	AllocatedCPUByLimit() int
	// Exclusive returns whether this grant is exclusive.
	Exclusive() bool
	// SetAllocatedCPU sets the amount of milli-CPU allocated.
	SetAllocatedCPU(allocatedCPUs int)
	// SetAllocatedCPUByRequest sets the amount of milli-CPU allocated by request.
	SetAllocatedCPUByRequest(allocatedCPUs int)
	// SetAllocatedCPUByLimit sets the amount of milli-CPU allocated by limit.
	SetAllocatedCPUByLimit(allocatedCPUs int)
	// AllocatedMemory returns the amount of memory allocated in KB.
	AllocatedMemory() uint64
	// SetAllocatedMemory sets the amount of memory allocated in KB.
	SetAllocatedMemory(memory uint64)
	// SharedCPUSet returns the set of CPUs this container can use.
	SharedCPUSet() cpuset.CPUSet
	// Memset returns the memory affinity set.
	Memset() cpuset.CPUSet
	// Release releases the resources held by this grant.
	Release()
}

var _ Grant = &grant{}

// grant implements the Grant interface.
type grant struct {
	containerCtx          policy.ContainerContext
	node                  Node
	exclusive             bool
	allocatedCPUs         int // milliCPUs
	allocatedCPUByRequest int
	allocatedCPUByLimit   int
	allocatedMemory       uint64 // memory in KB
}

// newGrant creates a new grant for a container.
func newGrant(n Node, ctrCtx policy.ContainerContext, exclusive bool, grantedCPUs int) Grant {
	return &grant{
		node:                  n,
		containerCtx:          ctrCtx,
		exclusive:             exclusive,
		allocatedCPUs:         grantedCPUs,
		allocatedCPUByRequest: 0,
		allocatedCPUByLimit:   0,
		allocatedMemory:       0,
	}
}

// GetContext returns the container context.
func (g *grant) GetContext() policy.ContainerContext {
	return g.containerCtx
}

func (g *grant) GetNode() Node {
	return g.node
}

func (g *grant) String() string {
	return fmt.Sprintf("<Grant: node %s, container %s, exclusive %v, allocatedCPUs %d, allocatedMemory %d KB>",
		g.node.Name(), g.containerCtx.Request.ContainerMeta.Name, g.exclusive, g.allocatedCPUs, g.allocatedMemory)
}

func (g *grant) AllocatedCPUs() int {
	return g.allocatedCPUs
}

func (g *grant) AllocatedCPUByRequest() int {
	return g.allocatedCPUByRequest
}

func (g *grant) AllocatedCPUByLimit() int {
	return g.allocatedCPUByLimit
}

func (g *grant) Exclusive() bool {
	return g.exclusive
}

func (g *grant) SetAllocatedCPU(allocatedCPUs int) {
	g.allocatedCPUs = allocatedCPUs
}

func (g *grant) SetAllocatedCPUByRequest(allocatedCPUs int) {
	g.allocatedCPUByRequest = allocatedCPUs
}

func (g *grant) SetAllocatedCPUByLimit(allocatedCPUs int) {
	g.allocatedCPUByLimit = allocatedCPUs
}

func (g *grant) AllocatedMemory() uint64 {
	return g.allocatedMemory
}

func (g *grant) SetAllocatedMemory(memory uint64) {
	g.allocatedMemory = memory
}

func (g *grant) SharedCPUSet() cpuset.CPUSet {
	return g.node.FreeResource().SharableCPUs()
}

func (g *grant) Memset() cpuset.CPUSet {
	return g.node.FreeResource().Memset()
}

func (g *grant) Release() {
	g.node.FreeResource().Release(g)
}
