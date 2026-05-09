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
	WorkloadClassLabelKey = "mpam.kunpeng.huawei.com/workload-class"
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

// DynamicTuningDecision is a generic output from planner to execution layer.
// Concrete knobs (MB/L3/L3MAX/MBMIN/L3MIN/CPU QoS) can be expanded later.
type DynamicTuningDecision struct {
	ShouldApply        bool
	NodeName           string
	InterferenceReason InterferenceReason
	Reason             string
}

// OnlinePodSource provides online pod cgroup paths for local node.
type OnlinePodSource interface {
	ListOnlinePodCgroups(ctx context.Context, nodeName string) ([]OnlinePodCgroup, error)
}

// OnlinePodPublisher sends online pod list to external agent.
type OnlinePodPublisher interface {
	PublishOnlinePods(ctx context.Context, req AgentAnalyzeRequest) error
}

// InterferenceResultSource provides interference analysis from external agent.
type InterferenceResultSource interface {
	GetInterference(ctx context.Context, nodeName string) (AgentAnalyzeResult, error)
}

// DynamicTuningPlanner converts agent analysis into local tuning decisions.
type DynamicTuningPlanner interface {
	Plan(ctx context.Context, nodeName string, result AgentAnalyzeResult) (DynamicTuningDecision, error)
}

// DynamicTuningApplier applies planned tuning decisions to local node.
type DynamicTuningApplier interface {
	Apply(ctx context.Context, decision DynamicTuningDecision) error
}

// DefaultDynamicTuningPlanner is a no-op planner placeholder for initial skeleton.
type DefaultDynamicTuningPlanner struct{}

// Plan returns no-op decision. Real tuning policy can be added later.
func (p DefaultDynamicTuningPlanner) Plan(_ context.Context, nodeName string, _ AgentAnalyzeResult) (DynamicTuningDecision, error) {
	return DynamicTuningDecision{
		ShouldApply: false,
		NodeName:    nodeName,
		Reason:      "default no-op planner",
	}, nil
}

// Coordinator orchestrates one dynamic-control cycle:
// collect online pods -> ask interference agent -> plan tuning -> apply tuning.
type Coordinator struct {
	NodeIdentity NodeIdentity
	OnlineSource OnlinePodSource
	Publisher    OnlinePodPublisher
	ResultSource InterferenceResultSource
	Planner      DynamicTuningPlanner
	Applier      DynamicTuningApplier
	Clock        func() time.Time
}

func (c *Coordinator) setDefaults() {
	if c.Planner == nil {
		c.Planner = DefaultDynamicTuningPlanner{}
	}
	if c.Clock == nil {
		c.Clock = time.Now
	}
}

// Validate checks mandatory dependencies for minimal runnable coordinator.
func (c *Coordinator) Validate() error {
	c.setDefaults()

	if c.NodeIdentity == nil {
		return fmt.Errorf("node identity must not be nil")
	}
	if c.OnlineSource == nil {
		return fmt.Errorf("online source must not be nil")
	}
	if c.Publisher == nil {
		return fmt.Errorf("publisher must not be nil")
	}
	if c.ResultSource == nil {
		return fmt.Errorf("result source must not be nil")
	}
	if c.Planner == nil {
		return fmt.Errorf("planner must not be nil")
	}
	if c.Applier == nil {
		return fmt.Errorf("applier must not be nil")
	}
	return nil
}

func (c *Coordinator) validatePublishDeps() error {
	c.setDefaults()
	if c.NodeIdentity == nil {
		return fmt.Errorf("node identity must not be nil")
	}
	if c.OnlineSource == nil {
		return fmt.Errorf("online source must not be nil")
	}
	if c.Publisher == nil {
		return fmt.Errorf("publisher must not be nil")
	}
	return nil
}

func (c *Coordinator) validateApplyDeps() error {
	c.setDefaults()
	if c.NodeIdentity == nil {
		return fmt.Errorf("node identity must not be nil")
	}
	if c.ResultSource == nil {
		return fmt.Errorf("result source must not be nil")
	}
	if c.Planner == nil {
		return fmt.Errorf("planner must not be nil")
	}
	if c.Applier == nil {
		return fmt.Errorf("applier must not be nil")
	}
	return nil
}

// PublishOnlinePodsOnce executes one publish cycle:
// collect online pod cgroup paths and publish to external agent.
func (c *Coordinator) PublishOnlinePodsOnce(ctx context.Context) error {
	if err := c.validatePublishDeps(); err != nil {
		return err
	}

	nodeName := c.NodeIdentity.NodeName()
	if nodeName == "" {
		return fmt.Errorf("node name must not be empty")
	}

	pods, err := c.OnlineSource.ListOnlinePodCgroups(ctx, nodeName)
	if err != nil {
		return err
	}

	return c.Publisher.PublishOnlinePods(ctx, AgentAnalyzeRequest{
		NodeName: nodeName,
		Time:     c.Clock(),
		Pods:     pods,
	})
}

// ApplyInterferenceOnce executes one apply cycle:
// pull interference result, plan tuning decision, and apply if needed.
func (c *Coordinator) ApplyInterferenceOnce(ctx context.Context) error {
	if err := c.validateApplyDeps(); err != nil {
		return err
	}

	nodeName := c.NodeIdentity.NodeName()
	if nodeName == "" {
		return fmt.Errorf("node name must not be empty")
	}

	result, err := c.ResultSource.GetInterference(ctx, nodeName)
	if err != nil {
		return err
	}

	decision, err := c.Planner.Plan(ctx, nodeName, result)
	if err != nil {
		return err
	}
	if !decision.ShouldApply {
		return nil
	}

	return c.Applier.Apply(ctx, decision)
}
