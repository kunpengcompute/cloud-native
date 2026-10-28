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

package ioutils

import (
	"io"
	"os"
)

func WriteToFileFromReader(path string, r io.Reader) (err error) {
	// Create (truncate if exists) with 0644 perms
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	// Make sure we return a close error if that's the only error.
	defer func() {
		if cerr := f.Close(); err == nil && cerr != nil {
			err = cerr
		}
	}()

	if _, err = io.Copy(f, r); err != nil {
		return err
	}

	return f.Sync() // ensure contents hit disk
}
