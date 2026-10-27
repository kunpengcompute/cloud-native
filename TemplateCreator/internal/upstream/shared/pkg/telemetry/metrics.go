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

package telemetry

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/resource"
)

type noopMetricExporter struct{}

func (noopMetricExporter) Temporality(sdkmetric.InstrumentKind) metricdata.Temporality {
	return metricdata.CumulativeTemporality
}

func (noopMetricExporter) Aggregation(sdkmetric.InstrumentKind) sdkmetric.Aggregation {
	return sdkmetric.AggregationDrop{}
}

func (noopMetricExporter) Export(context.Context, *metricdata.ResourceMetrics) error {
	return nil
}

func (noopMetricExporter) ForceFlush(context.Context) error {
	return nil
}

func (noopMetricExporter) Shutdown(context.Context) error {
	return nil
}

func NewMeterExporter(ctx context.Context, extraOption ...otlpmetricgrpc.Option) (sdkmetric.Exporter, error) {
	opts := []otlpmetricgrpc.Option{
		otlpmetricgrpc.WithInsecure(),
		otlpmetricgrpc.WithEndpoint(otelCollectorGRPCEndpoint),
	}
	opts = append(opts, extraOption...)

	metricExporter, metricErr := otlpmetricgrpc.New(
		ctx,
		opts...,
	)
	if metricErr != nil {
		return nil, fmt.Errorf("failed to create metric exporter: %w", metricErr)
	}

	return metricExporter, nil
}

func NewMeterProvider(metricsExporter sdkmetric.Exporter, metricExportPeriod time.Duration, res *resource.Resource, extraOption ...sdkmetric.Option) (metric.MeterProvider, error) {
	opts := []sdkmetric.Option{
		sdkmetric.WithReader(
			sdkmetric.NewPeriodicReader(
				metricsExporter,
				sdkmetric.WithInterval(metricExportPeriod),
			),
		),
	}

	if res != nil {
		opts = append(opts, sdkmetric.WithResource(res))
	}

	opts = append(opts, extraOption...)

	return sdkmetric.NewMeterProvider(opts...), nil
}
