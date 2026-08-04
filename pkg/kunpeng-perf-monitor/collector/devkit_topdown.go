// Copyright 2017 The Prometheus Authors
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
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

const devkitTopdownSubsystem = "devkit_topdown"

// devkitTopdownCollector 通过异步执行 DevKit Tuner top-down CLI 并缓存解析结果，
// 在 scrape 时零延迟读缓存。采集与暴露解耦为两条独立路径（见设计方案 2.1）。
type devkitTopdownCollector struct {
	cyclesDesc       *prometheus.Desc
	instructionsDesc *prometheus.Desc
	ipcDesc          *prometheus.Desc
	boundPercentDesc *prometheus.Desc
	pmuEventDesc     *prometheus.Desc
	successDesc      *prometheus.Desc
	lastSuccessDesc  *prometheus.Desc

	watcher    *DevkitConfigWatcher
	binaryPath string
	interval   time.Duration
	cache      atomic.Pointer[topdownMetricCache]
	collecting atomic.Bool
	runCommand devkitCommandRunner
	logger     *slog.Logger
}

func init() {
	RegisterCollector("devkit-topdown", defaultDisabled, NewDevkitTopdownCollector)
}

// NewDevkitTopdownCollector 创建 devkit-topdown 采集器，并启动后台异步采集循环。
func NewDevkitTopdownCollector(logger *slog.Logger) (Collector, error) {
	c := &devkitTopdownCollector{
		cyclesDesc: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, devkitTopdownSubsystem, "cycles"),
			"CPU cycles observed in the latest collection window.",
			[]string{"level", "target_type", "target"},
			nil,
		),
		instructionsDesc: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, devkitTopdownSubsystem, "instructions"),
			"Instructions observed in the latest collection window.",
			[]string{"level", "target_type", "target"},
			nil,
		),
		ipcDesc: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, devkitTopdownSubsystem, "ipc_ratio"),
			"Instructions per cycle in the latest collection window.",
			[]string{"level", "target_type", "target"},
			nil,
		),
		boundPercentDesc: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, devkitTopdownSubsystem, "bound_percent"),
			"TopDown pipeline bound percentage.",
			[]string{"name", "path", "level", "preferred_event", "target_type", "target"},
			nil,
		),
		pmuEventDesc: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, devkitTopdownSubsystem, "pmu_event_count_value"),
			"PMU event count observed in the latest collection window.",
			[]string{"event", "level", "target_type", "target"},
			nil,
		),
		successDesc: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, devkitTopdownSubsystem, "collection_success"),
			"Whether the latest DevKit TopDown collection and parse succeeded.",
			[]string{"target_type", "target"},
			nil,
		),
		lastSuccessDesc: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, devkitTopdownSubsystem, "last_success_unixtime_seconds"),
			"Unix time of the latest successful DevKit TopDown collection across all scopes.",
			nil,
			nil,
		),
		watcher:    getSharedDevkitConfigWatcher(logger),
		binaryPath: devkitBinaryPath(),
		interval:   devkitCollectInterval(),
		runCommand: runDevkitCommand,
		logger:     logger,
	}
	go c.collectLoop()
	return c, nil
}

// Update 实现 Collector 接口：零延迟读缓存，不触发采集、不阻塞 scrape。
func (c *devkitTopdownCollector) Update(ch chan<- prometheus.Metric) error {
	cached := c.cache.Load()
	if cached == nil {
		// 首轮采集尚未完成，交由框架按 ErrNoData 处理。
		return ErrNoData
	}

	targetType, target := cached.targetType, cached.target
	if cached.success {
		ch <- prometheus.MustNewConstMetric(c.cyclesDesc, prometheus.GaugeValue, float64(cached.cycles), "0", targetType, target)
		ch <- prometheus.MustNewConstMetric(c.instructionsDesc, prometheus.GaugeValue, float64(cached.instructions), "0", targetType, target)
		ch <- prometheus.MustNewConstMetric(c.ipcDesc, prometheus.GaugeValue, cached.ipc, "0", targetType, target)
		for _, node := range cached.nodes {
			ch <- prometheus.MustNewConstMetric(
				c.boundPercentDesc, prometheus.GaugeValue, node.value,
				node.name, node.path, strconv.Itoa(node.level), node.preferredEvent, targetType, target,
			)
		}
		for _, evt := range cached.pmuEvents {
			ch <- prometheus.MustNewConstMetric(c.pmuEventDesc, prometheus.GaugeValue, float64(evt.count), evt.event, "0", targetType, target)
		}
	}

	var success float64
	if cached.success {
		success = 1
	}
	ch <- prometheus.MustNewConstMetric(c.successDesc, prometheus.GaugeValue, success, targetType, target)
	if cached.lastSuccessTime > 0 {
		ch <- prometheus.MustNewConstMetric(c.lastSuccessDesc, prometheus.GaugeValue, cached.lastSuccessTime)
	}
	return nil
}

// collectLoop 启动时立即采集一次，随后按 ticker 周期循环。
func (c *devkitTopdownCollector) collectLoop() {
	c.collectOnce()
	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()
	for range ticker.C {
		c.collectOnce()
	}
}

// collectOnce 执行单轮采集；用 atomic.Bool 防重入，上一轮未完成则跳过本轮。
func (c *devkitTopdownCollector) collectOnce() {
	if !c.collecting.CompareAndSwap(false, true) {
		c.logger.Debug("devkit-topdown: previous collection is still running; skipping this cycle")
		return
	}
	defer c.collecting.Store(false)

	cfg := c.watcher.load().Topdown
	args := topdownCommandArgs(cfg)
	attempt, metadataErr := parseDevkitAttemptMetadata(args)
	if metadataErr != nil {
		c.logger.Error("devkit-topdown: current argv metadata is invalid", "err", metadataErr, "cli_args", args)
		return
	}
	var cache *topdownMetricCache
	roundID, err := sharedDevkitCollectionCoordinator.run(
		c.logger,
		"devkit-topdown",
		c.binaryPath,
		args,
		func(_ uint64) error {
			output, runErr := c.runCLI(cfg, args)
			if runErr != nil {
				return fmt.Errorf("CLI execution failed: %w", runErr)
			}
			parsed, parseErr := parseTopdownOutput(output, attempt, c.logger)
			if parseErr != nil {
				return fmt.Errorf("parse failed: %w", parseErr)
			}
			cache = parsed
			return nil
		},
		"target_type", attempt.targetType,
		"target", attempt.target,
		"cpu", cfg.CPU,
		"pid", cfg.PID,
		"duration_seconds", cfg.Duration,
		"level", 0,
	)
	if err != nil {
		c.logger.Error("devkit-topdown: current collection failed", "round_id", roundID, "err", err)
		c.storeFailure(attempt)
		return
	}
	cache.lastSuccessTime = float64(time.Now().Unix())
	c.cache.Store(cache)
}

// runCLI 组装并执行 top-down 命令，超时 = duration + 5s，防止 CLI hang。
func (c *devkitTopdownCollector) runCLI(cfg DevkitTopdownConfig, args []string) (string, error) {
	timeout := time.Duration(cfg.Duration+5) * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	runner := c.runCommand
	if runner == nil {
		runner = runDevkitCommand
	}
	return runner(ctx, c.binaryPath, args...)
}

// storeFailure 仅保存本轮失败状态和最近成功时间，不保留上一轮业务数据。
func (c *devkitTopdownCollector) storeFailure(attempt devkitAttemptMetadata) {
	var lastSuccessTime float64
	if prev := c.cache.Load(); prev != nil {
		lastSuccessTime = prev.lastSuccessTime
	}
	c.cache.Store(&topdownMetricCache{
		devkitAttemptMetadata: attempt,
		success:               false,
		lastSuccessTime:       lastSuccessTime,
	})
}

// topdownCommandArgs 返回实际传给 DevKit CLI 的参数，命令执行与结构化日志共用。
func topdownCommandArgs(cfg DevkitTopdownConfig) []string {
	args := []string{"tuner", "top-down"}
	d := cfg.Duration
	if d < minDevkitDuration || d > maxDevkitDuration {
		d = defaultDevkitDuration
	}
	args = append(args, "-d", strconv.Itoa(d), "-L", "0")

	switch {
	case cfg.PID != "":
		args = append(args, "-p", cfg.PID)
	case cfg.CPU != "":
		args = append(args, "-c", cfg.CPU)
	}
	return args
}
