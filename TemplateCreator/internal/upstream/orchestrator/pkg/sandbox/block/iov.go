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
	"fmt"
	"math"
	"os"

	"github.com/tklauser/go-sysconf"

	"gitcode.com/boostkit/cloud-native.git/TemplateCreator/internal/upstream/shared/pkg/utils"
)

var (
	// IOV_MAX is the limit of the vectors that can be passed in a single ioctl call.
	IOV_MAX = utils.Must(getIOVMax())

	PAGE_SIZE = os.Getpagesize()
	PAGE_MASK = ^(int64(PAGE_SIZE) - 1)
	INT_MAX   = int64(math.MaxInt32)

	// This is maximum bytes that can be read/written in a single operation.
	//
	// https://unix.stackexchange.com/questions/794316/why-linux-read-avoids-using-full-2-gib-in-one-call
	// https://stackoverflow.com/questions/70368651/why-cant-linux-write-more-than-2147479552-bytes
	MAX_RW_COUNT = INT_MAX & PAGE_MASK
)

func getIOVMax() (int, error) {
	iovMax, err := sysconf.Sysconf(sysconf.SC_IOV_MAX)
	if err != nil {
		return 0, fmt.Errorf("failed to get IOV_MAX: %w", err)
	}

	return int(iovMax), nil
}

func getAlignedMaxRwCount(blockSize int64) int64 {
	return (MAX_RW_COUNT / blockSize) * blockSize
}
