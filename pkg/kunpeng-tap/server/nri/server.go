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

// Package nri implements the NRI plugin server
package nri

import (
	"context"
	"fmt"
	"strings"

	"github.com/containerd/nri/pkg/api"
	"github.com/containerd/nri/pkg/stub"
	"k8s.io/klog/v2"

	"kunpeng.huawei.com/kunpeng-cloud-computing/api/kunpeng-tap/policy-manager/v1alpha1"
	"kunpeng.huawei.com/kunpeng-cloud-computing/pkg/kunpeng-tap/cache"
	"kunpeng.huawei.com/kunpeng-cloud-computing/pkg/kunpeng-tap/policy"
	"kunpeng.huawei.com/kunpeng-cloud-computing/pkg/kunpeng-tap/server"
)

const (
	// PluginName is the name of the NRI plugin
	PluginName = "kunpeng-tap"
	// PluginIdx is the index of the NRI plugin
	PluginIdx = "00"
)

// Agent implements both the NRI plugin interface and server.ProxyServer interface
type Agent struct {
	cache       cache.Cache
	hookManager policy.HookManager
	stub        stub.Stub
	mask        api.EventMask
	socketPath  string
}

// NewNriAgent creates a new NRI server instance
func NewNriAgent(cache cache.Cache, hookManager policy.HookManager, socketPath string) (server.ProxyServer, error) {
	klog.InfoS("Creating NRI server", "socketPath", socketPath)

	plugin := &Agent{
		cache:       cache,
		hookManager: hookManager,
		mask: api.MustParseEventMask("RunPodSandbox,StopPodSandbox,RemovePodSandbox,CreateContainer," +
			"PostCreateContainer,StartContainer,StopContainer,UpdateContainer,RemoveContainer"),
		socketPath: socketPath,
	}

	// Initialize and configure stub
	opts := []stub.Option{
		stub.WithPluginName(PluginName),
		stub.WithPluginIdx(PluginIdx),
	}

	// Add socket path option if specified
	if plugin.socketPath != "" {
		klog.InfoS("Using custom NRI socket path", "path", plugin.socketPath)
		opts = append(opts, stub.WithSocketPath(plugin.socketPath))
	}

	var err error
	plugin.stub, err = stub.New(plugin, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create NRI plugin stub: %w", err)
	}

	return plugin, nil
}

// Run starts the NRI server
func (p *Agent) Run() error {
	klog.InfoS("Starting NRI server")
	if err := p.start(); err != nil {
		klog.InfoS("", "Failed to start NRI plugin", err)
		return err
	}
	return nil
}

// Shutdown gracefully shuts down the NRI server
func (p *Agent) Shutdown(ctx context.Context) {
	klog.InfoS("Shutting down NRI server")
	if p.stub != nil {
		p.stub.Stop()
	}
}

// start starts the NRI plugin
func (p *Agent) start() error {
	if p.stub == nil {
		return fmt.Errorf("NRI plugin stub is not initialized")
	}

	err := p.stub.Start(context.Background())
	if err != nil {
		return fmt.Errorf("failed to start NRI plugin: %w", err)
	}

	return nil
}

// Configure is called when the plugin is being configured
func (p *Agent) Configure(ctx context.Context, config, runtime, version string) (stub.EventMask, error) {
	klog.InfoS("Configuring NRI plugin", "config", config, "runtime", runtime, "version", version)

	// Return the events we want to subscribe to
	return p.mask, nil
}

// Synchronize is called to synchronize the plugin state with the runtime
func (p *Agent) Synchronize(ctx context.Context, pods []*api.PodSandbox,
	containers []*api.Container) ([]*api.ContainerUpdate, error) {
	klog.InfoS("Synchronizing NRI plugin", "pods", len(pods), "containers", len(containers))

	// Synchronize cache with current runtime state
	p.synchronizePods(pods)
	p.synchronizeContainers(containers)

	// Trigger resource state rebuild from cached containers
	// This ensures topology-aware policy is aware of existing container NUMA placements
	if rebuilder, ok := p.hookManager.(policy.StateSynchronizer); ok {
		if err := rebuilder.RebuildAllocationsFromCache(); err != nil {
			klog.ErrorS(err, "Failed to rebuild allocations from cache")
		} else {
			klog.InfoS("Successfully rebuilt allocations from cache")
		}
	}

	return nil, nil
}

// synchronizePods synchronizes pods with the cache
func (p *Agent) synchronizePods(pods []*api.PodSandbox) {
	for _, pod := range pods {
		if pod == nil {
			continue
		}
		klog.InfoS("Synchronizing pod", "id", pod.Id, "name", pod.Name)
		// Insert pod into cache if not exists
		if _, found := p.cache.LookupPod(pod.Id); !found {
			_, err := p.cache.InsertPod(pod.Id, pod, &cache.PodStatus{
				CgroupParent: "", // Will be extracted from pod data
			})
			if err != nil {
				klog.ErrorS(err, "Failed to insert pod during synchronization", "id", pod.Id)
			}
		}
	}
}

// synchronizeContainers synchronizes containers with the cache
func (p *Agent) synchronizeContainers(containers []*api.Container) {
	for _, container := range containers {
		if container == nil {
			continue
		}
		cpuset := ""
		if container.Linux != nil && container.Linux.Resources != nil && container.Linux.Resources.Cpu != nil {
			cpuset = container.Linux.Resources.Cpu.Cpus
		}
		klog.InfoS("Synchronizing container", "id", container.Id, "name", container.Name, "container's cpuset", cpuset)
		// Insert container into cache if not exists
		if _, found := p.cache.LookupContainer(container.Id); !found {
			_, err := p.cache.InsertContainer(container.Id, container)
			if err != nil {
				klog.ErrorS(err, "Failed to insert container during synchronization", "id", container.Id)
			}
		}
	}
}

// RunPodSandbox is called when a pod sandbox is being started
func (p *Agent) RunPodSandbox(ctx context.Context, pod *api.PodSandbox) error {
	klog.InfoS("Running pod sandbox", "podID", pod.Id, "podName", pod.Name, "namespace", pod.Namespace)

	// Insert pod into cache
	_, err := p.cache.InsertPod(pod.Id, pod, &cache.PodStatus{
		CgroupParent: p.extractCgroupParent(pod),
	})
	if err != nil {
		klog.ErrorS(err, "Failed to insert pod into cache", "podID", pod.Id)
		return err
	}

	klog.InfoS("Pod sandbox successfully added to cache", "podID", pod.Id)
	return nil
}

// StopPodSandbox is called when a pod sandbox is being stopped
func (p *Agent) StopPodSandbox(ctx context.Context, pod *api.PodSandbox) error {
	klog.InfoS("Stopping pod sandbox", "podID", pod.Id, "podName", pod.Name, "namespace", pod.Namespace)

	// Pod stop logic can be added here if needed
	// For now, we just log the event
	klog.InfoS("Pod sandbox stop event processed", "podID", pod.Id)
	return nil
}

// RemovePodSandbox is called when a pod sandbox is being removed
func (p *Agent) RemovePodSandbox(ctx context.Context, pod *api.PodSandbox) error {
	klog.InfoS("Removing pod sandbox", "podID", pod.Id, "podName", pod.Name, "namespace", pod.Namespace)

	// Remove pod from cache
	removedPod := p.cache.DeletePod(pod.Id)
	if removedPod != nil {
		klog.InfoS("Pod sandbox successfully removed from cache", "podID", pod.Id)
	} else {
		klog.InfoS("Pod sandbox not found in cache during removal", "podID", pod.Id)
	}

	return nil
}

// CreateContainer is called when a container is being created
func (p *Agent) CreateContainer(ctx context.Context, pod *api.PodSandbox,
	container *api.Container) (*api.ContainerAdjustment, []*api.ContainerUpdate, error) {
	klog.InfoS("Creating container",
		"containerID", container.Id,
		"containerName", container.Name,
		"podID", pod.Id,
		"podName", pod.Name)

	// Ensure pod is in cache first for hook manager to access
	if _, found := p.cache.LookupPod(pod.Id); !found {
		klog.InfoS("Pod not found in cache, inserting pod first", "podID", pod.Id)
		_, err := p.cache.InsertPod(pod.Id, pod, &cache.PodStatus{
			CgroupParent: p.extractCgroupParent(pod),
		})
		if err != nil {
			klog.ErrorS(err, "Failed to insert pod into cache during CreateContainer", "podID", pod.Id)
			return nil, nil, err
		}
	}

	// Convert NRI request to hook request format
	hookReq, err := p.convertToHookRequest(pod, container)
	if err != nil {
		klog.ErrorS(err, "Failed to convert NRI request to hook request")
		return nil, nil, err
	}

	// Call the pre-create container hook
	klog.InfoS("Calling PreCreateContainerHook", "containerID", container.Id, "hookRequest", hookReq)
	hookResp, err := p.hookManager.PreCreateContainerHook(ctx, hookReq)
	if err != nil {
		klog.ErrorS(err, "PreCreateContainerHook failed")
		return nil, nil, err
	}

	klog.InfoS("PreCreateContainerHook response", "containerID", container.Id, "hookResponse", hookResp)

	// Convert hook response to NRI container adjustment
	adjustment := p.convertToCtrAdjustment(hookResp)

	// Insert container into cache after successful hook processing
	_, err = p.cache.InsertContainer(container.Id, container)
	if err != nil {
		// Don't return error here as the hook processing was successful
		// The container will be inserted again in PostCreateContainer if needed
		klog.ErrorS(err, "Failed to insert container into cache during CreateContainer", "containerID", container.Id)
	} else {
		klog.InfoS("Container successfully added to cache during CreateContainer", "containerID", container.Id)
	}

	klog.InfoS("Container creation adjustment", "containerID", container.Id, "adjustment", adjustment)
	return adjustment, nil, nil
}

// PostCreateContainer is called after a container has been created
func (p *Agent) PostCreateContainer(ctx context.Context, pod *api.PodSandbox, container *api.Container) error {
	klog.InfoS("Post-create container", "containerID", container.Id, "containerName", container.Name, "podID", pod.Id)

	// Container should already be in cache from CreateContainer
	// Just log the event for now
	klog.InfoS("Container post-create event processed", "containerID", container.Id)
	return nil
}

// StartContainer is called when a container is being started
func (p *Agent) StartContainer(ctx context.Context, pod *api.PodSandbox, container *api.Container) error {
	klog.InfoS("Starting container", "containerID", container.Id, "containerName", container.Name, "podID", pod.Id)

	// Container start logic can be added here if needed
	// For now, we just log the event
	klog.InfoS("Container start event processed", "containerID", container.Id)
	return nil
}

// StopContainer is called when a container is being stopped
func (p *Agent) StopContainer(ctx context.Context, pod *api.PodSandbox,
	container *api.Container) ([]*api.ContainerUpdate, error) {
	klog.InfoS("Stopping container", "containerID", container.Id, "containerName", container.Name, "podID", pod.Id)

	// Convert NRI request to hook request format
	hookReq, err := p.convertToHookRequest(pod, container)
	if err != nil {
		klog.ErrorS(err, "Failed to convert NRI request to hook request for stop")
		return nil, err
	}

	// Call the post-stop container hook
	klog.InfoS("Calling PostStopContainerHook", "containerID", container.Id, "hookRequest", hookReq)
	_, err = p.hookManager.PostStopContainerHook(ctx, hookReq)
	if err != nil {
		klog.ErrorS(err, "PostStopContainerHook failed")
		return nil, err
	}

	klog.InfoS("PostStopContainerHook completed successfully", "containerID", container.Id)

	return nil, nil
}

// UpdateContainer is called when a container is being updated
func (p *Agent) UpdateContainer(ctx context.Context, pod *api.PodSandbox,
	container *api.Container, resources *api.LinuxResources) ([]*api.ContainerUpdate, error) {
	klog.InfoS("Updating container", "containerID", container.Id, "containerName", container.Name, "podID", pod.Id)

	// For now, we don't need to perform any operations during container updates
	// Just log the event and return no updates
	klog.InfoS("Container update event processed", "containerID", container.Id)
	return nil, nil
}

// RemoveContainer is called when a container is being removed
func (p *Agent) RemoveContainer(ctx context.Context, pod *api.PodSandbox, container *api.Container) error {
	klog.InfoS("Removing container", "containerID", container.Id, "containerName", container.Name, "podID", pod.Id)

	// Convert NRI request to hook request format for removal
	hookReq, err := p.convertToHookRequest(pod, container)
	if err != nil {
		klog.ErrorS(err, "Failed to convert NRI request to hook request for removal")
		return err
	}

	// Call the pre-remove container hook
	_, err = p.hookManager.PreRemoveContainerHook(ctx, hookReq)
	if err != nil {
		klog.ErrorS(err, "PreRemoveContainerHook failed")
		return err
	}

	// Remove container from cache
	removedContainer := p.cache.DeleteContainer(container.Id)
	if removedContainer != nil {
		klog.InfoS("Container successfully removed from cache", "containerID", container.Id)
	} else {
		klog.InfoS("Container not found in cache during removal", "containerID", container.Id)
	}

	return nil
}

// convertToHookRequest converts NRI pod and container to hook request format
func (p *Agent) convertToHookRequest(pod *api.PodSandbox,
	container *api.Container) (*v1alpha1.ContainerResourceHookRequest, error) {
	// Create hook request with basic metadata
	hookReq := &v1alpha1.ContainerResourceHookRequest{
		PodMeta:              p.convertPodMetadata(pod),
		ContainerMeta:        p.convertContainerMetadata(container),
		ContainerResources:   p.convertContainerResources(container),
		ContainerAnnotations: container.Annotations,
		ContainerEnvs:        p.convertEnvironmentVariables(container.Env),
		PodCgroupParent:      p.extractCgroupParent(pod),
		PodAnnotations:       pod.Annotations,
		PodLabels:            pod.Labels,
		PodResources:         p.convertPodResources(pod),
	}

	return hookReq, nil
}

// convertToCtrAdjustment converts hook response to NRI container adjustment
func (p *Agent) convertToCtrAdjustment(hookResp *v1alpha1.ContainerResourceHookResponse) *api.ContainerAdjustment {
	if hookResp == nil {
		return nil
	}

	adjustment := &api.ContainerAdjustment{}

	// Convert container resources using helper methods
	if hookResp.ContainerResources != nil {
		p.setLinuxResources(adjustment, hookResp.ContainerResources)
	}

	// Convert annotations
	if hookResp.ContainerAnnotations != nil {
		adjustment.Annotations = hookResp.ContainerAnnotations
	}

	// Convert environment variables
	if hookResp.ContainerEnvs != nil {
		for key, value := range hookResp.ContainerEnvs {
			adjustment.AddEnv(key, value)
		}
	}

	return adjustment
}

// setLinuxResources sets Linux resources using NRI adjustment helper methods
func (p *Agent) setLinuxResources(adjustment *api.ContainerAdjustment,
	resources *v1alpha1.LinuxContainerResources) {
	// Set CPU resources with value validation to prevent scope bypass
	if resources.CpuPeriod > 0 {
		adjustment.SetLinuxCPUPeriod(resources.CpuPeriod)
	} else if resources.CpuPeriod != 0 {
		klog.Warningf("Ignoring invalid CpuPeriod value: %d", resources.CpuPeriod)
	}
	if resources.CpuQuota > 0 {
		adjustment.SetLinuxCPUQuota(resources.CpuQuota)
	} else if resources.CpuQuota != 0 {
		klog.Warningf("Ignoring invalid CpuQuota value: %d", resources.CpuQuota)
	}
	if resources.CpuShares > 0 {
		adjustment.SetLinuxCPUShares(uint64(resources.CpuShares))
	} else if resources.CpuShares != 0 {
		klog.Warningf("Ignoring invalid CpuShares value: %d", resources.CpuShares)
	}
	if resources.CpusetCpus != "" {
		adjustment.SetLinuxCPUSetCPUs(resources.CpusetCpus)
	}
	if resources.CpusetMems != "" {
		adjustment.SetLinuxCPUSetMems(resources.CpusetMems)
	}

	// Set Memory resources with value validation
	if resources.MemoryLimitInBytes > 0 {
		adjustment.SetLinuxMemoryLimit(resources.MemoryLimitInBytes)
	} else if resources.MemoryLimitInBytes != 0 {
		klog.Warningf("Ignoring invalid MemoryLimitInBytes value: %d", resources.MemoryLimitInBytes)
	}

	// Set OOM score adjustment with range validation (valid range: -1000 to 1000)
	if resources.OomScoreAdj != 0 {
		if resources.OomScoreAdj < -1000 || resources.OomScoreAdj > 1000 {
			klog.Warningf("Ignoring out-of-range OomScoreAdj value: %d", resources.OomScoreAdj)
		} else {
			oomScore := int(resources.OomScoreAdj)
			adjustment.SetLinuxOomScoreAdj(&oomScore)
		}
	}
}

// convertPodMetadata converts NRI PodSandbox to PodSandboxMetadata
func (p *Agent) convertPodMetadata(pod *api.PodSandbox) *v1alpha1.PodSandboxMetadata {
	return &v1alpha1.PodSandboxMetadata{
		Name:      pod.Name,
		Namespace: pod.Namespace,
		Uid:       pod.Uid,
		Id:        pod.Id,
	}
}

// convertContainerMetadata converts NRI Container to ContainerMetadata
func (p *Agent) convertContainerMetadata(container *api.Container) *v1alpha1.ContainerMetadata {
	return &v1alpha1.ContainerMetadata{
		Name: container.Name,
		Id:   container.Id,
	}
}

// convertContainerResources converts NRI container resources to LinuxContainerResources
func (p *Agent) convertContainerResources(container *api.Container) *v1alpha1.LinuxContainerResources {
	if container.Linux == nil || container.Linux.Resources == nil {
		return nil
	}

	resources := container.Linux.Resources
	containerResources := &v1alpha1.LinuxContainerResources{}

	// Extract CPU resources
	if cpu := resources.GetCpu(); cpu != nil {
		p.extractCPUResources(cpu, containerResources)
	}

	// Extract Memory resources
	if memory := resources.GetMemory(); memory != nil {
		p.extractMemoryResources(memory, containerResources)
	}

	// OomScoreAdj is typically set at container level, not in resources
	containerResources.OomScoreAdj = 0

	return containerResources
}

// extractCPUResources extracts CPU resources from NRI LinuxCPU
func (p *Agent) extractCPUResources(cpu *api.LinuxCPU,
	containerResources *v1alpha1.LinuxContainerResources) {
	if period := cpu.GetPeriod(); period != nil {
		containerResources.CpuPeriod = int64(period.Value)
	}
	if quota := cpu.GetQuota(); quota != nil {
		containerResources.CpuQuota = quota.Value
	}
	if shares := cpu.GetShares(); shares != nil {
		containerResources.CpuShares = int64(shares.Value)
	}
	// Validate CpusetCpus and CpusetMems before assignment to avoid bypassing scope
	if cpus := cpu.GetCpus(); cpus != "" {
		containerResources.CpusetCpus = cpus
	}
	if mems := cpu.GetMems(); mems != "" {
		containerResources.CpusetMems = mems
	}
}

// extractMemoryResources extracts memory resources from NRI LinuxMemory
func (p *Agent) extractMemoryResources(memory *api.LinuxMemory,
	containerResources *v1alpha1.LinuxContainerResources) {
	if limit := memory.GetLimit(); limit != nil {
		containerResources.MemoryLimitInBytes = limit.Value
	}
}

// convertEnvironmentVariables converts NRI environment variables to map[string]string
func (p *Agent) convertEnvironmentVariables(envVars []string) map[string]string {
	envMap := make(map[string]string)
	for _, env := range envVars {
		// NRI env variables are in "KEY=VALUE" format
		if len(env) > 0 {
			// Split on first '=' to handle values that contain '='
			if idx := strings.Index(env, "="); idx > 0 {
				key := env[:idx]
				value := env[idx+1:]
				envMap[key] = value
			} else {
				// Environment variable without value
				envMap[env] = ""
			}
		}
	}
	return envMap
}

// extractCgroupParent extracts cgroup parent from NRI PodSandbox
func (p *Agent) extractCgroupParent(pod *api.PodSandbox) string {
	if pod.Linux != nil {
		return pod.Linux.CgroupParent
	}
	return ""
}

// convertPodResources converts NRI pod resources to LinuxContainerResources
func (p *Agent) convertPodResources(pod *api.PodSandbox) *v1alpha1.LinuxContainerResources {
	if pod.Linux == nil || pod.Linux.Resources == nil {
		return nil
	}

	resources := pod.Linux.Resources
	podResources := &v1alpha1.LinuxContainerResources{}

	// Extract CPU resources
	if cpu := resources.GetCpu(); cpu != nil {
		if period := cpu.GetPeriod(); period != nil {
			podResources.CpuPeriod = int64(period.Value)
		}
		if quota := cpu.GetQuota(); quota != nil {
			podResources.CpuQuota = quota.Value
		}
		if shares := cpu.GetShares(); shares != nil {
			podResources.CpuShares = int64(shares.Value)
		}
		podResources.CpusetCpus = cpu.GetCpus()
		podResources.CpusetMems = cpu.GetMems()
	}

	// Extract Memory resources
	if memory := resources.GetMemory(); memory != nil {
		if limit := memory.GetLimit(); limit != nil {
			podResources.MemoryLimitInBytes = limit.Value
		}
	}

	// OomScoreAdj for pod level
	podResources.OomScoreAdj = 0

	return podResources
}
