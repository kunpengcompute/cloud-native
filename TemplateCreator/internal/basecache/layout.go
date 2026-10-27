package basecache

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// CacheSchemaVersion 隔离未来不兼容的 cache 布局或 key 语义。
const CacheSchemaVersion = "v1"

// RuntimeABI 是决定 Firecracker snapshot 能否安全复用的稳定版本组合。
// 字段顺序同时定义 namespace JSON 的编码顺序，修改时必须升级 CacheSchemaVersion。
type RuntimeABI struct {
	InfraCommit        string `json:"infra_commit"`
	Architecture       string `json:"architecture"`
	TemplateVersion    string `json:"template_version"`
	KernelVersion      string `json:"kernel_version"`
	FirecrackerVersion string `json:"firecracker_version"`
	EnvdVersion        string `json:"envd_version"`
	BusyboxVersion     string `json:"busybox_version"`
}

// Layout 描述一个 runtime ABI namespace 下的全部持久路径。
type Layout struct {
	Root           string
	Namespace      string
	NamespaceRoot  string
	TemplateRoot   string
	BuildIndexRoot string
	CorruptRoot    string
	LockPath       string
	MetadataPath   string
	CacheScope     string
	ABI            RuntimeABI
}

type namespaceInput struct {
	CacheSchemaVersion string `json:"cache_schema_version"`
	RuntimeABI
}

type namespaceMetadata struct {
	CacheSchemaVersion string     `json:"cache_schema_version"`
	Namespace          string     `json:"namespace"`
	ABI                RuntimeABI `json:"runtime_abi"`
}

// NewLayout 计算可复现的 ABI namespace，并组装持久目录布局。
func NewLayout(root string, abi RuntimeABI) (Layout, error) {
	if !filepath.IsAbs(root) {
		return Layout{}, fmt.Errorf("cache root must be absolute")
	}
	for name, value := range map[string]string{
		"InfraCommit": abi.InfraCommit, "Architecture": abi.Architecture,
		"TemplateVersion": abi.TemplateVersion, "KernelVersion": abi.KernelVersion,
		"FirecrackerVersion": abi.FirecrackerVersion, "EnvdVersion": abi.EnvdVersion,
		"BusyboxVersion": abi.BusyboxVersion,
	} {
		if strings.TrimSpace(value) == "" {
			return Layout{}, fmt.Errorf("runtime ABI %s is required", name)
		}
	}
	encoded, err := json.Marshal(namespaceInput{CacheSchemaVersion: CacheSchemaVersion, RuntimeABI: abi})
	if err != nil {
		return Layout{}, fmt.Errorf("encode runtime ABI namespace: %w", err)
	}
	digest := sha256.Sum256(encoded)
	namespace := hex.EncodeToString(digest[:])
	cleanRoot := filepath.Clean(root)
	namespaceRoot := filepath.Join(cleanRoot, CacheSchemaVersion, namespace)

	return Layout{
		Root: cleanRoot, Namespace: namespace, NamespaceRoot: namespaceRoot,
		TemplateRoot:   filepath.Join(namespaceRoot, "templates"),
		BuildIndexRoot: filepath.Join(namespaceRoot, "build-index"),
		CorruptRoot:    filepath.Join(namespaceRoot, "corrupt"),
		LockPath:       filepath.Join(namespaceRoot, "cache.lock"),
		MetadataPath:   filepath.Join(namespaceRoot, "namespace.json"),
		CacheScope:     "templatecreator-base-v1-" + namespace,
		ABI:            abi,
	}, nil
}

// Prepare 创建持久目录和 namespace metadata。已经存在的 metadata 必须与
// 当前 ABI 完全一致；不一致或损坏时拒绝覆盖，避免错误复用 snapshot。
func Prepare(layout Layout) error {
	for _, directory := range []string{layout.TemplateRoot, layout.BuildIndexRoot, layout.CorruptRoot} {
		if err := os.MkdirAll(directory, 0o755); err != nil {
			return fmt.Errorf("create base cache directory %s: %w", directory, err)
		}
	}
	expected := namespaceMetadata{
		CacheSchemaVersion: CacheSchemaVersion,
		Namespace:          layout.Namespace,
		ABI:                layout.ABI,
	}
	if err := validateNamespaceMetadata(layout.MetadataPath, expected); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}

	encoded, err := json.MarshalIndent(expected, "", "  ")
	if err != nil {
		return fmt.Errorf("encode base cache namespace metadata: %w", err)
	}
	encoded = append(encoded, '\n')
	if err := createFileWithoutReplace(layout.MetadataPath, encoded); err != nil {
		if errors.Is(err, os.ErrExist) {
			// 另一个进程可能同时发布了 metadata；只接受完全相同的内容。
			return validateNamespaceMetadata(layout.MetadataPath, expected)
		}
		return fmt.Errorf("publish base cache namespace metadata: %w", err)
	}

	return nil
}

func validateNamespaceMetadata(path string, expected namespaceMetadata) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return fmt.Errorf("base cache namespace metadata %q must be a regular non-symlink file", path)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read base cache namespace metadata: %w", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(content))
	decoder.DisallowUnknownFields()
	var actual namespaceMetadata
	if err := decoder.Decode(&actual); err != nil {
		return fmt.Errorf("decode base cache namespace metadata: %w", err)
	}
	if err := requireJSONEOF(decoder); err != nil {
		return fmt.Errorf("decode base cache namespace metadata: %w", err)
	}
	if actual != expected {
		return fmt.Errorf("base cache namespace metadata mismatch: got %#v, want %#v", actual, expected)
	}

	return nil
}

func requireJSONEOF(decoder *json.Decoder) error {
	var trailing any
	if err := decoder.Decode(&trailing); errors.Is(err, io.EOF) {
		return nil
	} else if err != nil {
		return err
	}
	return fmt.Errorf("unexpected trailing JSON value")
}

// createFileWithoutReplace 先完整写入同目录临时文件，再通过 hard link 原子发布。
// link 不会覆盖已有目标，因此并发初始化也不会破坏另一个进程的 metadata。
func createFileWithoutReplace(path string, content []byte) (returnErr error) {
	temporary, err := os.CreateTemp(filepath.Dir(path), ".namespace-*.tmp")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer func() {
		returnErr = errors.Join(returnErr, os.Remove(temporaryPath))
	}()
	if err := temporary.Chmod(0o644); err != nil {
		_ = temporary.Close()
		return err
	}
	if _, err := temporary.Write(content); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Link(temporaryPath, path); err != nil {
		return err
	}

	return nil
}
