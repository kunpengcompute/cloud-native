// Copyright 2023 FoundryLabs, Inc.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
