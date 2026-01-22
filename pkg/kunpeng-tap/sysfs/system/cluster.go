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
	"sort"
	"strconv"
	"strings"

	"k8s.io/klog/v2"
	"k8s.io/utils/cpuset"
)

// Cluster represents a CPU cluster within a NUMA node.
// On 950 model machines, each NUMA node contains multiple clusters,
// with each cluster typically containing 8 logical CPUs.
type Cluster interface {
	ID() ID
	NodeID() ID
	CPUSet() cpuset.CPUSet
	Size() int
	SetNodeID(nodeID ID)
}

var _ Cluster = &cluster{}

type cluster struct {
	id     ID            // Cluster ID
	nodeID ID            // NUMA node ID this cluster belongs to
	cpus   cpuset.CPUSet // CPUs in this cluster
}

func (c *cluster) ID() ID {
	return c.id
}

func (c *cluster) NodeID() ID {
	return c.nodeID
}

func (c *cluster) CPUSet() cpuset.CPUSet {
	return c.cpus
}

func (c *cluster) Size() int {
	return c.cpus.Size()
}

func (c *cluster) SetNodeID(nodeID ID) {
	c.nodeID = nodeID
}

// ClusterInfo holds cluster topology information for the system.
type ClusterInfo struct {
	// Clusters maps cluster ID to Cluster
	Clusters map[ID]Cluster
	// NodeClusters maps NUMA node ID to list of cluster IDs
	NodeClusters map[ID][]ID
	// CPUCluster maps CPU ID to cluster ID
	CPUCluster map[ID]ID
}

// NewClusterInfo creates a new empty ClusterInfo.
func NewClusterInfo() *ClusterInfo {
	return &ClusterInfo{
		Clusters:     make(map[ID]Cluster),
		NodeClusters: make(map[ID][]ID),
		CPUCluster:   make(map[ID]ID),
	}
}

// DiscoverClusters discovers cluster topology from sysfs.
// It reads /sys/devices/system/cpu/cpu*/topology/cluster_cpus_list to determine
// which CPUs belong to which cluster.
// Returns nil if cluster topology is not available (non-950 model machines).
func DiscoverClusters(sysRoot string) (*ClusterInfo, error) {
	sysPath := filepath.Join("/", sysRoot, "sys")
	cpuPath := filepath.Join(sysPath, sysfsCPUPath)

	entries, err := filepath.Glob(filepath.Join(cpuPath, "cpu[0-9]*"))
	if err != nil {
		return nil, fmt.Errorf("failed to glob CPU entries: %w", err)
	}

	if len(entries) == 0 {
		return nil, fmt.Errorf("no CPU entries found")
	}

	info := NewClusterInfo()
	clusterCPUs := make(map[string]cpuset.CPUSet) // cluster_cpus_list string -> CPUSet
	clusterIDs := make(map[string]ID)             // cluster_cpus_list string -> cluster ID

	nextClusterID := 0

	for _, entry := range entries {
		cpuIDStr := filepath.Base(entry)
		cpuIDStr = strings.TrimPrefix(cpuIDStr, "cpu")
		cpuID, err := strconv.Atoi(cpuIDStr)
		if err != nil {
			continue
		}

		clusterCPUsListPath := filepath.Join(entry, "topology", "cluster_cpus_list")
		data, err := os.ReadFile(clusterCPUsListPath)
		if err != nil {
			// cluster_cpus_list not available, cluster topology not supported
			klog.V(4).InfoS("Cluster topology not available",
				"cpu", cpuID,
				"path", clusterCPUsListPath,
				"error", err)
			return nil, nil
		}

		clusterCPUsListStr := strings.TrimSpace(string(data))
		if clusterCPUsListStr == "" {
			continue
		}

		// Parse the cluster CPUs list
		clusterCPUSet, err := cpuset.Parse(clusterCPUsListStr)
		if err != nil {
			klog.ErrorS(err, "Failed to parse cluster_cpus_list",
				"cpu", cpuID,
				"value", clusterCPUsListStr)
			continue
		}

		// Check if we've seen this cluster before
		if _, exists := clusterIDs[clusterCPUsListStr]; !exists {
			clusterIDs[clusterCPUsListStr] = nextClusterID
			clusterCPUs[clusterCPUsListStr] = clusterCPUSet
			nextClusterID++
		}

		// Map CPU to cluster
		info.CPUCluster[cpuID] = clusterIDs[clusterCPUsListStr]
	}

	if len(clusterIDs) == 0 {
		return nil, nil
	}

	// Create cluster objects
	for clusterCPUsListStr, clusterID := range clusterIDs {
		cpuSet := clusterCPUs[clusterCPUsListStr]
		info.Clusters[clusterID] = &cluster{
			id:     clusterID,
			nodeID: -1, // Will be set later
			cpus:   cpuSet,
		}
	}

	klog.V(3).InfoS("Discovered cluster topology",
		"numClusters", len(info.Clusters))

	return info, nil
}

// AssociateWithNodes associates clusters with their NUMA nodes based on CPU membership.
// This should be called after both cluster and node discovery is complete.
func (ci *ClusterInfo) AssociateWithNodes(cpuNodeMap map[ID]ID) {
	if ci == nil {
		return
	}

	// For each cluster, determine which node it belongs to
	for clusterID, clstr := range ci.Clusters {
		cpuList := clstr.CPUSet().List()
		if len(cpuList) == 0 {
			continue
		}

		// Use the first CPU to determine the node
		// All CPUs in a cluster should belong to the same node
		firstCPU := ID(cpuList[0])
		if nodeID, ok := cpuNodeMap[firstCPU]; ok {
			clstr.SetNodeID(nodeID)

			// Add cluster to node's cluster list
			ci.NodeClusters[nodeID] = append(ci.NodeClusters[nodeID], clusterID)
		}
	}

	// Sort cluster IDs for each node
	for nodeID := range ci.NodeClusters {
		sort.Ints(ci.NodeClusters[nodeID])
	}

	klog.V(3).InfoS("Associated clusters with nodes",
		"nodeClusters", ci.NodeClusters)
}

// GetClusterByID returns the cluster with the given ID.
func (ci *ClusterInfo) GetClusterByID(id ID) Cluster {
	if ci == nil {
		return nil
	}
	return ci.Clusters[id]
}

// GetClusterIDForCPU returns the cluster ID for the given CPU.
func (ci *ClusterInfo) GetClusterIDForCPU(cpuID ID) (ID, bool) {
	if ci == nil {
		return -1, false
	}
	clusterID, ok := ci.CPUCluster[cpuID]
	return clusterID, ok
}

// GetClustersForNode returns the cluster IDs for the given NUMA node.
func (ci *ClusterInfo) GetClustersForNode(nodeID ID) []ID {
	if ci == nil {
		return nil
	}
	return ci.NodeClusters[nodeID]
}

// HasClusters returns true if cluster topology information is available.
func (ci *ClusterInfo) HasClusters() bool {
	return ci != nil && len(ci.Clusters) > 0
}

// GetMaxClusterSize returns the maximum cluster size (number of CPUs) among all clusters.
// Returns 0 if no clusters are available.
func (ci *ClusterInfo) GetMaxClusterSize() int {
	if ci == nil || len(ci.Clusters) == 0 {
		return 0
	}

	maxSize := 0
	for _, c := range ci.Clusters {
		if c.Size() > maxSize {
			maxSize = c.Size()
		}
	}
	return maxSize
}
