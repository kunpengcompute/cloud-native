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
	"time"
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

// InterferenceReason is the dominant interference dimension reported by analyzer.
type InterferenceReason string

const (
	InterferenceReasonUnknown InterferenceReason = "unknown"
	InterferenceReasonL3      InterferenceReason = "l3"
	InterferenceReasonMB      InterferenceReason = "mb"
	InterferenceReasonCPU     InterferenceReason = "cpu"
)

// InterferenceItem describes one interference target.
type InterferenceItem struct {
	PodUID string  `json:"pod_uid"`
	Score  float64 `json:"score"`
}

// AgentAnalyzeResult is the output from external interference analyzer.
type AgentAnalyzeResult struct {
	Reason     InterferenceReason
	TTLSeconds int64
	Items      []InterferenceItem
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

	return c.Engine.HandleInterference(ctx, nodeName, result)
}
