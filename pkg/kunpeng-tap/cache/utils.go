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
	"context"
	"encoding/json"
	"strings"

	dockertypes "github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	corev1 "k8s.io/api/core/v1"
	resapi "k8s.io/apimachinery/pkg/api/resource"
	criv1 "k8s.io/cri-api/pkg/apis/runtime/v1"
	"k8s.io/klog/v2"

	"kunpeng.huawei.com/kunpeng-cloud-computing/api/kunpeng-tap/policy-manager/v1alpha1"
	"kunpeng.huawei.com/kunpeng-cloud-computing/pkg/kunpeng-tap/server"
	"kunpeng.huawei.com/kunpeng-cloud-computing/pkg/kunpeng-tap/server/docker/utils"
)

const (
	// Constants for converting back and forth between CPU requirements in
	// terms of milli-CPUs and kernel cgroup/scheduling parameters.

	// MinShares is the minimum cpu.shares accepted by cgroups.
	MinShares = 2
	// MaxShares is the minimum cpu.shares accepted by cgroups.
	MaxShares = 262144
	// SharesPerCPU is cpu.shares worth one full CPU.
	SharesPerCPU = 1024
	// MilliCPUToCPU is milli-CPUs worth a full CPU.
	MilliCPUToCPU = 1000
	// QuotaPeriod is 100000 microseconds, or 100ms
	QuotaPeriod = 100000
	// MinQuotaPeriod is 1000 microseconds, or 1ms
	MinQuotaPeriod = 1000

	// CgroupParent: Info["info"]["runtimeSpec"]["annotations"][crioCgroupParent]
	crioCgroupParent = "io.kubernetes.cri-o.CgroupParent"
)

// sharesToMilliCPU converts CFS CPU shares to milli-CPUs.
func sharesToMilliCPU(shares int64) int64 {
	if shares == MinShares {
		return 0
	}
	return int64(float64(shares*MilliCPUToCPU)/float64(SharesPerCPU) + 0.5)
}

// quotaToMilliCPU converts CFS quota and period to milli-CPUs.
func quotaToMilliCPU(quota, period int64) int64 {
	if quota == 0 || period == 0 {
		return 0
	}
	return int64(float64(quota*MilliCPUToCPU)/float64(period) + 0.5)
}

// SharesToMilliCPU converts CFS CPU shares to milliCPU.
func SharesToMilliCPU(shares int64) int64 {
	return sharesToMilliCPU(shares)
}

// QuotaToMilliCPU converts CFS quota and period to milliCPU.
func QuotaToMilliCPU(quota, period int64) int64 {
	return quotaToMilliCPU(quota, period)
}

// cgroupParentToQOS tries to map Pod cgroup parent to QOS class.
func cgroupParentToQoS(dir string) corev1.PodQOSClass {
	var qos corev1.PodQOSClass

	// The parent directory naming scheme depends on the cgroup driver in use.
	// Thus, rely on substring matching
	split := strings.Split(strings.TrimPrefix(dir, "/"), "/")
	switch {
	case len(split) < 2:
		qos = corev1.PodQOSClass("")
	case strings.Contains(split[1], strings.ToLower(string(corev1.PodQOSBurstable))):
		qos = corev1.PodQOSBurstable
	case strings.Contains(split[1], strings.ToLower(string(corev1.PodQOSBestEffort))):
		qos = corev1.PodQOSBestEffort
	default:
		qos = corev1.PodQOSGuaranteed
	}

	return qos
}

func EstimateRequirements(res *v1alpha1.LinuxContainerResources, cgroupParent string) *corev1.ResourceRequirements {
	var qos corev1.PodQOSClass

	resources := &corev1.ResourceRequirements{
		Requests: corev1.ResourceList{},
		Limits:   corev1.ResourceList{},
	}

	if res == nil {
		klog.V(0).InfoS("Linux container resources not found", "error", "resources is nil")
		return resources
	}
	if cgroupParent != "" {
		qos = cgroupParentToQoS(cgroupParent)
	}

	// calculate CPU request
	if value := sharesToMilliCPU(res.CpuShares); value > 0 {
		qty := resapi.NewMilliQuantity(value, resapi.DecimalSI)
		resources.Requests[corev1.ResourceCPU] = *qty
	}
	// get memory limit
	if value := res.MemoryLimitInBytes; value > 0 {
		qty := resapi.NewQuantity(value, resapi.DecimalSI)
		resources.Limits[corev1.ResourceMemory] = *qty
	}

	// set or calculate CPU limit, set memory request if known
	if qos == corev1.PodQOSGuaranteed {
		resources.Limits[corev1.ResourceCPU] = resources.Requests[corev1.ResourceCPU]
		resources.Requests[corev1.ResourceMemory] = resources.Limits[corev1.ResourceMemory]
	} else {
		if value := QuotaToMilliCPU(res.CpuQuota, res.CpuPeriod); value > 0 {
			qty := resapi.NewMilliQuantity(value, resapi.DecimalSI)
			resources.Limits[corev1.ResourceCPU] = *qty
		}
	}

	return resources
}

// estimateComputeResources calculates resource requests/limits from a CRI request.
func estimateComputeResources(lnx *criv1.LinuxContainerResources) corev1.ResourceRequirements {
	var qos corev1.PodQOSClass

	resources := corev1.ResourceRequirements{
		Requests: corev1.ResourceList{},
		Limits:   corev1.ResourceList{},
	}

	if lnx == nil {
		return resources
	}

	// default QOS as BestEffort
	qos = corev1.PodQOSBestEffort

	// calculate CPU request
	if value := sharesToMilliCPU(lnx.CpuShares); value > 0 {
		qty := resapi.NewMilliQuantity(value, resapi.DecimalSI)
		resources.Requests[corev1.ResourceCPU] = *qty
	}

	// get memory limit
	if value := lnx.MemoryLimitInBytes; value > 0 {
		qty := resapi.NewQuantity(value, resapi.DecimalSI)
		resources.Limits[corev1.ResourceMemory] = *qty
	}

	// set or calculate CPU limit, set memory request if known
	if qos == corev1.PodQOSGuaranteed {
		resources.Limits[corev1.ResourceCPU] = resources.Requests[corev1.ResourceCPU]
		resources.Requests[corev1.ResourceMemory] = resources.Limits[corev1.ResourceMemory]
	} else {
		if value := quotaToMilliCPU(lnx.CpuQuota, lnx.CpuPeriod); value > 0 {
			qty := resapi.NewMilliQuantity(value, resapi.DecimalSI)
			resources.Limits[corev1.ResourceCPU] = *qty
		}
	}

	return resources
}

// ValidateCachedContainers checks if containers in cache still exist in the system
// by comparing with the list of running containers from Docker API, and returns a list of container IDs that
// should be removed from the cache
func (cch *cache) ValidateCachedContainers(allContainerIds []string) []string {
	cch.Lock()
	defer cch.Unlock()

	var staleContainers []string

	// Create a map of container IDs for quick lookup
	containerMap := make(map[string]bool)
	for _, containerId := range allContainerIds {
		containerMap[containerId] = true
	}

	// Check all containers in the cache
	for _, container := range cch.Containers {
		found := false

		// First check for exact ID match
		if containerMap[container.ID] {
			found = true
		} else {
			// If no exact match, check for prefix matches (handling shortened IDs)
			for _, dockerContainerId := range allContainerIds {
				if strings.HasPrefix(dockerContainerId, container.ID) || strings.HasPrefix(container.ID, dockerContainerId) {
					found = true
					break
				}
			}
		}

		// If container not found in running containers, mark it for removal
		if !found {
			podID := container.PodID
			klog.V(3).InfoS("Found stale container in cache", "containerID", container.ID, "podID", podID)
			staleContainers = append(staleContainers, container.ID)
		}
	}

	return staleContainers
}

// CleanupStaleContainers removes containers from cache that no longer exist in the system
func (cch *cache) CleanupStaleContainers(staleContainerIds []string) int {
	count := 0

	// Remove stale containers from cache
	for _, id := range staleContainerIds {
		cch.DeleteContainer(id)
		count++
	}

	klog.V(3).InfoS("Cleaned up stale containers", "count", count)
	return count
}

func (cch *cache) LoadStoreDocker(dockerClient client.CommonAPIClient, cgroupDriver string) error {

	type dockerWrapper struct {
		dockertypes.ContainerJSON
		dockertypes.Container
	}
	sandboxes := []dockerWrapper{}
	containers := []dockerWrapper{}
	cs, err := dockerClient.ContainerList(context.TODO(), dockertypes.ContainerListOptions{All: false})
	if err != nil {
		klog.ErrorS(err, "Failed to get container list during failover")
		return err
	}
	for _, c := range cs {
		containerJson, err := dockerClient.ContainerInspect(context.TODO(), c.ID)
		if err != nil {
			klog.ErrorS(err, "Failed to get container details", "containerId", c.ID)
			continue
		}
		runtimeResourceType := utils.GetRuntimeResourceType(c.Labels)
		if runtimeResourceType == server.RuntimeContainerResource {
			containers = append(containers, dockerWrapper{
				ContainerJSON: containerJson,
				Container:     c,
			})
		} else {
			sandboxes = append(sandboxes, dockerWrapper{
				ContainerJSON: containerJson,
				Container:     c,
			})
		}
	}

	// need to backup pod meta first
	for _, s := range sandboxes {
		labels, annos := utils.SplitLabelsAndAnnotations(s.Labels)
		podInfo := &v1alpha1.PodSandboxHookRequest{
			Labels:       labels,
			Annotations:  annos,
			CgroupParent: utils.ToCriCgroupPath(cgroupDriver, s.ContainerJSON.HostConfig.CgroupParent),
			PodMeta: &v1alpha1.PodSandboxMetadata{
				Name: s.Name,
				Uid:  s.ID,
			},
			RuntimeHandler: "Docker",
			Resources:      utils.HostConfigToResource(s.ContainerJSON.HostConfig),
		}
		cch.InsertPod(s.ID, podInfo, nil)
	}

	for _, c := range containers {
		_, annos := utils.SplitLabelsAndAnnotations(c.Labels)
		cInfo := &v1alpha1.ContainerResourceHookRequest{
			ContainerResources:   utils.HostConfigToResource(c.ContainerJSON.HostConfig),
			ContainerAnnotations: annos,
			ContainerMeta: &v1alpha1.ContainerMetadata{
				Name: c.Name,
				Id:   c.ID,
			},
		}
		podID := c.Labels[utils.SandboxIDLabelKey]

		podCheckPoint, exit := cch.LookupPod(podID)
		if exit {
			cInfo.PodMeta = &v1alpha1.PodSandboxMetadata{
				Name:      podCheckPoint.GetName(),
				Uid:       podCheckPoint.GetUID(),
				Namespace: podCheckPoint.GetNamespace(),
			}
			cInfo.PodResources = podCheckPoint.GetLinuxResources()

		}

		if c.Config != nil {
			cInfo.ContainerEnvs = utils.SplitDockerEnv(c.Config.Env)
		}
		cch.InsertContainer(cInfo.ContainerMeta.Id, cInfo)
	}
	return nil
}

type SandboxInfo struct {
	Config *criv1.PodSandboxConfig `json:"config"`
	// Note: RuntimeSpec may not be populated if the sandbox has not been fully created.
	RuntimeSpec *specs.Spec `json:"runtimeSpec"`
}

type ContainerInfo struct {
	Config *criv1.ContainerConfig `json:"config"`
	// Note: RuntimeSpec may not be populated if the sandbox has not been fully created.
	RuntimeSpec *specs.Spec `json:"runtimeSpec"`
}

// Helper method to process a single pod sandbox
func (cch *cache) processPodSandbox(containerdClient criv1.RuntimeServiceClient, pod *criv1.PodSandbox) error {
	podStatus, err := containerdClient.PodSandboxStatus(context.TODO(), &criv1.PodSandboxStatusRequest{
		PodSandboxId: pod.GetId(),
		Verbose:      true,
	})
	if err != nil {
		klog.ErrorS(err, "Failed to get pod details", "podId", pod.GetId())
		return err
	}
	klog.V(3).InfoS("Get pod status", "PodStatus", podStatus)
	// Parse pod info from status
	podInfoInStatus := &SandboxInfo{}
	if err := json.Unmarshal([]byte(podStatus.Info["info"]), podInfoInStatus); err != nil {
		klog.ErrorS(err, "Failed to get pod details", "podId", pod.GetId())
		return err
	}
	klog.V(3).InfoS("Get pod info", "PodInfo", podInfoInStatus)

	podInfo := &v1alpha1.PodSandboxHookRequest{
		Labels:      pod.GetLabels(),
		Annotations: pod.GetAnnotations(),
		PodMeta: &v1alpha1.PodSandboxMetadata{
			Name:      pod.GetMetadata().GetName(),
			Namespace: pod.GetMetadata().GetNamespace(),
			Uid:       pod.GetMetadata().GetUid(),
		},
		RuntimeHandler: pod.GetRuntimeHandler(),
		Resources:      RuntimeSpecToResource(podInfoInStatus.RuntimeSpec),
	}
	// Get CgroupParent from pod info
	if podInfoInStatus.Config != nil && podInfoInStatus.Config.Linux != nil {
		podInfo.CgroupParent = podInfoInStatus.Config.Linux.CgroupParent
	} else if podInfoInStatus.RuntimeSpec != nil {
		podInfo.CgroupParent = podInfoInStatus.RuntimeSpec.Annotations[crioCgroupParent]
	}
	klog.V(3).InfoS("Get CgroupParent from pod info", "CgroupParent", podInfo.CgroupParent)

	cch.InsertPod(pod.GetId(), podInfo, nil)
	return nil
}

// Helper method to process a single container
func (cch *cache) processContainer(containerdClient criv1.RuntimeServiceClient, container *criv1.Container) error {
	containerStatus, err := containerdClient.ContainerStatus(context.TODO(), &criv1.ContainerStatusRequest{
		ContainerId: container.GetId(),
		Verbose:     true,
	})
	if err != nil {
		klog.ErrorS(err, "Failed to get container details", "Container", container.GetId())
		return err
	}
	// Parse container info from status
	containerInfoInStatus := &ContainerInfo{}
	klog.V(3).InfoS("Get container info", "PodInfo", containerInfoInStatus)
	if err := json.Unmarshal([]byte(containerStatus.Info["info"]), containerInfoInStatus); err != nil {
		klog.ErrorS(err, "Failed to get container details", "podId", container.GetId())
		return err
	}

	cInfo := &v1alpha1.ContainerResourceHookRequest{
		ContainerResources:   RuntimeSpecToResource(containerInfoInStatus.RuntimeSpec),
		ContainerAnnotations: container.GetAnnotations(),
		ContainerMeta: &v1alpha1.ContainerMetadata{
			Name: container.GetMetadata().GetName(),
			Id:   container.GetId(),
		},
	}
	// Get pod info from cache
	podCheckPoint, exit := cch.LookupPod(container.GetPodSandboxId())
	if exit {
		cInfo.PodMeta = &v1alpha1.PodSandboxMetadata{
			Name:      podCheckPoint.GetName(),
			Uid:       podCheckPoint.GetUID(),
			Namespace: podCheckPoint.GetNamespace(),
			Id:        container.GetPodSandboxId(),
		}
		cInfo.PodResources = podCheckPoint.GetLinuxResources()
	}

	// Get container envs from container info
	if containerInfoInStatus.Config != nil {
		cInfo.ContainerEnvs = ContainerdEnvConvert(containerInfoInStatus.Config.Envs)
	}

	cch.InsertContainer(cInfo.ContainerMeta.Id, cInfo)

	// Set the correct container state from the CRI response.
	// InsertContainer -> fromDockerRunRequest hardcodes ContainerStateCreating,
	// but we need the actual runtime state for proper state filtering.
	cch.SetContainerState(cInfo.ContainerMeta.Id, ContainerState(int32(container.GetState())))
	return nil
}

func (cch *cache) LoadStoreContainerd(containerdClient criv1.RuntimeServiceClient) error {

	podResponse := &criv1.ListPodSandboxResponse{}
	podResponse, err := containerdClient.ListPodSandbox(context.TODO(), &criv1.ListPodSandboxRequest{})
	if err != nil {
		return err
	}

	for _, pod := range podResponse.Items {
		if err := cch.processPodSandbox(containerdClient, pod); err != nil {
			// Log error but continue processing other pods
			continue
		}
	}

	// Only list running containers, consistent with Docker's All: false behavior.
	// Exited containers should not be cached as their stale cpusets would
	// incorrectly consume resource pool capacity during state rebuild.
	var containerResponse *criv1.ListContainersResponse
	containerResponse, err = containerdClient.ListContainers(context.TODO(), &criv1.ListContainersRequest{
		Filter: &criv1.ContainerFilter{
			State: &criv1.ContainerStateValue{
				State: criv1.ContainerState_CONTAINER_RUNNING,
			},
		},
	})
	if err != nil {
		return err
	}

	for _, container := range containerResponse.Containers {
		if err := cch.processContainer(containerdClient, container); err != nil {
			// Log error but continue processing other containers
			continue
		}
	}

	return nil
}

func ContainerdEnvConvert(containerdEnvs []*criv1.KeyValue) map[string]string {
	env := make(map[string]string)

	for _, containerdEnv := range containerdEnvs {
		if containerdEnv != nil {
			env[containerdEnv.Key] = containerdEnv.Value
		}
	}

	return env
}

// Helper method to process CPU resources from runtime spec
func processCPUResources(linuxContainerResources *v1alpha1.LinuxContainerResources, cpu *specs.LinuxCPU) {
	if cpu.Period != nil {
		linuxContainerResources.CpuPeriod = int64(*cpu.Period)
	}
	if cpu.Shares != nil {
		linuxContainerResources.CpuShares = int64(*cpu.Shares)
	}
	if cpu.Quota != nil {
		linuxContainerResources.CpuQuota = *cpu.Quota
	}
	linuxContainerResources.CpusetCpus = cpu.Cpus
	linuxContainerResources.CpusetMems = cpu.Mems
}

// Helper method to process memory resources from runtime spec
func processMemoryResources(linuxContainerResources *v1alpha1.LinuxContainerResources, memory *specs.LinuxMemory) {
	if memory.Limit != nil {
		linuxContainerResources.MemoryLimitInBytes = *memory.Limit
	}
	if memory.Swap != nil {
		linuxContainerResources.MemorySwapLimitInBytes = *memory.Swap
	}
}

func RuntimeSpecToResource(runtimeSpec *specs.Spec) *v1alpha1.LinuxContainerResources {

	if runtimeSpec == nil || runtimeSpec.Linux == nil || runtimeSpec.Linux.Resources == nil {
		return nil
	}

	linuxContainerResources := &v1alpha1.LinuxContainerResources{}
	if runtimeSpec.Linux.Resources.CPU != nil {
		processCPUResources(linuxContainerResources, runtimeSpec.Linux.Resources.CPU)
	}

	if runtimeSpec.Linux.Resources.Memory != nil {
		processMemoryResources(linuxContainerResources, runtimeSpec.Linux.Resources.Memory)
	}

	if runtimeSpec.Process != nil && runtimeSpec.Process.OOMScoreAdj != nil {
		linuxContainerResources.OomScoreAdj = int64(*runtimeSpec.Process.OOMScoreAdj)
	}

	return linuxContainerResources
}
