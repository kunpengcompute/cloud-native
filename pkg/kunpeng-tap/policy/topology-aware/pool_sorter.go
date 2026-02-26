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
	"sort"

	"k8s.io/klog/v2"
	"kunpeng.huawei.com/kunpeng-cloud-computing/cmd/kunpeng-tap/proxy/options"
)

// sortPoolsByScore sorts pools by their scores for the given request.
// The pools parameter allows sorting a filtered subset of pools (filter-first strategy).
func (p *TopologyAwarePolicy) sortPoolsByScore(req Request, affinity map[int]int32, pools []Node) (map[int]Score, []Node) {
	scores := make(map[int]Score, len(pools))

	// Only calculate scores for the provided pools
	for _, n := range pools {
		scores[n.NodeID()] = n.GetScore(req, p)
	}

	sort.Slice(pools, func(i, j int) bool {
		return p.compareNodes(req, scores, affinity, pools[i], pools[j])
	})

	// Output sorted pools info
	for _, n := range pools {
		klog.V(5).InfoS("Sorted pool", "node", n.Name())
	}
	return scores, pools
}

// compareNodes compares two pool nodes by scores for allocation preference.
func (p *TopologyAwarePolicy) compareNodes(request Request, scores map[int]Score, affinity map[int]int32, node1, node2 Node) bool {
	// Comparison algorithm:
	// 1. Resource capacity check (whether request can be satisfied)
	// 2. Depth comparison (deeper tree nodes preferred, Cluster depth > NUMA depth)
	// 3. Shared capacity comparison (more shared capacity preferred)
	// 4. Colocated containers comparison (fewer colocated containers preferred)
	// 5. GPU affinity comparison (higher GPU affinity preferred)
	// 6. ID comparison (smaller ID preferred)

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

	// 2. Check depth (Cluster nodes have greater depth than NUMA nodes)
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

// compareResourceCapacity compares resource capacity between two nodes.
// Uses SharedCapacity() (based on limit) for sorting to ensure selected node can accommodate limit.
func (p *TopologyAwarePolicy) compareResourceCapacity(request Request, node1, node2 Node) (bool, bool) {
	capacity1 := node1.FreeResource().GetScore(request, p).SharedCapacity()
	capacity2 := node2.FreeResource().GetScore(request, p).SharedCapacity()

	// Nodes with sufficient capacity win
	if capacity1 >= 0 && capacity2 < 0 {
		klog.V(5).InfoS("Sufficient capacity wins", "winner", node1.Name(), "capacity", capacity1, "failed", node2.Name(), "capacity", capacity2)
		return true, true // node1 wins, comparison complete
	}

	if capacity1 < 0 && capacity2 >= 0 {
		klog.V(5).InfoS("Sufficient capacity wins", "winner", node2.Name(), "capacity", capacity2, "failed", node1.Name(), "capacity", capacity1)
		return false, true // node2 wins, comparison complete
	}

	// Both have capacity or both lack capacity - let subsequent logic decide
	return false, false // tie, continue comparison
}

// compareDepth compares depths of two nodes.
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

// compareSharedCapacity compares shared capacity between two nodes.
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

// compareColocated compares number of colocated containers.
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

// compareGPUAffinity compares GPU affinity between two nodes.
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
		// GPU-First strategy: enhance GPU affinity weight
		if p.resourcePriority == options.ResourcePriorityGPUFirst && baseAffinity > 0 {
			affinity1 += baseAffinity * 100 // Enhance weight 100x to prioritize GPU affinity
		} else {
			affinity1 += baseAffinity
		}
	}
	for _, numaID := range numaIDs2 {
		baseAffinity := affinity[int(numaID)]
		// GPU-First strategy: enhance GPU affinity weight
		if p.resourcePriority == options.ResourcePriorityGPUFirst && baseAffinity > 0 {
			affinity2 += baseAffinity * 100 // Enhance weight 100x to prioritize GPU affinity
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

// applyResourcePriorityStrategy applies the configured resource priority strategy.
func (p *TopologyAwarePolicy) applyResourcePriorityStrategy(request Request, affinity map[int]int32, node1, node2 Node) (bool, bool) {
	switch p.resourcePriority {
	case options.ResourcePriorityGPUFirst:
		// GPU-first strategy: compare GPU affinity first, then CPU capacity
		if result, done := p.compareGPUAffinity(request, affinity, node1, node2); done {
			return result, true
		}
		// If GPU affinity is same, compare CPU capacity
		if result, done := p.compareCPUCapacity(node1, node2); done {
			return result, true
		}
	case options.ResourcePriorityCPUFirst:
		fallthrough
	default:
		// CPU-first (default): compare CPU capacity first, then GPU affinity
		if result, done := p.compareCPUCapacity(node1, node2); done {
			return result, true
		}
		// If CPU capacity is same, compare GPU affinity
		if result, done := p.compareGPUAffinity(request, affinity, node1, node2); done {
			return result, true
		}
	}

	return false, false // continue other comparisons
}

// compareCPUCapacity compares CPU capacity between two nodes.
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
