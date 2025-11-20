/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"path/filepath"

	"github.com/docker/docker/client"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	criv1 "k8s.io/cri-api/pkg/apis/runtime/v1"
	"k8s.io/klog/v2"

	"kunpeng.huawei.com/kunpeng-cloud-computing/cmd/kunpeng-tap/proxy/options"
	"kunpeng.huawei.com/kunpeng-cloud-computing/pkg/kunpeng-tap/cache"
	"kunpeng.huawei.com/kunpeng-cloud-computing/pkg/kunpeng-tap/monitoring"
	"kunpeng.huawei.com/kunpeng-cloud-computing/pkg/kunpeng-tap/policy"
	numa_aware "kunpeng.huawei.com/kunpeng-cloud-computing/pkg/kunpeng-tap/policy/numa-aware"
	topologyaware "kunpeng.huawei.com/kunpeng-cloud-computing/pkg/kunpeng-tap/policy/topology-aware"
	"kunpeng.huawei.com/kunpeng-cloud-computing/pkg/kunpeng-tap/server"
	"kunpeng.huawei.com/kunpeng-cloud-computing/pkg/kunpeng-tap/server/containerd"
	"kunpeng.huawei.com/kunpeng-cloud-computing/pkg/kunpeng-tap/server/dispatcher"
	dispatch "kunpeng.huawei.com/kunpeng-cloud-computing/pkg/kunpeng-tap/server/dispatcher"
	"kunpeng.huawei.com/kunpeng-cloud-computing/pkg/kunpeng-tap/server/docker"
	"kunpeng.huawei.com/kunpeng-cloud-computing/pkg/kunpeng-tap/version"
)

func main() {

	flag.StringVar(&options.RuntimeProxyEndpoint, "runtime-proxy-endpoint", options.DefaultRuntimeProxyEndpoint,
		"runtimeproxy service endpoint.")
	flag.StringVar(&options.ContainerRuntimeEndpoint, "container-runtime-service-endpoint", options.DefaultDockerRuntimeServiceEndpoint,
		"container runtime service endpoint.")
	flag.StringVar(&options.ContainerRuntimeMode, "container-runtime-mode", options.DefaultContainerRuntimeMode,
		"container engine(Containerd|Docker).")
	flag.StringVar(&options.ResourcePolicy, "resource-policy", options.DefaultResourcePolicy,
		"container resource policy(numa-aware|topology-aware).")
	flag.DurationVar(&options.GracefulTimeout, "graceful-timeout", options.DefaultGracefulTimeout,
		"the duration for which the server gracefully wait for existing connections to finish, default 15s.")
	flag.StringVar(&options.MetricsAddr, "metrics-bind-address", ":9091", "The address the metrics endpoint binds to. Default :9091")
	flag.BoolVar(&options.Version, "version", false, "The version of kunpeng-tap proxy")
	flag.BoolVar(&options.EnableMemoryTopology, "enable-memory-topology", false,
		"Enable memory topology awareness. Default false")

	klog.InitFlags(nil)
	flag.Parse()

	if options.Version {
		version.PrintVersion()
		return
	}

	if err := os.Remove(options.RuntimeProxyEndpoint); err != nil && !os.IsNotExist(err) {
		klog.Fatalf("failed to unlink %v: %v", options.RuntimeProxyEndpoint, err)
	}

	if err := os.MkdirAll(filepath.Dir(options.RuntimeProxyEndpoint), 0750); err != nil {
		klog.Fatalf("failed to mkdir %v: %v", filepath.Dir(options.RuntimeProxyEndpoint), err)
	}

	cache, err := cache.NewCache(cache.Options{CacheDir: "/topology_cache"})
	if err != nil {
		klog.Fatalf("failed to create cache: %v", err)
	}

	// 创建策略选项
	policyOpts := policy.NewPolicyOptions()
	policyOpts.EnableMemoryTopology = options.EnableMemoryTopology

	var p policy.HookManager

	switch options.ResourcePolicy {
	case options.TopologyAwarePolicy:
		p = policy.NewPolicyManagerWithPolicies(cache, []policy.Policy{
			topologyaware.NewTopologyAwarePolicy(cache, policyOpts),
		})
	case options.NumaAwarePolicy:
		p = policy.NewPolicyManagerWithPolicies(cache, []policy.Policy{
			numa_aware.NewNumaAwarePolicy(cache),
		})
	default:
		klog.Fatalf("unknown resource policy %v", options.ResourcePolicy)
	}
	dispatcher := dispatch.NewDispatcher(p, cache)

	var proxyServer server.ProxyServer

	switch options.ContainerRuntimeMode {
	case options.ContainerRuntimeModeContainerd:
		proxyServer = NewContainerdProxyServer(dispatcher, cache)
	case options.ContainerRuntimeModeDocker:
		proxyServer = NewDockerProxyServer(dispatcher, cache)
	default:
		klog.Fatalf("unknown runtime engine backend %v", options.ContainerRuntimeMode)
	}

	// Expose the Prometheus http endpoint
	go monitoring.ExportMetrics()
	monitoring.ProxyHealthz.Set(1)
	go proxyServer.Run()

	c := make(chan os.Signal, 1)
	// We'll accept graceful shutdowns when quit via SIGINT (Ctrl+C)
	// SIGKILL, SIGQUIT or SIGTERM (Ctrl+/) will not be caught.
	signal.Notify(c, os.Interrupt)
	<-c

	ctx, cancel := context.WithTimeout(context.Background(), options.GracefulTimeout)
	defer cancel()

	proxyServer.Shutdown(ctx)

	klog.InfoS("Proxy server shutting down")
	os.Exit(0)
}

func NewContainerdProxyServer(dispatcher dispatcher.Dispatcher, cache cache.Cache) server.ProxyServer {
	containerRuntimeConn, err := grpc.NewClient(
		options.GRPCPassthroughScheme+options.ContainerRuntimeEndpoint,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(containerd.Dialer),
	)
	if err != nil {
		klog.Fatalf("fail to create runtime service client %v", err)
	}

	runtimeServiceClient := criv1.NewRuntimeServiceClient(containerRuntimeConn)
	// According to the version of cri api supported by backend runtime, create the corresponding cri server.
	if _, err := runtimeServiceClient.Version(context.Background(), &criv1.VersionRequest{}); err != nil {
		klog.Fatalf("fail to create runtime service client %v", err)
	}

	containerdClient := criv1.NewRuntimeServiceClient(containerRuntimeConn)

	err = cache.LoadStoreContainerd(containerdClient)
	if err != nil {
		klog.Fatalf("failed to load container and pod into cache: %v", err)
	}

	// 默认使用NUMA-aware策略
	return containerd.NewContainerdServer(
		containerd.NewCriServer(runtimeServiceClient, dispatcher),
		containerRuntimeConn,
	)
}

func NewDockerProxyServer(dispatcher dispatcher.Dispatcher, cache cache.Cache) server.ProxyServer {
	dockerClient, err := client.NewClientWithOpts(client.WithHost(options.UnixSocketPrefix+options.ContainerRuntimeEndpoint), client.WithAPIVersionNegotiation())
	if err != nil {
		klog.Fatalf("failed to get docker client from %v: %v", options.UnixSocketPrefix+options.ContainerRuntimeEndpoint, err)
	}

	info, err := dockerClient.Info(context.Background())
	if err != nil {
		klog.Fatalf("failed to get docker info: %v", err)
	}

	err = cache.LoadStoreDocker(dockerClient, info.CgroupDriver)
	if err != nil {
		klog.Fatalf("failed to load container and pod into cache: %v", err)
	}

	dispatcher.SetDockerCgroupDriver(info.CgroupDriver)
	// 默认使用NUMA-aware策略
	return docker.NewDockerServer(docker.NewDockerHandler(
		docker.ReverseProxy(options.ContainerRuntimeEndpoint),
		cache,
		dockerClient,
		dispatcher,
	))
}
