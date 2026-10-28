// TemplateCreator modification: upstream import paths rewritten; no behavioral changes.

package limit

import (
	"context"
	"time"

	"go.uber.org/zap"

	featureflags "gitcode.com/cloud-native/cloud-native/TemplateCreator/internal/upstream/shared/pkg/featureflags"
	"gitcode.com/cloud-native/cloud-native/TemplateCreator/internal/upstream/shared/pkg/logger"
)

func (l *Limiter) UpdateUploadLimitSemaphore(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			uploadLimitFlag := l.featureFlags.IntFlag(ctx, featureflags.GcloudConcurrentUploadLimit)

			// Update the semaphore with the new value
			if err := l.gCloudUploadLimiter.SetLimit(int64(uploadLimitFlag)); err != nil {
				logger.L().Error(ctx, "failed to adjust upload semaphore", zap.Error(err))
			}
		case <-l.done:
			return
		}
	}
}
