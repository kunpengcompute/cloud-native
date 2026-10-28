//go:build linux

package runtime

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"

	"github.com/launchdarkly/go-sdk-common/v3/ldlog"
	"go.opentelemetry.io/otel/metric/noop"
	"go.uber.org/zap"

	"gitcode.com/boostkit/cloud-native.git/TemplateCreator/internal/upstream/orchestrator/pkg/proxy"
	"gitcode.com/boostkit/cloud-native.git/TemplateCreator/internal/upstream/orchestrator/pkg/sandbox"
	"gitcode.com/boostkit/cloud-native.git/TemplateCreator/internal/upstream/orchestrator/pkg/sandbox/nbd"
	"gitcode.com/boostkit/cloud-native.git/TemplateCreator/internal/upstream/orchestrator/pkg/sandbox/network"
	"gitcode.com/boostkit/cloud-native.git/TemplateCreator/internal/upstream/orchestrator/pkg/tcpfirewall"
	featureflags "gitcode.com/boostkit/cloud-native.git/TemplateCreator/internal/upstream/shared/pkg/featureflags"
	"gitcode.com/boostkit/cloud-native.git/TemplateCreator/internal/upstream/shared/pkg/logger"
)

// ProcessConfig 只包含进程级 runtime 所需的显式值；生产初始化不会修改环境变量或替换全局 logger。
type ProcessConfig struct {
	// ScratchRoot 是各构建 job 临时目录的共同父目录。
	ScratchRoot string
	// BaseCache 配置可选的跨 job 持久 storage 和 runtime ABI namespace。
	BaseCache BaseCacheConfig
	// ProxyPort 是 sandbox proxy 的本地监听端口。
	ProxyPort uint16
	// NBDPoolSize 为稳定配置 ABI 保留；infra 2026.09 会从 sysfs 自动发现 NBD device，
	// 不接受调用方指定的 pool size。
	NBDPoolSize int
	// NewSlots 是新建 network slot 的缓冲数量。
	NewSlots int
	// ReusedSlots 是可复用 network slot 的缓冲数量。
	ReusedSlots int
	// LogWriter 接收 TemplateCreator 和 upstream Builder 日志；nil 表示不输出。
	LogWriter io.Writer
	// DebugLogs 控制是否输出 debug 级别日志。
	DebugLogs bool
	// Logger 允许 Creator 与 ProcessRuntime 复用同一 infra logger；nil 时由 runtime 自行创建。
	Logger logger.Logger
}

// ProcessRuntime 持有可以安全地在多个构建 job 间共享的进程级资源。
type ProcessRuntime struct {
	// Config 是已经应用 runtime 默认值的配置。
	Config ProcessConfig
	// Logger 和 FeatureFlags 供抽取的 upstream Builder 依赖使用。
	Logger       logger.Logger
	FeatureFlags *featureflags.Client
	// Sandboxes 是 proxy、firewall 与 job factory 共享的活动 sandbox registry。
	Sandboxes *sandbox.Map
	// SandboxProxy 和 TCPFirewall 提供 guest 构建阶段所需的本地网络服务。
	SandboxProxy *proxy.SandboxProxy
	TCPFirewall  *tcpfirewall.Proxy
	// DevicePool 和 NetworkPool 是有界共享池，job 只借用 slot，不拥有池本身。
	DevicePool  *nbd.DevicePool
	NetworkPool *network.Pool
	// NetworkConfig 是所有共享网络组件采用的同一份解析结果。
	NetworkConfig network.Config
	cancel        context.CancelFunc
	closeOnce     sync.Once
	closeErr      error
}

// newProcessLifetimeContext 只由 ProcessRuntime.Close 停止进程级服务。
// 初始化 context 仍用于启动阶段校验，但构建请求或批次取消不能提前停掉在途 job 的共享资源。
func newProcessLifetimeContext(initializationCtx context.Context) (context.Context, context.CancelFunc) {
	return context.WithCancel(context.WithoutCancel(initializationCtx))
}

// closeSandboxProxy 在 graceful Shutdown 超时后再调用一次 Close；上游实现会在
// 已取消的 context 下改用强制关闭，避免活动 HTTP 连接让 Creator 永久卡住。
func closeSandboxProxy(ctx context.Context, server interface{ Close(context.Context) error }) error {
	err := server.Close(ctx)
	if err != nil && ctx.Err() != nil {
		return errors.Join(err, server.Close(ctx))
	}
	return err
}

// NewProcessRuntime 启动所有 job 共享的 proxy、firewall 及有界 device/network pool。
// 可写 Template storage 和 cache 被有意排除，它们由 BuildJob 为每次构建独立创建。
func NewProcessRuntime(ctx context.Context, config ProcessConfig) (*ProcessRuntime, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := validateInfra202609BaseCacheEnabled(config.BaseCache.Enabled); err != nil {
		return nil, err
	}
	if config.ProxyPort == 0 {
		config.ProxyPort = 5007
	}
	if config.NBDPoolSize <= 0 {
		config.NBDPoolSize = 8
	}
	if config.NewSlots <= 0 {
		config.NewSlots = 8
	}
	if config.ReusedSlots <= 0 {
		config.ReusedSlots = 8
	}

	processCtx, cancel := newProcessLifetimeContext(ctx)
	runtime := &ProcessRuntime{Config: config, cancel: cancel}
	// 从这里开始，每个初始化步骤都可能留下活动资源；统一走 fail 回滚路径，
	// 使调用方同时拿到原始失败和 cleanup 失败。
	fail := func(cause error) (*ProcessRuntime, error) {
		if runtime.Logger != nil {
			runtime.Logger.Error(ctx, "ProcessRuntime 初始化失败", zap.String("stage", "process_runtime_initialization_failed"), zap.Error(cause))
		}
		return nil, errors.Join(cause, runtime.Close(context.Background()))
	}

	log := config.Logger
	var err error
	if log == nil {
		log, err = newProcessLogger(config.LogWriter, config.DebugLogs)
		if err != nil {
			cancel()
			return nil, fmt.Errorf("create logger: %w", err)
		}
	}
	runtime.Logger = log
	runtime.Logger.Info(ctx, "ProcessRuntime 初始化开始", zap.String("stage", "process_runtime_initialization_started"), zap.Uint16("proxy_port", config.ProxyPort))
	runtime.FeatureFlags, err = featureflags.NewClientWithLogLevel(ldlog.Error)
	if err != nil {
		return fail(fmt.Errorf("create feature flags: %w", err))
	}
	runtime.Logger.Debug(ctx, "Feature flags client 初始化完成", zap.String("stage", "feature_flags_initialized"))
	runtime.Sandboxes = sandbox.NewSandboxesMap()
	// sandbox proxy 与 firewall 查询同一个进程级 map；job-local factory 会把
	// 活动 sandbox 注册到这里，使两个网络组件观察一致状态。
	runtime.SandboxProxy, err = proxy.NewSandboxProxy(noop.NewMeterProvider(), config.ProxyPort, runtime.Sandboxes, runtime.FeatureFlags)
	if err != nil {
		return fail(fmt.Errorf("create sandbox proxy: %w", err))
	}
	runtime.Logger.Info(ctx, "Sandbox proxy 初始化完成", zap.String("stage", "sandbox_proxy_initialized"), zap.Uint16("proxy_port", config.ProxyPort))
	go func() {
		if startErr := runtime.SandboxProxy.Start(processCtx); startErr != nil && !errors.Is(startErr, http.ErrServerClosed) && !errors.Is(startErr, context.Canceled) {
			runtime.Logger.Error(processCtx, "sandbox proxy stopped", zap.Error(startErr))
		}
	}()

	runtime.NetworkConfig, err = network.ParseConfig()
	if err != nil {
		return fail(fmt.Errorf("parse network defaults: %w", err))
	}
	runtime.Logger.Info(ctx, "网络配置解析完成", zap.String("stage", "network_config_loaded"))
	runtime.TCPFirewall = tcpfirewall.New(runtime.Logger, runtime.NetworkConfig, runtime.Sandboxes, noop.NewMeterProvider(), runtime.FeatureFlags)
	go func() { _ = runtime.TCPFirewall.Start(processCtx) }()
	runtime.Logger.Info(ctx, "TCP firewall 已启动", zap.String("stage", "tcp_firewall_started"))

	runtime.DevicePool, err = nbd.NewDevicePool()
	if err != nil {
		return fail(fmt.Errorf("create NBD pool: %w", err))
	}
	// 获取空闲 NBD/network slot 可能阻塞，因此在后台填充；BuildJob 通过
	// upstream sandbox factory 消费 slot。
	go runtime.DevicePool.Populate(processCtx)
	runtime.Logger.Info(ctx, "NBD device pool 后台填充已启动", zap.String("stage", "nbd_pool_population_started"))

	slotStorage, err := network.NewStorageLocal(processCtx, runtime.NetworkConfig)
	if err != nil {
		return fail(fmt.Errorf("create network storage: %w", err))
	}
	runtime.NetworkPool = network.NewPool(config.NewSlots, config.ReusedSlots, slotStorage, runtime.NetworkConfig)
	go runtime.NetworkPool.Populate(processCtx)
	runtime.Logger.Info(ctx, "Network slot pool 后台填充已启动", zap.String("stage", "network_pool_population_started"), zap.Int("new_slots", config.NewSlots), zap.Int("reused_slots", config.ReusedSlots))
	runtime.Logger.Info(ctx, "ProcessRuntime 初始化完成", zap.String("stage", "process_runtime_initialized"))
	if err := ctx.Err(); err != nil {
		return fail(fmt.Errorf("process runtime initialization canceled: %w", err))
	}

	return runtime, nil
}

// Close 停止后台生产者并释放全部进程级资源；重复调用返回第一次关闭的结果。
func (runtime *ProcessRuntime) Close(ctx context.Context) error {
	runtime.closeOnce.Do(func() {
		if runtime.Logger != nil {
			runtime.Logger.Info(ctx, "ProcessRuntime 关闭开始", zap.String("stage", "process_runtime_close_started"))
		}
		// 必须先取消 producer，再关闭并排空 pool，避免关闭期间仍有 goroutine 写入新 slot。
		if runtime.cancel != nil {
			runtime.cancel()
		}
		// 某一项关闭失败后仍继续释放其他资源，最终通过 errors.Join 返回全部 cause。
		var result error
		if runtime.NetworkPool != nil {
			result = errors.Join(result, runtime.NetworkPool.Close(ctx))
		}
		if runtime.DevicePool != nil {
			result = errors.Join(result, runtime.DevicePool.Close(ctx))
		}
		if runtime.TCPFirewall != nil {
			result = errors.Join(result, runtime.TCPFirewall.Close(ctx))
		}
		if runtime.SandboxProxy != nil {
			result = errors.Join(result, closeSandboxProxy(ctx, runtime.SandboxProxy))
		}
		if runtime.FeatureFlags != nil {
			result = errors.Join(result, runtime.FeatureFlags.Close(ctx))
		}
		runtime.closeErr = result
		if runtime.Logger != nil {
			if result != nil {
				runtime.Logger.Error(ctx, "ProcessRuntime 关闭失败", zap.String("stage", "process_runtime_close_failed"), zap.Error(result))
			} else {
				runtime.Logger.Info(ctx, "ProcessRuntime 关闭完成", zap.String("stage", "process_runtime_close_completed"))
			}
		}
	})
	return runtime.closeErr
}
