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

package block

import (
	"maps"
	"sync"
	"sync/atomic"

	"gitcode.com/boostkit/cloud-native.git/TemplateCreator/internal/upstream/shared/pkg/storage/header"
)

// PrefetchData contains block access data for prefetch mapping.
type PrefetchData struct {
	// BlockEntries contains metadata for each block index
	BlockEntries map[uint64]PrefetchBlockEntry
	// BlockSize is the size of each block in bytes
	BlockSize int64
}

// AccessType represents the type of access that caused a block to be loaded.
type AccessType string

const (
	// Read indicates a block loaded by a read operation.
	Read AccessType = "read"
	// Write indicates a block loaded by a write operation.
	Write AccessType = "write"
	// Prefetch indicates a proactively prefetched block, not a real fault.
	Prefetch AccessType = "prefetch"
)

// BlockEntry holds metadata about a tracked block.
type PrefetchBlockEntry struct {
	Index      uint64
	Order      uint64
	AccessType AccessType
}

type PrefetchTracker struct {
	mu sync.RWMutex

	blockSize int64

	// blockEntries stores metadata for each block index
	blockEntries map[uint64]PrefetchBlockEntry
	// orderCounter tracks the next order number to assign
	orderCounter uint64

	isTracking atomic.Bool
}

func NewPrefetchTracker(blockSize int64) *PrefetchTracker {
	t := &PrefetchTracker{
		blockSize:    blockSize,
		blockEntries: make(map[uint64]PrefetchBlockEntry),
		orderCounter: 1,
	}
	t.isTracking.Store(true)

	return t
}

// Add adds an offset to the tracker with metadata about the access.
func (t *PrefetchTracker) Add(off int64, accessType AccessType) {
	if !t.isTracking.Load() {
		return
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	idx := uint64(header.BlockIdx(off, t.blockSize))

	// Only add if not already tracked
	if _, ok := t.blockEntries[idx]; !ok {
		t.blockEntries[idx] = PrefetchBlockEntry{
			Index:      idx,
			Order:      t.orderCounter,
			AccessType: accessType,
		}
		t.orderCounter++
	}
}

func (t *PrefetchTracker) PrefetchData() PrefetchData {
	// Stop tracking new blocks, only optimization as we don't need to track blocks after the prefetch data is collected.
	// There might be a race condition with the lock, but we don't care.
	t.isTracking.Store(false)

	t.mu.RLock()
	defer t.mu.RUnlock()

	result := make(map[uint64]PrefetchBlockEntry, len(t.blockEntries))
	maps.Copy(result, t.blockEntries)

	return PrefetchData{
		BlockEntries: result,
		BlockSize:    t.blockSize,
	}
}
