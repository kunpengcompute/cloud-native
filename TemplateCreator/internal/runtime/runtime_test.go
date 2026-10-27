package runtime

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

type recordingResource struct {
	name   string
	closed *[]string
	err    error
}

func (resource *recordingResource) Close(context.Context) error {
	*resource.closed = append(*resource.closed, resource.name)
	return resource.err
}

func TestNewManagedRuntimeRollsBackInitializedResources(t *testing.T) {
	var closed []string
	wantErr := errors.New("pool initialization failed")
	initializers := []resourceInitializer{
		func(context.Context) (resource, error) {
			return &recordingResource{name: "proxy", closed: &closed}, nil
		},
		func(context.Context) (resource, error) {
			return nil, wantErr
		},
	}

	_, err := newManagedRuntime(context.Background(), initializers)
	if !errors.Is(err, wantErr) {
		t.Fatalf("newManagedRuntime() error = %v, want %v", err, wantErr)
	}
	if !reflect.DeepEqual(closed, []string{"proxy"}) {
		t.Fatalf("rollback close order = %v, want [proxy]", closed)
	}
}

func TestManagedRuntimeClosesInReverseOrderOnlyOnce(t *testing.T) {
	var closed []string
	initializers := []resourceInitializer{
		func(context.Context) (resource, error) {
			return &recordingResource{name: "proxy", closed: &closed}, nil
		},
		func(context.Context) (resource, error) { return &recordingResource{name: "pool", closed: &closed}, nil },
	}

	runtime, err := newManagedRuntime(context.Background(), initializers)
	if err != nil {
		t.Fatal(err)
	}
	if err := runtime.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := runtime.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(closed, []string{"pool", "proxy"}) {
		t.Fatalf("close order = %v, want [pool proxy]", closed)
	}
}

func TestNewManagedRuntimePreservesInitializationAndRollbackErrors(t *testing.T) {
	var closed []string
	initErr := errors.New("pool initialization failed")
	closeErr := errors.New("proxy close failed")
	_, err := newManagedRuntime(context.Background(), []resourceInitializer{
		func(context.Context) (resource, error) {
			return &recordingResource{name: "proxy", closed: &closed, err: closeErr}, nil
		},
		func(context.Context) (resource, error) { return nil, initErr },
	})
	if !errors.Is(err, initErr) || !errors.Is(err, closeErr) {
		t.Fatalf("newManagedRuntime() error = %v, want joined init and close errors", err)
	}
}

func TestNewManagedRuntimeStopsOnCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	called := false
	_, err := newManagedRuntime(ctx, []resourceInitializer{
		func(context.Context) (resource, error) {
			called = true
			return nil, nil
		},
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("newManagedRuntime() error = %v, want context canceled", err)
	}
	if called {
		t.Fatal("initializer called after context cancellation")
	}
}
