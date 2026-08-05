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
	"os"
	"regexp"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"sigs.k8s.io/yaml"
)

const (
	// devkitBinaryPathEnv 指向镜像内 DevKit Tuner CLI 的可执行文件。
	devkitBinaryPathEnv = "DEVKIT_BINARY_PATH"
	// devkitConfigNamespaceEnv/devkitConfigNameEnv 指向承载采集参数的 ConfigMap。
	devkitConfigNamespaceEnv = "DEVKIT_CONFIG_NAMESPACE"
	devkitConfigNameEnv      = "DEVKIT_CONFIG_NAME"
	// devkitCollectIntervalEnv 是异步采集 ticker 周期（秒），建议等于 scrape_interval。
	devkitCollectIntervalEnv = "DEVKIT_COLLECT_INTERVAL"

	defaultDevkitBinaryPath   = "/usr/local/devkit/devkit"
	defaultDevkitConfigName   = "kunpeng-perf-monitor-devkit-config"
	defaultDevkitDuration     = 3
	defaultDevkitMemoryPeriod = 1000
	defaultDevkitInterval     = 15 * time.Second
	defaultDevkitSyncTimeout  = 5 * time.Second
	minDevkitDuration         = 1
	maxDevkitDuration         = 5
	// 两个 Collector 的 duration 最大各 5s，再预留 1s 进程与 Parser 开销。
	devkitNominalCapacityBudget = 11 * time.Second
)

// cpuRangePattern 校验 CPU 范围，如 "0" "0,1,2" "0-31" "10,80,89-99"。
var cpuRangePattern = regexp.MustCompile(`^\d+(-\d+)?(,\d+(-\d+)?)*$`)

// devkitCollectIntervalPattern restricts the environment value to ASCII digits.
var devkitCollectIntervalPattern = regexp.MustCompile(`^[0-9]+$`)

// pidListPattern 校验 PID 列表，仅接受纯数字或逗号分隔的数字列表，拒绝 "ALL"。
var pidListPattern = regexp.MustCompile(`^\d+(,\d+)*$`)

// DevkitTopdownConfig 是 top-down 采集器的配置合同。
// 采集范围只支持 cpu/pid（互斥），-L 恒为 0，不提供 cgroup、profileLevel 字段。
type DevkitTopdownConfig struct {
	CPU      string `json:"cpu"`      // 与 pid 互斥
	PID      string `json:"pid"`      // 纯数字或数字列表，不接受 "ALL"
	Duration int    `json:"duration"` // 仅 1..5，默认 3
}

// DevkitMemoryConfig 是 memory 采集器的配置合同。
// memory 无 pid/cgroup 入口，只支持 cpu 与全系统采集。
type DevkitMemoryConfig struct {
	CPU      string `json:"cpu"`      // 为空时采集所有 CPU 核
	Duration int    `json:"duration"` // 仅 1..5，默认 3
	Period   int    `json:"period"`   // 仅 100 或 1000
}

// DevkitConfig 是 ConfigMap 中 devkit-tuner.yaml 的完整结构。
type DevkitConfig struct {
	Topdown DevkitTopdownConfig `json:"topdown"`
	Memory  DevkitMemoryConfig  `json:"memory"`
}

// YAML 解码结构使用指针保存 duration 的字段存在性，区分缺失默认值与显式非法的 0。
type rawDevkitConfig struct {
	Topdown rawDevkitTopdownConfig `json:"topdown"`
	Memory  rawDevkitMemoryConfig  `json:"memory"`
}

type rawDevkitTopdownConfig struct {
	CPU      string `json:"cpu"`
	PID      string `json:"pid"`
	Duration *int   `json:"duration"`
}

type rawDevkitMemoryConfig struct {
	CPU      string `json:"cpu"`
	Duration *int   `json:"duration"`
	Period   *int   `json:"period"`
}

// validate 对整份配置做静态校验；返回错误时调用方应保留上一份有效配置。
func (c *DevkitConfig) validate() error {
	if c.Topdown.CPU != "" && c.Topdown.PID != "" {
		return fmt.Errorf("topdown CPU and PID are mutually exclusive")
	}
	if c.Topdown.CPU != "" && !cpuRangePattern.MatchString(c.Topdown.CPU) {
		return fmt.Errorf("topdown CPU range has invalid format: %q", c.Topdown.CPU)
	}
	if c.Topdown.PID != "" && !pidListPattern.MatchString(c.Topdown.PID) {
		return fmt.Errorf("topdown PID accepts only a number or a comma-separated number list: %q", c.Topdown.PID)
	}
	if c.Memory.CPU != "" && !cpuRangePattern.MatchString(c.Memory.CPU) {
		return fmt.Errorf("memory CPU range has invalid format: %q", c.Memory.CPU)
	}
	if c.Memory.Period != 100 && c.Memory.Period != 1000 {
		return fmt.Errorf("memory period must be 100 or 1000: %d", c.Memory.Period)
	}
	if c.Memory.Duration == 1 && c.Memory.Period == 1000 {
		return fmt.Errorf("when memory duration=1, period must be omitted or explicitly set to 100")
	}
	return nil
}

// withDefaults 返回填好默认值的副本，供命令组装阶段使用。
func (c DevkitConfig) withDefaults() DevkitConfig {
	if c.Topdown.Duration < minDevkitDuration || c.Topdown.Duration > maxDevkitDuration {
		c.Topdown.Duration = defaultDevkitDuration
	}
	if c.Memory.Duration < minDevkitDuration || c.Memory.Duration > maxDevkitDuration {
		c.Memory.Duration = defaultDevkitDuration
	}
	if c.Memory.Period != 100 && c.Memory.Period != 1000 {
		if c.Memory.Duration == 1 {
			c.Memory.Period = 100
		} else {
			c.Memory.Period = defaultDevkitMemoryPeriod
		}
	}
	return c
}

func parseDevkitConfig(raw []byte) (DevkitConfig, error) {
	var decoded rawDevkitConfig
	if err := yaml.UnmarshalStrict(raw, &decoded); err != nil {
		return DevkitConfig{}, fmt.Errorf("parse YAML: %w", err)
	}

	topdownDuration, err := normalizeDevkitDuration("topdown", decoded.Topdown.Duration)
	if err != nil {
		return DevkitConfig{}, err
	}
	memoryDuration, err := normalizeDevkitDuration("memory", decoded.Memory.Duration)
	if err != nil {
		return DevkitConfig{}, err
	}
	memoryPeriod, err := normalizeDevkitMemoryPeriod(memoryDuration, decoded.Memory.Period)
	if err != nil {
		return DevkitConfig{}, err
	}
	cfg := DevkitConfig{
		Topdown: DevkitTopdownConfig{
			CPU:      decoded.Topdown.CPU,
			PID:      decoded.Topdown.PID,
			Duration: topdownDuration,
		},
		Memory: DevkitMemoryConfig{
			CPU:      decoded.Memory.CPU,
			Duration: memoryDuration,
			Period:   memoryPeriod,
		},
	}
	if err := cfg.validate(); err != nil {
		return DevkitConfig{}, err
	}
	return cfg.withDefaults(), nil
}

func normalizeDevkitMemoryPeriod(duration int, value *int) (int, error) {
	if value == nil {
		if duration == 1 {
			return 100, nil
		}
		return defaultDevkitMemoryPeriod, nil
	}
	if *value != 100 && *value != 1000 {
		return 0, fmt.Errorf("memory period must be 100 or 1000: %d", *value)
	}
	if duration == 1 && *value == 1000 {
		return 0, fmt.Errorf("when memory duration=1, period must be omitted or explicitly set to 100")
	}
	return *value, nil
}

func normalizeDevkitDuration(section string, value *int) (int, error) {
	if value == nil {
		return defaultDevkitDuration, nil
	}
	if *value < minDevkitDuration || *value > maxDevkitDuration {
		return 0, fmt.Errorf("%s duration must be an integer between %d and %d: %d", section, minDevkitDuration, maxDevkitDuration, *value)
	}
	return *value, nil
}

// DevkitConfigWatcher 通过 client-go Informer 监听 ConfigMap，
// 用 atomic.Pointer 无锁地向采集 goroutine 发布最新配置。
type DevkitConfigWatcher struct {
	clientset kubernetes.Interface
	namespace string
	name      string
	config    atomic.Pointer[DevkitConfig]
	logger    *slog.Logger
}

// devkitConfigWatcherProvider ensures TopDown and Memory share one informer,
// one clientset and one initial-sync wait for the same ConfigMap.
type devkitConfigWatcherProvider struct {
	once       sync.Once
	newWatcher func(*slog.Logger) *DevkitConfigWatcher
	watcher    *DevkitConfigWatcher
}

func (p *devkitConfigWatcherProvider) get(logger *slog.Logger) *DevkitConfigWatcher {
	p.once.Do(func() {
		p.watcher = p.newWatcher(logger)
	})
	return p.watcher
}

var sharedDevkitConfigWatcherProvider = devkitConfigWatcherProvider{
	newWatcher: newDevkitConfigWatcher,
}

func getSharedDevkitConfigWatcher(logger *slog.Logger) *DevkitConfigWatcher {
	return sharedDevkitConfigWatcherProvider.get(logger)
}

// initializeSharedDevkitConfigWatcher initializes the shared watcher with a
// neutral component logger before collector factories add collector-specific
// attributes in the map iteration below.
func initializeSharedDevkitConfigWatcher(logger *slog.Logger) {
	sharedDevkitConfigWatcherProvider.get(logger.With("component", "devkit-config"))
}

// devkitBinaryPath 返回 DevKit CLI 路径，未设置环境变量时取默认安装路径。
func devkitBinaryPath() string {
	if path := os.Getenv(devkitBinaryPathEnv); path != "" {
		return path
	}
	return defaultDevkitBinaryPath
}

// parseDevkitCollectInterval parses the environment value as a positive integer
// number of seconds and rejects unit suffixes, fractions and overflow.
func parseDevkitCollectInterval(raw string) (time.Duration, error) {
	if raw == "" {
		return defaultDevkitInterval, nil
	}

	if !devkitCollectIntervalPattern.MatchString(raw) {
		return 0, fmt.Errorf("must be a positive integer number of seconds")
	}
	secs, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || secs <= 0 {
		return 0, fmt.Errorf("must be a positive integer number of seconds")
	}
	const maxDurationSeconds = int64((1<<63 - 1) / int64(time.Second))
	if secs > maxDurationSeconds {
		return 0, fmt.Errorf("exceeds the maximum supported duration")
	}
	return time.Duration(secs) * time.Second, nil
}

// devkitCollectInterval returns the async collection ticker period.
func devkitCollectInterval() time.Duration {
	interval, err := parseDevkitCollectInterval(os.Getenv(devkitCollectIntervalEnv))
	if err != nil {
		return defaultDevkitInterval
	}
	return interval
}

// newDevkitConfigWatcher 构造并启动 ConfigMap Watcher。
// 若集群内配置不可用（如本地运行、无 ServiceAccount），返回的 watcher 仍可用，
// 只是始终提供默认配置，保证采集器不因缺少 ConfigMap 而无法启动。
func newDevkitConfigWatcher(logger *slog.Logger) *DevkitConfigWatcher {
	interval, err := parseDevkitCollectInterval(os.Getenv(devkitCollectIntervalEnv))
	if err != nil {
		logger.Warn("devkit_collect_interval_invalid",
			"env", devkitCollectIntervalEnv,
			"value", os.Getenv(devkitCollectIntervalEnv),
			"reason", err,
			"fallback_seconds", defaultDevkitInterval/time.Second,
		)
		interval = defaultDevkitInterval
	}
	warnDevkitCapacity(logger, interval)

	namespace := os.Getenv(devkitConfigNamespaceEnv)
	if namespace == "" {
		namespace = "default"
	}
	name := os.Getenv(devkitConfigNameEnv)
	if name == "" {
		name = defaultDevkitConfigName
	}

	w := &DevkitConfigWatcher{
		namespace: namespace,
		name:      name,
		logger:    logger,
	}
	// 先放入一份带默认值的空配置，保证 Load() 永不返回 nil。
	initial := DevkitConfig{}
	w.config.Store(&initial)

	config, err := rest.InClusterConfig()
	if err != nil {
		logger.Warn("devkit: in-cluster config is unavailable; using default collection configuration", "err", err)
		return w
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		logger.Warn("devkit: failed to create Kubernetes clientset; using default collection configuration", "err", err)
		return w
	}
	w.clientset = clientset
	w.start(context.Background())
	return w
}

// start 启动 Informer，等待首次缓存同步后再返回，并持续监听目标 ConfigMap 的 Add/Update 事件。
func (w *DevkitConfigWatcher) start(ctx context.Context) {
	factory := informers.NewSharedInformerFactoryWithOptions(
		w.clientset,
		0,
		informers.WithNamespace(w.namespace),
		informers.WithTweakListOptions(func(opts *metav1.ListOptions) {
			opts.FieldSelector = fmt.Sprintf("metadata.name=%s", w.name)
		}),
	)
	informer := factory.Core().V1().ConfigMaps().Informer()
	handlerRegistration, err := informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    func(obj interface{}) { w.onUpdate(obj) },
		UpdateFunc: func(_, obj interface{}) { w.onUpdate(obj) },
		DeleteFunc: func(obj interface{}) { w.onDelete(obj) },
	})
	if err != nil {
		w.logger.Warn("devkit: failed to register ConfigMap event handler; using default collection configuration", "err", err)
		return
	}
	go factory.Start(ctx.Done())

	syncCtx, cancel := context.WithTimeout(ctx, defaultDevkitSyncTimeout)
	defer cancel()
	if !cache.WaitForCacheSync(syncCtx.Done(), handlerRegistration.HasSynced) {
		if ctxErr := ctx.Err(); ctxErr != nil {
			w.logger.Info("devkit: ConfigMap watcher stopped before initial sync",
				"namespace", w.namespace, "name", w.name, "err", ctxErr)
			return
		}
		w.logger.Warn("devkit: initial ConfigMap sync timed out; continuing with default collection configuration",
			"namespace", w.namespace, "name", w.name, "timeout", defaultDevkitSyncTimeout)
		return
	}
	w.logger.Info("devkit: ConfigMap watcher started and completed initial sync", "namespace", w.namespace, "name", w.name)
}

func warnDevkitCapacity(logger *slog.Logger, interval time.Duration) {
	if logger == nil || interval >= devkitNominalCapacityBudget {
		return
	}
	logger.Warn("devkit_capacity_warning",
		"collect_interval_seconds", interval.Seconds(),
		"nominal_budget_seconds", devkitNominalCapacityBudget.Seconds(),
		"reason", "collect interval is below the fixed healthy-path capacity budget",
	)
}

// onUpdate 解析 ConfigMap 并原子替换配置；解析或校验失败则保留上一份有效配置。
func (w *DevkitConfigWatcher) onUpdate(obj interface{}) {
	cm, ok := obj.(*corev1.ConfigMap)
	if !ok {
		return
	}
	raw, ok := cm.Data["devkit-tuner.yaml"]
	if !ok {
		w.logger.Warn("devkit: ConfigMap is missing devkit-tuner.yaml; keeping last known good configuration")
		return
	}
	cfg, err := parseDevkitConfig([]byte(raw))
	if err != nil {
		w.logger.Warn("devkit_config_rejected",
			"config_key", "devkit-tuner.yaml",
			"action", "keep_last_known_good",
			"err", err,
		)
		return
	}
	w.config.Store(&cfg)
	w.logger.Info("devkit: collection configuration reloaded",
		"topdown_cpu", cfg.Topdown.CPU, "topdown_pid", cfg.Topdown.PID, "topdown_duration", cfg.Topdown.Duration,
		"memory_cpu", cfg.Memory.CPU, "memory_duration", cfg.Memory.Duration,
		"memory_metric", 1, "memory_period", cfg.Memory.Period)
}

// onDelete 保留最后一份有效配置。自动回退 system 默认值可能扩大采集范围，
// 因此删除 ConfigMap 只记录 Warning，恢复默认值必须由用户显式提交配置。
func (w *DevkitConfigWatcher) onDelete(obj interface{}) {
	cm, ok := obj.(*corev1.ConfigMap)
	if !ok {
		tombstone, tombstoneOK := obj.(cache.DeletedFinalStateUnknown)
		if !tombstoneOK {
			return
		}
		cm, ok = tombstone.Obj.(*corev1.ConfigMap)
		if !ok {
			return
		}
	}
	if cm.Name != w.name || (cm.Namespace != "" && cm.Namespace != w.namespace) {
		return
	}
	w.logger.Warn("devkit_config_deleted",
		"namespace", w.namespace,
		"name", w.name,
		"action", "keep_last_known_good",
	)
}

// load 返回当前生效配置（已填默认值），永不返回 nil。
func (w *DevkitConfigWatcher) load() DevkitConfig {
	return w.config.Load().withDefaults()
}
