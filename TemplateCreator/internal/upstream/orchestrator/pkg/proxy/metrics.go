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

package proxy

import (
	"context"

	"go.opentelemetry.io/otel/metric"

	"gitcode.com/cloud-native/cloud-native/TemplateCreator/internal/upstream/shared/pkg/telemetry"
	"gitcode.com/cloud-native/cloud-native/TemplateCreator/internal/upstream/shared/pkg/utils"
)

// Metrics holds ingress proxy metrics.
type Metrics struct {
	connectionsPerSandbox metric.Int64Histogram
	connectionDuration    metric.Int64Histogram
	connectionsBlocked    metric.Int64Counter
}

// NewMetrics creates a new Metrics instance.
func NewMetrics(meterProvider metric.MeterProvider) *Metrics {
	meter := meterProvider.Meter("orchestrator.proxy.sandbox")

	return &Metrics{
		connectionsPerSandbox: utils.Must(telemetry.GetHistogram(meter, telemetry.IngressProxyConnectionsPerSandboxHistogramName)),
		connectionDuration:    utils.Must(telemetry.GetHistogram(meter, telemetry.IngressProxyConnectionDurationHistogramName)),
		connectionsBlocked:    utils.Must(telemetry.GetCounter(meter, telemetry.IngressProxyConnectionsBlockedTotal)),
	}
}

// RecordConnectionsPerSandbox records the current connection count for a sandbox.
func (m *Metrics) RecordConnectionsPerSandbox(ctx context.Context, count int64) {
	m.connectionsPerSandbox.Record(ctx, count)
}

// RecordConnectionDuration records the duration of a proxied connection.
func (m *Metrics) RecordConnectionDuration(ctx context.Context, durationMs int64) {
	m.connectionDuration.Record(ctx, durationMs)
}

// RecordConnectionBlocked records a connection that was blocked by the connection limit.
func (m *Metrics) RecordConnectionBlocked(ctx context.Context) {
	m.connectionsBlocked.Add(ctx, 1)
}
