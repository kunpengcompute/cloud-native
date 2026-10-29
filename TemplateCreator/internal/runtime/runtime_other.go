//go:build !linux

package runtime

import (
	"context"
	"errors"
	"io"

	"gitcode.com/boostkit/cloud-native.git/TemplateCreator/internal/upstream/shared/pkg/logger"
)

// ErrUnsupportedPlatform 表示真实 TemplateCreator runtime 只能运行在 Linux。
var ErrUnsupportedPlatform = errors.New("TemplateCreator runtime requires Linux")

// ProcessConfig 在非 Linux build 中保留与 Linux 相同的 API shape，便于跨平台编译检查。
type ProcessConfig struct {
	ScratchRoot string
	BaseCache   BaseCacheConfig
	ProxyPort   uint16
	// NBDPoolSize 为跨平台配置 ABI 保留；Linux infra 2026.09 runtime 不使用该值。
	NBDPoolSize int
	NewSlots    int
	ReusedSlots int
	LogWriter   io.Writer
	DebugLogs   bool
	Logger      logger.Logger
}

// ProcessRuntime 是非 Linux build 的占位类型，不持有真实资源。
type ProcessRuntime struct{}

// NewProcessRuntime 在非 Linux 平台返回 ErrUnsupportedPlatform。
func NewProcessRuntime(context.Context, ProcessConfig) (*ProcessRuntime, error) {
	return nil, ErrUnsupportedPlatform
}

// Close 对未创建资源的占位 runtime 安全返回。
func (*ProcessRuntime) Close(context.Context) error { return nil }
