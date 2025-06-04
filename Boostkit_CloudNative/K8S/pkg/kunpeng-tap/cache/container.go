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

package cache

import (
	"sort"

	"kunpeng.huawei.com/kunpeng-cloud-computing/api/kunpeng-tap/policy-manager/v1alpha1"

	v1 "k8s.io/api/core/v1"
	criv1 "k8s.io/cri-api/pkg/apis/runtime/v1"
	"k8s.io/klog/v2"
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
	if c.LinuxReq == nil {
		c.LinuxReq = &v1alpha1.LinuxContainerResources{}
	}
	c.LinuxReq.CpuPeriod = value
	c.markPending(CRI)
}

func (c *container) SetCPUQuota(value int64) {
	if c.LinuxReq == nil {
		c.LinuxReq = &v1alpha1.LinuxContainerResources{}
	}
	c.LinuxReq.CpuQuota = value
	c.markPending(CRI)
}

func (c *container) SetCPUShares(value int64) {
	if c.LinuxReq == nil {
		c.LinuxReq = &v1alpha1.LinuxContainerResources{}
	}
	c.LinuxReq.CpuShares = value
	c.markPending(CRI)
}

func (c *container) SetMemoryLimit(value int64) {
	if c.LinuxReq == nil {
		c.LinuxReq = &v1alpha1.LinuxContainerResources{}
	}
	c.LinuxReq.MemoryLimitInBytes = value
	c.markPending(CRI)
}

func (c *container) SetOomScoreAdj(value int64) {
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
