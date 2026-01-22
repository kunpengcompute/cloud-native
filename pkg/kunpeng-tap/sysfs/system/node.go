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
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"k8s.io/klog/v2"
	"k8s.io/utils/cpuset"
)

var _ Node = &node{}

// MemoryType is an enum for the Node memory
type MemoryType int

const (
	// MemoryTypeDRAM means that the node has regular DRAM-type memory
	MemoryTypeDRAM MemoryType = iota
)

type node struct {
	path       string        // sysfs path
	id         ID            // node id
	pkg        ID            // package id
	die        ID            // die id
	cpus       cpuset.CPUSet // cpus in this node
	distance   []int         // distance/cost to other NUMA nodes
	hasMemory  bool          // node has memory
	normalMem  bool          // node has normal memory
	memoryType MemoryType    // type of memory in this node
}

// Node methods
func (n *node) ID() ID {
	return n.id
}

func (n *node) PackageID() ID {
	return n.pkg
}

func (n *node) DieID() ID {
	return n.die
}

func (n *node) CPUSet() cpuset.CPUSet {
	return n.cpus
}

func (n *node) Distance() []int {
	return n.distance
}

func (n *node) DistanceFrom(id ID) int {
	if id >= 0 && id < ID(len(n.distance)) {
		return n.distance[id]
	}
	return -1
}

func (n *node) SetPackageID(id ID) {
	n.pkg = id
}

func (n *node) SetDieID(id ID) {
	n.die = id
}

// MemoryInfo returns memory statistics for the NUMA node in kilobytes (KB).
func (n *node) MemoryInfo() (*MemInfo, error) {
	meminfo := filepath.Join(n.path, "meminfo")

	// Read the meminfo file
	data, err := os.ReadFile(meminfo)
	if err != nil {
		return nil, fmt.Errorf("failed to read meminfo file: %v", err)
	}

	// Create a new MemInfo struct to store the results
	buf := &MemInfo{
		MemSet: cpuset.New(n.id), // 设置 MemSet 为当前节点的 ID
	}

	// Parse each line of the meminfo file
	for _, line := range strings.Split(string(data), "\n") {
		if len(strings.TrimSpace(line)) == 0 {
			continue
		}

		// Split the line into fields
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) < 4 {
			continue
		}

		// Extract the key and value
		key := fields[2]
		valueStr := fields[3]

		// Parse the value as uint64
		value, err := strconv.ParseUint(valueStr, 10, 64)
		if err != nil {
			klog.V(4).InfoS("Failed to parse memory value", "key", key, "value", valueStr, "error", err)
			continue
		}

		// Check for unit and convert if necessary to KB
		if len(fields) >= 5 {
			unit := fields[4]
			value = convertValueToKB(value, unit)
		}

		// Store the value in the appropriate field
		switch key {
		case "MemTotal:":
			buf.MemTotal = value
		case "MemFree:":
			buf.MemFree = value
		case "MemUsed:":
			buf.MemUsed = value
		}
	}

	// 如果 MemUsed 没有直接提供，则计算它
	if buf.MemUsed == 0 && buf.MemTotal > 0 && buf.MemFree > 0 {
		buf.MemUsed = buf.MemTotal - buf.MemFree
	}

	return buf, nil
}

// convertValueToKB converts a value based on its unit to kilobytes (KB)
func convertValueToKB(value uint64, unit string) uint64 {
	switch unit {
	case "kB":
		return value // Already in KB
	case "MB":
		return value * 1024 // Convert MB to KB
	case "GB":
		return value * 1024 * 1024 // Convert GB to KB
	case "TB":
		return value * 1024 * 1024 * 1024 // Convert TB to KB
	case "B":
		return value / 1024 // Convert B to KB (integer division)
	default:
		// If unit is not recognized, assume it's already in KB
		return value
	}
}

// Additional methods for the Node interface
func (n *node) HasMemory() bool {
	return n.hasMemory
}

func (n *node) HasNormalMemory() bool {
	return n.normalMem
}

func (n *node) GetMemoryType() MemoryType {
	return n.memoryType
}

func (n *node) SetMemory(has bool) {
	n.hasMemory = has
}

func (n *node) SetNormalMemory(has bool) {
	n.normalMem = has
}

func (n *node) SetMemoryType(memType MemoryType) {
	n.memoryType = memType
}
