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

package metrics

import (
	"fmt"

	"go.opentelemetry.io/otel/metric"

	"gitcode.com/boostkit/cloud-native.git/TemplateCreator/internal/upstream/shared/pkg/telemetry"
)

const (
	orchestratorBlockSlices      = "orchestrator.blocks.slices"
	orchestratorBlockChunksFetch = "orchestrator.blocks.chunks.fetch"
	orchestratorBlockChunksStore = "orchestrator.blocks.chunks.store"
)

type Metrics struct {
	// SlicesMetric is used to measure page faulting performance.
	SlicesTimerFactory telemetry.TimerFactory

	// WriteChunksMetric is used to measure the time taken to download chunks from remote storage
	RemoteReadsTimerFactory telemetry.TimerFactory

	// WriteChunksMetric is used to measure performance of writing chunks to disk.
	WriteChunksTimerFactory telemetry.TimerFactory
}

func NewMetrics(meterProvider metric.MeterProvider) (Metrics, error) {
	var m Metrics

	blocksMeter := meterProvider.Meter("internal.sandbox.block.metrics")

	var err error
	if m.SlicesTimerFactory, err = telemetry.NewTimerFactory(
		blocksMeter, orchestratorBlockSlices,
		"Time taken to retrieve memory slices",
		"Total bytes requested",
		"Total page faults",
	); err != nil {
		return m, fmt.Errorf("error creating slices timer factory: %w", err)
	}

	if m.RemoteReadsTimerFactory, err = telemetry.NewTimerFactory(
		blocksMeter, orchestratorBlockChunksFetch,
		"Time taken to fetch memory chunks from remote store",
		"Total bytes fetched from remote store",
		"Total remote fetches",
	); err != nil {
		return m, fmt.Errorf("error creating reads timer factory: %w", err)
	}

	if m.WriteChunksTimerFactory, err = telemetry.NewTimerFactory(
		blocksMeter, orchestratorBlockChunksStore,
		"Time taken to write memory chunks to disk",
		"Total bytes written to disk",
		"Total cache writes",
	); err != nil {
		return m, fmt.Errorf("failed to get stored chunks metric: %w", err)
	}

	return m, nil
}
