// TemplateCreator modification: upstream import paths rewritten; no behavioral changes.

package sbxlogger

import (
	"gitcode.com/cloud-native/cloud-native/TemplateCreator/internal/upstream/shared/pkg/logger"
)

var (
	sandboxLoggerInternal logger.Logger = logger.NewNopLogger()
	sandboxLoggerExternal logger.Logger = logger.NewNopLogger()
)

func SetSandboxLoggerInternal(logger logger.Logger) {
	sandboxLoggerInternal = logger
}

func SetSandboxLoggerExternal(logger logger.Logger) {
	sandboxLoggerExternal = logger
}

func I(m LoggerMetadata) *SandboxLogger {
	return &SandboxLogger{sandboxLoggerInternal.With(m.LoggerMetadata().Fields()...)}
}

func E(m LoggerMetadata) *SandboxLogger {
	return &SandboxLogger{sandboxLoggerExternal.With(m.LoggerMetadata().Fields()...)}
}
