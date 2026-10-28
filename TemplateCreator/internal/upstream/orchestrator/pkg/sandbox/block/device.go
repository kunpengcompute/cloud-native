// TemplateCreator modification: upstream import paths rewritten; no behavioral changes.

package block

import (
	"context"
	"io"

	"gitcode.com/cloud-native/cloud-native/TemplateCreator/internal/upstream/shared/pkg/storage"
	"gitcode.com/cloud-native/cloud-native/TemplateCreator/internal/upstream/shared/pkg/storage/header"
)

type BytesNotAvailableError struct{}

func (BytesNotAvailableError) Error() string {
	return "The requested bytes are not available on the device"
}

type Slicer interface {
	Slice(ctx context.Context, off, length int64) ([]byte, error)
	BlockSize() int64
}

type ReadonlyDevice interface {
	storage.SeekableReader
	io.Closer
	Slicer
	BlockSize() int64
	Header() *header.Header
}

type Device interface {
	ReadonlyDevice
	io.WriterAt
}
