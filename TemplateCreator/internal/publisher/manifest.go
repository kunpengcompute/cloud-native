// Package publisher 解析完整 E2B 对象闭包，并以原子 manifest 作为 Local Store 完成标记。
package publisher

import (
	"encoding/json"
	"time"
)

// ManifestSchemaVersion 是 TemplateCreator completion manifest 的首个 schema 版本。
const ManifestSchemaVersion = 1

// Manifest 只在所有引用的 Template 对象均已持久化后发布。
type Manifest struct {
	// SchemaVersion 指定调用方应采用的 manifest schema。
	SchemaVersion int `json:"schema_version"`
	// TemplateID/BuildID 标识公开 Template 和最终物理构建。
	TemplateID string `json:"template_id"`
	BuildID    string `json:"build_id"`
	// ImageRef 记录本次 Template 的 OCI 基础镜像。
	ImageRef string `json:"image_ref"`
	// ImageTarPath 记录本次 Template 使用的本地 docker save 归档（与 ImageRef 互斥）。
	ImageTarPath string `json:"image_tar_path,omitempty"`
	// Format 记录本次构建采用的 SDK 语义。
	Format string `json:"format"`
	// Publication 记录对象发布策略。
	Publication string `json:"publication"`
	// Objects 是完整闭包中每个对象的 key、size 和 checksum。
	Objects []ObjectRecord `json:"objects"`
	// AncestorBuildIDs 列出 final header 间接依赖的历史 build。
	AncestorBuildIDs []string `json:"ancestor_build_ids,omitempty"`
	// Metadata 是公开 BuildMetadata 的原始 JSON，避免 internal package 反向依赖根 package。
	Metadata json.RawMessage `json:"build_metadata"`
	// InfraCommit 记录生成这些 E2B-native 对象的 Builder 源码版本。
	InfraCommit string `json:"infra_commit"`
	// BuiltAt 是 manifest 最终发布的 UTC 时间。
	BuiltAt time.Time `json:"built_at"`
	// Labels 是调用方提供并复制后的非 secret 标签。
	Labels map[string]string `json:"labels,omitempty"`
	// IdempotencyDigest 用于审计完成结果对应的安全请求摘要。
	IdempotencyDigest string `json:"idempotency_digest,omitempty"`
}

// ObjectRecord 描述发布闭包中的一个 E2B-native storage 对象。
type ObjectRecord struct {
	// Key 保留 E2B 原生 slash-separated object key。
	Key string `json:"key"`
	// Size 是实际持久化字节数。
	Size int64 `json:"size"`
	// SHA256 是实际持久化内容的十六进制摘要。
	SHA256 string `json:"sha256"`
}

// ObjectSource 把 E2B-native key 映射到一个本地 staging 文件。
type ObjectSource struct {
	// Key 是发布后保留的对象 key。
	Key string
	// Path 是构建 staging 区中的绝对源文件路径。
	Path string
}

// PublishRequest 包含待发布的完整对象闭包和 manifest metadata。
type PublishRequest struct {
	// Manifest 在 Publish 内补齐 Objects 后最后原子写入。
	Manifest Manifest
	// Objects 必须先于 manifest 全部写入成功。
	Objects []ObjectSource
}

// PublishLocation 定位最终 completion manifest。
type PublishLocation struct {
	// StoreURL 是持久化根位置。
	StoreURL string
	// ManifestKey 是 StoreURL 下的最终 manifest key。
	ManifestKey string
}
