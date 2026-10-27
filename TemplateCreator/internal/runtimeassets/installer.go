// Package runtimeassets 从现有、module bundled、本地复用或下载来源安装 checksum 固定的 Builder runtime 文件。
package runtimeassets

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const defaultHTTPTimeout = 5 * time.Minute

// Asset 是一个 runtime 文件经过 manifest 验证后的内部安装描述。
type Asset struct {
	// Type/Version/Architecture 共同标识该 runtime 资产。
	Type         string
	Version      string
	Architecture string
	// URL 是本地来源均不可用时采用的可选下载地址。
	URL string
	// BundledPath 是 module 或 runtime bundle 内随附文件的绝对解析结果。
	BundledPath string
	// ReusePaths 是按顺序尝试的 e2b-infra 本地候选路径。
	ReusePaths []string
	// SHA256 是所有来源都必须通过的统一内容门禁。
	SHA256 string
	// Destination 是相对 Installer.Root 的安全目标路径。
	Destination string
	// Mode 是最终安装文件的权限位。
	Mode os.FileMode
}

// Source 标识已安装资产实际采用的来源。
type Source string

const (
	// SourceExisting 表示目标位置原本就有 checksum 匹配的文件。
	SourceExisting Source = "existing"
	// SourceBundled 表示使用 module 或 runtime bundle 随附文件。
	SourceBundled Source = "bundled"
	// SourceLocalReuse 表示复用了 checksum 匹配的 e2b-infra 文件。
	SourceLocalReuse Source = "local_reuse"
	// SourceDownload 表示从 manifest URL 下载。
	SourceDownload Source = "download"
)

// InstalledAsset 记录最终路径和来源，不包含下载 credential。
type InstalledAsset struct {
	// Type/Version/Architecture 复制自已验证的 Asset。
	Type         string
	Version      string
	Architecture string
	// Path 是 checksum 验证通过后的最终绝对路径。
	Path string
	// SHA256 是最终文件应满足的内容摘要。
	SHA256 string
	// Source 表示本次实际采用哪一种安装来源。
	Source Source
	// SourcePath 仅对本地来源有值，用于 Go API 审计，不应无条件输出到 CLI。
	SourcePath string
}

// Installer 在 Root 下原子准备 runtime 资产。
// InfraRoot 可把 `/fc-envd/envd` 等 manifest 路径重映射到 test/chroot，
// 或挂载在另一个目录下的 e2b-infra 根文件系统。
type Installer struct {
	// Root 是 TemplateCreator 自己的 runtime 安装根目录。
	Root string
	// InfraRoot 是可选的 e2b-infra 根文件系统映射前缀。
	InfraRoot string
	// Client 控制下载 transport 和 timeout；nil 时使用安全默认 client。
	Client *http.Client
}

// Install 按 manifest 顺序准备资产，只有全部文件有效后才返回结果。
func (installer Installer) Install(ctx context.Context, assets []Asset) ([]InstalledAsset, error) {
	if strings.TrimSpace(installer.Root) == "" {
		return nil, fmt.Errorf("runtime root is required")
	}
	root, err := filepath.Abs(installer.Root)
	if err != nil {
		return nil, fmt.Errorf("resolve runtime root: %w", err)
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, fmt.Errorf("create runtime root: %w", err)
	}

	results := make([]InstalledAsset, 0, len(assets))
	for _, asset := range assets {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		result, err := installer.installOne(ctx, root, asset)
		if err != nil {
			return nil, fmt.Errorf("install %s %s for %s: %w", asset.Type, asset.Version, asset.Architecture, err)
		}
		results = append(results, result)
	}

	return results, nil
}

// installOne 按 existing → bundled → local reuse → download 顺序为单个资产选择来源。
func (installer Installer) installOne(ctx context.Context, root string, asset Asset) (InstalledAsset, error) {
	destination, err := safeDestination(root, asset.Destination)
	if err != nil {
		return InstalledAsset{}, err
	}

	valid, err := fileHasChecksum(destination, asset.SHA256)
	if err != nil && !os.IsNotExist(err) {
		return InstalledAsset{}, fmt.Errorf("inspect existing destination: %w", err)
	}
	if valid {
		if err := os.Chmod(destination, asset.Mode.Perm()); err != nil {
			return InstalledAsset{}, fmt.Errorf("correct existing destination mode: %w", err)
		}
		return installedResult(asset, destination, SourceExisting, ""), nil
	}

	// 候选顺序是 contract：bundled 是 module 提供的可复现 fallback，
	// 之后才尝试 checksum 兼容的 e2b-infra 文件。
	type localCandidate struct {
		path   string
		source Source
	}
	candidates := make([]localCandidate, 0, len(asset.ReusePaths)+1)
	if asset.BundledPath != "" {
		candidates = append(candidates, localCandidate{path: asset.BundledPath, source: SourceBundled})
	}
	for _, candidate := range asset.ReusePaths {
		candidates = append(candidates, localCandidate{path: installer.resolveReusePath(candidate), source: SourceLocalReuse})
	}

	var lastLocalErr error
	for _, candidate := range candidates {
		source := candidate.path
		file, openErr := os.Open(source)
		if os.IsNotExist(openErr) {
			continue
		}
		if openErr != nil {
			return InstalledAsset{}, fmt.Errorf("open reuse source %q: %w", source, openErr)
		}
		// bundled、reuse、download 都经过同一个 installReader checksum 门禁；
		// 本地候选不匹配时只会继续 fallback，绝不会被复制到最终位置。
		installErr := installReader(ctx, destination, asset, file)
		closeErr := file.Close()
		if installErr == nil && closeErr != nil {
			installErr = fmt.Errorf("close reuse source %q: %w", source, closeErr)
		}
		if installErr == nil {
			return installedResult(asset, destination, candidate.source, source), nil
		}
		lastLocalErr = fmt.Errorf("local source %q: %w", source, installErr)
	}

	if asset.URL == "" {
		if lastLocalErr != nil {
			return InstalledAsset{}, lastLocalErr
		}
		return InstalledAsset{}, fmt.Errorf("no usable local source and no download URL")
	}
	if err := installer.download(ctx, destination, asset); err != nil {
		if lastLocalErr != nil {
			return InstalledAsset{}, fmt.Errorf("%v; download: %w", lastLocalErr, err)
		}
		return InstalledAsset{}, err
	}

	return installedResult(asset, destination, SourceDownload, ""), nil
}

// resolveReusePath 将 manifest 中以 `/` 为根的复用路径映射到可选 InfraRoot。
func (installer Installer) resolveReusePath(manifestPath string) string {
	if installer.InfraRoot == "" {
		// 在实际部署 host 上，`/fc-envd/envd` 已经正确地以当前 `/` 为根；
		// 只有挂载另一套 rootfs 时才需要 InfraRoot。
		return filepath.Clean(manifestPath)
	}
	relative := strings.TrimPrefix(filepath.Clean(manifestPath), string(filepath.Separator))

	return filepath.Join(installer.InfraRoot, relative)
}

// download 只接受 HTTP 200，并把响应流交给统一的原子 checksum 安装路径。
func (installer Installer) download(ctx context.Context, destination string, asset Asset) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, asset.URL, nil)
	if err != nil {
		return fmt.Errorf("create download request: %w", err)
	}
	client := installer.Client
	if client == nil {
		client = &http.Client{Timeout: defaultHTTPTimeout}
	}
	response, err := client.Do(request)
	if err != nil {
		return fmt.Errorf("download %s: %w", asset.URL, err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4<<10))
		return fmt.Errorf("download %s: HTTP %s", asset.URL, response.Status)
	}
	if err := installReader(ctx, destination, asset, response.Body); err != nil {
		return fmt.Errorf("download %s: %w", asset.URL, err)
	}

	return nil
}

// installReader 边复制边校验 SHA-256，通过后才原子替换 destination。
func installReader(ctx context.Context, destination string, asset Asset, source io.Reader) (returnErr error) {
	// 临时文件必须与目标同目录，才能保证 rename 原子且不跨 filesystem。
	temporary, err := os.CreateTemp(filepath.Dir(destination), "."+filepath.Base(destination)+".partial-*")
	if err != nil {
		return fmt.Errorf("create temporary file: %w", err)
	}
	temporaryPath := temporary.Name()
	temporaryClosed := false
	defer func() {
		if !temporaryClosed {
			if closeErr := temporary.Close(); returnErr == nil && closeErr != nil {
				returnErr = fmt.Errorf("close temporary file: %w", closeErr)
			}
		}
		if removeErr := os.Remove(temporaryPath); returnErr == nil && removeErr != nil && !os.IsNotExist(removeErr) {
			returnErr = fmt.Errorf("remove temporary file: %w", removeErr)
		}
	}()

	hash := sha256.New()
	if _, err := io.Copy(io.MultiWriter(temporary, hash), contextReader{ctx: ctx, reader: source}); err != nil {
		return fmt.Errorf("copy asset: %w", err)
	}
	actualChecksum := fmt.Sprintf("%x", hash.Sum(nil))
	if actualChecksum != asset.SHA256 {
		return fmt.Errorf("checksum mismatch: got %s, want %s", actualChecksum, asset.SHA256)
	}
	if err := temporary.Sync(); err != nil {
		return fmt.Errorf("sync temporary file: %w", err)
	}
	if err := temporary.Chmod(asset.Mode.Perm()); err != nil {
		return fmt.Errorf("set temporary file mode: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close temporary file before rename: %w", err)
	}
	temporaryClosed = true
	if err := os.Rename(temporaryPath, destination); err != nil {
		return fmt.Errorf("atomically replace destination: %w", err)
	}
	// 返回成功前既要持久化文件内容，也要持久化 directory entry。
	if err := syncDirectory(filepath.Dir(destination)); err != nil {
		return err
	}

	return nil
}

// contextReader 让长时间复制在每次 Read 前响应 context 取消。
type contextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (reader contextReader) Read(buffer []byte) (int, error) {
	if err := reader.ctx.Err(); err != nil {
		return 0, err
	}

	return reader.reader.Read(buffer)
}

// safeDestination 验证相对路径并逐段拒绝 symlink，防止资产写出 caller-selected root。
func safeDestination(root, relative string) (string, error) {
	if relative == "" || filepath.IsAbs(relative) || filepath.Clean(relative) != relative || relative == "." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("unsafe destination %q", relative)
	}

	// 对每个 parent 使用 Lstat，而不是让 MkdirAll 跟随 symlink；
	// runtime 资产绝不能逃逸调用方选择的 root。
	current := root
	parts := strings.Split(filepath.Dir(relative), string(filepath.Separator))
	for _, part := range parts {
		if part == "." || part == "" {
			continue
		}
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if os.IsNotExist(err) {
			if err := os.Mkdir(current, 0o755); err != nil && !os.IsExist(err) {
				return "", fmt.Errorf("create destination directory %q: %w", current, err)
			}
			continue
		}
		if err != nil {
			return "", fmt.Errorf("inspect destination directory %q: %w", current, err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("destination directory %q is a symlink", current)
		}
		if !info.IsDir() {
			return "", fmt.Errorf("destination parent %q is not a directory", current)
		}
	}

	destination := filepath.Join(root, relative)
	if info, err := os.Lstat(destination); err == nil && info.Mode()&os.ModeSymlink != 0 {
		return "", fmt.Errorf("destination %q is a symlink", destination)
	} else if err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("inspect destination %q: %w", destination, err)
	}

	return destination, nil
}

// fileHasChecksum 重新读取整个现有文件，不能仅凭文件名或版本目录判断可复用。
func fileHasChecksum(path, expected string) (bool, error) {
	file, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return false, err
	}

	return fmt.Sprintf("%x", hash.Sum(nil)) == expected, nil
}

// syncDirectory 确保 rename 后的目录项在 crash 后仍然存在。
func syncDirectory(path string) error {
	directory, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open destination directory for sync: %w", err)
	}
	defer directory.Close()
	if err := directory.Sync(); err != nil {
		return fmt.Errorf("sync destination directory: %w", err)
	}

	return nil
}

// installedResult 从已验证 Asset 构造不含 credential 的稳定返回值。
func installedResult(asset Asset, path string, source Source, sourcePath string) InstalledAsset {
	return InstalledAsset{
		Type:         asset.Type,
		Version:      asset.Version,
		Architecture: asset.Architecture,
		Path:         path,
		SHA256:       asset.SHA256,
		Source:       source,
		SourcePath:   sourcePath,
	}
}
