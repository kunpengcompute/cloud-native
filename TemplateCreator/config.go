package templatecreator

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

var (
	// ErrInvalidConfig 标识无效的进程级 Creator 配置，可通过 errors.Is 判断。
	ErrInvalidConfig = errors.New("invalid creator config")
	// ErrInvalidRequest 标识无效的单次构建请求，可通过 errors.Is 判断。
	ErrInvalidRequest = errors.New("invalid create request")
)

// validateCreatorConfig 在启动任何进程级资源前验证必填路径、runtime ABI 和默认构建参数。
func validateCreatorConfig(cfg CreatorConfig) error {
	if strings.TrimSpace(cfg.ScratchRoot) == "" {
		return fmt.Errorf("%w: ScratchRoot is required", ErrInvalidConfig)
	}
	if !filepath.IsAbs(cfg.ScratchRoot) {
		return fmt.Errorf("%w: ScratchRoot must be absolute", ErrInvalidConfig)
	}

	if err := validateFileStoreURL(cfg.Store.URL); err != nil {
		return fmt.Errorf("%w: Store.URL: %w", ErrInvalidConfig, err)
	}
	if err := validateCacheConfig(cfg); err != nil {
		return fmt.Errorf("%w: Cache: %w", ErrInvalidConfig, err)
	}
	if err := validateRuntimeConfig(cfg.Runtime); err != nil {
		return fmt.Errorf("%w: Runtime: %w", ErrInvalidConfig, err)
	}
	if err := validateBuildSpec(cfg.Defaults); err != nil {
		return fmt.Errorf("%w: Defaults: %w", ErrInvalidConfig, err)
	}
	if cfg.MaxParallel < 0 {
		return fmt.Errorf("%w: MaxParallel must not be negative", ErrInvalidConfig)
	}
	switch cfg.LogLevel {
	case "", LogLevelOff, LogLevelInfo, LogLevelDebug:
	default:
		return fmt.Errorf("%w: LogLevel must be one of off, info, or debug", ErrInvalidConfig)
	}

	return nil
}

// normalizeCreatorConfig 应用不影响调用方原始配置的进程默认值，并深拷贝可变的 Steps。
func normalizeCreatorConfig(cfg CreatorConfig) CreatorConfig {
	if strings.TrimSpace(cfg.ScratchRoot) == "" {
		cfg.ScratchRoot = DefaultScratchRoot
	}
	if strings.TrimSpace(cfg.Store.URL) == "" {
		cfg.Store.URL = DefaultStoreURL()
	}
	if cfg.MaxParallel == 0 {
		cfg.MaxParallel = DefaultMaxParallel
	}
	if cfg.Format == "" {
		cfg.Format = DefaultBuildFormat
	}
	if cfg.Publication == "" {
		cfg.Publication = DefaultPublicationMode
	}
	if cfg.Defaults.VCPU == 0 {
		cfg.Defaults.VCPU = DefaultVCPU
	}
	if cfg.Defaults.MemoryMB == 0 {
		cfg.Defaults.MemoryMB = DefaultMemoryMB
	}
	if cfg.Defaults.DiskSizeMB == 0 {
		cfg.Defaults.DiskSizeMB = DefaultDiskSizeMB
	}
	// 显式 LogLevel 优先；零值继续兼容旧版 LogWriter/DebugLogs 调用方式。
	if cfg.LogLevel == "" {
		switch {
		case cfg.DebugLogs:
			cfg.LogLevel = LogLevelDebug
		case cfg.LogWriter != nil:
			cfg.LogLevel = LogLevelInfo
		default:
			cfg.LogLevel = LogLevelOff
		}
	}
	switch cfg.LogLevel {
	case LogLevelOff:
		cfg.LogWriter = nil
		cfg.DebugLogs = false
	case LogLevelInfo:
		cfg.DebugLogs = false
	case LogLevelDebug:
		cfg.DebugLogs = true
	}
	cfg.Defaults = cloneBuildSpec(cfg.Defaults)
	if cfg.Cache.Enabled {
		cfg.Cache.Root = filepath.Clean(cfg.Cache.Root)
	}

	return cfg
}

// defaultProvisioningSteps 对齐 E2B Python SDK from_dockerfile 在仅指定 FROM 时
// 建立 Docker 初始上下文并恢复 E2B 默认上下文的四个请求步骤。
func defaultProvisioningSteps() []TemplateStep {
	return []TemplateStep{
		{Type: "USER", Args: []string{"root"}},
		{Type: "WORKDIR", Args: []string{"/"}},
		{Type: "USER", Args: []string{"user"}},
		{Type: "WORKDIR", Args: []string{"/home/user"}},
	}
}

// validateCacheConfig 保证持久 cache 不会落入会被清理的 ScratchRoot，
// 也不会与最终 Store 混用，避免清理 cache 时误删已发布产物。
func validateCacheConfig(cfg CreatorConfig) error {
	if !cfg.Cache.Enabled {
		return nil
	}
	if strings.TrimSpace(cfg.Cache.Root) == "" {
		return fmt.Errorf("Cache.Root is required when cache is enabled")
	}
	if !filepath.IsAbs(cfg.Cache.Root) {
		return fmt.Errorf("Cache.Root must be absolute")
	}
	storeURL, err := url.Parse(cfg.Store.URL)
	if err != nil {
		// Store URL 的详细错误由 validateFileStoreURL 负责；这里保留防御性返回。
		return fmt.Errorf("parse Store.URL: %w", err)
	}
	for name, root := range map[string]string{
		"ScratchRoot": cfg.ScratchRoot,
		"Store.URL":   storeURL.Path,
	} {
		if filesystemPathsOverlap(cfg.Cache.Root, root) {
			return fmt.Errorf("Cache.Root and %s must not overlap", name)
		}
	}
	// infra 2026.09 使用旧 storage/header contract；在独立适配完成前 fail closed，
	// 避免把 2026.29 的共享 index 语义混入目标版本构建。
	return fmt.Errorf("persistent base cache is not supported by the infra 2026.09 runtime adapter")
}

// filesystemPathsOverlap 按词法路径判断两个绝对 root 是否相同或互为祖先。
// symlink 的真实指向由 CLI 只读 preflight 在访问宿主文件系统时继续检查。
func filesystemPathsOverlap(first, second string) bool {
	contains := func(parent, child string) bool {
		relative, err := filepath.Rel(filepath.Clean(parent), filepath.Clean(child))
		if err != nil {
			return false
		}
		return relative == "." || (relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)))
	}

	return contains(first, second) || contains(second, first)
}

// validateFileStoreURL 将 Demo 的持久化边界限制为本机绝对 file URL。
func validateFileStoreURL(rawURL string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("parse URL: %w", err)
	}
	if parsed.Scheme != "file" {
		return fmt.Errorf("only file:// stores are supported by the Demo")
	}
	if parsed.Host != "" && parsed.Host != "localhost" {
		return fmt.Errorf("file:// host must be empty or localhost")
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" {
		return fmt.Errorf("file:// store must not contain query or fragment")
	}
	if !filepath.IsAbs(parsed.Path) {
		return fmt.Errorf("file:// store path must be absolute")
	}

	return nil
}

// validateRuntimeConfig 检查 Builder 查找资产及记录 runtime ABI 所需的字段是否完整。
func validateRuntimeConfig(cfg RuntimeConfig) error {
	required := []struct {
		name  string
		value string
	}{
		{name: "RuntimeDir", value: cfg.RuntimeDir},
		{name: "InfraCommit", value: cfg.InfraCommit},
		{name: "TemplateVersion", value: cfg.TemplateVersion},
		{name: "Architecture", value: cfg.Architecture},
		{name: "KernelVersion", value: cfg.KernelVersion},
		{name: "FirecrackerVersion", value: cfg.FirecrackerVersion},
		{name: "EnvdVersion", value: cfg.EnvdVersion},
		{name: "EnvdPath", value: cfg.EnvdPath},
		{name: "BusyboxDir", value: cfg.BusyboxDir},
		{name: "BusyboxVersion", value: cfg.BusyboxVersion},
	}
	for _, field := range required {
		if strings.TrimSpace(field.value) == "" {
			return fmt.Errorf("%s is required", field.name)
		}
	}
	if !filepath.IsAbs(cfg.RuntimeDir) {
		return fmt.Errorf("RuntimeDir must be absolute")
	}
	if !filepath.IsAbs(cfg.EnvdPath) {
		return fmt.Errorf("EnvdPath must be absolute")
	}
	if !filepath.IsAbs(cfg.BusyboxDir) {
		return fmt.Errorf("BusyboxDir must be absolute")
	}
	assets := []struct {
		name       string
		path       string
		sha256     string
		executable bool
	}{
		{name: "kernel", path: filepath.Join(cfg.RuntimeDir, "kernels", cfg.KernelVersion, cfg.Architecture, "vmlinux.bin"), sha256: cfg.KernelSHA256},
		{name: "Firecracker", path: filepath.Join(cfg.RuntimeDir, "firecrackers", cfg.FirecrackerVersion, cfg.Architecture, "firecracker"), sha256: cfg.FirecrackerSHA256, executable: true},
		{name: "envd", path: cfg.EnvdPath, sha256: cfg.EnvdSHA256, executable: true},
		{name: "BusyBox", path: filepath.Join(cfg.BusyboxDir, cfg.BusyboxVersion, cfg.Architecture, "busybox"), sha256: cfg.BusyboxSHA256, executable: true},
	}
	for _, asset := range assets {
		if err := validateRuntimeAssetFile(asset.name, asset.path, asset.sha256, asset.executable); err != nil {
			return err
		}
	}

	return nil
}

// validateRuntimeAssetFile 让公共 Go API 与 CLI preflight 使用同等级别的资产证明。
func validateRuntimeAssetFile(name, assetPath, expectedSHA256 string, executable bool) error {
	if !isLowerHex(expectedSHA256, 64) {
		return fmt.Errorf("%s SHA-256 must be 64 lowercase hexadecimal characters", name)
	}
	file, err := os.Open(assetPath)
	if err != nil {
		return fmt.Errorf("open %s runtime asset %s: %w", name, assetPath, err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return fmt.Errorf("inspect %s runtime asset %s: %w", name, assetPath, err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("%s runtime asset %s is not a regular file", name, assetPath)
	}
	if executable && info.Mode().Perm()&0o111 == 0 {
		return fmt.Errorf("%s runtime asset %s is not executable", name, assetPath)
	}
	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return fmt.Errorf("hash %s runtime asset %s: %w", name, assetPath, err)
	}
	actual := fmt.Sprintf("%x", hasher.Sum(nil))
	if actual != expectedSHA256 {
		return fmt.Errorf("%s runtime asset SHA-256 mismatch: got %s, want %s", name, actual, expectedSHA256)
	}
	return nil
}

// validateCreateRequest 验证每次构建独有、无法由 Creator 默认配置补齐的输入。
func validateCreateRequest(req CreateRequest) error {
	imageRef := strings.TrimSpace(req.ImageRef)
	imageTarPath := strings.TrimSpace(req.ImageTarPath)
	if imageRef == "" && imageTarPath == "" {
		return fmt.Errorf("%w: ImageRef or ImageTarPath is required", ErrInvalidRequest)
	}
	if imageRef != "" && imageTarPath != "" {
		return fmt.Errorf("%w: ImageRef and ImageTarPath are mutually exclusive", ErrInvalidRequest)
	}

	return nil
}

// normalizeCreateRequest 为未提供镜像的 Go API 请求补齐集中默认镜像。
func normalizeCreateRequest(req CreateRequest) CreateRequest {
	if strings.TrimSpace(req.ImageRef) == "" && strings.TrimSpace(req.ImageTarPath) == "" {
		req.ImageRef = DefaultImageRef
	}

	return req
}

// resolveBuildOptions 合并 Creator 默认值、请求级 BuildSpec 与两个 profile 覆盖值。
func resolveBuildOptions(defaults BuildSpec, defaultFormat BuildFormat, defaultPublication PublicationMode, override *BuildSpec, formatOverride BuildFormat, publicationOverride PublicationMode) (BuildSpec, BuildFormat, PublicationMode, error) {
	format := defaultFormat
	if format == "" {
		format = DefaultBuildFormat
	}
	if formatOverride != "" {
		format = formatOverride
	}
	if !validBuildFormat(format) {
		return BuildSpec{}, "", "", fmt.Errorf("unsupported build format %q", format)
	}

	publication := defaultPublication
	if publication == "" {
		publication = DefaultPublicationMode
	}
	if publicationOverride != "" {
		publication = publicationOverride
	}
	if !validPublicationMode(publication) {
		return BuildSpec{}, "", "", fmt.Errorf("unsupported publication mode %q", publication)
	}

	resolved := cloneBuildSpec(defaults)
	if resolved.VCPU == 0 {
		resolved.VCPU = DefaultVCPU
	}
	if resolved.MemoryMB == 0 {
		resolved.MemoryMB = DefaultMemoryMB
	}
	if resolved.DiskSizeMB == 0 {
		resolved.DiskSizeMB = DefaultDiskSizeMB
	}
	// 判断 Steps 是否已配置（nil 即未配置，继承 format 默认步骤）；
	// 显式空切片属于“已配置”，不进入该分支，语义为清空全部步骤。
	if resolved.Steps == nil {
		resolved.Steps = stepsForBuildFormat(format)
	}
	if override != nil {
		if override.VCPU != 0 {
			resolved.VCPU = override.VCPU
		}
		if override.MemoryMB != 0 {
			resolved.MemoryMB = override.MemoryMB
		}
		if override.DiskSizeMB != 0 {
			resolved.DiskSizeMB = override.DiskSizeMB
		}
		resolved.HugePages = override.HugePages
		if override.StartCommand != "" {
			resolved.StartCommand = override.StartCommand
		}
		if override.ReadyCommand != "" {
			resolved.ReadyCommand = override.ReadyCommand
		}
		// override 提供了 Steps（nil 表示未提供而继承默认，非 nil 含空切片
		// 表示显式配置）时整体替换默认步骤。
		if override.Steps != nil {
			resolved.Steps = cloneTemplateSteps(override.Steps)
		}
	}
	if err := validateBuildSpec(resolved); err != nil {
		return BuildSpec{}, "", "", err
	}

	return resolved, format, publication, nil
}

// resolveBuildSpec 将请求级 override 合并到进程默认值，并返回与调用方切片隔离的完整配置。
func resolveBuildSpec(defaults BuildSpec, override *BuildSpec) (BuildSpec, error) {
	resolved, _, _, err := resolveBuildOptions(defaults, DefaultBuildFormat, DefaultPublicationMode, override, "", "")
	return resolved, err
}

// validateBuildSpec 确保传给 Builder 的核心资源数量均为正数。
func validateBuildSpec(spec BuildSpec) error {
	if spec.VCPU <= 0 {
		return fmt.Errorf("VCPU must be positive")
	}
	if spec.MemoryMB <= 0 {
		return fmt.Errorf("MemoryMB must be positive")
	}
	if spec.DiskSizeMB <= 0 {
		return fmt.Errorf("DiskSizeMB must be positive")
	}

	return nil
}

// cloneBuildSpec 深拷贝 BuildSpec 中可变的 Steps，避免配置在并发构建间被外部修改。
func cloneBuildSpec(spec BuildSpec) BuildSpec {
	spec.Steps = cloneTemplateSteps(spec.Steps)

	return spec
}

// cloneTemplateSteps 同时复制 step 切片及每个 step 的 Args 切片。
func cloneTemplateSteps(steps []TemplateStep) []TemplateStep {
	// 注意：下面的比较判断的是“调用方是否配置了 Steps”，而不是“切片是否为空”。
	// Steps 为 nil 与非 nil 是两种不同的配置状态：nil 表示未配置（由 format 决定
	// 默认步骤）；非 nil（包括显式空切片）表示已配置，其中空切片的语义是“清空
	// 全部请求步骤”。clone 必须原样保持该区分：若改用 len(steps) == 0 把空切片
	// 折叠成 nil，“显式清空”会被误判为“未配置”而重新继承默认步骤。
	if steps != nil {
		result := make([]TemplateStep, len(steps))
		for i, step := range steps {
			// Creator 持有的配置应从调用方视角保持不可变。
			result[i] = step
			result[i].Args = append([]string(nil), step.Args...)
		}
		return result
	}
	return nil
}
