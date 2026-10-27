// TemplateCreator modification: upstream import paths rewritten; no behavioral changes.

package layer

import (
	"context"

	"gitcode.com/cloud-native/cloud-native/TemplateCreator/internal/upstream/orchestrator/pkg/sandbox"
	"gitcode.com/cloud-native/cloud-native/TemplateCreator/internal/upstream/orchestrator/pkg/template/metadata"
)

type FunctionActionFn func(ctx context.Context, sbx *sandbox.Sandbox, cmdMeta metadata.Template) (metadata.Template, error)

type FunctionAction struct {
	fn FunctionActionFn
}

func NewFunctionAction(fn FunctionActionFn) ActionExecutor {
	return &FunctionAction{fn: fn}
}

func (e *FunctionAction) Execute(ctx context.Context, sbx *sandbox.Sandbox, cmdMeta metadata.Template) (metadata.Template, error) {
	return e.fn(ctx, sbx, cmdMeta)
}
