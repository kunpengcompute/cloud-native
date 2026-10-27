// TemplateCreator modification: upstream import paths rewritten; no behavioral changes.

package limit

import (
	"context"
	"sync"

	featureflags "gitcode.com/cloud-native/cloud-native/TemplateCreator/internal/upstream/shared/pkg/featureflags"
	"gitcode.com/cloud-native/cloud-native/TemplateCreator/internal/upstream/shared/pkg/utils"
)

type Limiter struct {
	gCloudUploadLimiter *utils.AdjustableSemaphore
	featureFlags        *featureflags.Client

	done      chan struct{}
	closeOnce sync.Once
}

func New(ctx context.Context, featureFlags *featureflags.Client) (*Limiter, error) {
	uploadLimiter, err := utils.NewAdjustableSemaphore(int64(featureflags.GcloudConcurrentUploadLimit.Fallback()))
	if err != nil {
		return nil, err
	}

	l := &Limiter{
		gCloudUploadLimiter: uploadLimiter,
		featureFlags:        featureFlags,
		done:                make(chan struct{}),
	}

	go l.UpdateUploadLimitSemaphore(ctx)

	return l, nil
}

func (l *Limiter) Close(_ context.Context) error {
	l.closeOnce.Do(func() {
		close(l.done)
	})

	return nil
}
