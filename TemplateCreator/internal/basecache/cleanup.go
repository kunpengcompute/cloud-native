package basecache

import (
	"errors"
	"os"
)

// closeJoined 在错误路径关闭临时句柄，并把清理失败与主错误合并返回，
// 避免清理错误被静默丢弃。
func closeJoined(err error, temporary *os.File) error {
	if closeErr := temporary.Close(); closeErr != nil && !errors.Is(closeErr, os.ErrClosed) {
		return errors.Join(err, closeErr)
	}
	return err
}
