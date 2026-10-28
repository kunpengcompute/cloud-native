// Package bundle 把 completion manifest 声明的完整 Template 对象闭包导出为单文件归档。
package bundle

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"gitcode.com/cloud-native/cloud-native/TemplateCreator/internal/publisher"
)

const archiveRootPrefix = "template-bundle-"

// Request 指定一个 Local Store completion manifest 和最终 bundle 路径。
type Request struct {
	StoreRoot    string
	ManifestPath string
	OutputPath   string
	OutputRoot   string
}

// Result 描述已经原子发布的 bundle 及其校验文件。
type Result struct {
	TemplateID  string `json:"template_id"`
	BuildID     string `json:"build_id"`
	ObjectCount int    `json:"object_count"`
	OutputPath  string `json:"output_path"`
	SHA256Path  string `json:"sha256_path"`
	SHA256      string `json:"sha256"`
	Size        int64  `json:"size"`
}

type verifiedObject struct {
	record publisher.ObjectRecord
	path   string
}

// Export 校验 completion manifest 的完整对象闭包，再原子生成 tar.gz 和 SHA-256 sidecar。
func Export(ctx context.Context, request Request) (Result, error) {
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	storeRoot, err := cleanAbsoluteDirectory("StoreRoot", request.StoreRoot)
	if err != nil {
		return Result{}, err
	}
	manifestPath, err := cleanAbsolutePath("ManifestPath", request.ManifestPath)
	if err != nil {
		return Result{}, err
	}
	outputPath, err := cleanAbsolutePath("OutputPath", request.OutputPath)
	if err != nil {
		return Result{}, err
	}
	if manifestPath == outputPath {
		return Result{}, errors.New("OutputPath must differ from ManifestPath")
	}

	manifestBytes, manifest, err := readManifest(manifestPath)
	if err != nil {
		return Result{}, err
	}
	archiveRoot, err := archiveRootForManifest(manifest)
	if err != nil {
		return Result{}, err
	}
	objects, err := verifyObjects(ctx, storeRoot, manifest.Objects)
	if err != nil {
		return Result{}, err
	}
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return Result{}, fmt.Errorf("create output directory: %w", err)
	}

	temporary, err := os.CreateTemp(filepath.Dir(outputPath), ".template-bundle-*.tmp")
	if err != nil {
		return Result{}, fmt.Errorf("create temporary bundle: %w", err)
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
	gzipWriter := gzip.NewWriter(io.MultiWriter(temporary, hash))
	tarWriter := tar.NewWriter(gzipWriter)
	writeErr := writeArchive(ctx, tarWriter, archiveRoot, manifestBytes, objects)
	closeTarErr := tarWriter.Close()
	closeGzipErr := gzipWriter.Close()
	if writeErr != nil || closeTarErr != nil || closeGzipErr != nil {
		return Result{}, closeJoined(fmt.Errorf("write bundle: %w", errors.Join(writeErr, closeTarErr, closeGzipErr)), temporary)
	}
	if err := temporary.Sync(); err != nil {
		return Result{}, closeJoined(fmt.Errorf("sync bundle: %w", err), temporary)
	}
	if err := temporary.Chmod(0o644); err != nil {
		return Result{}, closeJoined(fmt.Errorf("chmod bundle: %w", err), temporary)
	}
	if err := temporary.Close(); err != nil {
		return Result{}, fmt.Errorf("close bundle: %w", err)
	}
	info, err := os.Stat(temporaryPath)
	if err != nil {
		return Result{}, fmt.Errorf("stat bundle: %w", err)
	}
	digest := hex.EncodeToString(hash.Sum(nil))

	if err := os.Rename(temporaryPath, outputPath); err != nil {
		return Result{}, fmt.Errorf("publish bundle: %w", err)
	}
	keepTemporary = true
	if err := syncDirectory(filepath.Dir(outputPath)); err != nil {
		return Result{}, fmt.Errorf("sync output directory: %w", err)
	}
	checksumPath := outputPath + ".sha256"
	checksum := []byte(fmt.Sprintf("%s  %s\n", digest, filepath.Base(outputPath)))
	if err := writeAtomic(checksumPath, checksum); err != nil {
		return Result{}, fmt.Errorf("publish bundle checksum: %w", err)
	}

	return Result{
		TemplateID: manifest.TemplateID, BuildID: manifest.BuildID, ObjectCount: len(objects),
		OutputPath: outputPath, SHA256Path: checksumPath, SHA256: digest, Size: info.Size(),
	}, nil
}

// archiveRootForManifest 为每个 bundle 生成独立的顶层目录，避免多个归档
// 解压到同一父目录时共享 manifest.json 和 objects 路径。
func archiveRootForManifest(manifest publisher.Manifest) (string, error) {
	if err := validateArchiveComponent("template_id", manifest.TemplateID); err != nil {
		return "", err
	}
	if err := validateArchiveComponent("build_id", manifest.BuildID); err != nil {
		return "", err
	}
	return archiveRootPrefix + manifest.TemplateID + "-" + manifest.BuildID, nil
}

func validateArchiveComponent(name, value string) error {
	if value == "" || value == "." || value == ".." {
		return fmt.Errorf("%s is not a safe archive path component", name)
	}
	for _, character := range value {
		if character == '/' || character == '\\' || character == 0 || character < 0x20 || character == 0x7f {
			return fmt.Errorf("%s contains an unsafe archive path component", name)
		}
	}
	return nil
}

func readManifest(manifestPath string) ([]byte, publisher.Manifest, error) {
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, publisher.Manifest{}, fmt.Errorf("read completion manifest: %w", err)
	}
	var manifest publisher.Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, publisher.Manifest{}, fmt.Errorf("decode completion manifest: %w", err)
	}
	if manifest.SchemaVersion != publisher.ManifestSchemaVersion {
		return nil, publisher.Manifest{}, fmt.Errorf("unsupported completion manifest schema_version %d", manifest.SchemaVersion)
	}
	if manifest.TemplateID == "" || manifest.BuildID == "" || (manifest.ImageRef == "" && manifest.ImageTarPath == "") || (manifest.ImageRef != "" && manifest.ImageTarPath != "") || !json.Valid(manifest.Metadata) {
		return nil, publisher.Manifest{}, errors.New("completion manifest is missing required identity or metadata")
	}
	if len(manifest.Objects) == 0 {
		return nil, publisher.Manifest{}, errors.New("completion manifest contains no objects")
	}

	return data, manifest, nil
}

func verifyObjects(ctx context.Context, storeRoot string, records []publisher.ObjectRecord) ([]verifiedObject, error) {
	objects := make([]verifiedObject, 0, len(records))
	seen := make(map[string]struct{}, len(records))
	for _, record := range records {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if err := validateObjectRecord(record); err != nil {
			return nil, err
		}
		if _, exists := seen[record.Key]; exists {
			return nil, fmt.Errorf("duplicate object key %q", record.Key)
		}
		seen[record.Key] = struct{}{}
		objectPath := filepath.Join(storeRoot, filepath.FromSlash(record.Key))
		resolved, err := filepath.EvalSymlinks(objectPath)
		if err != nil {
			return nil, fmt.Errorf("resolve object %q: %w", record.Key, err)
		}
		if !isWithin(storeRoot, resolved) {
			return nil, fmt.Errorf("object %q escapes StoreRoot through symlink", record.Key)
		}
		lstat, err := os.Lstat(objectPath)
		if err != nil {
			return nil, fmt.Errorf("inspect object %q: %w", record.Key, err)
		}
		if !lstat.Mode().IsRegular() {
			return nil, fmt.Errorf("object %q is not a regular file", record.Key)
		}
		if lstat.Size() != record.Size {
			return nil, fmt.Errorf("object %q size = %d, want %d", record.Key, lstat.Size(), record.Size)
		}
		objects = append(objects, verifiedObject{record: record, path: objectPath})
	}

	return objects, nil
}

func validateObjectRecord(record publisher.ObjectRecord) error {
	cleaned := path.Clean(record.Key)
	if record.Key == "" || record.Key == "." || path.IsAbs(record.Key) || cleaned != record.Key || strings.HasPrefix(cleaned, "../") || strings.Contains(record.Key, "\\") {
		return fmt.Errorf("unsafe object key %q", record.Key)
	}
	if record.Size < 0 {
		return fmt.Errorf("object %q has negative size", record.Key)
	}
	digest, err := hex.DecodeString(record.SHA256)
	if err != nil || len(digest) != sha256.Size {
		return fmt.Errorf("object %q has invalid SHA-256", record.Key)
	}

	return nil
}

func writeArchive(ctx context.Context, writer *tar.Writer, archiveRoot string, manifest []byte, objects []verifiedObject) error {
	if err := writeEntry(writer, path.Join(archiveRoot, "manifest.json"), int64(len(manifest)), strings.NewReader(string(manifest))); err != nil {
		return err
	}
	for _, object := range objects {
		if err := ctx.Err(); err != nil {
			return err
		}
		file, err := os.Open(object.path)
		if err != nil {
			return fmt.Errorf("open object %q: %w", object.record.Key, err)
		}
		hash := sha256.New()
		reader := io.TeeReader(&contextReader{ctx: ctx, reader: file}, hash)
		err = writeEntry(writer, path.Join(archiveRoot, "objects", object.record.Key), object.record.Size, reader)
		closeErr := file.Close()
		if err != nil || closeErr != nil {
			return fmt.Errorf("archive object %q: %w", object.record.Key, errors.Join(err, closeErr))
		}
		digest := hex.EncodeToString(hash.Sum(nil))
		if digest != object.record.SHA256 {
			return fmt.Errorf("object %q SHA-256 = %s, want %s", object.record.Key, digest, object.record.SHA256)
		}
	}

	return nil
}

func writeEntry(writer *tar.Writer, name string, size int64, reader io.Reader) error {
	header := &tar.Header{Name: name, Mode: 0o644, Size: size, Typeflag: tar.TypeReg}
	if err := writer.WriteHeader(header); err != nil {
		return err
	}
	_, err := io.Copy(writer, reader)
	return err
}

type contextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (reader *contextReader) Read(buffer []byte) (int, error) {
	if err := reader.ctx.Err(); err != nil {
		return 0, err
	}
	return reader.reader.Read(buffer)
}

func cleanAbsoluteDirectory(name, value string) (string, error) {
	cleaned, err := cleanAbsolutePath(name, value)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(cleaned)
	if err != nil {
		return "", fmt.Errorf("inspect %s: %w", name, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("%s is not a directory", name)
	}
	resolved, err := filepath.EvalSymlinks(cleaned)
	if err != nil {
		return "", fmt.Errorf("resolve %s: %w", name, err)
	}

	return resolved, nil
}

func cleanAbsolutePath(name, value string) (string, error) {
	if !filepath.IsAbs(value) {
		return "", fmt.Errorf("%s must be an absolute path", name)
	}

	return filepath.Clean(value), nil
}

func isWithin(root, candidate string) bool {
	relative, err := filepath.Rel(root, candidate)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func writeAtomic(destination string, data []byte) error {
	directory := filepath.Dir(destination)
	temporary, err := os.CreateTemp(directory, ".bundle-checksum-*.tmp")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	keep := false
	defer func() {
		if !keep {
			if rmErr := os.Remove(temporaryPath); rmErr != nil && !os.IsNotExist(rmErr) {
				// 临时文件清理失败不影响已产出的结果，此处仅显式检查错误。
			}
		}
	}()
	if _, err := temporary.Write(data); err != nil {
		return closeJoined(err, temporary)
	}
	if err := temporary.Sync(); err != nil {
		return closeJoined(err, temporary)
	}
	if err := temporary.Chmod(0o644); err != nil {
		return closeJoined(err, temporary)
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Rename(temporaryPath, destination); err != nil {
		return err
	}
	keep = true

	return syncDirectory(directory)
}

func syncDirectory(directory string) error {
	file, err := os.Open(directory)
	if err != nil {
		return err
	}
	defer file.Close()

	return file.Sync()
}
