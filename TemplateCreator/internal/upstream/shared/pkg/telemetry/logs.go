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

package telemetry

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	"go.opentelemetry.io/otel/log"
	nooplog "go.opentelemetry.io/otel/log/noop"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/resource"
	"google.golang.org/grpc/encoding/gzip"
)

// LogProvider extends log.LoggerProvider with a Shutdown method
// so the batch processor and exporter are properly flushed on exit.
type LogProvider interface {
	log.LoggerProvider
	Shutdown(ctx context.Context) error
}

type noopLogProvider struct{ nooplog.LoggerProvider }

func (noopLogProvider) Shutdown(context.Context) error { return nil }

func NewNoopLogProvider() LogProvider { return noopLogProvider{} }

func NewLogProvider(ctx context.Context, res *resource.Resource, extraOpts ...otlploggrpc.Option) (LogProvider, error) {
	opts := []otlploggrpc.Option{
		otlploggrpc.WithInsecure(),
		otlploggrpc.WithEndpoint(otelCollectorGRPCEndpoint),
		otlploggrpc.WithCompressor(gzip.Name),
	}
	opts = append(opts, extraOpts...)

	exporter, err := otlploggrpc.New(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create logs exporter: %w", err)
	}

	p := sdklog.NewLoggerProvider(
		sdklog.WithResource(res),
		sdklog.WithProcessor(sdklog.NewBatchProcessor(exporter)),
	)

	return p, nil
}
