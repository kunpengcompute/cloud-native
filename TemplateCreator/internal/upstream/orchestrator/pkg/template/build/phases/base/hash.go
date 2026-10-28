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

package base

import (
	"context"
	"fmt"
	"strconv"

	"go.opentelemetry.io/otel/attribute"

	"gitcode.com/cloud-native/cloud-native/TemplateCreator/internal/upstream/orchestrator/pkg/template/build/phases"
	"gitcode.com/cloud-native/cloud-native/TemplateCreator/internal/upstream/orchestrator/pkg/template/build/storage/cache"
	featureflags "gitcode.com/cloud-native/cloud-native/TemplateCreator/internal/upstream/shared/pkg/featureflags"
	"gitcode.com/cloud-native/cloud-native/TemplateCreator/internal/upstream/shared/pkg/telemetry"
)

func (bb *BaseBuilder) Hash(ctx context.Context, _ phases.LayerResult) (string, error) {
	ctx, span := tracer.Start(ctx, "hash base")
	defer span.End()

	var baseSource string
	if bb.Config.FromTemplate != nil {
		// When building from template, use the base template metadata
		baseSource = fmt.Sprintf("template:%s", bb.Config.FromTemplate.GetBuildID())
	} else {
		// Note: When "default" tag is used, the cached version might become ambiguous (not always default)
		// To update it now, you need to force the rebuild of the template, which will update this layer for all templates
		// in the team. This is okay for now, as the cache is not shared between teams, but it might need to be changed
		// when global caches are implemented.

		// When building from image, use the image name
		baseSource = bb.Config.FromImage
	}

	provisionVersion := provisionScriptFile
	if val := bb.featureFlags.IntFlag(
		ctx,
		featureflags.BuildProvisionVersion,
		featureflags.TemplateContext(bb.Config.TemplateID),
		featureflags.TeamContext(bb.Config.TeamID),
		// for dev environments (fallback value), use the provision script hash
	); val != featureflags.BuildProvisionVersion.Fallback() {
		provisionVersion = strconv.FormatInt(int64(val), 10)
	}

	telemetry.SetAttributes(ctx,
		attribute.String("index_version", bb.index.Version()),
		attribute.String("provision_version", provisionVersion),
		attribute.String("base_source", baseSource),
		attribute.Int64("disk_size_mb", bb.Config.DiskSizeMB),
	)

	return cache.HashKeys(
		bb.index.Version(),
		provisionVersion,
		strconv.FormatInt(bb.Config.DiskSizeMB, 10),
		baseSource,
	), nil
}
