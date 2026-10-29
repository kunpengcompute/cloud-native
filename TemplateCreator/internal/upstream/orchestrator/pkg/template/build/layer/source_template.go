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
	"fmt"

	sbxtemplate "gitcode.com/boostkit/cloud-native.git/TemplateCreator/internal/upstream/orchestrator/pkg/sandbox/template"
)

var _ SourceTemplateProvider = (*CacheSourceTemplateProvider)(nil)

type CacheSourceTemplateProvider struct {
	buildID string
}

func NewCacheSourceTemplateProvider(
	buildID string,
) *CacheSourceTemplateProvider {
	return &CacheSourceTemplateProvider{
		buildID: buildID,
	}
}

func (cstp *CacheSourceTemplateProvider) Get(ctx context.Context, templateCache *sbxtemplate.Cache) (sbxtemplate.Template, error) {
	template, err := templateCache.GetTemplate(
		ctx,
		cstp.buildID,
		false,
		true,
	)
	if err != nil {
		return nil, fmt.Errorf("get template snapshot: %w", err)
	}

	return template, nil
}

var _ SourceTemplateProvider = (*DirectSourceTemplateProvider)(nil)

type DirectSourceTemplateProvider struct {
	SourceTemplate sbxtemplate.Template
}

func NewDirectSourceTemplateProvider(template sbxtemplate.Template) *DirectSourceTemplateProvider {
	return &DirectSourceTemplateProvider{SourceTemplate: template}
}

func (dstp *DirectSourceTemplateProvider) Get(_ context.Context, _ *sbxtemplate.Cache) (sbxtemplate.Template, error) {
	return dstp.SourceTemplate, nil
}
