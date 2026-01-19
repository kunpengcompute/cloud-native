/*
 * Copyright 2022 The Koordinator Authors.
 * Copyright 2025 Huawei Technology corp.
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

package policy

import (
	"context"
	"sync"

	"google.golang.org/grpc"
	"k8s.io/klog/v2"
	"kunpeng.huawei.com/kunpeng-cloud-computing/api/kunpeng-tap/policy-manager/v1alpha1"
	"kunpeng.huawei.com/kunpeng-cloud-computing/cmd/kunpeng-tap/proxy/options"
	"kunpeng.huawei.com/kunpeng-cloud-computing/pkg/kunpeng-tap/cache"
)

// PolicyOptions contains configuration options for policies
type PolicyOptions struct {
	// EnableMemoryTopology controls whether memory topology awareness is enabled
	EnableMemoryTopology bool
	// ResourcePriority controls the resource allocation priority strategy
	ResourcePriority string
	// EnableClusterAffinity controls whether cluster-level CPU affinity is enabled
	// This feature is only effective on 950 model machines
	EnableClusterAffinity bool
}

// NewPolicyOptions creates a new PolicyOptions with default values
func NewPolicyOptions() *PolicyOptions {
	return &PolicyOptions{
		EnableMemoryTopology:  options.EnableMemoryTopology,
		ResourcePriority:      options.ResourcePriorityCPUFirst,
		EnableClusterAffinity: options.EnableClusterAffinity,
	}
}

// HookContext provides context information for policy hooks
type HookContext interface {
	// FromProxy converts proxy request to hook context
	FromProxy(req interface{})
}

// policyManager implements PolicyManager interface
type policyManager struct {
	mu       sync.RWMutex
	policies map[string]Policy
	cache    cache.Cache
}

var (
	singleton *policyManager
	once      sync.Once
)

// NewPolicyManagerWithPolicies creates a new policy manager with a list of policies
func NewPolicyManagerWithPolicies(cache cache.Cache, policies []Policy) PolicyManager {
	once.Do(func() {
		policiesMap := make(map[string]Policy)
		for _, policy := range policies {
			policiesMap[policy.Name()] = policy
		}
		singleton = &policyManager{
			mu:       sync.RWMutex{},
			policies: policiesMap,
			cache:    cache,
		}
	})
	return singleton
}

// NewPolicyManager creates a new policy manager or returns the existing singleton
func NewPolicyManager(cache cache.Cache) PolicyManager {
	once.Do(func() {
		singleton = &policyManager{
			mu:       sync.RWMutex{},
			policies: make(map[string]Policy),
			cache:    cache,
		}
	})
	return singleton
}

// RegisterPolicy registers a policy with the manager
func (pm *policyManager) RegisterPolicy(policy Policy) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	policyName := policy.Name()
	if _, exists := pm.policies[policyName]; exists {
		klog.InfoS("Policy already registered", "policy", policyName)
		return
	}

	policy.SetCache(pm.cache)
	pm.policies[policyName] = policy
	klog.InfoS("Registered policy", "policy", policyName)
}

// GetPolicy returns a policy by name
func (pm *policyManager) GetPolicy(name string) Policy {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return pm.policies[name]
}

// ListPolicies returns all registered policies
func (pm *policyManager) ListPolicies() []Policy {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	result := make([]Policy, 0, len(pm.policies))
	for _, policy := range pm.policies {
		result = append(result, policy)
	}
	return result
}

func (pm *policyManager) SetCache(cache cache.Cache) {
	if pm.cache != nil {
		klog.InfoS("PolicyManager cache was already set")
		return
	}
	pm.cache = cache
}

func (pm *policyManager) GetCache() cache.Cache {
	return pm.cache
}

// PreRunPodSandboxHook calls RuntimeHookServer before pod creating, and would merge RunPodSandboxHookResponse
// and Original RunPodSandboxRequest generating a new RunPodSandboxRequest to transfer to backend runtime engine.
// RuntimeHookServer should ensure the correct operations basing on RunPodSandboxHookRequest.
func (pm *policyManager) PreRunPodSandboxHook(ctx context.Context, req *v1alpha1.PodSandboxHookRequest, opts ...grpc.CallOption) (*v1alpha1.PodSandboxHookResponse, error) {
	klog.V(5).InfoS("Received PreRunPodSandboxHook request", "request", req.String())
	resp := &v1alpha1.PodSandboxHookResponse{
		Annotations:  req.GetAnnotations(),
		Labels:       req.GetLabels(),
		CgroupParent: req.GetCgroupParent(),
		Resources:    req.GetResources(),
	}
	klog.V(5).InfoS("Sending PreRunPodSandboxHook response", "pod", req.PodMeta, "response", resp)
	return resp, nil
}

// PostStopPodSandboxHook calls RuntimeHookServer after pod deleted. RuntimeHookServer could do resource setting garbage collection
// sanity check after PodSandBox stopped.
func (pm *policyManager) PostStopPodSandboxHook(ctx context.Context, req *v1alpha1.PodSandboxHookRequest, opts ...grpc.CallOption) (*v1alpha1.PodSandboxHookResponse, error) {
	klog.V(5).InfoS("Received PostStopPodSandboxHook request", "request", req)
	resp := &v1alpha1.PodSandboxHookResponse{
		Annotations:  req.GetAnnotations(),
		Labels:       req.GetLabels(),
		CgroupParent: req.GetCgroupParent(),
		Resources:    req.GetResources(),
	}
	klog.V(5).InfoS("Sending PostStopPodSandboxHook response", "pod", req.PodMeta, "response", resp)
	return resp, nil
}

func (pm *policyManager) PreRemovePodSandboxHook(ctx context.Context, req *v1alpha1.PodSandboxHookRequest, opts ...grpc.CallOption) (*v1alpha1.PodSandboxHookResponse, error) {
	klog.V(5).InfoS("Received PreRemovePodSandboxHook request", "request", req)
	resp := &v1alpha1.PodSandboxHookResponse{
		Annotations:  req.GetAnnotations(),
		Labels:       req.GetLabels(),
		CgroupParent: req.GetCgroupParent(),
		Resources:    req.GetResources(),
	}
	klog.V(5).InfoS("Sending PreRemovePodSandboxHook response", "pod", req.PodMeta, "response", resp)
	return resp, nil
}

// PreCreateContainerHook calls all policies before container creation
func (pm *policyManager) PreCreateContainerHook(ctx context.Context, req *v1alpha1.ContainerResourceHookRequest, opts ...grpc.CallOption) (*v1alpha1.ContainerResourceHookResponse, error) {
	klog.V(5).InfoS("Starting PreCreateContainer Hook")

	resp := &v1alpha1.ContainerResourceHookResponse{
		ContainerAnnotations: req.GetContainerAnnotations(),
		ContainerResources:   req.GetContainerResources(),
		PodCgroupParent:      req.GetPodCgroupParent(),
		ContainerEnvs:        req.GetContainerEnvs(),
	}

	containerCtx := &ContainerContext{}
	containerCtx.FromProxy(req)

	// Execute the hook for each registered policy
	var allocs []*Allocation
	for _, policy := range pm.ListPolicies() {
		alloc, err := policy.PreCreateContainerHook(containerCtx)
		if err != nil {
			klog.ErrorS(err, "Policy hook execution failed", "policy", policy.Name(), "hook", "PreCreateContainer")
			continue
		}

		if alloc != nil {
			allocs = append(allocs, alloc)
		}
	}

	// Merge all allocations into the response
	mergeAllocations(allocs, resp)

	klog.V(5).InfoS("Sending PreCreateContainerHook response", "pod", req.PodMeta, "container", req.ContainerMeta, "response", resp)
	return resp, nil
}

// PreStartContainerHook calls RuntimeHookServer before container starting, RuntimeHookServer could do some
// resource adjustments before container launching.
func (pm *policyManager) PreStartContainerHook(ctx context.Context, req *v1alpha1.ContainerResourceHookRequest, opts ...grpc.CallOption) (*v1alpha1.ContainerResourceHookResponse, error) {
	klog.V(5).InfoS("Received PreStartContainerHook request", "request", req)
	resp := &v1alpha1.ContainerResourceHookResponse{
		ContainerAnnotations: req.GetContainerAnnotations(),
		ContainerResources:   req.GetContainerResources(),
		PodCgroupParent:      req.GetPodCgroupParent(),
		ContainerEnvs:        req.GetContainerEnvs(),
	}
	klog.V(5).InfoS("Sending PreStartContainerHook response",
		"pod", req.PodMeta,
		"container", req.ContainerMeta,
		"response", resp)
	return resp, nil
}

// PostStartContainerHook calls RuntimeHookServer after container starting. RuntimeHookServer could do resource
// set checking after container launch.
func (pm *policyManager) PostStartContainerHook(ctx context.Context, req *v1alpha1.ContainerResourceHookRequest, opts ...grpc.CallOption) (*v1alpha1.ContainerResourceHookResponse, error) {
	klog.V(5).InfoS("Received PostStartContainerHook request", "request", req)
	resp := &v1alpha1.ContainerResourceHookResponse{
		ContainerAnnotations: req.GetContainerAnnotations(),
		ContainerResources:   req.GetContainerResources(),
		PodCgroupParent:      req.GetPodCgroupParent(),
		ContainerEnvs:        req.GetContainerEnvs(),
	}
	klog.V(5).InfoS("Sending PostStartContainerHook response",
		"pod", req.PodMeta,
		"container", req.ContainerMeta,
		"response", resp)
	return resp, nil
}

// PostStopContainerHook calls RuntimeHookServer after container stop. RuntimeHookServer could do resource setting
// garbage collection.
func (pm *policyManager) PostStopContainerHook(ctx context.Context, req *v1alpha1.ContainerResourceHookRequest, opts ...grpc.CallOption) (*v1alpha1.ContainerResourceHookResponse, error) {
	klog.V(5).InfoS("Received PostStopContainerHook request", "request", req)
	resp := &v1alpha1.ContainerResourceHookResponse{
		ContainerAnnotations: req.GetContainerAnnotations(),
		ContainerResources:   req.GetContainerResources(),
		PodCgroupParent:      req.GetPodCgroupParent(),
		ContainerEnvs:        req.GetContainerEnvs(),
	}

	containerCtx := &ContainerContext{
		Request: ContainerRequest{
			ContainerMeta: ContainerMeta{
				ID: req.ContainerMeta.Id,
			},
		},
	}

	// Execute the hook for each registered policy
	var allocs []*Allocation
	for _, policy := range pm.ListPolicies() {
		alloc, err := policy.PostStopContainerHook(containerCtx)
		if err != nil {
			klog.ErrorS(err, "Policy hook execution failed", "policy", policy.Name(), "hook", "PostStopContainer")
			continue
		}

		if alloc != nil {
			allocs = append(allocs, alloc)
		}
	}

	// Merge all allocations into the response
	mergeAllocations(allocs, resp)

	klog.V(5).InfoS("Sending PostStopContainerHook response",
		"container", req.ContainerMeta,
		"response", resp)
	return resp, nil
}

func (pm *policyManager) PreRemoveContainerHook(ctx context.Context, req *v1alpha1.ContainerResourceHookRequest, opts ...grpc.CallOption) (*v1alpha1.ContainerResourceHookResponse, error) {
	klog.V(5).InfoS("Received PreRemoveContainerHook request", "request", req)
	resp := &v1alpha1.ContainerResourceHookResponse{
		ContainerAnnotations: req.GetContainerAnnotations(),
		ContainerResources:   req.GetContainerResources(),
		PodCgroupParent:      req.GetPodCgroupParent(),
		ContainerEnvs:        req.GetContainerEnvs(),
	}

	containerCtx := &ContainerContext{
		Request: ContainerRequest{
			ContainerMeta: ContainerMeta{
				ID: req.ContainerMeta.Id,
			},
		},
	}

	// Execute the hook for each registered policy
	var allocs []*Allocation
	for _, policy := range pm.ListPolicies() {
		alloc, err := policy.PostStopContainerHook(containerCtx)
		if err != nil {
			klog.ErrorS(err, "Policy hook execution failed", "policy", policy.Name(), "hook", "PreRemoveContainerHook")
			continue
		}

		if alloc != nil {
			allocs = append(allocs, alloc)
		}
	}

	// Merge all allocations into the response
	mergeAllocations(allocs, resp)

	klog.V(5).InfoS("Sending PreRemoveContainerHook response",
		"pod", req.PodMeta,
		"container", req.ContainerMeta,
		"response", resp)
	return resp, nil
}

// PreUpdateContainerResourcesHook calls RuntimeHookServer before container resource update to keep resource policy
// consistent
func (pm *policyManager) PreUpdateContainerResourcesHook(ctx context.Context, req *v1alpha1.ContainerResourceHookRequest, opts ...grpc.CallOption) (*v1alpha1.ContainerResourceHookResponse, error) {
	klog.V(5).InfoS("Received PreUpdateContainerResourcesHook request", "request", req)
	resp := &v1alpha1.ContainerResourceHookResponse{
		ContainerAnnotations: req.GetContainerAnnotations(),
		ContainerResources:   req.GetContainerResources(),
		PodCgroupParent:      req.GetPodCgroupParent(),
		ContainerEnvs:        req.GetContainerEnvs(),
	}
	klog.V(5).InfoS("Sending PreUpdateContainerResourcesHook response",
		"pod", req.PodMeta,
		"container", req.ContainerMeta,
		"response", resp)
	return resp, nil
}

type Allocation struct {
	Resources *v1alpha1.LinuxContainerResources
}

func NewAllocation() *Allocation {
	return &Allocation{
		Resources: &v1alpha1.LinuxContainerResources{},
	}
}

func (all *Allocation) SetCPUSetCpus(cpuSet string) {
	all.Resources.CpusetCpus = cpuSet
}

func (all *Allocation) SetCPUSetMems(mems string) {
	all.Resources.CpusetMems = mems
}

// Helper method to ensure ContainerResources is initialized
func (all *Allocation) ensureContainerResources(resp *v1alpha1.ContainerResourceHookResponse) {
	if resp.ContainerResources == nil {
		resp.ContainerResources = &v1alpha1.LinuxContainerResources{}
	}
}

// Helper method to merge CPU-related resources
func (all *Allocation) mergeCPUResources(resp *v1alpha1.ContainerResourceHookResponse) {
	if all.Resources.CpuPeriod != 0 {
		all.ensureContainerResources(resp)
		resp.ContainerResources.CpuPeriod = all.Resources.CpuPeriod
	}
	if all.Resources.CpuQuota != 0 {
		all.ensureContainerResources(resp)
		resp.ContainerResources.CpuQuota = all.Resources.CpuQuota
	}
	if all.Resources.CpuShares != 0 {
		all.ensureContainerResources(resp)
		resp.ContainerResources.CpuShares = all.Resources.CpuShares
	}
}

// Helper method to merge CPU set resources
func (all *Allocation) mergeCPUSetResources(resp *v1alpha1.ContainerResourceHookResponse) {
	if all.Resources.CpusetMems != "" {
		all.ensureContainerResources(resp)
		resp.ContainerResources.CpusetMems = all.Resources.CpusetMems
	}
	if all.Resources.CpusetCpus != "" {
		all.ensureContainerResources(resp)
		resp.ContainerResources.CpusetCpus = all.Resources.CpusetCpus
	}
}

// Helper method to merge memory-related resources
func (all *Allocation) mergeMemoryResources(resp *v1alpha1.ContainerResourceHookResponse) {
	if all.Resources.MemoryLimitInBytes != 0 {
		all.ensureContainerResources(resp)
		resp.ContainerResources.MemoryLimitInBytes = all.Resources.MemoryLimitInBytes
	}
	if all.Resources.MemorySwapLimitInBytes != 0 {
		all.ensureContainerResources(resp)
		resp.ContainerResources.MemorySwapLimitInBytes = all.Resources.MemorySwapLimitInBytes
	}
}

func (all *Allocation) Merge(resp *v1alpha1.ContainerResourceHookResponse) {
	klog.V(5).InfoS("Merging allocation response", "response", resp)
	if all.Resources == nil || resp == nil {
		return
	}
	all.mergeCPUResources(resp)
	all.mergeCPUSetResources(resp)
	all.mergeMemoryResources(resp)
}
