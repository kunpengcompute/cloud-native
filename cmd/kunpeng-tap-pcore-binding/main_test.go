/*
 * Copyright (c) 2026 Huawei Technology corp.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package main

import (
	"flag"
	"io"
	"os"
	"reflect"
	"testing"
	"time"
)

func TestSplitCSV(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want []string
	}{
		{name: "empty", raw: "", want: nil},
		{name: "trim and skip empty", raw: " default, ,kube-system ", want: []string{"default", "kube-system"}},
		{name: "preserve duplicates", raw: "kata,kata", want: []string{"kata", "kata"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := splitCSV(tt.raw); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("splitCSV(%q) = %v, want %v", tt.raw, got, tt.want)
			}
		})
	}
}

func TestParseConfig(t *testing.T) {
	oldCommandLine := flag.CommandLine
	oldArgs := os.Args
	t.Cleanup(func() {
		flag.CommandLine = oldCommandLine
		os.Args = oldArgs
	})

	flag.CommandLine = flag.NewFlagSet("pcore-binding-test", flag.ContinueOnError)
	flag.CommandLine.SetOutput(io.Discard)
	os.Args = []string{
		"kunpeng-tap-pcore-binding",
		"--nri-socket-path=/tmp/nri-test.sock",
		"--scan-interval=3s",
		"--cgroup-root=/tmp/cgroup-test",
		"--namespace-whitelist=default, kube-system,,",
		"--runtimeclass-whitelist=kata-clh, kata-qemu",
		"--dry-run=true",
	}

	got := parseConfig()
	if got.SocketPath != "/tmp/nri-test.sock" {
		t.Fatalf("SocketPath = %q, want %q", got.SocketPath, "/tmp/nri-test.sock")
	}
	if got.ScanInterval != 3*time.Second {
		t.Fatalf("ScanInterval = %s, want 3s", got.ScanInterval)
	}
	if got.CgroupRoot != "/tmp/cgroup-test" {
		t.Fatalf("CgroupRoot = %q, want %q", got.CgroupRoot, "/tmp/cgroup-test")
	}
	if want := []string{"default", "kube-system"}; !reflect.DeepEqual(got.Namespaces, want) {
		t.Fatalf("Namespaces = %v, want %v", got.Namespaces, want)
	}
	if want := []string{"kata-clh", "kata-qemu"}; !reflect.DeepEqual(got.RuntimeClasses, want) {
		t.Fatalf("RuntimeClasses = %v, want %v", got.RuntimeClasses, want)
	}
	if !got.DryRun {
		t.Fatal("DryRun = false, want true")
	}
}
