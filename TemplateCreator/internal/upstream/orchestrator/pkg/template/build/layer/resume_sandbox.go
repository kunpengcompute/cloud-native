// TemplateCreator modification: upstream import paths rewritten; no behavioral changes.

package layer

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"gitcode.com/cloud-native/cloud-native/TemplateCreator/internal/upstream/orchestrator/pkg/sandbox"
	sbxtemplate "gitcode.com/cloud-native/cloud-native/TemplateCreator/internal/upstream/orchestrator/pkg/sandbox/template"
	"gitcode.com/cloud-native/cloud-native/TemplateCreator/internal/upstream/orchestrator/pkg/template/build/config"
	"gitcode.com/cloud-native/cloud-native/TemplateCreator/internal/upstream/shared/pkg/id"
)

// ResumeSandbox creates sandboxes for resuming existing templates
type ResumeSandbox struct {
	config         sandbox.Config
	sandboxFactory *sandbox.Factory
	timeout        time.Duration
}

var _ SandboxCreator = (*ResumeSandbox)(nil)

func NewResumeSandbox(config sandbox.Config, sandboxFactory *sandbox.Factory, timeout time.Duration) *ResumeSandbox {
	return &ResumeSandbox{config: config, sandboxFactory: sandboxFactory, timeout: timeout}
}

func (rs *ResumeSandbox) Sandbox(
	ctx context.Context,
	layerExecutor *LayerExecutor,
	template sbxtemplate.Template,
) (*sandbox.Sandbox, error) {
	sbx, err := rs.sandboxFactory.ResumeSandbox(
		ctx,
		template,
		rs.config,
		sandbox.RuntimeMetadata{
			TemplateID:  layerExecutor.Config.TemplateID,
			SandboxID:   config.InstanceBuildPrefix + id.Generate(),
			ExecutionID: uuid.NewString(),
		},
		time.Now(),
		time.Now().Add(rs.timeout),
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("resume sandbox: %w", err)
	}

	return sbx, nil
}
