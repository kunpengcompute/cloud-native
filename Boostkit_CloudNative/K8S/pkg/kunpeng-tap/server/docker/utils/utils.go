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

package utils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path"
	"strconv"
	"strings"

	dockertypes "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"k8s.io/klog/v2"

	"github.com/gorilla/mux"
	"kunpeng.huawei.com/kunpeng-cloud-computing/api/kunpeng-tap/policy-manager/v1alpha1"
	"kunpeng.huawei.com/kunpeng-cloud-computing/pkg/kunpeng-tap/server"
)

const (
	ContainerTypeLabelKey       = "io.kubernetes.docker.type"
	ContainerTypeLabelSandbox   = "podsandbox"
	ContainerTypeLabelContainer = "container"
	SandboxIDLabelKey           = "io.kubernetes.sandbox.id"
)

type DockerCreateRequest struct {
	ConfigWrapper
	RuntimeResourceType server.RuntimeResourceType
	// ContainerRawName is the name of docker container
	ContainerRawName string
	// labels in docker container
	ContainerRawLabel map[string]string
	// annotation in docker container
	ContainerRawAnnotations map[string]string
	// ContainerName is the name of container in pod
	ContainerName string
	// kubernetes pod name
	PodName string
	// kubernetes pod namespace
	PodNamespace string
	// kubernetes pod uid
	PodUid string
}

type ConfigWrapper struct {
	*container.Config
	HostConfig       *container.HostConfig
	NetworkingConfig *network.NetworkingConfig
}

func GetRuntimeResourceType(labels map[string]string) server.RuntimeResourceType {
	if labels[ContainerTypeLabelKey] == ContainerTypeLabelSandbox {
		return server.RuntimePodResource
	}
	return server.RuntimeContainerResource
}

func EncodeBody(obj interface{}) (io.Reader, error) {
	if obj == nil {
		return nil, nil
	}

	body, err := EncodeData(obj)
	if err != nil {
		return nil, err
	}
	return body, nil
}

func EncodeData(data interface{}) (*bytes.Buffer, error) {
	params := bytes.NewBuffer(nil)
	if data != nil {
		if err := json.NewEncoder(params).Encode(data); err != nil {
			return nil, err
		}
	}
	return params, nil
}

func CalculateContentLength(body io.Reader) (l int64, err error) {
	if body == nil {
		return -1, fmt.Errorf("reader is nil")
	}
	buf := &bytes.Buffer{}
	nRead, err := io.Copy(buf, body)
	if err != nil {
		return -1, err
	}
	l = nRead
	return
}

func SplitLabelsAndAnnotations(configs map[string]string) (labels map[string]string, annos map[string]string) {
	labels = make(map[string]string)
	annos = make(map[string]string)
	for k, v := range configs {
		if strings.HasPrefix(k, "annotation.") {
			annos[strings.TrimPrefix(k, "annotation.")] = v
		} else {
			labels[k] = v
		}
	}
	return
}

func HostConfigToResource(config *container.HostConfig) *v1alpha1.LinuxContainerResources {
	if config == nil {
		return nil
	}
	return &v1alpha1.LinuxContainerResources{
		CpuPeriod:              config.CPUPeriod,
		CpuQuota:               config.CPUQuota,
		CpuShares:              config.CPUShares,
		MemoryLimitInBytes:     config.Memory,
		OomScoreAdj:            int64(config.OomScoreAdj),
		CpusetCpus:             config.CpusetCpus,
		CpusetMems:             config.CpusetMems,
		MemorySwapLimitInBytes: config.MemorySwap,
	}
}

func SplitDockerEnv(dockerEnvs []string) map[string]string {
	res := make(map[string]string)
	for _, str := range dockerEnvs {
		tokens := strings.SplitN(str, "=", 2)
		if len(tokens) != 2 {
			continue
		}
		res[tokens[0]] = tokens[1]
	}
	return res
}

func DockerContainers2ContainerIds(allContainers []dockertypes.Container) []string {
	containerIds := []string{}
	for _, container := range allContainers {
		containerIds = append(containerIds, container.ID)
	}
	return containerIds
}

func ToCriCgroupPath(cgroupDriver, cgroupParent string) string {
	if cgroupDriver != "systemd" {
		return cgroupParent
	}

	if cgroupParent == "" {
		return cgroupParent
	}

	// for docker + systemd combination, the cgroup parent is pod dir, for example:
	// kubepods-besteffort-pod7712555c_ce62_454a_9e18_9ff0217b8941.slice,
	// so the full path for it is :
	// /kubepods.slice/kubepods-besteffort.slice/kubepods-besteffort-pod7712555c_ce62_454a_9e18_9ff0217b8941.slice
	cgroupHierarcy := strings.Split(cgroupParent, "-")
	prefix := cgroupHierarcy[0]
	fullPath := "/"
	for i := 0; i < len(cgroupHierarcy)-1; i++ {
		fullPath = path.Join(fullPath, fmt.Sprintf("%s.slice", prefix))
		prefix = fmt.Sprintf("%s-%s", prefix, cgroupHierarcy[i+1])
	}
	fullPath = path.Join(fullPath, prefix)
	return fullPath
}

func UpdateHostConfigByResource(config *container.HostConfig, resources *v1alpha1.LinuxContainerResources) *container.HostConfig {
	if config == nil || resources == nil {
		return config
	}
	config.CPUPeriod = resources.CpuPeriod
	config.CPUQuota = resources.CpuQuota
	config.CPUShares = resources.CpuShares
	config.Memory = resources.MemoryLimitInBytes
	config.OomScoreAdj = int(resources.OomScoreAdj)
	config.CpusetCpus = resources.CpusetCpus
	config.CpusetMems = resources.CpusetMems
	config.MemorySwap = resources.MemorySwapLimitInBytes
	return config
}

// GenerateExpectedCgroupParent is adapted from Dockershim
func GenerateExpectedCgroupParent(cgroupDriver, cgroupParent string) string {
	if cgroupParent != "" {
		// if docker uses the systemd cgroup driver, it expects *.slice style names for cgroup parent.
		// if we configured kubelet to use --cgroup-driver=cgroupfs, and docker is configured to use systemd driver
		// docker will fail to launch the container because the name we provide will not be a valid slice.
		// this is a very good thing.
		if cgroupDriver == "systemd" {
			// Pass only the last component of the cgroup path to systemd.
			cgroupParent = path.Base(cgroupParent)
		}
	}
	klog.V(4).InfoS("Setting cgroup parent", "cgroupParent", cgroupParent)
	return cgroupParent
}

func GenerateEnvList(envs map[string]string) (result []string) {
	for key, value := range envs {
		result = append(result, fmt.Sprintf("%s=%s", key, value))
	}
	return
}

// DockerStopRequest represents a Docker stop container request
type DockerStopRequest struct {
	ContainerID string
	Timeout     *int
}

// ParseDockerStopRequest parses the Docker stop container request from HTTP request
func ParseDockerStopRequest(req *http.Request) (DockerStopRequest, error) {
	vars := mux.Vars(req)
	containerID := vars["containerid"]
	if containerID == "" {
		return DockerStopRequest{}, fmt.Errorf("container ID is empty")
	}

	// Parse timeout from query parameters
	var timeout *int
	if timeoutStr := req.URL.Query().Get("t"); timeoutStr != "" {
		if t, err := strconv.Atoi(timeoutStr); err == nil {
			timeout = &t
		}
	}

	return DockerStopRequest{
		ContainerID: containerID,
		Timeout:     timeout,
	}, nil
}

func ParseDockerResponseCode(response string) int {
	if strings.Contains(response, "No such container") {
		return 404
	} else if strings.Contains(response, "container already stopped") {
		return 304
	} else if strings.Contains(response, "server error") {
		return 500
	}
	return 204
}
