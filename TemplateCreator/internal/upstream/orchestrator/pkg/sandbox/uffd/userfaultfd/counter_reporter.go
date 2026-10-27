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

package userfaultfd

import (
	"context"
	"time"

	"go.uber.org/zap"

	"gitcode.com/cloud-native/cloud-native/TemplateCreator/internal/upstream/shared/pkg/logger"
)

// counterReporter tracks counts per type/key and logs them all in a single message.
type counterReporter struct {
	counts    map[string]uint64
	startTime time.Time
	endTime   time.Time
	logger    logger.Logger
	msg       string
}

func newCounterReporter(logger logger.Logger, msg string) *counterReporter {
	return &counterReporter{
		counts:    make(map[string]uint64),
		startTime: time.Time{},
		endTime:   time.Time{},
		logger:    logger,
		msg:       msg,
	}
}

func (c *counterReporter) Increase(eventType string) {
	if c.totalCount() == 0 {
		c.startTime = time.Now()
	}

	c.counts[eventType]++

	c.endTime = time.Now()
}

func (c *counterReporter) totalCount() uint64 {
	var total uint64
	for _, count := range c.counts {
		total += count
	}

	return total
}

func (c *counterReporter) log(ctx context.Context, closing bool) {
	total := c.totalCount()
	if total > 0 {
		fields := []zap.Field{
			zap.Uint64("count", total),
			zap.Time("start", c.startTime),
			zap.Time("end", c.endTime),
			zap.Bool("closing", closing),
		}

		for eventType, count := range c.counts {
			fields = append(fields, zap.Uint64(eventType, count))
		}

		c.logger.Debug(ctx, c.msg, fields...)

		c.counts = make(map[string]uint64)
	}
}

func (c *counterReporter) Close(ctx context.Context) {
	c.log(ctx, true)
}

func (c *counterReporter) Log(ctx context.Context) {
	c.log(ctx, false)
}
