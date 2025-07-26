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

package options

import "time"

const (
	DefaultRuntimeProxyEndpoint         = "/var/run/kunpeng/tap-runtime-proxy.sock"
	DefaultDockerRuntimeServiceEndpoint = "/var/run/docker.sock"
	DefaultNRISocketPath                = "/var/run/nri/nri.sock"

	ContainerRuntimeModeContainerd = "Containerd"
	ContainerRuntimeModeDocker     = "Docker"
	ContainerRuntimeModeNRI        = "NRI"
	DefaultContainerRuntimeMode    = ContainerRuntimeModeDocker

	TopologyAwarePolicy   = "topology-aware"
	NumaAwarePolicy       = "numa-aware"
	DefaultResourcePolicy = TopologyAwarePolicy

	// Resource priority options for topology-aware policy
	ResourcePriorityCPUFirst = "cpu-first"
	ResourcePriorityGPUFirst = "gpu-first"
	DefaultResourcePriority  = ResourcePriorityCPUFirst

	DefaultGracefulTimeout = time.Second * 15

	SkipTAPLabel = "tap.kunpeng.huawei.com/skip"

	UnixSocketPrefix      = "unix://"
	GRPCPassthroughScheme = "passthrough:///"
)

var (
	RuntimeProxyEndpoint     string
	ContainerRuntimeEndpoint string
	ContainerRuntimeMode     string
	NRISocketPath            string
	ResourcePolicy           string
	ResourcePriority         string
	GracefulTimeout          time.Duration
	MetricsAddr              string
	Version                  bool
	EnableMemoryTopology     bool
	NumaAlignOption          string
)
