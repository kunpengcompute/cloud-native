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
	"time"

	"gitcode.com/boostkit/cloud-native.git/TemplateCreator/internal/upstream/orchestrator/pkg/sandbox"
	sbxtemplate "gitcode.com/boostkit/cloud-native.git/TemplateCreator/internal/upstream/orchestrator/pkg/sandbox/template"
	"gitcode.com/boostkit/cloud-native.git/TemplateCreator/internal/upstream/orchestrator/pkg/template/metadata"
)

const (
	waitEnvdTimeout = 60 * time.Second
)

// SandboxCreator creates sandboxes for layer building
type SandboxCreator interface {
	Sandbox(
		ctx context.Context,
		layerExecutor *LayerExecutor,
		template sbxtemplate.Template,
	) (*sandbox.Sandbox, error)
}

// ActionExecutor executes actions within a sandbox during layer building
type ActionExecutor interface {
	Execute(ctx context.Context, sbx *sandbox.Sandbox, meta metadata.Template) (metadata.Template, error)
}

// SourceTemplateProvider provides the source template for the layer build
type SourceTemplateProvider interface {
	Get(ctx context.Context, templateCache *sbxtemplate.Cache) (sbxtemplate.Template, error)
}

// LayerBuildCommand encapsulates all parameters needed for building a layer
type LayerBuildCommand struct {
	SourceTemplate SourceTemplateProvider
	CurrentLayer   metadata.Template
	Hash           string
	UpdateEnvd     bool
	SandboxCreator SandboxCreator
	ActionExecutor ActionExecutor
}
