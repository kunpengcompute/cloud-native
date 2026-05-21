/*
 * Copyright 2022 The Koordinator Authors.
 * Modifications Copyright 2025 Huawei Technology corp.
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

package sysfs

import (
	"fmt"
	"math"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/shirou/gopsutil/v3/cpu"
	"k8s.io/klog/v2"
)

// type NUMANodeInfo map[int]cpuset.CPUSet

// CPUInfo contains the NUMA, socket, and core IDs associated with a CPU.
type CPUInfo struct {
	NUMANodeID int
	SocketID   int
	CoreID     int
}

// CPUDetails is a map from CPU ID to Core ID, Socket ID, and NUMA ID.
type CPUDetails map[int]CPUInfo

// CPUTopology contains details of node cpu, where :
// CPU  - logical CPU, cadvisor - thread
// Core - physical CPU, cadvisor - Core
// Socket - socket, cadvisor - Socket
// NUMA Node - NUMA cell, cadvisor - Node
type CPUTopology struct {
	NumCPUs      int
	NumCores     int
	NumSockets   int
	NumNUMANodes int
	CPUDetails   CPUDetails
}

// 将从 grpcClient 当中获取到的内容转换为 CPUTopology
func Discover(c *CPUTotalInfo) (*CPUTopology, error) {
	// grpcClient 发送请求来获知
	numCPUs := c.NumberCPUs
	numCores := len(c.CoreToCPU)
	numNumaNodes := len(c.NodeToCPU)
	numSockets := len(c.SocketToCPU)
	cpuDetails := make(CPUDetails)

	for numaNodeId, v := range c.NodeToCPU {
		for _, numaNodeInfo := range v {
			cpuId := numaNodeInfo.CPUID
			tmpInfo := cpuDetails[int(cpuId)]
			tmpInfo.NUMANodeID = int(numaNodeId)
			cpuDetails[int(cpuId)] = tmpInfo
		}
	}

	for coreId, v := range c.CoreToCPU {
		for _, coreInfo := range v {
			cpuId := coreInfo.CPUID
			tmpInfo := cpuDetails[int(cpuId)]
			tmpInfo.CoreID = int(coreId)
			cpuDetails[int(cpuId)] = tmpInfo
		}
	}

	return &CPUTopology{
		NumCPUs:      int(numCPUs),
		NumCores:     numCores,
		NumSockets:   numSockets,
		NumNUMANodes: numNumaNodes,
		CPUDetails:   cpuDetails,
	}, nil
}

// 移动到 grpcServer 端
// CPUTotalInfo describes the total number infos of the local cpu, e.g. the number of cores, the number of numa nodes
type CPUTotalInfo struct {
	NumberCPUs  int32                     `json:"numberCPUs"`
	CoreToCPU   map[int32][]ProcessorInfo `json:"coreToCPU"`
	NodeToCPU   map[int32][]ProcessorInfo `json:"nodeToCPU"`
	SocketToCPU map[int32][]ProcessorInfo `json:"socketToCPU"`
}

// LocalCPUInfo contains the cpu information collected from the node
type LocalCPUInfo struct {
	// ProcessorInfos contains topology information of all available CPUs
	ProcessorInfos []ProcessorInfo `json:"processorInfos,omitempty"`
	// TotalInfo stores the numbers of cpu processors, cores, sockets and nodes
	TotalInfo CPUTotalInfo `json:"totalInfo,omitempty"`
}

type numaNode struct {
	numaID   int32
	cpuStart int32
	cpuEnd   int32
}

type ProcessorInfo struct {
	// logic CPU/ processor ID
	CPUID int32 `json:"cpu"`
	// physical CPU core ID
	CoreID int32 `json:"core"`
	// cpu socket ID
	SocketID int32 `json:"socket"`
	// numa node ID
	NodeID int32 `json:"node"`
}

func GetLocalCPUInfo() (*LocalCPUInfo, error) {
	return getCpu()
}

func GetNumaNodeCPUSet(c *CPUTopology, nodeid int) string {
	mincpuid := math.MaxInt
	maxcpuid := -1
	for cpuid, cpuinfo := range c.CPUDetails {
		if cpuinfo.NUMANodeID == nodeid {
			mincpuid = min(mincpuid, cpuid)
			maxcpuid = max(maxcpuid, cpuid)
		}
	}

	result := strconv.Itoa(mincpuid) + "-" + strconv.Itoa(maxcpuid)
	return result
}

func getNodeCPU(nodeName string) (int32, int32, error) {
	cpulist, err := os.ReadFile(fmt.Sprintf("/sys/devices/system/node/%s/cpulist", nodeName))
	if err != nil {
		klog.ErrorS(err, "Failed to read CPU list", "node", nodeName)
		return -1, -1, err
	}
	cpuRange := strings.TrimSpace(string(cpulist))
	re := regexp.MustCompile(`(\d+)-(\d+)`)
	matches := re.FindStringSubmatch(cpuRange)
	if len(matches) != 3 {
		return -1, -1, fmt.Errorf("wrong cpu range format found")
	}
	cpuStart, err := strconv.Atoi(matches[1])
	if err != nil {
		return -1, -1, err
	}
	cpuEnd, err := strconv.Atoi(matches[2])
	if err != nil {
		return -1, -1, err
	}
	klog.InfoS("Node CPU range", "NUMA node", nodeName, "start", cpuStart, "end", cpuEnd)
	return int32(cpuStart), int32(cpuEnd), nil
}

func getNumaNodes() ([]numaNode, error) {
	// 读取 NUMA 节点信息
	nodes, err := os.ReadDir("/sys/devices/system/node/")
	if err != nil {
		klog.ErrorS(err, "Failed to read NUMA nodes")
		return nil, err
	}

	var nodeList []numaNode
	for _, node := range nodes {
		if strings.HasPrefix(node.Name(), "node") {
			nodeIDStr := strings.TrimPrefix(node.Name(), "node")
			nodeID, err := strconv.Atoi(nodeIDStr)
			if err != nil {
				klog.ErrorS(err, "Failed to parse NUMA node ID", "nodeIDStr", nodeIDStr)
				return nil, err
			}

			cpuStart, cpuEnd, err := getNodeCPU(node.Name())
			if err != nil {
				klog.ErrorS(err, "Failed to read NUMA CPUs", "node", node.Name())
				return nil, err
			}
			nodeList = append(nodeList, numaNode{int32(nodeID), cpuStart, cpuEnd})
		}
	}

	klog.InfoS("NUMA nodes count", "total", len(nodeList))
	return nodeList, nil
}

func getProcessorInfo(nodeList *[]numaNode, info cpu.InfoStat) (ProcessorInfo, error) {
	// 获取 CPU 核心信息
	coreID, err := strconv.Atoi(info.CoreID)
	if err != nil {
		klog.ErrorS(err, "Failed to parse core ID", "coreID", info.CoreID)
		return ProcessorInfo{}, err
	}
	cpuID := info.CPU
	nodeID := -1
	for _, node := range *nodeList {
		if cpuID >= int32(node.cpuStart) && cpuID <= int32(node.cpuEnd) {
			nodeID = int(node.numaID)
			break
		}
	}
	return ProcessorInfo{
		CPUID:    info.CPU,
		CoreID:   int32(coreID),
		SocketID: -1,
		NodeID:   int32(nodeID),
	}, nil
}

func fillTotalInfo(processorList *[]ProcessorInfo) CPUTotalInfo {
	coreToCPU := make(map[int32][]ProcessorInfo)
	nodeToCPU := make(map[int32][]ProcessorInfo)

	for _, i := range *processorList {
		coreToCPU[i.CoreID] = append(coreToCPU[i.CoreID], i)
		nodeToCPU[i.NodeID] = append(nodeToCPU[i.NodeID], i)
	}

	return CPUTotalInfo{
		int32(len(*processorList)),
		coreToCPU,
		nodeToCPU,
		map[int32][]ProcessorInfo{},
	}
}

func getCpu() (*LocalCPUInfo, error) {
	// 获取 CPU 信息
	info, err := cpu.Info()
	if err != nil {
		klog.ErrorS(err, "Failed to get CPU info")
		return nil, err
	}

	nodeList, err := getNumaNodes()
	if err != nil {
		klog.ErrorS(err, "Failed to get NUMA nodes")
		return nil, err
	}

	var processorList []ProcessorInfo
	for _, ci := range info {
		pi, err := getProcessorInfo(&nodeList, ci)
		if err != nil {
			klog.ErrorS(err, "Failed to get processor info")
			return nil, err
		}
		processorList = append(processorList, pi)
	}

	cpuTotalInfo := fillTotalInfo(&processorList)
	klog.InfoS("CPU total info", "numberCPUs", cpuTotalInfo.NumberCPUs)
	return &LocalCPUInfo{
		ProcessorInfos: processorList,
		TotalInfo:      cpuTotalInfo,
	}, nil
}

// 解析 CpusetCpus 字段
func parseCpusetCpus(cpuset string) []int {
	cpus := []int{}
	ranges := strings.Split(cpuset, ",")
	for _, r := range ranges {

		if !strings.Contains(r, "-") {
			cpu, err := strconv.Atoi(r)
			if err != nil {
				klog.ErrorS(err, "Failed to parse CPU ID", "cpuSet", cpuset, "range", r)
				continue
			}
			cpus = append(cpus, cpu)
			continue
		}

		parts := strings.Split(r, "-")
		start, err := strconv.Atoi(parts[0])
		if err != nil {
			klog.ErrorS(err, "Failed to parse start CPU ID", "cpuSet", cpuset, "range", r)
			continue
		}
		end, err := strconv.Atoi(parts[1])
		if err != nil {
			klog.ErrorS(err, "Failed to parse end CPU ID", "cpuSet", cpuset, "range", r)
			continue
		}
		for i := start; i <= end; i++ {
			cpus = append(cpus, i)
		}
	}
	return cpus
}

// Helper function to find min and max CPU IDs
func findMinMaxCpus(cpus []int) (int, int) {
	if len(cpus) == 0 {
		return -1, -1
	}
	minCpu, maxCpu := cpus[0], cpus[0]
	for _, cpu := range cpus {
		if cpu < minCpu {
			minCpu = cpu
		}
		if cpu > maxCpu {
			maxCpu = cpu
		}
	}
	return minCpu, maxCpu
}

// Helper function to check if both CPUs are in the same NUMA node
func areCpusInNode(node []ProcessorInfo, minCpu, maxCpu int) bool {
	foundMin, foundMax := false, false
	for _, cpu := range node {
		if cpu.CPUID == int32(minCpu) {
			foundMin = true
		}
		if cpu.CPUID == int32(maxCpu) {
			foundMax = true
		}
		if foundMin && foundMax {
			return true
		}
	}
	return false
}

func GetNumaNodeFromCpuSet(cpuInfo *LocalCPUInfo, cpuSet string) int32 {
	cpus := parseCpusetCpus(cpuSet)
	if len(cpus) == 0 {
		return -1
	}

	minCpu, maxCpu := findMinMaxCpus(cpus)

	for nodeID, node := range cpuInfo.TotalInfo.NodeToCPU {
		if areCpusInNode(node, minCpu, maxCpu) {
			return nodeID
		}
	}

	return -1
}
