// TemplateCreator modification: upstream import paths rewritten; no behavioral changes.

package sandbox

import (
	"context"
	"io"

	"gitcode.com/cloud-native/cloud-native/TemplateCreator/internal/upstream/orchestrator/pkg/sandbox/rootfs"
	"gitcode.com/cloud-native/cloud-native/TemplateCreator/internal/upstream/shared/pkg/storage/header"
)

type DiffCreator interface {
	process(ctx context.Context, out io.Writer) (*header.DiffMetadata, error)
}

type RootfsDiffCreator struct {
	rootfs    rootfs.Provider
	closeHook func(context.Context) error
}

func (r *RootfsDiffCreator) process(ctx context.Context, out io.Writer) (*header.DiffMetadata, error) {
	return r.rootfs.ExportDiff(ctx, out, r.closeHook)
}
