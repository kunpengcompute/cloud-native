// Copyright 2019 Intel Corporation. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cache

import (
	"sort"

	"github.com/containerd/nri/pkg/api"
	v1 "k8s.io/api/core/v1"
	criv1 "k8s.io/cri-api/pkg/apis/runtime/v1"
	"k8s.io/klog/v2"

	"kunpeng.huawei.com/kunpeng-cloud-computing/api/kunpeng-tap/policy-manager/v1alpha1"
)

// Create a container for a create request.
func (c *container) fromCreateRequest(req *criv1.CreateContainerRequest) error {
	// TODO: need implementation
	return nil
}

// Create container from a container list response.
func (c *container) fromListResponse(lrc *criv1.Container) error {
	// TODO: need implementation
	return nil
}

func (c *container) fromDockerRunRequest(req *v1alpha1.ContainerResourceHookRequest) error {
	if req.PodMeta != nil {
		c.PodID = req.PodMeta.Id
	}

	pod, ok := c.cache.Pods[c.PodID]
	if !ok {
		return cacheError("can't find cached pod %s for listed container", c.PodID)
	}

	c.ID = req.ContainerMeta.Id
	c.Name = req.ContainerMeta.Name
	c.Namespace = pod.Namespace
	c.State = ContainerStateCreating
	c.Resources = pod.Resources // TODO: fix this
	c.LinuxReq = req.ContainerResources

	klog.V(5).InfoS("ContainerResourceHookRequest", "request", req)
	if _, ok := pod.containers[c.Name]; !ok {
		pod.containers[c.Name] = c.ID
	}

	return nil
}

// Create container from NRI Container.
func (c *container) fromNriContainer(nriContainer *api.Container) error {
	c.PodID = nriContainer.PodSandboxId

	pod, ok := c.cache.Pods[c.PodID]
	if !ok {
		return cacheError("can't find cached pod %s for NRI container", c.PodID)
	}

	c.ID = nriContainer.Id
	c.Name = nriContainer.Name
	c.Namespace = pod.Namespace

	// Convert NRI container state to internal state
	c.State = c.convertNriContainerState(nriContainer.State)

	c.Resources = pod.Resources // Use pod resources as base

	// Convert NRI Linux container resources
	if nriContainer.Linux != nil && nriContainer.Linux.Resources != nil {
		c.LinuxReq = c.convertNriLinuxResources(nriContainer.Linux.Resources)
	}

	klog.InfoS("NRI Container", "container id", c.ID, "name", c.Name)

	// Update pod's container mapping
	if _, ok := pod.containers[c.Name]; !ok {
		pod.containers[c.Name] = c.ID
	}

	return nil
}

// convertNriContainerState converts NRI ContainerState to internal ContainerState
func (c *container) convertNriContainerState(nriState api.ContainerState) ContainerState {
	switch nriState {
	case api.ContainerState_CONTAINER_CREATED:
		return ContainerStateCreated
	case api.ContainerState_CONTAINER_PAUSED:
		// Map paused to created since we don't have a specific paused state
		return ContainerStateCreated
	case api.ContainerState_CONTAINER_RUNNING:
		return ContainerStateRunning
	case api.ContainerState_CONTAINER_STOPPED:
		return ContainerStateExited
	default:
		return ContainerStateCreating
	}
}

// convertNriLinuxResources converts NRI LinuxResources to v1alpha1.LinuxContainerResources
func (c *container) convertNriLinuxResources(nriRes *api.LinuxResources) *v1alpha1.LinuxContainerResources {
	if nriRes == nil {
		return nil
	}

	resources := &v1alpha1.LinuxContainerResources{}

	// Convert CPU resources with value validation to prevent scope bypass
	if cpu := nriRes.GetCpu(); cpu != nil {
		if period := cpu.GetPeriod(); period != nil {
			if int64(period.Value) > 0 {
				resources.CpuPeriod = int64(period.Value)
			} else {
				klog.Warningf("Ignoring invalid CpuPeriod value in container resource conversion: %d", period.Value)
			}
		}
		if quota := cpu.GetQuota(); quota != nil {
			if quota.Value > 0 {
				resources.CpuQuota = quota.Value
			} else {
				klog.Warningf("Ignoring invalid CpuQuota value in container resource conversion: %d", quota.Value)
			}
		}
		if shares := cpu.GetShares(); shares != nil {
			if int64(shares.Value) > 0 {
				resources.CpuShares = int64(shares.Value)
			} else {
				klog.Warningf("Ignoring invalid CpuShares value in container resource conversion: %d", shares.Value)
			}
		}
		// Validate CpusetCpus and CpusetMems before assignment to avoid bypassing scope
		if cpus := cpu.GetCpus(); cpus != "" {
			resources.CpusetCpus = cpus
		}
		if mems := cpu.GetMems(); mems != "" {
			resources.CpusetMems = mems
		}
	}

	// Convert Memory resources with value validation
	if memory := nriRes.GetMemory(); memory != nil {
		if limit := memory.GetLimit(); limit != nil {
			if limit.Value > 0 {
				resources.MemoryLimitInBytes = limit.Value
			} else {
				klog.Warningf("Ignoring invalid MemoryLimitInBytes value in container resource conversion: %d", limit.Value)
			}
		}
	}

	// Note: OomScoreAdj might be available in container level
	resources.OomScoreAdj = 0

	return resources
}

// fromBasicInfo creates a container with basic information (for NRI containers)
func (c *container) fromBasicInfo(containerID string) error {
	c.ID = containerID
	c.Name = containerID // Use ID as name if no other info available
	c.State = ContainerStateCreating
	c.CacheID = containerID

	klog.InfoS("Created container from basic info", "id", containerID)
	return nil
}

func (c *container) PrettyName() string {
	if c.prettyName != "" {
		return c.prettyName
	}
	if pod, ok := c.GetPod(); !ok {
		c.prettyName = c.PodID + ":" + c.Name
	} else {
		c.prettyName = pod.GetName() + ":" + c.Name
	}
	return c.prettyName
}

func (c *container) GetPod() (Pod, bool) {
	pod, found := c.cache.Pods[c.PodID]
	return pod, found
}

func (c *container) GetID() string {
	return c.ID
}

func (c *container) GetPodID() string {
	return c.PodID
}

func (c *container) GetCacheID() string {
	return c.CacheID
}

func (c *container) GetName() string {
	return c.Name
}

func (c *container) GetNamespace() string {
	return c.Namespace
}

func (c *container) UpdateState(state ContainerState) {
	c.State = state
}

func (c *container) GetState() ContainerState {
	return c.State
}

func (c *container) GetQOSClass() v1.PodQOSClass {
	var qos v1.PodQOSClass

	if pod, found := c.GetPod(); found {
		qos = pod.GetQOSClass()
	}

	return qos
}

func (c *container) GetResourceRequirements() v1.ResourceRequirements {
	return c.Resources
}

func (c *container) GetLinuxResources() *v1alpha1.LinuxContainerResources {
	if c.LinuxReq == nil {
		return nil
	}

	return c.LinuxReq
}

func (c *container) GetCPUPeriod() int64 {
	if c.LinuxReq == nil {
		return 0
	}
	return c.LinuxReq.CpuPeriod
}

func (c *container) GetCPUQuota() int64 {
	if c.LinuxReq == nil {
		return 0
	}
	return c.LinuxReq.CpuQuota
}

func (c *container) GetCPUShares() int64 {
	if c.LinuxReq == nil {
		return 0
	}
	return c.LinuxReq.CpuShares
}

func (c *container) GetMemoryLimit() int64 {
	if c.LinuxReq == nil {
		return 0
	}
	return c.LinuxReq.MemoryLimitInBytes
}

func (c *container) GetOomScoreAdj() int64 {
	if c.LinuxReq == nil {
		return 0
	}
	return c.LinuxReq.OomScoreAdj
}

func (c *container) GetCpusetCpus() string {
	if c.LinuxReq == nil {
		return ""
	}
	return c.LinuxReq.CpusetCpus
}

func (c *container) GetCpusetMems() string {
	if c.LinuxReq == nil {
		return ""
	}
	return c.LinuxReq.CpusetMems
}

func (c *container) SetLinuxResources(req *v1alpha1.LinuxContainerResources) {
	c.LinuxReq = req
	c.markPending(CRI)
}

func (c *container) SetCPUPeriod(value int64) {
	if value < 0 {
		klog.Warningf("Ignoring invalid CpuPeriod value: %d", value)
		return
	}
	if c.LinuxReq == nil {
		c.LinuxReq = &v1alpha1.LinuxContainerResources{}
	}
	c.LinuxReq.CpuPeriod = value
	c.markPending(CRI)
}

func (c *container) SetCPUQuota(value int64) {
	if value < 0 {
		klog.Warningf("Ignoring invalid CpuQuota value: %d", value)
		return
	}
	if c.LinuxReq == nil {
		c.LinuxReq = &v1alpha1.LinuxContainerResources{}
	}
	c.LinuxReq.CpuQuota = value
	c.markPending(CRI)
}

func (c *container) SetCPUShares(value int64) {
	if value < 0 {
		klog.Warningf("Ignoring invalid CpuShares value: %d", value)
		return
	}
	if c.LinuxReq == nil {
		c.LinuxReq = &v1alpha1.LinuxContainerResources{}
	}
	c.LinuxReq.CpuShares = value
	c.markPending(CRI)
}

func (c *container) SetMemoryLimit(value int64) {
	if value < 0 {
		klog.Warningf("Ignoring invalid MemoryLimitInBytes value: %d", value)
		return
	}
	if c.LinuxReq == nil {
		c.LinuxReq = &v1alpha1.LinuxContainerResources{}
	}
	c.LinuxReq.MemoryLimitInBytes = value
	c.markPending(CRI)
}

func (c *container) SetOomScoreAdj(value int64) {
	if value < -1000 || value > 1000 {
		klog.Warningf("Ignoring out-of-range OomScoreAdj value: %d", value)
		return
	}
	if c.LinuxReq == nil {
		c.LinuxReq = &v1alpha1.LinuxContainerResources{}
	}
	c.LinuxReq.OomScoreAdj = value
	c.markPending(CRI)
}

func (c *container) SetCpusetCpus(value string) {
	if c.LinuxReq == nil {
		c.LinuxReq = &v1alpha1.LinuxContainerResources{}
	}
	c.LinuxReq.CpusetCpus = value
	c.markPending(CRI)
}

func (c *container) SetCpusetMems(value string) {
	if c.LinuxReq == nil {
		c.LinuxReq = &v1alpha1.LinuxContainerResources{}
	}
	c.LinuxReq.CpusetMems = value
	c.markPending(CRI)
}

func (c *container) markPending(controllers ...string) {
	if c.pending == nil {
		c.pending = make(map[string]struct{})
	}
	for _, ctrl := range controllers {
		c.pending[ctrl] = struct{}{}
		c.cache.markPending(c)
	}
}

func (c *container) ClearPending(controller string) {
	delete(c.pending, controller)
	if len(c.pending) == 0 {
		c.cache.clearPending(c)
	}
}

func (c *container) GetPending() []string {
	if c.pending == nil {
		return nil
	}
	pending := make([]string, 0, len(c.pending))
	for controller := range c.pending {
		pending = append(pending, controller)
	}
	sort.Strings(pending)
	return pending
}

func (c *container) HasPending(controller string) bool {
	if c.pending == nil {
		return false
	}
	_, pending := c.pending[controller]
	return pending
}

func (c *container) String() string {
	return c.PrettyName()
}
