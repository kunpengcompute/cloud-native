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

package buildcontext

import (
	"golang.org/x/sync/errgroup"

	"gitcode.com/boostkit/cloud-native.git/TemplateCreator/internal/upstream/orchestrator/pkg/cfg"
	"gitcode.com/boostkit/cloud-native.git/TemplateCreator/internal/upstream/orchestrator/pkg/template/build/config"
	"gitcode.com/boostkit/cloud-native.git/TemplateCreator/internal/upstream/shared/pkg/storage"
)

type BuildContext struct {
	BuilderConfig  cfg.BuilderConfig
	Config         config.TemplateConfig
	Template       storage.TemplateFiles
	UploadErrGroup *errgroup.Group
	EnvdVersion    string
	CacheScope     string
	IsV1Build      bool
	Version        string
}
