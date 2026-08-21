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
	calls       int
	lastNode    string
	lastReasons []InterferenceReason
	err         error
}

func (u *fakeDynamicPolicyUpdater) ApplyReasons(
	_ context.Context,
	nodeName string,
	reasons []InterferenceReason,
) error {
	u.calls++
	u.lastNode = nodeName
	u.lastReasons = reasons
	return u.err
}

func TestReasonDispatchTuningEngineHandleInterference(t *testing.T) {
	t.Run("dispatch normalized unique actionable reasons once", func(t *testing.T) {
		updater := &fakeDynamicPolicyUpdater{}
		engine := NewReasonDispatchTuningEngine(updater)
		result := AgentAnalyzeResult{Reasons: []InterferenceReason{
			InterferenceReasonL3,
			InterferenceReasonMB,
			InterferenceReasonL3,
			InterferenceReasonNone,
			InterferenceReason(" CPU "),
			InterferenceReason("unsupported"),
		}}
		if err := engine.HandleInterference(context.Background(), "node-a", result); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := []InterferenceReason{
			InterferenceReasonCPU,
			InterferenceReasonMB,
			InterferenceReasonL3,
		}
		if updater.calls != 1 || updater.lastNode != "node-a" {
			t.Fatalf("unexpected updater call: %+v", updater)
		}
		if len(updater.lastReasons) != len(want) {
			t.Fatalf("unexpected reasons: got %v, want %v", updater.lastReasons, want)
		}
		for i := range want {
			if updater.lastReasons[i] != want[i] {
				t.Fatalf("unexpected reasons: got %v, want %v", updater.lastReasons, want)
			}
		}
	})

	for _, tt := range []struct {
		name    string
		reasons []InterferenceReason
	}{
		{name: "empty reasons"},
		{name: "none reason", reasons: []InterferenceReason{InterferenceReasonNone}},
		{name: "unsupported reasons", reasons: []InterferenceReason{"unknown", "other"}},
	} {
		t.Run(tt.name+" noop", func(t *testing.T) {
			updater := &fakeDynamicPolicyUpdater{}
			engine := NewReasonDispatchTuningEngine(updater)
			if err := engine.HandleInterference(
				context.Background(),
				"node-a",
				AgentAnalyzeResult{Reasons: tt.reasons},
			); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if updater.calls != 0 {
				t.Fatalf("updater should not be called")
			}
		})
	}

	t.Run("nil updater", func(t *testing.T) {
		engine := &ReasonDispatchTuningEngine{}
		if err := engine.HandleInterference(
			context.Background(),
			"node-a",
			AgentAnalyzeResult{Reasons: []InterferenceReason{InterferenceReasonL3}},
		); err == nil {
			t.Fatalf("expected error")
		}
	})

	t.Run("propagate updater error", func(t *testing.T) {
		want := errors.New("apply failed")
		updater := &fakeDynamicPolicyUpdater{err: want}
		engine := NewReasonDispatchTuningEngine(updater)
		err := engine.HandleInterference(
			context.Background(),
			"node-a",
			AgentAnalyzeResult{Reasons: []InterferenceReason{InterferenceReasonCPU}},
		)
		if !errors.Is(err, want) {
			t.Fatalf("expected %v, got %v", want, err)
		}
	})
}
