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
	"kunpeng.huawei.com/kunpeng-cloud-computing/pkg/kunpeng-tap/sysfs/system"
)

// createSocketNodes creates socket nodes and returns a map of socket ID to Node.
func (p *TopologyAwarePolicy) createSocketNodes() map[system.ID]Node {
	sockets := make(map[system.ID]Node)

	for _, socketID := range p.sys.PackageIDs() {
		var socket Node

		if p.root != nil {
			socket = NewSocketNode(p, socketID, p.root, p.sys.Package(socketID))
			klog.V(5).InfoS("Created pool for ", "name", p.root.Name()+"/"+socket.Name())
		} else {
			// Single socket systems use socket node as root directly.
			socket = NewSocketNode(p, socketID, nilNode, p.sys.Package(socketID))
			p.root = socket
			klog.V(5).InfoS("Created single socket pool for ", "name", socket.Name())
		}

		p.nodes[socket.Name()] = socket
		sockets[socketID] = socket
	}

	return sockets
}

// createNumaNodes creates NUMA nodes and returns a map of NUMA ID to Node.
func (p *TopologyAwarePolicy) createNumaNodes(sockets map[system.ID]Node) map[system.ID]Node {
	numaNodes := make(map[system.ID]Node)

	for _, numaNodeID := range p.sys.NodeIDs() {
		parent := sockets[p.sys.Node(numaNodeID).PackageID()]

		numaNode := NewNumaNode(p, numaNodeID, parent)
		p.nodes[numaNode.Name()] = numaNode
		numaNodes[numaNodeID] = numaNode

		klog.V(5).InfoS("Created NUMA node",
			"numaNodeID", numaNodeID,
			"parent", parent.Name())
	}

	return numaNodes
}

// createClusterNodes creates cluster nodes under NUMA nodes when cluster affinity is enabled.
func (p *TopologyAwarePolicy) createClusterNodes(numaNodes map[system.ID]Node) {
	if !p.enableClusterAffinity {
		return
	}

	clusterIDs := p.sys.ClusterIDs()
	if len(clusterIDs) == 0 {
		klog.V(4).InfoS("Cluster affinity enabled but no clusters available")
		return
	}

	for numaNodeID, numaNode := range numaNodes {
		nodeClusterIDs := p.sys.NodeClusters(numaNodeID)
		if len(nodeClusterIDs) == 0 {
			continue
		}

		for _, clusterID := range nodeClusterIDs {
			sysCluster := p.sys.Cluster(clusterID)
			if sysCluster == nil {
				klog.Warningf("Cluster %d not found in system", clusterID)
				continue
			}

			clusterNode := NewClusterNode(p, sysCluster, numaNode)
			p.nodes[clusterNode.Name()] = clusterNode

			klog.V(5).InfoS("Created cluster node",
				"clusterID", clusterID,
				"numaNodeID", numaNodeID,
				"cpus", sysCluster.CPUSet().String(),
				"parent", numaNode.Name())
		}
	}

	klog.InfoS("Cluster nodes created",
		"totalClusters", len(clusterIDs))
}

// validateSocketStructure checks if socket structure is valid.
func (p *TopologyAwarePolicy) validateSocketStructure() bool {
	if len(p.sys.PackageIDs()) == 0 {
		klog.Warning("No valid socket (package) found in system")
		return false
	}

	for _, socketID := range p.sys.PackageIDs() {
		if socketID < 0 {
			klog.Warningf("Invalid socket ID detected: socketID=%d", socketID)
			return false
		}
		pkg := p.sys.Package(socketID)
		if len(pkg.NodeIDs()) == 0 {
			klog.Warningf("Socket has no associated NUMA nodes: socketID=%d", socketID)
			return false
		}
	}
	return true
}

// buildResourcePoolsByTopology builds a hierarchical tree of pools based on HW topology.
func (p *TopologyAwarePolicy) buildResourcePoolsByTopology() error {
	if err := p.checkHWTopology(); err != nil {
		return err
	}
	klog.Info("Building pools by topology")

	p.nodes = make(map[string]Node)

	hasValidSocketStructure := p.validateSocketStructure()
	if !hasValidSocketStructure {
		return fmt.Errorf("invalid socket structure detected")
	}

	if len(p.sys.PackageIDs()) > 1 {
		p.root = NewVirtualNode(p, "root", nilNode)
		p.nodes[p.root.Name()] = p.root
		klog.V(5).InfoS("Created pool for ", "name", p.root.Name())
	} else {
		klog.V(5).InfoS("Omitted pool for virtual root (single-socket system)")
	}

	sockets := p.createSocketNodes()
	numaNodes := p.createNumaNodes(sockets)
	p.createClusterNodes(numaNodes)

	// Assign node IDs while collecting pools in DFS order.
	p.pools = make([]Node, 0, len(p.nodes))
	var collectNodes func(n Node)
	collectNodes = func(n Node) {
		if n.IsNil() {
			return
		}

		p.pools = append(p.pools, n)
		n.SetNodeID(p.nodeCnt)
		p.nodeCnt++

		if p.depth < n.Depth() {
			p.depth = n.Depth()
		}

		for _, child := range n.Children() {
			collectNodes(child)
		}
	}
	collectNodes(p.root)

	clusterCount := len(p.sys.ClusterIDs())
	klog.InfoS("Topology pool setup completed",
		"socketCount", len(sockets),
		"numaCount", len(p.sys.NodeIDs()),
		"clusterCount", clusterCount,
		"clusterAffinityEnabled", p.enableClusterAffinity)

	p.root.Dump("Topology pool setup completed")
	return nil
}

// checkHWTopology validates the hardware topology.
func (p *TopologyAwarePolicy) checkHWTopology() error {
	klog.Info("Checking hardware topology")

	if err := p.sys.ValidateTopology(); err != nil {
		klog.ErrorS(err, "Hardware topology check failed")
		return err
	}

	klog.Info("Hardware topology check passed")
	return nil
}
