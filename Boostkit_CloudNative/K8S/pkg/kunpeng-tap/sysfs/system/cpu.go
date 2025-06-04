/*
 * Copyright (c) 2025 Huawei Technology corp.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package system

import "k8s.io/utils/cpuset"

var _ CPU = &cpu{}

// cpu represent the logical CPU in OS.
type cpu struct {
	path     string        // sysfs path
	id       ID            // CPU id
	pkg      ID            // package id
	die      ID            // die id
	node     ID            // node id
	core     ID            // core id
	threads  cpuset.CPUSet // sibling/hyper-threads
	online   bool          // whether this CPU is online
	isolated bool          // whether this CPU is isolated
}

// CPU methods
func (c *cpu) ID() ID {
	return c.id
}

func (c *cpu) PackageID() ID {
	return c.pkg
}

func (c *cpu) DieID() ID {
	return c.die
}

func (c *cpu) NodeID() ID {
	return c.node
}

func (c *cpu) CoreID() ID {
	return c.core
}

func (c *cpu) ThreadCPUSet() cpuset.CPUSet {
	return c.threads
}

func (c *cpu) Isolated() bool {
	return c.isolated
}

func (c *cpu) Online() bool {
	return c.online
}
