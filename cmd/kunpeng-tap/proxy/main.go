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
	"kunpeng.huawei.com/kunpeng-cloud-computing/pkg/kunpeng-tap/server/docker"
	"kunpeng.huawei.com/kunpeng-cloud-computing/pkg/kunpeng-tap/server/nri"
	"kunpeng.huawei.com/kunpeng-cloud-computing/pkg/kunpeng-tap/version"
)

func main() {
	parseFlags()

	if options.Version {
		version.PrintVersion()
		return
	}

	validateOptions()
	setupRuntimeEndpoint()

	cache := createCache()
	policyManager := createPolicyManager(cache)
	d := dispatcher.NewDispatcher(policyManager, cache)
	proxyServer := createProxyServer(d, policyManager, cache)

	startMonitor(proxyServer)
	go proxyServer.Run()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	<-c

	ctx, cancel := context.WithTimeout(context.Background(), options.GracefulTimeout)
	defer cancel()

	proxyServer.Shutdown(ctx)
	klog.InfoS("Proxy server shutting down")
	os.Exit(0)
}

// parseFlags initializes and parses command line flags
func parseFlags() {
	flag.StringVar(&options.RuntimeProxyEndpoint, "runtime-proxy-endpoint", options.DefaultRuntimeProxyEndpoint,
		"runtimeproxy service endpoint.")
	flag.StringVar(&options.ContainerRuntimeEndpoint, "container-runtime-service-endpoint", options.DefaultDockerRuntimeServiceEndpoint,
		"container runtime service endpoint.")
	flag.StringVar(&options.ContainerRuntimeMode, "container-runtime-mode", options.DefaultContainerRuntimeMode,
		"container engine(Containerd|Docker|NRI).")
	flag.StringVar(&options.NRISocketPath, "nri-socket-path", options.DefaultNRISocketPath,
		"NRI socket path (only used when container-runtime-mode is NRI).")
	flag.StringVar(&options.ResourcePolicy, "resource-policy", options.DefaultResourcePolicy,
		"container resource policy(numa-aware|topology-aware).")
	flag.StringVar(&options.ResourcePriority, "resource-priority", options.DefaultResourcePriority,
		"resource allocation priority for topology-aware policy(cpu-first|gpu-first). Default cpu-first")
	flag.DurationVar(&options.GracefulTimeout, "graceful-timeout", options.DefaultGracefulTimeout,
		"the duration for which the server gracefully wait for existing connections to finish, default 15s.")
	flag.StringVar(&options.MetricsAddr, "metrics-bind-address", "", "The address the metrics endpoint binds to. Default empty (disabled)")
	flag.BoolVar(&options.Version, "version", false, "The version of kunpeng-tap proxy")
	flag.BoolVar(&options.EnableMemoryTopology, "enable-memory-topology", false,
		"Enable memory topology awareness. Default false")
	flag.BoolVar(&options.EnableClusterAffinity, "topology-cluster-affinity", false,
		"Enable cluster-level CPU affinity for 950 model machines. Default false")

	klog.InitFlags(nil)
	flag.Parse()
}

// validateOptions validates command line options
func validateOptions() {
	if options.ResourcePriority != options.ResourcePriorityCPUFirst &&
		options.ResourcePriority != options.ResourcePriorityGPUFirst {
		klog.Fatalf("invalid resource priority %v, must be one of: %v, %v",
			options.ResourcePriority, options.ResourcePriorityCPUFirst, options.ResourcePriorityGPUFirst)
	}
}

// setupRuntimeEndpoint prepares the runtime proxy endpoint
func setupRuntimeEndpoint() {
	if err := os.Remove(options.RuntimeProxyEndpoint); err != nil && !os.IsNotExist(err) {
		klog.Fatalf("failed to unlink %v: %v", options.RuntimeProxyEndpoint, err)
	}

	if err := os.MkdirAll(filepath.Dir(options.RuntimeProxyEndpoint), 0750); err != nil {
		klog.Fatalf("failed to mkdir %v: %v", filepath.Dir(options.RuntimeProxyEndpoint), err)
	}
}

// createCache creates and returns a new cache instance
func createCache() cache.Cache {
	cache, err := cache.NewCache(cache.Options{CacheDir: "/topology_cache"})
	if err != nil {
		klog.Fatalf("failed to create cache: %v", err)
	}
	return cache
}

// createPolicyManager creates and configures the policy manager
func createPolicyManager(cache cache.Cache) policy.HookManager {
	policyOpts := createPolicyOptions()

	switch options.ResourcePolicy {
	case options.TopologyAwarePolicy:
		return policy.NewPolicyManagerWithPolicies(cache, []policy.Policy{
			topologyaware.NewTopologyAwarePolicy(cache, policyOpts),
		})
	case options.NumaAwarePolicy:
		return policy.NewPolicyManagerWithPolicies(cache, []policy.Policy{
			numa_aware.NewNumaAwarePolicy(cache),
		})
	default:
		klog.Fatalf("unknown resource policy %v", options.ResourcePolicy)
		return nil
	}
}

// createPolicyOptions creates and configures policy options
func createPolicyOptions() *policy.PolicyOptions {
	policyOpts := policy.NewPolicyOptions()
	policyOpts.EnableMemoryTopology = options.EnableMemoryTopology
	policyOpts.ResourcePriority = options.ResourcePriority
	policyOpts.EnableClusterAffinity = options.EnableClusterAffinity

	return policyOpts
}

// createProxyServer creates the appropriate proxy server based on runtime mode
func createProxyServer(d dispatcher.Dispatcher, policyManager policy.HookManager, cache cache.Cache) server.ProxyServer {
	switch options.ContainerRuntimeMode {
	case options.ContainerRuntimeModeContainerd:
		proxyServer := NewContainerdProxyServer(d, cache)
		// Trigger resource state rebuild after cache population
		triggerRebuildAllocations(policyManager)
		return proxyServer
	case options.ContainerRuntimeModeDocker:
		proxyServer := NewDockerProxyServer(d, cache)
		// Trigger resource state rebuild after cache population
		triggerRebuildAllocations(policyManager)
		return proxyServer
	case options.ContainerRuntimeModeNRI:
		// NRI mode works as a plugin, no proxy socket needed
		// For NRI, rebuild is triggered in Synchronize callback
		klog.InfoS("NRI mode: skipping proxy socket setup", "nriSocketPath", options.NRISocketPath)
		return NewNriProxyServer(policyManager, cache, options.NRISocketPath)
	default:
		klog.Fatalf("unknown runtime engine backend %v", options.ContainerRuntimeMode)
		return nil
	}
}

// triggerRebuildAllocations triggers resource allocation state rebuild from cache
func triggerRebuildAllocations(policyManager policy.HookManager) {
	if rebuilder, ok := policyManager.(policy.StateSynchronizer); ok {
		if err := rebuilder.RebuildAllocationsFromCache(); err != nil {
			klog.ErrorS(err, "Failed to rebuild allocations from cache")
		} else {
			klog.InfoS("Successfully rebuilt allocations from cache")
		}
	}
}

// startMonitor starts the promethues monitoring server
func startMonitor(proxyServer server.ProxyServer) {
	if options.MetricsAddr != "" {
		go monitoring.ExportMetrics()
		monitoring.ProxyHealthz.Set(1)
	}
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

// NewNriProxyServer creates a NRI proxy server.
func NewNriProxyServer(hookManager policy.HookManager, cache cache.Cache, nriSocketPath string) server.ProxyServer {
	klog.InfoS("Creating NRI proxy server", "socketPath", nriSocketPath)
	proxyServer, err := nri.NewNriServer(cache, hookManager, nriSocketPath)
	if err != nil {
		klog.Fatalf("Failed to create NRI proxy server: %v", err)
	}
	return proxyServer
}
