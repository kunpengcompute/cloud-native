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

package commands

import (
	"context"
	"fmt"

	"go.uber.org/zap/zapcore"

	"gitcode.com/cloud-native/cloud-native/TemplateCreator/internal/upstream/orchestrator/pkg/proxy"
	"gitcode.com/cloud-native/cloud-native/TemplateCreator/internal/upstream/orchestrator/pkg/template/build/sandboxtools"
	"gitcode.com/cloud-native/cloud-native/TemplateCreator/internal/upstream/orchestrator/pkg/template/metadata"
	templatemanager "gitcode.com/cloud-native/cloud-native/TemplateCreator/internal/upstream/shared/pkg/grpc/template-manager"
	"gitcode.com/cloud-native/cloud-native/TemplateCreator/internal/upstream/shared/pkg/logger"
)

type Run struct{}

var _ Command = (*Run)(nil)

func (r *Run) Execute(
	ctx context.Context,
	logger logger.Logger,
	lvl zapcore.Level,
	proxy *proxy.SandboxProxy,
	sandboxID string,
	prefix string,
	step *templatemanager.TemplateStep,
	cmdMetadata metadata.Context,
) (metadata.Context, error) {
	args := step.GetArgs()
	// args: [command optional_user]
	if len(args) < 1 {
		return metadata.Context{}, fmt.Errorf("RUN requires command argument")
	}

	originalMetadata := cmdMetadata

	// If a custom command user is specified, use it
	if len(args) >= 2 {
		cmdMetadata.User = args[1]
	}

	cmd := args[0]
	err := sandboxtools.RunCommandWithLogger(
		ctx,
		proxy,
		logger,
		lvl,
		prefix,
		sandboxID,
		cmd,
		cmdMetadata,
	)
	if err != nil {
		return metadata.Context{}, fmt.Errorf("failed to run command '%s': %w", cmd, err)
	}

	return originalMetadata, nil
}
