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
)

// TuningEngine encapsulates interference handling end-to-end (plan + apply).
// ReasonDispatchTuningEngine is the default implementation.
type TuningEngine interface {
	EnsurePolicy(ctx context.Context, nodeName string) error
	HandleInterference(ctx context.Context, nodeName string, result AgentAnalyzeResult) error
}

// ReasonDispatchTuningEngine is a direct TuningEngine implementation.
// It forwards supported interference reasons to DynamicPolicyUpdater.
type ReasonDispatchTuningEngine struct {
	Updater DynamicPolicyUpdater
}

// NewReasonDispatchTuningEngine creates a reason-dispatch tuning engine.
func NewReasonDispatchTuningEngine(updater DynamicPolicyUpdater) *ReasonDispatchTuningEngine {
	return &ReasonDispatchTuningEngine{Updater: updater}
}

func (e *ReasonDispatchTuningEngine) EnsurePolicy(ctx context.Context, nodeName string) error {
	if e.Updater == nil {
		return fmt.Errorf("dynamic policy updater must not be nil")
	}
	if nodeName == "" {
		return fmt.Errorf("node name must not be empty")
	}
	return e.Updater.EnsurePolicy(ctx, nodeName)
}

func (e *ReasonDispatchTuningEngine) HandleInterference(
	ctx context.Context,
	nodeName string,
	result AgentAnalyzeResult,
) error {
	if e.Updater == nil {
		return fmt.Errorf("dynamic policy updater must not be nil")
	}
	if nodeName == "" {
		return fmt.Errorf("node name must not be empty")
	}

	reasons, _ := normalizeInterferenceReasons(result.Reasons, false)
	if len(reasons) == 0 {
		return nil
	}
	return e.Updater.ApplyReasons(ctx, nodeName, reasons)
}
