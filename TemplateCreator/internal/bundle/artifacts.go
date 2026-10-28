package bundle

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// ArtifactsResult 描述按 TemplateID/BuildID 目录导出的原始 Template 产物。
type ArtifactsResult struct {
	TemplateID  string `json:"template_id"`
	BuildID     string `json:"build_id"`
	ObjectCount int    `json:"object_count"`
	OutputPath  string `json:"output_path"`
	Size        int64  `json:"size"`
}

// ExportArtifacts 将 completion manifest 声明的对象闭包导出为可直接搬运的目录树：
// <output-root>/<template-id>/<build-id>/<artifact>，并在 Template 目录保存 manifest.json。
func ExportArtifacts(ctx context.Context, request Request) (ArtifactsResult, error) {
	if err := ctx.Err(); err != nil {
		return ArtifactsResult{}, err
	}
	storeRoot, err := cleanAbsoluteDirectory("StoreRoot", request.StoreRoot)
	if err != nil {
		return ArtifactsResult{}, err
	}
	manifestPath, err := cleanAbsolutePath("ManifestPath", request.ManifestPath)
	if err != nil {
		return ArtifactsResult{}, err
	}
	outputRoot, err := cleanAbsolutePath("OutputRoot", request.OutputRoot)
	if err != nil {
		return ArtifactsResult{}, err
	}
	manifestBytes, manifest, err := readManifest(manifestPath)
	if err != nil {
		return ArtifactsResult{}, err
	}
	if err := validateArchiveComponent("template_id", manifest.TemplateID); err != nil {
		return ArtifactsResult{}, err
	}
	objects, err := verifyObjects(ctx, storeRoot, manifest.Objects)
	if err != nil {
		return ArtifactsResult{}, err
	}
	if err := os.MkdirAll(outputRoot, 0o755); err != nil {
		return ArtifactsResult{}, fmt.Errorf("create output root: %w", err)
	}
	templateDir := filepath.Join(outputRoot, manifest.TemplateID)
	if err := os.MkdirAll(templateDir, 0o755); err != nil {
		return ArtifactsResult{}, fmt.Errorf("create Template output directory: %w", err)
	}
	if err := writeAtomic(filepath.Join(templateDir, "manifest.json"), manifestBytes); err != nil {
		return ArtifactsResult{}, fmt.Errorf("write exported manifest: %w", err)
	}

	var size int64
	for _, object := range objects {
		if err := ctx.Err(); err != nil {
			return ArtifactsResult{}, err
		}
		if err := validateExportObjectPath(templateDir, object.record.Key); err != nil {
			return ArtifactsResult{}, err
		}
		destination := filepath.Join(templateDir, filepath.FromSlash(object.record.Key))
		if err := copyAtomic(object.path, destination); err != nil {
			return ArtifactsResult{}, fmt.Errorf("export object %q: %w", object.record.Key, err)
		}
		size += object.record.Size
	}

	return ArtifactsResult{TemplateID: manifest.TemplateID, BuildID: manifest.BuildID, ObjectCount: len(objects), OutputPath: templateDir, Size: size}, nil
}

func validateExportObjectPath(templateDir, key string) error {
	cleaned := filepath.Join(templateDir, filepath.FromSlash(key))
	if !isWithin(templateDir, cleaned) || strings.Contains(key, "\\") {
		return fmt.Errorf("unsafe exported object path %q", key)
	}
	return nil
}

func copyAtomic(source, destination string) error {
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(destination), ".template-artifact-*.tmp")
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
	input, err := os.Open(source)
	if err != nil {
		return closeJoined(err, temporary)
	}
	_, copyErr := io.Copy(temporary, input)
	closeInputErr := input.Close()
	if copyErr != nil || closeInputErr != nil {
		return closeJoined(errorsJoin(copyErr, closeInputErr), temporary)
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
	return syncDirectory(filepath.Dir(destination))
}

// errorsJoin 避免为单纯的文件复制路径引入额外的错误处理分支。
func errorsJoin(first, second error) error {
	if first != nil && second != nil {
		return fmt.Errorf("%v; %w", first, second)
	}
	if first != nil {
		return first
	}
	return second
}
