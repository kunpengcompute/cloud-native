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

package uffd

import (
	"context"

	"github.com/bits-and-blooms/bitset"

	"gitcode.com/boostkit/cloud-native.git/TemplateCreator/internal/upstream/orchestrator/pkg/sandbox/block"
	"gitcode.com/boostkit/cloud-native.git/TemplateCreator/internal/upstream/orchestrator/pkg/sandbox/fc"
	"gitcode.com/boostkit/cloud-native.git/TemplateCreator/internal/upstream/shared/pkg/storage/header"
	"gitcode.com/boostkit/cloud-native.git/TemplateCreator/internal/upstream/shared/pkg/utils"
)

type NoopMemory struct {
	size      int64
	blockSize int64

	exit *utils.ErrorOnce
}

var _ MemoryBackend = (*NoopMemory)(nil)

func NewNoopMemory(size, blockSize int64) *NoopMemory {
	return &NoopMemory{
		size:      size,
		blockSize: blockSize,
		exit:      utils.NewErrorOnce(),
	}
}

func (m *NoopMemory) Prefault(_ context.Context, _ int64, _ []byte) error {
	return nil
}

func (m *NoopMemory) DiffMetadata(ctx context.Context, f *fc.Process) (*header.DiffMetadata, error) {
	diffInfo, err := f.MemoryInfo(ctx, m.blockSize)
	if err != nil {
		return nil, err
	}

	dirty := diffInfo.Dirty.Difference(diffInfo.Empty)

	numberOfPages := header.TotalBlocks(m.size, m.blockSize)

	empty := bitset.New(uint(numberOfPages))
	empty.FlipRange(0, uint(numberOfPages))

	empty = empty.Difference(dirty)

	return &header.DiffMetadata{
		Dirty:     dirty,
		Empty:     empty,
		BlockSize: m.blockSize,
	}, nil
}

func (m *NoopMemory) PrefetchData(_ context.Context) (block.PrefetchData, error) {
	// NoopMemory doesn't track block accesses, so return empty data
	return block.PrefetchData{
		BlockEntries: nil,
		BlockSize:    m.blockSize,
	}, nil
}

func (m *NoopMemory) Start(context.Context, string) error {
	return nil
}

func (m *NoopMemory) Stop() error {
	m.exit.SetSuccess()

	// This should be idempotent, so no need to return an error if it's already set.
	return nil
}

func (m *NoopMemory) Ready() chan struct{} {
	ch := make(chan struct{})
	close(ch)

	return ch
}

func (m *NoopMemory) Exit() *utils.ErrorOnce {
	return m.exit
}
