package templatecreator

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"gitcode.com/cloud-native/cloud-native/TemplateCreator/internal/runtimeassets"
)

const defaultPrepareRuntimeTimeout = 5 * time.Minute

// PrepareRuntimeRequest 描述一次 runtime 资产准备请求。
type PrepareRuntimeRequest struct {
	// ManifestPath 是版本化 runtime manifest 的绝对路径。
	ManifestPath string
	// RuntimeRoot 是最终安装 kernel、Firecracker、envd 和 busybox 的绝对根目录。
	RuntimeRoot string
	// Architecture 选择 manifest 中的一套 CPU 架构；为空时使用当前进程的 runtime.GOARCH。
	Architecture string
	// E2BInfraRoot 是可选的已部署 e2b-infra 根文件系统，用于复用 checksum 匹配的本地资产。
	E2BInfraRoot string
	// Timeout 是下载每个远程资产使用的 HTTP 超时；0 使用默认值，负数无效。
	Timeout time.Duration
}

// PreparedRuntimeAsset 描述一个已经通过 SHA-256 校验并安装到本机的 runtime 文件。
type PreparedRuntimeAsset struct {
	// Type 是资产类型，例如 kernel、firecracker、envd 或 busybox。
	Type string
	// Version 是 manifest 锁定的资产版本。
	Version string
	// Architecture 是该文件适用的 CPU 架构。
	Architecture string
	// Path 是安装完成后的绝对文件路径，可直接交给 Builder 使用。
	Path string
	// SHA256 是安装文件内容的十六进制 SHA-256 摘要。
	SHA256 string
	// Source 表示本次采用 existing、bundled、local_reuse 或 download 中的哪一种来源。
	Source string
	// SourcePath 记录实际来源路径；CLI 输出会有意省略它，避免泄露调用机目录布局。
	SourcePath string
}

// PrepareRuntimeResult 返回可直接放入 CreatorConfig.Runtime 的配置和逐项安装信息。
type PrepareRuntimeResult struct {
	// Runtime 是根据实际安装结果组装的固定 runtime ABI 配置。
	Runtime RuntimeConfig
	// Assets 按 manifest 选择顺序列出本次确认可用的全部 runtime 文件。
	Assets []PreparedRuntimeAsset
}

// PrepareRuntime 根据 manifest 准备全部 Builder runtime 文件，不修改进程级环境变量。
// 资产只有在版本、架构和 SHA-256 均符合 manifest 后才会进入最终 RuntimeRoot。
func PrepareRuntime(ctx context.Context, request PrepareRuntimeRequest) (PrepareRuntimeResult, error) {
	request, err := normalizePrepareRuntimeRequest(request)
	if err != nil {
		return PrepareRuntimeResult{}, err
	}

	manifestFile, err := os.Open(request.ManifestPath)
	if err != nil {
		return PrepareRuntimeResult{}, fmt.Errorf("open runtime ManifestPath: %w", err)
	}
	manifest, parseErr := parseRuntimeManifest(manifestFile)
	closeErr := manifestFile.Close()
	if parseErr != nil {
		return PrepareRuntimeResult{}, parseErr
	}
	if closeErr != nil {
		return PrepareRuntimeResult{}, fmt.Errorf("close runtime ManifestPath: %w", closeErr)
	}

	selected, err := manifest.selectArchitecture(request.Architecture)
	if err != nil {
		return PrepareRuntimeResult{}, err
	}
	assets := make([]runtimeassets.Asset, 0, len(selected))
	for _, asset := range selected {
		mode, _ := parseAssetMode(asset.Mode) // manifest validator 已保证 mode 可解析。
		var bundledPath string
		if asset.BundledPath != "" {
			// bundled path 相对 manifest 解析，使 Go module 或解压后的 runtime bundle
			// 可以整体移动，而不会固化打包机器上的绝对路径。
			bundledPath = filepath.Join(filepath.Dir(request.ManifestPath), filepath.FromSlash(asset.BundledPath))
		}
		assets = append(assets, runtimeassets.Asset{
			Type:         string(asset.Type),
			Version:      asset.Version,
			Architecture: asset.Architecture,
			URL:          asset.URL,
			BundledPath:  bundledPath,
			ReusePaths:   append([]string(nil), asset.ReusePaths...),
			SHA256:       asset.SHA256,
			Destination:  asset.Destination,
			Mode:         os.FileMode(mode),
		})
	}

	installed, err := (runtimeassets.Installer{
		Root:      request.RuntimeRoot,
		InfraRoot: request.E2BInfraRoot,
		Client:    &http.Client{Timeout: request.Timeout},
	}).Install(ctx, assets)
	if err != nil {
		return PrepareRuntimeResult{}, fmt.Errorf("prepare runtime assets: %w", err)
	}

	// RuntimeConfig 必须来自实际安装结果，不能相信调用方对版本的猜测；
	// 同一组值随后会写入 BuildMetadata，作为 snapshot ABI 的可审计记录。
	result := PrepareRuntimeResult{
		Runtime: RuntimeConfig{
			RuntimeDir:      request.RuntimeRoot,
			InfraCommit:     manifest.InfraCommit,
			TemplateVersion: manifest.TemplateVersion,
			Architecture:    request.Architecture,
			BusyboxDir:      filepath.Join(request.RuntimeRoot, "busybox"),
		},
		Assets: make([]PreparedRuntimeAsset, 0, len(installed)),
	}
	for _, asset := range installed {
		result.Assets = append(result.Assets, PreparedRuntimeAsset{
			Type:         asset.Type,
			Version:      asset.Version,
			Architecture: asset.Architecture,
			Path:         asset.Path,
			SHA256:       asset.SHA256,
			Source:       string(asset.Source),
			SourcePath:   asset.SourcePath,
		})
		switch runtimeAssetType(asset.Type) {
		case assetKernel:
			result.Runtime.KernelVersion = asset.Version
			result.Runtime.KernelSHA256 = asset.SHA256
		case assetFirecracker:
			result.Runtime.FirecrackerVersion = asset.Version
			result.Runtime.FirecrackerSHA256 = asset.SHA256
		case assetEnvd:
			result.Runtime.EnvdVersion = asset.Version
			result.Runtime.EnvdPath = asset.Path
			result.Runtime.EnvdSHA256 = asset.SHA256
		case assetBusybox:
			result.Runtime.BusyboxVersion = asset.Version
			result.Runtime.BusyboxSHA256 = asset.SHA256
		}
	}

	return result, nil
}

// normalizePrepareRuntimeRequest 验证绝对路径约束、补齐默认值并清理路径表示。
func normalizePrepareRuntimeRequest(request PrepareRuntimeRequest) (PrepareRuntimeRequest, error) {
	if request.ManifestPath == "" || !filepath.IsAbs(request.ManifestPath) {
		return PrepareRuntimeRequest{}, fmt.Errorf("ManifestPath must be an absolute path")
	}
	if request.RuntimeRoot == "" || !filepath.IsAbs(request.RuntimeRoot) {
		return PrepareRuntimeRequest{}, fmt.Errorf("RuntimeRoot must be an absolute path")
	}
	if request.E2BInfraRoot != "" && !filepath.IsAbs(request.E2BInfraRoot) {
		return PrepareRuntimeRequest{}, fmt.Errorf("E2BInfraRoot must be an absolute path when provided")
	}
	if request.Timeout < 0 {
		return PrepareRuntimeRequest{}, fmt.Errorf("Timeout must not be negative")
	}
	if request.Timeout == 0 {
		request.Timeout = defaultPrepareRuntimeTimeout
	}
	if request.Architecture == "" {
		request.Architecture = runtime.GOARCH
	}
	request.ManifestPath = filepath.Clean(request.ManifestPath)
	request.RuntimeRoot = filepath.Clean(request.RuntimeRoot)
	if request.E2BInfraRoot != "" {
		request.E2BInfraRoot = filepath.Clean(request.E2BInfraRoot)
	}

	return request, nil
}
