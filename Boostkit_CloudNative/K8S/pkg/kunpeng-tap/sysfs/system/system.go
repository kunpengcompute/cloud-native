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

// ID represents a unique identifier for system components (packages, nodes, CPUs, etc.)
type ID = int

const (
	// sysfs devices/cpu subdirectory path
	sysfsCPUPath = "devices/system/cpu"
	// sysfs device/node subdirectory path
	sysfsNumaNodePath = "devices/system/node"
)

// System represents the system resources.
type System interface {
	Discover() error
	AllowedSet() cpuset.CPUSet
	ValidateTopology() error
	PackageIDs() []ID
	NodeIDs() []ID
	Package(id ID) CPUPackage
	Node(id ID) Node
	NodeDistance(from, to ID) int
	GPUIDs() []ID                  // Returns all GPU IDs
	GPU(id ID) GPU                 // Returns GPU by ID
	NodeGPUs(nodeID ID) []ID       // Returns GPUs attached to a NUMA node
	MemoryInfo() (*MemInfo, error) // 返回系统的总内存信息
}

// CPUPackage is a physical package (a collection of CPUs).
type CPUPackage interface {
	ID() ID
	CPUSet() cpuset.CPUSet
	DieIDs() []ID
	NodeIDs() []ID
	AddCPU(ID)
	AddNode(ID)
	AddDie(ID)
	AddCPUToDie(dieID ID, cpuID ID)
	AddNodeToDie(dieID ID, nodeID ID)
	DieCPUSet(ID) cpuset.CPUSet
	DieNodeIDs(ID) []ID
	GetSortedNodes() []ID          // 返回已排序的节点ID列表
	MemoryInfo() (*MemInfo, error) // 返回包的内存信息
}

// Node represents a NUMA node.
type Node interface {
	ID() ID
	PackageID() ID
	DieID() ID
	CPUSet() cpuset.CPUSet
	Distance() []int
	DistanceFrom(ID) int
	SetPackageID(ID)
	SetDieID(ID)
	MemoryInfo() (*MemInfo, error)
	HasMemory() bool
	HasNormalMemory() bool
	GetMemoryType() MemoryType
	SetMemoryType(MemoryType)
	SetNormalMemory(bool)
	SetMemory(bool)
}

// MemInfo memory info for the node (partial content from the meminfo sysfs entry).
type MemInfo struct {
	MemTotal uint64 // kb
	MemFree  uint64 // kb
	MemUsed  uint64 // kb
	MemSet   cpuset.CPUSet
}

// CPU represents a CPU core.
type CPU interface {
	ID() ID
	PackageID() ID
	DieID() ID
	NodeID() ID
	CoreID() ID
	ThreadCPUSet() cpuset.CPUSet
	Isolated() bool
	Online() bool
}

// GPU represents a GPU device.
type GPU interface {
	ID() ID
	NodeID() ID // NUMA node ID where the GPU is located
	PCIAddress() string
	VendorID() string    // 返回供应商ID
	DeviceID() string    // 返回设备ID
	DeviceClass() string // 返回设备类
}

type gpu struct {
	id          ID     // GPU ID
	nodeID      ID     // NUMA node ID
	pciAddress  string // PCI address in format "0000:00:00.0"
	vendorID    string // 供应商ID，如 "0x10de" 表示 NVIDIA
	deviceID    string // 设备ID
	deviceClass string // 设备类，如 "0x0302"
}

// GPU methods
func (g *gpu) ID() ID {
	return g.id
}

func (g *gpu) NodeID() ID {
	return g.nodeID
}

func (g *gpu) PCIAddress() string {
	return g.pciAddress
}

func (g *gpu) VendorID() string {
	return g.vendorID
}

func (g *gpu) DeviceID() string {
	return g.deviceID
}

func (g *gpu) DeviceClass() string {
	return g.deviceClass
}

var _ System = &system{}

type system struct {
	path           string            // sysfs path
	sysRoot        string            // sysfs mount point
	packages       map[ID]CPUPackage // physical packages
	nodes          map[ID]Node       // NUMA nodes
	cpus           map[ID]CPU        // CPUs
	offline        cpuset.CPUSet     // offlined CPUs
	isolated       cpuset.CPUSet     // isolated CPUs
	threadsPerCore int               // hyperthreads per core
	gpus           map[ID]GPU        // GPUs
}

// NewSystem creates a new System instance, discovering the system resources.
// sysRoot is the path to the sysfs mount point, default is "/sys".
func NewSystem(sysRoot string) (System, error) {
	// If sysRoot is not set, use the default path
	system := &system{}

	system.SetSysRoot(sysRoot)

	err := system.Discover()
	if err != nil {
		return nil, err
	}

	return system, nil
}

func (s *system) AllowedSet() cpuset.CPUSet {
	cpus := cpuset.New()
	for _, cpu := range s.cpus {
		cpus = cpus.Union(cpuset.New(cpu.ID()))
	}
	cpus = cpus.Union(s.offline).Union(s.isolated)

	klog.V(3).InfoS("Allowed CPU set", "cpus", cpus)
	return cpus
}

// Set the sysfs root path
func (s *system) SetSysRoot(sysRoot string) {
	s.sysRoot = sysRoot
	s.path = filepath.Join("/", sysRoot, "sys")

	if _, err := os.Stat(s.path); err != nil {
		klog.ErrorS(err, "Failed to access sysfs path", "path", s.path)
	}

	klog.V(3).InfoS("Setting sysfs root path", "path", s.path)
}

// Helper method to populate node package information
func (s *system) populateNodePackageInfo() error {
	if len(s.nodes) == 0 {
		return nil
	}

	for _, pkg := range s.packages {
		for _, nodeID := range pkg.GetSortedNodes() {
			if node, ok := s.nodes[nodeID]; ok {
				node.SetPackageID(pkg.ID())
			} else {
				return fmt.Errorf("can't find package for ID %d", nodeID)
			}
		}
	}
	return nil
}

func (s *system) Discover() error {
	// Discover CPUs
	if err := s.discoverCPUs(); err != nil {
		return err
	}

	if err := s.discoverNodes(); err != nil {
		return err
	}

	if err := s.discoverPackages(); err != nil {
		return err
	}

	// 发现GPU设备
	if err := s.discoverGPUs(); err != nil {
		klog.ErrorS(err, "Failed to discover GPUs")
		// 不要因为GPU发现失败而中断整个发现过程
	}

	// 填充节点信息
	if err := s.populateNodePackageInfo(); err != nil {
		return err
	}
	return nil
}

func (s *system) discoverCPUs() error {
	// TODO: 删除此处检查，允许重复进行感知，以支持重复感知
	if s.cpus != nil {
		return nil
	}

	s.cpus = make(map[ID]CPU)

	// Discover CPUs and populate the map
	_, err := readSysfsEntry(s.path, filepath.Join(sysfsCPUPath, "isolated"), &s.isolated, ",")
	if err != nil {
		klog.ErrorS(err, "Failed to get set of isolated cpus")
	}

	entries, _ := filepath.Glob(filepath.Join(s.path, sysfsCPUPath, "cpu[0-9]*"))
	for _, entry := range entries {
		if err := s.discoverCPU(entry); err != nil {
			return err
		}
	}

	return nil
}

// Helper method to read CPU topology information
func (s *system) readCPUTopology(cpu *cpu, path string) {
	if _, err := readSysfsEntry(path, "topology/physical_package_id", &cpu.pkg); err != nil {
		klog.V(3).InfoS("Can't get physical_package_id", "error", err)
		cpu.pkg = -1
	}
	if _, err := readSysfsEntry(path, "topology/die_id", &cpu.die); err != nil {
		klog.V(3).InfoS("Can't get die_id", "error", err)
		cpu.die = -1
	}
	if _, err := readSysfsEntry(path, "topology/core_id", &cpu.core); err != nil {
		klog.V(3).InfoS("Can't get core_id", "error", err)
		cpu.core = -1
	}
	if _, err := readSysfsEntry(path, "topology/thread_siblings_list", &cpu.threads, ","); err != nil {
		klog.V(3).InfoS("Can't get thread_siblings_list", "error", err)
		cpu.threads = cpuset.New(cpu.id)
	}
}

// Helper method to update threads per core count
func (s *system) updateThreadsPerCore(cpu *cpu) {
	if s.threadsPerCore < 1 {
		s.threadsPerCore = 1
	}
	if cpu.threads.Size() > s.threadsPerCore {
		s.threadsPerCore = cpu.threads.Size()
	}
}

func (s *system) discoverCPU(path string) error {
	cpu := &cpu{path: path, id: getEnumeratedID(path), online: true}
	cpu.isolated = s.isolated.Contains(cpu.id)

	if online, err := readSysfsEntry(path, "online", nil); err == nil {
		cpu.online = (online != "" && online[0] != '0')
	}

	if cpu.online {
		s.readCPUTopology(cpu, path)
	} else {
		s.offline = s.offline.Union(cpuset.New(cpu.id))
	}

	if node, _ := filepath.Glob(filepath.Join(path, "node[0-9]*")); len(node) == 1 {
		cpu.node = getEnumeratedID(node[0])
	} else {
		return fmt.Errorf("exactly one node per cpu allowed")
	}

	s.updateThreadsPerCore(cpu)

	s.cpus[cpu.id] = cpu

	return nil
}

func (s *system) discoverNodes() error {
	if s.nodes != nil {
		return nil
	}

	s.nodes = make(map[ID]Node)

	// Discover NUMA nodes and populate the map
	entries, _ := filepath.Glob(filepath.Join(s.path, sysfsNumaNodePath, "node[0-9]*"))
	for _, entry := range entries {
		if err := s.discoverNode(entry); err != nil {
			return err
		}
	}

	// Discover memory information for nodes
	if err := s.discoverNodeMemory(); err != nil {
		return fmt.Errorf("failed to discover node memory information: %v", err)
	}

	// Improve readability, print the NUMA nodes with CPUs
	cpuNodes := []int{}
	for id, node := range s.nodes {
		if node.CPUSet().Size() > 0 {
			cpuNodes = append(cpuNodes, int(id))
		}
	}

	klog.V(3).InfoS("NUMA nodes with CPUs", "nodes", cpuNodes)
	return nil
}

// Helper method to discover and set nodes with memory
func (s *system) discoverNodesWithMemory() error {
	memoryNodeIDs, err := readSysfsEntry(s.path, filepath.Join(sysfsNumaNodePath, "has_memory"), nil)
	if err != nil {
		klog.ErrorS(err, "Failed to discover nodes with memory")
		return nil // Don't fail the entire discovery process
	}

	if memoryNodeIDs == "" {
		return nil
	}

	memoryNodes, err := cpuset.Parse(memoryNodeIDs)
	if err != nil {
		klog.ErrorS(err, "Failed to parse nodes with memory", "nodes", memoryNodeIDs)
		return nil
	}

	for _, node := range s.nodes {
		if memoryNodes.Contains(int(node.ID())) {
			node.SetMemory(true)
		}
	}
	return nil
}

// Helper method to discover and set nodes with normal memory
func (s *system) discoverNodesWithNormalMemory() error {
	normalMemNodeIDs, err := readSysfsEntry(s.path, filepath.Join(sysfsNumaNodePath, "has_normal_memory"), nil)
	if err != nil {
		klog.ErrorS(err, "Failed to discover nodes with normal memory")
		return nil
	}

	if normalMemNodeIDs == "" {
		return nil
	}

	normalMemNodes, err := cpuset.Parse(normalMemNodeIDs)
	if err != nil {
		klog.ErrorS(err, "Failed to parse nodes with normal memory", "nodes", normalMemNodeIDs)
		return nil
	}

	for _, node := range s.nodes {
		if normalMemNodes.Contains(int(node.ID())) {
			node.SetNormalMemory(true)
			node.SetMemoryType(MemoryTypeDRAM)
		}
	}
	return nil
}

// Helper method to read memory information for nodes
func (s *system) readNodeMemoryInfo() {
	for _, node := range s.nodes {
		if !node.HasMemory() {
			klog.InfoS("Node %d doesn't have memory", node.ID())
			continue
		}
		memInfo, err := node.MemoryInfo()
		if err != nil {
			klog.ErrorS(err, "Failed to read memory info for node", "nodeID", node.ID())
		} else {
			klog.V(3).InfoS("Node memory info",
				"nodeID", node.ID(),
				"memTotal", memInfo.MemTotal,
				"memFree", memInfo.MemFree)
		}
	}
}

// discoverNodeMemory discovers memory information for NUMA nodes
func (s *system) discoverNodeMemory() error {
	// Find nodes with memory
	if err := s.discoverNodesWithMemory(); err != nil {
		return err
	}

	// Find nodes with normal memory
	if err := s.discoverNodesWithNormalMemory(); err != nil {
		return err
	}

	// Set all nodes with memory to have DRAM memory type and read memory info
	s.readNodeMemoryInfo()

	return nil
}

func (s *system) discoverNode(path string) error {
	node := &node{path: path, id: getEnumeratedID(path)}

	if _, err := readSysfsEntry(path, "cpulist", &node.cpus, ","); err != nil {
		return err
	}
	if _, err := readSysfsEntry(path, "distance", &node.distance); err != nil {
		return err
	}

	s.nodes[node.id] = node

	return nil
}

func (s *system) discoverPackages() error {
	if s.packages != nil {
		return nil
	}

	s.packages = make(map[ID]CPUPackage)

	// Discover CPU packages and populate the map
	for _, cpu := range s.cpus {
		pkg, found := s.packages[cpu.PackageID()]
		if !found {
			pkg = &cpuPackage{
				id:       cpu.PackageID(),
				cpus:     cpuset.New(),
				nodes:    cpuset.New(),
				dies:     cpuset.New(),
				dieCPUs:  make(map[ID]cpuset.CPUSet),
				dieNodes: make(map[ID]cpuset.CPUSet),
			}
			s.packages[cpu.PackageID()] = pkg
		}
		// 添加 CPU 到包
		pkg.AddCPU(cpu.ID())

		// 添加 Node 到包
		nodeID := cpu.NodeID()
		pkg.AddNode(nodeID)

		// 添加 Die 到包
		dieID := cpu.DieID()
		pkg.AddDie(dieID)

		// 将 CPU 添加到对应的 Die
		pkg.AddCPUToDie(dieID, cpu.ID())

		// 将 Node 添加到对应的 Die
		pkg.AddNodeToDie(dieID, nodeID)

		// Set die information directly on the node during discovery
		if node, ok := s.nodes[nodeID]; ok {
			node.SetDieID(dieID)
		}
	}

	return nil
}

// SystemError represents system-related errors
type SystemError struct {
	msg string
}

func (e *SystemError) Error() string {
	return e.msg
}

// NewSystemError creates a new SystemError
func NewSystemError(format string, args ...interface{}) error {
	return &SystemError{
		msg: fmt.Sprintf(format, args...),
	}
}

// ValidateTopology validates system topology constraints
func (s *system) ValidateTopology() error {
	// 多个 Socket 间不能共享NUMA
	if err := s.validateSocketNodes(); err != nil {
		return err
	}

	// 多个 Die 间不能共享NUMA
	if err := s.validateDieNodes(); err != nil {
		return err
	}

	// NUMA 距离矩阵必须对称
	if err := s.validateNumaDistances(); err != nil {
		return err
	}

	return nil
}

// Validate NUMA nodes are not shared between sockets
func (s *system) validateSocketNodes() error {
	socketNodes := make(map[ID]cpuset.CPUSet)
	for _, socketID := range s.PackageIDs() {
		pkg := s.Package(socketID)
		socketNodes[socketID] = cpuset.New(pkg.NodeIDs()...)
	}

	for id1, nodes1 := range socketNodes {
		for id2, nodes2 := range socketNodes {
			if id1 == id2 {
				continue
			}
			if shared := nodes1.Intersection(nodes2); !shared.IsEmpty() {
				klog.ErrorS(nil, "Invalid hardware topology with NUMA nodes",
					"error", "shared NUMA nodes between sockets",
					"socket1", id1,
					"socket2", id2,
					"sharedNodes", shared.String())
				return NewSystemError("sockets #%v and #%v share NUMA node(s) %s",
					id1, id2, shared.String())
			}
		}
	}
	return nil
}

// Validate NUMA nodes are not shared between dies
func (s *system) validateDieNodes() error {
	for _, socketID := range s.PackageIDs() {
		pkg := s.Package(socketID)
		for _, id1 := range pkg.DieIDs() {
			nodes1 := cpuset.New(pkg.DieNodeIDs(id1)...)
			for _, id2 := range pkg.DieIDs() {
				if id1 == id2 {
					continue
				}
				nodes2 := cpuset.New(pkg.DieNodeIDs(id2)...)
				if shared := nodes1.Intersection(nodes2); !shared.IsEmpty() {
					klog.ErrorS(nil, "Invalid hardware topology",
						"error", "shared NUMA nodes between dies",
						"socket", socketID,
						"die1", id1,
						"die2", id2,
						"sharedNodes", shared.String())
					return NewSystemError("socket #%v: dies #%v and #%v share NUMA node(s) %s",
						socketID, id1, id2, shared.String())
				}
			}
		}
	}
	return nil
}

// Validate NUMA distance matrix symmetry
func (s *system) validateNumaDistances() error {
	for _, from := range s.NodeIDs() {
		for _, to := range s.NodeIDs() {
			d1 := s.NodeDistance(from, to)
			d2 := s.NodeDistance(to, from)
			if d1 != d2 {
				klog.ErrorS(nil, "Invalid hardware topology",
					"error", "asymmetric NUMA distances",
					"node1", from,
					"node2", to,
					"distance1", d1,
					"distance2", d2)
				return NewSystemError("asymmetric NUMA distances between nodes #%v and #%v: %v != %v",
					from, to, d1, d2)
			}
		}
	}
	return nil
}

// PackageIDs returns a sorted slice of physical package IDs
func (s *system) PackageIDs() []ID {
	ids := make([]ID, 0, len(s.packages))
	for id := range s.packages {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	return ids
}

// NodeIDs returns a sorted slice of NUMA node IDs
func (s *system) NodeIDs() []ID {
	ids := make([]ID, 0, len(s.nodes))
	for id := range s.nodes {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	return ids
}

// Package returns the package with the specified ID
func (s *system) Package(id ID) CPUPackage {
	return s.packages[id]
}

// Node returns the NUMA node with the specified ID
func (s *system) Node(id ID) Node {
	return s.nodes[id]
}

// NodeDistance returns the distance between two NUMA nodes
func (s *system) NodeDistance(from, to ID) int {
	if node, ok := s.nodes[from]; ok {
		return node.DistanceFrom(to)
	}
	return -1
}

// GPUIDs returns a sorted slice of GPU IDs
func (s *system) GPUIDs() []ID {
	ids := make([]ID, 0, len(s.gpus))
	for id := range s.gpus {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	return ids
}

// GPU returns the GPU with the specified ID
func (s *system) GPU(id ID) GPU {
	return s.gpus[id]
}

// NodeGPUs returns the GPUs attached to a specific NUMA node
func (s *system) NodeGPUs(nodeID ID) []ID {
	var gpus []ID
	for id, gpu := range s.gpus {
		if gpu.NodeID() == nodeID {
			gpus = append(gpus, id)
		}
	}
	sort.Ints(gpus)
	for _, id := range gpus {
		klog.V(4).InfoS("Found GPU attached to NUMA node", "gpuID", id, "numaNode", nodeID)
	}
	return gpus
}

// GPUDeviceClass 定义了不同类型的GPU设备类标识
type GPUDeviceClass struct {
	ClassID     string // 设备类ID，如 "0x0302"
	Description string // 设备类描述
}

// 已知的GPU设备类型
var knownGPUClasses = []GPUDeviceClass{
	{ClassID: "0x0302", Description: "3D Controller"},
	// 可以根据需要添加更多GPU类型
}

// isGPUDevice 检查设备是否为GPU设备
func isGPUDevice(classStr string) bool {
	// 移除可能的空白字符
	classStr = strings.TrimSpace(classStr)

	// 检查是否匹配任何已知的GPU设备类
	for _, gpuClass := range knownGPUClasses {
		if strings.HasPrefix(classStr, gpuClass.ClassID) {
			return true
		}
	}

	return false
}

// readGPUDeviceInfo 读取GPU设备的详细信息
func (s *system) readGPUDeviceInfo(pciDevicesPath, pciAddress string) (*gpu, error) {
	// 读取设备类信息
	classPath := filepath.Join(pciDevicesPath, pciAddress, "class")
	classBytes, err := os.ReadFile(classPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read device class: %v", err)
	}

	classStr := strings.TrimSpace(string(classBytes))
	if !isGPUDevice(classStr) {
		return nil, fmt.Errorf("not a GPU device: %s", classStr)
	}

	// 读取设备的NUMA节点
	numaNodePath := filepath.Join(pciDevicesPath, pciAddress, "numa_node")
	numaNodeBytes, err := os.ReadFile(numaNodePath)
	if err != nil {
		klog.V(3).InfoS("Failed to read NUMA node for GPU", "pciAddress", pciAddress, "error", err)
		return nil, err
	}

	numaNodeStr := strings.TrimSpace(string(numaNodeBytes))
	numaNode, err := strconv.Atoi(numaNodeStr)
	if err != nil {
		klog.V(3).InfoS("Invalid NUMA node value for GPU", "pciAddress", pciAddress, "value", numaNodeStr)
		return nil, err
	}

	// NUMA值为-1表示没有NUMA亲和性，设为0
	if numaNode < 0 {
		klog.V(3).InfoS("GPU has no NUMA affinity, setting to -1", "pciAddress", pciAddress)
		numaNode = -1
	}

	// 读取设备的供应商和设备ID（可选）
	vendorID, deviceID := s.readGPUVendorAndDeviceID(pciDevicesPath, pciAddress)

	return &gpu{
		id:          ID(-1), // ID将在外部设置
		nodeID:      ID(numaNode),
		pciAddress:  pciAddress,
		vendorID:    vendorID,
		deviceID:    deviceID,
		deviceClass: classStr,
	}, nil
}

// readGPUVendorAndDeviceID 读取GPU设备的供应商和设备ID
func (s *system) readGPUVendorAndDeviceID(pciDevicesPath, pciAddress string) (string, string) {
	vendorPath := filepath.Join(pciDevicesPath, pciAddress, "vendor")
	vendorBytes, err := os.ReadFile(vendorPath)
	vendorID := ""
	if err == nil {
		vendorID = strings.TrimSpace(string(vendorBytes))
	}

	devicePath := filepath.Join(pciDevicesPath, pciAddress, "device")
	deviceBytes, err := os.ReadFile(devicePath)
	deviceID := ""
	if err == nil {
		deviceID = strings.TrimSpace(string(deviceBytes))
	}

	return vendorID, deviceID
}

// discoverGPUs discovers GPU devices and their NUMA node associations
func (s *system) discoverGPUs() error {
	if s.gpus != nil {
		return nil
	}

	s.gpus = make(map[ID]GPU)

	// 查找所有PCI设备
	pciDevicesPath := filepath.Join(s.path, "bus/pci/devices")
	entries, err := os.ReadDir(pciDevicesPath)
	if err != nil {
		return fmt.Errorf("failed to read PCI devices directory: %v", err)
	}

	gpuID := 0
	for _, entry := range entries {
		pciAddress := entry.Name()

		gpuDevice, err := s.readGPUDeviceInfo(pciDevicesPath, pciAddress)
		if err != nil {
			// 不是GPU设备或读取失败，跳过
			continue
		}

		// 设置GPU ID
		gpuDevice.id = ID(gpuID)

		// 添加到GPU映射
		s.gpus[gpuDevice.id] = gpuDevice
		gpuID++

		klog.V(3).InfoS("Discovered GPU",
			"id", gpuDevice.id,
			"pciAddress", pciAddress,
			"numaNode", gpuDevice.nodeID,
			"class", gpuDevice.deviceClass,
			"vendor", gpuDevice.vendorID,
			"device", gpuDevice.deviceID)
	}

	klog.V(3).InfoS("Discovered GPUs", "count", len(s.gpus))
	return nil
}

// MemoryInfo returns the aggregated memory information for the entire system.
func (s *system) MemoryInfo() (*MemInfo, error) {
	// 创建一个新的 MemInfo 结构体来存储聚合结果
	result := &MemInfo{
		MemTotal: 0,
		MemFree:  0,
		MemUsed:  0,
		MemSet:   cpuset.New(),
	}

	// 遍历系统中的所有 NUMA 节点
	for _, nodeID := range s.NodeIDs() {
		node := s.Node(nodeID)
		if node == nil {
			continue
		}

		// 获取节点的内存信息
		nodeMemInfo, err := node.MemoryInfo()
		if err != nil {
			klog.ErrorS(err, "Failed to get memory info for node", "nodeID", nodeID)
			continue
		}

		// 累加内存信息
		result.MemTotal += nodeMemInfo.MemTotal
		result.MemFree += nodeMemInfo.MemFree
		result.MemUsed += nodeMemInfo.MemUsed
		result.MemSet = result.MemSet.Union(nodeMemInfo.MemSet)
	}

	return result, nil
}
