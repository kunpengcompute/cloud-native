//go:build linux

package basecache

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

const lockRetryInterval = 10 * time.Millisecond

// FileLock 使用宿主文件系统 flock 串行化同一 ABI namespace 的 Builder 写入。
// 每次 Acquire 都打开独立 file description，因此同进程与跨进程调用使用同一语义。
type FileLock struct {
	path string
}

// LockLease 表示已经取得的排他锁；调用方必须调用 Release。
type LockLease struct {
	file        *os.File
	releaseOnce sync.Once
	releaseErr  error
}

// NewFileLock 创建一个轻量锁句柄；实际文件在 Acquire 时打开。
func NewFileLock(path string) *FileLock {
	return &FileLock{path: path}
}

// Acquire 等待排他锁，并在等待期间响应 context 取消。
func (lock *FileLock) Acquire(ctx context.Context) (*LockLease, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(lock.path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open base cache lock %q: %w", lock.path, err)
	}
	for {
		if err := ctx.Err(); err != nil {
			return nil, errors.Join(err, file.Close())
		}
		err := unix.Flock(int(file.Fd()), unix.LOCK_EX|unix.LOCK_NB)
		if err == nil {
			return &LockLease{file: file}, nil
		}
		if !errors.Is(err, syscall.EWOULDBLOCK) && !errors.Is(err, syscall.EAGAIN) && !errors.Is(err, syscall.EINTR) {
			return nil, errors.Join(fmt.Errorf("acquire base cache lock %q: %w", lock.path, err), file.Close())
		}

		timer := time.NewTimer(lockRetryInterval)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return nil, errors.Join(ctx.Err(), file.Close())
		case <-timer.C:
		}
	}
}

// Release 释放 flock 和底层文件描述符；重复调用返回第一次释放结果。
func (lease *LockLease) Release() error {
	if lease == nil {
		return nil
	}
	lease.releaseOnce.Do(func() {
		unlockErr := unix.Flock(int(lease.file.Fd()), unix.LOCK_UN)
		closeErr := lease.file.Close()
		if unlockErr != nil || closeErr != nil {
			lease.releaseErr = errors.Join(unlockErr, closeErr)
		}
	})

	return lease.releaseErr
}
