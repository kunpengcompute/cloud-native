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

package commands

import (
	"context"
	"fmt"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap/zapcore"

	"gitcode.com/cloud-native/cloud-native/TemplateCreator/internal/upstream/orchestrator/pkg/proxy"
	"gitcode.com/cloud-native/cloud-native/TemplateCreator/internal/upstream/orchestrator/pkg/sandbox"
	"gitcode.com/cloud-native/cloud-native/TemplateCreator/internal/upstream/orchestrator/pkg/template/build/buildcontext"
	"gitcode.com/cloud-native/cloud-native/TemplateCreator/internal/upstream/orchestrator/pkg/template/metadata"
	templatemanager "gitcode.com/cloud-native/cloud-native/TemplateCreator/internal/upstream/shared/pkg/grpc/template-manager"
	"gitcode.com/cloud-native/cloud-native/TemplateCreator/internal/upstream/shared/pkg/logger"
	"gitcode.com/cloud-native/cloud-native/TemplateCreator/internal/upstream/shared/pkg/storage"
	"gitcode.com/cloud-native/cloud-native/TemplateCreator/internal/upstream/shared/pkg/utils"
)

var tracer = otel.Tracer("gitcode.com/cloud-native/cloud-native/TemplateCreator/internal/upstream/orchestrator/pkg/template/build/commands")

type CommandExecutor struct {
	buildcontext.BuildContext

	buildStorage storage.StorageProvider
	proxy        *proxy.SandboxProxy
}

func NewCommandExecutor(
	buildContext buildcontext.BuildContext,
	buildStorage storage.StorageProvider,
	proxy *proxy.SandboxProxy,
) *CommandExecutor {
	return &CommandExecutor{
		BuildContext: buildContext,

		buildStorage: buildStorage,
		proxy:        proxy,
	}
}

func (ce *CommandExecutor) getCommand(
	step *templatemanager.TemplateStep,
) (Command, error) {
	cmdType := strings.ToUpper(step.GetType())

	var cmd Command
	switch cmdType {
	case "ADD", "COPY":
		cmd = &Copy{
			FilesStorage: ce.buildStorage,
			CacheScope:   ce.CacheScope,
		}
	case "RUN":
		cmd = &Run{}
	case "USER":
		cmd = &User{}
	case "WORKDIR":
		cmd = &Workdir{}
	case "ENV", "ARG":
		cmd = &Env{}
	}

	if cmd == nil {
		return nil, fmt.Errorf("command type %s is not implemented", cmdType)
	}

	return cmd, nil
}

func (ce *CommandExecutor) Execute(
	ctx context.Context,
	userLogger logger.Logger,
	lvl zapcore.Level,
	sbx *sandbox.Sandbox,
	prefix string,
	step *templatemanager.TemplateStep,
	cmdMetadata metadata.Context,
) (metadata.Context, error) {
	ctx, span := tracer.Start(ctx, "apply-command", trace.WithAttributes(
		attribute.String("prefix", prefix),
		attribute.String("sandbox.id", sbx.Runtime.SandboxID),
		attribute.String("step.type", step.GetType()),
		attribute.StringSlice("step.args", step.GetArgs()),
		attribute.String("step.files.hash", utils.Sprintp(step.FilesHash)), //nolint:protogetter // we need the nil check too
	))
	defer span.End()

	cmd, err := ce.getCommand(step)
	if err != nil {
		return metadata.Context{}, fmt.Errorf("failed to get command for step %s: %w", step.GetType(), err)
	}

	cmdMetadata, err = cmd.Execute(
		ctx,
		userLogger,
		lvl,
		ce.proxy,
		sbx.Runtime.SandboxID,
		prefix,
		step,
		cmdMetadata,
	)
	if err != nil {
		return metadata.Context{}, err
	}

	return cmdMetadata, nil
}
