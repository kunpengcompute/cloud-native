// TemplateCreator modification: upstream import paths rewritten; no behavioral changes.

package user

import (
	"context"
	"fmt"

	"go.uber.org/zap/zapcore"

	"gitcode.com/cloud-native/cloud-native/TemplateCreator/internal/upstream/orchestrator/pkg/proxy"
	"gitcode.com/cloud-native/cloud-native/TemplateCreator/internal/upstream/orchestrator/pkg/sandbox"
	"gitcode.com/cloud-native/cloud-native/TemplateCreator/internal/upstream/orchestrator/pkg/template/build/buildcontext"
	"gitcode.com/cloud-native/cloud-native/TemplateCreator/internal/upstream/orchestrator/pkg/template/build/commands"
	"gitcode.com/cloud-native/cloud-native/TemplateCreator/internal/upstream/orchestrator/pkg/template/build/layer"
	"gitcode.com/cloud-native/cloud-native/TemplateCreator/internal/upstream/orchestrator/pkg/template/build/metrics"
	"gitcode.com/cloud-native/cloud-native/TemplateCreator/internal/upstream/orchestrator/pkg/template/build/phases"
	"gitcode.com/cloud-native/cloud-native/TemplateCreator/internal/upstream/orchestrator/pkg/template/build/phases/steps"
	"gitcode.com/cloud-native/cloud-native/TemplateCreator/internal/upstream/orchestrator/pkg/template/build/storage/cache"
	template_manager "gitcode.com/cloud-native/cloud-native/TemplateCreator/internal/upstream/shared/pkg/grpc/template-manager"
	"gitcode.com/cloud-native/cloud-native/TemplateCreator/internal/upstream/shared/pkg/logger"
)

type UserBuilder struct {
	*steps.StepBuilder

	user string
}

func New(
	buildContext buildcontext.BuildContext,
	sandboxFactory *sandbox.Factory,
	logger logger.Logger,
	proxy *proxy.SandboxProxy,
	layerExecutor *layer.LayerExecutor,
	commandExecutor *commands.CommandExecutor,
	index cache.Index,
	metrics *metrics.BuildMetrics,
	user string,
	force *bool,
) *UserBuilder {
	return &UserBuilder{
		StepBuilder: steps.New(
			buildContext,
			sandboxFactory,
			logger,
			proxy,
			layerExecutor,
			commandExecutor,
			index,
			metrics,
			&template_manager.TemplateStep{
				Type:  "USER",
				Args:  []string{user, "true"},
				Force: force,
			},
			// This step number shouldn't be used, but in case it does, defining as 1
			1,
			zapcore.DebugLevel,
		),
		user: user,
	}
}

func (ub *UserBuilder) Prefix() string {
	return "base"
}

func (ub *UserBuilder) String(_ context.Context) (string, error) {
	return fmt.Sprintf("%s %s", prefix, ub.user), nil
}

func (ub *UserBuilder) Metadata() phases.PhaseMeta {
	return phases.PhaseMeta{
		Phase:    metrics.PhaseBase,
		StepType: "base",
	}
}
