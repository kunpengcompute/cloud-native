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

// L3TuningExecutor applies L3 related tuning actions. Concrete operations are
// intentionally out of scope in this version.
type L3TuningExecutor interface {
	ApplyL3(ctx context.Context, decision DynamicTuningDecision) error
}

// MBTuningExecutor applies MB related tuning actions. Concrete operations are
// intentionally out of scope in this version.
type MBTuningExecutor interface {
	ApplyMB(ctx context.Context, decision DynamicTuningDecision) error
}

// CPUTuningExecutor applies CPU related tuning actions. Concrete operations are
// intentionally out of scope in this version.
type CPUTuningExecutor interface {
	ApplyCPU(ctx context.Context, decision DynamicTuningDecision) error
}

// Noop* executors are placeholders used before real system-level operations are implemented.
type NoopL3Executor struct{}
type NoopMBExecutor struct{}
type NoopCPUExecutor struct{}

func (e NoopL3Executor) ApplyL3(_ context.Context, _ DynamicTuningDecision) error   { return nil }
func (e NoopMBExecutor) ApplyMB(_ context.Context, _ DynamicTuningDecision) error   { return nil }
func (e NoopCPUExecutor) ApplyCPU(_ context.Context, _ DynamicTuningDecision) error { return nil }

// DynamicPolicyUpdater updates per-node dynamic QoSPolicy CR.
type DynamicPolicyUpdater interface {
	ApplyReason(ctx context.Context, nodeName string, reason InterferenceReason) error
}

// DynamicPolicy*Executor delegates concrete action to QoSPolicyDynamicUpdater.
// This keeps execution path uniform: dynamic module updates one QoSPolicy CR,
// and existing QoSPolicy reconciler applies the final resctrl group changes.
type DynamicPolicyL3Executor struct{ Updater DynamicPolicyUpdater }
type DynamicPolicyMBExecutor struct{ Updater DynamicPolicyUpdater }
type DynamicPolicyCPUExecutor struct{ Updater DynamicPolicyUpdater }

func (e DynamicPolicyL3Executor) ApplyL3(ctx context.Context, decision DynamicTuningDecision) error {
	if e.Updater == nil {
		return fmt.Errorf("dynamic policy updater must not be nil")
	}
	return e.Updater.ApplyReason(ctx, decision.NodeName, InterferenceReasonL3)
}

func (e DynamicPolicyMBExecutor) ApplyMB(ctx context.Context, decision DynamicTuningDecision) error {
	if e.Updater == nil {
		return fmt.Errorf("dynamic policy updater must not be nil")
	}
	return e.Updater.ApplyReason(ctx, decision.NodeName, InterferenceReasonMB)
}

func (e DynamicPolicyCPUExecutor) ApplyCPU(ctx context.Context, decision DynamicTuningDecision) error {
	if e.Updater == nil {
		return fmt.Errorf("dynamic policy updater must not be nil")
	}
	return e.Updater.ApplyReason(ctx, decision.NodeName, InterferenceReasonCPU)
}

// ReasonDispatchTuningApplier dispatches tuning execution by interference reason.
// It keeps orchestration logic in one place while concrete operations stay behind interfaces.
type ReasonDispatchTuningApplier struct {
	L3Executor  L3TuningExecutor
	MBExecutor  MBTuningExecutor
	CPUExecutor CPUTuningExecutor
}

func (a *ReasonDispatchTuningApplier) setDefaults() {
	if a.L3Executor == nil {
		a.L3Executor = NoopL3Executor{}
	}
	if a.MBExecutor == nil {
		a.MBExecutor = NoopMBExecutor{}
	}
	if a.CPUExecutor == nil {
		a.CPUExecutor = NoopCPUExecutor{}
	}
}

// Apply dispatches by decision.InterferenceReason.
func (a *ReasonDispatchTuningApplier) Apply(ctx context.Context, decision DynamicTuningDecision) error {
	a.setDefaults()
	if !decision.ShouldApply {
		return nil
	}

	reason := normalizeInterferenceReason(decision.InterferenceReason)
	switch reason {
	case InterferenceReasonL3:
		return a.L3Executor.ApplyL3(ctx, decision)
	case InterferenceReasonMB:
		return a.MBExecutor.ApplyMB(ctx, decision)
	case InterferenceReasonCPU:
		return a.CPUExecutor.ApplyCPU(ctx, decision)
	default:
		return fmt.Errorf("unsupported interference reason: %s", reason)
	}
}
