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

package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
	"go.uber.org/zap"

	featureflags "gitcode.com/boostkit/cloud-native.git/TemplateCreator/internal/upstream/shared/pkg/featureflags"
	"gitcode.com/boostkit/cloud-native.git/TemplateCreator/internal/upstream/shared/pkg/logger"
)

const (
	cacheFilePermissions = 0o600
	cacheDirPermissions  = 0o700
)

type cache struct {
	rootPath  string
	chunkSize int64
	inner     StorageProvider
	flags     *featureflags.Client

	tracer trace.Tracer
}

var _ StorageProvider = (*cache)(nil)

func WrapInNFSCache(
	ctx context.Context,
	rootPath string,
	inner StorageProvider,
	flags *featureflags.Client,
) StorageProvider {
	cacheTracer := tracer

	createCacheSpans := flags.BoolFlag(ctx, featureflags.CreateStorageCacheSpansFlag)
	if !createCacheSpans {
		cacheTracer = noop.NewTracerProvider().Tracer("gitcode.com/boostkit/cloud-native.git/TemplateCreator/internal/upstream/shared/pkg/storage")
	}

	return &cache{
		rootPath:  rootPath,
		inner:     inner,
		chunkSize: MemoryChunkSize,
		flags:     flags,
		tracer:    cacheTracer,
	}
}

func (c cache) DeleteObjectsWithPrefix(ctx context.Context, prefix string) error {
	// no need to wait for cache deletion before returning
	go func(ctx context.Context) {
		c.deleteCachedObjectsWithPrefix(ctx, prefix)
	}(context.WithoutCancel(ctx))

	return c.inner.DeleteObjectsWithPrefix(ctx, prefix)
}

func (c cache) UploadSignedURL(ctx context.Context, path string, ttl time.Duration) (string, error) {
	return c.inner.UploadSignedURL(ctx, path, ttl)
}

func (c cache) OpenBlob(ctx context.Context, path string, objectType ObjectType) (Blob, error) {
	innerObject, err := c.inner.OpenBlob(ctx, path, objectType)
	if err != nil {
		return nil, fmt.Errorf("failed to open object: %w", err)
	}

	localPath := filepath.Join(c.rootPath, path)
	if err = os.MkdirAll(localPath, cacheDirPermissions); err != nil {
		return nil, fmt.Errorf("failed to create cache directory: %w", err)
	}

	return &cachedBlob{
		path:      localPath,
		chunkSize: c.chunkSize,
		inner:     innerObject,
		flags:     c.flags,
		tracer:    c.tracer,
	}, nil
}

func (c cache) OpenSeekable(ctx context.Context, path string, objectType SeekableObjectType) (Seekable, error) {
	innerObject, err := c.inner.OpenSeekable(ctx, path, objectType)
	if err != nil {
		return nil, fmt.Errorf("failed to open object: %w", err)
	}

	localPath := filepath.Join(c.rootPath, path)
	if err = os.MkdirAll(localPath, cacheDirPermissions); err != nil {
		return nil, fmt.Errorf("failed to create cache directory: %w", err)
	}

	return &cachedSeekable{
		path:      localPath,
		chunkSize: c.chunkSize,
		inner:     innerObject,
		flags:     c.flags,
		tracer:    c.tracer,
	}, nil
}

func (c cache) GetDetails() string {
	return fmt.Sprintf("[Caching file storage, base path set to %s, which wraps %s]",
		c.rootPath, c.inner.GetDetails())
}

func (c cache) deleteCachedObjectsWithPrefix(ctx context.Context, prefix string) {
	fullPrefix := filepath.Join(c.rootPath, prefix)
	if err := os.RemoveAll(fullPrefix); err != nil {
		logger.L().Error(ctx, "failed to remove object with prefix",
			zap.String("prefix", prefix),
			zap.String("path", fullPrefix),
			zap.Error(err))
	}
}

func ignoreEOF(err error) error {
	if errors.Is(err, io.EOF) {
		return nil
	}

	return err
}
