package templatecreator

import (
	"context"
	"io"

	"gitcode.com/cloud-native/cloud-native/TemplateCreator/internal/upstream/shared/pkg/logger"
)

// LogLevel 控制 TemplateCreator 及其复用 infra Builder 的运行日志级别。
type LogLevel string

const (
	// LogLevelOff 关闭可配置运行日志。
	LogLevelOff LogLevel = "off"
	// LogLevelInfo 输出主要构建阶段和 info 及以上级别日志。
	LogLevelInfo LogLevel = "info"
	// LogLevelDebug 在 info 基础上输出 debug 诊断日志。
	LogLevelDebug LogLevel = "debug"
)

// CreatorConfig 配置由一个 Creator 实例持有、供多个 Template 构建共享的进程级资源。
type CreatorConfig struct {
	// ScratchRoot 是临时构建数据的绝对根目录；每个物理构建在其下使用隔离的 job 目录。
	ScratchRoot string
	// Store 指定最终产物的持久化位置，不用于存放临时构建数据。
	Store StoreConfig
	// Runtime 固定本进程所有构建使用的 Builder runtime ABI。
	Runtime RuntimeConfig
	// Cache 控制可选的单机持久化共享 base-layer cache；零值保持原有 job-local 行为。
	Cache CacheConfig
	// Defaults 是每次构建的默认资源和命令配置，CreateRequest.BuildOverride 可按字段覆盖。
	// Steps 为 nil 时，Creator 自动加入 E2B 默认 USER/WORKDIR 上下文步骤；非 nil 时使用调用方值。
	Defaults BuildSpec
	// Format 是未在 CreateRequest 中指定时采用的构建格式；零值使用集中默认值。
	Format BuildFormat
	// Publication 是未在 CreateRequest 中指定时采用的发布模式；零值使用集中默认值。
	Publication PublicationMode
	// MaxParallel 限制当前 Creator 同时执行的物理构建数；0 会归一化为 1。
	MaxParallel int
	// LogWriter 接收 TemplateCreator 及其复用 infra Builder 的运行日志；nil 表示不输出日志。
	// CLI 应传入 stderr，避免污染 stdout 的最终 JSON。
	LogWriter io.Writer
	// LogLevel 显式选择 off、info 或 debug；零值兼容旧 LogWriter/DebugLogs 配置。
	LogLevel LogLevel
	// DebugLogs 控制是否额外输出 debug 级别日志。
	// Deprecated: 请改用 LogLevel；仅在 LogLevel 为零值时保留兼容行为。
	DebugLogs bool
	// progressLogger 由 NewCreator 在内部创建并传给 ProcessRuntime，确保组件共享同一日志并发锁。
	progressLogger logger.Logger
}

// CacheConfig 控制同一台构建机上跨进程、跨物理构建复用的 base-layer cache。
// Cache 仅用于加速构建，最终交付给 E2B 的 Template 仍必须完整发布到 Store。
type CacheConfig struct {
	// Enabled 显式启用共享 cache；false 时 Root 不参与构建。
	Enabled bool
	// Root 是持久 cache 的绝对根目录，启用时必填。
	Root string
	// AllowMutableImageTags 允许 tag 形式的 ImageRef 进入共享 cache。
	// 默认 false，避免 tag 指向新镜像后静默复用旧 base layer。
	AllowMutableImageTags bool
}

// StoreConfig 选择最终 Artifact Store；当前 Demo 仅支持 file URL。
type StoreConfig struct {
	// URL 是 Artifact Store 的绝对 file URL，例如 file:///opt/templatecreator/store。
	URL string
}

// RuntimeConfig 固定构建与恢复 E2B snapshot 所依赖的 Builder runtime ABI。
type RuntimeConfig struct {
	// RuntimeDir 是 kernels、firecrackers、envd 和 busybox 等资产采用 Builder 固定布局的绝对根目录。
	RuntimeDir string
	// InfraCommit 标识提取 Builder 实现所对应的 e2b-infra commit，并随构建 metadata 发布。
	InfraCommit string
	// TemplateVersion 是传给 Builder 的 Template 配置版本。
	TemplateVersion string
	// Architecture 是 runtime 资产及产物面向的 CPU 架构，例如 amd64 或 arm64。
	Architecture string
	// KernelVersion 是 Builder 选择 guest kernel 时使用的固定版本。
	KernelVersion string
	// KernelSHA256 是对应 guest kernel 的十六进制 SHA-256。
	KernelSHA256 string
	// FirecrackerVersion 是创建 snapshot 的 Firecracker 固定版本。
	FirecrackerVersion string
	// FirecrackerSHA256 是对应 Firecracker binary 的十六进制 SHA-256。
	FirecrackerSHA256 string
	// EnvdVersion 是写入 guest 并参与 snapshot ABI 校验的 envd 固定版本。
	EnvdVersion string
	// EnvdPath 是与 EnvdVersion 对应的 envd 可执行文件绝对路径。
	EnvdPath string
	// EnvdSHA256 是 EnvdPath 对应文件的十六进制 SHA-256。
	EnvdSHA256 string
	// BusyboxDir 是包含按版本和架构组织的 BusyBox 资产的绝对根目录。
	BusyboxDir string
	// BusyboxVersion 是 Builder 查找 BusyBox 资产时使用的固定版本。
	BusyboxVersion string
	// BusyboxSHA256 是对应 BusyBox binary 的十六进制 SHA-256。
	BusyboxSHA256 string
	// ProxyPort 是进程级 sandbox proxy 监听的本地端口；0 由 runtime 使用默认端口。
	ProxyPort uint16
}

// BuildSpec 描述一次 Template 构建的资源、启动命令和 provisioning steps。
// 作为 BuildOverride 使用时，数值 0、空字符串和 nil Steps 表示继承 Creator 默认值；
// 非 nil override 中的 HugePages 始终显式覆盖默认值，空但非 nil 的 Steps 可清空默认步骤。
type BuildSpec struct {
	// VCPU 是分配给 guest 的虚拟 CPU 数量。
	VCPU int64
	// MemoryMB 是 guest 内存大小，单位为 MiB。
	MemoryMB int64
	// DiskSizeMB 是 guest 根文件系统容量，单位为 MiB。
	DiskSizeMB int64
	// HugePages 指示是否使用 host HugePages 承载 guest 内存。
	HugePages bool
	// StartCommand 是由 guest 内 envd 启动的可选命令。
	StartCommand string
	// ReadyCommand 是用于判断 guest 是否就绪的可选命令。
	ReadyCommand string
	// Steps 是按顺序交给 Builder 执行的 Template provisioning steps。
	Steps []TemplateStep
}

// TemplateStep 描述一个稳定的公开 provisioning step，避免向调用方暴露 upstream 类型。
type TemplateStep struct {
	// Type 是 Builder 识别的 step 类型。
	Type string
	// Args 是按原顺序传给该 step 的参数。
	Args []string
}

// RegistryCredentials 保存 RegistryAuthProvider 为单次物理构建解析出的短期凭据。
type RegistryCredentials struct {
	// Username 是私有 OCI registry 的用户名。
	Username string
	// Password 是私有 OCI registry 的密码或 token；TemplateCreator 不会将其写入磁盘、metadata 或任务内容摘要。
	Password string
}

// RegistryAuthProvider 为私有 OCI 镜像按需解析凭据。
// 实现不得记录或持久化返回的 secret。
type RegistryAuthProvider interface {
	// Credentials 返回拉取 imageRef 所需的临时凭据。
	Credentials(ctx context.Context, imageRef string) (RegistryCredentials, error)
}

// CreateRequest 描述一次同步 Template 构建及发布请求。
type CreateRequest struct {
	// ImageRef 是作为 Template 基础的 OCI 镜像引用。
	ImageRef string
	// ImageTarPath 是 docker save 导出的本地镜像 tar 文件路径；与 ImageRef 互斥。
	ImageTarPath string
	// BuildOverride 按 BuildSpec 的继承规则覆盖 CreatorConfig.Defaults；nil 表示完全使用默认值。
	BuildOverride *BuildSpec
	// Format 覆盖 CreatorConfig.Format；零值继承 Creator 默认值。
	Format BuildFormat
	// Publication 覆盖 CreatorConfig.Publication；零值继承 Creator 默认值。
	Publication PublicationMode
	// RegistryAuth 是拉取私有镜像时使用的可选凭据提供者，不参与任务内容摘要且不会持久化。
	RegistryAuth RegistryAuthProvider
	// IdempotencyKey 是可选的稳定任务编号；相同编号和相同请求会复用已发布结果，相同编号但请求不同会返回冲突。
	IdempotencyKey string
	// Labels 是复制到最终 manifest 的调用方标签，并作为安全请求摘要的一部分参与任务内容比较。
	Labels map[string]string
	// BypassBaseCache 强制本次物理构建不读取旧 base layer，但成功后仍可更新共享 cache。
	// 相同任务编号对应的成功结果仍会优先复用；调用方需要物理重建时应同时使用新的任务编号。
	BypassBaseCache bool
}

// CreateResult 仅在全部对象及最终 completion manifest 成功发布后返回。
type CreateResult struct {
	// TemplateID 是对外稳定的 E2B Template 标识符。
	TemplateID string
	// BuildID 是本次物理构建及其对象闭包的标识符。
	BuildID string
	// Artifacts 定位已经完整发布的 completion manifest。
	Artifacts ArtifactLocation
	// Metadata 记录本次构建实际使用的源版本、runtime ABI 和资源参数。
	Metadata BuildMetadata
	// Registration 返回 PostgreSQL 注册辅助文件的位置和执行前提示。
	// SQL 正文写入 final Build 目录但不进入公开结果或 completion manifest。
	Registration RegistrationSQLResult
}

// RegistrationSQLResult 描述随 Template 结果生成的 PostgreSQL 注册辅助文件。
// 该 SQL 只生成、不执行；目标 E2B 的 team、alias 和 cache 仍由管理员确认。
type RegistrationSQLResult struct {
	// ObjectKey 是 Local Store 和未来 S3 uploader 使用的 slash key；它不属于 completion manifest。
	ObjectKey string
	// LocalPath 是当前 file:// Store 中便于直接查看和执行的绝对路径。
	LocalPath string
	// Warnings 是执行前必须展示给用户的目标环境检查提示。
	Warnings []string
}

// ArtifactLocation 定位一个已完整发布、可作为完成标记读取的 Template manifest。
type ArtifactLocation struct {
	// StoreURL 是包含构建对象和 manifest 的 Artifact Store URL。
	StoreURL string
	// ManifestKey 是 StoreURL 内最终 manifest 的对象 key。
	ManifestKey string
}

// BuildMetadata 记录一次构建实际使用的源版本、runtime ABI、rootfs 和资源配置。
type BuildMetadata struct {
	// Format 记录本次请求实际采用的构建格式。
	Format BuildFormat `json:"format"`
	// Publication 记录本次请求实际采用的发布模式。
	Publication PublicationMode `json:"publication"`
	// InfraCommit 是本次 Builder 实现对应的 e2b-infra commit。
	InfraCommit string
	// TemplateVersion 是本次构建传给 Builder 的 Template 配置版本。
	TemplateVersion string
	// Architecture 是本次构建目标 runtime 的 CPU 架构。
	Architecture string
	// EnvdVersion 是 Builder 报告的实际 envd 版本。
	EnvdVersion string
	// KernelVersion 是 Builder 报告的实际 guest kernel 版本。
	KernelVersion string
	// FirecrackerVersion 是 Builder 报告的实际 Firecracker 版本。
	FirecrackerVersion string
	// RootfsSizeMB 是 Builder 生成的 rootfs 实际大小，单位为 MiB。
	RootfsSizeMB int64
	// Resources 是解析继承规则后实际用于构建的完整 BuildSpec。
	Resources BuildSpec
	// Scheduling 是供调用方稳定消费的调度资源摘要。
	Scheduling SchedulingMetadata
	// MachineInfo 是创建 Firecracker snapshot 的宿主机 CPU 事实。
	// 目标 E2B 应使用这些字段调度兼容节点，不能从任意参考 Build 复制或伪造。
	MachineInfo MachineInfo `json:"machine_info"`
}

// MachineInfo 与 e2b-infra env_builds 使用相同的 CPU 字段语义。
// 它描述构建机，不单独构成目标节点兼容性证明。
type MachineInfo struct {
	CPUArchitecture string   `json:"cpu_architecture"`
	CPUFamily       string   `json:"cpu_family"`
	CPUModel        string   `json:"cpu_model"`
	CPUModelName    string   `json:"cpu_model_name"`
	CPUFlags        []string `json:"cpu_flags"`
}

// SchedulingMetadata 包含调用方可稳定消费、用于调度 Template 的资源事实。
type SchedulingMetadata struct {
	// VCPU 是 guest 所需的虚拟 CPU 数量。
	VCPU int64
	// MemoryMB 是 guest 所需内存，单位为 MiB。
	MemoryMB int64
	// DiskSizeMB 是 guest 根文件系统容量，单位为 MiB。
	DiskSizeMB int64
}
