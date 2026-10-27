package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"go.uber.org/zap"

	templatecreator "gitcode.com/cloud-native/cloud-native/TemplateCreator"
	"gitcode.com/cloud-native/cloud-native/TemplateCreator/internal/preflight"
	buildruntime "gitcode.com/cloud-native/cloud-native/TemplateCreator/internal/runtime"
)

const (
	defaultBuildTimeout = templatecreator.DefaultBuildTimeout
	defaultVCPU         = templatecreator.DefaultVCPU
	defaultMemoryMB     = templatecreator.DefaultMemoryMB
	defaultDiskSizeMB   = templatecreator.DefaultDiskSizeMB
	defaultMaxParallel  = templatecreator.DefaultMaxParallel
	defaultProxyPort    = uint(templatecreator.DefaultProxyPort)
	defaultLogLevel     = string(templatecreator.DefaultLogLevel)
	logLevelOff         = "off"
	logLevelInfo        = "info"
	logLevelDebug       = "debug"
)

// commandUsage 是面向初学者的完整帮助页；默认值复用实际 flag 常量，避免说明漂移。
var commandUsage = fmt.Sprintf(`TemplateCreator 模板构建演示命令

用途：
  使用 prepare-runtime 生成的运行时资产，从 OCI 镜像构建可供 E2B 使用的 Template。

用法：
  templatecreator-demo [参数]

必填参数：
  -runtime-info <路径>  prepare-runtime 输出的 runtime-info.json 绝对路径
  -scratch-root <目录>  存放临时构建数据的现有可写目录
  -store-root <目录>    存放最终 Template 对象的现有可写目录
  -image <镜像>         作为 Template 基础的 OCI 镜像引用，与 -image-tar 互斥
  -image-tar <文件>     作为 Template 基础的 docker save 镜像 tar 文件，与 -image 互斥

可选参数：
  -export-root <目录>    原始 Template 导出根目录；普通模式默认使用 <store-root>/exports，简化模式默认使用 <work-dir>
  -cache-root <目录>     兼容保留；infra 2026.09 不支持持久 base-layer cache，设置后返回错误
  -idempotency-key <值>  重试同一任务时使用的任务编号
  -vcpu <数量>           guest 虚拟 CPU 数量（默认 %d）
  -memory-mb <MiB>       guest 内存（默认 %d MiB）
  -disk-size-mb <MiB>    guest 根文件系统大小（默认 %d MiB）
  -hugepages             使用 host HugePages 承载 guest 内存
  -start-command <命令>  由 envd 启动的命令
  -ready-command <命令>  检查 guest 就绪状态的命令
	  -format <格式>         fromImage 或 fromDockerfile（默认 %s）
	  -publication <模式>    simple 或 all（默认 %s）
  -timeout <时长>        整体构建超时（默认 %dm）
  -max-parallel <数量>   每个 Creator 的最大物理构建并发数（默认 %d）
  -proxy-port <端口>     本地 sandbox proxy 端口（默认 %d）
  -log-level <级别>      运行日志级别（默认 %s）：off、info 或 debug
  -disable-base-cache   兼容保留；infra 2026.09 默认不启用持久 cache
  -allow-mutable-image-tag-cache 兼容保留；infra 2026.09 不支持
  -bypass-base-cache    兼容保留；infra 2026.09 不支持
  -preflight-only        只执行只读环境检查，不初始化 Creator
  -h, --help             显示帮助信息

示例：
  sudo templatecreator-demo \
    -runtime-info /opt/templatecreator/runtime-info.json \
    -scratch-root /opt/templatecreator/scratch \
    -store-root /opt/templatecreator/store \
    -image docker.io/library/debian:bookworm-slim

简化模式：
  sudo templatecreator-demo -image-id redis:e2b-x86 -work-dir /opt/templatecreator
  该模式从 /opt/templatecreator/redis:e2b-x86.tar 读取镜像，自动使用同目录下的
  runtime-info.json，把 scratch、store 放到 work-dir 子目录，并默认直接导出到 work-dir。

结果：
  成功后向标准输出写入 JSON，并在 stderr 提示 registration.sql 路径和执行前检查项。
  构建成功后默认导出原始 Template 产物；输出结构为 <导出根>/<TemplateID>/<BuildID>/。
  可通过 -export-root 指定导出根目录，导出目录写入 stderr。
`, defaultVCPU, defaultMemoryMB, defaultDiskSizeMB, templatecreator.DefaultBuildFormat, templatecreator.DefaultPublicationMode, defaultBuildTimeout/time.Minute, defaultMaxParallel, defaultProxyPort, defaultLogLevel)

// options 汇总 CLI 参数，并在进入 TemplateCreator 公共 API 前完成基础校验。
type options struct {
	// RuntimeInfoPath/ScratchRoot/StoreRoot 必须是绝对路径。
	RuntimeInfoPath string
	ScratchRoot     string
	StoreRoot       string
	CacheRoot       string
	ExportRoot      string
	ImageID         string
	WorkDir         string
	StepsPath       string
	Format          string
	Publication     string
	// ImageRef/ImageTarPath 描述互斥的基础镜像输入。
	ImageRef     string
	ImageTarPath string
	// IdempotencyKey 描述本次构建身份与重试语义。
	IdempotencyKey string
	// Build/Timeout/MaxParallel/ProxyPort 控制资源与进程级 runtime。
	Build       templatecreator.BuildSpec
	Timeout     time.Duration
	MaxParallel int
	ProxyPort   uint16
	// LogLevel 控制写入 stderr 的运行进度；off 不影响最终错误输出。
	LogLevel string
	// Cache flags 为稳定 CLI contract 保留；infra 2026.09 adapter 会拒绝启用共享 cache。
	DisableBaseCache          bool
	AllowMutableImageTagCache bool
	BypassBaseCache           bool
	// PreflightOnly 使命令在只读检查后停止，不持有 NBD/network 等资源。
	PreflightOnly bool
}

// runtimeInfo 是 prepare-runtime stdout JSON 的严格输入 schema。
// 该文件属于外部输入，不能因为由配套命令生成就跳过路径与字段校验。
type runtimeInfo struct {
	Architecture    string             `json:"architecture"`
	InfraCommit     string             `json:"infra_commit"`
	TemplateVersion string             `json:"template_version"`
	RuntimeRoot     string             `json:"runtime_root"`
	Assets          []runtimeInfoAsset `json:"assets"`
}

// runtimeInfoAsset 是 runtime-info 中单个已验证资产的必要字段。
type runtimeInfoAsset struct {
	Type    string `json:"type"`
	Version string `json:"version"`
	Path    string `json:"path"`
	SHA256  string `json:"sha256,omitempty"`
	Source  string `json:"source,omitempty"`
}

// runtimePaths 保存按 Builder 固定布局重建后的文件路径和 checksum。
type runtimePaths struct {
	Kernel            string
	Firecracker       string
	Envd              string
	Busybox           string
	KernelSHA256      string
	FirecrackerSHA256 string
	EnvdSHA256        string
	BusyboxSHA256     string
}

// commandOutput 是 Demo 唯一写入 stdout 的 JSON contract。
type commandOutput struct {
	Ready      bool                          `json:"ready"`
	Checks     []preflight.Check             `json:"checks"`
	Result     *templatecreator.CreateResult `json:"result,omitempty"`
	TemplateID string                        `json:"template_id,omitempty"`
	FinalID    string                        `json:"final_id,omitempty"`
}

// demoCreator 把 CLI 与具体 Creator 实现解耦，使 help/preflight 测试无需初始化特权资源。
type demoCreator interface {
	CreateTemplate(context.Context, templatecreator.CreateRequest) (templatecreator.CreateResult, error)
	Close(context.Context) error
}

// dependencies 集中保存有副作用的入口，保证只读路径可在测试中被证明不会越界调用。
type dependencies struct {
	preflight       func(preflight.Config) preflight.Report
	newCreator      func(context.Context, templatecreator.CreatorConfig) (demoCreator, error)
	exportArtifacts func(context.Context, templatecreator.ExportTemplateArtifactsRequest) (templatecreator.ExportTemplateArtifactsResult, error)
	// logOutput 与 stdout 分离；生产入口固定为 stderr，测试可注入独立 buffer。
	logOutput io.Writer
}

func main() {
	log.SetFlags(0)
	deps := dependencies{
		preflight: preflight.Run,
		newCreator: func(ctx context.Context, config templatecreator.CreatorConfig) (demoCreator, error) {
			return templatecreator.NewCreator(ctx, config)
		},
		exportArtifacts: templatecreator.ExportTemplateArtifacts,
		logOutput:       os.Stderr,
	}
	if err := run(context.Background(), os.Args[1:], os.Stdout, deps); err != nil {
		log.Fatal(err)
	}
}

// parseFlags 保留简洁的内部调用接口；CLI 入口通过 parseFlagsWithHelp 获取帮助文本。
func parseFlags(arguments []string) (options, error) {
	result, _, err := parseFlagsWithHelp(arguments)
	return result, err
}

// parseFlagsWithHelp 将 FlagSet 的诊断和帮助暂存到内存，避免普通参数错误污染标准输出。
func parseFlagsWithHelp(arguments []string) (options, string, error) {
	flags := flag.NewFlagSet("templatecreator-demo", flag.ContinueOnError)
	var flagOutput bytes.Buffer
	flags.SetOutput(&flagOutput)
	flags.Usage = func() {
		_, _ = io.WriteString(flags.Output(), commandUsage)
	}
	result := options{
		RuntimeInfoPath: templatecreator.DefaultRuntimeInfoPath,
		ScratchRoot:     templatecreator.DefaultScratchRoot,
		StoreRoot:       templatecreator.DefaultStoreRoot,
		ImageRef:        templatecreator.DefaultImageRef,
		Build:           templatecreator.DefaultBuildSpec(),
		Timeout:         defaultBuildTimeout, MaxParallel: defaultMaxParallel, LogLevel: defaultLogLevel,
		Format: string(templatecreator.DefaultBuildFormat), Publication: string(templatecreator.DefaultPublicationMode),
	}
	var proxyPort uint
	flags.StringVar(&result.RuntimeInfoPath, "runtime-info", templatecreator.DefaultRuntimeInfoPath, "prepare-runtime 输出的 JSON 绝对路径")
	flags.StringVar(&result.ScratchRoot, "scratch-root", templatecreator.DefaultScratchRoot, "存放临时构建数据的可写目录")
	flags.StringVar(&result.StoreRoot, "store-root", templatecreator.DefaultStoreRoot, "存放最终 Template 对象的可写目录")
	flags.StringVar(&result.CacheRoot, "cache-root", "", "兼容保留；infra 2026.09 不支持持久 base-layer cache")
	flags.StringVar(&result.ExportRoot, "export-root", "", "原始 Template 导出根目录；普通模式默认使用 <store-root>/exports，简化模式默认使用 <work-dir>")
	flags.StringVar(&result.ImageID, "image-id", "", "简化模式镜像 ID；对应 <work-dir>/<image-id>.tar")
	flags.StringVar(&result.WorkDir, "work-dir", "", "简化模式工作目录；自动推导 runtime、scratch、store 和默认导出目录")
	flags.StringVar(&result.StepsPath, "steps", "", "可选 steps JSON 文件路径")
	flags.StringVar(&result.ImageRef, "image", templatecreator.DefaultImageRef, "作为 Template 基础的 OCI 镜像引用")
	flags.StringVar(&result.ImageTarPath, "image-tar", "", "作为 Template 基础的 docker save 镜像 tar 文件")
	flags.StringVar(&result.Format, "format", string(templatecreator.DefaultBuildFormat), "构建格式：fromImage 或 fromDockerfile")
	flags.StringVar(&result.Publication, "publication", string(templatecreator.DefaultPublicationMode), "发布模式：simple 或 all")
	flags.StringVar(&result.IdempotencyKey, "idempotency-key", "", "重试同一任务时使用的可选任务编号")
	flags.Int64Var(&result.Build.VCPU, "vcpu", defaultVCPU, "guest 虚拟 CPU 数量")
	flags.Int64Var(&result.Build.MemoryMB, "memory-mb", defaultMemoryMB, "guest 内存，单位 MiB")
	flags.Int64Var(&result.Build.DiskSizeMB, "disk-size-mb", defaultDiskSizeMB, "guest 根文件系统大小，单位 MiB")
	flags.BoolVar(&result.Build.HugePages, "hugepages", false, "使用 host HugePages 承载 guest 内存")
	flags.StringVar(&result.Build.StartCommand, "start-command", "", "由 envd 启动的可选命令")
	flags.StringVar(&result.Build.ReadyCommand, "ready-command", "", "可选的就绪检查命令")
	flags.DurationVar(&result.Timeout, "timeout", defaultBuildTimeout, "整体构建超时")
	flags.IntVar(&result.MaxParallel, "max-parallel", defaultMaxParallel, "每个 Creator 的最大物理构建并发数")
	flags.UintVar(&proxyPort, "proxy-port", defaultProxyPort, "本地 sandbox proxy 端口")
	flags.StringVar(&result.LogLevel, "log-level", defaultLogLevel, "运行日志级别：off、info 或 debug")
	flags.BoolVar(&result.DisableBaseCache, "disable-base-cache", false, "兼容保留；infra 2026.09 默认不启用持久 cache")
	flags.BoolVar(&result.AllowMutableImageTagCache, "allow-mutable-image-tag-cache", false, "兼容保留；infra 2026.09 不支持")
	flags.BoolVar(&result.BypassBaseCache, "bypass-base-cache", false, "兼容保留；infra 2026.09 不支持")
	flags.BoolVar(&result.PreflightOnly, "preflight-only", false, "只执行只读检查，不初始化 Creator")
	if err := flags.Parse(arguments); err != nil {
		return options{}, flagOutput.String(), fmt.Errorf("parse arguments: %w", err)
	}
	if flags.NArg() != 0 {
		return options{}, "", fmt.Errorf("positional arguments are not supported: %v", flags.Args())
	}
	imageFlagSet, imageTarFlagSet := false, false
	flags.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "image":
			imageFlagSet = true
		case "image-tar":
			imageTarFlagSet = true
		}
	})
	if imageTarFlagSet && !imageFlagSet {
		result.ImageRef = ""
	}
	if result.ImageID != "" || result.WorkDir != "" {
		if result.ImageID == "" || result.WorkDir == "" {
			return options{}, "", fmt.Errorf("-image-id and -work-dir must be provided together")
		}
		result.RuntimeInfoPath = filepath.Join(result.WorkDir, "runtime-info.json")
		result.ScratchRoot = filepath.Join(result.WorkDir, "scratch")
		result.StoreRoot = filepath.Join(result.WorkDir, "store")
		// 简化模式默认直接导出到 work-dir；显式 -export-root 始终优先。
		if result.ExportRoot == "" {
			result.ExportRoot = result.WorkDir
		}
		result.ImageTarPath = filepath.Join(result.WorkDir, result.ImageID+".tar")
		result.ImageRef = ""
	}
	if result.ImageRef == "" && result.ImageTarPath == "" {
		result.ImageRef = templatecreator.DefaultImageRef
	}
	if result.ImageRef != "" && result.ImageTarPath != "" {
		return options{}, "", fmt.Errorf("-image and -image-tar are mutually exclusive")
	}
	if result.Timeout <= 0 {
		return options{}, "", fmt.Errorf("-timeout must be positive")
	}
	if result.MaxParallel <= 0 {
		return options{}, "", fmt.Errorf("-max-parallel must be positive")
	}
	if proxyPort == 0 || proxyPort > 65535 {
		return options{}, "", fmt.Errorf("-proxy-port must be between 1 and 65535")
	}
	switch result.LogLevel {
	case logLevelOff, logLevelInfo, logLevelDebug:
	default:
		return options{}, "", fmt.Errorf("-log-level must be one of off, info, or debug")
	}
	if result.Format != string(templatecreator.BuildFormatFromImage) && result.Format != string(templatecreator.BuildFormatFromDockerfile) {
		return options{}, "", fmt.Errorf("-format must be fromImage or fromDockerfile")
	}
	if result.Publication != string(templatecreator.PublicationSimple) && result.Publication != string(templatecreator.PublicationAll) {
		return options{}, "", fmt.Errorf("-publication must be simple or all")
	}
	if result.CacheRoot != "" && result.DisableBaseCache {
		return options{}, "", fmt.Errorf("-cache-root cannot be combined with -disable-base-cache")
	}
	if result.CacheRoot == "" && result.AllowMutableImageTagCache {
		return options{}, "", fmt.Errorf("-allow-mutable-image-tag-cache requires -cache-root")
	}
	if result.CacheRoot == "" && result.BypassBaseCache {
		return options{}, "", fmt.Errorf("-bypass-base-cache requires -cache-root")
	}
	result.ProxyPort = uint16(proxyPort)
	for name, path := range map[string]string{"-runtime-info": result.RuntimeInfoPath, "-scratch-root": result.ScratchRoot, "-store-root": result.StoreRoot, "-cache-root": result.CacheRoot, "-export-root": result.ExportRoot, "-image-tar": result.ImageTarPath, "-steps": result.StepsPath} {
		if path == "" {
			continue
		}
		if !filepath.IsAbs(path) {
			return options{}, "", fmt.Errorf("%s must be an absolute path", name)
		}
	}
	if result.CacheRoot != "" {
		return options{}, "", fmt.Errorf("-cache-root is not supported by the infra 2026.09 runtime adapter")
	}
	return result, "", nil
}

// loadRuntimeInfo 严格解析 runtime-info，并重建 Builder 预期路径，拒绝任意 JSON 路径注入。
func loadRuntimeInfo(path string) (templatecreator.RuntimeConfig, runtimePaths, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return templatecreator.RuntimeConfig{}, runtimePaths{}, fmt.Errorf("read runtime info: %w", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(content))
	decoder.DisallowUnknownFields()
	var info runtimeInfo
	if err := decoder.Decode(&info); err != nil {
		return templatecreator.RuntimeConfig{}, runtimePaths{}, fmt.Errorf("decode runtime info: %w", err)
	}
	if err := requireEOF(decoder); err != nil {
		return templatecreator.RuntimeConfig{}, runtimePaths{}, err
	}
	if !filepath.IsAbs(info.RuntimeRoot) || info.Architecture == "" || info.InfraCommit == "" || info.TemplateVersion == "" {
		return templatecreator.RuntimeConfig{}, runtimePaths{}, fmt.Errorf("runtime info requires an absolute runtime_root, architecture, infra_commit, and template_version")
	}
	assets := make(map[string]runtimeInfoAsset, len(info.Assets))
	for _, asset := range info.Assets {
		if _, exists := assets[asset.Type]; exists {
			return templatecreator.RuntimeConfig{}, runtimePaths{}, fmt.Errorf("runtime info contains duplicate %s asset", asset.Type)
		}
		assets[asset.Type] = asset
	}
	for _, required := range []string{"kernel", "firecracker", "envd", "busybox"} {
		asset, exists := assets[required]
		if !exists || asset.Version == "" || !filepath.IsAbs(asset.Path) {
			return templatecreator.RuntimeConfig{}, runtimePaths{}, fmt.Errorf("runtime info requires %s asset with version and absolute path", required)
		}
	}
	// 不直接接受 JSON 中的任意路径：按照 RuntimeRoot、version 和 architecture
	// 重建精确 Builder 布局，并要求 prepare-runtime 输出与之完全一致。
	expected := map[string]string{
		"kernel":      filepath.Join(info.RuntimeRoot, "kernels", assets["kernel"].Version, info.Architecture, "vmlinux.bin"),
		"firecracker": filepath.Join(info.RuntimeRoot, "firecrackers", assets["firecracker"].Version, info.Architecture, "firecracker"),
		"envd":        filepath.Join(info.RuntimeRoot, "envd", assets["envd"].Version, info.Architecture, "envd"),
		"busybox":     filepath.Join(info.RuntimeRoot, "busybox", assets["busybox"].Version, info.Architecture, "busybox"),
	}
	for assetType, expectedPath := range expected {
		if filepath.Clean(assets[assetType].Path) != expectedPath {
			return templatecreator.RuntimeConfig{}, runtimePaths{}, fmt.Errorf("runtime info %s path %q does not match Builder layout %q", assetType, assets[assetType].Path, expectedPath)
		}
	}
	paths := runtimePaths{
		Kernel: expected["kernel"], Firecracker: expected["firecracker"], Envd: expected["envd"], Busybox: expected["busybox"],
		KernelSHA256: assets["kernel"].SHA256, FirecrackerSHA256: assets["firecracker"].SHA256,
		EnvdSHA256: assets["envd"].SHA256, BusyboxSHA256: assets["busybox"].SHA256,
	}
	config := templatecreator.RuntimeConfig{
		RuntimeDir: info.RuntimeRoot, InfraCommit: info.InfraCommit, TemplateVersion: info.TemplateVersion, Architecture: info.Architecture,
		KernelVersion: assets["kernel"].Version, KernelSHA256: paths.KernelSHA256,
		FirecrackerVersion: assets["firecracker"].Version, FirecrackerSHA256: paths.FirecrackerSHA256,
		EnvdVersion: assets["envd"].Version, EnvdPath: paths.Envd, EnvdSHA256: paths.EnvdSHA256,
		BusyboxDir: filepath.Join(info.RuntimeRoot, "busybox"), BusyboxVersion: assets["busybox"].Version, BusyboxSHA256: paths.BusyboxSHA256,
	}
	return config, paths, nil
}

// requireEOF 保证 runtime-info 只包含一个 JSON value，拒绝尾随或拼接数据。
func requireEOF(decoder *json.Decoder) error {
	var trailing any
	if err := decoder.Decode(&trailing); err == io.EOF {
		return nil
	} else if err != nil {
		return fmt.Errorf("decode trailing runtime info data: %w", err)
	}
	return fmt.Errorf("runtime info contains multiple JSON values")
}

// run 编排 help、参数校验、preflight、真实构建和 JSON 输出。
func run(ctx context.Context, arguments []string, output io.Writer, deps dependencies) error {
	options, help, err := parseFlagsWithHelp(arguments)
	if errors.Is(err, flag.ErrHelp) {
		// help 是正常退出路径；仅在此处转发暂存文本，并原样返回底层写入错误。
		_, writeErr := io.WriteString(output, help)
		return writeErr
	}
	if err != nil {
		return err
	}
	if options.StepsPath != "" {
		steps, stepsErr := loadSteps(options.StepsPath)
		if stepsErr != nil {
			return stepsErr
		}
		options.Build.Steps = steps
	}
	logWriter := deps.logOutput
	// resultNoticeWriter 不受 -log-level 控制；即使关闭进度日志，注册 SQL
	// 提示和显式 bundle 导出路径仍必须展示。生产入口将该 writer 固定为 stderr。
	resultNoticeWriter := deps.logOutput
	if options.LogLevel == logLevelOff {
		logWriter = nil
	}
	progressLogger, err := buildruntime.NewLogger(logWriter, options.LogLevel == logLevelDebug)
	if err != nil {
		return fmt.Errorf("initialize CLI logger: %w", err)
	}
	progressLogger.Info(ctx, "CLI 参数解析完成", zap.String("stage", "arguments_parsed"), zap.String("log_level", options.LogLevel))
	runtimeConfig, paths, err := loadRuntimeInfo(options.RuntimeInfoPath)
	if err != nil {
		progressLogger.Error(ctx, "runtime-info 加载失败", zap.String("stage", "runtime_info_load_failed"), zap.Error(err))
		return err
	}
	progressLogger.Info(ctx, "runtime-info 加载完成", zap.String("stage", "runtime_info_loaded"), zap.String("architecture", runtimeConfig.Architecture))
	runtimeConfig.ProxyPort = options.ProxyPort
	// Preflight 必须先于 NewCreator：运行时初始化会启动 proxy/firewall goroutine，
	// 并打开需要特权的 NBD 与网络资源。
	progressLogger.Info(ctx, "只读 preflight 检查开始", zap.String("stage", "preflight_started"))
	report := deps.preflight(preflight.Config{
		GOOS: runtime.GOOS, GOARCH: runtime.GOARCH, Architecture: runtimeConfig.Architecture,
		ImageRef: options.ImageRef, ImageTarPath: options.ImageTarPath, ScratchRoot: options.ScratchRoot, StoreRoot: options.StoreRoot, CacheRoot: options.CacheRoot,
		KernelPath: paths.Kernel, FirecrackerPath: paths.Firecracker, EnvdPath: paths.Envd, BusyboxPath: paths.Busybox,
		KernelSHA256: paths.KernelSHA256, FirecrackerSHA256: paths.FirecrackerSHA256, EnvdSHA256: paths.EnvdSHA256, BusyboxSHA256: paths.BusyboxSHA256,
		MemoryMB: options.Build.MemoryMB, HugePages: options.Build.HugePages,
	})
	response := commandOutput{Ready: report.Ready(), Checks: report.Checks}
	if !report.Ready() {
		progressLogger.Warn(ctx, "preflight 检查未通过", zap.String("stage", "preflight_failed"), zap.Int("check_count", len(report.Checks)))
		if err := writeOutput(output, response); err != nil {
			return err
		}
		return fmt.Errorf("preflight failed; no build was started")
	}
	progressLogger.Info(ctx, "preflight 检查通过", zap.String("stage", "preflight_passed"), zap.Int("check_count", len(report.Checks)))
	if options.PreflightOnly {
		progressLogger.Info(ctx, "preflight-only 已完成，不启动构建", zap.String("stage", "preflight_only_completed"))
		return writeOutput(output, response)
	}

	buildCtx, cancel := context.WithTimeout(ctx, options.Timeout)
	defer cancel()
	// 通过 URL 结构体构造 file:// 地址，确保空格等路径字符得到正确转义。
	storeURL := (&url.URL{Scheme: "file", Path: filepath.Clean(options.StoreRoot)}).String()
	progressLogger.Info(ctx, "Creator 初始化开始", zap.String("stage", "creator_initialization_started"))
	creator, err := deps.newCreator(buildCtx, templatecreator.CreatorConfig{
		ScratchRoot: filepath.Clean(options.ScratchRoot), Store: templatecreator.StoreConfig{URL: storeURL}, Runtime: runtimeConfig,
		Cache:    templatecreator.CacheConfig{Enabled: options.CacheRoot != "" && !options.DisableBaseCache, Root: filepath.Clean(options.CacheRoot), AllowMutableImageTags: options.AllowMutableImageTagCache},
		Defaults: options.Build, Format: templatecreator.BuildFormat(options.Format), Publication: templatecreator.PublicationMode(options.Publication), MaxParallel: options.MaxParallel, LogWriter: logWriter, LogLevel: templatecreator.LogLevel(options.LogLevel),
	})
	if err != nil {
		progressLogger.Error(ctx, "Creator 初始化失败", zap.String("stage", "creator_initialization_failed"), zap.Error(err))
		return err
	}
	progressLogger.Info(ctx, "Creator 初始化完成", zap.String("stage", "creator_initialization_completed"))
	result, buildErr := creator.CreateTemplate(buildCtx, templatecreator.CreateRequest{ImageRef: options.ImageRef, ImageTarPath: options.ImageTarPath, Format: templatecreator.BuildFormat(options.Format), Publication: templatecreator.PublicationMode(options.Publication), IdempotencyKey: options.IdempotencyKey, BypassBaseCache: options.BypassBaseCache})
	// 无论构建是否成功都关闭进程资源；构建和关闭分别失败时同时保留两个错误。
	progressLogger.Info(ctx, "Creator 关闭开始", zap.String("stage", "creator_close_started"))
	closeErr := creator.Close(context.Background())
	if closeErr != nil {
		progressLogger.Error(ctx, "Creator 关闭失败", zap.String("stage", "creator_close_failed"), zap.Error(closeErr))
	} else {
		progressLogger.Info(ctx, "Creator 关闭完成", zap.String("stage", "creator_close_completed"))
	}
	if buildErr != nil || closeErr != nil {
		progressLogger.Error(ctx, "Template 创建未成功", zap.String("stage", "template_creation_failed"), zap.Error(errors.Join(buildErr, closeErr)))
		return errors.Join(buildErr, closeErr)
	}
	progressLogger.Info(ctx, "Template 创建成功", zap.String("stage", "template_creation_completed"), zap.String("template_id", result.TemplateID), zap.String("build_id", result.BuildID))
	response.Result = &result
	response.TemplateID = result.TemplateID
	response.FinalID = result.BuildID
	if err := writeOutput(output, response); err != nil {
		return err
	}
	if result.Registration.LocalPath != "" {
		writer := progressLoggerWriter(resultNoticeWriter)
		_, _ = fmt.Fprintf(writer, "PostgreSQL 注册 SQL：%s\n", result.Registration.LocalPath)
		for _, warning := range result.Registration.Warnings {
			_, _ = fmt.Fprintf(writer, "提示：%s\n", warning)
		}
	}
	exportRoot := options.ExportRoot
	if exportRoot == "" {
		exportRoot = filepath.Join(options.StoreRoot, "exports")
	}
	exporter := deps.exportArtifacts
	if exporter == nil {
		// 测试或嵌入调用方可以显式省略导出器；生产 main 已注入真实实现。
		return nil
	}
	progressLogger.Info(ctx, "Template 原始产物导出开始", zap.String("stage", "template_artifacts_export_started"), zap.String("output_root", exportRoot))
	artifacts, exportErr := exporter(buildCtx, templatecreator.ExportTemplateArtifactsRequest{Artifacts: result.Artifacts, OutputRoot: exportRoot})
	if exportErr != nil {
		progressLogger.Error(ctx, "Template 原始产物导出失败", zap.String("stage", "template_artifacts_export_failed"), zap.String("output_root", exportRoot), zap.Error(exportErr))
		return fmt.Errorf("export Template artifacts: %w", exportErr)
	}
	progressLogger.Info(ctx, "Template 原始产物导出完成", zap.String("stage", "template_artifacts_export_completed"), zap.String("output_path", artifacts.OutputPath))
	_, _ = fmt.Fprintf(progressLoggerWriter(resultNoticeWriter), "Template 原始产物目录：%s\n", artifacts.OutputPath)
	return nil
}

func loadSteps(path string) ([]templatecreator.TemplateStep, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read steps file: %w", err)
	}
	var steps []templatecreator.TemplateStep
	if err := json.Unmarshal(data, &steps); err != nil {
		return nil, fmt.Errorf("decode steps file: %w", err)
	}
	return steps, nil
}

func progressLoggerWriter(writer io.Writer) io.Writer {
	if writer == nil {
		return io.Discard
	}
	return writer
}

// writeOutput 统一格式化 stdout，确保调用方始终能按同一 JSON schema 解析。
func writeOutput(output io.Writer, response commandOutput) error {
	encoder := json.NewEncoder(output)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(response); err != nil {
		return fmt.Errorf("write result: %w", err)
	}
	return nil
}
