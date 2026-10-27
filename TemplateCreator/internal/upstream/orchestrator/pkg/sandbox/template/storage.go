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

// TemplateCreator modification: upstream import paths rewritten; no behavioral changes.

package template

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	blockmetrics "gitcode.com/cloud-native/cloud-native/TemplateCreator/internal/upstream/orchestrator/pkg/sandbox/block/metrics"
	"gitcode.com/cloud-native/cloud-native/TemplateCreator/internal/upstream/orchestrator/pkg/sandbox/build"
	"gitcode.com/cloud-native/cloud-native/TemplateCreator/internal/upstream/shared/pkg/storage"
	"gitcode.com/cloud-native/cloud-native/TemplateCreator/internal/upstream/shared/pkg/storage/header"
)

const (
	oldMemfileHugePageSize = 2 << 20 // 2 MiB
	oldRootfsBlockSize     = 2 << 11 // 4 KiB
)

type Storage struct {
	header *header.Header
	source *build.File
}

func storageHeaderObjectType(diffType build.DiffType) (storage.ObjectType, bool) {
	switch diffType {
	case build.Memfile:
		return storage.MemfileHeaderObjectType, true
	case build.Rootfs:
		return storage.RootFSHeaderObjectType, true
	default:
		return storage.UnknownObjectType, false
	}
}

func objectType(diffType build.DiffType) (storage.SeekableObjectType, bool) {
	switch diffType {
	case build.Memfile:
		return storage.MemfileObjectType, true
	case build.Rootfs:
		return storage.RootFSObjectType, true
	default:
		return storage.UnknownSeekableObjectType, false
	}
}

func NewStorage(
	ctx context.Context,
	store *build.DiffStore,
	buildId string,
	fileType build.DiffType,
	h *header.Header,
	persistence storage.StorageProvider,
	metrics blockmetrics.Metrics,
) (*Storage, error) {
	if h == nil {
		headerObjectPath := buildId + "/" + string(fileType) + storage.HeaderSuffix
		headerObjectType, ok := storageHeaderObjectType(fileType)
		if !ok {
			return nil, build.UnknownDiffTypeError{DiffType: fileType}
		}

		headerObject, err := persistence.OpenBlob(ctx, headerObjectPath, headerObjectType)
		if err != nil {
			return nil, err
		}

		diffHeader, err := header.Deserialize(ctx, headerObject)

		// If we can't find the diff header in storage, we switch to templates without a headers
		if err != nil && !errors.Is(err, storage.ErrObjectNotExist) {
			return nil, fmt.Errorf("failed to deserialize header: %w", err)
		}

		if err == nil {
			h = diffHeader
		}
	}

	// If we can't find the diff header in storage, we try to find the "old" style template without a header as a fallback.
	if h == nil {
		objectPath := buildId + "/" + string(fileType)
		objectType, ok := objectType(fileType)
		if !ok {
			return nil, build.UnknownDiffTypeError{DiffType: fileType}
		}
		object, err := persistence.OpenSeekable(ctx, objectPath, objectType)
		if err != nil {
			return nil, err
		}

		size, err := object.Size(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get object size: %w", err)
		}

		id, err := uuid.Parse(buildId)
		if err != nil {
			return nil, fmt.Errorf("failed to parse build id: %w", err)
		}

		// TODO: This is a workaround for the old style template without a header.
		// We don't know the block size of the old style template, so we set it manually.
		var blockSize uint64
		switch fileType {
		case build.Memfile:
			blockSize = oldMemfileHugePageSize
		case build.Rootfs:
			blockSize = oldRootfsBlockSize
		default:
			return nil, fmt.Errorf("unsupported file type: %s", fileType)
		}

		h, err = header.NewHeader(&header.Metadata{
			// The version is always 1 for the old style template without a header.
			Version:     1,
			BuildId:     id,
			BaseBuildId: id,
			Size:        uint64(size),
			BlockSize:   blockSize,
			Generation:  1,
		}, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create header for old style template: %w", err)
		}
	}

	b := build.NewFile(h, store, fileType, persistence, metrics)

	return &Storage{
		source: b,
		header: h,
	}, nil
}

func (d *Storage) ReadAt(ctx context.Context, p []byte, off int64) (int, error) {
	return d.source.ReadAt(ctx, p, off)
}

func (d *Storage) Size(_ context.Context) (int64, error) {
	return int64(d.header.Metadata.Size), nil
}

func (d *Storage) BlockSize() int64 {
	return int64(d.header.Metadata.BlockSize)
}

func (d *Storage) Slice(ctx context.Context, off, length int64) ([]byte, error) {
	return d.source.Slice(ctx, off, length)
}

func (d *Storage) Header() *header.Header {
	return d.header
}

func (d *Storage) Close() error {
	return nil
}
