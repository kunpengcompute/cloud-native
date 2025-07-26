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
	// GrantedCPUByRequest returns the amount of milli-CPU granted by request.
	GrantedCPUByRequest() int
	// GrantedCPUByLimit returns the amount of milli-CPU granted by limit.
	GrantedCPUByLimit() int
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
	node                Node
	isolated            cpuset.CPUSet
	sharable            cpuset.CPUSet
	grantedShared       int
	grantedCPUByRequest int
	grantedCPUByLimit   int
	memoryTotal         uint64 // 总内存量（KB）
	grantedMemory       uint64 // 已分配内存量（KB）
}

func newSupply(n Node, isolated cpuset.CPUSet, sharable cpuset.CPUSet) Supply {
	// 获取节点的内存信息
	memTotal := uint64(0)
	memInfo, err := n.MemoryInfo()
	if err == nil && memInfo != nil {
		memTotal = memInfo.MemTotal
	}

	return &supply{
		node:                n,
		isolated:            isolated,
		sharable:            sharable,
		grantedShared:       0,
		grantedCPUByRequest: 0,
		grantedCPUByLimit:   0,
		memoryTotal:         memTotal,
		grantedMemory:       0,
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

// Collect collects the given supply into this one.
func (s *supply) Collect(more Supply) {
	moreSupply, ok := more.(*supply)
	if !ok {
		klog.ErrorS(nil, "Failed to collect supply", "supply", more)
		return
	}
	s.isolated = s.isolated.Union(moreSupply.isolated)
	s.sharable = s.sharable.Union(moreSupply.sharable)
	s.grantedShared += moreSupply.grantedShared
	s.grantedCPUByRequest += moreSupply.grantedCPUByRequest
	s.grantedCPUByLimit += moreSupply.grantedCPUByLimit
	s.memoryTotal += moreSupply.memoryTotal
	s.grantedMemory += moreSupply.grantedMemory
}

// Score collects data for scoring this supply wrt. the given request.
func (s *supply) GetScore(req Request) Score {
	score := &score{
		supply:    s,
		request:   req,
		colocated: 0,
	}
	// 计算可分配的共享 CPU
	score.shared = s.AllocatableSharedCPU() - req.CPULimit()
	score.sharedByRequest = s.AllocatableCPUByRequest() - req.CPURequest()
	score.sharedByLimit = s.AllocatableCPUByLimit() - req.CPULimit()
	score.memoryCapacity = s.AllocatableMemory()

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

func (s *supply) GrantedCPUByRequest() int {
	return s.grantedCPUByRequest
}

func (s *supply) GrantedCPUByLimit() int {
	return s.grantedCPUByLimit
}

func (s *supply) AllocatableSharedCPU() int {
	shared := 1000 * s.sharable.Size()
	return shared - s.grantedShared
}

func (s *supply) TotalSharedCPU() int {
	shared := 1000 * s.sharable.Size()
	return shared
}

func (s *supply) AllocatableCPUByRequest() int {
	return s.TotalSharedCPU() - s.grantedCPUByRequest
}

func (s *supply) AllocatableCPUByLimit() int {
	return s.TotalSharedCPU() - s.grantedCPUByLimit
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
	requestCpu := resource.GetRequests().Cpu().MilliValue()
	limitCpu := resource.GetLimits().Cpu().MilliValue()

	// requestCpu值的和不超出 TotalSharedCPU
	totalSharedCPU := s.TotalSharedCPU()
	if requestCpu+int64(s.GrantedCPUByRequest()) > int64(totalSharedCPU) {
		return nil, fmt.Errorf("request CPU %d exceeds total shared CPU %d", requestCpu, totalSharedCPU)
	}

	// limitCpu 值超出 TotalSharedCPU
	if limitCpu > 0 && int64(totalSharedCPU) < limitCpu {
		return nil, fmt.Errorf("not enough shared CPU for %d in %s(-%d) of %s",
			limitCpu, s.sharable.String(), s.grantedShared, s.node.Name())
	}

	// Update the granted CPU for grant and supply.
	grant.SetAllocatedCPU(int(limitCpu))
	grant.SetAllocatedCPUByRequest(int(requestCpu))
	grant.SetAllocatedCPUByLimit(int(limitCpu))

	// Update the granted CPU for supply.
	s.grantedShared += int(limitCpu)
	s.grantedCPUByRequest += int(requestCpu)
	s.grantedCPUByLimit += int(limitCpu)

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
	s.grantedCPUByRequest -= g.AllocatedCPUByRequest()
	s.grantedCPUByLimit -= g.AllocatedCPUByLimit()
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
	MemoryLimit() int64
	MemoryRequest() int64
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
	memLimit            int64             // KB
	memRequest          int64             // KB
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

func (r *request) MemoryLimit() int64 {
	return r.memLimit
}

func (r *request) MemoryRequest() int64 {
	return r.memRequest
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
	if containerEnvs == nil {
		return false, nil
	}

	var deviceIDs []string
	hasRequest := false

	// 检查分配设备环境变量
	if allocDevices := containerEnvs[config.AllocateEnvName]; allocDevices != "" {
		hasRequest = true
		deviceIDs = append(deviceIDs, extractDeviceIDs(allocDevices, config.DeviceRegex)...)
	}

	// 检查可见设备环境变量
	if visibleDevices := containerEnvs[config.VisibleEnvName]; visibleDevices != "" {
		hasRequest = true
		deviceIDs = append(deviceIDs, extractVisibleDeviceIDs(visibleDevices, config.DeviceRegex)...)
	}

	return hasRequest, deviceIDs
}

// extractDeviceIDs 从设备字符串中提取设备ID列表
func extractDeviceIDs(deviceStr, regex string) []string {
	if deviceStr == "" {
		return nil
	}

	var deviceIDs []string
	devices := strings.Split(deviceStr, ",")

	for _, device := range devices {
		if deviceID, ok := parseDeviceID(device, regex); ok {
			deviceIDs = append(deviceIDs, deviceID)
		}
	}

	return deviceIDs
}

// extractVisibleDeviceIDs 从可见设备字符串中提取设备ID列表
func extractVisibleDeviceIDs(deviceStr, regex string) []string {
	if deviceStr == "" {
		return nil
	}

	var deviceIDs []string
	devices := strings.Split(deviceStr, ",")

	for _, device := range devices {
		device = strings.TrimSpace(device)
		if device == "" {
			continue
		}

		// 尝试使用正则表达式提取设备ID
		if deviceID, ok := parseDeviceID(device, regex); ok {
			deviceIDs = append(deviceIDs, deviceID)
		} else {
			// 如果正则表达式匹配失败，直接使用原始字符串作为设备ID
			deviceIDs = append(deviceIDs, device)
		}
	}

	return deviceIDs
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
		r.memRequest = resourceReq.Memory().Value()
	}

	if resourceLimit != nil && resourceLimit.Memory() != nil {
		r.memLimit = resourceLimit.Memory().Value()
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
	// AllocatedCPUByRequest returns the amount of milli-CPU allocated by request.
	AllocatedCPUByRequest() int
	// AllocatedCPUByLimit returns the amount of milli-CPU allocated by limit.
	AllocatedCPUByLimit() int
	// Exclusive returns whether this grant is exclusive.
	Exclusive() bool
	// SetAllocatedCPU sets the amount of milli-CPU allocated.
	SetAllocatedCPU(allocatedCPUs int)
	// SetAllocatedCPUByRequest sets the amount of milli-CPU allocated by request.
	SetAllocatedCPUByRequest(allocatedCPUs int)
	// SetAllocatedCPUByLimit sets the amount of milli-CPU allocated by limit.
	SetAllocatedCPUByLimit(allocatedCPUs int)
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
	containerCtx          policy.ContainerContext
	node                  Node
	exclusive             bool
	allocatedCPUs         int // milliCPUs
	allocatedCPUByRequest int
	allocatedCPUByLimit   int
	allocatedMemory       uint64 // memory in KB
}

func newGrant(n Node, ctrCtx policy.ContainerContext, exclusive bool, grantedCPUs int) Grant {
	return &grant{
		node:                  n,
		containerCtx:          ctrCtx,
		exclusive:             exclusive,
		allocatedCPUs:         grantedCPUs,
		allocatedCPUByRequest: 0,
		allocatedCPUByLimit:   0,
		allocatedMemory:       0,
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

func (g *grant) AllocatedCPUByRequest() int {
	return g.allocatedCPUByRequest
}

func (g *grant) AllocatedCPUByLimit() int {
	return g.allocatedCPUByLimit
}

func (g *grant) Exclusive() bool {
	return g.exclusive
}

func (g *grant) SetAllocatedCPU(allocatedCPUs int) {
	g.allocatedCPUs = allocatedCPUs
}

func (g *grant) SetAllocatedCPUByRequest(allocatedCPUs int) {
	g.allocatedCPUByRequest = allocatedCPUs
}

func (g *grant) SetAllocatedCPUByLimit(allocatedCPUs int) {
	g.allocatedCPUByLimit = allocatedCPUs
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
	// SharedCapacityByRequest returns the remaining shared capacity by request.
	SharedCapacityByRequest() int
	// SharedCapacityByLimit returns the remaining shared capacity by limit.
	SharedCapacityByLimit() int
	// MemoryCapacity returns the remaining memory capacity.
	MemoryCapacity() uint64
	// Colocated returns the number of containers already allocated to this node.
	Colocated() int
	// GPUCount returns the number of GPUs attached to this node.
	GPUCount() int
	// String returns the score as a string.
	String() string
}

// score implements the Score interface
type score struct {
	supply          Supply  // CPU supply (node)
	request         Request // CPU request (container)
	isolated        int     // remaining isolated CPUs
	shared          int     // remaining shared capacity
	sharedByRequest int     // remaining shared capacity by request
	sharedByLimit   int     // remaining shared capacity by limit
	memoryCapacity  uint64  // remaining memory capacity
	colocated       int     // number of colocated containers
	gpuCount        int     // number of GPUs attached to this node
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

func (s *score) SharedCapacityByRequest() int {
	return s.sharedByRequest
}

func (s *score) SharedCapacityByLimit() int {
	return s.sharedByLimit
}

func (s *score) MemoryCapacity() uint64 {
	return s.memoryCapacity
}

func (s *score) Colocated() int {
	return s.colocated
}

func (s *score) GPUCount() int {
	return s.gpuCount
}
