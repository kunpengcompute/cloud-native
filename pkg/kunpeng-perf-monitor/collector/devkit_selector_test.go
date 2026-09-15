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
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCanonicalizeDevkitCPUSelector(t *testing.T) {
	tests := map[string]string{
		"1,0":         "0-1",
		"8-2":         "2-8",
		"0,0":         "0",
		"0-3,2-5":     "0-5",
		"10,8-9,7":    "7-10",
		"10,80,89-99": "10,80,89-99",
	}
	for raw, want := range tests {
		t.Run(raw, func(t *testing.T) {
			got, err := canonicalizeDevkitCPUSelector(raw, 1024)
			if err != nil {
				t.Fatalf("canonicalize CPU selector: %v", err)
			}
			if got != want {
				t.Fatalf("canonical CPU selector = %q, want %q", got, want)
			}
		})
	}
}

func TestCanonicalizeDevkitPIDSelector(t *testing.T) {
	got, err := canonicalizeDevkitPIDSelector("728174,1,1", 4194304)
	if err != nil {
		t.Fatalf("canonicalize PID selector: %v", err)
	}
	if got != "1,728174" {
		t.Fatalf("canonical PID selector = %q, want 1,728174", got)
	}
}

func TestDevkitSelectorRejectsResourceAndNumericExtremes(t *testing.T) {
	cpuAtLengthAndElementLimits := strings.Repeat("0,", 511) + "00"
	if got := len(cpuAtLengthAndElementLimits); got != 1024 {
		t.Fatalf("test CPU selector length = %d, want 1024", got)
	}
	if got := strings.Count(cpuAtLengthAndElementLimits, ",") + 1; got != 512 {
		t.Fatalf("test CPU selector elements = %d, want 512", got)
	}
	if _, err := canonicalizeDevkitCPUSelector(cpuAtLengthAndElementLimits, 1024); err != nil {
		t.Fatalf("CPU selector at length and element limits was rejected: %v", err)
	}

	pidAtLengthLimit := strings.Repeat("0", 511) + "1"
	if got := len(pidAtLengthLimit); got != 512 {
		t.Fatalf("test PID selector length = %d, want 512", got)
	}
	if got, err := canonicalizeDevkitPIDSelector(pidAtLengthLimit, 4194304); err != nil || got != "1" {
		t.Fatalf("PID selector at length limit = %q, %v; want 1, nil", got, err)
	}
	pidAtElementLimit := strings.TrimSuffix(strings.Repeat("1,", 32), ",")
	if got, err := canonicalizeDevkitPIDSelector(pidAtElementLimit, 4194304); err != nil || got != "1" {
		t.Fatalf("PID selector at element limit = %q, %v; want 1, nil", got, err)
	}

	tests := []struct {
		name    string
		parse   func() error
		wantErr string
	}{
		{name: "CPU 1025 characters", parse: func() error {
			_, err := canonicalizeDevkitCPUSelector(strings.Repeat("0,", 511)+"000", 1024)
			return err
		}, wantErr: "length_limit"},
		{name: "PID 513 characters", parse: func() error {
			_, err := canonicalizeDevkitPIDSelector(strings.Repeat("1", 513), 4194304)
			return err
		}, wantErr: "length_limit"},
		{name: "PID 33 elements", parse: func() error {
			_, err := canonicalizeDevkitPIDSelector(strings.TrimSuffix(strings.Repeat("1,", 33), ","), 4194304)
			return err
		}, wantErr: "element_limit"},
		{name: "CPU uint64 overflow", parse: func() error {
			_, err := canonicalizeDevkitCPUSelector("18446744073709551616", 1024)
			return err
		}, wantErr: "overflow"},
		{name: "PID uint64 overflow", parse: func() error {
			_, err := canonicalizeDevkitPIDSelector("18446744073709551616", 4194304)
			return err
		}, wantErr: "overflow"},
		{name: "PID zero", parse: func() error {
			_, err := canonicalizeDevkitPIDSelector("0", 4194304)
			return err
		}, wantErr: "value_out_of_range"},
		{name: "CPU above maximum", parse: func() error {
			_, err := canonicalizeDevkitCPUSelector("1025", 1024)
			return err
		}, wantErr: "value_out_of_range"},
		{name: "PID above maximum", parse: func() error {
			_, err := canonicalizeDevkitPIDSelector("4194305", 4194304)
			return err
		}, wantErr: "value_out_of_range"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.parse(); err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error = %v, want substring %q", err, tt.wantErr)
			}
		})
	}
}

func TestDevkitCPUSelectorElementLimitCountsRawElements(t *testing.T) {
	// 513 个空元素组成的字符串未超过 CPU 字符上限，可单独验证元素数门禁
	// 先于单个元素的格式校验生效。
	_, err := canonicalizeDevkitCPUSelector(strings.Repeat(",", 512), 1024)
	if err == nil || !strings.Contains(err.Error(), "element_limit") || !strings.Contains(err.Error(), "512") {
		t.Fatalf("CPU element limit error = %v, want element_limit at 512 elements", err)
	}
}

func TestDevkitSelectorUpperBounds(t *testing.T) {
	tempDir := t.TempDir()
	possiblePath := filepath.Join(tempDir, "possible")
	pidMaxPath := filepath.Join(tempDir, "pid_max")
	if err := os.WriteFile(possiblePath, []byte("0-383\n"), 0o644); err != nil {
		t.Fatalf("write possible CPUs: %v", err)
	}
	if err := os.WriteFile(pidMaxPath, []byte("1048576\n"), 0o644); err != nil {
		t.Fatalf("write pid_max: %v", err)
	}

	originalPossiblePath, originalPIDMaxPath := devkitCPUPossiblePath, devkitPIDMaxPath
	originalCPUFallback, originalPIDFallback := fallbackMaxDevkitCPU, fallbackMaxDevkitPID
	t.Cleanup(func() {
		devkitCPUPossiblePath, devkitPIDMaxPath = originalPossiblePath, originalPIDMaxPath
		fallbackMaxDevkitCPU, fallbackMaxDevkitPID = originalCPUFallback, originalPIDFallback
	})
	devkitCPUPossiblePath, devkitPIDMaxPath = possiblePath, pidMaxPath
	fallbackMaxDevkitCPU, fallbackMaxDevkitPID = 1024, 4194304

	if got := devkitMaxCPU(); got != 383 {
		t.Fatalf("max CPU = %d, want 383", got)
	}
	if got := devkitMaxPID(); got != 1048576 {
		t.Fatalf("max PID = %d, want 1048576", got)
	}
	devkitCPUPossiblePath, devkitPIDMaxPath = filepath.Join(tempDir, "missing-cpu"), filepath.Join(tempDir, "missing-pid")
	if got := devkitMaxCPU(); got != 1024 {
		t.Fatalf("fallback max CPU = %d, want 1024", got)
	}
	if got := devkitMaxPID(); got != 4194304 {
		t.Fatalf("fallback max PID = %d, want 4194304", got)
	}
}
