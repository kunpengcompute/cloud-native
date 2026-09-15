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
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"
)

func TestDevkitCollectionLogStateKeepsSuccessfulRoundsQuietAtInfo(t *testing.T) {
	var logs bytes.Buffer
	state := newDevkitCollectionLogState(slog.New(slog.NewTextHandler(&logs, nil)), "devkit-topdown")
	now := time.Unix(100, 0)
	state.now = func() time.Time { return now }

	attempt := state.start(1, "/opt/devkit/devkit", []string{"tuner", "top-down", "-d", "3"}, "target", "system")
	state.finish(attempt, nil)
	now = now.Add(time.Minute)
	attempt = state.start(2, "/opt/devkit/devkit", []string{"tuner", "top-down", "-d", "3"}, "target", "system")
	state.finish(attempt, nil)

	logText := logs.String()
	if count := strings.Count(logText, "devkit_collection_parameters_initialized"); count != 1 {
		t.Fatalf("parameter initialization logs = %d, want 1; logs:\n%s", count, logText)
	}
	if strings.Contains(logText, "collection_start") || strings.Contains(logText, "collection_finish") {
		t.Fatalf("successful rounds emitted INFO start/finish logs:\n%s", logText)
	}
}

func TestDevkitCollectionLogStateRateLimitsFailuresAndLogsRecovery(t *testing.T) {
	var logs bytes.Buffer
	state := newDevkitCollectionLogState(slog.New(slog.NewTextHandler(&logs, nil)), "devkit-memory")
	now := time.Unix(200, 0)
	state.now = func() time.Time { return now }
	args := []string{"tuner", "memory", "-d", "3"}

	record := func(roundID uint64, err error) {
		attempt := state.start(roundID, "/opt/devkit/devkit", args, "target", "system")
		state.finish(attempt, err)
	}
	failure := newDevkitCollectionFailure("parse", errors.New("missing DDR section"))
	record(1, failure)
	now = now.Add(time.Minute)
	record(2, failure)
	if count := strings.Count(logs.String(), "devkit_collection_failed"); count != 1 {
		t.Fatalf("repeated failure logs = %d, want 1; logs:\n%s", count, logs.String())
	}

	now = now.Add(devkitFailureLogWindow)
	record(3, failure)
	if count := strings.Count(logs.String(), "devkit_collection_failed"); count != 2 {
		t.Fatalf("post-window failure logs = %d, want 2; logs:\n%s", count, logs.String())
	}

	now = now.Add(time.Second)
	record(4, newDevkitCollectionFailure("cli_execution", errors.New("exit status 1")))
	if count := strings.Count(logs.String(), "devkit_collection_failed"); count != 3 {
		t.Fatalf("changed failure logs = %d, want 3; logs:\n%s", count, logs.String())
	}

	now = now.Add(time.Second)
	record(5, nil)
	record(6, nil)
	if count := strings.Count(logs.String(), "devkit_collection_recovered"); count != 1 {
		t.Fatalf("recovery logs = %d, want 1; logs:\n%s", count, logs.String())
	}
}

func TestDevkitCollectionLogStateLogsFailureAfterParameterChange(t *testing.T) {
	var logs bytes.Buffer
	state := newDevkitCollectionLogState(slog.New(slog.NewTextHandler(&logs, nil)), "devkit-topdown")
	now := time.Unix(300, 0)
	state.now = func() time.Time { return now }
	failure := newDevkitCollectionFailure("parse", errors.New("invalid report"))

	attempt := state.start(1, "/opt/devkit/devkit", []string{"tuner", "top-down", "-c", "0"})
	state.finish(attempt, failure)
	now = now.Add(time.Second)
	attempt = state.start(2, "/opt/devkit/devkit", []string{"tuner", "top-down", "-c", "1"})
	state.finish(attempt, failure)

	if count := strings.Count(logs.String(), "devkit_collection_failed"); count != 2 {
		t.Fatalf("failure logs across parameter change = %d, want 2; logs:\n%s", count, logs.String())
	}
	if !strings.Contains(logs.String(), "devkit_collection_parameters_changed") {
		t.Fatalf("missing parameter-change log:\n%s", logs.String())
	}
}

func TestDevkitCollectionDebugOutputIsTruncated(t *testing.T) {
	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	state := newDevkitCollectionLogState(logger, "devkit-topdown")
	originalLimit := maxDevkitDebugOutputCharacters
	maxDevkitDebugOutputCharacters = 32
	t.Cleanup(func() { maxDevkitDebugOutputCharacters = originalLimit })

	state.debugCLIOutput(7, "stdout", strings.Repeat("x", 100))

	logText := logs.String()
	if !strings.Contains(logText, "devkit_cli_output") || !strings.Contains(logText, "truncated=true") {
		t.Fatalf("missing bounded debug output metadata:\n%s", logText)
	}
	if strings.Contains(logText, strings.Repeat("x", 100)) {
		t.Fatalf("debug log contains unbounded CLI output:\n%s", logText)
	}
}

func TestDevkitConfigLogsOnlyFirstLoadAndChangesAtInfo(t *testing.T) {
	var logs bytes.Buffer
	watcher := &DevkitConfigWatcher{logger: slog.New(slog.NewTextHandler(&logs, nil))}
	initial := DevkitConfig{}
	watcher.config.Store(&initial)

	watcher.onUpdate(testDevkitConfigMap("topdown:\n  duration: 3\nmemory:\n  duration: 3\n"))
	watcher.onUpdate(testDevkitConfigMap("topdown:\n  duration: 3\nmemory:\n  duration: 3\n"))
	watcher.onUpdate(testDevkitConfigMap("topdown:\n  duration: 4\nmemory:\n  duration: 3\n"))

	logText := logs.String()
	if count := strings.Count(logText, "devkit_config_loaded"); count != 1 {
		t.Fatalf("initial config logs = %d, want 1; logs:\n%s", count, logText)
	}
	if count := strings.Count(logText, "devkit_config_changed"); count != 1 {
		t.Fatalf("changed config logs = %d, want 1; logs:\n%s", count, logText)
	}
}
