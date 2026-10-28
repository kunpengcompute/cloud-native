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

package lock

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"syscall"

	"github.com/google/uuid"

	"gitcode.com/cloud-native/cloud-native/TemplateCreator/internal/upstream/shared/pkg/utils"
)

type AtomicImmutableFile struct {
	lockFile *os.File
	tempFile *os.File
	filename string

	closeOnce sync.Once
}

func (f *AtomicImmutableFile) Write(p []byte) (n int, err error) {
	return f.tempFile.Write(p)
}

var _ io.Writer = (*AtomicImmutableFile)(nil)

func OpenFile(ctx context.Context, filename string) (*AtomicImmutableFile, error) {
	lockFile, err := TryAcquireLock(ctx, filename)
	if err != nil {
		return nil, err
	}

	tempFilename := fmt.Sprintf("%s.temp.%s", filename, uuid.NewString())
	tempFile, err := os.OpenFile(tempFilename, os.O_WRONLY|os.O_CREATE, 0o600)
	if err != nil {
		utils.Cleanup(ctx, "failed to release lock",
			func() error { return ReleaseLock(ctx, lockFile) })

		return nil, fmt.Errorf("failed to open temp file: %w", err)
	}

	return &AtomicImmutableFile{
		lockFile: lockFile,
		tempFile: tempFile,
		filename: filename,
	}, nil
}

func (f *AtomicImmutableFile) Commit(ctx context.Context) error {
	return f.close(ctx, true)
}

func (f *AtomicImmutableFile) Close(ctx context.Context) error {
	return f.close(ctx, false)
}

func (f *AtomicImmutableFile) close(ctx context.Context, success bool) error {
	var err error

	f.closeOnce.Do(func() {
		var errs []error

		defer utils.Cleanup(ctx, "failed to unlock file", func() error {
			return ReleaseLock(ctx, f.lockFile)
		})

		if err = f.tempFile.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close temp file: %w", err))
		}

		if success {
			if err = utils.RenameOrDeleteFile(ctx, f.tempFile.Name(), f.filename); err != nil {
				// someone else may have written the file successfully
				if !errors.Is(err, syscall.EEXIST) {
					// if not, report the error
					errs = append(errs, fmt.Errorf("failed to commit file: %w", err))
				}
			}
		} else {
			errs = append(errs, os.Remove(f.tempFile.Name()))
		}

		err = errors.Join(errs...)
	})

	return err
}
