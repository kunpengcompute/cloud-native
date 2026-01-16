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

import "fmt"

// TopologyConfig defines the hardware topology configuration for testing
// This allows separation of topology data from the setup process
type TopologyConfig struct {
	Name                   string // Machine model name, e.g., "950-SMT", "950-NoSMT", "Large"
	TotalCPUs              int    // Total number of CPUs
	SocketCount            int    // Number of sockets
	NUMACount              int    // Total number of NUMA nodes
	NUMAPerSocket          int    // Number of NUMA nodes per socket
	CPUsPerNUMA            int    // Number of CPUs per NUMA node
	CPUsPerSocket          int    // Number of CPUs per socket
	ClusterCount           int    // Total number of clusters (0 means no cluster support)
	ClustersPerNUMA        int    // Number of clusters per NUMA node
	CPUsPerCluster         int    // Number of CPUs per cluster
	SMTEnabled             bool   // Whether SMT (Hyper-Threading) is enabled
	SupportsClusterFeature bool   // Whether this machine supports cluster topology feature
}

// Validate checks if the topology configuration is valid
func (c *TopologyConfig) Validate() error {
	if c.TotalCPUs <= 0 {
		return fmt.Errorf("TotalCPUs must be positive")
	}
	if c.SocketCount <= 0 {
		return fmt.Errorf("SocketCount must be positive")
	}
	if c.NUMACount <= 0 {
		return fmt.Errorf("NUMACount must be positive")
	}
	if c.CPUsPerNUMA <= 0 {
		return fmt.Errorf("CPUsPerNUMA must be positive")
	}
	if c.TotalCPUs != c.NUMACount*c.CPUsPerNUMA {
		return fmt.Errorf("TotalCPUs (%d) != NUMACount (%d) * CPUsPerNUMA (%d)",
			c.TotalCPUs, c.NUMACount, c.CPUsPerNUMA)
	}
	if c.ClusterCount > 0 && c.CPUsPerCluster <= 0 {
		return fmt.Errorf("CPUsPerCluster must be positive when ClusterCount > 0")
	}
	return nil
}

// GetNUMARange returns the CPU range string for a specific NUMA node
func (c *TopologyConfig) GetNUMARange(numaID int) string {
	if numaID < 0 || numaID >= c.NUMACount {
		return ""
	}
	start := numaID * c.CPUsPerNUMA
	end := start + c.CPUsPerNUMA - 1
	return fmt.Sprintf("%d-%d", start, end)
}

// GetSocketRange returns the CPU range string for a specific socket
func (c *TopologyConfig) GetSocketRange(socketID int) string {
	if socketID < 0 || socketID >= c.SocketCount {
		return ""
	}
	start := socketID * c.CPUsPerSocket
	end := start + c.CPUsPerSocket - 1
	return fmt.Sprintf("%d-%d", start, end)
}

// GetClusterRange returns the CPU range string for a specific cluster
func (c *TopologyConfig) GetClusterRange(clusterID int) string {
	if clusterID < 0 || clusterID >= c.ClusterCount || c.CPUsPerCluster <= 0 {
		return ""
	}
	start := clusterID * c.CPUsPerCluster
	end := start + c.CPUsPerCluster - 1
	return fmt.Sprintf("%d-%d", start, end)
}

// GetSystemRange returns the CPU range string for the entire system
func (c *TopologyConfig) GetSystemRange() string {
	return fmt.Sprintf("0-%d", c.TotalCPUs-1)
}

// GetAllNUMARanges returns all NUMA node CPU ranges
func (c *TopologyConfig) GetAllNUMARanges() []string {
	ranges := make([]string, c.NUMACount)
	for i := 0; i < c.NUMACount; i++ {
		ranges[i] = c.GetNUMARange(i)
	}
	return ranges
}

// GetAllSocketRanges returns all socket CPU ranges
func (c *TopologyConfig) GetAllSocketRanges() []string {
	ranges := make([]string, c.SocketCount)
	for i := 0; i < c.SocketCount; i++ {
		ranges[i] = c.GetSocketRange(i)
	}
	return ranges
}

// GetAllClusterRanges returns all cluster CPU ranges
func (c *TopologyConfig) GetAllClusterRanges() []string {
	if c.ClusterCount <= 0 {
		return nil
	}
	ranges := make([]string, c.ClusterCount)
	for i := 0; i < c.ClusterCount; i++ {
		ranges[i] = c.GetClusterRange(i)
	}
	return ranges
}

// ==================== Predefined Topology Configurations ====================

// Config950SMT is the predefined configuration for 950 model with SMT enabled
// 2 Sockets, 4 NUMA nodes, 384 CPUs total
// Each NUMA has 96 CPUs, divided into 3 clusters of 32 CPUs each
var Config950SMT = TopologyConfig{
	Name:                   "950-SMT",
	TotalCPUs:              384,
	SocketCount:            2,
	NUMACount:              4,
	NUMAPerSocket:          2,
	CPUsPerNUMA:            96,
	CPUsPerSocket:          192,
	ClusterCount:           12,
	ClustersPerNUMA:        3,
	CPUsPerCluster:         32,
	SMTEnabled:             true,
	SupportsClusterFeature: true,
}

// Config950NoSMT is the predefined configuration for 950 model with SMT disabled
// 2 Sockets, 4 NUMA nodes, 192 CPUs total
// Each NUMA has 48 CPUs, divided into 3 clusters of 16 CPUs each
var Config950NoSMT = TopologyConfig{
	Name:                   "950-NoSMT",
	TotalCPUs:              192,
	SocketCount:            2,
	NUMACount:              4,
	NUMAPerSocket:          2,
	CPUsPerNUMA:            48,
	CPUsPerSocket:          96,
	ClusterCount:           12,
	ClustersPerNUMA:        3,
	CPUsPerCluster:         16,
	SMTEnabled:             false,
	SupportsClusterFeature: true,
}

// ==================== Topology Validator ====================

// TopologyValidator provides validation methods based on TopologyConfig
// This dynamically generates expected ranges based on the topology configuration
type TopologyValidator struct {
	config TopologyConfig
}

// NewTopologyValidator creates a new TopologyValidator from a TopologyConfig
func NewTopologyValidator(config TopologyConfig) *TopologyValidator {
	return &TopologyValidator{config: config}
}

// Config returns the underlying TopologyConfig
func (v *TopologyValidator) Config() TopologyConfig {
	return v.config
}

// IsValidClusterRange checks if the CPU set belongs to a valid cluster range
func (v *TopologyValidator) IsValidClusterRange(cpuSet string) bool {
	if v.config.ClusterCount <= 0 {
		return false
	}
	for _, expectedRange := range v.config.GetAllClusterRanges() {
		if cpuSet == expectedRange {
			return true
		}
	}
	return false
}

// IsValidNUMARange checks if the CPU set belongs to a valid NUMA range
func (v *TopologyValidator) IsValidNUMARange(cpuSet string) bool {
	for _, expectedRange := range v.config.GetAllNUMARanges() {
		if cpuSet == expectedRange {
			return true
		}
	}
	return false
}

// IsValidSocketRange checks if the CPU set belongs to a valid socket range
func (v *TopologyValidator) IsValidSocketRange(cpuSet string) bool {
	for _, expectedRange := range v.config.GetAllSocketRanges() {
		if cpuSet == expectedRange {
			return true
		}
	}
	return false
}

// IsValidSystemRange checks if the CPU set covers the entire system
func (v *TopologyValidator) IsValidSystemRange(cpuSet string) bool {
	return cpuSet == v.config.GetSystemRange()
}

// GetAllocationType determines the allocation type based on the CPU set
func (v *TopologyValidator) GetAllocationType(cpuSet string) string {
	if v.config.ClusterCount > 0 && v.IsValidClusterRange(cpuSet) {
		return "Cluster"
	} else if v.IsValidNUMARange(cpuSet) {
		return "NUMA"
	} else if v.IsValidSocketRange(cpuSet) {
		return "Socket"
	} else if v.IsValidSystemRange(cpuSet) {
		return "System"
	}
	return "Unknown"
}
