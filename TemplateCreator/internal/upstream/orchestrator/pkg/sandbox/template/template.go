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
	"io"

	"gitcode.com/boostkit/cloud-native.git/TemplateCreator/internal/upstream/orchestrator/pkg/sandbox/block"
	"gitcode.com/boostkit/cloud-native.git/TemplateCreator/internal/upstream/orchestrator/pkg/template/metadata"
	"gitcode.com/boostkit/cloud-native.git/TemplateCreator/internal/upstream/shared/pkg/storage"
)

type Template interface {
	Files() storage.TemplateCacheFiles
	Memfile(ctx context.Context) (block.ReadonlyDevice, error)
	Rootfs() (block.ReadonlyDevice, error)
	Snapfile() (File, error)
	Metadata() (metadata.Template, error)
	Close(ctx context.Context) error
}

func closeTemplate(ctx context.Context, t Template) (e error) {
	closable := make([]io.Closer, 0)

	memfile, err := t.Memfile(ctx)
	if err != nil {
		e = errors.Join(e, err)
	} else {
		closable = append(closable, memfile)
	}

	rootfs, err := t.Rootfs()
	if err != nil {
		e = errors.Join(e, err)
	} else {
		closable = append(closable, rootfs)
	}

	snapfile, err := t.Snapfile()
	if err != nil {
		e = errors.Join(e, err)
	} else {
		closable = append(closable, snapfile)
	}

	for _, c := range closable {
		if err := c.Close(); err != nil {
			e = errors.Join(e, err)
		}
	}

	if e != nil {
		return fmt.Errorf("error closing template: %w", e)
	}

	return nil
}

type NoopFile struct{}

func (n *NoopFile) Close() error {
	return nil
}

func (n *NoopFile) Path() string {
	return "/dev/null"
}
