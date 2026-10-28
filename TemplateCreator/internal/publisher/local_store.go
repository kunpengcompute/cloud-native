package publisher

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// LocalStore 将完整 Template 对象闭包发布到 file URL 指定的本地目录。
type LocalStore struct {
	// rawURL 保留调用方原始 URL，用于返回稳定 PublishLocation。
	rawURL string
	// root 是清理后的本地绝对根目录。
	root string
}

// NewLocalStore 解析并验证 Local Artifact Store URL；当前只接受本机绝对 file URL。
func NewLocalStore(rawURL string) (*LocalStore, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("parse store URL: %w", err)
	}
	if parsed.Scheme != "file" {
		return nil, fmt.Errorf("unsupported store scheme %q: only file:// is supported", parsed.Scheme)
	}
	if parsed.Host != "" && parsed.Host != "localhost" {
		return nil, fmt.Errorf("file store host must be empty or localhost")
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" {
		return nil, fmt.Errorf("file store must not contain query or fragment")
	}
	if !filepath.IsAbs(parsed.Path) {
		return nil, fmt.Errorf("file store path must be absolute")
	}

	return &LocalStore{rawURL: rawURL, root: filepath.Clean(parsed.Path)}, nil
}

// Publish 先持久化全部对象，最后原子发布唯一的 completion manifest。
func (s *LocalStore) Publish(ctx context.Context, request PublishRequest) (PublishLocation, error) {
	if s == nil {
		return PublishLocation{}, errors.New("local store is nil")
	}
	if err := validateManifest(request.Manifest); err != nil {
		return PublishLocation{}, err
	}
	if len(request.Objects) == 0 {
		return PublishLocation{}, errors.New("publish requires at least one object")
	}

	records := make([]ObjectRecord, 0, len(request.Objects))
	seen := make(map[string]struct{}, len(request.Objects))
	for _, object := range request.Objects {
		if err := ctx.Err(); err != nil {
			return PublishLocation{}, err
		}
		if err := validateObjectKey(object.Key); err != nil {
			return PublishLocation{}, err
		}
		if _, exists := seen[object.Key]; exists {
			return PublishLocation{}, fmt.Errorf("duplicate object key %q", object.Key)
		}
		seen[object.Key] = struct{}{}

		record, err := s.publishObject(ctx, object)
		if err != nil {
			return PublishLocation{}, fmt.Errorf("publish object %q: %w", object.Key, err)
		}
		records = append(records, record)
	}

	// manifest 是唯一 completion marker；只有每个对象都到达 durable storage 后，
	// 才编码并原子安装它，防止消费方把半成品误认为成功。
	manifest := request.Manifest
	manifest.Objects = records
	manifestKey := path.Join("_templatecreator", "manifests", manifest.TemplateID, manifest.BuildID+".json")
	manifestData, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return PublishLocation{}, fmt.Errorf("encode manifest: %w", err)
	}
	manifestData = append(manifestData, '\n')
	if err := s.writeAtomic(ctx, manifestKey, manifestData, ".manifest-*.tmp", 0o644); err != nil {
		return PublishLocation{}, fmt.Errorf("publish manifest: %w", err)
	}

	return PublishLocation{StoreURL: s.rawURL, ManifestKey: manifestKey}, nil
}

// WriteTemplateInfo 原子写入便于人工查看的 Template 创建结果。
// 该文件不是 completion marker；调用方可在写入失败时保留已发布 Template。
func (s *LocalStore) WriteTemplateInfo(ctx context.Context, templateID string, data []byte) error {
	if s == nil {
		return errors.New("local store is nil")
	}
	if err := validateKeySegment("TemplateID", templateID); err != nil {
		return err
	}
	if len(data) == 0 || !json.Valid(data) {
		return errors.New("template info must be valid JSON")
	}

	return s.writeAtomic(ctx, path.Join(templateID, "info.json"), data, ".info-*.tmp", 0o644)
}

// WriteRegistrationSQL 原子写入 final Build 目录下的 PostgreSQL 注册辅助文件。
// 该文件有意不加入 completion manifest；未来 S3 uploader 需要显式追加上传。
func (s *LocalStore) WriteRegistrationSQL(ctx context.Context, buildID string, data []byte) error {
	if s == nil {
		return errors.New("local store is nil")
	}
	if err := validateKeySegment("BuildID", buildID); err != nil {
		return err
	}
	if len(data) == 0 {
		return errors.New("registration SQL must not be empty")
	}

	return s.writeAtomic(ctx, path.Join(buildID, "registration.sql"), data, ".registration-*.tmp", 0o644)
}

// validateManifest 在落盘前拒绝缺失字段、危险 ID 和无效 metadata。
func validateManifest(manifest Manifest) error {
	if manifest.SchemaVersion != ManifestSchemaVersion {
		return fmt.Errorf("unsupported manifest schema version %d", manifest.SchemaVersion)
	}
	if err := validateKeySegment("TemplateID", manifest.TemplateID); err != nil {
		return err
	}
	if err := validateKeySegment("BuildID", manifest.BuildID); err != nil {
		return err
	}
	if strings.TrimSpace(manifest.ImageRef) == "" && strings.TrimSpace(manifest.ImageTarPath) == "" {
		return errors.New("manifest ImageRef or ImageTarPath is required")
	}
	if strings.TrimSpace(manifest.ImageRef) != "" && strings.TrimSpace(manifest.ImageTarPath) != "" {
		return errors.New("manifest ImageRef and ImageTarPath are mutually exclusive")
	}
	if strings.TrimSpace(manifest.InfraCommit) == "" {
		return errors.New("manifest InfraCommit is required")
	}
	if manifest.BuiltAt.IsZero() {
		return errors.New("manifest BuiltAt is required")
	}
	if len(manifest.Metadata) == 0 || !json.Valid(manifest.Metadata) {
		return errors.New("manifest Metadata must be valid JSON")
	}
	for _, ancestor := range manifest.AncestorBuildIDs {
		if err := validateKeySegment("ancestor build ID", ancestor); err != nil {
			return err
		}
	}

	return nil
}

// validateKeySegment 保证调用方 ID 只能占用单个安全路径段。
func validateKeySegment(name, value string) error {
	if value == "" || value == "." || value == ".." || strings.ContainsAny(value, "/\\") {
		return fmt.Errorf("unsafe %s %q", name, value)
	}

	return nil
}

// validateObjectKey 按 E2B object key 的 slash 语义拒绝绝对路径、回退和反斜杠。
func validateObjectKey(key string) error {
	// object key 在所有平台都使用 slash；不能用 filepath.Clean，
	// 否则持久化 key 会随 host OS 改变。
	cleaned := path.Clean(key)
	if key == "" || key == "." || path.IsAbs(key) || cleaned != key || strings.HasPrefix(cleaned, "../") || strings.Contains(key, "\\") {
		return fmt.Errorf("unsafe object key %q", key)
	}

	return nil
}

// publishObject 边复制边计算 checksum，并以同目录 rename 原子替换目标对象。
func (s *LocalStore) publishObject(ctx context.Context, object ObjectSource) (ObjectRecord, error) {
	source, err := os.Open(object.Path)
	if err != nil {
		return ObjectRecord{}, err
	}
	defer source.Close()

	info, err := source.Stat()
	if err != nil {
		return ObjectRecord{}, err
	}
	if !info.Mode().IsRegular() {
		return ObjectRecord{}, fmt.Errorf("source is not a regular file")
	}

	directory, err := s.ensureDirectory(path.Dir(object.Key))
	if err != nil {
		return ObjectRecord{}, err
	}
	destination := filepath.Join(directory, filepath.Base(filepath.FromSlash(object.Key)))

	// 临时文件与目标位于同一目录，保证 rename 原子；复制时同步 hash，
	// 使 manifest 记录的正是最终持久化字节。
	temporary, err := os.CreateTemp(directory, ".object-*.tmp")
	if err != nil {
		return ObjectRecord{}, err
	}
	temporaryPath := temporary.Name()
	keepTemporary := false
	defer func() {
		if !keepTemporary {
			if rmErr := os.Remove(temporaryPath); rmErr != nil && !os.IsNotExist(rmErr) {
				// 临时文件清理失败不影响已产出的结果，此处仅显式检查错误。
			}
		}
	}()

	hash := sha256.New()
	written, copyErr := io.Copy(io.MultiWriter(temporary, hash), &contextReader{ctx: ctx, reader: source})
	if copyErr != nil {
		return ObjectRecord{}, closeJoined(copyErr, temporary)
	}
	if err := temporary.Sync(); err != nil {
		return ObjectRecord{}, closeJoined(err, temporary)
	}
	if err := temporary.Chmod(0o644); err != nil {
		return ObjectRecord{}, closeJoined(err, temporary)
	}
	if err := temporary.Close(); err != nil {
		return ObjectRecord{}, err
	}
	if err := os.Rename(temporaryPath, destination); err != nil {
		return ObjectRecord{}, err
	}
	keepTemporary = true
	// rename 后 fsync directory，确保已经报告的成功在 crash 后仍可恢复。
	if err := syncDirectory(directory); err != nil {
		return ObjectRecord{}, err
	}

	return ObjectRecord{Key: object.Key, Size: written, SHA256: hex.EncodeToString(hash.Sum(nil))}, nil
}

// writeAtomic 将内存数据通过同目录临时文件、fsync 和 rename 原子写入 Store。
func (s *LocalStore) writeAtomic(ctx context.Context, key string, data []byte, pattern string, mode os.FileMode) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := validateObjectKey(key); err != nil {
		return err
	}

	directory, err := s.ensureDirectory(path.Dir(key))
	if err != nil {
		return err
	}
	destination := filepath.Join(directory, filepath.Base(filepath.FromSlash(key)))

	temporary, err := os.CreateTemp(directory, pattern)
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	keepTemporary := false
	defer func() {
		if !keepTemporary {
			if rmErr := os.Remove(temporaryPath); rmErr != nil && !os.IsNotExist(rmErr) {
				// 临时文件清理失败不影响已产出的结果，此处仅显式检查错误。
			}
		}
	}()

	if _, err := (&contextWriter{ctx: ctx, writer: temporary}).Write(data); err != nil {
		return closeJoined(err, temporary)
	}
	if err := temporary.Sync(); err != nil {
		return closeJoined(err, temporary)
	}
	if err := temporary.Chmod(mode); err != nil {
		return closeJoined(err, temporary)
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Rename(temporaryPath, destination); err != nil {
		return err
	}
	keepTemporary = true

	return syncDirectory(directory)
}

// ensureDirectory 逐段创建真实目录，并保证 relative 无法通过 symlink 逃逸 Store root。
func (s *LocalStore) ensureDirectory(relative string) (string, error) {
	if err := ensureRealDirectory(s.root); err != nil {
		return "", fmt.Errorf("store root: %w", err)
	}
	if relative == "." || relative == "" {
		return s.root, nil
	}

	// 必须逐段 walk + lstat；单次 MkdirAll 会跟随攻击者可控 symlink，
	// 从而逃逸配置的 Store root。
	current := s.root
	for _, segment := range strings.Split(relative, "/") {
		if err := validateKeySegment("directory segment", segment); err != nil {
			return "", err
		}
		current = filepath.Join(current, segment)
		if err := ensureRealDirectory(current); err != nil {
			return "", err
		}
	}

	return current, nil
}

// ensureRealDirectory 创建或确认一个非 symlink 的真实目录。
func ensureRealDirectory(directory string) error {
	info, err := os.Lstat(directory)
	if err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("directory %q is a symlink", directory)
		}
		if !info.IsDir() {
			return fmt.Errorf("path %q is not a directory", directory)
		}

		return nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.Mkdir(directory, 0o755); err != nil && !errors.Is(err, os.ErrExist) {
		return err
	}

	info, err = os.Lstat(directory)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("directory %q is a symlink", directory)
	}
	if !info.IsDir() {
		return fmt.Errorf("path %q is not a directory", directory)
	}

	return nil
}

// syncDirectory 持久化目录 entry 变化。
func syncDirectory(path string) error {
	directory, err := os.Open(path)
	if err != nil {
		return err
	}
	defer directory.Close()

	return directory.Sync()
}

// contextReader 在每次底层读取前检查取消，使大对象复制可以及时停止。
type contextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r *contextReader) Read(buffer []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}

	return r.reader.Read(buffer)
}

// contextWriter 在写入原子文件前检查取消。
type contextWriter struct {
	ctx    context.Context
	writer io.Writer
}

func (w *contextWriter) Write(buffer []byte) (int, error) {
	if err := w.ctx.Err(); err != nil {
		return 0, err
	}

	return w.writer.Write(buffer)
}
