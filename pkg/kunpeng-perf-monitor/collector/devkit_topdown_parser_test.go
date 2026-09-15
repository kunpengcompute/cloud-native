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

func TestParseTopdownSanitizedFixture(t *testing.T) {
	fixture := sanitizedTopdownFixture("the system")

	parsed, err := parseTopdownOutput(fixture, devkitAttemptMetadata{targetType: "system", target: "system"}, nil)
	if err != nil {
		t.Fatalf("parse sanitized fixture: %v", err)
	}

	const wantPath = "bad_speculation.branch_mispredicts.indirect_branch"
	for _, node := range parsed.nodes {
		if node.path == wantPath {
			if node.level != 3 {
				t.Fatalf("Indirect Branch level = %d, want 3", node.level)
			}
			return
		}
	}
	t.Fatalf("missing expected TopDown node path %q", wantPath)
}

func TestParseTopdownScopeFromReport(t *testing.T) {
	tests := []struct {
		name       string
		fixture    string
		attempt    devkitAttemptMetadata
		wantType   string
		wantTarget string
	}{
		{name: "system", fixture: sanitizedTopdownFixture("the system"), attempt: devkitAttemptMetadata{targetType: "system", target: "system"}, wantType: "system", wantTarget: "system"},
		{name: "cpu", fixture: sanitizedTopdownFixture("CPU(s) 2,4-5"), attempt: devkitAttemptMetadata{targetType: "cpu", target: "cpu2,4-5", targetValue: "2,4-5"}, wantType: "cpu", wantTarget: "cpu2,4-5"},
		{name: "pid", fixture: sanitizedTopdownFixture("process id '42'"), attempt: devkitAttemptMetadata{targetType: "pid", target: "pid42", targetValue: "42"}, wantType: "pid", wantTarget: "pid42"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed, err := parseTopdownOutput(tt.fixture, tt.attempt, nil)
			if err != nil {
				t.Fatalf("parse fixture: %v", err)
			}
			if parsed.targetType != tt.wantType {
				t.Fatalf("targetType = %q, want %q", parsed.targetType, tt.wantType)
			}
			if parsed.target != tt.wantTarget {
				t.Fatalf("target = %q, want %q", parsed.target, tt.wantTarget)
			}
		})
	}
}

func TestParseTopdownAcceptsCanonicalCPUScopeAndPreservesRawTarget(t *testing.T) {
	fixture := sanitizedTopdownFixture("CPU(s) 2,4-5")
	fixture = strings.Replace(fixture, "Top-down metrics of CPU(s) 2,4-5:", "Top-down metrics of CPU(s) 0-1:", 1)
	attempt := devkitAttemptMetadata{targetType: "cpu", target: "cpu1,0", targetValue: "1,0"}

	parsed, err := parseTopdownOutput(fixture, attempt, nil)
	if err != nil {
		t.Fatalf("parse canonical CPU scope: %v", err)
	}
	if parsed.target != "cpu1,0" {
		t.Fatalf("target = %q, want raw cpu1,0", parsed.target)
	}
}

func TestParseTopdownDoesNotRequireReportTitle(t *testing.T) {
	fixture := sanitizedTopdownFixture("the system")
	fixture = removeLineContaining(fixture, "TOP-DOWN Summary Report-ALL")

	parsed, err := parseTopdownOutput(fixture, devkitAttemptMetadata{targetType: "system", target: "system"}, nil)
	if err != nil {
		t.Fatalf("parse fixture without report title: %v", err)
	}
	if parsed.targetType != "system" {
		t.Fatalf("targetType = %q, want system", parsed.targetType)
	}
}

func TestParseTopdownRequiresRecognizedScope(t *testing.T) {
	fixture := sanitizedTopdownFixture("the system")
	tests := []struct {
		name   string
		output string
	}{
		{name: "missing", output: removeLineContaining(fixture, "Top-down metrics of the system:")},
		{name: "unknown", output: strings.Replace(fixture, "Top-down metrics of the system:", "Top-down metrics of workload foo:", 1)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := parseTopdownOutput(tt.output, devkitAttemptMetadata{targetType: "system", target: "system"}, nil); err == nil {
				t.Fatal("expected scope parse error")
			}
		})
	}
}

func TestParseTopdownWarnsForMultipleReports(t *testing.T) {
	fixture := sanitizedTopdownFixture("the system")
	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, nil))

	parsed, err := parseTopdownOutput(fixture+"\n"+fixture, devkitAttemptMetadata{targetType: "system", target: "system"}, logger)
	if err != nil {
		t.Fatalf("parse multiple reports: %v", err)
	}
	if parsed.targetType != "system" {
		t.Fatalf("targetType = %q, want system", parsed.targetType)
	}
	if !strings.Contains(logs.String(), "multiple_topdown_reports") {
		t.Fatalf("missing multiple report warning in logs:\n%s", logs.String())
	}
}

func TestParseTopdownRejectsLastReportWithoutCommand(t *testing.T) {
	first := sanitizedTopdownFixture("the system")
	last := removeLineContaining(sanitizedTopdownFixture("the system"), "Command")

	_, err := parseTopdownOutput(first+"\n"+last, devkitAttemptMetadata{targetType: "system", target: "system"}, nil)
	if err == nil || !strings.Contains(err.Error(), "missing Command metadata") {
		t.Fatalf("error = %v, want selected report missing Command error", err)
	}
}

func TestParseTopdownRejectsScopeMismatch(t *testing.T) {
	fixture := sanitizedTopdownFixture("the system")

	_, err := parseTopdownOutput(fixture, devkitAttemptMetadata{targetType: "cpu", target: "cpu0-2", targetValue: "0-2"}, nil)
	if err == nil || !strings.Contains(err.Error(), "TopDown scope does not match current argv") {
		t.Fatalf("error = %v, want scope mismatch error", err)
	}
}

func removeLineContaining(text, marker string) string {
	lines := strings.Split(text, "\n")
	kept := lines[:0]
	for _, line := range lines {
		if !strings.Contains(line, marker) {
			kept = append(kept, line)
		}
	}
	return strings.Join(kept, "\n")
}
