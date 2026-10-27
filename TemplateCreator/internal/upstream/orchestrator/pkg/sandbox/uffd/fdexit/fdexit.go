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

package fdexit

import (
	"errors"
	"fmt"
	"os"
	"sync"
)

// FdExit is a wrapper around a pipe that allows to signal the exit of the uffd.
type FdExit struct {
	r    *os.File
	w    *os.File
	exit func() error

	closeOnce sync.Once
	closeErr  error
}

func New() (*FdExit, error) {
	r, w, err := os.Pipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create exit fd: %w", err)
	}

	return &FdExit{
		r: r,
		w: w,
		exit: sync.OnceValue(func() error {
			_, writeErr := w.Write([]byte{0})
			if writeErr != nil {
				return fmt.Errorf("failed write to exit writer: %w", writeErr)
			}

			return nil
		}),
	}, nil
}

func (e *FdExit) SignalExit() error {
	return e.exit()
}

func (e *FdExit) Reader() int32 {
	return int32(e.r.Fd())
}

// Close closes the pipe file descriptors. It is safe to call multiple times.
func (e *FdExit) Close() error {
	e.closeOnce.Do(func() {
		e.closeErr = errors.Join(e.SignalExit(), e.r.Close(), e.w.Close())
	})

	return e.closeErr
}
