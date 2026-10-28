// TemplateCreator modification: upstream import paths rewritten; no behavioral changes.

package user

import (
	"context"

	"gitcode.com/cloud-native/cloud-native/TemplateCreator/internal/upstream/orchestrator/pkg/template/build/phases"
	"gitcode.com/cloud-native/cloud-native/TemplateCreator/internal/upstream/orchestrator/pkg/template/build/storage/cache"
)

const prefix = "DEFAULT USER"

func (ub *UserBuilder) Hash(_ context.Context, sourceLayer phases.LayerResult) (string, error) {
	return cache.HashKeys(
		sourceLayer.Hash,
		prefix,
		ub.user,
	), nil
}
