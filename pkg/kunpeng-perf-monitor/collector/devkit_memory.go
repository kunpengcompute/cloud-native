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
	"log/slog"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

const devkitMemorySubsystem = "devkit_memory"

// devkitMemoryCollector 异步执行 devkit tuner memory 并缓存解析结果，
// scrape 时零延迟读缓存。结构与 devkitTopdownCollector 同构。
type devkitMemoryCollector struct {
	cacheMissDesc          *prometheus.Desc
	ddrSystemBandwidthDesc *prometheus.Desc
	accessBandwidthDesc    *prometheus.Desc
	accessHitDesc          *prometheus.Desc
	l3ReadBandwidthDesc    *prometheus.Desc
	l3ReadHitBandwidthDesc *prometheus.Desc
	l3ReadHitDesc          *prometheus.Desc
	ddrcBandwidthDesc      *prometheus.Desc
	collectionSuccessDesc  *prometheus.Desc
	lastSuccessDesc        *prometheus.Desc

	configWatcher *DevkitConfigWatcher
	binaryPath    string
	cache         atomic.Pointer[memoryMetricCache]
	collecting    atomic.Bool
	runCommand    devkitCommandRunner
	logger        *slog.Logger
	interval      time.Duration
	logState      *devkitCollectionLogState
}

func init() {
	RegisterCollector("devkit-memory", defaultDisabled, NewDevkitMemoryCollector)
}

// NewDevkitMemoryCollector 创建并启动 devkit-memory 采集器。
// 后台 goroutine 立即执行首次采集，随后按 ticker 周期循环；框架无 Close 生命周期，
// goroutine 随进程终止而退出。
func NewDevkitMemoryCollector(logger *slog.Logger) (Collector, error) {
	c := &devkitMemoryCollector{
		cacheMissDesc: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, devkitMemorySubsystem, "cache_miss_percent"),
			"Core cache miss percentage per component.",
			[]string{"component", "target_type", "target", "period_milliseconds"},
			nil,
		),
		ddrSystemBandwidthDesc: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, devkitMemorySubsystem, "ddr_system_bandwidth_megabytes_per_second"),
			"System-wide DDR bandwidth in MB/s.",
			[]string{"operation", "target_type", "target", "period_milliseconds"},
			nil,
		),
		accessBandwidthDesc: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, devkitMemorySubsystem, "access_bandwidth_megabytes_per_second"),
			"L1/L2/TLB access bandwidth in MB/s.",
			[]string{"component", "cpu", "target_type", "target", "period_milliseconds"},
			nil,
		),
		accessHitDesc: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, devkitMemorySubsystem, "access_hit_percent"),
			"L1/L2/TLB access hit rate percentage.",
			[]string{"component", "cpu", "target_type", "target", "period_milliseconds"},
			nil,
		),
		l3ReadBandwidthDesc: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, devkitMemorySubsystem, "l3_read_bandwidth_megabytes_per_second"),
			"L3 read bandwidth in MB/s.",
			[]string{"node", "ccl", "target_type", "target", "period_milliseconds"},
			nil,
		),
		l3ReadHitBandwidthDesc: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, devkitMemorySubsystem, "l3_read_hit_bandwidth_megabytes_per_second"),
			"L3 read hit bandwidth in MB/s.",
			[]string{"node", "ccl", "target_type", "target", "period_milliseconds"},
			nil,
		),
		l3ReadHitDesc: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, devkitMemorySubsystem, "l3_read_hit_percent"),
			"L3 read hit rate percentage.",
			[]string{"node", "ccl", "target_type", "target", "period_milliseconds"},
			nil,
		),
		ddrcBandwidthDesc: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, devkitMemorySubsystem, "ddrc_bandwidth_megabytes_per_second"),
			"Per-controller DDRC bandwidth in MB/s.",
			[]string{"node", "ddrc", "operation", "target_type", "target", "period_milliseconds"},
			nil,
		),
		collectionSuccessDesc: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, devkitMemorySubsystem, "collection_success"),
			"Whether the latest DevKit Memory collection and parse succeeded.",
			[]string{"target_type", "target", "period_milliseconds"},
			nil,
		),
		lastSuccessDesc: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, devkitMemorySubsystem, "last_success_unixtime_seconds"),
			"Unix time of the latest successful DevKit Memory collection across all scopes and periods.",
			nil,
			nil,
		),
		configWatcher: getSharedDevkitConfigWatcher(logger),
		binaryPath:    devkitBinaryPath(),
		runCommand:    runDevkitCommand,
		logger:        logger,
		interval:      devkitCollectInterval(),
		logState:      newDevkitCollectionLogState(logger, "devkit-memory"),
	}
	logger.Info("devkit_collector_started", "collector", "devkit-memory", "binary_path", c.binaryPath, "interval_seconds", c.interval.Seconds())
	go c.collectLoop()
	return c, nil
}

// collectLoop 是后台异步采集循环：立即采一次，随后按 ticker 周期采集。
func (c *devkitMemoryCollector) collectLoop() {
	c.collectOnce()
	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()
	for range ticker.C {
		c.collectOnce()
	}
}

// collectOnce 执行单轮采集，用 atomic.Bool 防重入：上一轮未完成则跳过本轮。
func (c *devkitMemoryCollector) collectOnce() {
	if !c.collecting.CompareAndSwap(false, true) {
		c.logger.Debug("devkit-memory: previous collection is still running; skipping this cycle")
		return
	}
	defer c.collecting.Store(false)

	cfg := c.configWatcher.load().Memory
	args := memoryCommandArgs(cfg)
	logState := c.collectionLogState()
	attempt, metadataErr := parseDevkitAttemptMetadata(args)
	if metadataErr != nil {
		logAttempt := logState.start(0, c.binaryPath, args)
		logState.finish(logAttempt, newDevkitCollectionFailure("argv_metadata", metadataErr))
		return
	}
	var parsed *memoryMetricCache
	_, err := sharedDevkitCollectionCoordinator.run(
		logState,
		c.binaryPath,
		args,
		func(roundID uint64) error {
			output, runErr := c.runCLI(cfg, args)
			if runErr != nil {
				return newDevkitCollectionFailure("cli_execution", runErr)
			}
			logState.debugCLIOutput(roundID, "stdout", output)
			parsedOutput, parseErr := parseMemoryOutput(output, attempt, c.logger)
			if parseErr != nil {
				return newDevkitCollectionFailure("parse", parseErr)
			}
			crossCheckMemory(parsedOutput, c.logger)
			logState.debugParseSummary(roundID,
				"cache_miss", len(parsedOutput.cacheMiss),
				"l3_rows", len(parsedOutput.l3),
				"ddrc_cells", len(parsedOutput.ddrc),
			)
			parsed = parsedOutput
			return nil
		},
		"target_type", attempt.targetType,
		"target", attempt.target,
		"cpu", cfg.CPU,
		"duration_seconds", cfg.Duration,
		"metric", 1,
		"period_milliseconds", cfg.Period,
	)
	if err != nil {
		c.markFailure(attempt)
		return
	}
	parsed.lastSuccessTime = float64(time.Now().Unix())
	c.cache.Store(parsed)
}

func (c *devkitMemoryCollector) collectionLogState() *devkitCollectionLogState {
	// 测试或特殊构造可能未注入 logState；懒初始化保持与生产 factory 相同的日志合同。
	if c.logState == nil {
		c.logState = newDevkitCollectionLogState(c.logger, "devkit-memory")
	}
	return c.logState
}

// runCLI 执行 memory 命令，超时 = duration + 5s，防止 CLI hang。
func (c *devkitMemoryCollector) runCLI(cfg DevkitMemoryConfig, args []string) (string, error) {
	timeout := time.Duration(cfg.Duration+5) * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	runner := c.runCommand
	if runner == nil {
		runner = runDevkitCommand
	}
	return runner(ctx, c.binaryPath, args...)
}

// markFailure 仅保存本轮失败状态和最近成功时间，不保留上一轮业务数据。
func (c *devkitMemoryCollector) markFailure(attempt devkitAttemptMetadata) {
	var lastSuccessTime float64
	if prev := c.cache.Load(); prev != nil {
		lastSuccessTime = prev.lastSuccessTime
	}
	c.cache.Store(&memoryMetricCache{
		devkitAttemptMetadata: attempt,
		success:               false,
		lastSuccessTime:       lastSuccessTime,
	})
}

// memoryCommandArgs 返回实际传给 DevKit CLI 的参数，命令执行与结构化日志共用。
func memoryCommandArgs(cfg DevkitMemoryConfig) []string {
	duration := cfg.Duration
	if duration < minDevkitDuration || duration > maxDevkitDuration {
		duration = defaultDevkitDuration
	}
	period := cfg.Period
	if period != 100 && period != 1000 {
		if duration == 1 {
			period = 100
		} else {
			period = defaultDevkitMemoryPeriod
		}
	}
	// metric=1 (ALL) 是 Collector 的固定采集合同，不接受外部配置覆盖。
	args := []string{"tuner", "memory", "-d", strconv.Itoa(duration), "-m", "1", "-P", strconv.Itoa(period)}
	if cfg.CPU != "" {
		args = append(args, "-c", cfg.CPU)
	}
	return args
}

// Update 实现 Collector 接口，零延迟读缓存并写入 channel。
func (c *devkitMemoryCollector) Update(ch chan<- prometheus.Metric) error {
	cached := c.cache.Load()
	if cached == nil {
		return ErrNoData
	}

	success := 0.0
	if cached.success {
		success = 1.0
	}
	period := strconv.Itoa(cached.periodMilliseconds)
	ch <- prometheus.MustNewConstMetric(
		c.collectionSuccessDesc, prometheus.GaugeValue, success,
		cached.targetType, cached.target, period,
	)
	if cached.lastSuccessTime > 0 {
		ch <- prometheus.MustNewConstMetric(c.lastSuccessDesc, prometheus.GaugeValue, cached.lastSuccessTime)
	}

	// 采集失败时只发布健康状态和已有的最后成功时间。
	if !cached.success {
		return nil
	}

	for _, item := range cached.cacheMiss {
		ch <- prometheus.MustNewConstMetric(c.cacheMissDesc, prometheus.GaugeValue, item.percent, item.component, cached.targetType, cached.target, period)
	}
	for _, item := range cached.ddrSystem {
		ch <- prometheus.MustNewConstMetric(c.ddrSystemBandwidthDesc, prometheus.GaugeValue, item.value, item.operation, cached.targetType, cached.target, period)
	}
	for _, cell := range cached.access {
		if cell.hasBW {
			ch <- prometheus.MustNewConstMetric(c.accessBandwidthDesc, prometheus.GaugeValue, cell.bandwidth, cell.component, cell.cpu, cached.targetType, cached.target, period)
		}
		if cell.hasHit {
			ch <- prometheus.MustNewConstMetric(c.accessHitDesc, prometheus.GaugeValue, cell.hitPercent, cell.component, cell.cpu, cached.targetType, cached.target, period)
		}
	}
	for _, row := range cached.l3 {
		ch <- prometheus.MustNewConstMetric(c.l3ReadBandwidthDesc, prometheus.GaugeValue, row.readBandwidth, row.node, row.ccl, cached.targetType, cached.target, period)
		ch <- prometheus.MustNewConstMetric(c.l3ReadHitBandwidthDesc, prometheus.GaugeValue, row.readHitBandwidth, row.node, row.ccl, cached.targetType, cached.target, period)
		ch <- prometheus.MustNewConstMetric(c.l3ReadHitDesc, prometheus.GaugeValue, row.readHitPercent, row.node, row.ccl, cached.targetType, cached.target, period)
	}
	for _, cell := range cached.ddrc {
		ch <- prometheus.MustNewConstMetric(c.ddrcBandwidthDesc, prometheus.GaugeValue, cell.read, cell.node, cell.ddrc, "read", cached.targetType, cached.target, period)
		ch <- prometheus.MustNewConstMetric(c.ddrcBandwidthDesc, prometheus.GaugeValue, cell.write, cell.node, cell.ddrc, "write", cached.targetType, cached.target, period)
	}
	return nil
}
