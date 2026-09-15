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
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

func TestRunDevkitCommandPreservesDeadlineExceeded(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	_, err := runDevkitCommand(ctx, "sh", "-c", "exec sleep 1")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected context deadline error, got %v", err)
	}
}

func TestRunDevkitCommandStopsChildProcessOnTimeout(t *testing.T) {
	tempDir := t.TempDir()
	pidFile := filepath.Join(tempDir, "child.pid")
	script := filepath.Join(tempDir, "spawn-child.sh")
	contents := "#!/bin/sh\nsleep 3 &\necho $! > \"$1\"\nwait\n"
	if err := os.WriteFile(script, []byte(contents), 0o755); err != nil {
		t.Fatalf("write child script: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	startedAt := time.Now()
	_, err := runDevkitCommand(ctx, script, pidFile)
	elapsed := time.Since(startedAt)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected context deadline error, got %v", err)
	}
	if elapsed >= time.Second {
		t.Fatalf("CLI timeout returned after %s, want less than 1s", elapsed)
	}
	rawPID, readErr := os.ReadFile(pidFile)
	if readErr != nil {
		t.Fatalf("read child PID: %v", readErr)
	}
	childPID, parseErr := strconv.Atoi(strings.TrimSpace(string(rawPID)))
	if parseErr != nil {
		t.Fatalf("parse child PID: %v", parseErr)
	}
	defer func() {
		_ = syscall.Kill(childPID, syscall.SIGKILL) // Best-effort cleanup for the RED implementation.
	}()

	deadline := time.Now().Add(2 * time.Second)
	for processIsRunning(childPID) && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if processIsRunning(childPID) {
		t.Fatalf("child process %d is still running after CLI timeout", childPID)
	}
}

func processIsRunning(pid int) bool {
	stat, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "stat"))
	if err != nil {
		return false
	}
	fields := strings.Fields(string(stat))
	return len(fields) > 2 && fields[2] != "Z"
}

// TestDevkitCollectorsSerializeRealCLIExecutions exercises both collectors'
// real collectOnce paths. The injected runner blocks the first call so the test
// can prove that the second call cannot enter until the shared gate is released.
func TestDevkitCollectorsSerializeRealCLIExecutions(t *testing.T) {
	logFile, err := os.CreateTemp(t.TempDir(), "collector-*.log")
	if err != nil {
		t.Fatalf("create collector log: %v", err)
	}
	t.Cleanup(func() { _ = logFile.Close() })

	entries := make(chan string, 2)
	releases := make(chan struct{}, 2)
	runner := func(ctx context.Context, _ string, args ...string) (string, error) {
		command := strings.Join(args, " ")
		select {
		case entries <- command:
		case <-ctx.Done():
			return "", ctx.Err()
		}
		select {
		case <-releases:
			return "intentionally invalid report\n", nil
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}

	cfg := &DevkitConfig{}
	watcher := &DevkitConfigWatcher{}
	watcher.config.Store(cfg)
	logger := slog.New(slog.NewTextHandler(logFile, nil))

	topdown := &devkitTopdownCollector{
		watcher:    watcher,
		binaryPath: "fake-devkit",
		runCommand: runner,
		logger:     logger,
	}
	memory := &devkitMemoryCollector{
		configWatcher: watcher,
		binaryPath:    "fake-devkit",
		runCommand:    runner,
		logger:        logger,
	}

	start := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		<-start
		topdown.collectOnce()
	}()
	go func() {
		defer wg.Done()
		<-start
		memory.collectOnce()
	}()
	close(start)

	waitEntry := func(timeout time.Duration) string {
		t.Helper()
		select {
		case command := <-entries:
			return command
		case <-time.After(timeout):
			t.Fatalf("timed out waiting for fake CLI entry after %s", timeout)
			return ""
		}
	}

	firstCommand := waitEntry(time.Second)
	select {
	case secondCommand := <-entries:
		releases <- struct{}{}
		releases <- struct{}{}
		t.Fatalf("second CLI entered before the first was released: first=%q second=%q", firstCommand, secondCommand)
	case <-time.After(200 * time.Millisecond):
		// Expected: the second collectOnce is waiting at the process-wide gate.
	}

	releases <- struct{}{}
	secondCommand := waitEntry(time.Second)
	releases <- struct{}{}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("collectOnce calls did not finish after both runners were released")
	}

	commands := firstCommand + "\n" + secondCommand
	if count := strings.Count(commands, "tuner top-down"); count != 1 {
		t.Fatalf("expected one TopDown CLI call, got %d; calls:\n%s", count, commands)
	}
	if count := strings.Count(commands, "tuner memory"); count != 1 {
		t.Fatalf("expected one Memory CLI call, got %d; calls:\n%s", count, commands)
	}

	if err := logFile.Sync(); err != nil {
		t.Fatalf("sync collector log: %v", err)
	}
	logs, err := os.ReadFile(logFile.Name())
	if err != nil {
		t.Fatalf("read collector log: %v", err)
	}
	logText := string(logs)
	if strings.Contains(logText, "msg=collection_start") || strings.Contains(logText, "msg=collection_finish") {
		t.Fatalf("successful rounds emitted INFO start/finish logs:\n%s", logText)
	}
	if count := strings.Count(logText, "msg=devkit_collection_parameters_initialized"); count != 2 {
		t.Fatalf("expected two parameter initialization logs, got %d; logs:\n%s", count, logText)
	}
	for _, collectorName := range []string{"devkit-topdown", "devkit-memory"} {
		marker := "msg=devkit_collection_parameters_initialized collector=" + collectorName
		if count := strings.Count(logText, marker); count != 1 {
			t.Fatalf("expected one initialization log for %s, got %d; logs:\n%s", collectorName, count, logText)
		}
	}
	if !strings.Contains(logText, "cli_args=\"[tuner top-down") ||
		!strings.Contains(logText, "cli_args=\"[tuner memory") {
		t.Fatalf("structured logs do not contain both CLI argument lists:\n%s", logText)
	}

}
