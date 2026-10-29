//go:build linux

package runtime

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"go.opentelemetry.io/otel/metric/noop"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"gitcode.com/boostkit/cloud-native.git/TemplateCreator/internal/upstream/orchestrator/pkg/cfg"
	"gitcode.com/boostkit/cloud-native.git/TemplateCreator/internal/upstream/orchestrator/pkg/sandbox"
	blockmetrics "gitcode.com/boostkit/cloud-native.git/TemplateCreator/internal/upstream/orchestrator/pkg/sandbox/block/metrics"
	"gitcode.com/boostkit/cloud-native.git/TemplateCreator/internal/upstream/orchestrator/pkg/sandbox/network"
	sbxtemplate "gitcode.com/boostkit/cloud-native.git/TemplateCreator/internal/upstream/orchestrator/pkg/sandbox/template"
	build "gitcode.com/boostkit/cloud-native.git/TemplateCreator/internal/upstream/orchestrator/pkg/template/build"
	buildconfig "gitcode.com/boostkit/cloud-native.git/TemplateCreator/internal/upstream/orchestrator/pkg/template/build/config"
	registryauth "gitcode.com/boostkit/cloud-native.git/TemplateCreator/internal/upstream/orchestrator/pkg/template/build/core/oci/auth"
	"gitcode.com/boostkit/cloud-native.git/TemplateCreator/internal/upstream/orchestrator/pkg/template/build/core/systeminit"
	buildmetrics "gitcode.com/boostkit/cloud-native.git/TemplateCreator/internal/upstream/orchestrator/pkg/template/build/metrics"
	artifactsregistry "gitcode.com/boostkit/cloud-native.git/TemplateCreator/internal/upstream/shared/pkg/artifacts-registry"
	"gitcode.com/boostkit/cloud-native.git/TemplateCreator/internal/upstream/shared/pkg/dockerhub"
	templatemanager "gitcode.com/boostkit/cloud-native.git/TemplateCreator/internal/upstream/shared/pkg/grpc/template-manager"
	upstreamstorage "gitcode.com/boostkit/cloud-native.git/TemplateCreator/internal/upstream/shared/pkg/storage"
)

// JobStep 是 runtime package 面向私有 Builder contract 的 Template step 副本。
type JobStep struct {
	Type string
	Args []string
}

// JobRequest 包含组装一个隔离 infra 2026.09 Builder 所需的全部不可变输入。
type JobRequest struct {
	TemplateID string
	BuildID    string
	ImageRef   string
	// ImageTarPath 是 docker save 导出的本地镜像归档路径，与 ImageRef 互斥。
	ImageTarPath string

	// 这些字段保留稳定的内部 contract；2026.09 adapter 会拒绝 UseBaseCache。
	BypassBaseCache bool
	UseBaseCache    bool
	CacheScope      string
	TemplateRoot    string

	JobRoot        string
	RuntimeRoot    string
	EnvdPath       string
	BusyboxDir     string
	BusyboxVersion string
	BusyboxSHA256  string

	TemplateVersion    string
	Architecture       string
	KernelVersion      string
	FirecrackerVersion string

	VCPU       int64
	MemoryMB   int64
	DiskSizeMB int64
	HugePages  bool

	StartCommand string
	ReadyCommand string
	Steps        []JobStep
	RegistryAuth registryauth.RegistryAuthProvider
}

// builderConfigForJob 组装 2026.09 BuilderConfig。runtime view 解决旧版本
// 不识别 architecture 子目录的问题；`/fc-vm` 保持 snapshot host path ABI。
func builderConfigForJob(request JobRequest, networkConfig network.Config, view infra202609RuntimeView) cfg.BuilderConfig {
	return cfg.BuilderConfig{
		AllowSandboxInternet:   true,
		EnvdTimeout:            10 * time.Second,
		FirecrackerVersionsDir: view.FirecrackerVersionsDir,
		HostEnvdPath:           request.EnvdPath,
		HostKernelsDir:         view.KernelsDir,
		OrchestratorBaseDir:    filepath.Join(request.JobRoot, "orchestrator"),
		SandboxDir:             "/fc-vm",
		TemplatesDir:           filepath.Join(request.JobRoot, "orchestrator", "build-templates"),
		DefaultCacheDir:        filepath.Join(request.JobRoot, "orchestrator", "build"),
		NetworkConfig:          networkConfig,
		StorageConfig: upstreamstorage.Config{
			SandboxCacheDir:  filepath.Join(request.JobRoot, "sandbox-cache"),
			SnapshotCacheDir: filepath.Join(request.JobRoot, "snapshot-cache"),
			TemplateCacheDir: filepath.Join(request.JobRoot, "template-cache"),
		},
	}
}

// BuildJob 创建 job 隔离的可写依赖，然后调用真实 infra 2026.09 Builder。
func (runtime *ProcessRuntime) BuildJob(ctx context.Context, request JobRequest) (JobResult, error) {
	runtime.Logger.Info(ctx, "构建 job 准备开始", zap.String("stage", "build_job_preparation_started"), zap.String("template_id", request.TemplateID), zap.String("build_id", request.BuildID))
	if request.UseBaseCache {
		return JobResult{}, fmt.Errorf("shared base cache is not supported by the infra 2026.09 runtime adapter")
	}
	busyboxPath := filepath.Join(request.BusyboxDir, request.BusyboxVersion, request.Architecture, "busybox")
	if err := processBusyboxInstaller.install(busyboxPath, request.BusyboxSHA256, &systeminit.BusyboxBinary); err != nil {
		return JobResult{}, err
	}

	templateRoot := filepath.Join(request.JobRoot, "templates")
	buildRoot := filepath.Join(request.JobRoot, "build-cache")
	request.TemplateRoot = templateRoot
	request.CacheScope = request.TemplateID + "-" + request.BuildID
	directories := []string{
		request.JobRoot,
		templateRoot,
		buildRoot,
		filepath.Join(request.JobRoot, "orchestrator", "build"),
		filepath.Join(request.JobRoot, "orchestrator", "build-templates"),
		filepath.Join(request.JobRoot, "sandbox-cache"),
		filepath.Join(request.JobRoot, "snapshot-cache"),
		filepath.Join(request.JobRoot, "template-cache"),
	}
	for _, directory := range directories {
		if err := os.MkdirAll(directory, 0o755); err != nil {
			return JobResult{}, fmt.Errorf("create job directory %s: %w", directory, err)
		}
	}
	runtime.Logger.Debug(ctx, "构建 job 目录准备完成", zap.String("stage", "build_job_directories_ready"), zap.String("build_id", request.BuildID), zap.Int("directory_count", len(directories)))

	view, err := prepareInfra202609RuntimeView(request.JobRoot, request.RuntimeRoot, request.Architecture, request.KernelVersion, request.FirecrackerVersion)
	if err != nil {
		return JobResult{}, fmt.Errorf("prepare infra 2026.09 runtime view: %w", err)
	}
	templateStorage, err := newJobLocalStorage(templateRoot)
	if err != nil {
		return JobResult{}, fmt.Errorf("create template storage: %w", err)
	}
	buildStorage, err := newJobLocalStorage(buildRoot)
	if err != nil {
		return JobResult{}, fmt.Errorf("create build-cache storage: %w", err)
	}
	runtime.Logger.Info(ctx, "构建 job 本地 storage 准备完成", zap.String("stage", "build_job_storage_ready"), zap.String("build_id", request.BuildID))

	builderConfig := builderConfigForJob(request, runtime.NetworkConfig, view)
	config := cfg.Config{BuilderConfig: builderConfig}
	blockMetrics, err := blockmetrics.NewMetrics(noop.NewMeterProvider())
	if err != nil {
		return JobResult{}, fmt.Errorf("create block metrics: %w", err)
	}
	templateCache, err := sbxtemplate.NewCache(config, runtime.FeatureFlags, templateStorage, blockMetrics)
	if err != nil {
		return JobResult{}, fmt.Errorf("create template cache: %w", err)
	}
	templateCache.Start(ctx)
	defer templateCache.Stop()

	artifactRegistry, err := artifactsregistry.NewLocalArtifactsRegistry()
	if err != nil {
		return JobResult{}, fmt.Errorf("create local artifact registry: %w", err)
	}
	metrics, err := buildmetrics.NewBuildMetrics(noop.NewMeterProvider())
	if err != nil {
		return JobResult{}, fmt.Errorf("create build metrics: %w", err)
	}
	// 2026.09 Factory 的 hoststats/cgroup 参数允许 nil；与 upstream create-build 一致。
	factory := sandbox.NewFactory(builderConfig, runtime.NetworkPool, runtime.DevicePool, runtime.FeatureFlags, nil, nil)
	builder := build.NewBuilder(
		builderConfig,
		runtime.Logger,
		runtime.FeatureFlags,
		factory,
		templateStorage,
		buildStorage,
		artifactRegistry,
		dockerhub.NewNoopRemoteRepository(),
		runtime.SandboxProxy,
		runtime.Sandboxes,
		templateCache,
		metrics,
	)
	runtime.Logger.Info(ctx, "构建 job 依赖装配完成", zap.String("stage", "build_job_dependencies_ready"), zap.String("build_id", request.BuildID))

	return executeBuilder(ctx, builder, request, runtime.Logger.Detach(ctx).Core())
}

// JobResult 保持 TemplateCreator 稳定 contract；2026.09 Builder 只直接返回
// envd 与 rootfs size，kernel/Firecracker 来自已经通过 preflight 的请求 profile。
type JobResult struct {
	EnvdVersion        string
	KernelVersion      string
	FirecrackerVersion string
	RootfsSizeMB       int64
	TemplateRoot       string
}

type jobBuilder interface {
	Build(context.Context, upstreamstorage.TemplateFiles, buildconfig.TemplateConfig, zapcore.Core) (*build.Result, error)
}

var _ jobBuilder = (*build.Builder)(nil)

// executeBuilder 将稳定 JobRequest 转换为 infra 2026.09 TemplateConfig。
func executeBuilder(ctx context.Context, builder jobBuilder, request JobRequest, logsCore zapcore.Core) (JobResult, error) {
	if request.UseBaseCache {
		return JobResult{}, fmt.Errorf("shared base cache is not supported by the infra 2026.09 runtime adapter")
	}
	force := true
	config := buildconfig.TemplateConfig{
		Version:              request.TemplateVersion,
		TeamID:               "local",
		TemplateID:           request.TemplateID,
		CacheScope:           request.TemplateID + "-" + request.BuildID,
		StartCmd:             request.StartCommand,
		VCpuCount:            request.VCPU,
		MemoryMB:             request.MemoryMB,
		DiskSizeMB:           request.DiskSizeMB,
		HugePages:            request.HugePages,
		ReadyCmd:             request.ReadyCommand,
		FromImage:            request.ImageRef,
		ImageTarPath:         request.ImageTarPath,
		Architecture:         request.Architecture,
		RegistryAuthProvider: request.RegistryAuth,
		Force:                &force,
		Steps:                convertJobSteps(request.Steps),
		FirecrackerVersion:   request.FirecrackerVersion,
		KernelVersion:        request.KernelVersion,
	}

	stageLogger := zap.New(logsCore) //nolint:forbidigo // 与 upstream Builder 共用同一个 zap core
	stageLogger.Info("调用 upstream Builder.Build", zap.String("stage", "upstream_builder_call_started"), zap.String("template_id", request.TemplateID), zap.String("build_id", request.BuildID))
	result, err := builder.Build(ctx, upstreamstorage.TemplateFiles{BuildID: request.BuildID}, config, logsCore)
	if err != nil {
		stageLogger.Error("upstream Builder.Build 返回失败", zap.String("stage", "upstream_builder_call_failed"), zap.String("template_id", request.TemplateID), zap.String("build_id", request.BuildID), zap.Error(err))
		return JobResult{}, fmt.Errorf("real Builder.Build: %w", err)
	}
	if result == nil {
		return JobResult{}, fmt.Errorf("real Builder.Build returned nil result")
	}
	stageLogger.Info("upstream Builder.Build 返回成功", zap.String("stage", "upstream_builder_call_completed"), zap.String("template_id", request.TemplateID), zap.String("build_id", request.BuildID))

	return JobResult{
		EnvdVersion:        result.EnvdVersion,
		KernelVersion:      request.KernelVersion,
		FirecrackerVersion: request.FirecrackerVersion,
		RootfsSizeMB:       result.RootfsSizeMB,
		TemplateRoot:       request.TemplateRoot,
	}, nil
}

func convertJobSteps(steps []JobStep) []*templatemanager.TemplateStep {
	result := make([]*templatemanager.TemplateStep, len(steps))
	for index, step := range steps {
		result[index] = &templatemanager.TemplateStep{Type: step.Type, Args: append([]string(nil), step.Args...)}
	}
	return result
}
