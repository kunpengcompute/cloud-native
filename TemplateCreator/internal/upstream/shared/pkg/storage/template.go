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
)

const (
	GuestEnvdPath = "/usr/bin/envd"

	MemfileName  = "memfile"
	RootfsName   = "rootfs.ext4"
	SnapfileName = "snapfile"
	MetadataName = "metadata.json"

	HeaderSuffix = ".header"
)

type TemplateFiles struct {
	BuildID string `json:"build_id"`
}

// Key for the cache. Unique for template-build pair.
func (t TemplateFiles) CacheKey() string {
	return t.BuildID
}

func (t TemplateFiles) StorageDir() string {
	return t.BuildID
}

func (t TemplateFiles) StorageMemfilePath() string {
	return fmt.Sprintf("%s/%s", t.StorageDir(), MemfileName)
}

func (t TemplateFiles) StorageMemfileHeaderPath() string {
	return fmt.Sprintf("%s/%s%s", t.StorageDir(), MemfileName, HeaderSuffix)
}

func (t TemplateFiles) StorageRootfsPath() string {
	return fmt.Sprintf("%s/%s", t.StorageDir(), RootfsName)
}

func (t TemplateFiles) StorageRootfsHeaderPath() string {
	return fmt.Sprintf("%s/%s%s", t.StorageDir(), RootfsName, HeaderSuffix)
}

func (t TemplateFiles) StorageSnapfilePath() string {
	return fmt.Sprintf("%s/%s", t.StorageDir(), SnapfileName)
}

func (t TemplateFiles) StorageMetadataPath() string {
	return fmt.Sprintf("%s/%s", t.StorageDir(), MetadataName)
}
