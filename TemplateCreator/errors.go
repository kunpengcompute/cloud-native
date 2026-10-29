package templatecreator

import (
	"fmt"
	"strings"
)

// BuildPhase 标识 Template 创建流程失败时所处的稳定阶段。
type BuildPhase string

const (
	// BuildPhaseValidate 表示公共配置或请求验证阶段。
	BuildPhaseValidate BuildPhase = "validate"
	// BuildPhasePrepare 表示准备 job 目录、runtime 或构建依赖的阶段。
	BuildPhasePrepare BuildPhase = "prepare"
	// BuildPhasePullImage 表示拉取 OCI 基础镜像的阶段。
	BuildPhasePullImage BuildPhase = "pull_image"
	// BuildPhaseBuild 表示真实 Builder 创建 rootfs 和 snapshot 的阶段。
	BuildPhaseBuild BuildPhase = "build"
	// BuildPhasePublishObjects 表示发布 Template 对象闭包的阶段。
	BuildPhasePublishObjects BuildPhase = "publish_objects"
	// BuildPhasePublishManifest 表示最后发布 completion manifest 的阶段。
	BuildPhasePublishManifest BuildPhase = "publish_manifest"
	// BuildPhaseCleanup 表示删除本次临时 job 数据的阶段。
	BuildPhaseCleanup BuildPhase = "cleanup"
)

// BuildError 为底层 cause 补充稳定的 Template/Build 标识和失败阶段。
// 该类型故意不保存完整请求或 registry credential，避免 error 被记录时泄露 secret。
type BuildError struct {
	// TemplateID 是已经分配的公开 Template 标识；分配前失败时可能为空。
	TemplateID string
	// BuildID 是已经分配的物理构建标识；分配前失败时可能为空。
	BuildID string
	// Phase 指出错误发生在哪个稳定流程阶段。
	Phase BuildPhase
	// DiagnosticsLocation 是可选的诊断产物位置，不包含诊断内容本身。
	DiagnosticsLocation string
	// Cause 保存可通过 errors.Is/errors.As 继续检查的原始错误。
	Cause error
}

// Error 实现 error，并且不会序列化请求或 credential 结构。
func (e *BuildError) Error() string {
	if e == nil {
		return "<nil>"
	}

	parts := []string{"template build failed"}
	if e.TemplateID != "" {
		parts = append(parts, "template="+e.TemplateID)
	}
	if e.BuildID != "" {
		parts = append(parts, "build="+e.BuildID)
	}
	if e.Phase != "" {
		parts = append(parts, "phase="+string(e.Phase))
	}
	message := strings.Join(parts, " ")
	if e.Cause != nil {
		message = fmt.Sprintf("%s: %v", message, e.Cause)
	}

	return message
}

// Unwrap 暴露原始 cause，供 errors.Is 和 errors.As 使用。
func (e *BuildError) Unwrap() error {
	if e == nil {
		return nil
	}

	return e.Cause
}
