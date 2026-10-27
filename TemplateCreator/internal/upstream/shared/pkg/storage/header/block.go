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

package header

func TotalBlocks(size, blockSize int64) int64 {
	return (size + blockSize - 1) / blockSize
}

func BlocksOffsets(size, blockSize int64) []int64 {
	offsets := make([]int64, TotalBlocks(size, blockSize))

	for i := range offsets {
		offsets[i] = BlockOffset(int64(i), blockSize)
	}

	return offsets
}

func BlockIdx(off, blockSize int64) int64 {
	return off / blockSize
}

func BlockOffset(idx, blockSize int64) int64 {
	return idx * blockSize
}
