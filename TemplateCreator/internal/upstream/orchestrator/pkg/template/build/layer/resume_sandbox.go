// Copyright (c) 2023-2026 e2b Devs Inc.
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
