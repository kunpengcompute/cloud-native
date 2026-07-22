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
	"errors"
	"testing"
)

type fakeDynamicPolicyUpdater struct {
	calls      int
	lastNode   string
	lastReason InterferenceReason
	err        error
}

func (u *fakeDynamicPolicyUpdater) ApplyReason(_ context.Context, nodeName string, reason InterferenceReason) error {
	u.calls++
	u.lastNode = nodeName
	u.lastReason = reason
	return u.err
}

func TestReasonDispatchTuningEngineHandleInterference(t *testing.T) {
	t.Run("dispatch", func(t *testing.T) {
		updater := &fakeDynamicPolicyUpdater{}
		engine := NewReasonDispatchTuningEngine(updater)
		if err := engine.HandleInterference(context.Background(), "node-a", AgentAnalyzeResult{Reason: InterferenceReasonMB}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if updater.calls != 1 || updater.lastNode != "node-a" || updater.lastReason != InterferenceReasonMB {
			t.Fatalf("unexpected updater call: %+v", updater)
		}
	})

	t.Run("unknown reason noop", func(t *testing.T) {
		updater := &fakeDynamicPolicyUpdater{}
		engine := NewReasonDispatchTuningEngine(updater)
		if err := engine.HandleInterference(context.Background(), "node-a", AgentAnalyzeResult{Reason: InterferenceReasonUnknown}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if updater.calls != 0 {
			t.Fatalf("updater should not be called")
		}
	})

	t.Run("none reason noop", func(t *testing.T) {
		updater := &fakeDynamicPolicyUpdater{}
		engine := NewReasonDispatchTuningEngine(updater)
		if err := engine.HandleInterference(context.Background(), "node-a", AgentAnalyzeResult{Reason: InterferenceReasonNone}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if updater.calls != 0 {
			t.Fatalf("updater should not be called")
		}
	})

	t.Run("nil updater", func(t *testing.T) {
		engine := &ReasonDispatchTuningEngine{}
		if err := engine.HandleInterference(context.Background(), "node-a", AgentAnalyzeResult{Reason: InterferenceReasonL3}); err == nil {
			t.Fatalf("expected error")
		}
	})

	t.Run("propagate updater error", func(t *testing.T) {
		want := errors.New("apply failed")
		updater := &fakeDynamicPolicyUpdater{err: want}
		engine := NewReasonDispatchTuningEngine(updater)
		err := engine.HandleInterference(context.Background(), "node-a", AgentAnalyzeResult{Reason: InterferenceReasonCPU})
		if !errors.Is(err, want) {
			t.Fatalf("expected %v, got %v", want, err)
		}
	})
}
