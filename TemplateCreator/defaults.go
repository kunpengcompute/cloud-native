package templatecreator

import (
	"net/url"
	"time"
)

// BuildFormat 选择 SDK 构建语义：直接 from image，或兼容 only-FROM Dockerfile。
type BuildFormat string

const (
	BuildFormatFromImage      BuildFormat = "fromImage"
	BuildFormatFromDockerfile BuildFormat = "fromDockerfile"
)

// PublicationMode 选择只发布运行闭包，或保留 Builder staging 中的全部对象。
type PublicationMode string

const (
	PublicationSimple PublicationMode = "simple"
	PublicationAll    PublicationMode = "all"
)

const (
	DefaultImageRef        = "docker.io/library/debian:bookworm-slim"
	DefaultRuntimeInfoPath = "/opt/templatecreator/runtime-info.json"
	DefaultScratchRoot     = "/opt/templatecreator/scratch"
	DefaultStoreRoot       = "/opt/templatecreator/store"
	DefaultExportRoot      = ""
	DefaultBuildTimeout    = 30 * time.Minute
	DefaultVCPU            = int64(2)
	DefaultMemoryMB        = int64(1024)
	DefaultDiskSizeMB      = int64(4096)
	DefaultMaxParallel     = 1
	DefaultProxyPort       = uint16(5007)
	DefaultLogLevel        = LogLevelInfo
	DefaultBuildFormat     = BuildFormatFromImage
	DefaultPublicationMode = PublicationAll
)

// DefaultBuildSpec 返回集中维护的资源默认值；Steps 留空，由 format 在请求解析阶段决定。
func DefaultBuildSpec() BuildSpec {
	return BuildSpec{VCPU: DefaultVCPU, MemoryMB: DefaultMemoryMB, DiskSizeMB: DefaultDiskSizeMB}
}

// DefaultStoreURL 返回默认本地 Store 的 file URL。
func DefaultStoreURL() string {
	return (&url.URL{Scheme: "file", Path: DefaultStoreRoot}).String()
}

func validBuildFormat(format BuildFormat) bool {
	return format == BuildFormatFromImage || format == BuildFormatFromDockerfile
}

func validPublicationMode(mode PublicationMode) bool {
	return mode == PublicationSimple || mode == PublicationAll
}

func stepsForBuildFormat(format BuildFormat) []TemplateStep {
	if format == BuildFormatFromDockerfile {
		return defaultProvisioningSteps()
	}
	return []TemplateStep{}
}
