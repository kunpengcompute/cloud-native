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

package sbxlogger

import (
	"context"

	"go.uber.org/zap"

	"gitcode.com/cloud-native/cloud-native/TemplateCreator/internal/upstream/shared/pkg/logger"
)

type SandboxLogger struct {
	logger.Logger
}

type HealthCheckAction int

const (
	Success HealthCheckAction = iota
	Fail
	ReportSuccess
	ReportFail
)

type SandboxMetricsFields struct {
	Timestamp      int64
	CPUCount       uint32
	CPUUsedPercent float32
	MemTotalMiB    uint64
	MemUsedMiB     uint64
}

func (sl *SandboxLogger) Metrics(ctx context.Context, metrics SandboxMetricsFields) {
	sl.Info(
		ctx,
		"",
		zap.String("category", "metrics"),
		zap.Float32("cpuUsedPct", metrics.CPUUsedPercent),
		zap.Uint32("cpuCount", metrics.CPUCount),
		zap.Uint64("memTotalMiB", metrics.MemTotalMiB),
		zap.Uint64("memUsedMiB", metrics.MemUsedMiB),
	)
}

func (sl *SandboxLogger) Healthcheck(ctx context.Context, action HealthCheckAction) {
	switch action {
	case Success:
		sl.Info(ctx, "Sandbox healthcheck recovered",
			zap.Bool("healthcheck", true))
	case Fail:
		sl.Error(ctx, "Sandbox healthcheck started failing",
			zap.Bool("healthcheck", false))
	case ReportSuccess:
		sl.Info(
			ctx,
			"Control sandbox healthcheck was successful",
			zap.Bool("healthcheck", true))
	case ReportFail:
		sl.Error(ctx, "Control sandbox healthcheck was unsuccessful",
			zap.Bool("healthcheck", false))
	}
}
