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

package dispatcher

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"github.com/docker/docker/api/types/container"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1"
	"k8s.io/klog/v2"

	"kunpeng.huawei.com/kunpeng-cloud-computing/api/kunpeng-tap/policy-manager/v1alpha1"
	"kunpeng.huawei.com/kunpeng-cloud-computing/pkg/kunpeng-tap/cache"
	"kunpeng.huawei.com/kunpeng-cloud-computing/pkg/kunpeng-tap/monitoring"
	"kunpeng.huawei.com/kunpeng-cloud-computing/pkg/kunpeng-tap/policy"
	"kunpeng.huawei.com/kunpeng-cloud-computing/pkg/kunpeng-tap/server"
	"kunpeng.huawei.com/kunpeng-cloud-computing/pkg/kunpeng-tap/server/docker/utils"
)

type Dispatcher interface {
	SetDockerCgroupDriver(dockerCgroupDriver string)
	InterceptRuntimeRequest(ctx context.Context, hookType policy.HookType, request interface{}, labels map[string]string, handler grpc.UnaryHandler) (interface{}, error)
	ParseContainerRequest(req interface{}) interface{}
	ParsePodRequest(req interface{}) interface{}
	Dispatch(ctx context.Context, hookType policy.HookType, request interface{}, labels map[string]string) interface{}
	InsertIntoCacheIfNeed(criResp, hookReq interface{})
	DeleteFromCacheIfNeed(request interface{})
	BackfillRequest(proxyReq, hookReq, hookResp interface{})
}

type dispatcher struct {
	hookManager        policy.HookManager
	cache              cache.Cache
	dockerCgroupDriver string
}

func NewDispatcher(hookmanager policy.HookManager, cache cache.Cache) Dispatcher {
	return &dispatcher{
		hookManager: hookmanager,
		cache:       cache,
	}
}

func (d *dispatcher) SetDockerCgroupDriver(dockerCgroupDriver string) {
	d.dockerCgroupDriver = dockerCgroupDriver
}

func (d *dispatcher) InterceptRuntimeRequest(ctx context.Context, hookType policy.HookType, request interface{}, labels map[string]string, handler grpc.UnaryHandler) (interface{}, error) {
	runtimeResourceType := GetResourceTypeFromHookType(hookType)

	var hookRequest interface{}
	switch runtimeResourceType {
	case server.RuntimeContainerResource:
		hookRequest = d.ParseContainerRequest(request)
	case server.RuntimePodResource:
		hookRequest = d.ParsePodRequest(request)
	}
	defer d.DeleteFromCacheIfNeed(request)

	var hookResponse interface{}
	if hookRequest != nil {
		klog.V(5).InfoS("Send request to hook manager", "HookRequest", hookRequest, "HookType", hookType)
		hookResponse = d.Dispatch(ctx, hookType, hookRequest, labels)
		if hookResponse != nil {
			d.BackfillRequest(request, hookRequest, hookResponse)
			klog.V(5).InfoS("Backfill hook response to request", "HookResponse", hookResponse, "ProxyRequest", request, "HookType", hookType)
		}
	}

	klog.V(5).InfoS("Send request to container runtime", "Request", request)
	// If the hook response is nil, it means that the hook is not enabled or not implemented.
	resp, err := handler(ctx, request)
	if err != nil {
		klog.ErrorS(err, "fail to call container runtime")
		return nil, err
	}

	if hookRequest != nil {
		d.InsertIntoCacheIfNeed(resp, hookRequest)
	}

	return resp, err
}

func (d *dispatcher) Dispatch(ctx context.Context, hookType policy.HookType, request interface{}, labels map[string]string) interface{} {
	if SkipTAPHandle(labels) {
		klog.V(5).InfoS("Skip sending request to PreRunPodSandboxHook", "podReq", request)
		return nil
	}

	klog.V(5).InfoS("Send request to "+string(hookType), "podReq", request)
	hookStartTime := time.Now()
	hookResp, err := dispatchInternal(ctx, hookType, d.hookManager, request)
	if err != nil {
		klog.ErrorS(err, "Error from "+string(hookType))
	}

	hookDuration := time.Since(hookStartTime).Seconds()
	monitoring.HookRequestDuration.WithLabelValues(string(hookType)).Observe(hookDuration)

	return hookResp
}

// InsertIntoCacheIfNeed insert hook request into cache.
// containerRuntimeResp is used to get the pod/container id.
func (d *dispatcher) InsertIntoCacheIfNeed(containerRuntimeResp, hookReq interface{}) {
	var err error
	switch response := containerRuntimeResp.(type) {
	case *runtimeapi.CreateContainerResponse:
		containerId := response.GetContainerId()
		klog.V(5).InfoS("Insert container into cache", "ContainerId", containerId)
		_, err = d.cache.InsertContainer(containerId, hookReq)
	case *runtimeapi.RunPodSandboxResponse:
		podId := response.PodSandboxId
		klog.V(5).InfoS("Insert pod into cache", "PodId", podId)
		_, err = d.cache.InsertPod(podId, hookReq, nil)
	case *container.ContainerCreateCreatedBody:
		containerId := response.ID
		switch hookReq := hookReq.(type) {
		case *v1alpha1.ContainerResourceHookRequest:
			klog.V(5).InfoS("Insert container into cache", "ContainerId", containerId)
			_, err = d.cache.InsertContainer(containerId, hookReq)
		case *v1alpha1.PodSandboxHookRequest:
			klog.V(5).InfoS("Insert pod into cache", "PodId", containerId)
			_, err = d.cache.InsertPod(containerId, hookReq, nil)
		}
	default:
		klog.V(5).InfoS("InsertIntoCacheIfNeed Unknown response type", "ContainerRuntimeRespType", reflect.TypeOf(containerRuntimeResp).String())
	}

	if err != nil {
		klog.ErrorS(err, "Failed to insert into cache")
	}
}

func (d *dispatcher) DeleteFromCacheIfNeed(request interface{}) {
	switch request := request.(type) {
	case *runtimeapi.StopContainerRequest:
		klog.V(5).InfoS("Delete container from cache", "ContainerId", request.GetContainerId())
		d.cache.DeleteContainer(request.GetContainerId())
	case *runtimeapi.StopPodSandboxRequest:
		klog.V(5).InfoS("Delete pod from cache", "PodId", request.GetPodSandboxId())
		d.cache.DeletePod(request.GetPodSandboxId())
	default:
		klog.V(5).InfoS("DeleteFromCacheIfNeed Unknown request type", "RequestType", reflect.TypeOf(request).String())
	}
}

func dispatchInternal(ctx context.Context, hookType policy.HookType,
	hookManager policy.HookManager, request interface{}) (response interface{}, err error) {
	switch hookType {
	case policy.PreRunPodSandbox:
		return hookManager.PreRunPodSandboxHook(ctx, request.(*v1alpha1.PodSandboxHookRequest))
	case policy.PostStopPodSandbox:
		return hookManager.PostStopPodSandboxHook(ctx, request.(*v1alpha1.PodSandboxHookRequest))
	case policy.PreRemovePodSandbox:
		return hookManager.PreRemovePodSandboxHook(ctx, request.(*v1alpha1.PodSandboxHookRequest))
	case policy.PreCreateContainer:
		return hookManager.PreCreateContainerHook(ctx, request.(*v1alpha1.ContainerResourceHookRequest))
	case policy.PreStartContainer:
		return hookManager.PreStartContainerHook(ctx, request.(*v1alpha1.ContainerResourceHookRequest))
	case policy.PreUpdateContainerResources:
		return hookManager.PreUpdateContainerResourcesHook(ctx, request.(*v1alpha1.ContainerResourceHookRequest))
	case policy.PostStartContainer:
		return hookManager.PostStartContainerHook(ctx, request.(*v1alpha1.ContainerResourceHookRequest))
	case policy.PostStopContainer:
		return hookManager.PostStopContainerHook(ctx, request.(*v1alpha1.ContainerResourceHookRequest))
	}
	return nil, status.Errorf(codes.Unimplemented, "method %v not implemented", string(hookType))
}

func (d *dispatcher) BackfillRequest(proxyReq, hookReq, hookResp interface{}) {
	if hookResp == nil {
		return
	}

	switch response := hookResp.(type) {
	case *v1alpha1.PodSandboxHookResponse:
		if response != nil {
			BackfillPodRequest(proxyReq, hookReq, response, d.dockerCgroupDriver)
		}
	case *v1alpha1.ContainerResourceHookResponse:
		if response != nil {
			BackfillContainerRequest(proxyReq, hookReq, response, d.dockerCgroupDriver)
		}
	}
}

// BackfillPodRequest fill proxy pod request and hook pod request by hook pod response
func BackfillPodRequest(proxyPodReq interface{}, hookPodReq interface{}, hookPodResponse *v1alpha1.PodSandboxHookResponse, dockerCgroupDriver string) {
	switch proxyRequest := proxyPodReq.(type) {
	case *runtimeapi.RunPodSandboxRequest:
		if proxyRequest.Config == nil {
			proxyRequest.Config = &runtimeapi.PodSandboxConfig{}
		}
		if hookPodResponse.Annotations != nil {
			proxyRequest.Config.Annotations = hookPodResponse.Annotations
			hookPodReq.(*v1alpha1.PodSandboxHookRequest).Annotations = hookPodResponse.Annotations
		}
		if hookPodResponse.Labels != nil {
			proxyRequest.Config.Labels = hookPodResponse.Labels
			hookPodReq.(*v1alpha1.PodSandboxHookRequest).Labels = hookPodResponse.Labels
		}
		if hookPodResponse.CgroupParent != "" {
			proxyRequest.Config.Linux.CgroupParent = hookPodResponse.CgroupParent
			hookPodReq.(*v1alpha1.PodSandboxHookRequest).CgroupParent = hookPodResponse.CgroupParent
		}
		if hookPodResponse.Resources != nil {
			proxyRequest.Config.Linux.Resources = TransferToCRIResources(hookPodResponse.Resources)
			hookPodReq.(*v1alpha1.PodSandboxHookRequest).Resources = hookPodResponse.Resources
		}
	case *utils.ConfigWrapper:
		if hookPodResponse.Resources != nil {
			proxyRequest.HostConfig = utils.UpdateHostConfigByResource(proxyRequest.HostConfig, hookPodResponse.Resources)
			hookPodReq.(*v1alpha1.PodSandboxHookRequest).Resources = hookPodResponse.Resources
		}
		if hookPodResponse.CgroupParent != "" {
			proxyRequest.HostConfig.CgroupParent = utils.GenerateExpectedCgroupParent(dockerCgroupDriver, hookPodResponse.CgroupParent)
			hookPodReq.(*v1alpha1.PodSandboxHookRequest).CgroupParent = hookPodResponse.CgroupParent
		}
	}
}

// BackfillContainerRequest fill proxy container request and hook container request by hook container response
func BackfillContainerRequest(proxyContainerReq interface{}, hookContainerReq interface{}, hookContainerResponse *v1alpha1.ContainerResourceHookResponse, dockerCgroupDriver string) {
	switch proxyRequest := proxyContainerReq.(type) {
	case *runtimeapi.CreateContainerRequest:
		if proxyRequest.Config == nil {
			proxyRequest.Config = &runtimeapi.ContainerConfig{}
		}
		if hookContainerResponse.ContainerAnnotations != nil {
			proxyRequest.Config.Annotations = hookContainerResponse.ContainerAnnotations
			hookContainerReq.(*v1alpha1.ContainerResourceHookRequest).ContainerAnnotations = hookContainerResponse.ContainerAnnotations
		}
		if hookContainerResponse.ContainerResources != nil {
			proxyRequest.Config.Linux.Resources = TransferToCRIResources(hookContainerResponse.ContainerResources)
			hookContainerReq.(*v1alpha1.ContainerResourceHookRequest).ContainerResources = hookContainerResponse.ContainerResources
		}
		if hookContainerResponse.PodCgroupParent != "" {
			proxyRequest.SandboxConfig.Linux.CgroupParent = hookContainerResponse.PodCgroupParent
			hookContainerReq.(*v1alpha1.ContainerResourceHookRequest).PodCgroupParent = hookContainerResponse.PodCgroupParent
		}
		proxyRequest.Config.Envs = TransferToCRIContainerEnvs(hookContainerResponse.GetContainerEnvs())
		hookContainerReq.(*v1alpha1.ContainerResourceHookRequest).ContainerEnvs = hookContainerResponse.GetContainerEnvs()
	case *utils.ConfigWrapper:
		if hookContainerResponse.ContainerResources != nil {
			proxyRequest.HostConfig = utils.UpdateHostConfigByResource(proxyRequest.HostConfig, hookContainerResponse.ContainerResources)
			hookContainerReq.(*v1alpha1.ContainerResourceHookRequest).ContainerResources = hookContainerResponse.ContainerResources
		}
		if hookContainerResponse.PodCgroupParent != "" {
			proxyRequest.HostConfig.CgroupParent = utils.GenerateExpectedCgroupParent(dockerCgroupDriver, hookContainerResponse.PodCgroupParent)
			hookContainerReq.(*v1alpha1.ContainerResourceHookRequest).PodCgroupParent = hookContainerResponse.PodCgroupParent
		}
		if hookContainerResponse.ContainerEnvs != nil {
			proxyRequest.Env = utils.GenerateEnvList(hookContainerResponse.ContainerEnvs)
			hookContainerReq.(*v1alpha1.ContainerResourceHookRequest).ContainerEnvs = hookContainerResponse.ContainerEnvs
		}
	}
}

func (d *dispatcher) ParseContainerRequest(req interface{}) interface{} {
	switch request := req.(type) {
	case *runtimeapi.CreateContainerRequest:
		// get the pod info from local store
		podID := request.GetPodSandboxId()
		podInfo, exist := d.cache.LookupPod(podID)
		if !exist {
			err := fmt.Errorf("fail to get pod(%v) related to container", podID)
			klog.Errorf("fail to parse request %v %v", request, err)
			return nil
		}
		return &v1alpha1.ContainerResourceHookRequest{
			PodMeta: &v1alpha1.PodSandboxMetadata{
				Name:      podInfo.GetName(),
				Uid:       podInfo.GetUID(),
				Namespace: podInfo.GetNamespace(),
				Id:        podInfo.GetID(),
			},
			PodResources: podInfo.GetLinuxResources(),
			ContainerMeta: &v1alpha1.ContainerMetadata{
				Name:    request.GetConfig().GetMetadata().GetName(),
				Attempt: request.GetConfig().GetMetadata().GetAttempt(),
			},
			ContainerAnnotations: request.GetConfig().GetAnnotations(),
			ContainerResources:   TransferToHookResources(request.GetConfig().GetLinux().GetResources()),
			PodAnnotations:       podInfo.GetAnnotations(),
			PodLabels:            podInfo.GetLabels(),
			PodCgroupParent:      podInfo.GetCgroupParentDir(),
			ContainerEnvs:        TransferCRIContainerEnvsToMap(request.GetConfig().GetEnvs()),
		}
	case utils.DockerCreateRequest:
		podID := request.ConfigWrapper.Config.Labels[utils.SandboxIDLabelKey]
		podInfo, exist := d.cache.LookupPod(podID)
		if !exist {
			err := fmt.Errorf("fail to get pod(%v) related to container", podID)
			klog.Errorf("fail to parse request %v %v", request, err)
			return nil
		}

		return &v1alpha1.ContainerResourceHookRequest{
			PodMeta: &v1alpha1.PodSandboxMetadata{
				Name:      podInfo.GetName(),
				Uid:       podInfo.GetUID(),
				Namespace: podInfo.GetNamespace(),
				Id:        podInfo.GetID(),
			},
			PodResources: podInfo.GetLinuxResources(),
			ContainerMeta: &v1alpha1.ContainerMetadata{
				Name: request.ContainerName,
			},
			ContainerAnnotations: request.ContainerRawAnnotations,
			ContainerResources:   utils.HostConfigToResource(request.HostConfig),
			PodAnnotations:       podInfo.GetAnnotations(),
			PodLabels:            podInfo.GetLabels(),
			PodCgroupParent:      podInfo.GetCgroupParentDir(),
			ContainerEnvs:        utils.SplitDockerEnv(request.Env),
		}
	case *runtimeapi.StopContainerRequest:
		return &v1alpha1.ContainerResourceHookRequest{
			ContainerMeta: &v1alpha1.ContainerMetadata{
				Id: request.GetContainerId(),
			},
		}
	}
	return nil
}

func (d *dispatcher) ParsePodRequest(req interface{}) interface{} {
	switch request := req.(type) {
	case *runtimeapi.RunPodSandboxRequest:
		return &v1alpha1.PodSandboxHookRequest{
			PodMeta: &v1alpha1.PodSandboxMetadata{
				Name:      request.GetConfig().GetMetadata().GetName(),
				Namespace: request.GetConfig().GetMetadata().GetNamespace(),
				Uid:       request.GetConfig().GetMetadata().GetUid(),
			},
			RuntimeHandler: request.GetRuntimeHandler(),
			Annotations:    request.GetConfig().GetAnnotations(),
			Labels:         request.GetConfig().GetLabels(),
			CgroupParent:   request.GetConfig().GetLinux().GetCgroupParent(),
		}
	case utils.DockerCreateRequest:
		return &v1alpha1.PodSandboxHookRequest{
			PodMeta: &v1alpha1.PodSandboxMetadata{
				Name:      request.PodName,
				Namespace: request.PodNamespace,
				Uid:       request.PodUid,
			},
			Labels:         request.ContainerRawLabel,
			Annotations:    request.ContainerRawAnnotations,
			CgroupParent:   utils.ToCriCgroupPath(d.dockerCgroupDriver, request.ConfigWrapper.HostConfig.CgroupParent),
			Resources:      utils.HostConfigToResource(request.ConfigWrapper.HostConfig),
			RuntimeHandler: "docker",
		}
	}
	return nil
}
