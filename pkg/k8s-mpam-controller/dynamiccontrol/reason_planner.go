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

// ReasonBasedTuningPlanner plans apply/no-apply based on interference reason.
// The first version keeps the policy intentionally simple:
// - apply on l3/mb/cpu
// - skip on unknown
type ReasonBasedTuningPlanner struct{}

// Plan returns a coarse decision from interference reason.
func (p ReasonBasedTuningPlanner) Plan(_ context.Context, nodeName string, result AgentAnalyzeResult) (DynamicTuningDecision, error) {
	if nodeName == "" {
		return DynamicTuningDecision{}, fmt.Errorf("node name must not be empty")
	}

	reason := normalizeInterferenceReason(result.Reason)
	switch reason {
	case InterferenceReasonL3, InterferenceReasonMB, InterferenceReasonCPU:
		return DynamicTuningDecision{
			ShouldApply:        true,
			NodeName:           nodeName,
			InterferenceReason: reason,
			Reason:             fmt.Sprintf("interference reason=%s", reason),
		}, nil
	default:
		return DynamicTuningDecision{
			ShouldApply:        false,
			NodeName:           nodeName,
			InterferenceReason: reason,
			Reason:             fmt.Sprintf("skip dynamic tuning for reason=%s", reason),
		}, nil
	}
}
