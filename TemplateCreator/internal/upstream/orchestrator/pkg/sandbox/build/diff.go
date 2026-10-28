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

package build

import (
	"context"
	"fmt"
	"io"
	"path/filepath"

	"gitcode.com/cloud-native/cloud-native/TemplateCreator/internal/upstream/orchestrator/pkg/sandbox/block"
	"gitcode.com/cloud-native/cloud-native/TemplateCreator/internal/upstream/shared/pkg/id"
	"gitcode.com/cloud-native/cloud-native/TemplateCreator/internal/upstream/shared/pkg/storage"
)

type DiffType string

type NoDiffError struct{}

func (NoDiffError) Error() string {
	return "the diff is empty"
}

const (
	Memfile DiffType = storage.MemfileName
	Rootfs  DiffType = storage.RootfsName
)

type Diff interface {
	io.Closer
	storage.SeekableReader
	block.Slicer
	CacheKey() DiffStoreKey
	CachePath() (string, error)
	FileSize() (int64, error)
	Init(ctx context.Context) error
}

type NoDiff struct{}

var _ Diff = (*NoDiff)(nil)

func (n *NoDiff) CachePath() (string, error) {
	return "", NoDiffError{}
}

func (n *NoDiff) Slice(_ context.Context, _, _ int64) ([]byte, error) {
	return nil, NoDiffError{}
}

func (n *NoDiff) Close() error {
	return nil
}

func (n *NoDiff) ReadAt(_ context.Context, _ []byte, _ int64) (int, error) {
	return 0, NoDiffError{}
}

func (n *NoDiff) FileSize() (int64, error) {
	return 0, NoDiffError{}
}

func (n *NoDiff) Size(_ context.Context) (int64, error) {
	return 0, NoDiffError{}
}

func (n *NoDiff) CacheKey() DiffStoreKey {
	return ""
}

func (n *NoDiff) Init(context.Context) error {
	return NoDiffError{}
}

func (n *NoDiff) BlockSize() int64 {
	return 0
}

func GenerateDiffCachePath(basePath string, buildId string, diffType DiffType) string {
	cachePathSuffix := id.Generate()

	cacheFile := fmt.Sprintf("%s-%s-%s", buildId, diffType, cachePathSuffix)

	return filepath.Join(basePath, cacheFile)
}
