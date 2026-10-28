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

package storage

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/google/uuid"
)

type TemplateCacheFiles struct {
	TemplateFiles

	// CacheIdentifier is used to distinguish between each entry in the cache to prevent deleting the cache files when the template cache entry is being closed and a new one is being created.
	CacheIdentifier string

	config Config
}

func (t TemplateFiles) CacheFiles(config Config) (TemplateCacheFiles, error) {
	identifier, err := uuid.NewRandom()
	if err != nil {
		return TemplateCacheFiles{}, fmt.Errorf("failed to generate identifier: %w", err)
	}

	tcf := TemplateCacheFiles{
		TemplateFiles:   t,
		CacheIdentifier: identifier.String(),
		config:          config,
	}

	cacheDir := tcf.cacheDir()

	err = os.MkdirAll(cacheDir, os.ModePerm)
	if err != nil {
		return TemplateCacheFiles{}, fmt.Errorf("failed to create cache dir '%s': %w", cacheDir, err)
	}

	return tcf, nil
}

func (c TemplateCacheFiles) CacheSnapfilePath() string {
	return filepath.Join(c.cacheDir(), SnapfileName)
}

func (c TemplateCacheFiles) CacheMetadataPath() string {
	return filepath.Join(c.cacheDir(), MetadataName)
}

func (c TemplateCacheFiles) cacheDir() string {
	return filepath.Join(c.config.TemplateCacheDir, c.BuildID, "cache", c.CacheIdentifier)
}

func (c TemplateCacheFiles) Close() error {
	return os.RemoveAll(c.cacheDir())
}
