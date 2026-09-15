// Copyright (c) 2025 Huawei Technology corp.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

//go:build linux

package collector

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

func TestParseMemoryFixtures(t *testing.T) {
	tests := []struct {
		name    string
		fixture string
		attempt devkitAttemptMetadata
		wantCPU string
	}{
		{name: "system", fixture: sanitizedMemoryFixture("all", "4.50"), attempt: devkitAttemptMetadata{targetType: "system", target: "system", periodMilliseconds: 1000}, wantCPU: "all"},
		{name: "cpu", fixture: sanitizedMemoryFixture("2", "5.25"), attempt: devkitAttemptMetadata{targetType: "cpu", target: "cpu2,4-5", targetValue: "2,4-5", periodMilliseconds: 1000}, wantCPU: "2"},
		{name: "period_100", fixture: sanitizedMemoryFixture("2", "5.25"), attempt: devkitAttemptMetadata{targetType: "cpu", target: "cpu2,4-5", targetValue: "2,4-5", periodMilliseconds: 100}, wantCPU: "2"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed, err := parseMemoryOutput(tt.fixture, tt.attempt, nil)
			if err != nil {
				t.Fatalf("parse fixture: %v", err)
			}
			if !parsed.success || parsed.targetType != tt.attempt.targetType || parsed.target != tt.attempt.target || parsed.periodMilliseconds != tt.attempt.periodMilliseconds {
				t.Fatalf("unexpected status or metadata: %+v", parsed)
			}
			if len(parsed.cacheMiss) != 4 {
				t.Fatalf("cache miss member count = %d, want 4", len(parsed.cacheMiss))
			}
			if len(parsed.ddrSystem) != 2 {
				t.Fatalf("DDR system member count = %d, want 2", len(parsed.ddrSystem))
			}
			if !memoryAccessHasCPU(parsed.access, tt.wantCPU) {
				t.Fatalf("missing Access cells for CPU %q", tt.wantCPU)
			}
			if len(parsed.l3) == 0 || len(parsed.ddrc) == 0 {
				t.Fatalf("missing dynamic topology data: l3=%d ddrc=%d", len(parsed.l3), len(parsed.ddrc))
			}
		})
	}
}

func TestParseMemoryPreservesNAHalfCell(t *testing.T) {
	parsed, err := parseMemoryOutput(sanitizedMemoryFixture("all", "4.50"), devkitAttemptMetadata{targetType: "system", target: "system", periodMilliseconds: 1000}, nil)
	if err != nil {
		t.Fatalf("parse fixture: %v", err)
	}

	for _, cell := range parsed.access {
		if cell.cpu == "all" && cell.component == "l2d_tlb" {
			if cell.hasBW {
				t.Fatal("N/A bandwidth was exposed as a value")
			}
			if !cell.hasHit || cell.hitPercent != 85.30 {
				t.Fatalf("hit half-cell = (%v, %v), want (true, 85.30)", cell.hasHit, cell.hitPercent)
			}
			return
		}
	}
	t.Fatal("missing l2d_tlb Access cell")
}

func TestParseMemoryRequiresEveryCacheMissMember(t *testing.T) {
	fixture := replaceMemoryFixtureOnce(
		t,
		sanitizedMemoryFixture("all", "4.50"),
		"L2I        23.82%\n",
		"",
	)

	_, err := parseMemoryOutput(fixture, devkitAttemptMetadata{targetType: "system", target: "system", periodMilliseconds: 1000}, nil)
	if err == nil || !strings.Contains(err.Error(), "required Cache Miss component is missing: l2i") {
		t.Fatalf("error = %v, want missing l2i error", err)
	}
}

func TestParseMemoryRequiresBothDDRSystemDirections(t *testing.T) {
	fixture := replaceMemoryFixtureOnce(
		t,
		sanitizedMemoryFixture("all", "4.50"),
		"ddrc_write        154.49MB/s\n",
		"",
	)

	_, err := parseMemoryOutput(fixture, devkitAttemptMetadata{targetType: "system", target: "system", periodMilliseconds: 1000}, nil)
	if err == nil || !strings.Contains(err.Error(), "DDR system is missing required operation: ddrc_write") {
		t.Fatalf("error = %v, want missing ddrc_write error", err)
	}
}

func TestParseMemoryWarnsForMultipleReportsAndUsesLast(t *testing.T) {
	first := sanitizedMemoryFixture("all", "4.50")
	last := sanitizedMemoryFixture("2", "5.25")
	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, nil))

	parsed, err := parseMemoryOutput(first+"\n"+last, devkitAttemptMetadata{targetType: "cpu", target: "cpu2,4-5", targetValue: "2,4-5", periodMilliseconds: 1000}, logger)
	if err != nil {
		t.Fatalf("parse multiple reports: %v", err)
	}
	if len(parsed.cacheMiss) == 0 || parsed.cacheMiss[0].component != "l1d" || parsed.cacheMiss[0].percent != 5.25 {
		t.Fatalf("selected report does not match last fixture: %+v", parsed.cacheMiss)
	}
	if !strings.Contains(logs.String(), "multiple_memory_reports") {
		t.Fatalf("missing multiple report warning:\n%s", logs.String())
	}
}

func TestParseMemoryRejectsLastReportWithoutCommand(t *testing.T) {
	first := sanitizedMemoryFixture("all", "4.50")
	last := removeLineContaining(sanitizedMemoryFixture("all", "4.50"), "Command")

	_, err := parseMemoryOutput(first+"\n"+last, devkitAttemptMetadata{targetType: "system", target: "system", periodMilliseconds: 1000}, nil)
	if err == nil || !strings.Contains(err.Error(), "missing Command metadata") {
		t.Fatalf("error = %v, want selected report missing Command error", err)
	}
}

func TestCrossCheckMemoryUsesFixedToleranceAndDiagnosticFields(t *testing.T) {
	t.Run("difference_at_tolerance_does_not_warn", func(t *testing.T) {
		var logs bytes.Buffer
		cache := &memoryMetricCache{l3: []memoryL3Row{
			{node: "0", ccl: "all", readBandwidth: 10.5, readHitBandwidth: 20},
			{node: "0", ccl: "0", readBandwidth: 10, readHitBandwidth: 20},
		}}

		crossCheckMemory(cache, slog.New(slog.NewTextHandler(&logs, nil)))
		if logs.Len() != 0 {
			t.Fatalf("difference equal to tolerance produced warning: %s", logs.String())
		}
	})

	t.Run("difference_above_tolerance_warns_with_fields", func(t *testing.T) {
		var logs bytes.Buffer
		cache := &memoryMetricCache{ddrc: []memoryDDRCCell{
			{node: "1", ddrc: "total", read: 10.6, write: 20},
			{node: "1", ddrc: "0", read: 10, write: 20},
		}}

		crossCheckMemory(cache, slog.New(slog.NewTextHandler(&logs, nil)))
		for _, required := range []string{
			"memory_aggregate_mismatch",
			"table=DDRC",
			"node=1",
			"total=10.6",
			"sum=10",
			"difference=",
			"tolerance=0.5",
		} {
			if !strings.Contains(logs.String(), required) {
				t.Fatalf("warning missing %q:\n%s", required, logs.String())
			}
		}
	})
}

func memoryAccessHasCPU(cells []memoryAccessCell, cpu string) bool {
	for _, cell := range cells {
		if cell.cpu == cpu {
			return true
		}
	}
	return false
}

func replaceMemoryFixtureOnce(t *testing.T, text, old, replacement string) string {
	t.Helper()
	if !strings.Contains(text, old) {
		t.Fatalf("fixture does not contain %q", old)
	}
	return strings.Replace(text, old, replacement, 1)
}
