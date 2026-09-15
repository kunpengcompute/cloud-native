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
// +build linux

package collector

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	ktesting "k8s.io/client-go/testing"
	kcache "k8s.io/client-go/tools/cache"
)

func TestValidateDevkitBinaryPath(t *testing.T) {
	tempDir := t.TempDir()
	executable := filepath.Join(tempDir, "devkit")
	if err := os.WriteFile(executable, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatalf("write executable: %v", err)
	}
	nonExecutable := filepath.Join(tempDir, "not-executable")
	if err := os.WriteFile(nonExecutable, []byte("binary"), 0o644); err != nil {
		t.Fatalf("write non-executable: %v", err)
	}

	tests := []struct {
		name    string
		path    string
		wantErr string
	}{
		{name: "executable regular file", path: executable},
		{name: "relative path", path: "devkit", wantErr: "absolute"},
		{name: "missing path", path: filepath.Join(tempDir, "missing"), wantErr: "stat"},
		{name: "directory", path: tempDir, wantErr: "regular file"},
		{name: "missing execute bit", path: nonExecutable, wantErr: "executable"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateDevkitBinaryPath(tt.path)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("validate executable: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error = %v, want substring %q", err, tt.wantErr)
			}
		})
	}
}

func TestValidateDevkitConfigLocation(t *testing.T) {
	tests := []struct {
		name      string
		namespace string
		configMap string
		wantErr   string
	}{
		{name: "valid", namespace: "monitoring", configMap: "kunpeng.devkit-config"},
		{name: "invalid namespace", namespace: "Monitoring", configMap: "valid", wantErr: "namespace"},
		{name: "namespace is not a label", namespace: "team.monitoring", configMap: "valid", wantErr: "namespace"},
		{name: "invalid ConfigMap name", namespace: "default", configMap: "devkit_config", wantErr: "ConfigMap name"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateDevkitConfigLocation(tt.namespace, tt.configMap)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("validate location: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error = %v, want substring %q", err, tt.wantErr)
			}
		})
	}
}

func TestDevkitConfigFieldSelectorUsesStructuredAPI(t *testing.T) {
	if got, want := devkitConfigFieldSelector("devkit-config"), "metadata.name=devkit-config"; got != want {
		t.Fatalf("field selector = %q, want %q", got, want)
	}
}

func TestMemoryCommandAlwaysUsesMetricOne(t *testing.T) {
	cfg := DevkitMemoryConfig{
		Duration: 3,
		Period:   1000,
	}
	want := []string{"tuner", "memory", "-d", "3", "-m", "1", "-P", "1000"}

	if got := memoryCommandArgs(cfg); !reflect.DeepEqual(got, want) {
		t.Fatalf("memory command args = %v, want %v", got, want)
	}
}

func TestMemoryDurationOneDefaultBuildsPeriodOneHundred(t *testing.T) {
	cfg, err := parseDevkitConfig([]byte("memory:\n  duration: 1\n"))
	if err != nil {
		t.Fatalf("parse duration-one config: %v", err)
	}
	want := []string{"tuner", "memory", "-d", "1", "-m", "1", "-P", "100"}
	if got := memoryCommandArgs(cfg.Memory); !reflect.DeepEqual(got, want) {
		t.Fatalf("memory command args = %v, want %v", got, want)
	}
	if got := memoryCommandArgs(DevkitMemoryConfig{Duration: 1}); !reflect.DeepEqual(got, want) {
		t.Fatalf("memory command args from zero-value period = %v, want %v", got, want)
	}
}

func TestDevkitConfigWatcherAppliesDurationDefaultsAndBoundaries(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want DevkitConfig
	}{
		{
			name: "missing durations use defaults",
			raw:  "topdown: {}\nmemory: {}\n",
			want: DevkitConfig{
				Topdown: DevkitTopdownConfig{Duration: 3},
				Memory:  DevkitMemoryConfig{Duration: 3, Period: 1000},
			},
		},
		{
			name: "minimum duration",
			raw:  "topdown:\n  duration: 1\nmemory:\n  duration: 1\n  period: 100\n",
			want: DevkitConfig{
				Topdown: DevkitTopdownConfig{Duration: 1},
				Memory:  DevkitMemoryConfig{Duration: 1, Period: 100},
			},
		},
		{
			name: "memory duration one uses period one hundred by default",
			raw:  "memory:\n  duration: 1\n",
			want: DevkitConfig{
				Topdown: DevkitTopdownConfig{Duration: 3},
				Memory:  DevkitMemoryConfig{Duration: 1, Period: 100},
			},
		},
		{
			name: "maximum duration",
			raw:  "topdown:\n  duration: 5\nmemory:\n  duration: 5\n",
			want: DevkitConfig{
				Topdown: DevkitTopdownConfig{Duration: 5},
				Memory:  DevkitMemoryConfig{Duration: 5, Period: 1000},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			watcher := newTestDevkitConfigWatcher(DevkitConfig{})
			watcher.onUpdate(testDevkitConfigMap(tt.raw))
			if got := watcher.load(); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("loaded config = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestDevkitConfigWatcherRejectsInvalidDurationAndLegacyMetric(t *testing.T) {
	baseline := DevkitConfig{
		Topdown: DevkitTopdownConfig{CPU: "0", Duration: 2},
		Memory:  DevkitMemoryConfig{CPU: "1", Duration: 2, Period: 100},
	}
	tests := []struct {
		name string
		raw  string
	}{
		{name: "topdown zero", raw: "topdown:\n  duration: 0\n"},
		{name: "topdown negative", raw: "topdown:\n  duration: -1\n"},
		{name: "topdown above maximum", raw: "topdown:\n  duration: 6\n"},
		{name: "topdown non integer", raw: "topdown:\n  duration: 1.5\n"},
		{name: "memory zero", raw: "memory:\n  duration: 0\n"},
		{name: "memory negative", raw: "memory:\n  duration: -1\n"},
		{name: "memory above maximum", raw: "memory:\n  duration: 6\n"},
		{name: "memory non integer", raw: "memory:\n  duration: 1.5\n"},
		{name: "memory duration one with period one thousand", raw: "memory:\n  duration: 1\n  period: 1000\n"},
		{name: "memory period zero", raw: "memory:\n  period: 0\n"},
		{name: "legacy memory metric", raw: "memory:\n  metric: 1\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			watcher := newTestDevkitConfigWatcher(baseline)
			watcher.onUpdate(testDevkitConfigMap(tt.raw))
			if got := watcher.load(); !reflect.DeepEqual(got, baseline) {
				t.Fatalf("invalid update replaced config: got %+v, want previous %+v", got, baseline)
			}
		})
	}
}

func TestDevkitConfigPreservesSupportedNoncanonicalSelectors(t *testing.T) {
	cfg, err := parseDevkitConfig([]byte("topdown:\n  cpu: 1,0\nmemory:\n  cpu: 8-2\n"))
	if err != nil {
		t.Fatalf("parse supported noncanonical selectors: %v", err)
	}
	if cfg.Topdown.CPU != "1,0" || cfg.Memory.CPU != "8-2" {
		t.Fatalf("selectors were rewritten: topdown=%q memory=%q", cfg.Topdown.CPU, cfg.Memory.CPU)
	}
}

func TestDevkitConfigWatcherRejectsExtremeSelectorsAndKeepsLastKnownGood(t *testing.T) {
	baseline := DevkitConfig{
		Topdown: DevkitTopdownConfig{CPU: "0", Duration: 2},
		Memory:  DevkitMemoryConfig{CPU: "1", Duration: 2, Period: 100},
	}
	tests := []struct {
		name string
		raw  string
	}{
		{name: "CPU overflow", raw: "topdown:\n  cpu: '18446744073709551616'\n"},
		{name: "PID overflow", raw: "topdown:\n  pid: '18446744073709551616'\n"},
		{name: "PID zero", raw: "topdown:\n  pid: '0'\n"},
		{name: "CPU length limit", raw: "topdown:\n  cpu: '" + strings.Repeat("0,", 511) + "000'\n"},
		{name: "CPU element limit", raw: "topdown:\n  cpu: '" + strings.Repeat(",", 512) + "'\n"},
		{name: "PID length limit", raw: "topdown:\n  pid: '" + strings.Repeat("1", 513) + "'\n"},
		{name: "PID element limit", raw: "topdown:\n  pid: '" + strings.TrimSuffix(strings.Repeat("1,", 33), ",") + "'\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			watcher := newTestDevkitConfigWatcher(baseline)
			watcher.onUpdate(testDevkitConfigMap(tt.raw))
			if got := watcher.load(); !reflect.DeepEqual(got, baseline) {
				t.Fatalf("invalid selector replaced config: got %+v, want previous %+v", got, baseline)
			}
		})
	}
}

func TestDevkitConfigWatcherLogsRejectedDurationPeriodCombination(t *testing.T) {
	baseline := DevkitConfig{
		Topdown: DevkitTopdownConfig{Duration: 3},
		Memory:  DevkitMemoryConfig{Duration: 3, Period: 1000},
	}
	var logs bytes.Buffer
	watcher := &DevkitConfigWatcher{logger: slog.New(slog.NewTextHandler(&logs, nil))}
	watcher.config.Store(&baseline)

	watcher.onUpdate(testDevkitConfigMap("memory:\n  duration: 1\n  period: 1000\n"))

	if got := watcher.load(); !reflect.DeepEqual(got, baseline) {
		t.Fatalf("invalid duration/period update replaced config: got %+v, want previous %+v", got, baseline)
	}
	for _, required := range []string{
		"devkit_config_rejected",
		"when memory duration=1, period must be omitted or explicitly set to 100",
		"action=keep_last_known_good",
	} {
		if !strings.Contains(logs.String(), required) {
			t.Fatalf("rejection log missing %q:\n%s", required, logs.String())
		}
	}
}

func newTestDevkitConfigWatcher(initial DevkitConfig) *DevkitConfigWatcher {
	watcher := &DevkitConfigWatcher{
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	watcher.config.Store(&initial)
	return watcher
}

func testDevkitConfigMap(raw string) *corev1.ConfigMap {
	return &corev1.ConfigMap{Data: map[string]string{"devkit-tuner.yaml": raw}}
}

func TestDevkitConfigWatcherInitialSync(t *testing.T) {
	const namespace = "devkit-config-test"
	clientset := fake.NewClientset(&corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      defaultDevkitConfigName,
		},
		Data: map[string]string{
			"devkit-tuner.yaml": `
topdown:
  duration: 5
memory:
  period: 100
`,
		},
	})
	listStarted := make(chan struct{})
	selectorSeen := make(chan string, 1)
	releaseList := make(chan struct{})
	var listOnce sync.Once
	var releaseOnce sync.Once
	release := func() { releaseOnce.Do(func() { close(releaseList) }) }
	t.Cleanup(release)
	clientset.PrependReactor("list", "configmaps", func(action ktesting.Action) (bool, runtime.Object, error) {
		listAction, ok := action.(ktesting.ListAction)
		if !ok {
			selectorSeen <- "<not a list action>"
		} else {
			selectorSeen <- listAction.GetListRestrictions().Fields.String()
		}
		listOnce.Do(func() { close(listStarted) })
		<-releaseList
		return false, nil, nil
	})

	watcher := &DevkitConfigWatcher{
		clientset: clientset,
		namespace: namespace,
		name:      defaultDevkitConfigName,
		logger:    slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	initial := DevkitConfig{}
	watcher.config.Store(&initial)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	startReturned := make(chan struct{})
	go func() {
		watcher.start(ctx)
		close(startReturned)
	}()

	select {
	case <-listStarted:
	case <-time.After(time.Second):
		t.Fatal("informer did not start the initial ConfigMap LIST")
	}
	select {
	case selector := <-selectorSeen:
		wantSelector := "metadata.name=" + defaultDevkitConfigName
		if selector != wantSelector {
			t.Fatalf("ConfigMap LIST field selector = %q, want %q", selector, wantSelector)
		}
	case <-time.After(time.Second):
		t.Fatal("did not observe the ConfigMap LIST field selector")
	}
	select {
	case <-startReturned:
		t.Fatal("watcher.start returned before the initial ConfigMap LIST was released")
	case <-time.After(50 * time.Millisecond):
	}
	release()
	select {
	case <-startReturned:
	case <-time.After(time.Second):
		t.Fatal("watcher.start did not return after initial ConfigMap LIST completed")
	}

	config := watcher.load()
	if config.Topdown.Duration != 5 {
		t.Fatalf("topdown duration = %d, want ConfigMap value 5", config.Topdown.Duration)
	}
	if config.Memory.Period != 100 {
		t.Fatalf("memory period = %d, want ConfigMap value 100", config.Memory.Period)
	}
}

func TestDevkitConfigWatcherProviderInitializesOnce(t *testing.T) {
	var calls atomic.Int32
	want := &DevkitConfigWatcher{}
	provider := devkitConfigWatcherProvider{
		newWatcher: func(*slog.Logger) *DevkitConfigWatcher {
			calls.Add(1)
			return want
		},
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	const goroutines = 8
	results := make(chan *DevkitConfigWatcher, goroutines)
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for range goroutines {
		go func() {
			defer wg.Done()
			results <- provider.get(logger)
		}()
	}
	wg.Wait()
	close(results)

	for got := range results {
		if got != want {
			t.Fatalf("provider returned watcher %p, want shared watcher %p", got, want)
		}
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("watcher factory called %d times, want 1", got)
	}
}

func TestDevkitCapacityWarningUsesFixedMaximumAtStartup(t *testing.T) {
	tests := []struct {
		interval string
		wantWarn bool
	}{
		{interval: "10", wantWarn: true},
		{interval: "11", wantWarn: false},
		{interval: "15", wantWarn: false},
	}

	for _, tt := range tests {
		t.Run(tt.interval+"s", func(t *testing.T) {
			t.Setenv(devkitCollectIntervalEnv, tt.interval)
			var logs bytes.Buffer
			newDevkitConfigWatcher(slog.New(slog.NewTextHandler(&logs, nil)))

			gotWarn := strings.Contains(logs.String(), "devkit_capacity_warning")
			if gotWarn != tt.wantWarn {
				t.Fatalf("capacity warning = %v, want %v; logs:\n%s", gotWarn, tt.wantWarn, logs.String())
			}
		})
	}
}

func TestDevkitCollectIntervalAcceptsPositiveIntegerSeconds(t *testing.T) {
	t.Setenv(devkitCollectIntervalEnv, "30")

	if got := devkitCollectInterval(); got != 30*time.Second {
		t.Fatalf("collect interval = %s, want 30s", got)
	}
}

func TestDevkitCollectIntervalEnforcesBusinessLimit(t *testing.T) {
	originalLimit := maxDevkitCollectIntervalSeconds
	maxDevkitCollectIntervalSeconds = 3600
	t.Cleanup(func() { maxDevkitCollectIntervalSeconds = originalLimit })

	if got, err := parseDevkitCollectInterval("3600"); err != nil || got != time.Hour {
		t.Fatalf("parse upper boundary = %s, %v; want 1h, nil", got, err)
	}
	if _, err := parseDevkitCollectInterval("3601"); err == nil || !strings.Contains(err.Error(), "business_limit") {
		t.Fatalf("parse over business limit error = %v, want business_limit", err)
	}
}

func TestDevkitCollectIntervalRejectsUnitsAndNonIntegers(t *testing.T) {
	tests := []string{"15s", "5m", "1.5", "1e3", "+30", " 15", "0", "-1", "invalid", "9223372036854775807"}
	for _, raw := range tests {
		t.Run(raw, func(t *testing.T) {
			t.Setenv(devkitCollectIntervalEnv, raw)

			if got := devkitCollectInterval(); got != defaultDevkitInterval {
				t.Fatalf("collect interval for %q = %s, want default %s", raw, got, defaultDevkitInterval)
			}
		})
	}
}

func TestDevkitConfigWatcherWarnsInvalidCollectInterval(t *testing.T) {
	t.Setenv(devkitCollectIntervalEnv, "5m")
	var logs bytes.Buffer

	newDevkitConfigWatcher(slog.New(slog.NewTextHandler(&logs, nil)))

	if !strings.Contains(logs.String(), "devkit_collect_interval_invalid") {
		t.Fatalf("invalid interval warning missing from logs: %s", logs.String())
	}
}

func TestDevkitConfigWatcherLogsCollectIntervalBusinessLimit(t *testing.T) {
	t.Setenv(devkitCollectIntervalEnv, "3601")
	var logs bytes.Buffer

	newDevkitConfigWatcher(slog.New(slog.NewTextHandler(&logs, nil)))

	if !strings.Contains(logs.String(), "devkit_collect_interval_invalid") ||
		!strings.Contains(logs.String(), "business_limit") ||
		!strings.Contains(logs.String(), `value=3601`) {
		t.Fatalf("business limit warning is incomplete: %s", logs.String())
	}
}

func TestDevkitConfigUpdateDoesNotRepeatCapacityWarning(t *testing.T) {
	var logs bytes.Buffer
	initial := DevkitConfig{}
	watcher := &DevkitConfigWatcher{logger: slog.New(slog.NewTextHandler(&logs, nil))}
	watcher.config.Store(&initial)

	watcher.onUpdate(testDevkitConfigMap("topdown:\n  duration: 5\nmemory:\n  duration: 5\n"))
	if strings.Contains(logs.String(), "devkit_capacity_warning") {
		t.Fatalf("ConfigMap update repeated startup-only capacity warning:\n%s", logs.String())
	}
}

func TestDevkitConfigDeleteWarnsAndKeepsLastKnownGood(t *testing.T) {
	baseline := DevkitConfig{
		Topdown: DevkitTopdownConfig{CPU: "0", Duration: 5},
		Memory:  DevkitMemoryConfig{CPU: "1", Duration: 4, Period: 100},
	}
	tests := []struct {
		name string
		obj  interface{}
	}{
		{
			name: "direct object",
			obj: &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
				Namespace: "default",
				Name:      defaultDevkitConfigName,
			}},
		},
		{
			name: "tombstone",
			obj: kcache.DeletedFinalStateUnknown{Obj: &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
				Namespace: "default",
				Name:      defaultDevkitConfigName,
			}}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var logs bytes.Buffer
			watcher := &DevkitConfigWatcher{
				namespace: "default",
				name:      defaultDevkitConfigName,
				logger:    slog.New(slog.NewTextHandler(&logs, nil)),
			}
			watcher.config.Store(&baseline)

			watcher.onDelete(tt.obj)

			if got := watcher.load(); !reflect.DeepEqual(got, baseline) {
				t.Fatalf("delete replaced last-known-good config: got %+v, want %+v", got, baseline)
			}
			if !strings.Contains(logs.String(), "devkit_config_deleted") {
				t.Fatalf("missing delete warning:\n%s", logs.String())
			}
		})
	}
}
