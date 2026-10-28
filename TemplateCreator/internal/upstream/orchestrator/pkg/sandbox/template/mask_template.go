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

package template

import (
	"context"

	"gitcode.com/cloud-native/cloud-native/TemplateCreator/internal/upstream/orchestrator/pkg/sandbox/block"
	"gitcode.com/cloud-native/cloud-native/TemplateCreator/internal/upstream/orchestrator/pkg/template/metadata"
	"gitcode.com/cloud-native/cloud-native/TemplateCreator/internal/upstream/shared/pkg/storage"
)

type MaskTemplate struct {
	template Template

	memfile *block.ReadonlyDevice
}

type MaskTemplateOption func(*MaskTemplate)

func WithMemfile(memfile block.ReadonlyDevice) MaskTemplateOption {
	return func(c *MaskTemplate) {
		c.memfile = &memfile
	}
}

func NewMaskTemplate(
	template Template,
	opts ...MaskTemplateOption,
) *MaskTemplate {
	t := &MaskTemplate{
		template: template,
		memfile:  nil,
	}

	for _, opt := range opts {
		opt(t)
	}

	return t
}

func (c *MaskTemplate) Close(_ context.Context) error {
	if c.memfile != nil {
		return (*c.memfile).Close()
	}

	return nil
}

func (c *MaskTemplate) Files() storage.TemplateCacheFiles {
	return c.template.Files()
}

func (c *MaskTemplate) Memfile(ctx context.Context) (block.ReadonlyDevice, error) {
	ctx, span := tracer.Start(ctx, "mask-template-memfile")
	defer span.End()

	if c.memfile != nil {
		return *c.memfile, nil
	}

	return c.template.Memfile(ctx)
}

func (c *MaskTemplate) Rootfs() (block.ReadonlyDevice, error) {
	return c.template.Rootfs()
}

func (c *MaskTemplate) Snapfile() (File, error) {
	return c.template.Snapfile()
}

func (c *MaskTemplate) Metadata() (metadata.Template, error) {
	return c.template.Metadata()
}
