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

package memory

// Region is a mapping of a region of memory of the guest to a region of memory on the host.
// The serialization is based on the Firecracker UFFD protocol communication.
// https://github.com/firecracker-microvm/firecracker/blob/ceeca6a14284537ae0b2a192cd2ffef10d3a81e2/src/vmm/src/persist.rs#L96
type Region struct {
	BaseHostVirtAddr uintptr `json:"base_host_virt_addr"`
	Size             uintptr `json:"size"`
	Offset           uintptr `json:"offset"`
	// This field is deprecated in the newer version of the Firecracker with a new field `page_size`.
	PageSize uintptr `json:"page_size_kib"` // This is actually in bytes in the deprecated version.
}

// endOffset returns the end offset of the region in bytes.
// The end offset is exclusive.
func (r *Region) endOffset() int64 {
	return int64(r.Offset + r.Size)
}

// endHostVirtAddr returns the end address of the region in host virtual address.
// The end address is exclusive.
func (r *Region) endHostVirtAddr() uintptr {
	return r.BaseHostVirtAddr + r.Size
}

// shiftedOffset returns the offset of the given address in the region.
func (r *Region) shiftedOffset(addr uintptr) int64 {
	return int64(addr - r.BaseHostVirtAddr + r.Offset)
}

// shiftedHostVirtAddr returns the host virtual address of the given offset in the region.
func (r *Region) shiftedHostVirtAddr(off int64) uintptr {
	return uintptr(off) + r.BaseHostVirtAddr - r.Offset
}
