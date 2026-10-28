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

package utils

import (
	"context"
	"fmt"
	"os"

	"go.uber.org/zap"

	"gitcode.com/cloud-native/cloud-native/TemplateCreator/internal/upstream/shared/pkg/logger"
)

// RenameOrDeleteFile tries to rename a file but will not replace the target if it already exists.
// If the file already exists, the file will be deleted. The old file will always be deleted.
func RenameOrDeleteFile(ctx context.Context, oldPath, newPath string) error {
	return removeOrDeleteFile(ctx, oldPath, newPath, &osFns{})
}

func removeOrDeleteFile(ctx context.Context, oldPath, newPath string, os fileOps) error {
	defer func() {
		if err := os.Remove(oldPath); err != nil {
			logger.L().Warn(ctx, "failed to remove existing file",
				zap.Error(err),
				zap.String("path", oldPath),
			)
		}
	}()

	if err := os.Link(oldPath, newPath); err != nil {
		return fmt.Errorf("failed to create hard link: %w", err)
	}

	return nil
}

type osFns struct{}

func (o osFns) Link(oldPath, newPath string) error {
	return os.Link(oldPath, newPath)
}

func (o osFns) Remove(path string) error {
	return os.Remove(path)
}

var _ fileOps = (*osFns)(nil)

type fileOps interface {
	Link(oldPath, newPath string) error
	Remove(path string) error
}
