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
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/google/uuid"

	"gitcode.com/boostkit/cloud-native.git/TemplateCreator/internal/upstream/shared/pkg/storage/header"
)

type Local struct {
	f    *os.File
	path string

	header *header.Header
}

var _ ReadonlyDevice = (*Local)(nil)

func NewLocal(path string, blockSize int64, buildID uuid.UUID) (*Local, error) {
	f, err := os.OpenFile(path, os.O_RDONLY, os.ModePerm)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}

	info, err := f.Stat()
	if err != nil {
		err = errors.Join(err, f.Close())

		return nil, fmt.Errorf("failed to get file info: %w", err)
	}

	h, err := header.NewHeader(header.NewTemplateMetadata(
		buildID,
		uint64(blockSize),
		uint64(info.Size()),
	), nil)
	if err != nil {
		err = errors.Join(err, f.Close())

		return nil, fmt.Errorf("failed to create header: %w", err)
	}

	return &Local{
		f:      f,
		path:   path,
		header: h,
	}, nil
}

func (d *Local) Path() string {
	return d.path
}

func (d *Local) ReadAt(ctx context.Context, p []byte, off int64) (int, error) {
	slice, err := d.Slice(ctx, off, int64(len(p)))
	if err != nil {
		return 0, fmt.Errorf("failed to slice mmap: %w", err)
	}

	return copy(p, slice), nil
}

func (d *Local) Size(_ context.Context) (int64, error) {
	return int64(d.header.Metadata.Size), nil
}

func (d *Local) BlockSize() int64 {
	return int64(d.header.Metadata.BlockSize)
}

func (d *Local) Close() (e error) {
	err := d.f.Close()
	if err != nil {
		return fmt.Errorf("error closing file: %w", err)
	}

	return nil
}

func (d *Local) Slice(_ context.Context, off, length int64) ([]byte, error) {
	end := off + length
	size := int64(d.header.Metadata.Size)
	if end > size {
		end = size
		length = end - off
	}

	out := make([]byte, length)
	_, err := d.f.ReadAt(out, off)
	if err != nil {
		return nil, err
	}

	return out, nil
}

func (d *Local) Header() *header.Header {
	return d.header
}

func (d *Local) UpdateHeaderSize() error {
	info, err := d.f.Stat()
	if err != nil {
		return fmt.Errorf("failed to get file info: %w", err)
	}

	d.header.Metadata.Size = uint64(info.Size())

	return nil
}
