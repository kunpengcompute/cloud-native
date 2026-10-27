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

package network

import (
	"context"
	"fmt"
	"strconv"
	"sync"
)

type StorageMemory struct {
	config      Config
	slotsSize   int
	freeSlots   []bool
	freeSlotsMu sync.Mutex
}

func NewStorageMemory(slotsSize int, config Config) (*StorageMemory, error) {
	return &StorageMemory{
		config:      config,
		slotsSize:   slotsSize,
		freeSlots:   make([]bool, slotsSize),
		freeSlotsMu: sync.Mutex{},
	}, nil
}

func (s *StorageMemory) Acquire(_ context.Context) (*Slot, error) {
	s.freeSlotsMu.Lock()
	defer s.freeSlotsMu.Unlock()

	// Simple slot tracking in memory
	// We skip the first slot because it's the host slot
	for slotIdx := 1; slotIdx < s.slotsSize; slotIdx++ {
		key := getMemoryKey(slotIdx)
		if !s.freeSlots[slotIdx] {
			s.freeSlots[slotIdx] = true

			return NewSlot(key, slotIdx, s.config)
		}
	}

	return nil, fmt.Errorf("failed to acquire IP slot: no empty slots found")
}

func (s *StorageMemory) Release(ips *Slot) error {
	s.freeSlotsMu.Lock()
	defer s.freeSlotsMu.Unlock()

	s.freeSlots[ips.Idx] = false

	return nil
}

func getMemoryKey(slotIdx int) string {
	return strconv.Itoa(slotIdx)
}
