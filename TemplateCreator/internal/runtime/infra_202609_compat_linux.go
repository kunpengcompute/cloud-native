//go:build linux

package runtime

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	upstreamstorage "gitcode.com/cloud-native/cloud-native/TemplateCreator/internal/upstream/shared/pkg/storage"
)

// infra202609RuntimeView 是旧 Builder 期望的无 architecture 子目录视图。
type infra202609RuntimeView struct {
	KernelsDir             string
	FirecrackerVersionsDir string
}

// validateInfra202609BaseCacheEnabled 在旧 storage contract 完成独立适配前拒绝共享 cache。
func validateInfra202609BaseCacheEnabled(enabled bool) error {
	if enabled {
		return fmt.Errorf("persistent base cache is not supported by the infra 2026.09 runtime adapter")
	}

	return nil
}

// prepareInfra202609RuntimeView 将稳定的多架构资产目录映射为 2026.09 Builder 查找布局。
func prepareInfra202609RuntimeView(jobRoot, runtimeRoot, architecture, kernelVersion, firecrackerVersion string) (infra202609RuntimeView, error) {
	runtimeRoot, err := filepath.Abs(runtimeRoot)
	if err != nil {
		return infra202609RuntimeView{}, fmt.Errorf("resolve runtime root: %w", err)
	}
	viewRoot := filepath.Join(jobRoot, "runtime-compat")
	view := infra202609RuntimeView{
		KernelsDir:             filepath.Join(viewRoot, "kernels"),
		FirecrackerVersionsDir: filepath.Join(viewRoot, "firecrackers"),
	}
	assets := []struct {
		name   string
		source string
		target string
	}{
		{
			name:   "kernel",
			source: filepath.Join(runtimeRoot, "kernels", kernelVersion, architecture, "vmlinux.bin"),
			target: filepath.Join(view.KernelsDir, kernelVersion, "vmlinux.bin"),
		},
		{
			name:   "Firecracker",
			source: filepath.Join(runtimeRoot, "firecrackers", firecrackerVersion, architecture, "firecracker"),
			target: filepath.Join(view.FirecrackerVersionsDir, firecrackerVersion, "firecracker"),
		},
	}
	for _, asset := range assets {
		info, err := os.Stat(asset.source)
		if err != nil {
			return infra202609RuntimeView{}, fmt.Errorf("%s runtime asset %s: %w", asset.name, asset.source, err)
		}
		if !info.Mode().IsRegular() {
			return infra202609RuntimeView{}, fmt.Errorf("%s runtime asset %s is not a regular file", asset.name, asset.source)
		}
		if err := os.MkdirAll(filepath.Dir(asset.target), 0o755); err != nil {
			return infra202609RuntimeView{}, fmt.Errorf("create %s compatibility directory: %w", asset.name, err)
		}
		if err := os.Symlink(asset.source, asset.target); err != nil {
			return infra202609RuntimeView{}, fmt.Errorf("link %s runtime asset: %w", asset.name, err)
		}
	}

	return view, nil
}

// jobLocalStorage 为单个构建提供显式 root，不依赖 2026.09 storage 的进程环境变量。
type jobLocalStorage struct {
	root string
}

type jobLocalObject struct {
	path string
}

type jobLocalRangeReader struct {
	io.Reader
	file *os.File
}

// infra202609BusyboxInstaller 将 2026.09 upstream 的 x86-only embed 替换为
// 当前 runtime profile 已验证的架构 binary。首次安装后不允许进程内切换内容。
type infra202609BusyboxInstaller struct {
	mu     sync.Mutex
	sha256 string
}

var processBusyboxInstaller infra202609BusyboxInstaller

func (installer *infra202609BusyboxInstaller) install(sourcePath, expectedSHA256 string, destination *[]byte) error {
	content, err := os.ReadFile(sourcePath)
	if err != nil {
		return fmt.Errorf("read infra 2026.09 BusyBox %s: %w", sourcePath, err)
	}
	actualSHA256 := fmt.Sprintf("%x", sha256.Sum256(content))
	if actualSHA256 != expectedSHA256 {
		return fmt.Errorf("infra 2026.09 BusyBox SHA-256 mismatch: got %s, want %s", actualSHA256, expectedSHA256)
	}

	installer.mu.Lock()
	defer installer.mu.Unlock()
	if installer.sha256 != "" && installer.sha256 != actualSHA256 {
		return fmt.Errorf("infra 2026.09 BusyBox is already initialized with SHA-256 %s; refusing %s", installer.sha256, actualSHA256)
	}
	if installer.sha256 == "" {
		*destination = append([]byte(nil), content...)
		installer.sha256 = actualSHA256
	}
	return nil
}

var (
	_ upstreamstorage.StorageProvider = (*jobLocalStorage)(nil)
	_ upstreamstorage.Blob            = (*jobLocalObject)(nil)
	_ upstreamstorage.Seekable        = (*jobLocalObject)(nil)
)

func newJobLocalStorage(root string) (upstreamstorage.StorageProvider, error) {
	absolute, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve local storage root: %w", err)
	}
	if err := os.MkdirAll(absolute, 0o755); err != nil {
		return nil, fmt.Errorf("create local storage root: %w", err)
	}

	return &jobLocalStorage{root: absolute}, nil
}

func (storage *jobLocalStorage) DeleteObjectsWithPrefix(_ context.Context, prefix string) error {
	objectPath, err := storage.objectPath(prefix)
	if err != nil {
		return err
	}

	return os.RemoveAll(objectPath)
}

func (storage *jobLocalStorage) UploadSignedURL(_ context.Context, _ string, _ time.Duration) (string, error) {
	return "", fmt.Errorf("job-local storage does not support signed URLs")
}

func (storage *jobLocalStorage) OpenBlob(_ context.Context, key string, _ upstreamstorage.ObjectType) (upstreamstorage.Blob, error) {
	objectPath, err := storage.prepareObjectPath(key)
	if err != nil {
		return nil, err
	}

	return &jobLocalObject{path: objectPath}, nil
}

func (storage *jobLocalStorage) OpenSeekable(_ context.Context, key string, _ upstreamstorage.SeekableObjectType) (upstreamstorage.Seekable, error) {
	objectPath, err := storage.prepareObjectPath(key)
	if err != nil {
		return nil, err
	}

	return &jobLocalObject{path: objectPath}, nil
}

func (storage *jobLocalStorage) GetDetails() string {
	return fmt.Sprintf("[TemplateCreator job-local storage, base path %s]", storage.root)
}

func (storage *jobLocalStorage) prepareObjectPath(key string) (string, error) {
	objectPath, err := storage.objectPath(key)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(objectPath), 0o755); err != nil {
		return "", fmt.Errorf("create object directory: %w", err)
	}

	return objectPath, nil
}

func (storage *jobLocalStorage) objectPath(key string) (string, error) {
	if key == "" || strings.Contains(key, `\`) {
		return "", fmt.Errorf("unsafe storage object key %q", key)
	}
	clean := filepath.Clean(filepath.FromSlash(key))
	if filepath.IsAbs(clean) || clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("unsafe storage object key %q", key)
	}
	objectPath := filepath.Join(storage.root, clean)
	relative, err := filepath.Rel(storage.root, objectPath)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("storage object key %q escapes root", key)
	}

	return objectPath, nil
}

func (object *jobLocalObject) WriteTo(_ context.Context, destination io.Writer) (int64, error) {
	file, err := os.Open(object.path)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, upstreamstorage.ErrObjectNotExist
		}
		return 0, err
	}
	defer file.Close()

	return io.Copy(destination, file)
}

func (object *jobLocalObject) Put(_ context.Context, data []byte) error {
	return os.WriteFile(object.path, data, 0o644)
}

func (object *jobLocalObject) Exists(_ context.Context) (bool, error) {
	_, err := os.Stat(object.path)
	if os.IsNotExist(err) {
		return false, nil
	}

	return err == nil, err
}

func (object *jobLocalObject) ReadAt(_ context.Context, buffer []byte, offset int64) (int, error) {
	file, err := os.Open(object.path)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, upstreamstorage.ErrObjectNotExist
		}
		return 0, err
	}
	defer file.Close()

	return file.ReadAt(buffer, offset)
}

func (object *jobLocalObject) Size(_ context.Context) (int64, error) {
	info, err := os.Stat(object.path)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, upstreamstorage.ErrObjectNotExist
		}
		return 0, err
	}

	return info.Size(), nil
}

func (object *jobLocalObject) StoreFile(_ context.Context, sourcePath string) error {
	source, err := os.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("open source file: %w", err)
	}
	defer source.Close()

	return copyJobLocalFile(object.path, source)
}

// copyJobLocalFile 以流式方式写入 rootfs 等大对象，并截断可能存在的旧内容。
func copyJobLocalFile(destinationPath string, source io.Reader) (returnErr error) {
	destination, err := os.OpenFile(destinationPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return fmt.Errorf("open destination file: %w", err)
	}
	defer func() {
		if err := destination.Close(); err != nil && returnErr == nil {
			returnErr = fmt.Errorf("close destination file: %w", err)
		}
	}()

	if _, err := io.Copy(destination, source); err != nil {
		return fmt.Errorf("copy source file: %w", err)
	}

	return nil
}

func (object *jobLocalObject) OpenRangeReader(_ context.Context, offset, length int64) (io.ReadCloser, error) {
	file, err := os.Open(object.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, upstreamstorage.ErrObjectNotExist
		}
		return nil, err
	}

	return &jobLocalRangeReader{Reader: io.NewSectionReader(file, offset, length), file: file}, nil
}

func (reader *jobLocalRangeReader) Close() error {
	return reader.file.Close()
}
