// TemplateCreator modification: upstream import paths rewritten; no behavioral changes.

package finalize

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	tt "text/template"
	"time"

	"go.uber.org/zap/zapcore"

	"gitcode.com/cloud-native/cloud-native/TemplateCreator/internal/upstream/orchestrator/pkg/proxy"
	"gitcode.com/cloud-native/cloud-native/TemplateCreator/internal/upstream/orchestrator/pkg/template/build/buildcontext"
	"gitcode.com/cloud-native/cloud-native/TemplateCreator/internal/upstream/orchestrator/pkg/template/build/sandboxtools"
	"gitcode.com/cloud-native/cloud-native/TemplateCreator/internal/upstream/orchestrator/pkg/template/metadata"
	"gitcode.com/cloud-native/cloud-native/TemplateCreator/internal/upstream/shared/pkg/logger"
)

const configurationTimeout = 5 * time.Minute

//go:embed configure.sh
var configureScriptFile string
var ConfigureScriptTemplate = tt.Must(tt.New("provisioning-finish-script").Parse(configureScriptFile))

type ConfigurationParams struct {
	EnvID      string
	TemplateID string
	BuildID    string
}

func runConfiguration(
	ctx context.Context,
	userLogger logger.Logger,
	bc buildcontext.BuildContext,
	proxy *proxy.SandboxProxy,
	sandboxID string,
) error {
	ctx, span := tracer.Start(ctx, "run configuration")
	defer span.End()

	// Run configuration script
	var scriptDef bytes.Buffer
	err := ConfigureScriptTemplate.Execute(&scriptDef, ConfigurationParams{
		EnvID:      bc.Config.TemplateID,
		TemplateID: bc.Config.TemplateID,
		BuildID:    bc.Template.BuildID,
	})
	if err != nil {
		return fmt.Errorf("error executing provision script: %w", err)
	}

	err = sandboxtools.RunCommandWithLogger(
		ctx,
		proxy,
		userLogger,
		zapcore.DebugLevel,
		"config",
		sandboxID,
		scriptDef.String(),
		metadata.Context{
			User: "root",
		},
	)
	if err != nil {
		return fmt.Errorf("error running configuration script: %w", err)
	}

	return nil
}
