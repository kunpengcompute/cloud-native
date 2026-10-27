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

package sandbox

import (
	"fmt"
	"net"
	"sync"

	"gitcode.com/cloud-native/cloud-native/TemplateCreator/internal/upstream/shared/pkg/smap"
)

type MapSubscriber interface {
	OnInsert(sandbox *Sandbox)
	OnRemove(sandboxID string)
}

type Map struct {
	sandboxes *smap.Map[*Sandbox]

	subs     []MapSubscriber
	subsLock sync.RWMutex
}

func (m *Map) Subscribe(subscriber MapSubscriber) {
	m.subsLock.Lock()
	defer m.subsLock.Unlock()

	m.subs = append(m.subs, subscriber)
}

func (m *Map) trigger(fn func(MapSubscriber)) {
	m.subsLock.RLock()
	defer m.subsLock.RUnlock()

	for _, subscriber := range m.subs {
		fn(subscriber)
	}
}

func (m *Map) Items() map[string]*Sandbox {
	return m.sandboxes.Items()
}

func (m *Map) Count() int {
	return m.sandboxes.Count()
}

func (m *Map) Get(sandboxID string) (*Sandbox, bool) {
	return m.sandboxes.Get(sandboxID)
}

func (m *Map) GetByHostPort(hostPort string) (*Sandbox, error) {
	reqIP, _, err := net.SplitHostPort(hostPort)
	if err != nil {
		return nil, fmt.Errorf("error parsing remote address %s: %w", hostPort, err)
	}

	for _, sbx := range m.sandboxes.Items() {
		if sbx.Slot.HostIPString() == reqIP {
			return sbx, nil
		}
	}

	return nil, fmt.Errorf("sandbox with address %s not found", hostPort)
}

func (m *Map) Insert(sbx *Sandbox) {
	m.sandboxes.Insert(sbx.Runtime.SandboxID, sbx)

	go m.trigger(func(s MapSubscriber) {
		s.OnInsert(sbx)
	})
}

func (m *Map) Remove(sandboxID string) {
	m.sandboxes.Remove(sandboxID)

	go m.trigger(func(s MapSubscriber) {
		s.OnRemove(sandboxID)
	})
}

func (m *Map) RemoveByLifecycleID(sandboxID, lifecycleID string) {
	removed := m.sandboxes.RemoveCb(sandboxID, func(_ string, v *Sandbox, exists bool) bool {
		if !exists {
			return false
		}

		if v == nil {
			return false
		}

		return v.LifecycleID == lifecycleID
	})

	if removed {
		go m.trigger(func(s MapSubscriber) {
			s.OnRemove(sandboxID)
		})
	}
}

func NewSandboxesMap() *Map {
	return &Map{sandboxes: smap.New[*Sandbox]()}
}
