// TemplateCreator modification: upstream import paths rewritten; no behavioral changes.

package commands

import (
	"context"

	"go.uber.org/zap/zapcore"

	"gitcode.com/cloud-native/cloud-native/TemplateCreator/internal/upstream/orchestrator/pkg/proxy"
	"gitcode.com/cloud-native/cloud-native/TemplateCreator/internal/upstream/orchestrator/pkg/template/metadata"
	templatemanager "gitcode.com/cloud-native/cloud-native/TemplateCreator/internal/upstream/shared/pkg/grpc/template-manager"
	"gitcode.com/cloud-native/cloud-native/TemplateCreator/internal/upstream/shared/pkg/logger"
)

type Command interface {
	Execute(
		ctx context.Context,
		logger logger.Logger,
		lvl zapcore.Level,
		proxy *proxy.SandboxProxy,
		sandboxID string,
		prefix string,
		step *templatemanager.TemplateStep,
		cmdMetadata metadata.Context,
	) (metadata.Context, error)
}
