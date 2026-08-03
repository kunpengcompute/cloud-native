/*
 * Copyright (c) 2026 Huawei Technology corp.
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

package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"k8s.io/klog/v2"

	"kunpeng.huawei.com/kunpeng-cloud-computing/pkg/kunpeng-tap-pcore-biding/plugin"
	"kunpeng.huawei.com/kunpeng-cloud-computing/pkg/kunpeng-tap-pcore-biding/topology"
)

func main() {
	cfg := parseConfig()
	pairs, err := topology.DiscoverSiblingPairs()
	if err != nil {
		klog.Exitf("discover sibling pairs failed: %v", err)
	}

	agent, err := plugin.New(cfg, pairs)
	if err != nil {
		klog.Exitf("create plugin failed: %v", err)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if err := agent.Run(ctx); err != nil {
		klog.Exitf("plugin run failed: %v", err)
	}
}

func parseConfig() plugin.Config {
	cfg := plugin.DefaultConfig()
	namespaces := strings.Join(cfg.Namespaces, ",")
	runtimeClasses := strings.Join(cfg.RuntimeClasses, ",")

	flag.StringVar(&cfg.SocketPath, "nri-socket-path", cfg.SocketPath, "NRI socket path")
	flag.DurationVar(&cfg.ScanInterval, "scan-interval", cfg.ScanInterval, "cpuset reconciliation scan interval")
	flag.StringVar(&cfg.CgroupRoot, "cgroup-root", cfg.CgroupRoot, "cpuset cgroup root, auto-detected when empty")
	flag.StringVar(&namespaces, "namespace-whitelist", namespaces, "comma-separated namespace whitelist")
	flag.StringVar(&runtimeClasses, "runtimeclass-whitelist", runtimeClasses, "comma-separated runtimeClass whitelist")
	flag.BoolVar(&cfg.DryRun, "dry-run", cfg.DryRun, "log cpuset updates without writing cgroup files")
	flag.Parse()
	if cfg.ScanInterval <= 0 {
		klog.Exitf("scan interval must be greater than zero, got %s", cfg.ScanInterval)
	}

	cfg.Namespaces = splitCSV(namespaces)
	cfg.RuntimeClasses = splitCSV(runtimeClasses)
	if cfg.CgroupRoot == "" {
		root, err := plugin.DiscoverCpusetRoot()
		if err != nil {
			klog.Exitf("discover cpuset cgroup root failed: %v", err)
		}
		cfg.CgroupRoot = root
	}
	klog.InfoS("Using kunpeng tap pcore biding config", "nriSocketPath", cfg.SocketPath,
		"scanInterval", cfg.ScanInterval, "cgroupRoot", cfg.CgroupRoot,
		"namespaces", cfg.Namespaces, "runtimeClasses", cfg.RuntimeClasses, "dryRun", cfg.DryRun)
	return cfg
}

func splitCSV(raw string) []string {
	var out []string
	for _, item := range strings.Split(raw, ",") {
		item = strings.TrimSpace(item)
		if item != "" {
			out = append(out, item)
		}
	}
	return out
}
