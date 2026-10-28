// TemplateCreator modification: upstream import paths rewritten; no behavioral changes.

package steps

import (
	"context"
	"strings"

	"gitcode.com/cloud-native/cloud-native/TemplateCreator/internal/upstream/orchestrator/pkg/template/build/phases"
	"gitcode.com/cloud-native/cloud-native/TemplateCreator/internal/upstream/orchestrator/pkg/template/build/storage/cache"
	"gitcode.com/cloud-native/cloud-native/TemplateCreator/internal/upstream/shared/pkg/utils"
)

func (sb *StepBuilder) Hash(_ context.Context, sourceLayer phases.LayerResult) (string, error) {
	return cache.HashKeys(
		sourceLayer.Hash,
		sb.step.GetType(),
		strings.Join(sb.step.GetArgs(), " "),
		utils.Sprintp(sb.step.FilesHash), //nolint:protogetter // we need the nil check too
	), nil
}
