//go:build linux

package runtime

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"go.uber.org/zap"
)

func TestNewProcessLoggerRoutesInfraLogsToConfiguredWriter(t *testing.T) {
	var output bytes.Buffer
	log, err := newProcessLogger(&output, false)
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	log.Info(ctx, "runtime initializing", zap.String("stage", "runtime_init"))
	log.Debug(ctx, "hidden debug message")
	// Builder 接收的是 Detach 后的 zap core；这里同时锁定该路径仍写入同一个 sink。
	log.Detach(ctx).Info("builder entered", zap.String("stage", "builder"))
	if err := log.Sync(); err != nil {
		t.Fatal(err)
	}

	got := output.String()
	for _, want := range []string{"runtime initializing", "runtime_init", "builder entered", "builder"} {
		if !strings.Contains(got, want) {
			t.Errorf("logger output missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "hidden debug message") {
		t.Fatalf("info logger emitted debug entry:\n%s", got)
	}
	if strings.Contains(got, "\x1b[") {
		t.Fatalf("logger output contains terminal color escape:\n%q", got)
	}
}

func TestNewProcessLoggerEnablesDebugEntries(t *testing.T) {
	var output bytes.Buffer
	log, err := newProcessLogger(&output, true)
	if err != nil {
		t.Fatal(err)
	}

	log.Debug(context.Background(), "debug enabled", zap.String("stage", "test"))
	if err := log.Sync(); err != nil {
		t.Fatal(err)
	}
	if got := output.String(); !strings.Contains(got, "debug enabled") {
		t.Fatalf("debug logger output = %q, want debug entry", got)
	}
}
