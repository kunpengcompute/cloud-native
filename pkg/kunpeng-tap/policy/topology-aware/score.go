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

package topologyaware

import (
	"fmt"
)

// Score represents scoring data for a node.
type Score interface {
	Supply() Supply
	Request() Request
	SharedCapacity() int
	// SharedCapacityByRequest returns the remaining shared capacity by request.
	SharedCapacityByRequest() int
	// SharedCapacityByLimit returns the remaining shared capacity by limit.
	SharedCapacityByLimit() int
	// MemoryCapacity returns the remaining memory capacity.
	MemoryCapacity() uint64
	// Colocated returns the number of containers already allocated to this node.
	Colocated() int
	// GPUCount returns the number of GPUs attached to this node.
	GPUCount() int
	// String returns the score as a string.
	String() string
}

// score implements the Score interface.
type score struct {
	supply          Supply  // CPU supply (node)
	request         Request // CPU request (container)
	shared          int     // remaining shared capacity
	sharedByRequest int     // remaining shared capacity by request
	sharedByLimit   int     // remaining shared capacity by limit
	memoryCapacity  uint64  // remaining memory capacity
	colocated       int     // number of colocated containers
	gpuCount        int     // number of GPUs attached to this node
}

func (s *score) Supply() Supply {
	return s.supply
}

func (s *score) Request() Request {
	return s.request
}

func (s *score) String() string {
	return fmt.Sprintf("<Score: node %s, shared:%d, colocated:%d, gpuCount:%d >", s.supply.GetNode().Name(), s.shared, s.colocated, s.gpuCount)
}

func (s *score) SharedCapacity() int {
	return s.shared
}

func (s *score) SharedCapacityByRequest() int {
	return s.sharedByRequest
}

func (s *score) SharedCapacityByLimit() int {
	return s.sharedByLimit
}

func (s *score) MemoryCapacity() uint64 {
	return s.memoryCapacity
}

func (s *score) Colocated() int {
	return s.colocated
}

func (s *score) GPUCount() int {
	return s.gpuCount
}
