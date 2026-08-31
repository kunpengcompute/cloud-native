package dynamiccontrol

import (
	"context"
	"errors"
	"testing"
	"time"
)

type notifyingEngine struct {
	called chan string
}

func (e *notifyingEngine) EnsurePolicy(_ context.Context, nodeName string) error {
	e.called <- nodeName
	return nil
}

func (e *notifyingEngine) HandleInterference(_ context.Context, _ string, _ AgentAnalyzeResult) error {
	return nil
}

type failingEngine struct {
	err error
}

func (e *failingEngine) EnsurePolicy(_ context.Context, _ string) error {
	return e.err
}

func (e *failingEngine) HandleInterference(_ context.Context, _ string, _ AgentAnalyzeResult) error {
	return nil
}

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
	if err := r.runWithTimeout(ctx, func(taskCtx context.Context) error {
		called = true
		<-taskCtx.Done()
		return nil
	}); err != nil {
		t.Fatalf("runWithTimeout() unexpected error: %v", err)
	}
	if !called {
		t.Fatalf("expected task to be called")
	}
}

func TestSyncSchedulerStartEnsuresPolicyBeforeAgentIsAvailable(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	engine := &notifyingEngine{called: make(chan string, 1)}
	r := NewSyncScheduler(&Coordinator{
		NodeIdentity: fakeNodeIdentity{name: "node-a"},
		OnlineSource: fakeOnlineSource{},
		Agent:        &fakeAgentClient{getErr: errors.New("agent unavailable")},
		Engine:       engine,
		Clock:        time.Now,
	})
	r.PublishInterval = time.Hour
	r.ApplyInterval = time.Hour
	r.TaskTimeout = time.Second

	done := make(chan error, 1)
	go func() {
		done <- r.Start(ctx)
	}()

	select {
	case nodeName := <-engine.called:
		if nodeName != "node-a" {
			t.Fatalf("expected initialization for node-a, got %q", nodeName)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("expected policy initialization before querying the agent")
	}

	cancel()
	if err := <-done; err != nil {
		t.Fatalf("Start() unexpected error: %v", err)
	}
}

func TestSyncSchedulerStartReturnsPolicyInitializationError(t *testing.T) {
	want := errors.New("create policy failed")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	r := NewSyncScheduler(&Coordinator{
		NodeIdentity: fakeNodeIdentity{name: "node-a"},
		OnlineSource: fakeOnlineSource{},
		Agent:        &fakeAgentClient{},
		Engine:       &failingEngine{err: want},
		Clock:        time.Now,
	})

	err := r.Start(ctx)
	if !errors.Is(err, want) {
		t.Fatalf("Start() expected error %v, got %v", want, err)
	}
}
