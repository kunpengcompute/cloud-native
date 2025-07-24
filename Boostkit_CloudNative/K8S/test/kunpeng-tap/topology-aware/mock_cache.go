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
	"github.com/docker/docker/client"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/cri-api/pkg/apis/runtime/v1"
	"kunpeng.huawei.com/kunpeng-cloud-computing/api/kunpeng-tap/policy-manager/v1alpha1"
	"kunpeng.huawei.com/kunpeng-cloud-computing/pkg/kunpeng-tap/cache"
)

// MockCache implements cache.Cache interface for testing
type MockCache struct {
	containers map[string]*MockContainer
	pods       map[string]*MockPod
}

// NewMockCache creates a new mock cache
func NewMockCache() *MockCache {
	return &MockCache{
		containers: make(map[string]*MockContainer),
		pods:       make(map[string]*MockPod),
	}
}

// SetupContainerAndPod sets up a container and pod in the mock cache
func (m *MockCache) SetupContainerAndPod(containerID, podUID, containerName string) {
	pod := &MockPod{
		uid:  podUID,
		name: "test-pod",
	}
	m.pods[podUID] = pod

	container := &MockContainer{
		id:     containerID,
		name:   containerName,
		podUID: podUID,
		pod:    pod,
	}
	m.containers[containerID] = container
}

// ClearContainers clears all containers from the mock cache
func (m *MockCache) ClearContainers() {
	m.containers = make(map[string]*MockContainer)
}

// Cache interface implementation
func (m *MockCache) InsertPod(id string, msg interface{}, status *cache.PodStatus) (cache.Pod, error) {
	pod := &MockPod{uid: id, name: "test-pod"}
	m.pods[id] = pod
	return pod, nil
}

func (m *MockCache) DeletePod(id string) cache.Pod {
	pod := m.pods[id]
	delete(m.pods, id)
	if pod != nil {
		return pod
	}
	return nil
}

func (m *MockCache) LookupPod(id string) (cache.Pod, bool) {
	pod, exists := m.pods[id]
	return pod, exists
}

func (m *MockCache) InsertContainer(containerID string, msg interface{}) (cache.Container, error) {
	container := &MockContainer{id: containerID, name: "test-container"}
	m.containers[containerID] = container
	return container, nil
}

func (m *MockCache) UpdateContainerID(cacheID string, msg interface{}) (cache.Container, error) {
	if container, exists := m.containers[cacheID]; exists {
		return container, nil
	}
	return nil, nil
}

func (m *MockCache) DeleteContainer(id string) cache.Container {
	container := m.containers[id]
	delete(m.containers, id)
	if container != nil {
		return container
	}
	return nil
}

func (m *MockCache) LookupContainer(id string) (cache.Container, bool) {
	container, exists := m.containers[id]
	return container, exists
}

func (m *MockCache) GetPendingContainers() []cache.Container {
	var containers []cache.Container
	for _, container := range m.containers {
		containers = append(containers, container)
	}
	return containers
}

func (m *MockCache) GetPods() []cache.Pod {
	var pods []cache.Pod
	for _, pod := range m.pods {
		pods = append(pods, pod)
	}
	return pods
}

func (m *MockCache) GetContainers() []cache.Container {
	var containers []cache.Container
	for _, container := range m.containers {
		containers = append(containers, container)
	}
	return containers
}

func (m *MockCache) GetContainerCacheIds() []string {
	var ids []string
	for id := range m.containers {
		ids = append(ids, id)
	}
	return ids
}

func (m *MockCache) GetContainerIds() []string {
	var ids []string
	for _, container := range m.containers {
		ids = append(ids, container.GetID())
	}
	return ids
}

func (m *MockCache) RefreshPods(*v1.ListPodSandboxResponse, map[string]*cache.PodStatus) ([]cache.Pod, []cache.Pod, []cache.Container) {
	return nil, nil, nil
}

func (m *MockCache) RefreshContainers(*v1.ListContainersResponse) ([]cache.Container, []cache.Container) {
	return nil, nil
}

func (m *MockCache) GetNodeResources() []cache.NumaNodeResources {
	return []cache.NumaNodeResources{}
}

func (m *MockCache) ValidateCachedContainers(containerIds []string) []string {
	return []string{}
}

func (m *MockCache) CleanupStaleContainers(staleContainerIds []string) int {
	return 0
}

func (m *MockCache) LoadStoreDocker(dockerClient client.CommonAPIClient, cgroupDriver string) error {
	return nil
}

func (m *MockCache) LoadStoreContainerd(backendRuntimeServiceClient v1.RuntimeServiceClient) error {
	return nil
}

// MockPod implements cache.Pod interface for testing
type MockPod struct {
	uid  string
	name string
}

func (p *MockPod) String() string {
	return p.name
}

func (p *MockPod) PrettyName() string {
	return p.name
}

func (p *MockPod) GetUID() string {
	return p.uid
}

func (p *MockPod) GetName() string {
	return p.name
}

func (p *MockPod) GetNamespace() string {
	return "default"
}

func (p *MockPod) GetState() cache.PodState {
	return cache.PodStateReady
}

func (p *MockPod) GetQOSClass() corev1.PodQOSClass {
	return corev1.PodQOSBestEffort
}

func (p *MockPod) GetContainers() []cache.Container {
	return []cache.Container{}
}

func (p *MockPod) GetContainer(id string) (cache.Container, bool) {
	return nil, false
}

func (p *MockPod) GetID() string {
	return p.uid
}

func (p *MockPod) GetCacheID() string {
	return p.uid
}

func (p *MockPod) UpdateState(cache.PodState) {
}

func (p *MockPod) GetCgroupParent() string {
	return ""
}

func (p *MockPod) SetCgroupParent(string) {
}

func (p *MockPod) GetRuntimeHandler() string {
	return ""
}

func (p *MockPod) SetRuntimeHandler(string) {
}

func (p *MockPod) GetLinuxPodSandboxConfig() *v1.LinuxPodSandboxConfig {
	return nil
}

func (p *MockPod) SetLinuxPodSandboxConfig(*v1.LinuxPodSandboxConfig) {
}

func (p *MockPod) GetPodSandboxConfig() *v1.PodSandboxConfig {
	return nil
}

func (p *MockPod) SetPodSandboxConfig(*v1.PodSandboxConfig) {
}

func (p *MockPod) GetPodSandboxStatus() *v1.PodSandboxStatus {
	return nil
}

func (p *MockPod) SetPodSandboxStatus(*v1.PodSandboxStatus) {
}

func (p *MockPod) InsertContainer(cache.Container) {
}

func (p *MockPod) DeleteContainer(string) cache.Container {
	return nil
}

func (p *MockPod) GetLabels() map[string]string {
	return map[string]string{}
}

func (p *MockPod) GetAnnotations() map[string]string {
	return map[string]string{}
}

func (p *MockPod) GetCgroupParentDir() string {
	return ""
}

func (p *MockPod) GetPodResourceRequirements() corev1.ResourceRequirements {
	return corev1.ResourceRequirements{}
}

func (p *MockPod) GetLinuxResources() *v1alpha1.LinuxContainerResources {
	return nil
}

func (p *MockPod) GetInitContainers() []cache.Container {
	return []cache.Container{}
}

func (p *MockPod) GetInitContainer(string) (cache.Container, bool) {
	return nil, false
}

func (p *MockPod) InsertInitContainer(cache.Container) {
}

func (p *MockPod) DeleteInitContainer(string) cache.Container {
	return nil
}

// MockContainer implements cache.Container interface for testing
type MockContainer struct {
	id         string
	name       string
	podUID     string
	pod        *MockPod
	cpusetCpus string
}

func (c *MockContainer) String() string {
	return c.name
}

func (c *MockContainer) PrettyName() string {
	return c.name
}

func (c *MockContainer) GetPod() (cache.Pod, bool) {
	if c.pod != nil {
		return c.pod, true
	}
	return nil, false
}

func (c *MockContainer) GetID() string {
	return c.id
}

func (c *MockContainer) GetPodID() string {
	return c.podUID
}

func (c *MockContainer) GetCacheID() string {
	return c.id
}

func (c *MockContainer) GetName() string {
	return c.name
}

func (c *MockContainer) GetNamespace() string {
	return "default"
}

func (c *MockContainer) UpdateState(cache.ContainerState) {
}

func (c *MockContainer) GetState() cache.ContainerState {
	return cache.ContainerStateRunning
}

func (c *MockContainer) GetQOSClass() corev1.PodQOSClass {
	return corev1.PodQOSBestEffort
}

func (c *MockContainer) GetArgs() []string {
	return []string{}
}

func (c *MockContainer) SetArgs([]string) {
}

func (c *MockContainer) GetCommand() []string {
	return []string{}
}

func (c *MockContainer) SetCommand([]string) {
}

func (c *MockContainer) GetImage() string {
	return "test-image"
}

func (c *MockContainer) SetImage(string) {
}

func (c *MockContainer) GetEnvs() []*v1.KeyValue {
	return []*v1.KeyValue{}
}

func (c *MockContainer) SetEnvs([]*v1.KeyValue) {
}

func (c *MockContainer) GetMounts() []*v1.Mount {
	return []*v1.Mount{}
}

func (c *MockContainer) SetMounts([]*v1.Mount) {
}

func (c *MockContainer) GetDevices() []*v1.Device {
	return []*v1.Device{}
}

func (c *MockContainer) SetDevices([]*v1.Device) {
}

func (c *MockContainer) GetResourceRequirements() corev1.ResourceRequirements {
	return corev1.ResourceRequirements{}
}

func (c *MockContainer) GetLinuxResources() *v1alpha1.LinuxContainerResources {
	return nil
}

func (c *MockContainer) GetCPUPeriod() int64 {
	return 0
}

func (c *MockContainer) GetCPUQuota() int64 {
	return 0
}

func (c *MockContainer) GetCPUShares() int64 {
	return 0
}

func (c *MockContainer) GetMemoryLimit() int64 {
	return 0
}

func (c *MockContainer) GetOomScoreAdj() int64 {
	return 0
}

func (c *MockContainer) SetLinuxResources(*v1alpha1.LinuxContainerResources) {
	// Mock implementation - no-op for testing
}

func (c *MockContainer) SetCPUPeriod(int64) {
	// Mock implementation - no-op for testing
}

func (c *MockContainer) SetCPUQuota(int64) {
	// Mock implementation - no-op for testing
}

func (c *MockContainer) SetCPUShares(int64) {
	// Mock implementation - no-op for testing
}

func (c *MockContainer) SetMemoryLimit(int64) {
	// Mock implementation - no-op for testing
}

func (c *MockContainer) GetLinuxContainerConfig() *v1.LinuxContainerConfig {
	return nil
}

func (c *MockContainer) SetLinuxContainerConfig(*v1.LinuxContainerConfig) {
}

func (c *MockContainer) GetContainerConfig() *v1.ContainerConfig {
	return nil
}

func (c *MockContainer) SetContainerConfig(*v1.ContainerConfig) {
}

func (c *MockContainer) GetContainerStatus() *v1.ContainerStatus {
	return nil
}

func (c *MockContainer) SetContainerStatus(*v1.ContainerStatus) {
}

// Remove TopologyHints methods as they are not defined in cache package

func (c *MockContainer) GetCpusetCpus() string {
	return c.cpusetCpus
}

func (c *MockContainer) SetCpusetCpus(cpus string) {
	c.cpusetCpus = cpus
}

func (c *MockContainer) GetCpusetMems() string {
	return ""
}

func (c *MockContainer) SetCpusetMems(string) {
	// Mock implementation - no-op for testing
}

func (c *MockContainer) ClearPending(string) {
	// Mock implementation - no-op for testing
}

func (c *MockContainer) GetPending() []string {
	return []string{}
}

func (c *MockContainer) HasPending(string) bool {
	return false
}

// Simplified mock implementation - only implement essential methods for testing
