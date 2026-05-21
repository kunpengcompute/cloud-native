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

// ScoreContext provides external information needed for score calculation.
// This interface breaks the circular dependency between Node and TopologyAwarePolicy
// by allowing the Policy to provide colocation and GPU information without being
// directly referenced by Node.
type ScoreContext interface {
	// CountColocation returns the number of containers already allocated to the specified node.
	CountColocation(nodeID int) int
	// CountNodeGPUs returns the number of GPUs attached to the specified NUMA node.
	CountNodeGPUs(numaID system.ID) int
}

// Supply represents the resource capacity available at a node.
type Supply interface {
	// GetNode returns the node supplying this capacity.
	GetNode() Node
	// Collect collects the given supply into this one.
	Collect(Supply)
	// Clone clones the given supply.
	Clone() Supply
	// GetScore calculates how well this supply fits/fulfills the given request.
	// The ctx parameter provides external information (colocation count, GPU count)
	// that would otherwise require a circular dependency on the policy.
	GetScore(req Request, ctx ScoreContext) Score
	// GrantedShared returns the locally granted shared CPU capacity.
	GrantedShared() int
	// GrantedCPUByRequest returns the amount of milli-CPU granted by request.
	GrantedCPUByRequest() int
	// GrantedCPUByLimit returns the amount of milli-CPU granted by limit.
	GrantedCPUByLimit() int
	// AllocatableSharedCPU calculates the allocatable amount of shared CPU of this supply.
	AllocatableSharedCPU() int

	// Allocate allocates resources for the given request.
	Allocate(Request) (Grant, error)

	// SharableCPUs returns the sharable cpuset in this supply.
	SharableCPUs() cpuset.CPUSet
	// IsolatedCPUs returns the isolated cpuset in this supply.
	IsolatedCPUs() cpuset.CPUSet
	// TotalCPUs returns the total cpuset (isolated + sharable) in this supply.
	TotalCPUs() cpuset.CPUSet

	// Release releases the resources held by the given grant.
	Release(Grant)

	// String returns a printable representation of this supply.
	String() string

	// GrantedMemory returns the granted memory in KB.
	GrantedMemory() uint64
	// AllocatableMemory returns the allocatable memory in KB.
	AllocatableMemory() uint64
	// Memset returns the memory affinity set.
	Memset() cpuset.CPUSet
}

// supply implements the Supply interface.
type supply struct {
	node                Node
	isolated            cpuset.CPUSet
	sharable            cpuset.CPUSet
	grantedShared       int
	grantedCPUByRequest int
	grantedCPUByLimit   int
	memoryTotal         uint64 // Total memory in KB
	grantedMemory       uint64 // Granted memory in KB
}

// newSupply creates a new supply for the given node.
func newSupply(n Node, isolated cpuset.CPUSet, sharable cpuset.CPUSet) Supply {
	// Get memory info for the node
	memTotal := uint64(0)
	memInfo, err := n.MemoryInfo()
	if err == nil && memInfo != nil {
		memTotal = memInfo.MemTotal
	}

	return &supply{
		node:                n,
		isolated:            isolated,
		sharable:            sharable,
		grantedShared:       0,
		grantedCPUByRequest: 0,
		grantedCPUByLimit:   0,
		memoryTotal:         memTotal,
		grantedMemory:       0,
	}
}

func (s *supply) String() string {
	return fmt.Sprintf("<Supply: node %s, isolated %s, sharable %s, granted CPU %d, memory total %d KB, granted memory %d KB, allocatable memory %d KB>",
		s.node.Name(), s.isolated, s.sharable, s.grantedShared, s.memoryTotal, s.grantedMemory, s.AllocatableMemory())
}

func (s *supply) GetNode() Node {
	return s.node
}

// SharableCPUs returns the sharable CPUSet of this supply.
func (s *supply) SharableCPUs() cpuset.CPUSet {
	return s.sharable.Clone()
}

// IsolatedCPUs returns the isolated CPUSet of this supply.
func (s *supply) IsolatedCPUs() cpuset.CPUSet {
	return s.isolated.Clone()
}

// TotalCPUs returns the total CPUSet (isolated + sharable) of this supply.
func (s *supply) TotalCPUs() cpuset.CPUSet {
	return s.isolated.Union(s.sharable)
}

// Collect collects the given supply into this one.
func (s *supply) Collect(more Supply) {
	moreSupply, ok := more.(*supply)
	if !ok {
		klog.ErrorS(nil, "Failed to collect supply", "supply", more)
		return
	}
	s.isolated = s.isolated.Union(moreSupply.isolated)
	s.sharable = s.sharable.Union(moreSupply.sharable)
	s.grantedShared += moreSupply.grantedShared
	s.grantedCPUByRequest += moreSupply.grantedCPUByRequest
	s.grantedCPUByLimit += moreSupply.grantedCPUByLimit
	s.memoryTotal += moreSupply.memoryTotal
	s.grantedMemory += moreSupply.grantedMemory
}

// GetScore collects data for scoring this supply against the given request.
// The ctx parameter provides external information (colocation count, GPU count)
// that breaks the circular dependency on the policy.
func (s *supply) GetScore(req Request, ctx ScoreContext) Score {
	score := &score{
		supply:    s,
		request:   req,
		colocated: 0,
	}
	// Calculate allocatable shared CPU
	score.shared = s.AllocatableSharedCPU() - req.CPULimit()
	score.sharedByRequest = s.AllocatableCPUByRequest() - req.CPURequest()
	score.sharedByLimit = s.AllocatableCPUByLimit() - req.CPULimit()
	score.memoryCapacity = s.AllocatableMemory()

	// Calculate colocation score - use context to avoid circular dependency
	score.colocated = ctx.CountColocation(s.node.NodeID())

	// Calculate GPU count for this node - use context to avoid circular dependency
	numaIDs := s.node.GetNUMAIDs()
	score.gpuCount = 0
	for _, numaID := range numaIDs {
		score.gpuCount += ctx.CountNodeGPUs(numaID)
	}

	return score
}

func (s *supply) GrantedShared() int {
	return s.grantedShared
}

func (s *supply) GrantedCPUByRequest() int {
	return s.grantedCPUByRequest
}

func (s *supply) GrantedCPUByLimit() int {
	return s.grantedCPUByLimit
}

func (s *supply) AllocatableSharedCPU() int {
	shared := 1000 * s.sharable.Size()
	return shared - s.grantedShared
}

// TotalSharedCPU returns the total shared CPU capacity.
func (s *supply) TotalSharedCPU() int {
	shared := 1000 * s.sharable.Size()
	return shared
}

// AllocatableCPUByRequest returns the allocatable CPU by request.
func (s *supply) AllocatableCPUByRequest() int {
	return s.TotalSharedCPU() - s.grantedCPUByRequest
}

// AllocatableCPUByLimit returns the allocatable CPU by limit.
func (s *supply) AllocatableCPUByLimit() int {
	return s.TotalSharedCPU() - s.grantedCPUByLimit
}

func (s *supply) Allocate(req Request) (Grant, error) {
	grant, err := s.AllocateCPU(req)
	if err != nil {
		return nil, err
	}

	// Handle memory allocation
	memoryRequest := req.GetContext().Request.Resources.GetLimits().Memory().Value() / 1024 // Convert to KB
	if memoryRequest > 0 {
		// Check if there is enough memory
		if uint64(memoryRequest) > s.AllocatableMemory() {
			// Log warning but don't fail - CPU was already allocated
			klog.ErrorS(nil, "Not enough memory for container",
				"node", s.node.Name(),
				"request", req,
				"available", s.AllocatableMemory())
		}

		// Allocate memory
		s.grantedMemory += uint64(memoryRequest)

		// Set memory allocation info to grant
		grant.SetAllocatedMemory(uint64(memoryRequest))
	}

	return grant, nil
}

// AllocateCPU allocates CPU resources from this supply and returns a grant.
// Actual allocation uses request value, not limit (limit is only used for pool selection).
func (s *supply) AllocateCPU(req Request) (Grant, error) {
	grant := newGrant(s.node, req.GetContext(), false, 0)

	resource := req.GetContext().Request.Resources
	requestCpu := resource.GetRequests().Cpu().MilliValue()
	limitCpu := resource.GetLimits().Cpu().MilliValue()

	// Check if request exceeds total capacity
	totalSharedCPU := s.TotalSharedCPU()
	if requestCpu+int64(s.GrantedCPUByRequest()) > int64(totalSharedCPU) {
		return nil, fmt.Errorf("request CPU %d exceeds total shared CPU %d", requestCpu, totalSharedCPU)
	}

	// Allocate request value (limit capacity check was done in pool selection)
	allocatedCpu := int(requestCpu)

	klog.V(5).InfoS("Allocating CPU based on request",
		"requestCpu", requestCpu,
		"limitCpu", limitCpu,
		"allocatedCpu", allocatedCpu,
		"node", s.node.Name())

	// Update the granted CPU for grant and supply.
	grant.SetAllocatedCPU(allocatedCpu)
	grant.SetAllocatedCPUByRequest(int(requestCpu))
	grant.SetAllocatedCPUByLimit(int(limitCpu))

	// Update the granted CPU for supply.
	s.grantedShared += allocatedCpu
	s.grantedCPUByRequest += int(requestCpu)
	s.grantedCPUByLimit += int(limitCpu)

	return grant, nil
}

// AllocatableMemory returns the allocatable memory in KB.
func (s *supply) AllocatableMemory() uint64 {
	if s.memoryTotal <= s.grantedMemory {
		return 0
	}
	return s.memoryTotal - s.grantedMemory
}

// GrantedMemory returns the granted memory in KB.
func (s *supply) GrantedMemory() uint64 {
	return s.grantedMemory
}

func (s *supply) Memset() cpuset.CPUSet {
	memInfo, err := s.node.MemoryInfo()
	if err != nil {
		return cpuset.New()
	}
	return memInfo.MemSet
}

// Release releases the resources held by the given grant.
func (s *supply) Release(g Grant) {
	// Release CPU resources
	s.grantedShared -= g.AllocatedCPUs()
	s.grantedCPUByRequest -= g.AllocatedCPUByRequest()
	s.grantedCPUByLimit -= g.AllocatedCPUByLimit()
	// Ensure grantedShared doesn't go below 0
	if s.grantedShared < 0 {
		s.grantedShared = 0
	}

	// Release memory resources
	if memGrant, ok := g.(*grant); ok {
		if s.grantedMemory < memGrant.allocatedMemory {
			s.grantedMemory = 0
		} else {
			s.grantedMemory -= memGrant.allocatedMemory
		}
	}
}

func (s *supply) Clone() Supply {
	return newSupply(s.node, s.isolated, s.sharable)
}
