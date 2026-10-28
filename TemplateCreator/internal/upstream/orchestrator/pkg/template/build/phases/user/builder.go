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

package user

import (
	"context"
	"fmt"

	"go.uber.org/zap/zapcore"

	"gitcode.com/boostkit/cloud-native.git/TemplateCreator/internal/upstream/orchestrator/pkg/proxy"
	"gitcode.com/boostkit/cloud-native.git/TemplateCreator/internal/upstream/orchestrator/pkg/sandbox"
	"gitcode.com/boostkit/cloud-native.git/TemplateCreator/internal/upstream/orchestrator/pkg/template/build/buildcontext"
	"gitcode.com/boostkit/cloud-native.git/TemplateCreator/internal/upstream/orchestrator/pkg/template/build/commands"
	"gitcode.com/boostkit/cloud-native.git/TemplateCreator/internal/upstream/orchestrator/pkg/template/build/layer"
	"gitcode.com/boostkit/cloud-native.git/TemplateCreator/internal/upstream/orchestrator/pkg/template/build/metrics"
	"gitcode.com/boostkit/cloud-native.git/TemplateCreator/internal/upstream/orchestrator/pkg/template/build/phases"
	"gitcode.com/boostkit/cloud-native.git/TemplateCreator/internal/upstream/orchestrator/pkg/template/build/phases/steps"
	"gitcode.com/boostkit/cloud-native.git/TemplateCreator/internal/upstream/orchestrator/pkg/template/build/storage/cache"
	template_manager "gitcode.com/boostkit/cloud-native.git/TemplateCreator/internal/upstream/shared/pkg/grpc/template-manager"
	"gitcode.com/boostkit/cloud-native.git/TemplateCreator/internal/upstream/shared/pkg/logger"
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
