//go:build linux

package templatecreator

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"go.uber.org/zap"

	buildruntime "gitcode.com/boostkit/cloud-native.git/TemplateCreator/internal/runtime"
	registryauth "gitcode.com/boostkit/cloud-native.git/TemplateCreator/internal/upstream/orchestrator/pkg/template/build/core/oci/auth"
	templatemanager "gitcode.com/boostkit/cloud-native.git/TemplateCreator/internal/upstream/shared/pkg/grpc/template-manager"
)

// jobRuntime 是 Creator 对单次真实构建所需的最小 runtime contract。
type jobRuntime interface {
	BuildJob(context.Context, buildruntime.JobRequest) (buildruntime.JobResult, error)
}

// buildOutcome 在 Builder 成功后把发布阶段所需的 job 位置和可审计 metadata 交回 Creator。
type buildOutcome struct {
	TemplateID string
	BuildID    string
	JobRoot    string
	// TemplateRoot 是 Builder 实际写入 final/ancestor 对象的权威目录。
	TemplateRoot string
	Metadata     BuildMetadata
}

// buildTemplate 验证单次请求，将稳定公共类型适配为私有 Builder contract，并执行一个隔离 job。
func (creator *Creator) buildTemplate(ctx context.Context, request CreateRequest, templateID, buildID string) (buildOutcome, error) {
	if err := validateCreateRequest(request); err != nil {
		return buildOutcome{}, err
	}
	spec, format, publication, err := resolveBuildOptions(creator.config.Defaults, creator.config.Format, creator.config.Publication, request.BuildOverride, request.Format, request.Publication)
	if err != nil {
		return buildOutcome{}, fmt.Errorf("%w: BuildSpec: %v", ErrInvalidRequest, err)
	}
	var authProvider registryauth.RegistryAuthProvider
	if request.RegistryAuth != nil && strings.TrimSpace(request.ImageRef) != "" {
		// credential 仅为本次构建解析并保存在内存中，不进入 metadata 或任务内容摘要。
		credentials, err := request.RegistryAuth.Credentials(ctx, request.ImageRef)
		if err != nil {
			return buildOutcome{}, fmt.Errorf("resolve registry credentials: %w", err)
		}
		authProvider = registryauth.NewAuthProvider(&templatemanager.FromImageRegistry{Type: &templatemanager.FromImageRegistry_General{General: &templatemanager.GeneralRegistry{Username: credentials.Username, Password: credentials.Password}}})
	}
	steps := make([]buildruntime.JobStep, len(spec.Steps))
	for index, step := range spec.Steps {
		steps[index] = buildruntime.JobStep{Type: step.Type, Args: append([]string(nil), step.Args...)}
	}
	jobRoot := filepath.Join(creator.config.ScratchRoot, "jobs", buildID)
	runtime, ok := creator.runtime.(jobRuntime)
	if !ok {
		return buildOutcome{}, fmt.Errorf("creator runtime does not support template builds")
	}
	// 这里是稳定公共类型到私有 upstream Builder contract 的 adapter boundary；
	// upstream 类型不会越过该边界暴露给 module 调用方。
	creator.logger.Info(ctx, "真实 Builder 构建开始",
		zap.String("stage", "builder_started"), zap.String("template_id", templateID), zap.String("build_id", buildID), zap.String("image", request.ImageRef))
	result, err := runtime.BuildJob(ctx, buildruntime.JobRequest{
		TemplateID: templateID, BuildID: buildID, ImageRef: request.ImageRef, ImageTarPath: request.ImageTarPath, JobRoot: jobRoot,
		BypassBaseCache: request.BypassBaseCache,
		RuntimeRoot:     creator.config.Runtime.RuntimeDir, EnvdPath: creator.config.Runtime.EnvdPath,
		BusyboxDir: creator.config.Runtime.BusyboxDir, BusyboxVersion: creator.config.Runtime.BusyboxVersion,
		BusyboxSHA256:   creator.config.Runtime.BusyboxSHA256,
		TemplateVersion: creator.config.Runtime.TemplateVersion, Architecture: creator.config.Runtime.Architecture,
		KernelVersion:      creator.config.Runtime.KernelVersion,
		FirecrackerVersion: creator.config.Runtime.FirecrackerVersion, VCPU: spec.VCPU, MemoryMB: spec.MemoryMB,
		DiskSizeMB: spec.DiskSizeMB, HugePages: spec.HugePages, StartCommand: spec.StartCommand,
		ReadyCommand: spec.ReadyCommand, Steps: steps, RegistryAuth: authProvider,
	})
	if err != nil {
		creator.logger.Error(ctx, "真实 Builder 构建失败",
			zap.String("stage", "builder_failed"), zap.String("template_id", templateID), zap.String("build_id", buildID), zap.Error(err))
		return buildOutcome{}, &BuildError{TemplateID: templateID, BuildID: buildID, Phase: BuildPhaseBuild, Cause: err}
	}
	creator.logger.Info(ctx, "真实 Builder 构建完成",
		zap.String("stage", "builder_completed"), zap.String("template_id", templateID), zap.String("build_id", buildID), zap.Int64("rootfs_size_mb", result.RootfsSizeMB))
	if !filepath.IsAbs(result.TemplateRoot) || !creator.templateRootOwned(result.TemplateRoot, jobRoot) {
		return buildOutcome{}, &BuildError{
			TemplateID: templateID, BuildID: buildID, Phase: BuildPhaseBuild,
			Cause: fmt.Errorf("runtime returned TemplateRoot outside Creator-owned storage: %q", result.TemplateRoot),
		}
	}
	// snapshot restore 依赖该 ABI 组合；真实 Builder 报告的版本只要与固定 runtime
	// 有一项不一致，就拒绝进入发布阶段，避免产生“构建成功但无法恢复”的产物。
	if result.EnvdVersion != creator.config.Runtime.EnvdVersion || result.KernelVersion != creator.config.Runtime.KernelVersion || result.FirecrackerVersion != creator.config.Runtime.FirecrackerVersion {
		creator.logger.Error(ctx, "runtime ABI 校验失败", zap.String("stage", "runtime_abi_validation_failed"), zap.String("template_id", templateID), zap.String("build_id", buildID))
		return buildOutcome{}, &BuildError{
			TemplateID: templateID, BuildID: buildID, Phase: BuildPhaseBuild,
			Cause: fmt.Errorf("runtime ABI mismatch: envd=%q kernel=%q firecracker=%q, want envd=%q kernel=%q firecracker=%q",
				result.EnvdVersion, result.KernelVersion, result.FirecrackerVersion,
				creator.config.Runtime.EnvdVersion, creator.config.Runtime.KernelVersion, creator.config.Runtime.FirecrackerVersion),
		}
	}
	creator.logger.Info(ctx, "runtime ABI 校验通过", zap.String("stage", "runtime_abi_validated"), zap.String("template_id", templateID), zap.String("build_id", buildID))
	return buildOutcome{TemplateID: templateID, BuildID: buildID, JobRoot: jobRoot, TemplateRoot: result.TemplateRoot, Metadata: BuildMetadata{
		InfraCommit: creator.config.Runtime.InfraCommit, TemplateVersion: creator.config.Runtime.TemplateVersion,
		Format: format, Publication: publication,
		Architecture: creator.config.Runtime.Architecture, EnvdVersion: result.EnvdVersion,
		KernelVersion: result.KernelVersion, FirecrackerVersion: result.FirecrackerVersion,
		RootfsSizeMB: result.RootfsSizeMB, Resources: spec,
		Scheduling: SchedulingMetadata{VCPU: spec.VCPU, MemoryMB: spec.MemoryMB, DiskSizeMB: spec.DiskSizeMB},
		MachineInfo: MachineInfo{
			CPUArchitecture: creator.machineInfo.CPUArchitecture,
			CPUFamily:       creator.machineInfo.CPUFamily,
			CPUModel:        creator.machineInfo.CPUModel,
			CPUModelName:    creator.machineInfo.CPUModelName,
			CPUFlags:        append([]string{}, creator.machineInfo.CPUFlags...),
		},
	}}, nil
}

// templateRootOwned 防止 runtime contract 异常时把任意绝对路径交给发布器。
// job-local root 必须是当前 job 的 templates 子目录；共享 root 只能位于配置的 CacheRoot 下。
func (creator *Creator) templateRootOwned(root, jobRoot string) bool {
	cleanRoot := filepath.Clean(root)
	if cleanRoot == filepath.Join(filepath.Clean(jobRoot), "templates") {
		return true
	}
	if !creator.config.Cache.Enabled || creator.config.Cache.Root == "" {
		return false
	}
	relative, err := filepath.Rel(filepath.Clean(creator.config.Cache.Root), cleanRoot)
	return err == nil && relative != "." && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}
