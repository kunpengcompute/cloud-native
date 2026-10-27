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

package utils

import "sync"

// WaitMap allows you to wait for functions with given keys and execute them only once.
type WaitMap struct {
	mu sync.Mutex
	m  map[int64]func() error
}

func NewWaitMap() *WaitMap {
	return &WaitMap{
		m: make(map[int64]func() error),
	}
}

// Wait waits for the function with the given key to be executed.
// If the function is already executing, it waits for it to finish.
// If the function is not yet executing, it executes the function and returns its result.
func (m *WaitMap) Wait(key int64, fn func() error) error {
	m.mu.Lock()

	once, ok := m.m[key]
	if ok {
		m.mu.Unlock()

		return once()
	}

	once = sync.OnceValue(fn)

	m.m[key] = once

	m.mu.Unlock()

	return once()
}
