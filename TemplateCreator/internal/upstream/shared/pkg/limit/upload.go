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
