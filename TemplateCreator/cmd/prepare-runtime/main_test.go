package main

import (
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestParseFlagsAcceptsRuntimeArguments(t *testing.T) {
	manifest := filepath.Join(t.TempDir(), "manifest.json")
	runtimeRoot := filepath.Join(t.TempDir(), "runtime")
	infraRoot := filepath.Join(t.TempDir(), "infra-root")

	request, err := parseFlags([]string{
		"-manifest", manifest,
		"-runtime-root", runtimeRoot,
		"-arch", "arm64",
		"-e2b-infra-root", infraRoot,
		"-timeout", "45s",
	})
	if err != nil {
		t.Fatalf("parseFlags() error = %v", err)
	}
	if request.ManifestPath != manifest || request.RuntimeRoot != runtimeRoot || request.Architecture != "arm64" || request.E2BInfraRoot != infraRoot || request.Timeout != 45*time.Second {
		t.Fatalf("parseFlags() = %#v", request)
	}
}

func TestParseFlagsDefaultsArchitectureAndTimeout(t *testing.T) {
	request, err := parseFlags([]string{
		"-manifest", filepath.Join(t.TempDir(), "manifest.json"),
		"-runtime-root", filepath.Join(t.TempDir(), "runtime"),
	})
	if err != nil {
		t.Fatalf("parseFlags() error = %v", err)
	}
	if request.Architecture != runtime.GOARCH {
		t.Fatalf("Architecture = %q, want %q", request.Architecture, runtime.GOARCH)
	}
	if request.Timeout != 5*time.Minute {
		t.Fatalf("Timeout = %s, want 5m", request.Timeout)
	}
}

func TestParseFlagsRequiresManifestAndRuntimeRoot(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "manifest", args: []string{"-runtime-root", t.TempDir()}, want: "-manifest"},
		{name: "runtime root", args: []string{"-manifest", filepath.Join(t.TempDir(), "manifest.json")}, want: "-runtime-root"},
		{name: "positional", args: []string{"-manifest", "/manifest.json", "-runtime-root", "/runtime", "extra"}, want: "positional"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := parseFlags(test.args)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("parseFlags() error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestRunReportsArgumentErrorWithoutPreparingRuntime(t *testing.T) {
	var stdout bytes.Buffer
	err := run(context.Background(), []string{"-runtime-root", t.TempDir()}, &stdout)
	if err == nil || !strings.Contains(err.Error(), "-manifest") {
		t.Fatalf("run() error = %v, want -manifest error", err)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
}

func TestRunHelp(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "short", args: []string{"-h"}},
		{name: "long", args: []string{"--help"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var stdout bytes.Buffer
			err := run(context.Background(), test.args, &stdout)
			if err != nil {
				t.Fatalf("run() error = %v, want nil", err)
			}

			help := stdout.String()
			for _, want := range []string{
				"prepare-runtime",
				"用途",
				"用法",
				"必填参数",
				"可选参数",
				"-manifest",
				"-runtime-root",
				"-arch",
				"-e2b-infra-root",
				"-timeout",
				"示例",
				"结果",
				"-manifest \"$PWD/runtime-assets/manifest.json\"",
				"/opt/templatecreator/runtime",
				"$(go env GOARCH)",
				"> /opt/templatecreator/runtime-info.json",
				"runtime-info.json",
				"runtime manifest 的绝对路径",
				"安装 runtime 资产的绝对目录",
				"若提供，必须为绝对路径",
			} {
				if !strings.Contains(help, want) {
					t.Errorf("help output does not contain %q:\n%s", want, help)
				}
			}
			if strings.Contains(help, "flag: help requested") {
				t.Fatalf("help output contains flag.ErrHelp: %s", help)
			}
		})
	}
}

func TestRunHelpWriterFailure(t *testing.T) {
	wantErr := errors.New("help writer failed")
	err := run(context.Background(), []string{"-h"}, failingWriter{err: wantErr})
	if !errors.Is(err, wantErr) {
		t.Fatalf("run() error = %v, want error wrapping %v", err, wantErr)
	}
}

func TestRunParseErrorKeepsOutputEmpty(t *testing.T) {
	var stdout bytes.Buffer
	err := run(context.Background(), []string{"-unknown"}, &stdout)
	if err == nil || !strings.Contains(err.Error(), "flag provided but not defined") {
		t.Fatalf("run() error = %v, want unknown flag error", err)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
}

func TestRunHelpDocumentsAllFlagsAndDefaults(t *testing.T) {
	request, err := parseFlags([]string{
		"-manifest", filepath.Join(t.TempDir(), "manifest.json"),
		"-runtime-root", filepath.Join(t.TempDir(), "runtime"),
	})
	if err != nil {
		t.Fatalf("parseFlags() error = %v", err)
	}

	var stdout bytes.Buffer
	if err = run(context.Background(), []string{"--help"}, &stdout); err != nil {
		t.Fatalf("run() error = %v, want nil", err)
	}

	help := stdout.String()
	for _, flagName := range []string{
		"-manifest",
		"-runtime-root",
		"-arch",
		"-e2b-infra-root",
		"-timeout",
		"-h, --help",
	} {
		if count := strings.Count(help, flagName); count == 0 {
			t.Errorf("help output documents %s %d times, want at least once", flagName, count)
		}
	}
	for _, defaultValue := range []string{runtime.GOARCH, request.Timeout.String()} {
		if !strings.Contains(help, defaultValue) {
			t.Errorf("help output does not contain default value %q:\n%s", defaultValue, help)
		}
	}
}

type failingWriter struct {
	err error
}

func (writer failingWriter) Write([]byte) (int, error) {
	return 0, writer.err
}
