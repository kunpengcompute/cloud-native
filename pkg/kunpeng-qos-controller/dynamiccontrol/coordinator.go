/*
Copyright (c) Huawei Technologies Co., Ltd. 2023-2026. All rights reserved.

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

package dynamiccontrol

import (
	"context"
	"fmt"
	"strings"
	"time"

	"k8s.io/klog/v2"
)

const (
	// WorkloadClassLabelKey marks workload class for dynamic interference control.
	WorkloadClassLabelKey = "qos.kunpeng.huawei.com/workload-class"
	// WorkloadClassOnline marks pods that should be treated as online workloads.
	WorkloadClassOnline = "online"
	// WorkloadClassOffline marks pods that should be treated as offline workloads.
	WorkloadClassOffline = "offline"
)

// NodeIdentity provides local node identity.
type NodeIdentity interface {
	NodeName() string
}

// OnlinePodCgroup describes one online pod target sent to interference agent.
type OnlinePodCgroup struct {
	Namespace  string `json:"namespace"`
	Name       string `json:"name"`
	UID        string `json:"uid"`
	CgroupPath string `json:"cgroup_path"`
}

// AgentAnalyzeRequest is the payload sent to external interference agent.
type AgentAnalyzeRequest struct {
	NodeName string            `json:"node_name"`
	Time     time.Time         `json:"timestamp"`
	Pods     []OnlinePodCgroup `json:"pods"`
}

// InterferenceReason is one interference dimension reported by analyzer.
type InterferenceReason string

const (
	InterferenceReasonNone InterferenceReason = "none"
	InterferenceReasonL3   InterferenceReason = "l3"
	InterferenceReasonMB   InterferenceReason = "mb"
	InterferenceReasonCPU  InterferenceReason = "cpu"
)

// AgentAnalyzeResult is the output from external interference analyzer.
type AgentAnalyzeResult struct {
	Reasons []InterferenceReason
}

func normalizeInterferenceReasons(
	reasons []InterferenceReason,
	includeNone bool,
) ([]InterferenceReason, []InterferenceReason) {
	seen := make(map[InterferenceReason]struct{}, len(reasons))
	ignored := make([]InterferenceReason, 0)
	for _, reason := range reasons {
		normalized := InterferenceReason(strings.ToLower(strings.TrimSpace(string(reason))))
		switch normalized {
		case InterferenceReasonNone:
			if includeNone {
				seen[normalized] = struct{}{}
			}
		case InterferenceReasonCPU, InterferenceReasonMB, InterferenceReasonL3:
			seen[normalized] = struct{}{}
		default:
			ignored = append(ignored, reason)
		}
	}

	normalized := make([]InterferenceReason, 0, len(seen))
	if _, ok := seen[InterferenceReasonNone]; ok {
		normalized = append(normalized, InterferenceReasonNone)
	}
	for _, reason := range []InterferenceReason{
		InterferenceReasonCPU,
		InterferenceReasonMB,
		InterferenceReasonL3,
	} {
		if _, ok := seen[reason]; ok {
			normalized = append(normalized, reason)
		}
	}
	return normalized, ignored
}

// Coordinator orchestrates one dynamic-control cycle:
// collect online pods -> ask interference agent -> plan tuning -> apply tuning.
type Coordinator struct {
	NodeIdentity NodeIdentity
	OnlineSource OnlinePodSource
	Agent        AgentClient
	Engine       TuningEngine
	Clock        func() time.Time
}

// NewCoordinator creates a dynamic-control coordinator with explicit dependencies.
func NewCoordinator(
	nodeIdentity NodeIdentity,
	onlineSource OnlinePodSource,
	agent AgentClient,
	engine TuningEngine,
) *Coordinator {
	return &Coordinator{
		NodeIdentity: nodeIdentity,
		OnlineSource: onlineSource,
		Agent:        agent,
		Engine:       engine,
		Clock:        time.Now,
	}
}

// EnsurePolicyOnce creates the node-local dynamic policy with default settings.
func (c *Coordinator) EnsurePolicyOnce(ctx context.Context) error {
	nodeName := c.NodeIdentity.NodeName()
	if nodeName == "" {
		return fmt.Errorf("node name must not be empty")
	}

	return c.Engine.EnsurePolicy(ctx, nodeName)
}

// PublishOnlinePodsOnce executes one publish cycle:
// collect online pod cgroup paths and publish to external agent.
func (c *Coordinator) PublishOnlinePodsOnce(ctx context.Context) error {
	nodeName := c.NodeIdentity.NodeName()
	if nodeName == "" {
		return fmt.Errorf("node name must not be empty")
	}

	pods, err := c.OnlineSource.ListOnlinePodCgroups(ctx, nodeName)
	if err != nil {
		return err
	}

	return c.Agent.PublishOnlinePods(ctx, AgentAnalyzeRequest{
		NodeName: nodeName,
		Time:     c.Clock(),
		Pods:     pods,
	})
}

// ApplyInterferenceOnce executes one apply cycle:
// pull interference result, plan tuning decision, and apply if needed.
func (c *Coordinator) ApplyInterferenceOnce(ctx context.Context) error {
	nodeName := c.NodeIdentity.NodeName()
	if nodeName == "" {
		return fmt.Errorf("node name must not be empty")
	}

	result, err := c.Agent.GetInterference(ctx, nodeName)
	if err != nil {
		return err
	}
	klog.Infof(
		"dynamic-control received interference result: node=%s reasons=%v",
		nodeName, result.Reasons,
	)

	return c.Engine.HandleInterference(ctx, nodeName, result)
}
