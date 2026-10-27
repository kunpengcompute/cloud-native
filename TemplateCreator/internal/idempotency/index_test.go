package idempotency

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

func TestIndexWithKeyCommitsAndReusesCompletedResult(t *testing.T) {
	index, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	var calls atomic.Int32
	build := func() (json.RawMessage, error) {
		calls.Add(1)
		return json.RawMessage(`{"build_id":"one"}`), nil
	}
	first, reused, err := index.WithKey(context.Background(), "request-key", "digest-a", build)
	if err != nil || reused {
		t.Fatalf("first = %s, reused=%v, err=%v", first, reused, err)
	}
	second, reused, err := index.WithKey(context.Background(), "request-key", "digest-a", build)
	if err != nil || !reused || string(second) != string(first) {
		t.Fatalf("second = %s, reused=%v, err=%v", second, reused, err)
	}
	if calls.Load() != 1 {
		t.Fatalf("build calls = %d, want 1", calls.Load())
	}
}

func TestIndexWithKeyRejectsDigestConflict(t *testing.T) {
	index, _ := New(t.TempDir())
	_, _, err := index.WithKey(context.Background(), "request-key", "digest-a", func() (json.RawMessage, error) { return json.RawMessage(`{}`), nil })
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = index.WithKey(context.Background(), "request-key", "digest-b", func() (json.RawMessage, error) { return json.RawMessage(`{}`), nil })
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("conflict error = %v", err)
	}
}

func TestIndexWithKeyRejectsCorruptRecord(t *testing.T) {
	root := t.TempDir()
	index, _ := New(root)
	path := index.recordPath("request-key")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("not-json"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, _, err := index.WithKey(context.Background(), "request-key", "digest", func() (json.RawMessage, error) { return nil, nil })
	if err == nil || !strings.Contains(err.Error(), "corrupt") {
		t.Fatalf("corrupt error = %v", err)
	}
}

func TestIndexWithKeySerializesConcurrentSameKeyCalls(t *testing.T) {
	index, _ := New(t.TempDir())
	var calls atomic.Int32
	start := make(chan struct{})
	build := func() (json.RawMessage, error) {
		calls.Add(1)
		<-start
		return json.RawMessage(`{"ok":true}`), nil
	}
	const workers = 8
	var group sync.WaitGroup
	errorsCh := make(chan error, workers)
	for range workers {
		group.Add(1)
		go func() {
			defer group.Done()
			_, _, err := index.WithKey(context.Background(), "same", "digest", build)
			errorsCh <- err
		}()
	}
	for calls.Load() == 0 {
	}
	close(start)
	group.Wait()
	close(errorsCh)
	for err := range errorsCh {
		if err != nil {
			t.Fatal(err)
		}
	}
	if calls.Load() != 1 {
		t.Fatalf("build calls = %d, want 1", calls.Load())
	}
}

func TestIndexWithKeyLeavesNoRecordOrTemporaryFileOnBuildFailure(t *testing.T) {
	root := t.TempDir()
	index, _ := New(root)
	want := errors.New("build failed")
	_, _, err := index.WithKey(context.Background(), "key", "digest", func() (json.RawMessage, error) { return nil, want })
	if !errors.Is(err, want) {
		t.Fatalf("error = %v", err)
	}
	entries, err := os.ReadDir(index.directory)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("idempotency directory entries = %v, want empty", entries)
	}
}
