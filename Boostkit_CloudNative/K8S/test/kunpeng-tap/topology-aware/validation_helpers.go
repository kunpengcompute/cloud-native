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

// ==================== Basic NUMA Validation Functions ====================
// For the basic small topology (8 CPUs, single NUMA)

// isValidNUMARange checks if the CPU set belongs to one of the expected NUMA ranges
// This is for the basic small topology (8 CPUs, single NUMA)
func isValidNUMARange(cpuSet string) bool {
	expectedRanges := []string{
		"0-7", // Single NUMA node (8 CPUs)
	}

	for _, expectedRange := range expectedRanges {
		if cpuSet == expectedRange {
			return true
		}
	}
	return false
}

// ==================== Large Topology Validation Functions ====================
// 大型拓扑规格 (2 Socket, 96 CPUs):
// - 2 Sockets, 4 NUMA nodes, 24 CPUs/NUMA
// - Socket 0: NUMA 0 (0-23), NUMA 1 (24-47)
// - Socket 1: NUMA 2 (48-71), NUMA 3 (72-95)

// isValidLargeNUMARange checks if the CPU set belongs to NUMA ranges for large topology
// 96 CPUs total, 4 NUMA nodes, 24 CPUs per NUMA
func isValidLargeNUMARange(cpuSet string) bool {
	expectedRanges := []string{
		"0-23",  // NUMA 0
		"24-47", // NUMA 1
		"48-71", // NUMA 2
		"72-95", // NUMA 3
	}

	for _, expectedRange := range expectedRanges {
		if cpuSet == expectedRange {
			return true
		}
	}
	return false
}

// isValidLargeSocketRange checks if the CPU set belongs to Socket ranges for large topology
// 96 CPUs total, 2 sockets, 48 CPUs per socket
func isValidLargeSocketRange(cpuSet string) bool {
	expectedRanges := []string{
		"0-47",  // Socket 0 (NUMA 0 + NUMA 1)
		"48-95", // Socket 1 (NUMA 2 + NUMA 3)
	}

	for _, expectedRange := range expectedRanges {
		if cpuSet == expectedRange {
			return true
		}
	}
	return false
}

// isValidLargeSystemRange checks if the CPU set covers the entire system for large topology
// 96 CPUs total
func isValidLargeSystemRange(cpuSet string) bool {
	return cpuSet == "0-95" // 96 CPUs
}

// getLargeAllocationType determines the allocation type based on CPU set for large topology
// 96 CPUs, 24 CPUs/NUMA, 48 CPUs/Socket
func getLargeAllocationType(cpuSet string) string {
	if isValidLargeNUMARange(cpuSet) {
		return "NUMA"
	} else if isValidLargeSocketRange(cpuSet) {
		return "Socket"
	} else if isValidLargeSystemRange(cpuSet) {
		return "System"
	}
	return "Unknown"
}

// ==================== 950 Model Validation Functions ====================
// 950 机型规格 (2 Socket):
// - SMT 关闭: 192 CPUs (96 CPUs/Socket, 48 CPUs/NUMA, 16 CPUs/Cluster)
// - SMT 开启: 384 CPUs (192 CPUs/Socket, 96 CPUs/NUMA, 32 CPUs/Cluster)
//
// NOTE: 950 机型的验证函数已迁移到 topology_config.go 中的 TopologyValidator
// 使用 SetupTopology(Config950NoSMT) 或 SetupTopology(Config950SMT) 返回的 validator
// 进行动态验证，避免硬编码的验证函数
