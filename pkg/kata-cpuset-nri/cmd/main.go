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
	"os"
	"os/signal"
	"syscall"

	"k8s.io/klog/v2"

	"kunpeng.huawei.com/kunpeng-cloud-computing/pkg/kata-cpuset-nri/plugin"
	"kunpeng.huawei.com/kunpeng-cloud-computing/pkg/kata-cpuset-nri/topology"
)

func main() {
	cfg := plugin.DefaultConfig()
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
