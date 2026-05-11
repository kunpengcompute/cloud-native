package dynamiccontrol

import (
	"context"
	"testing"
	"time"
)

func TestSyncSchedulerStartValidate(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	r := &SyncScheduler{}
	if err := r.Start(ctx); err == nil {
		t.Fatalf("expected error when coordinator is nil")
	}

	r.Coordinator = &Coordinator{}
	r.PublishInterval = 0
	r.ApplyInterval = time.Second
	r.TaskTimeout = time.Second
	if err := r.Start(ctx); err == nil {
		t.Fatalf("expected error when publish interval <= 0")
	}

	r.PublishInterval = time.Second
	r.ApplyInterval = 0
	if err := r.Start(ctx); err == nil {
		t.Fatalf("expected error when apply interval <= 0")
	}

	r.ApplyInterval = time.Second
	r.TaskTimeout = 0
	if err := r.Start(ctx); err == nil {
		t.Fatalf("expected error when task timeout <= 0")
	}
}

func TestSyncSchedulerRunWithTimeout(t *testing.T) {
	t.Parallel()

	r := &SyncScheduler{TaskTimeout: 5 * time.Millisecond}
	ctx := context.Background()

	called := false
	r.runWithTimeout(ctx, "t1", func(taskCtx context.Context) error {
		called = true
		<-taskCtx.Done()
		return nil
	})
	if !called {
		t.Fatalf("expected task to be called")
	}
}

