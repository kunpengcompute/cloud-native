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

package template

import (
	"context"
	"errors"
	"fmt"
	"os"

	"gitcode.com/cloud-native/cloud-native/TemplateCreator/internal/upstream/shared/pkg/storage"
)

type storageFile struct {
	path string
}

func newStorageFile(
	ctx context.Context,
	persistence storage.StorageProvider,
	objectPath string,
	path string,
	objectType storage.ObjectType,
) (*storageFile, error) {
	f, err := os.Create(path)
	if err != nil {
		return nil, fmt.Errorf("failed to create file: %w", err)
	}

	defer f.Close()

	object, err := persistence.OpenBlob(ctx, objectPath, objectType)
	if err != nil {
		return nil, err
	}

	_, err = object.WriteTo(ctx, f)
	if err != nil {
		cleanupErr := os.Remove(path)

		return nil, fmt.Errorf("NEW STORAGE failed to write to file: %w", errors.Join(err, cleanupErr))
	}

	return &storageFile{
		path: path,
	}, nil
}

func (f *storageFile) Path() string {
	return f.path
}

func (f *storageFile) Close() error {
	return os.RemoveAll(f.path)
}
