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
	"kunpeng.huawei.com/kunpeng-cloud-computing/pkg/kunpeng-tap/sysfs/system"
)

// MockSystem implements system.System interface for testing
type MockSystem struct {
	packages               map[system.ID]*MockCPUPackage
	nodes                  map[system.ID]*MockNode
	cpus                   map[system.ID]*MockCPU
	gpus                   map[system.ID]*MockGPU
	clusters               map[system.ID]system.Cluster // Mock clusters
	allowedSet             cpuset.CPUSet
	packageIDs             []system.ID
	nodeIDs                []system.ID
	clusterIDs             []system.ID // Mock cluster IDs
	cpuIDs                 []system.ID
	gpuIDs                 []system.ID
	distances              map[system.ID]map[system.ID]int
	shouldFail             bool
	memInfo                *system.MemInfo
	supportsClusterFeature bool                // Mock value for SupportsClusterFeature()
	clusterInfo            *system.ClusterInfo // Mock cluster topology information
}

// NewMockSystem creates a new mock system
func NewMockSystem() *MockSystem {
	return &MockSystem{
		packages:   make(map[system.ID]*MockCPUPackage),
		nodes:      make(map[system.ID]*MockNode),
		cpus:       make(map[system.ID]*MockCPU),
		gpus:       make(map[system.ID]*MockGPU),
		clusters:   make(map[system.ID]system.Cluster),
		distances:  make(map[system.ID]map[system.ID]int),
		allowedSet: cpuset.CPUSet{},
		memInfo: &system.MemInfo{
			MemTotal: 16 * 1024 * 1024, // 16GB in KB
			MemFree:  8 * 1024 * 1024,  // 8GB in KB
			MemUsed:  8 * 1024 * 1024,  // 8GB in KB
			MemSet:   cpuset.New(0).Union(cpuset.New(1)),
		},
	}
}

// SetupSingleSocketTopology sets up a simple single-socket, single-NUMA topology
func (m *MockSystem) SetupSingleSocketTopology() {
	m.setupSingleSocketPackage()
	m.setupSingleSocketNode()
	m.setupSingleSocketCPUs()
	m.setupSingleSocketDistances()
}

// setupSingleSocketPackage creates package for single-socket topology
func (m *MockSystem) setupSingleSocketPackage() {
	pkgSet, _ := cpuset.Parse("0-7")
	pkg := &MockCPUPackage{
		id:      0,
		cpuSet:  pkgSet,
		nodeIDs: []system.ID{0},
		dieIDs:  []system.ID{},
	}
	m.packages[0] = pkg
	m.packageIDs = []system.ID{0}
}

// setupSingleSocketNode creates NUMA node for single-socket topology
func (m *MockSystem) setupSingleSocketNode() {
	nodeSet, _ := cpuset.Parse("0-7")
	node := &MockNode{
		id:         0,
		packageID:  0,
		dieID:      -1,
		cpuSet:     nodeSet,
		distance:   []int{10},
		hasMemory:  true,
		memoryType: system.MemoryTypeDRAM,
		memInfo: &system.MemInfo{
			MemTotal: 16 * 1024 * 1024,
			MemFree:  8 * 1024 * 1024,
			MemUsed:  8 * 1024 * 1024,
			MemSet:   cpuset.New(0),
		},
	}
	m.nodes[0] = node
	m.nodeIDs = []system.ID{0}
}

// setupSingleSocketCPUs creates CPUs for single-socket topology
func (m *MockSystem) setupSingleSocketCPUs() {
	for i := 0; i < 8; i++ {
		cpu := &MockCPU{
			id:        system.ID(i),
			packageID: 0,
			dieID:     -1,
			nodeID:    0,
			coreID:    system.ID(i / 2),
			isolated:  false,
			online:    true,
		}
		m.cpus[system.ID(i)] = cpu
		m.cpuIDs = append(m.cpuIDs, system.ID(i))
	}
}

// setupSingleSocketDistances sets up NUMA distances for single-socket topology
func (m *MockSystem) setupSingleSocketDistances() {
	m.distances[0] = map[system.ID]int{0: 10}
}

// SetupDualSocketTopology sets up a dual-socket topology
func (m *MockSystem) SetupDualSocketTopology() {
	m.setupDualSocketPackages()
	m.setupDualSocketNodes()
	m.setupDualSocketCPUs()
	m.setupDualSocketDistances()
	m.allowedSet, _ = cpuset.Parse("0-15")
}

// setupDualSocketPackages creates packages for dual-socket topology
func (m *MockSystem) setupDualSocketPackages() {
	// Package 0
	pkgSetA, _ := cpuset.Parse("0-7")
	pkg0 := &MockCPUPackage{
		id:      0,
		cpuSet:  pkgSetA,
		nodeIDs: []system.ID{0},
		dieIDs:  []system.ID{},
	}
	m.packages[0] = pkg0

	// Package 1
	pkgSetB, _ := cpuset.Parse("8-15")
	pkg1 := &MockCPUPackage{
		id:      1,
		cpuSet:  pkgSetB,
		nodeIDs: []system.ID{1},
		dieIDs:  []system.ID{},
	}
	m.packages[1] = pkg1
	m.packageIDs = []system.ID{0, 1}
}

// setupDualSocketNodes creates NUMA nodes for dual-socket topology
func (m *MockSystem) setupDualSocketNodes() {
	// NUMA Node 0
	nodeSetA, _ := cpuset.Parse("0-7")
	node0 := &MockNode{
		id:         0,
		packageID:  0,
		dieID:      -1,
		cpuSet:     nodeSetA,
		distance:   []int{10, 20},
		hasMemory:  true,
		memoryType: system.MemoryTypeDRAM,
		memInfo: &system.MemInfo{
			MemTotal: 8 * 1024 * 1024,
			MemFree:  4 * 1024 * 1024,
			MemUsed:  4 * 1024 * 1024,
			MemSet:   cpuset.New(0),
		},
	}
	m.nodes[0] = node0

	// NUMA Node 1
	nodeSetB, _ := cpuset.Parse("8-15")
	node1 := &MockNode{
		id:         1,
		packageID:  1,
		dieID:      -1,
		cpuSet:     nodeSetB,
		distance:   []int{20, 10},
		hasMemory:  true,
		memoryType: system.MemoryTypeDRAM,
		memInfo: &system.MemInfo{
			MemTotal: 8 * 1024 * 1024,
			MemFree:  4 * 1024 * 1024,
			MemUsed:  4 * 1024 * 1024,
			MemSet:   cpuset.New(1),
		},
	}
	m.nodes[1] = node1
	m.nodeIDs = []system.ID{0, 1}
}

// setupDualSocketCPUs creates CPUs for dual-socket topology
func (m *MockSystem) setupDualSocketCPUs() {
	for i := 0; i < 16; i++ {
		packageID := system.ID(0)
		nodeID := system.ID(0)
		if i >= 8 {
			packageID = 1
			nodeID = 1
		}
		cpu := &MockCPU{
			id:        system.ID(i),
			packageID: packageID,
			dieID:     -1,
			nodeID:    nodeID,
			coreID:    system.ID(i % 8 / 2),
			isolated:  false,
			online:    true,
		}
		m.cpus[system.ID(i)] = cpu
		m.cpuIDs = append(m.cpuIDs, system.ID(i))
	}
}

// setupDualSocketDistances sets up NUMA distances for dual-socket topology
func (m *MockSystem) setupDualSocketDistances() {
	m.distances[0] = map[system.ID]int{0: 10, 1: 20}
	m.distances[1] = map[system.ID]int{0: 20, 1: 10}
}

// SetupLargeTopology sets up a large topology: 2 Sockets, 4 NUMA nodes, 96 CPUs
// Socket 0: NUMA 0 (0-23), NUMA 1 (24-47)
// Socket 1: NUMA 2 (48-71), NUMA 3 (72-95)
func (m *MockSystem) SetupLargeTopology() {
	m.setupLargeTopologyPackages()
	m.setupLargeTopologyNodes()
	m.setupLargeTopologyCPUs()
	m.setupLargeTopologyDistances()
	m.allowedSet, _ = cpuset.Parse("0-95")
}

// SetupTopology sets up a topology based on the provided TopologyConfig
// This is the unified entry point for setting up any topology configuration
// Returns a TopologyValidator that can be used to validate CPU allocations
func (m *MockSystem) SetupTopology(config TopologyConfig) *TopologyValidator {
	if err := config.Validate(); err != nil {
		panic(fmt.Sprintf("Invalid topology config: %v", err))
	}

	// Setup packages (sockets)
	m.setupGenericPackages(config)

	// Setup NUMA nodes
	m.setupGenericNodes(config)

	// Setup CPUs
	m.setupGenericCPUs(config)

	// Setup NUMA distances
	m.setupGenericDistances(config)

	// Set allowed CPU set
	m.allowedSet, _ = cpuset.Parse(config.GetSystemRange())

	// Set supports cluster feature flag
	m.supportsClusterFeature = config.SupportsClusterFeature

	// Setup cluster info if applicable
	if config.ClusterCount > 0 {
		m.clusterInfo = m.createGenericClusterInfo(config)
	}

	return NewTopologyValidator(config)
}

// setupGenericPackages creates packages based on TopologyConfig
func (m *MockSystem) setupGenericPackages(config TopologyConfig) {
	for socketID := 0; socketID < config.SocketCount; socketID++ {
		cpuStart := socketID * config.CPUsPerSocket
		cpuEnd := cpuStart + config.CPUsPerSocket - 1
		pkgSet, _ := cpuset.Parse(fmt.Sprintf("%d-%d", cpuStart, cpuEnd))

		// Calculate NUMA nodes for this socket
		nodeIDs := make([]system.ID, config.NUMAPerSocket)
		for i := 0; i < config.NUMAPerSocket; i++ {
			nodeIDs[i] = system.ID(socketID*config.NUMAPerSocket + i)
		}

		pkg := &MockCPUPackage{
			id:      system.ID(socketID),
			cpuSet:  pkgSet,
			nodeIDs: nodeIDs,
			dieIDs:  []system.ID{},
		}
		m.packages[system.ID(socketID)] = pkg
		m.packageIDs = append(m.packageIDs, system.ID(socketID))
	}
}

// setupGenericNodes creates NUMA nodes based on TopologyConfig
func (m *MockSystem) setupGenericNodes(config TopologyConfig) {
	for numaID := 0; numaID < config.NUMACount; numaID++ {
		socketID := numaID / config.NUMAPerSocket
		cpuStart := numaID * config.CPUsPerNUMA
		cpuEnd := cpuStart + config.CPUsPerNUMA - 1

		nodeSet, _ := cpuset.Parse(fmt.Sprintf("%d-%d", cpuStart, cpuEnd))

		// Generate distance array (simple model: 10 for local, 12 for same socket, 20-22 for remote)
		distances := m.generateDistances(numaID, config)

		node := &MockNode{
			id:         system.ID(numaID),
			packageID:  system.ID(socketID),
			dieID:      -1,
			cpuSet:     nodeSet,
			distance:   distances,
			hasMemory:  true,
			memoryType: system.MemoryTypeDRAM,
			memInfo: &system.MemInfo{
				MemTotal: 64 * 1024 * 1024, // 64GB per NUMA
				MemFree:  32 * 1024 * 1024,
				MemUsed:  32 * 1024 * 1024,
				MemSet:   cpuset.New(numaID),
			},
		}
		m.nodes[system.ID(numaID)] = node
		m.nodeIDs = append(m.nodeIDs, system.ID(numaID))
	}
}

// generateDistances generates NUMA distance array for a node
func (m *MockSystem) generateDistances(numaID int, config TopologyConfig) []int {
	distances := make([]int, config.NUMACount)
	mySocket := numaID / config.NUMAPerSocket

	for otherNUMA := 0; otherNUMA < config.NUMACount; otherNUMA++ {
		otherSocket := otherNUMA / config.NUMAPerSocket
		if otherNUMA == numaID {
			distances[otherNUMA] = 10 // Local
		} else if mySocket == otherSocket {
			distances[otherNUMA] = 12 // Same socket
		} else {
			// Different socket: 20 + offset based on position
			offset := (numaID % config.NUMAPerSocket) + (otherNUMA % config.NUMAPerSocket)
			distances[otherNUMA] = 20 + (offset % 3)
		}
	}
	return distances
}

// setupGenericCPUs creates CPUs based on TopologyConfig
func (m *MockSystem) setupGenericCPUs(config TopologyConfig) {
	for cpuID := 0; cpuID < config.TotalCPUs; cpuID++ {
		numaID := cpuID / config.CPUsPerNUMA
		socketID := numaID / config.NUMAPerSocket

		var coreID system.ID
		if config.SMTEnabled {
			coreID = system.ID(cpuID / 2) // SMT: 2 threads per core
		} else {
			coreID = system.ID(cpuID) // No SMT: 1 thread per core
		}

		cpu := &MockCPU{
			id:        system.ID(cpuID),
			packageID: system.ID(socketID),
			dieID:     -1,
			nodeID:    system.ID(numaID),
			coreID:    coreID,
			isolated:  false,
			online:    true,
		}
		m.cpus[system.ID(cpuID)] = cpu
		m.cpuIDs = append(m.cpuIDs, system.ID(cpuID))
	}
}

// setupGenericDistances sets up NUMA distance matrix based on TopologyConfig
func (m *MockSystem) setupGenericDistances(config TopologyConfig) {
	for numaID := 0; numaID < config.NUMACount; numaID++ {
		distances := m.generateDistances(numaID, config)
		m.distances[system.ID(numaID)] = make(map[system.ID]int)
		for otherNUMA := 0; otherNUMA < config.NUMACount; otherNUMA++ {
			m.distances[system.ID(numaID)][system.ID(otherNUMA)] = distances[otherNUMA]
		}
	}
}

// createGenericClusterInfo creates ClusterInfo based on TopologyConfig
func (m *MockSystem) createGenericClusterInfo(config TopologyConfig) *system.ClusterInfo {
	clusterInfo := system.NewClusterInfo()
	m.clusters = make(map[system.ID]system.Cluster)
	m.clusterIDs = make([]system.ID, 0, config.ClusterCount)

	for clusterID := 0; clusterID < config.ClusterCount; clusterID++ {
		// Calculate which NUMA this cluster belongs to
		numaID := clusterID / config.ClustersPerNUMA

		// Calculate CPU range for this cluster
		cpuStart := clusterID * config.CPUsPerCluster
		cpuEnd := cpuStart + config.CPUsPerCluster - 1

		cpuRange := fmt.Sprintf("%d-%d", cpuStart, cpuEnd)
		cpuSet, _ := cpuset.Parse(cpuRange)

		cluster := &mockCluster{
			id:     system.ID(clusterID),
			nodeID: system.ID(numaID),
			cpus:   cpuSet,
		}

		// Add to clusterInfo
		clusterInfo.Clusters[system.ID(clusterID)] = cluster

		// Add to clusters map
		m.clusters[system.ID(clusterID)] = cluster
		m.clusterIDs = append(m.clusterIDs, system.ID(clusterID))

		clusterInfo.NodeClusters[system.ID(numaID)] = append(
			clusterInfo.NodeClusters[system.ID(numaID)],
			system.ID(clusterID),
		)

		for _, cpuID := range cpuSet.List() {
			clusterInfo.CPUCluster[system.ID(cpuID)] = system.ID(clusterID)
		}
	}

	return clusterInfo
}

// setupLargeTopologyPackages creates packages for large topology
func (m *MockSystem) setupLargeTopologyPackages() {
	// Package 0 (Socket 0)
	pkgSet0, _ := cpuset.Parse("0-47")
	pkg0 := &MockCPUPackage{
		id:      0,
		cpuSet:  pkgSet0,
		nodeIDs: []system.ID{0, 1},
		dieIDs:  []system.ID{},
	}
	m.packages[0] = pkg0

	// Package 1 (Socket 1)
	pkgSet1, _ := cpuset.Parse("48-95")
	pkg1 := &MockCPUPackage{
		id:      1,
		cpuSet:  pkgSet1,
		nodeIDs: []system.ID{2, 3},
		dieIDs:  []system.ID{},
	}
	m.packages[1] = pkg1
	m.packageIDs = []system.ID{0, 1}
}

// setupLargeTopologyNodes creates NUMA nodes for large topology
func (m *MockSystem) setupLargeTopologyNodes() {
	nodeConfigs := []struct {
		id        system.ID
		packageID system.ID
		cpuRange  string
		distances []int
	}{
		{0, 0, "0-23", []int{10, 12, 20, 22}},  // NUMA 0 (Socket 0)
		{1, 0, "24-47", []int{12, 10, 22, 20}}, // NUMA 1 (Socket 0)
		{2, 1, "48-71", []int{20, 22, 10, 12}}, // NUMA 2 (Socket 1)
		{3, 1, "72-95", []int{22, 20, 12, 10}}, // NUMA 3 (Socket 1)
	}

	for _, config := range nodeConfigs {
		m.createLargeTopologyNode(config.id, config.packageID, config.cpuRange, config.distances)
	}
	m.nodeIDs = []system.ID{0, 1, 2, 3}
}

// createLargeTopologyNode creates a single NUMA node for large topology
func (m *MockSystem) createLargeTopologyNode(id, packageID system.ID, cpuRange string, distances []int) {
	nodeSet, _ := cpuset.Parse(cpuRange)
	node := &MockNode{
		id:         id,
		packageID:  packageID,
		dieID:      -1,
		cpuSet:     nodeSet,
		distance:   distances,
		hasMemory:  true,
		memoryType: system.MemoryTypeDRAM,
		memInfo: &system.MemInfo{
			MemTotal: 32 * 1024 * 1024, // 32GB in KB
			MemFree:  16 * 1024 * 1024, // 16GB in KB
			MemUsed:  16 * 1024 * 1024, // 16GB in KB
			MemSet:   cpuset.New(int(id)),
		},
	}
	m.nodes[id] = node
}

// setupLargeTopologyCPUs creates CPUs for large topology
func (m *MockSystem) setupLargeTopologyCPUs() {
	for i := 0; i < 96; i++ {
		packageID, nodeID := m.calculateCPUPlacement(i)
		cpu := &MockCPU{
			id:        system.ID(i),
			packageID: packageID,
			dieID:     -1,
			nodeID:    nodeID,
			coreID:    system.ID(i % 24 / 2), // 12 cores per NUMA
			isolated:  false,
			online:    true,
		}
		m.cpus[system.ID(i)] = cpu
		m.cpuIDs = append(m.cpuIDs, system.ID(i))
	}
}

// calculateCPUPlacement determines package and node IDs for a CPU
func (m *MockSystem) calculateCPUPlacement(cpuID int) (packageID, nodeID system.ID) {
	switch {
	case cpuID < 24:
		return 0, 0 // Socket 0, NUMA 0
	case cpuID < 48:
		return 0, 1 // Socket 0, NUMA 1
	case cpuID < 72:
		return 1, 2 // Socket 1, NUMA 2
	default:
		return 1, 3 // Socket 1, NUMA 3
	}
}

// setupLargeTopologyDistances sets up NUMA distances for large topology
func (m *MockSystem) setupLargeTopologyDistances() {
	distanceMatrix := map[system.ID]map[system.ID]int{
		0: {0: 10, 1: 12, 2: 20, 3: 22},
		1: {0: 12, 1: 10, 2: 22, 3: 20},
		2: {0: 20, 1: 22, 2: 10, 3: 12},
		3: {0: 22, 1: 20, 2: 12, 3: 10},
	}
	m.distances = distanceMatrix
}

// SetupInvalidTopology sets up an invalid topology that will fail validation
func (m *MockSystem) SetupInvalidTopology() {
	m.shouldFail = true
}

// System interface implementation
func (m *MockSystem) Discover() error {
	if m.shouldFail {
		return fmt.Errorf("mock discovery failure")
	}
	return nil
}

func (m *MockSystem) AllowedSet() cpuset.CPUSet {
	return m.allowedSet
}

func (m *MockSystem) ValidateTopology() error {
	if m.shouldFail {
		return fmt.Errorf("mock topology validation failure")
	}
	return nil
}

func (m *MockSystem) PackageIDs() []system.ID {
	return m.packageIDs
}

func (m *MockSystem) NodeIDs() []system.ID {
	return m.nodeIDs
}

func (m *MockSystem) ClusterIDs() []system.ID {
	return m.clusterIDs
}

func (m *MockSystem) Package(id system.ID) system.CPUPackage {
	if pkg, exists := m.packages[id]; exists {
		return pkg
	}
	return nil
}

func (m *MockSystem) Node(id system.ID) system.Node {
	if node, exists := m.nodes[id]; exists {
		return node
	}
	return nil
}

func (m *MockSystem) Cluster(id system.ID) system.Cluster {
	if cluster, exists := m.clusters[id]; exists {
		return cluster
	}
	return nil
}

func (m *MockSystem) NodeDistance(from, to system.ID) int {
	if distances, exists := m.distances[from]; exists {
		if distance, exists := distances[to]; exists {
			return distance
		}
	}
	return 255 // Max distance for unknown nodes
}

func (m *MockSystem) GPUIDs() []system.ID {
	return m.gpuIDs
}

func (m *MockSystem) GPU(id system.ID) system.GPU {
	if gpu, exists := m.gpus[id]; exists {
		return gpu
	}
	return nil
}

func (m *MockSystem) NodeGPUs(nodeID system.ID) []system.ID {
	var gpus []system.ID
	for _, gpu := range m.gpus {
		if gpu.NodeID() == nodeID {
			gpus = append(gpus, gpu.ID())
		}
	}
	return gpus
}

func (m *MockSystem) NodeClusters(nodeID system.ID) []system.ID {
	// Use clusterInfo if available for backward compatibility
	if m.clusterInfo != nil {
		return m.clusterInfo.GetClustersForNode(nodeID)
	}

	// Fallback: build from clusters map
	var clusterIDs []system.ID
	for _, cluster := range m.clusters {
		if cluster.NodeID() == nodeID {
			clusterIDs = append(clusterIDs, cluster.ID())
		}
	}
	return clusterIDs
}

func (m *MockSystem) MemoryInfo() (*system.MemInfo, error) {
	if m.shouldFail {
		return nil, fmt.Errorf("mock memory info failure")
	}
	return m.memInfo, nil
}

// SupportsClusterFeature returns the mock value for cluster feature support.
// This allows tests to simulate both machines that support and don't support cluster topology.
func (m *MockSystem) SupportsClusterFeature() bool {
	return m.supportsClusterFeature
}

// SetSupportsClusterFeature sets the mock value for cluster feature support.
// Use this in tests to simulate machines with or without cluster topology support.
func (m *MockSystem) SetSupportsClusterFeature(supports bool) {
	m.supportsClusterFeature = supports
}

// ClusterInfo returns the mock cluster topology information.
// Returns nil if no cluster info has been set.
func (m *MockSystem) ClusterInfo() *system.ClusterInfo {
	return m.clusterInfo
}

// SetClusterInfo sets the mock cluster topology information.
// Use this in tests to simulate cluster topology for 950 machines.
func (m *MockSystem) SetClusterInfo(clusterInfo *system.ClusterInfo) {
	m.clusterInfo = clusterInfo
}

// MockCPUPackage implements system.CPUPackage interface for testing
type MockCPUPackage struct {
	id      system.ID
	cpuSet  cpuset.CPUSet
	nodeIDs []system.ID
	dieIDs  []system.ID
	dies    map[system.ID]*MockDie
}

func (p *MockCPUPackage) ID() system.ID {
	return p.id
}

func (p *MockCPUPackage) CPUSet() cpuset.CPUSet {
	return p.cpuSet
}

func (p *MockCPUPackage) DieIDs() []system.ID {
	return p.dieIDs
}

func (p *MockCPUPackage) NodeIDs() []system.ID {
	return p.nodeIDs
}

func (p *MockCPUPackage) AddCPU(id system.ID) {
	// Mock implementation - no-op for testing
}

func (p *MockCPUPackage) AddNode(id system.ID) {
	p.nodeIDs = append(p.nodeIDs, id)
}

func (p *MockCPUPackage) AddDie(id system.ID) {
	p.dieIDs = append(p.dieIDs, id)
}

func (p *MockCPUPackage) AddCPUToDie(dieID system.ID, cpuID system.ID) {
	// Mock implementation - no-op for testing
}

func (p *MockCPUPackage) AddNodeToDie(dieID system.ID, nodeID system.ID) {
	// Mock implementation - no-op for testing
}

func (p *MockCPUPackage) DieCPUSet(id system.ID) cpuset.CPUSet {
	if die, exists := p.dies[id]; exists {
		return die.cpuSet
	}
	return cpuset.CPUSet{}
}

func (p *MockCPUPackage) DieNodeIDs(id system.ID) []system.ID {
	if die, exists := p.dies[id]; exists {
		return die.nodeIDs
	}
	return []system.ID{}
}

func (p *MockCPUPackage) GetSortedNodes() []system.ID {
	return p.nodeIDs
}

func (p *MockCPUPackage) MemoryInfo() (*system.MemInfo, error) {
	memSet, _ := cpuset.Parse("0-1")
	return &system.MemInfo{
		MemTotal: 16 * 1024 * 1024,
		MemFree:  8 * 1024 * 1024,
		MemUsed:  8 * 1024 * 1024,
		MemSet:   memSet,
	}, nil
}

// MockDie represents a die within a package
type MockDie struct {
	id      system.ID
	cpuSet  cpuset.CPUSet
	nodeIDs []system.ID
}

// MockNode implements system.Node interface for testing
type MockNode struct {
	id         system.ID
	packageID  system.ID
	dieID      system.ID
	cpuSet     cpuset.CPUSet
	distance   []int
	hasMemory  bool
	normalMem  bool
	memoryType system.MemoryType
	memInfo    *system.MemInfo
}

func (n *MockNode) ID() system.ID {
	return n.id
}

func (n *MockNode) PackageID() system.ID {
	return n.packageID
}

func (n *MockNode) DieID() system.ID {
	return n.dieID
}

func (n *MockNode) CPUSet() cpuset.CPUSet {
	return n.cpuSet
}

func (n *MockNode) Distance() []int {
	return n.distance
}

func (n *MockNode) DistanceFrom(id system.ID) int {
	if int(id) < len(n.distance) {
		return n.distance[id]
	}
	return 255
}

func (n *MockNode) SetPackageID(id system.ID) {
	n.packageID = id
}

func (n *MockNode) SetDieID(id system.ID) {
	n.dieID = id
}

func (n *MockNode) MemoryInfo() (*system.MemInfo, error) {
	return n.memInfo, nil
}

func (n *MockNode) HasMemory() bool {
	return n.hasMemory
}

func (n *MockNode) HasNormalMemory() bool {
	return n.normalMem
}

func (n *MockNode) GetMemoryType() system.MemoryType {
	return n.memoryType
}

func (n *MockNode) SetMemoryType(memType system.MemoryType) {
	n.memoryType = memType
}

func (n *MockNode) SetNormalMemory(normal bool) {
	n.normalMem = normal
}

func (n *MockNode) SetMemory(hasMemory bool) {
	n.hasMemory = hasMemory
}

// MockCPU implements system.CPU interface for testing
type MockCPU struct {
	id        system.ID
	packageID system.ID
	dieID     system.ID
	nodeID    system.ID
	coreID    system.ID
	isolated  bool
	online    bool
}

func (c *MockCPU) ID() system.ID {
	return c.id
}

func (c *MockCPU) PackageID() system.ID {
	return c.packageID
}

func (c *MockCPU) DieID() system.ID {
	return c.dieID
}

func (c *MockCPU) NodeID() system.ID {
	return c.nodeID
}

func (c *MockCPU) CoreID() system.ID {
	return c.coreID
}

func (c *MockCPU) ThreadCPUSet() cpuset.CPUSet {
	cpuSet, _ := cpuset.Parse(fmt.Sprintf("%d", c.id))
	return cpuSet
}

func (c *MockCPU) Isolated() bool {
	return c.isolated
}

func (c *MockCPU) Online() bool {
	return c.online
}

// MockGPU implements system.GPU interface for testing
type MockGPU struct {
	id          system.ID
	nodeID      system.ID
	pciAddress  string
	vendorID    string
	deviceID    string
	deviceClass string
}

func (g *MockGPU) ID() system.ID {
	return g.id
}

func (g *MockGPU) NodeID() system.ID {
	return g.nodeID
}

func (g *MockGPU) PCIAddress() string {
	return g.pciAddress
}

func (g *MockGPU) VendorID() string {
	return g.vendorID
}

func (g *MockGPU) DeviceID() string {
	return g.deviceID
}

func (g *MockGPU) DeviceClass() string {
	return g.deviceClass
}

// mockCluster implements system.Cluster interface for testing
type mockCluster struct {
	id     system.ID
	nodeID system.ID
	cpus   cpuset.CPUSet
}

func (c *mockCluster) ID() system.ID {
	return c.id
}

func (c *mockCluster) NodeID() system.ID {
	return c.nodeID
}

func (c *mockCluster) CPUSet() cpuset.CPUSet {
	return c.cpus
}

func (c *mockCluster) Size() int {
	return c.cpus.Size()
}

func (c *mockCluster) SetNodeID(nodeID system.ID) {
	c.nodeID = nodeID
}
