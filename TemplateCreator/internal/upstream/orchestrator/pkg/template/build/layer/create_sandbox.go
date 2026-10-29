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
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"gitcode.com/boostkit/cloud-native.git/TemplateCreator/internal/upstream/orchestrator/pkg/sandbox"
	"gitcode.com/boostkit/cloud-native.git/TemplateCreator/internal/upstream/orchestrator/pkg/sandbox/block"
	"gitcode.com/boostkit/cloud-native.git/TemplateCreator/internal/upstream/orchestrator/pkg/sandbox/fc"
	sbxtemplate "gitcode.com/boostkit/cloud-native.git/TemplateCreator/internal/upstream/orchestrator/pkg/sandbox/template"
	"gitcode.com/boostkit/cloud-native.git/TemplateCreator/internal/upstream/orchestrator/pkg/template/build/config"
	"gitcode.com/boostkit/cloud-native.git/TemplateCreator/internal/upstream/orchestrator/pkg/template/constants"
	"gitcode.com/boostkit/cloud-native.git/TemplateCreator/internal/upstream/shared/pkg/env"
	"gitcode.com/boostkit/cloud-native.git/TemplateCreator/internal/upstream/shared/pkg/fc/models"
	"gitcode.com/boostkit/cloud-native.git/TemplateCreator/internal/upstream/shared/pkg/id"
	"gitcode.com/boostkit/cloud-native.git/TemplateCreator/internal/upstream/shared/pkg/utils"
)

// CreateSandbox creates sandboxes for new templates
type CreateSandbox struct {
	config         sandbox.Config
	timeout        time.Duration
	sandboxFactory *sandbox.Factory

	rootfsCachePath string
	ioEngine        *string
}

const (
	minEnvdVersionForKVMClock = "0.2.11"                 // Minimum version of envd that supports KVM clock
	DefaultIoEngine           = models.DriveIoEngineSync // Use the Sync io engine by default to avoid issues with Async.
)

var _ SandboxCreator = (*CreateSandbox)(nil)

type createSandboxOptions struct {
	rootfsCachePath string
	ioEngine        *string
}

type CreateSandboxOption func(*createSandboxOptions)

func WithIoEngine(ioEngine string) CreateSandboxOption {
	return func(opts *createSandboxOptions) {
		opts.ioEngine = &ioEngine
	}
}

func WithRootfsCachePath(rootfsCachePath string) CreateSandboxOption {
	return func(opts *createSandboxOptions) {
		opts.rootfsCachePath = rootfsCachePath
	}
}

func NewCreateSandbox(config sandbox.Config, sandboxFactory *sandbox.Factory, timeout time.Duration, options ...CreateSandboxOption) *CreateSandbox {
	opts := &createSandboxOptions{
		rootfsCachePath: "",
		ioEngine:        utils.ToPtr(DefaultIoEngine),
	}
	for _, option := range options {
		option(opts)
	}

	return &CreateSandbox{
		config:          config,
		timeout:         timeout,
		rootfsCachePath: opts.rootfsCachePath,
		sandboxFactory:  sandboxFactory,
		ioEngine:        opts.ioEngine,
	}
}

func (cs *CreateSandbox) Sandbox(
	ctx context.Context,
	layerExecutor *LayerExecutor,
	sourceTemplate sbxtemplate.Template,
) (s *sandbox.Sandbox, err error) {
	// Create new memfile with the size of the sandbox RAM, this updates the underlying memfile.
	// This is ok as the sandbox is started from the beginning.
	memfile, err := block.NewEmpty(
		cs.config.RamMB<<constants.ToMBShift,
		config.MemfilePageSize(cs.config.HugePages),
		uuid.MustParse(sourceTemplate.Files().BuildID),
	)
	if err != nil {
		return nil, fmt.Errorf("create memfile: %w", err)
	}

	template := sbxtemplate.NewMaskTemplate(sourceTemplate, sbxtemplate.WithMemfile(memfile))

	kvmClock, err := utils.IsGTEVersion(cs.config.Envd.Version, minEnvdVersionForKVMClock)
	if err != nil {
		return nil, fmt.Errorf("error comparing envd version: %w", err)
	}

	// In case of a new sandbox, base template ID is now used as the potentially exported template base ID.
	sbx, err := cs.sandboxFactory.CreateSandbox(
		ctx,
		cs.config,
		sandbox.RuntimeMetadata{
			TemplateID:  layerExecutor.Config.TemplateID,
			SandboxID:   config.InstanceBuildPrefix + id.Generate(),
			ExecutionID: uuid.NewString(),
		},
		template,
		cs.timeout,
		cs.rootfsCachePath,
		fc.ProcessOptions{
			InitScriptPath:      constants.SystemdInitPath,
			KernelLogs:          env.IsDevelopment(),
			SystemdToKernelLogs: false,
			KvmClock:            kvmClock,
			IoEngine:            cs.ioEngine,
		},
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("create sandbox: %w", err)
	}
	defer func() {
		if err != nil {
			// Close the sandbox in case of error to avoid leaking resources
			err = errors.Join(err, sbx.Close(ctx))
		}
	}()

	err = sbx.WaitForEnvd(
		ctx,
		waitEnvdTimeout,
	)
	if err != nil {
		return nil, fmt.Errorf("wait for envd: %w", err)
	}

	return sbx, nil
}
