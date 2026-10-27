package templatecreator

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"sync"
	"time"

	"go.uber.org/zap"

	"gitcode.com/cloud-native/cloud-native/TemplateCreator/internal/basecache"
	"gitcode.com/cloud-native/cloud-native/TemplateCreator/internal/idempotency"
	"gitcode.com/cloud-native/cloud-native/TemplateCreator/internal/machineinfo"
	buildruntime "gitcode.com/cloud-native/cloud-native/TemplateCreator/internal/runtime"
	"gitcode.com/cloud-native/cloud-native/TemplateCreator/internal/upstream/shared/pkg/logger"
)

// ErrIdempotencyConflict 表示某个任务编号已经由另一份安全请求内容完成。
// registry credential 永远不参与该摘要，避免 token/password 被散列或持久化。
var ErrIdempotencyConflict = idempotency.ErrConflict

// ErrCreatorClosed 表示 Creator 已开始关闭，不再接收新的 Template 请求。
var ErrCreatorClosed = errors.New("creator is closed")

const creatorCleanupTimeout = 30 * time.Second

// creatorRuntime 是 Creator 持有的最小进程级 runtime contract，便于隔离资源生命周期和测试。
type creatorRuntime interface {
	Close(context.Context) error
}

// creatorRuntimeFactory 在配置验证后创建真实或测试 runtime。
type creatorRuntimeFactory func(context.Context, CreatorConfig) (creatorRuntime, error)

// Creator 持有多个同步 Template 构建共享的进程级资源。
// 一个 Creator 可顺序或受 MaxParallel 限制地执行多个请求，使用结束后必须调用 Close。
type Creator struct {
	// config 是验证并归一化后的不可变配置副本。
	config CreatorConfig
	// runtime 持有 proxy、firewall、NBD/network pool 等进程级资源。
	runtime creatorRuntime
	// logger 与 upstream Builder 使用相同 infra logger 配置，并仅写调用方指定的 writer。
	logger logger.Logger
	// machineInfo 在生产 Creator 初始化时从构建宿主机采集并冻结，随后复制到每次 Build metadata。
	machineInfo MachineInfo
	// lifecycleMu 同时保护请求接纳和关闭状态，避免 Close 与新请求交错时漏等。
	lifecycleMu sync.Mutex
	closing     bool
	activeCalls int
	activeDone  chan struct{}
	closeDone   chan struct{}
	closeErr    error
	// idempotency 在 Artifact Store 中记录已完成结果；semaphore 只限制当前进程真正执行的物理构建。
	// 命中任务结果缓存的请求不会占用并发名额。
	idempotency *idempotency.Index
	semaphore   chan struct{}
	// removeAll 是窄化的清理测试 seam，用于验证构建错误与 cleanup 错误能够同时返回。
	removeAll func(string) error
}

// NewCreator 验证显式配置并初始化真实 Linux runtime。
// 初始化会启动 goroutine 并持有宿主机资源，因此成功返回的 Creator 必须调用 Close。
func NewCreator(ctx context.Context, config CreatorConfig) (*Creator, error) {
	config = normalizeCreatorConfig(config)
	// 先验证公开配置再检测 CPU，避免一个明显的配置错误被宿主机探测错误掩盖。
	if err := validateCreatorConfig(config); err != nil {
		return nil, err
	}
	detected, err := machineinfo.Detect()
	if err != nil {
		return nil, fmt.Errorf("detect build machine CPU: %w", err)
	}
	if detected.CPUArchitecture != config.Runtime.Architecture {
		return nil, fmt.Errorf("detect build machine CPU: architecture %q does not match runtime architecture %q", detected.CPUArchitecture, config.Runtime.Architecture)
	}
	creator, err := newCreatorWithFactory(ctx, config, func(ctx context.Context, config CreatorConfig) (creatorRuntime, error) {
		return buildruntime.NewProcessRuntime(ctx, processConfigFromCreatorConfig(config))
	})
	if err != nil {
		return nil, err
	}
	creator.machineInfo = MachineInfo{
		CPUArchitecture: detected.CPUArchitecture,
		CPUFamily:       detected.CPUFamily,
		CPUModel:        detected.CPUModel,
		CPUModelName:    detected.CPUModelName,
		CPUFlags:        append([]string{}, detected.CPUFlags...),
	}

	return creator, nil
}

// processConfigFromCreatorConfig 把稳定公开配置收敛到私有 runtime contract。
// logger 与 writer 只作为进程内状态传递，不参与构建结果、任务内容摘要或持久化。
func processConfigFromCreatorConfig(config CreatorConfig) buildruntime.ProcessConfig {
	return buildruntime.ProcessConfig{
		ScratchRoot: config.ScratchRoot,
		BaseCache: buildruntime.BaseCacheConfig{
			Enabled: config.Cache.Enabled, Root: config.Cache.Root,
			AllowMutableImageTags: config.Cache.AllowMutableImageTags,
			ABI: basecache.RuntimeABI{
				InfraCommit: config.Runtime.InfraCommit, Architecture: config.Runtime.Architecture,
				TemplateVersion: config.Runtime.TemplateVersion, KernelVersion: config.Runtime.KernelVersion,
				FirecrackerVersion: config.Runtime.FirecrackerVersion, EnvdVersion: config.Runtime.EnvdVersion,
				BusyboxVersion: config.Runtime.BusyboxVersion,
			},
		},
		ProxyPort: config.Runtime.ProxyPort,
		LogWriter: config.LogWriter,
		DebugLogs: config.DebugLogs,
		Logger:    config.progressLogger,
	}
}

// newCreatorWithFactory 统一生产和测试初始化路径，并保证部分初始化失败时回滚已持有资源。
func newCreatorWithFactory(ctx context.Context, config CreatorConfig, factory creatorRuntimeFactory) (*Creator, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	config = normalizeCreatorConfig(config)
	if err := validateCreatorConfig(config); err != nil {
		return nil, err
	}
	progressLogger, err := buildruntime.NewLogger(config.LogWriter, config.DebugLogs)
	if err != nil {
		return nil, fmt.Errorf("initialize TemplateCreator logger: %w", err)
	}
	config.progressLogger = progressLogger
	progressLogger.Info(ctx, "Creator runtime 初始化开始", zap.String("stage", "creator_runtime_initialization_started"))
	runtime, err := factory(ctx, config)
	if err != nil {
		progressLogger.Error(ctx, "Creator runtime 初始化失败", zap.String("stage", "creator_runtime_initialization_failed"), zap.Error(err))
		return nil, fmt.Errorf("initialize TemplateCreator runtime: %w", err)
	}
	progressLogger.Info(ctx, "Creator runtime 初始化完成", zap.String("stage", "creator_runtime_initialization_completed"))
	storeURL, _ := url.Parse(config.Store.URL)
	index, err := idempotency.New(storeURL.Path)
	if err != nil {
		// runtime 此时可能已经持有 goroutine 和 device pool；初始化任务结果索引失败时
		// 必须先回滚这些资源，并通过 errors.Join 同时保留根因和关闭错误。
		closeErr := runtime.Close(context.Background())
		progressLogger.Error(ctx, "任务结果索引初始化失败", zap.String("stage", "idempotency_initialization_failed"), zap.Error(err))
		return nil, errors.Join(fmt.Errorf("initialize idempotency index: %w", err), closeErr)
	}
	creator := &Creator{
		config: config, runtime: runtime, logger: progressLogger, idempotency: index,
		semaphore: make(chan struct{}, config.MaxParallel), removeAll: os.RemoveAll,
		closeDone: make(chan struct{}),
	}
	creator.activeDone = make(chan struct{})
	close(creator.activeDone) // 初始状态没有在途请求，Close 无需等待。
	progressLogger.Info(ctx, "Creator 初始化完成", zap.String("stage", "creator_initialized"), zap.Int("max_parallel", config.MaxParallel))
	return creator, nil
}

// beginCall 登记一次请求，和 Close 的状态切换使用同一把锁。
func (creator *Creator) beginCall() error {
	creator.lifecycleMu.Lock()
	defer creator.lifecycleMu.Unlock()
	if creator.closing {
		return ErrCreatorClosed
	}
	if creator.activeCalls == 0 {
		creator.activeDone = make(chan struct{})
	}
	creator.activeCalls++
	return nil
}

// endCall 在最后一个已接纳请求返回时通知 Close。
func (creator *Creator) endCall() {
	creator.lifecycleMu.Lock()
	defer creator.lifecycleMu.Unlock()
	creator.activeCalls--
	if creator.activeCalls == 0 {
		close(creator.activeDone)
	}
}

// Close 拒绝新请求，等待在途请求结束，再释放共享的进程级资源。
// 等待者的 context 到期只结束该次等待，不会提前关闭仍被构建使用的资源。
func (creator *Creator) Close(ctx context.Context) error {
	creator.lifecycleMu.Lock()
	if !creator.closing {
		creator.closing = true
		// 关闭工作使用独立于调用方 deadline 的 context；否则 Close 超时会破坏在途 job。
		go creator.finishClose(context.WithoutCancel(ctx), creator.activeDone)
	}
	done := creator.closeDone
	creator.lifecycleMu.Unlock()

	select {
	case <-done:
		return creator.closeErr
	case <-ctx.Done():
		return ctx.Err()
	}
}

// finishClose 只启动一次，activeDone 关闭后才释放 runtime。
func (creator *Creator) finishClose(ctx context.Context, activeDone <-chan struct{}) {
	<-activeDone
	// 在最后一个请求退出后才开始计时；已取消的调用方 context 不影响资源回收。
	ctx, cancel := context.WithTimeout(ctx, creatorCleanupTimeout)
	defer cancel()
	if creator.logger != nil {
		creator.logger.Info(ctx, "Creator 关闭开始", zap.String("stage", "creator_runtime_close_started"))
	}
	var err error
	if creator.runtime != nil {
		err = creator.runtime.Close(ctx)
	}
	if creator.logger != nil {
		if err != nil {
			creator.logger.Error(ctx, "Creator 关闭失败", zap.String("stage", "creator_runtime_close_failed"), zap.Error(err))
		} else {
			creator.logger.Info(ctx, "Creator 关闭完成", zap.String("stage", "creator_runtime_close_completed"))
		}
	}
	creator.closeErr = err
	close(creator.closeDone)
}
