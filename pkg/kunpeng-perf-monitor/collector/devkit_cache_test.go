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
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

func TestParseDevkitAttemptMetadata(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		want    devkitAttemptMetadata
		wantErr bool
	}{
		{
			name: "topdown_system",
			args: []string{"tuner", "top-down", "-d", "3", "-L", "0"},
			want: devkitAttemptMetadata{targetType: "system", target: "system"},
		},
		{
			name: "topdown_cpu",
			args: []string{"tuner", "top-down", "-d", "3", "-L", "0", "-c", "10,80,89-99"},
			want: devkitAttemptMetadata{targetType: "cpu", target: "cpu10,80,89-99", targetValue: "10,80,89-99"},
		},
		{
			name: "topdown_pid",
			args: []string{"tuner", "top-down", "-d", "3", "-L", "0", "-p", "17103"},
			want: devkitAttemptMetadata{targetType: "pid", target: "pid17103", targetValue: "17103"},
		},
		{
			name: "memory_system",
			args: []string{"tuner", "memory", "-d", "3", "-m", "1", "-P", "1000"},
			want: devkitAttemptMetadata{targetType: "system", target: "system", periodMilliseconds: 1000},
		},
		{
			name: "memory_cpu",
			args: []string{"tuner", "memory", "-d", "3", "-m", "1", "-P", "100", "-c", "10,80,89-99"},
			want: devkitAttemptMetadata{targetType: "cpu", target: "cpu10,80,89-99", targetValue: "10,80,89-99", periodMilliseconds: 100},
		},
		{
			name:    "topdown_rejects_cpu_and_pid",
			args:    []string{"tuner", "top-down", "-d", "3", "-L", "0", "-c", "0-3", "-p", "17103"},
			wantErr: true,
		},
		{
			name:    "memory_requires_period",
			args:    []string{"tuner", "memory", "-d", "3", "-m", "1"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseDevkitAttemptMetadata(tt.args)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected metadata parse error")
				}
				return
			}
			if err != nil {
				t.Fatalf("parse metadata: %v", err)
			}
			if got != tt.want {
				t.Fatalf("metadata = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestCommandBuildersProduceParseableAttemptMetadata(t *testing.T) {
	topdown, err := parseDevkitAttemptMetadata(topdownCommandArgs(DevkitTopdownConfig{
		CPU: "10,80,89-99", Duration: 3,
	}))
	if err != nil {
		t.Fatalf("parse TopDown command args: %v", err)
	}
	if topdown.targetType != "cpu" || topdown.target != "cpu10,80,89-99" {
		t.Fatalf("TopDown metadata = %+v", topdown)
	}

	memory, err := parseDevkitAttemptMetadata(memoryCommandArgs(DevkitMemoryConfig{
		CPU: "10,80,89-99", Duration: 3, Period: 100,
	}))
	if err != nil {
		t.Fatalf("parse Memory command args: %v", err)
	}
	if memory.targetType != "cpu" || memory.target != "cpu10,80,89-99" || memory.periodMilliseconds != 100 {
		t.Fatalf("Memory metadata = %+v", memory)
	}
}

func TestTopdownFailureCacheIsLightweightAndPreservesFreshness(t *testing.T) {
	collector := &devkitTopdownCollector{}
	collector.cache.Store(&topdownMetricCache{
		devkitAttemptMetadata: devkitAttemptMetadata{targetType: "system", target: "system"},
		cycles:                42,
		nodes:                 []topdownNode{{name: "backend_bound"}},
		pmuEvents:             []topdownPMUEvent{{event: "r0008", count: 1}},
		success:               true,
		lastSuccessTime:       123,
	})

	collector.storeFailure(devkitAttemptMetadata{targetType: "cpu", target: "cpu0-3", targetValue: "0-3"})
	failed := collector.cache.Load()
	if failed.success {
		t.Fatal("failure cache reports success")
	}
	if failed.targetType != "cpu" {
		t.Fatalf("failure targetType = %q, want current attempt scope cpu", failed.targetType)
	}
	if failed.target != "cpu0-3" {
		t.Fatalf("failure target = %q, want current attempt target cpu0-3", failed.target)
	}
	if failed.lastSuccessTime != 123 {
		t.Fatalf("lastSuccessTime = %v, want 123", failed.lastSuccessTime)
	}
	if failed.cycles != 0 || len(failed.nodes) != 0 || len(failed.pmuEvents) != 0 {
		t.Fatalf("failure cache retained business data: %+v", failed)
	}
}

func TestMemoryFailureCacheUsesAttemptScopeAndDropsBusinessData(t *testing.T) {
	collector := &devkitMemoryCollector{}
	collector.cache.Store(&memoryMetricCache{
		devkitAttemptMetadata: devkitAttemptMetadata{targetType: "system", target: "system", periodMilliseconds: 1000},
		cacheMiss:             []memoryCacheMiss{{component: "l1d", percent: 1}},
		ddrSystem:             []memoryDDRSystem{{operation: "ddrc_read", value: 2}},
		access:                []memoryAccessCell{{cpu: "all", component: "l1d"}},
		l3:                    []memoryL3Row{{node: "0", ccl: "all"}},
		ddrc:                  []memoryDDRCCell{{node: "0", ddrc: "total"}},
		success:               true,
		lastSuccessTime:       456,
	})

	collector.markFailure(devkitAttemptMetadata{targetType: "cpu", target: "cpu0-3", targetValue: "0-3", periodMilliseconds: 100})
	failed := collector.cache.Load()
	if failed.success {
		t.Fatal("failure cache reports success")
	}
	if failed.targetType != "cpu" {
		t.Fatalf("failure targetType = %q, want current attempt scope cpu", failed.targetType)
	}
	if failed.target != "cpu0-3" || failed.periodMilliseconds != 100 {
		t.Fatalf("failure metadata = %+v, want target cpu0-3 and period 100", failed.devkitAttemptMetadata)
	}
	if failed.lastSuccessTime != 456 {
		t.Fatalf("lastSuccessTime = %v, want 456", failed.lastSuccessTime)
	}
	if len(failed.cacheMiss) != 0 || len(failed.ddrSystem) != 0 || len(failed.access) != 0 || len(failed.l3) != 0 || len(failed.ddrc) != 0 {
		t.Fatalf("failure cache retained business data: %+v", failed)
	}
}

func TestFailureUpdatePublishesAttemptLabelsAndUnlabeledLastSuccess(t *testing.T) {
	tests := []struct {
		name       string
		wantLabels map[string]string
		update     func(chan<- prometheus.Metric) error
	}{
		{
			name: "topdown",
			wantLabels: map[string]string{
				"target_type": "cpu",
				"target":      "cpu0-3",
			},
			update: func(ch chan<- prometheus.Metric) error {
				collector := &devkitTopdownCollector{
					successDesc:     prometheus.NewDesc("test_topdown_collection_success", "help", []string{"target_type", "target"}, nil),
					lastSuccessDesc: prometheus.NewDesc("test_topdown_last_success_unixtime_seconds", "help", nil, nil),
				}
				collector.cache.Store(&topdownMetricCache{
					devkitAttemptMetadata: devkitAttemptMetadata{targetType: "cpu", target: "cpu0-3", targetValue: "0-3"},
					success:               false,
					lastSuccessTime:       123,
				})
				return collector.Update(ch)
			},
		},
		{
			name: "memory",
			wantLabels: map[string]string{
				"target_type":         "cpu",
				"target":              "cpu0-3",
				"period_milliseconds": "100",
			},
			update: func(ch chan<- prometheus.Metric) error {
				collector := &devkitMemoryCollector{
					collectionSuccessDesc: prometheus.NewDesc("test_memory_collection_success", "help", []string{"target_type", "target", "period_milliseconds"}, nil),
					lastSuccessDesc:       prometheus.NewDesc("test_memory_last_success_unixtime_seconds", "help", nil, nil),
				}
				collector.cache.Store(&memoryMetricCache{
					devkitAttemptMetadata: devkitAttemptMetadata{targetType: "cpu", target: "cpu0-3", targetValue: "0-3", periodMilliseconds: 100},
					success:               false,
					lastSuccessTime:       456,
				})
				return collector.Update(ch)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metrics := make(chan prometheus.Metric, 2)
			if err := tt.update(metrics); err != nil {
				t.Fatalf("Update returned error: %v", err)
			}
			if len(metrics) != 2 {
				t.Fatalf("published metric count = %d, want collection_success and last_success", len(metrics))
			}

			var foundLastSuccess, foundCollectionSuccess bool
			for range 2 {
				metric := <-metrics
				description := metric.Desc().String()
				if strings.Contains(description, "last_success_unixtime_seconds") {
					foundLastSuccess = true
					if !strings.Contains(description, "variableLabels: {}") {
						t.Fatalf("last_success metric unexpectedly has labels: %s", description)
					}
				} else if strings.Contains(description, "collection_success") {
					foundCollectionSuccess = true
					if got := prometheusMetricLabels(t, metric); !equalStringMap(got, tt.wantLabels) {
						t.Fatalf("collection_success labels = %v, want %v", got, tt.wantLabels)
					}
				}
			}
			if !foundLastSuccess {
				t.Fatal("missing last_success_unixtime_seconds metric")
			}
			if !foundCollectionSuccess {
				t.Fatal("missing collection_success metric")
			}
		})
	}
}

func TestTopdownSuccessUpdatePublishesTargetOnEveryBusinessMetric(t *testing.T) {
	collector := &devkitTopdownCollector{
		cyclesDesc:       prometheus.NewDesc("test_topdown_cycles", "help", []string{"level", "target_type", "target"}, nil),
		instructionsDesc: prometheus.NewDesc("test_topdown_instructions", "help", []string{"level", "target_type", "target"}, nil),
		ipcDesc:          prometheus.NewDesc("test_topdown_ipc", "help", []string{"level", "target_type", "target"}, nil),
		boundPercentDesc: prometheus.NewDesc("test_topdown_bound", "help", []string{"name", "path", "level", "preferred_event", "target_type", "target"}, nil),
		pmuEventDesc:     prometheus.NewDesc("test_topdown_pmu", "help", []string{"event", "level", "target_type", "target"}, nil),
		successDesc:      prometheus.NewDesc("test_topdown_collection_success", "help", []string{"target_type", "target"}, nil),
		lastSuccessDesc:  prometheus.NewDesc("test_topdown_last_success", "help", nil, nil),
	}
	collector.cache.Store(&topdownMetricCache{
		devkitAttemptMetadata: devkitAttemptMetadata{targetType: "pid", target: "pid17103", targetValue: "17103"},
		cycles:                1,
		instructions:          2,
		ipc:                   2,
		nodes:                 []topdownNode{{name: "retiring", path: "retiring", level: 1, value: 1}},
		pmuEvents:             []topdownPMUEvent{{event: "r0008", count: 2}},
		success:               true,
		lastSuccessTime:       123,
	})

	metrics := make(chan prometheus.Metric, 7)
	if err := collector.Update(metrics); err != nil {
		t.Fatalf("Update returned error: %v", err)
	}
	if len(metrics) != 7 {
		t.Fatalf("published metric count = %d, want 7", len(metrics))
	}
	for range 7 {
		metric := <-metrics
		labels := prometheusMetricLabels(t, metric)
		if strings.Contains(metric.Desc().String(), "last_success") {
			if len(labels) != 0 {
				t.Fatalf("last_success labels = %v, want none", labels)
			}
			continue
		}
		assertContainsLabels(t, labels, map[string]string{"target_type": "pid", "target": "pid17103"})
	}
}

func TestMemorySuccessUpdatePublishesTargetAndPeriodOnEveryBusinessMetric(t *testing.T) {
	collector := &devkitMemoryCollector{
		cacheMissDesc:          prometheus.NewDesc("test_memory_cache_miss", "help", []string{"component", "target_type", "target", "period_milliseconds"}, nil),
		ddrSystemBandwidthDesc: prometheus.NewDesc("test_memory_ddr", "help", []string{"operation", "target_type", "target", "period_milliseconds"}, nil),
		accessBandwidthDesc:    prometheus.NewDesc("test_memory_access_bw", "help", []string{"component", "cpu", "target_type", "target", "period_milliseconds"}, nil),
		accessHitDesc:          prometheus.NewDesc("test_memory_access_hit", "help", []string{"component", "cpu", "target_type", "target", "period_milliseconds"}, nil),
		l3ReadBandwidthDesc:    prometheus.NewDesc("test_memory_l3_read", "help", []string{"node", "ccl", "target_type", "target", "period_milliseconds"}, nil),
		l3ReadHitBandwidthDesc: prometheus.NewDesc("test_memory_l3_hit_bw", "help", []string{"node", "ccl", "target_type", "target", "period_milliseconds"}, nil),
		l3ReadHitDesc:          prometheus.NewDesc("test_memory_l3_hit", "help", []string{"node", "ccl", "target_type", "target", "period_milliseconds"}, nil),
		ddrcBandwidthDesc:      prometheus.NewDesc("test_memory_ddrc", "help", []string{"node", "ddrc", "operation", "target_type", "target", "period_milliseconds"}, nil),
		collectionSuccessDesc:  prometheus.NewDesc("test_memory_collection_success", "help", []string{"target_type", "target", "period_milliseconds"}, nil),
		lastSuccessDesc:        prometheus.NewDesc("test_memory_last_success", "help", nil, nil),
	}
	collector.cache.Store(&memoryMetricCache{
		devkitAttemptMetadata: devkitAttemptMetadata{targetType: "cpu", target: "cpu10,80,89-99", targetValue: "10,80,89-99", periodMilliseconds: 100},
		cacheMiss:             []memoryCacheMiss{{component: "l1d", percent: 1}},
		ddrSystem:             []memoryDDRSystem{{operation: "ddrc_read", value: 2}},
		access:                []memoryAccessCell{{cpu: "10", component: "l1d", bandwidth: 3, hasBW: true, hitPercent: 4, hasHit: true}},
		l3:                    []memoryL3Row{{node: "0", ccl: "all", readBandwidth: 5, readHitBandwidth: 6, readHitPercent: 7}},
		ddrc:                  []memoryDDRCCell{{node: "0", ddrc: "total", read: 8, write: 9}},
		success:               true,
		lastSuccessTime:       456,
	})

	metrics := make(chan prometheus.Metric, 11)
	if err := collector.Update(metrics); err != nil {
		t.Fatalf("Update returned error: %v", err)
	}
	if len(metrics) != 11 {
		t.Fatalf("published metric count = %d, want 11", len(metrics))
	}
	wantAttemptLabels := map[string]string{
		"target_type":         "cpu",
		"target":              "cpu10,80,89-99",
		"period_milliseconds": "100",
	}
	for range 11 {
		metric := <-metrics
		labels := prometheusMetricLabels(t, metric)
		if strings.Contains(metric.Desc().String(), "last_success") {
			if len(labels) != 0 {
				t.Fatalf("last_success labels = %v, want none", labels)
			}
			continue
		}
		assertContainsLabels(t, labels, wantAttemptLabels)
	}
}

func prometheusMetricLabels(t *testing.T, metric prometheus.Metric) map[string]string {
	t.Helper()
	dtoMetric := &dto.Metric{}
	if err := metric.Write(dtoMetric); err != nil {
		t.Fatalf("write Prometheus metric: %v", err)
	}
	labels := make(map[string]string, len(dtoMetric.Label))
	for _, pair := range dtoMetric.Label {
		labels[pair.GetName()] = pair.GetValue()
	}
	return labels
}

func equalStringMap(left, right map[string]string) bool {
	if len(left) != len(right) {
		return false
	}
	for key, value := range left {
		if right[key] != value {
			return false
		}
	}
	return true
}

func assertContainsLabels(t *testing.T, got, want map[string]string) {
	t.Helper()
	for key, value := range want {
		if got[key] != value {
			t.Fatalf("labels = %v, want %s=%q", got, key, value)
		}
	}
}
