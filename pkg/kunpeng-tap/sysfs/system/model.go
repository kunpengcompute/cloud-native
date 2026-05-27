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
	"bufio"
	"os"
	"strings"

	gopsutilcpu "github.com/shirou/gopsutil/v3/cpu"
	"k8s.io/klog/v2"
)

const (
	// CPU part identifier for 950 model machines
	// 0xd06 is the model code for 950 machines
	cpuModel950 = "0xd06"

	// Path to cpuinfo file
	cpuInfoPath = "/proc/cpuinfo"

	// Future: Add other machine model identifiers here
	// Example: cpuModelXYZ = "0x..."
)

var (
	// Cached result of supportsClusterFeature check
	supportsClusterFeatureCached *bool
)

// SupportsClusterFeature checks if the current machine supports cluster topology feature.
// Currently, this feature is supported by 950 model machines.
// In the future, this can be extended to check for other machine models that support cluster topology.
// The result is cached after the first check.
func SupportsClusterFeature() bool {
	if supportsClusterFeatureCached != nil {
		return *supportsClusterFeatureCached
	}

	// Check if the machine model supports cluster topology
	result := checkCPUInfoFileForCluster(cpuInfoPath)
	supportsClusterFeatureCached = &result
	return result
}

// checkCPUInfoFileForCluster reads /proc/cpuinfo and checks for machines that support cluster topology.
// First tries gopsutil, then falls back to reading /proc/cpuinfo directly.
func checkCPUInfoFileForCluster(path string) bool {
	// First try using gopsutil
	cpuinfo, err := gopsutilcpu.Info()
	if err == nil && len(cpuinfo) > 0 {
		model := cpuinfo[0].Model
		// Currently only 950 model supports cluster topology
		// Future: Add checks for other machine models here
		if model == cpuModel950 {
			klog.V(3).InfoS("Detected machine with cluster support via gopsutil",
				"cpuModel", model)
			return true
		}
		klog.V(4).InfoS("gopsutil returned model without cluster support",
			"cpuModel", model)
	} else {
		klog.V(4).InfoS("Failed to get CPU info via gopsutil, falling back to /proc/cpuinfo",
			"error", err)
	}

	// Fallback: read /proc/cpuinfo directly
	file, err := os.Open(path)
	if err != nil {
		klog.V(3).InfoS("Failed to open cpuinfo file, assuming no cluster support",
			"path", path,
			"error", err)
		return false
	}
	defer file.Close() // nolint: errcheck

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "CPU part") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) < 2 {
				continue
			}
			model := strings.TrimSpace(parts[1])
			// Currently only 950 model supports cluster topology
			// Future: Add checks for other machine models here
			if model == cpuModel950 {
				klog.V(3).InfoS("Detected machine with cluster support via /proc/cpuinfo",
					"cpuPart", model)
				return true
			}
		}
	}

	klog.V(3).InfoS("No machine with cluster support detected")
	return false
}

// ResetSupportsClusterFeatureCache resets the cached result for testing purposes.
func ResetSupportsClusterFeatureCache() {
	supportsClusterFeatureCached = nil
}
