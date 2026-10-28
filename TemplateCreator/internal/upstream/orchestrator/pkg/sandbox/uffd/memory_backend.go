// TemplateCreator modification: upstream import paths rewritten; no behavioral changes.

package uffd

import (
	"context"

	"gitcode.com/cloud-native/cloud-native/TemplateCreator/internal/upstream/orchestrator/pkg/sandbox/block"
	"gitcode.com/cloud-native/cloud-native/TemplateCreator/internal/upstream/orchestrator/pkg/sandbox/fc"
	"gitcode.com/cloud-native/cloud-native/TemplateCreator/internal/upstream/shared/pkg/storage/header"
	"gitcode.com/cloud-native/cloud-native/TemplateCreator/internal/upstream/shared/pkg/utils"
)

type MemoryBackend interface {
	DiffMetadata(ctx context.Context, f *fc.Process) (*header.DiffMetadata, error)
	PrefetchData(ctx context.Context) (block.PrefetchData, error)
	Prefault(ctx context.Context, offset int64, data []byte) error
	Start(ctx context.Context, sandboxId string) error
	Stop() error
	Ready() chan struct{}
	Exit() *utils.ErrorOnce
}
