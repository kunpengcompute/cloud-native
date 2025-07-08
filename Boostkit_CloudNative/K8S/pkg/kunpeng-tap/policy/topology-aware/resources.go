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
	"regexp"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
	"k8s.io/utils/cpuset"
	"kunpeng.huawei.com/kunpeng-cloud-computing/pkg/kunpeng-tap/policy"
	"kunpeng.huawei.com/kunpeng-cloud-computing/pkg/kunpeng-tap/sysfs/system"
)

type Supply interface {
	// GetNode returns the node supplying this capacity.
	GetNode() Node
	// Collect collects the given supply into this one.
	Collect(Supply)
	// Clone clones the given supply.
	Clone() Supply
	// GetScore calculates how well this supply fits/fulfills the given request.
	GetScore(Request) Score
	// Granted returns the locally granted capacity in this supply.
	GrantedShared() int
	// AllocatableSharedCPU calculates the allocatable amount of shared CPU of this supply.
	AllocatableSharedCPU() int

	Allocate(Request) (Grant, error)

	// SharableCPUs returns the sharable cpuset in this supply.
	SharableCPUs() cpuset.CPUSet

	Release(Grant)

	String() string

	// GrantedMemory 返回已分配的内存总量
	GrantedMemory() uint64
	// AllocatableMemory 返回可分配的内存总量
	AllocatableMemory() uint64
	// Memset 返回内存亲和集
	Memset() cpuset.CPUSet
}

type supply struct {
	node          Node
	isolated      cpuset.CPUSet
	sharable      cpuset.CPUSet
	grantedShared int
	// 新增内存相关字段
	memoryTotal   uint64 // 总内存量（KB）
	grantedMemory uint64 // 已分配内存量（KB）
}

func newSupply(n Node, isolated cpuset.CPUSet, sharable cpuset.CPUSet) Supply {
	// 获取节点的内存信息
	memTotal := uint64(0)
	memInfo, err := n.MemoryInfo()
	if err == nil && memInfo != nil {
		memTotal = memInfo.MemTotal
	}

	return &supply{
		node:          n,
		isolated:      isolated,
		sharable:      sharable,
		memoryTotal:   memTotal,
		grantedMemory: 0,
	}
}

func (s *supply) String() string {
	return fmt.Sprintf("<Supply: node %s, isolated %s, sharable %s, granted CPU %d, memory total %d KB, granted memory %d KB, allocatable memory %d KB>",
		s.node.Name(), s.isolated, s.sharable, s.grantedShared, s.memoryTotal, s.grantedMemory, s.AllocatableMemory())
}

func (s *supply) GetNode() Node {
	return s.node
}

// SharableCpus returns the sharable CPUSet of this supply.
func (cs *supply) SharableCPUs() cpuset.CPUSet {
	return cs.sharable.Clone()
}

func (s *supply) Collect(more Supply) {
	s.isolated = s.isolated.Union(more.(*supply).isolated)
	s.sharable = s.sharable.Union(more.(*supply).sharable)
}

// Score collects data for scoring this supply wrt. the given request.
func (s *supply) GetScore(req Request) Score {
	score := &score{
		supply:    s,
		request:   req,
		colocated: 0,
	}
	// 计算可分配的共享 CPU
	score.shared = s.AllocatableSharedCPU()
	score.shared -= req.CPULimit()

	// 计算 colocation score
	s.node.Policy().allocations.grants.Range(func(_, grantVal interface{}) bool {
		grant := grantVal.(Grant)
		if grant.GetNode().NodeID() == s.node.NodeID() {
			score.colocated++
		}
		return true
	})

	// 计算节点上的GPU数量
	numaIDs := s.node.GetNUMAIDs()
	score.gpuCount = 0
	for _, numaID := range numaIDs {
		score.gpuCount += len(s.node.Policy().sys.NodeGPUs(numaID))
	}

	return score
}

func (s *supply) GrantedShared() int {
	return s.grantedShared
}

func (s *supply) AllocatableSharedCPU() int {
	shared := 1000 * s.sharable.Size()
	return shared
}

func (s *supply) Allocate(req Request) (Grant, error) {
	grant, err := s.AllocateCPU(req)
	if err != nil {
		return nil, err
	}

	// 处理内存分配
	memoryRequest := req.GetContext().Request.Resources.GetLimits().Memory().Value() / 1024 // 转换为 KB
	if memoryRequest > 0 {
		// 检查是否有足够的内存
		if uint64(memoryRequest) > s.AllocatableMemory() {
			// 释放已分配的 CPU 资源
			// s.Release(grant)
			// return nil, fmt.Errorf("not enough memory for %d KB in node %s (available: %d KB)",
			// 	memoryRequest, s.node.Name(), s.AllocatableMemory())
			klog.ErrorS(nil, "Not enough memory for container",
				"node", s.node.Name(),
				"request", req,
				"available", s.AllocatableMemory())
		}

		// 分配内存
		s.grantedMemory += uint64(memoryRequest)

		// 设置内存分配信息到 grant
		grant.SetAllocatedMemory(uint64(memoryRequest))
	}

	return grant, nil
}

// AllocateCPU allocates CPU capacity from this supply and returns it as a grant.
func (s *supply) AllocateCPU(req Request) (Grant, error) {
	grant := newGrant(s.node, req.GetContext(), false, 0)

	resource := req.GetContext().Request.Resources
	limitCpu := resource.GetLimits().Cpu()

	// 节点内的共享CPU部分不满足 container 要求
	if limitCpu.MilliValue() > 0 && s.AllocatableSharedCPU() < int(limitCpu.MilliValue()) {

		return nil, fmt.Errorf("not enough shared CPU for %d in %s(-%d) of %s",
			limitCpu.MilliValue(), s.sharable.String(), s.grantedShared, s.node.Name())
	}
	s.grantedShared += int(limitCpu.MilliValue())
	grant.SetAllocatedCPU(int(limitCpu.MilliValue()))

	return grant, nil
}

// AllocatableMemory 返回可分配的内存总量（KB）
func (s *supply) AllocatableMemory() uint64 {
	if s.memoryTotal <= s.grantedMemory {
		return 0
	}
	return s.memoryTotal - s.grantedMemory
}

// GrantedMemory 返回已分配的内存总量（KB）
func (s *supply) GrantedMemory() uint64 {
	return s.grantedMemory
}

func (s *supply) Memset() cpuset.CPUSet {
	memInfo, err := s.node.MemoryInfo()
	if err != nil {
		return cpuset.New()
	}
	return memInfo.MemSet
}

// Release 释放已分配的资源
func (s *supply) Release(g Grant) {
	// 释放 CPU 资源
	s.grantedShared -= g.AllocatedCPUs()

	// 确保 grantedShared 不会小于 0
	if s.grantedShared < 0 {
		s.grantedShared = 0
	}

	// 释放内存资源
	if memGrant, ok := g.(*grant); ok {
		s.grantedMemory -= memGrant.allocatedMemory
		// 确保 grantedMemory 不会小于 0
		if s.grantedMemory < 0 {
			s.grantedMemory = 0
		}
	}
}

func (s *supply) Clone() Supply {
	return newSupply(s.node, s.isolated, s.sharable)
}

// Request represents a container's resource request
type Request interface {
	GetContext() policy.ContainerContext
	CPULimit() int
	CPURequest() int
	HasGPURequest() bool
	GetRequestedGPUDevices() []string
	// String returns a printable representation of this request.
	String() string
}

// request implements the Request interface
type request struct {
	container           policy.ContainerContext
	cpuLimit            int               // millicores
	cpuRequest          int               // millicores
	memLimit            int               // KB
	memRequest          int               // KB
	memType             system.MemoryType // memory type
	hasGPURequest       bool              // 是否请求GPU资源
	requestedGPUDevices []string          // 请求的GPU设备ID列表
}

func (r *request) GetContext() policy.ContainerContext {
	return r.container
}

func (r *request) CPULimit() int {
	return r.cpuLimit
}

func (r *request) CPURequest() int {
	return r.cpuRequest
}

func (r *request) HasGPURequest() bool {
	return r.hasGPURequest
}

func (r *request) GetRequestedGPUDevices() []string {
	return r.requestedGPUDevices
}

func (r *request) String() string {
	return fmt.Sprintf("cpu: %dm, gpu: %v, devices: %v",
		r.cpuLimit, r.hasGPURequest, r.requestedGPUDevices)
}

// DeviceType 表示加速器设备类型
type DeviceType string

const (
	// GPU 设备类型
	GPU DeviceType = "GPU"
	// NPU 设备类型
	NPU DeviceType = "NPU"
	// FPGA 设备类型
	FPGA DeviceType = "FPGA"
	// 其他设备类型可以在此添加
)

// DeviceEnvConfig 定义了设备环境变量配置
type DeviceEnvConfig struct {
	AllocateEnvName string     // 分配设备的环境变量名
	VisibleEnvName  string     // 可见设备的环境变量名
	DevicePrefix    string     // 设备路径前缀，如 "/dev/vacc"
	DeviceRegex     string     // 设备ID提取正则表达式
	DeviceType      DeviceType // 设备类型
}

// 已知设备环境变量配置
var knownDeviceConfigs = []DeviceEnvConfig{
	{
		AllocateEnvName: "VA_ALLOCATE_DEVICES",
		VisibleEnvName:  "VA_VISIBLE_DEVICES",
		DevicePrefix:    "/dev/vacc",
		DeviceRegex:     `/dev/vacc(\d+)`,
		DeviceType:      GPU,
	},
	// 可以添加更多设备配置
}

// parseDeviceID 从设备路径中提取设备ID
func parseDeviceID(device, regex string) (string, bool) {
	device = strings.TrimSpace(device)
	if match := regexp.MustCompile(regex).FindStringSubmatch(device); len(match) > 1 {
		return match[1], true
	}
	return "", false
}

// checkDeviceRequest 检查容器是否请求了特定类型的设备
func checkDeviceRequest(containerEnvs map[string]string, config DeviceEnvConfig) (bool, []string) {
	hasRequest := false
	deviceIDs := []string{}

	// 检查分配设备环境变量
	if allocDevices, ok := containerEnvs[config.AllocateEnvName]; ok {
		hasRequest = true
		devices := strings.Split(allocDevices, ",")
		for _, device := range devices {
			if deviceID, ok := parseDeviceID(device, config.DeviceRegex); ok {
				deviceIDs = append(deviceIDs, deviceID)
			}
		}
	}

	// 检查可见设备环境变量
	if visibleDevices, ok := containerEnvs[config.VisibleEnvName]; ok {
		hasRequest = true
		devices := strings.Split(visibleDevices, ",")
		for _, deviceID := range devices {
			deviceID = strings.TrimSpace(deviceID)
			if deviceID != "" {
				deviceIDs = append(deviceIDs, deviceID)
			}
		}
	}

	return hasRequest, deviceIDs
}

// Helper method to parse CPU and memory resources
func parseResourceRequirements(r *request, resourceReq, resourceLimit *corev1.ResourceList) {
	// 解析CPU资源
	if resourceLimit != nil && resourceLimit.Cpu() != nil {
		r.cpuLimit = int(resourceLimit.Cpu().MilliValue())
	}

	if resourceReq != nil && resourceReq.Cpu() != nil {
		r.cpuRequest = int(resourceReq.Cpu().MilliValue())
	}

	// 解析Memory请求
	if resourceReq != nil && resourceReq.Memory() != nil {
		r.memRequest = int(resourceReq.Memory().MilliValue())
	}

	if resourceLimit != nil && resourceLimit.Memory() != nil {
		r.memLimit = int(resourceLimit.Memory().MilliValue())
	} else {
		r.memLimit = r.memRequest
	}

	r.memType = system.MemoryTypeDRAM
}

// Helper method to process GPU device requests
func processGPUDeviceRequests(r *request, containerEnvs map[string]string) {
	if containerEnvs == nil {
		return
	}

	// 遍历所有已知设备配置
	for _, config := range knownDeviceConfigs {
		hasRequest, deviceIDs := checkDeviceRequest(containerEnvs, config)

		// 根据设备类型设置相应的字段
		switch config.DeviceType {
		case GPU:
			if hasRequest {
				r.hasGPURequest = true
				r.requestedGPUDevices = append(r.requestedGPUDevices, deviceIDs...)
			}
		case NPU:
			// TODO: 添加NPU设备类型的处理
		default:
			klog.ErrorS(nil, "Unknown device type", "deviceType", config.DeviceType)
		}
	}
}

// newRequest creates a new request from a container context
func newRequest(containerCtx policy.ContainerContext) Request {
	r := &request{
		container:           containerCtx,
		cpuLimit:            0,
		cpuRequest:          0,
		hasGPURequest:       false,
		requestedGPUDevices: []string{},
	}
	request := containerCtx.Request

	resourceReq, resourceLimit := request.Resources.GetRequests(), request.Resources.GetLimits()
	if resourceReq == nil || resourceLimit == nil {
		klog.V(0).InfoS("Resource requirements or limits are nil")
		return nil
	}
	parseResourceRequirements(r, resourceReq, resourceLimit)
	// 检查是否请求GPU资源
	klog.InfoS("Claims done, Start to Check GPU")
	processGPUDeviceRequests(r, containerCtx.Request.ContainerEnvs)

	return r
}

type Grant interface {
	GetContext() policy.ContainerContext
	// GetNode returns the Node this grant is allocated to.
	GetNode() Node
	// String returns a printable representation of this grant.
	String() string
	// AllocatedCPUs returns the amount of milli-CPU allocated.
	AllocatedCPUs() int
	// Exclusive returns whether this grant is exclusive.
	Exclusive() bool
	// SetAllocatedCPU sets the amount of milli-CPU allocated.
	SetAllocatedCPU(allocatedCPUs int)
	// AllocatedMemory returns the amount of memory allocated in KB.
	AllocatedMemory() uint64
	// SetAllocatedMemory sets the amount of memory allocated in KB.
	SetAllocatedMemory(memory uint64)
	// SharedCPUSet returns the set of CPUs this container can use.
	SharedCPUSet() cpuset.CPUSet

	Memset() cpuset.CPUSet

	Release()
}

var _ Grant = &grant{}

type grant struct {
	containerCtx    policy.ContainerContext
	node            Node
	exclusive       bool
	allocatedCPUs   int    // milliCPUs
	allocatedMemory uint64 // memory in KB
}

func newGrant(n Node, ctrCtx policy.ContainerContext, exclusive bool, grantedCPUs int) Grant {
	return &grant{
		node:            n,
		containerCtx:    ctrCtx,
		exclusive:       exclusive,
		allocatedCPUs:   grantedCPUs,
		allocatedMemory: 0,
	}
}

// GetRequest returns the container requests.
func (g *grant) GetContext() policy.ContainerContext {
	return g.containerCtx
}

func (g *grant) GetNode() Node {
	return g.node
}

func (g *grant) String() string {
	return fmt.Sprintf("<Grant: node %s, container %s, exclusive %v, allocatedCPUs %d, allocatedMemory %d KB>",
		g.node.Name(), g.containerCtx.Request.ContainerMeta.Name, g.exclusive, g.allocatedCPUs, g.allocatedMemory)
}

func (g *grant) AllocatedCPUs() int {
	return g.allocatedCPUs
}

func (g *grant) Exclusive() bool {
	return g.exclusive
}

func (g *grant) SetAllocatedCPU(allocatedCPUs int) {
	g.allocatedCPUs = allocatedCPUs
}

func (g *grant) AllocatedMemory() uint64 {
	return g.allocatedMemory
}

func (g *grant) SetAllocatedMemory(memory uint64) {
	g.allocatedMemory = memory
}

func (g *grant) SharedCPUSet() cpuset.CPUSet {
	return g.node.FreeResource().SharableCPUs()
}

func (g *grant) Memset() cpuset.CPUSet {
	return g.node.FreeResource().Memset()
}

func (g *grant) Release() {
	g.node.FreeResource().Release(g)
}

// Score represents scoring data for a node.
type Score interface {
	Supply() Supply

	Request() Request

	SharedCapacity() int
	// Colocated returns the number of containers already allocated to this node.
	Colocated() int
	// GPUCount returns the number of GPUs attached to this node.
	GPUCount() int
	// String returns the score as a string.
	String() string
}

// score implements the Score interface
type score struct {
	supply    Supply  // CPU supply (node)
	request   Request // CPU request (container)
	isolated  int     // remaining isolated CPUs
	shared    int     // remaining shared capacity
	colocated int     // number of colocated containers
	gpuCount  int     // number of GPUs attached to this node
}

func (s *score) Supply() Supply {
	return s.supply
}

func (s *score) Request() Request {
	return s.request
}

func (s *score) String() string {
	return fmt.Sprintf("<Score: node %s, shared:%d, colocated:%d, gpuCount:%d >", s.supply.GetNode().Name(), s.shared, s.colocated, s.gpuCount)
}

func (s *score) SharedCapacity() int {
	return s.shared
}

func (s *score) Colocated() int {
	return s.colocated
}

func (s *score) GPUCount() int {
	return s.gpuCount
}
