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

package steps

import (
	"context"
	"strings"

	"gitcode.com/boostkit/cloud-native.git/TemplateCreator/internal/upstream/orchestrator/pkg/template/build/phases"
	"gitcode.com/boostkit/cloud-native.git/TemplateCreator/internal/upstream/orchestrator/pkg/template/build/storage/cache"
	"gitcode.com/boostkit/cloud-native.git/TemplateCreator/internal/upstream/shared/pkg/utils"
)

func (sb *StepBuilder) Hash(_ context.Context, sourceLayer phases.LayerResult) (string, error) {
	return cache.HashKeys(
		sourceLayer.Hash,
		sb.step.GetType(),
		strings.Join(sb.step.GetArgs(), " "),
		utils.Sprintp(sb.step.FilesHash), //nolint:protogetter // we need the nil check too
	), nil
}
