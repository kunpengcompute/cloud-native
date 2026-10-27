//go:build linux

package basecache

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"
)

func TestFileLockSerializesInstancesAndReleases(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cache.lock")
	first := NewFileLock(path)
	second := NewFileLock(path)

	firstLease, err := first.Acquire(context.Background())
	if err != nil {
		t.Fatalf("first Acquire() error = %v", err)
	}

	acquired := make(chan *LockLease, 1)
	errorsCh := make(chan error, 1)
	go func() {
		lease, acquireErr := second.Acquire(context.Background())
		if acquireErr != nil {
			errorsCh <- acquireErr
			return
		}
		acquired <- lease
	}()

	select {
	case lease := <-acquired:
		_ = lease.Release()
		t.Fatal("second lock acquired before first release")
	case err := <-errorsCh:
		t.Fatalf("second Acquire() returned early: %v", err)
	case <-time.After(40 * time.Millisecond):
	}

	if err := firstLease.Release(); err != nil {
		t.Fatalf("first Release() error = %v", err)
	}
	select {
	case lease := <-acquired:
		if err := lease.Release(); err != nil {
			t.Fatalf("second Release() error = %v", err)
		}
	case err := <-errorsCh:
		t.Fatalf("second Acquire() error = %v", err)
	case <-time.After(time.Second):
		t.Fatal("second lock did not acquire after first release")
	}
}

func TestFileLockWaitRespondsToContextCancellation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cache.lock")
	held, err := NewFileLock(path).Acquire(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer held.Release()

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, acquireErr := NewFileLock(path).Acquire(ctx)
		done <- acquireErr
	}()
	time.Sleep(30 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Acquire() error = %v, want context.Canceled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("canceled lock waiter did not return")
	}
}

func TestFileLockReleaseIsIdempotent(t *testing.T) {
	lease, err := NewFileLock(filepath.Join(t.TempDir(), "cache.lock")).Acquire(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if err := lease.Release(); err != nil {
		t.Fatal(err)
	}
	if err := lease.Release(); err != nil {
		t.Fatalf("second Release() error = %v", err)
	}
}
