package runtime

import (
	"context"
	"io"

	"go.uber.org/zap/zapcore"

	"gitcode.com/boostkit/cloud-native.git/TemplateCreator/internal/upstream/shared/pkg/logger"
)

// NewLogger 通过 infra 既有 LoggerConfig.Cores 扩展点创建 TemplateCreator 日志器。
// writer 为 nil 时日志保持静默；该函数不会启用 infra 的 stdout console，避免污染 CLI JSON。
func NewLogger(writer io.Writer, debug bool) (logger.Logger, error) {
	if writer == nil {
		writer = io.Discard
	}

	encoderConfig := logger.GetConsoleEncoderConfig()
	// 日志可能被重定向到文件或采集系统，禁用 ANSI 颜色以保持内容稳定可读。
	encoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder
	level := zapcore.InfoLevel
	if debug {
		level = zapcore.DebugLevel
	}
	core := zapcore.NewCore(
		zapcore.NewConsoleEncoder(encoderConfig),
		zapcore.Lock(zapcore.AddSync(writer)),
		level,
	)

	return logger.NewLogger(context.Background(), logger.LoggerConfig{
		ServiceName:       "templatecreator",
		IsInternal:        true,
		IsDebug:           debug,
		DisableStacktrace: true,
		Cores:             []zapcore.Core{core},
	})
}

// newProcessLogger 保留进程 runtime 内部命名，并统一复用公共日志构造逻辑。
func newProcessLogger(writer io.Writer, debug bool) (logger.Logger, error) {
	return NewLogger(writer, debug)
}
