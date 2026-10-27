package templatecreator

import (
	"context"
	"fmt"
	"net/url"
	"path"
	"path/filepath"
	"strings"

	internalbundle "gitcode.com/cloud-native/cloud-native/TemplateCreator/internal/bundle"
)

// ExportBundleRequest 使用 CreateResult 返回的 ArtifactLocation 定位已完成 Template。
type ExportBundleRequest struct {
	Artifacts  ArtifactLocation
	OutputPath string
}

// ExportBundleResult 描述可传输到目标 E2B 的单文件 bundle 和 SHA-256 sidecar。
type ExportBundleResult struct {
	TemplateID  string `json:"template_id"`
	BuildID     string `json:"build_id"`
	ObjectCount int    `json:"object_count"`
	OutputPath  string `json:"output_path"`
	SHA256Path  string `json:"sha256_path"`
	SHA256      string `json:"sha256"`
	Size        int64  `json:"size"`
}

// ExportTemplateArtifactsRequest 指定原始 Template 产物的输出根目录。
type ExportTemplateArtifactsRequest struct {
	Artifacts  ArtifactLocation
	OutputRoot string
}

// ExportTemplateArtifactsResult 描述按 TemplateID/BuildID 目录导出的原始对象。
type ExportTemplateArtifactsResult struct {
	TemplateID  string `json:"template_id"`
	BuildID     string `json:"build_id"`
	ObjectCount int    `json:"object_count"`
	OutputPath  string `json:"output_path"`
	Size        int64  `json:"size"`
}

// ExportTemplateBundle 按 completion manifest 导出 final 与 ancestor 的完整对象闭包。
// 该函数只读取 Local Store，不接触 E2B 控制面，也不会把 registry credential 写入 bundle。
func ExportTemplateBundle(ctx context.Context, request ExportBundleRequest) (ExportBundleResult, error) {
	if err := validateFileStoreURL(request.Artifacts.StoreURL); err != nil {
		return ExportBundleResult{}, fmt.Errorf("Artifacts.StoreURL: %w", err)
	}
	manifestKey := request.Artifacts.ManifestKey
	cleanedKey := path.Clean(manifestKey)
	if manifestKey == "" || manifestKey == "." || path.IsAbs(manifestKey) || cleanedKey != manifestKey || strings.HasPrefix(cleanedKey, "../") || strings.Contains(manifestKey, "\\") {
		return ExportBundleResult{}, fmt.Errorf("unsafe manifest key %q", manifestKey)
	}
	parsed, err := url.Parse(request.Artifacts.StoreURL)
	if err != nil {
		return ExportBundleResult{}, fmt.Errorf("parse StoreURL: %w", err)
	}
	result, err := internalbundle.Export(ctx, internalbundle.Request{
		StoreRoot: parsed.Path, ManifestPath: filepath.Join(parsed.Path, filepath.FromSlash(manifestKey)),
		OutputPath: request.OutputPath,
	})
	if err != nil {
		return ExportBundleResult{}, err
	}

	return ExportBundleResult{
		TemplateID: result.TemplateID, BuildID: result.BuildID, ObjectCount: result.ObjectCount,
		OutputPath: result.OutputPath, SHA256Path: result.SHA256Path,
		SHA256: result.SHA256, Size: result.Size,
	}, nil
}

// ExportTemplateArtifacts 将 Template 完整对象闭包导出为目录树，不生成压缩包。
func ExportTemplateArtifacts(ctx context.Context, request ExportTemplateArtifactsRequest) (ExportTemplateArtifactsResult, error) {
	if err := validateFileStoreURL(request.Artifacts.StoreURL); err != nil {
		return ExportTemplateArtifactsResult{}, fmt.Errorf("Artifacts.StoreURL: %w", err)
	}
	manifestKey := request.Artifacts.ManifestKey
	cleanedKey := path.Clean(manifestKey)
	if manifestKey == "" || manifestKey == "." || path.IsAbs(manifestKey) || cleanedKey != manifestKey || strings.HasPrefix(cleanedKey, "../") || strings.Contains(manifestKey, "\\") {
		return ExportTemplateArtifactsResult{}, fmt.Errorf("unsafe manifest key %q", manifestKey)
	}
	parsed, err := url.Parse(request.Artifacts.StoreURL)
	if err != nil {
		return ExportTemplateArtifactsResult{}, fmt.Errorf("parse StoreURL: %w", err)
	}
	result, err := internalbundle.ExportArtifacts(ctx, internalbundle.Request{
		StoreRoot: parsed.Path, ManifestPath: filepath.Join(parsed.Path, filepath.FromSlash(manifestKey)), OutputRoot: request.OutputRoot,
	})
	if err != nil {
		return ExportTemplateArtifactsResult{}, err
	}
	return ExportTemplateArtifactsResult{TemplateID: result.TemplateID, BuildID: result.BuildID, ObjectCount: result.ObjectCount, OutputPath: result.OutputPath, Size: result.Size}, nil
}
