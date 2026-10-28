//go:build linux

package templatecreator

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"gitcode.com/boostkit/cloud-native.git/TemplateCreator/internal/publisher"
	upstreamid "gitcode.com/boostkit/cloud-native.git/TemplateCreator/internal/upstream/shared/pkg/id"
)

// CreateTemplate 同步构建并发布完整的本地 Template 对象闭包。
// 只有全部对象和最后的 completion manifest 都写入成功后才返回 CreateResult。
func (creator *Creator) CreateTemplate(ctx context.Context, request CreateRequest) (CreateResult, error) {
	if err := creator.beginCall(); err != nil {
		return CreateResult{}, err
	}
	defer creator.endCall()
	request = normalizeCreateRequest(request)
	creator.logger.Info(ctx, "Template 创建请求开始", zap.String("stage", "create_template_started"), zap.Bool("idempotency_enabled", request.IdempotencyKey != ""))
	if request.IdempotencyKey == "" {
		result, err := creator.createTemplateOnce(ctx, request, "")
		if err != nil {
			return CreateResult{}, err
		}
		result = creator.attachRegistrationSQL(ctx, request, result)
		creator.writeTemplateInfo(ctx, result)
		return result, nil
	}
	digest, err := creator.idempotencyDigest(request)
	if err != nil {
		return CreateResult{}, err
	}
	// 同一个任务编号的锁覆盖物理构建和发布全过程，因此并发调用不会观察到半成品。
	creator.logger.Info(ctx, "任务完成记录查询开始", zap.String("stage", "idempotency_lookup_started"))
	data, reused, err := creator.idempotency.WithKey(ctx, request.IdempotencyKey, digest, func() (json.RawMessage, error) {
		result, err := creator.createTemplateOnce(ctx, request, digest)
		if err != nil {
			return nil, err
		}
		return json.Marshal(result)
	})
	if err != nil {
		creator.logger.Error(ctx, "任务编号请求处理失败", zap.String("stage", "idempotency_request_failed"), zap.Error(err))
		return CreateResult{}, err
	}
	if reused {
		creator.logger.Info(ctx, "复用已完成的任务结果", zap.String("stage", "idempotency_result_reused"))
	} else {
		creator.logger.Info(ctx, "新的任务完成结果已记录", zap.String("stage", "idempotency_result_recorded"))
	}
	var result CreateResult
	if err := json.Unmarshal(data, &result); err != nil {
		return CreateResult{}, fmt.Errorf("decode idempotent CreateResult: %w", err)
	}
	// 注册 SQL 不属于 Template 构建内容摘要。即使复用了旧任务结果，也根据
	// 当前稳定默认值重新生成并落盘，不要求重新执行物理构建。
	result = creator.attachRegistrationSQL(ctx, request, result)
	creator.writeTemplateInfo(ctx, result)
	return result, nil
}

// attachRegistrationSQL 生成并保存 PostgreSQL 注册辅助文件。
// 该文件不属于 completion manifest；写入失败只告警，避免推翻已经完整发布的 Template。
func (creator *Creator) attachRegistrationSQL(ctx context.Context, request CreateRequest, result CreateResult) CreateResult {
	rendered, err := renderRegistrationSQL(registrationSQLRequest{
		StoreURL: creator.config.Store.URL, TemplateID: result.TemplateID, BuildID: result.BuildID,
		ImageRef: request.ImageRef, IdempotencyKey: request.IdempotencyKey, Metadata: result.Metadata,
	})
	if err != nil {
		creator.logger.Warn(ctx, "PostgreSQL 注册 SQL 生成失败，保留已发布 Template", zap.String("stage", "registration_sql_render_failed"), zap.String("template_id", result.TemplateID), zap.String("build_id", result.BuildID), zap.Error(err))
		return result
	}
	result.Registration = rendered.Result

	store, err := publisher.NewLocalStore(creator.config.Store.URL)
	if err == nil {
		err = store.WriteRegistrationSQL(ctx, result.BuildID, []byte(rendered.Contents))
	}
	if err != nil {
		creator.logger.Warn(ctx, "PostgreSQL 注册 SQL 写入失败，保留已发布 Template", zap.String("stage", "registration_sql_write_failed"), zap.String("template_id", result.TemplateID), zap.String("build_id", result.BuildID), zap.Error(err))
		return result
	}
	creator.logger.Info(ctx, "PostgreSQL 注册 SQL 写入完成", zap.String("stage", "registration_sql_written"), zap.String("template_id", result.TemplateID), zap.String("build_id", result.BuildID), zap.String("path", rendered.Result.LocalPath))
	return result
}

// writeTemplateInfo 保存与 CLI result 相同的 JSON，失败时只告警，不影响已发布 Template 的成功结果。
func (creator *Creator) writeTemplateInfo(ctx context.Context, result CreateResult) {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		creator.logger.Warn(ctx, "Template info.json 编码失败", zap.String("stage", "template_info_write_failed"), zap.String("template_id", result.TemplateID), zap.Error(err))
		return
	}
	data = append(data, '\n')

	store, err := publisher.NewLocalStore(creator.config.Store.URL)
	if err == nil {
		err = store.WriteTemplateInfo(ctx, result.TemplateID, data)
	}
	if err != nil {
		creator.logger.Warn(ctx, "Template info.json 写入失败，保留已发布结果", zap.String("stage", "template_info_write_failed"), zap.String("template_id", result.TemplateID), zap.Error(err))
		return
	}
	creator.logger.Info(ctx, "Template info.json 写入完成", zap.String("stage", "template_info_written"), zap.String("template_id", result.TemplateID))
}

// createTemplateOnce 执行一次真实构建、对象闭包解析、发布和临时目录清理。
func (creator *Creator) createTemplateOnce(ctx context.Context, request CreateRequest, idempotencyDigest string) (result CreateResult, returnErr error) {
	// 只有没有找到已完成任务结果、需要物理构建时才获取并发名额；等待期间仍响应 context 取消。
	creator.logger.Info(ctx, "等待物理构建并发名额", zap.String("stage", "build_slot_waiting"))
	select {
	case creator.semaphore <- struct{}{}:
		creator.logger.Info(ctx, "已取得物理构建并发名额", zap.String("stage", "build_slot_acquired"))
		defer func() { <-creator.semaphore }()
	case <-ctx.Done():
		creator.logger.Error(ctx, "等待物理构建并发名额失败", zap.String("stage", "build_slot_wait_failed"), zap.Error(ctx.Err()))
		return CreateResult{}, ctx.Err()
	}
	// 空槽位与取消可能同时就绪；取得槽位后再次检查，避免随机启动已取消的构建。
	if err := ctx.Err(); err != nil {
		return CreateResult{}, err
	}
	templateID, buildID := newBuildIdentifiers()
	creator.logger.Info(ctx, "已分配 Template 与 Build 标识",
		zap.String("stage", "identifiers_allocated"), zap.String("template_id", templateID), zap.String("build_id", buildID))
	jobRoot := filepath.Join(creator.config.ScratchRoot, "jobs", buildID)
	// 命名返回值允许 cleanup 失败使原本成功的结果失效；errors.Join 会同时保留
	// build/publish 根因与 cleanup 错误，避免因清理失败掩盖真正故障。
	defer func() {
		creator.logger.Info(ctx, "临时 job 清理开始", zap.String("stage", "job_cleanup_started"), zap.String("template_id", templateID), zap.String("build_id", buildID))
		if cleanupErr := creator.removeAll(jobRoot); cleanupErr != nil {
			result = CreateResult{}
			returnErr = errors.Join(returnErr, &BuildError{TemplateID: templateID, BuildID: buildID, Phase: BuildPhaseCleanup, Cause: cleanupErr})
			creator.logger.Error(ctx, "临时 job 清理失败", zap.String("stage", "job_cleanup_failed"), zap.String("template_id", templateID), zap.String("build_id", buildID), zap.Error(cleanupErr))
			return
		}
		creator.logger.Info(ctx, "临时 job 清理完成", zap.String("stage", "job_cleanup_completed"), zap.String("template_id", templateID), zap.String("build_id", buildID))
	}()
	outcome, err := creator.buildTemplate(ctx, request, templateID, buildID)
	if err != nil {
		return CreateResult{}, err
	}

	// final header 可能引用 ancestor build 的数据；必须先解析并验证完整闭包，
	// 之后才能发布任何 completion marker。
	creator.logger.Info(ctx, "Template 对象闭包解析开始", zap.String("stage", "closure_resolve_started"), zap.String("template_id", templateID), zap.String("build_id", buildID))
	publication := outcome.Metadata.Publication
	var closure publisher.Closure
	if publication == PublicationAll {
		closure, err = publisher.ResolveAll(outcome.TemplateRoot, buildID)
	} else {
		closure, err = publisher.ResolveClosure(outcome.TemplateRoot, buildID)
	}
	if err != nil {
		creator.logger.Error(ctx, "Template 对象闭包解析失败", zap.String("stage", "closure_resolve_failed"), zap.String("template_id", templateID), zap.String("build_id", buildID), zap.Error(err))
		return CreateResult{}, &BuildError{TemplateID: templateID, BuildID: buildID, Phase: BuildPhasePublishObjects, Cause: err}
	}
	creator.logger.Info(ctx, "Template 对象闭包解析完成", zap.String("stage", "closure_resolve_completed"), zap.String("template_id", templateID), zap.String("build_id", buildID), zap.Int("object_count", len(closure.Objects)), zap.Int("ancestor_count", len(closure.AncestorBuildIDs)))
	metadata, err := json.Marshal(outcome.Metadata)
	if err != nil {
		return CreateResult{}, &BuildError{TemplateID: templateID, BuildID: buildID, Phase: BuildPhasePublishManifest, Cause: fmt.Errorf("encode BuildMetadata: %w", err)}
	}
	store, err := publisher.NewLocalStore(creator.config.Store.URL)
	if err != nil {
		return CreateResult{}, &BuildError{TemplateID: templateID, BuildID: buildID, Phase: BuildPhasePublishObjects, Cause: err}
	}
	creator.logger.Info(ctx, "Template 对象发布开始", zap.String("stage", "publication_started"), zap.String("template_id", templateID), zap.String("build_id", buildID), zap.Int("object_count", len(closure.Objects)))
	location, err := store.Publish(ctx, publisher.PublishRequest{
		Objects: closure.Objects,
		Manifest: publisher.Manifest{
			SchemaVersion: publisher.ManifestSchemaVersion,
			TemplateID:    templateID, BuildID: buildID, ImageRef: request.ImageRef, ImageTarPath: request.ImageTarPath,
			Format: string(outcome.Metadata.Format), Publication: string(outcome.Metadata.Publication),
			AncestorBuildIDs: closure.AncestorBuildIDs, Metadata: metadata,
			InfraCommit: creator.config.Runtime.InfraCommit, BuiltAt: time.Now().UTC(),
			Labels: cloneLabels(request.Labels), IdempotencyDigest: idempotencyDigest,
		},
	})
	if err != nil {
		creator.logger.Error(ctx, "Template 对象发布失败", zap.String("stage", "publication_failed"), zap.String("template_id", templateID), zap.String("build_id", buildID), zap.Error(err))
		return CreateResult{}, &BuildError{TemplateID: templateID, BuildID: buildID, Phase: BuildPhasePublishObjects, Cause: err}
	}
	creator.logger.Info(ctx, "Template 对象与 completion manifest 发布完成", zap.String("stage", "publication_completed"), zap.String("template_id", templateID), zap.String("build_id", buildID), zap.String("manifest_key", location.ManifestKey))
	return CreateResult{
		TemplateID: templateID, BuildID: buildID,
		Artifacts: ArtifactLocation{StoreURL: location.StoreURL, ManifestKey: location.ManifestKey},
		Metadata:  outcome.Metadata,
	}, nil
}

// idempotencyDigest 仅摘要会改变 Template 产物的安全字段，用于区分同键重试与同键冲突。
func (creator *Creator) idempotencyDigest(request CreateRequest) (string, error) {
	if err := validateCreateRequest(request); err != nil {
		return "", err
	}
	spec, format, publication, err := resolveBuildOptions(creator.config.Defaults, creator.config.Format, creator.config.Publication, request.BuildOverride, request.Format, request.Publication)
	if err != nil {
		return "", fmt.Errorf("%w: BuildSpec: %v", ErrInvalidRequest, err)
	}
	// payload 只保留会影响产物的字段；RegistryAuth 被有意排除，确保 password/token
	// 既不会被散列，也不会进入任务完成记录。
	imageTarDigest := ""
	if request.ImageTarPath != "" {
		imageTarDigest, err = hashFileSHA256(request.ImageTarPath)
		if err != nil {
			return "", fmt.Errorf("hash ImageTarPath: %w", err)
		}
	}
	payload := struct {
		ImageRef       string            `json:"image_ref"`
		ImageTarPath   string            `json:"image_tar_path"`
		ImageTarSHA256 string            `json:"image_tar_sha256,omitempty"`
		Build          BuildSpec         `json:"build"`
		Format         BuildFormat       `json:"format"`
		Publication    PublicationMode   `json:"publication"`
		Labels         map[string]string `json:"labels,omitempty"`
		Runtime        struct {
			InfraCommit, TemplateVersion, Architecture, KernelVersion, FirecrackerVersion, EnvdVersion, BusyboxVersion string
		} `json:"runtime"`
	}{ImageRef: request.ImageRef, ImageTarPath: request.ImageTarPath, ImageTarSHA256: imageTarDigest, Build: spec, Format: format, Publication: publication, Labels: cloneLabels(request.Labels)}
	payload.Runtime.InfraCommit = creator.config.Runtime.InfraCommit
	payload.Runtime.TemplateVersion = creator.config.Runtime.TemplateVersion
	payload.Runtime.Architecture = creator.config.Runtime.Architecture
	payload.Runtime.KernelVersion = creator.config.Runtime.KernelVersion
	payload.Runtime.FirecrackerVersion = creator.config.Runtime.FirecrackerVersion
	payload.Runtime.EnvdVersion = creator.config.Runtime.EnvdVersion
	payload.Runtime.BusyboxVersion = creator.config.Runtime.BusyboxVersion
	data, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", sha256.Sum256(data)), nil
}

// hashFileSHA256 将本地 tar 内容纳入幂等摘要，避免同一路径替换镜像后错误复用旧结果。
func hashFileSHA256(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", hasher.Sum(nil)), nil
}

// newBuildIdentifiers 分别生成符合 E2B 公开格式的 Template ID 和可写入原生 header 的 UUID Build ID。
func newBuildIdentifiers() (string, string) {
	// Template ID 保留 E2B 公开格式；物理 build/layer ID 使用 UUID，
	// 因为原生 storage header 会把它编码为 uuid.UUID。
	return upstreamid.Generate(), uuid.NewString()
}

// cloneLabels 复制调用方 map，避免构建期间的外部修改造成 metadata 与任务内容摘要不一致。
func cloneLabels(labels map[string]string) map[string]string {
	if labels == nil {
		return nil
	}
	result := make(map[string]string, len(labels))
	for key, value := range labels {
		result[key] = value
	}
	return result
}
