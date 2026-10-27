// Copyright (c) 2023-2026 e2b Devs Inc.
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
