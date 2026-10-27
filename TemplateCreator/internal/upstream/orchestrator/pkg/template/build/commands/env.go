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
	"maps"
	"strings"

	"go.uber.org/zap/zapcore"

	"gitcode.com/cloud-native/cloud-native/TemplateCreator/internal/upstream/orchestrator/pkg/proxy"
	"gitcode.com/cloud-native/cloud-native/TemplateCreator/internal/upstream/orchestrator/pkg/template/build/sandboxtools"
	"gitcode.com/cloud-native/cloud-native/TemplateCreator/internal/upstream/orchestrator/pkg/template/metadata"
	templatemanager "gitcode.com/cloud-native/cloud-native/TemplateCreator/internal/upstream/shared/pkg/grpc/template-manager"
	"gitcode.com/cloud-native/cloud-native/TemplateCreator/internal/upstream/shared/pkg/logger"
)

type Env struct{}

var _ Command = (*Env)(nil)

func (e *Env) Execute(
	ctx context.Context,
	_ logger.Logger,
	_ zapcore.Level,
	proxy *proxy.SandboxProxy,
	sandboxID string,
	_ string,
	step *templatemanager.TemplateStep,
	cmdMetadata metadata.Context,
) (metadata.Context, error) {
	cmdType := strings.ToUpper(step.GetType())
	args := step.GetArgs()
	// args: [key1 value1 key2 value2 ...]
	if len(args) == 0 {
		return metadata.Context{}, fmt.Errorf("%s does not support passing no arguments", cmdType)
	}

	if len(args)%2 != 0 {
		return metadata.Context{}, fmt.Errorf("%s requires both a key and value arguments", cmdType)
	}

	envVars := maps.Clone(cmdMetadata.EnvVars)
	for i := 0; i < len(args)-1; i += 2 {
		k := args[i]
		v, err := evaluateValue(ctx, proxy, sandboxID, args[i+1])
		if err != nil {
			return metadata.Context{}, fmt.Errorf("failed to evaluate environment variable %s: %w", k, err)
		}

		envVars[k] = v
	}

	cmdMetadata.EnvVars = envVars

	return cmdMetadata, nil
}

func evaluateValue(
	ctx context.Context,
	proxy *proxy.SandboxProxy,
	sandboxID string,
	envValue string,
) (string, error) {
	// Escape characters that would break parsing or cause command execution.
	// $VAR expansion is preserved for environment variable interpolation.
	escaped := envValue
	escaped = strings.ReplaceAll(escaped, `\`, `\\`)
	escaped = strings.ReplaceAll(escaped, `"`, `\"`)
	escaped = strings.ReplaceAll(escaped, "`", "\\`")
	escaped = strings.ReplaceAll(escaped, "$(", "\\$(")

	cmd := fmt.Sprintf(`printf "%%s" "%s"`, escaped)

	err := sandboxtools.RunCommandWithOutput(
		ctx,
		proxy,
		sandboxID,
		cmd,
		metadata.Context{
			User: "root",
		},
		func(stdout, _ string) {
			envValue = stdout
		},
	)
	if err != nil {
		return "", err
	}

	return envValue, nil
}
