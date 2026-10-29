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

package writer

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap/zapcore"

	"gitcode.com/boostkit/cloud-native.git/TemplateCreator/internal/upstream/shared/pkg/logger"
)

// postProcessor prints out "..." 5 seconds after every log
// message, if no other logs have been printed
type postProcessor struct {
	logger   logger.Logger
	done     chan struct{}
	doneOnce sync.Once
	ticker   *time.Ticker
	interval time.Duration
}

func (p *postProcessor) hook(_ zapcore.Entry) error {
	p.ticker.Reset(p.interval)

	return nil
}

// Start the post-processing.
func (p *postProcessor) run(ctx context.Context) {
	for {
		select {
		case <-p.done:
			p.ticker.Stop()

			return
		case <-p.ticker.C:
			p.logger.Info(ctx, "...")
		}
	}
}

func NewPostProcessor(ctx context.Context, interval time.Duration, core zapcore.Core) (zapcore.Core, func()) {
	pp := &postProcessor{
		logger:   logger.NewTracedLoggerFromCore(core),
		done:     make(chan struct{}),
		interval: interval,
		ticker:   time.NewTicker(interval),
	}

	go pp.run(ctx)

	return zapcore.RegisterHooks(core, pp.hook), func() {
		pp.doneOnce.Do(func() {
			pp.done <- struct{}{}
		})
	}
}
