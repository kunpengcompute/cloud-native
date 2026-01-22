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

package docker

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	dockertypes "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/docker/docker/runconfig"
	"github.com/gorilla/mux"
	"k8s.io/klog/v2"

	"kunpeng.huawei.com/kunpeng-cloud-computing/pkg/kunpeng-tap/cache"
	"kunpeng.huawei.com/kunpeng-cloud-computing/pkg/kunpeng-tap/monitoring"
	"kunpeng.huawei.com/kunpeng-cloud-computing/pkg/kunpeng-tap/policy"
	"kunpeng.huawei.com/kunpeng-cloud-computing/pkg/kunpeng-tap/server"
	"kunpeng.huawei.com/kunpeng-cloud-computing/pkg/kunpeng-tap/server/dispatcher"
	"kunpeng.huawei.com/kunpeng-cloud-computing/pkg/kunpeng-tap/server/docker/utils"
)

type DockerHandler interface {
	HandleCreateContainer() func(http.ResponseWriter, *http.Request)
	HandleDeleteContainer() func(http.ResponseWriter, *http.Request)
	HandleStartContainer() func(http.ResponseWriter, *http.Request)
	HandleStopContainer() func(http.ResponseWriter, *http.Request)
	HandleUpdateContainer() func(http.ResponseWriter, *http.Request)
	Direct(wr http.ResponseWriter, req *http.Request) string
}

func NewDockerHandler(reverseProxy http.Handler, cache cache.Cache, dockerClient *client.Client, dispatcher dispatcher.Dispatcher,
) DockerHandler {
	return &dockerHandler{
		reverseProxy: reverseProxy,
		cache:        cache,
		dockerClient: dockerClient,
		dispatcher:   dispatcher,
	}
}

type dockerHandler struct {
	sync.Mutex
	reverseProxy http.Handler
	cache        cache.Cache
	dockerClient *client.Client
	dispatcher   dispatcher.Dispatcher
}

func (d *dockerHandler) Direct(wr http.ResponseWriter, req *http.Request) string {
	out := &bytes.Buffer{}
	multi := &mockRespWriter{wr, out, 0}
	klog.V(5).InfoS("Request", "header", req.Header, "method", req.Method, "url", req.URL.String())
	d.reverseProxy.ServeHTTP(multi, req)
	resp := out.String()
	klog.V(5).InfoS("Response", "code", multi.code, "body", resp, "headers", wr.Header())
	return resp
}

func (d *dockerHandler) HandleStartContainer() func(http.ResponseWriter, *http.Request) {
	return func(wr http.ResponseWriter, req *http.Request) {
		d.Mutex.Lock()
		defer d.Mutex.Unlock()
		klog.V(3).InfoS("Start container", "url", req.URL.String())
		start := time.Now()
		defer func() {
			duration := time.Since(start).Seconds()
			monitoring.ProxyRequestDuration.WithLabelValues(monitoring.ProxyStartContainerRequest).Observe(duration)
		}()

		vars := mux.Vars(req)
		containerID := vars["containerid"]

		klog.V(5).InfoS("Start container", "containerId", containerID)

		d.Direct(wr, req)
	}
}

func (d *dockerHandler) HandleStopContainer() func(http.ResponseWriter, *http.Request) {
	return func(wr http.ResponseWriter, req *http.Request) {
		d.Mutex.Lock()
		defer d.Mutex.Unlock()
		klog.V(3).InfoS("Stop container", "url", req.URL.String())
		start := time.Now()
		defer func() {
			duration := time.Since(start).Seconds()
			monitoring.ProxyRequestDuration.WithLabelValues(monitoring.ProxyStopContainerRequest).Observe(duration)
		}()

		// Parse Docker stop request
		dockerStopRequest, err := utils.ParseDockerStopRequest(req)
		if err != nil {
			klog.ErrorS(err, "Failed to parse docker stop request")
			http.Error(wr, err.Error(), http.StatusBadRequest)
			return
		}

		klog.V(5).InfoS("Stop container", "containerId", dockerStopRequest.ContainerID)

		// Parse hook request
		hookReq := d.dispatcher.ParseContainerRequest(dockerStopRequest)
		if hookReq == nil {
			klog.ErrorS(nil, "Failed to parse container request for hook")
			http.Error(wr, "Failed to parse container request", http.StatusInternalServerError)
			return
		}

		// Send request to Docker runtime first
		dockerResponse := d.Direct(wr, req)

		// Parse Docker response status code
		respCode := utils.ParseDockerResponseCode(dockerResponse)

		klog.V(5).InfoS("Docker stop response", "containerId", dockerStopRequest.ContainerID, "responseCode", respCode, "response", dockerResponse)

		// Only call PostStopContainer hook if the stop operation was successful
		if respCode != 204 && respCode != 304 {
			klog.V(3).InfoS("Skipping PostStopContainer hook due to Docker error", "containerId", dockerStopRequest.ContainerID, "responseCode", respCode)
			return
		}

		// Check if container exists in cache for hook
		_, exist := d.cache.LookupContainer(dockerStopRequest.ContainerID)
		if !exist {
			klog.V(3).InfoS("Container not found in cache for PostStopContainer hook", "containerId", dockerStopRequest.ContainerID)
			return
		}

		// Call PostStopContainer hook
		hookResp := d.dispatcher.Dispatch(context.Background(), policy.PostStopContainer, hookReq, nil)
		if hookResp != nil {
			klog.V(5).InfoS("PostStopContainer hook response", "containerId", dockerStopRequest.ContainerID, "response", hookResp)
		}

		// Delete container from cache after successful stop operation
		if _, exist := d.cache.LookupContainer(dockerStopRequest.ContainerID); exist {
			klog.V(3).InfoS("Remove container from container cache", "containerId", dockerStopRequest.ContainerID)
			d.cache.DeleteContainer(dockerStopRequest.ContainerID)
		}

	}
}

func (d *dockerHandler) HandleUpdateContainer() func(http.ResponseWriter, *http.Request) {
	return func(wr http.ResponseWriter, req *http.Request) {
		d.Mutex.Lock()
		defer d.Mutex.Unlock()
		klog.V(3).InfoS("Update container", "url", req.URL.String())
		start := time.Now()
		defer func() {
			duration := time.Since(start).Seconds()
			monitoring.ProxyRequestDuration.WithLabelValues(monitoring.ProxyUpdateContainerRequest).Observe(duration)
		}()

		vars := mux.Vars(req)
		containerID := vars["containerid"]

		klog.V(5).InfoS("Update container", "containerId", containerID)

		d.Direct(wr, req)
	}
}

func (d *dockerHandler) HandleDeleteContainer() func(http.ResponseWriter, *http.Request) {
	return func(wr http.ResponseWriter, req *http.Request) {
		d.Mutex.Lock()
		defer d.Mutex.Unlock()
		klog.V(3).InfoS("Delete container", "url", req.URL.String())
		start := time.Now()
		defer func() {
			duration := time.Since(start).Seconds()
			monitoring.ProxyRequestDuration.WithLabelValues(monitoring.ProxyRemoveContainerRequest).Observe(duration)
		}()

		vars := mux.Vars(req)
		containerID := vars["containerid"]

		klog.V(5).InfoS("Delete container", "containerId", containerID)

		d.Direct(wr, req)

		if _, exist := d.cache.LookupContainer(containerID); exist {
			klog.V(3).InfoS("Remove container from container cache", "containerId", containerID)
			d.cache.DeleteContainer(containerID)
		}
		if _, exist := d.cache.LookupPod(containerID); exist {
			klog.V(3).InfoS("Remove pod from pod cache", "podId", containerID)
			d.cache.DeletePod(containerID)
		}
	}
}

func (d *dockerHandler) HandleCreateContainer() func(http.ResponseWriter, *http.Request) {
	return func(wr http.ResponseWriter, req *http.Request) {
		d.Mutex.Lock()
		defer d.Mutex.Unlock()

		d.CleanCache()

		klog.V(3).InfoS("Create container", "url", req.URL.String())
		start := time.Now()
		defer func() {
			duration := time.Since(start).Seconds()
			monitoring.ProxyRequestDuration.WithLabelValues(monitoring.ProxyCreateContainerRequest).Observe(duration)
		}()

		dockerCreateRequest, err := d.ParseRequest(req)
		if err != nil {
			http.Error(wr, err.Error(), http.StatusInternalServerError)
			return
		}

		var hookReq, hookResp interface{}
		if dockerCreateRequest.RuntimeResourceType == server.RuntimeContainerResource {
			podID := dockerCreateRequest.ConfigWrapper.Config.Labels[utils.SandboxIDLabelKey]
			podInfo, exist := d.cache.LookupPod(podID)
			if !exist {
				// refuse the req
				klog.ErrorS(err, "Failed to get pod info", "podId", podID)
				http.Error(wr, "Failed to get pod info", http.StatusInternalServerError)
				return
			}
			klog.V(5).InfoS("Get pod from cache for container",
				"containerName", dockerCreateRequest.ContainerRawName,
				"podId", podID,
				"podInfo", podInfo)

			hookReq = d.dispatcher.ParseContainerRequest(dockerCreateRequest)

			hookResp = d.dispatcher.Dispatch(context.Background(), policy.PreCreateContainer, hookReq, podInfo.GetLabels())
		} else {
			hookReq = d.dispatcher.ParsePodRequest(dockerCreateRequest)
			hookResp = d.dispatcher.Dispatch(context.Background(), policy.PreRunPodSandbox, hookReq, dockerCreateRequest.ContainerRawLabel)
		}

		cfgBody := utils.ConfigWrapper{
			Config:           dockerCreateRequest.ConfigWrapper.Config,
			HostConfig:       dockerCreateRequest.ConfigWrapper.HostConfig,
			NetworkingConfig: dockerCreateRequest.ConfigWrapper.NetworkingConfig,
		}

		d.dispatcher.BackfillRequest(&cfgBody, hookReq, hookResp)

		// send req to docker
		klog.V(5).InfoS("Proxy request to docker runtime", "request", cfgBody)

		dockerResponse, err := d.ProxyRequestToDockerRuntime(cfgBody, req, wr)
		if err != nil {
			http.Error(wr, err.Error(), http.StatusInternalServerError)
			return
		}

		d.dispatcher.InsertIntoCacheIfNeed(dockerResponse, hookReq)
	}
}

func (d *dockerHandler) ParseRequest(req *http.Request) (utils.DockerCreateRequest, error) {
	dec := runconfig.ContainerDecoder{}
	containerConfig, hostConfig, networkingConfig, err := dec.DecodeConfig(req.Body)
	if err != nil {
		klog.ErrorS(err, "Failed to decode docker create config")
		return utils.DockerCreateRequest{}, err
	}

	klog.V(5).InfoS("Get decoded docker create config",
		"ContainerConfig", containerConfig,
		"HostConfig", hostConfig,
		"NetworkingConfig", networkingConfig)

	runtimeResourceType := utils.GetRuntimeResourceType(containerConfig.Labels)
	containerRawName := req.URL.Query().Get("name")
	tokens := strings.Split(containerRawName, "_")
	if len(tokens) != 6 {
		err := errors.New("len of tokens in container name is not equal 6: " + containerRawName)
		klog.ErrorS(err, "Failed to split k8s container name", "containerName", containerRawName)
		return utils.DockerCreateRequest{}, err
	}
	klog.V(5).InfoS("Get container name and resource type",
		"containerRawName", containerRawName,
		"runtimeResourceType", runtimeResourceType)

	labels, annos := utils.SplitLabelsAndAnnotations(containerConfig.Labels)

	return utils.DockerCreateRequest{
		ConfigWrapper: utils.ConfigWrapper{
			Config:           containerConfig,
			HostConfig:       hostConfig,
			NetworkingConfig: networkingConfig,
		},
		RuntimeResourceType:     runtimeResourceType,
		ContainerRawName:        containerRawName,
		ContainerRawLabel:       labels,
		ContainerRawAnnotations: annos,
		ContainerName:           tokens[1],
		PodName:                 tokens[2],
		PodNamespace:            tokens[3],
		PodUid:                  tokens[4],
	}, nil
}

func (d *dockerHandler) CleanCache() {
	// Create filters for running and created containers
	filterArgs := filters.NewArgs()
	filterArgs.Add("status", "running") // Add filter for running containers
	filterArgs.Add("status", "created") // Add another filter for created containers
	allContainers, err := d.dockerClient.ContainerList(
		context.Background(),
		dockertypes.ContainerListOptions{
			Filters: filterArgs,
		})
	if err != nil {
		klog.ErrorS(err, "Failed to list running/created containers")
	}

	staleContainerIds := d.cache.ValidateCachedContainers(utils.DockerContainers2ContainerIds(allContainers))
	// Clean up stale containers before processing the create request
	// This helps address concurrency issues where Delete requests might be delayed
	// and Create requests arrive first during container restarts
	cleanedCount := d.cache.CleanupStaleContainers(staleContainerIds)
	if cleanedCount > 0 {
		klog.V(3).InfoS("Cleaned up stale containers before processing create request", "count", cleanedCount)
	}
}

func (d *dockerHandler) ProxyRequestToDockerRuntime(
	cfgBody utils.ConfigWrapper,
	req *http.Request,
	wr http.ResponseWriter,
) (*container.ContainerCreateCreatedBody, error) {
	nBody, err := utils.EncodeBody(cfgBody)
	if err != nil {
		klog.ErrorS(err, "Failed to parse req to local store")
		return nil, err
	}
	req.Body = io.NopCloser(nBody)
	nBody, err = utils.EncodeBody(cfgBody)
	if err != nil {
		klog.ErrorS(err, "Failed to encode body")
	}
	newLength, err := utils.CalculateContentLength(nBody)
	if err != nil {
		klog.ErrorS(err, "Failed to calculate content length")
	}
	req.ContentLength = newLength
	resp := d.Direct(wr, req)

	createResp := &container.ContainerCreateCreatedBody{}
	err = json.Unmarshal([]byte(resp), createResp)
	if err != nil {
		klog.ErrorS(err, "Failed to Unmarshal create resp", "response", resp)
	}

	klog.V(5).InfoS("Response from create container", "response", resp)

	return createResp, nil
}
