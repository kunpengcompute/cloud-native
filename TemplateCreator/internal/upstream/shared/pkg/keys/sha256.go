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

package keys

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
)

type Sha256Hashing struct{}

func NewSHA256Hashing() *Sha256Hashing {
	return &Sha256Hashing{}
}

func (h *Sha256Hashing) Hash(key []byte) string {
	hashBytes := sha256.Sum256(key)

	hash64 := base64.RawStdEncoding.EncodeToString(hashBytes[:])

	return fmt.Sprintf(
		"$sha256$%s",
		hash64,
	)
}

func (h *Sha256Hashing) HashWithoutPrefix(key []byte) string {
	hashBytes := sha256.Sum256(key)

	return base64.RawStdEncoding.EncodeToString(hashBytes[:])
}
