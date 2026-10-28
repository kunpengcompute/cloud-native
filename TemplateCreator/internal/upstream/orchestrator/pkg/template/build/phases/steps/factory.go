// TemplateCreator modification: upstream import paths rewritten; no behavioral changes.

package steps

import (
	"go.uber.org/zap/zapcore"

	"gitcode.com/cloud-native/cloud-native/TemplateCreator/internal/upstream/orchestrator/pkg/proxy"
	"gitcode.com/cloud-native/cloud-native/TemplateCreator/internal/upstream/orchestrator/pkg/sandbox"
	"gitcode.com/cloud-native/cloud-native/TemplateCreator/internal/upstream/orchestrator/pkg/template/build/buildcontext"
	"gitcode.com/cloud-native/cloud-native/TemplateCreator/internal/upstream/orchestrator/pkg/template/build/commands"
	"gitcode.com/cloud-native/cloud-native/TemplateCreator/internal/upstream/orchestrator/pkg/template/build/layer"
	"gitcode.com/cloud-native/cloud-native/TemplateCreator/internal/upstream/orchestrator/pkg/template/build/metrics"
	"gitcode.com/cloud-native/cloud-native/TemplateCreator/internal/upstream/orchestrator/pkg/template/build/phases"
	"gitcode.com/cloud-native/cloud-native/TemplateCreator/internal/upstream/orchestrator/pkg/template/build/storage/cache"
	"gitcode.com/cloud-native/cloud-native/TemplateCreator/internal/upstream/shared/pkg/logger"
)

const defaultLoggingLevel = zapcore.InfoLevel

func CreateStepPhases(
	bc buildcontext.BuildContext,
	sandboxFactory *sandbox.Factory,
	logger logger.Logger,
	proxy *proxy.SandboxProxy,
	layerExecutor *layer.LayerExecutor,
	commandExecutor *commands.CommandExecutor,
	index cache.Index,
	metrics *metrics.BuildMetrics,
) []phases.BuilderPhase {
	steps := make([]phases.BuilderPhase, 0, len(bc.Config.Steps))

	for i, step := range bc.Config.Steps {
		steps = append(steps,
			New(
				bc,
				sandboxFactory,
				logger,
				proxy,
				layerExecutor,
				commandExecutor,
				index,
				metrics,
				step,
				i+1, // stepNumber starts from 1
				defaultLoggingLevel,
			),
		)
	}

	return steps
}
