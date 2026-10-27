//go:build linux

package runtime

import (
	"context"
	"errors"
	"testing"
)

func TestProcessLifetimeSurvivesInitializationContextCancellation(t *testing.T) {
	initializationCtx, cancelInitialization := context.WithCancel(context.Background())
	processCtx, stopProcess := newProcessLifetimeContext(initializationCtx)
	cancelInitialization()
	if err := processCtx.Err(); err != nil {
		t.Fatalf("runtime stopped with initialization context: %v", err)
	}
	stopProcess()
	if !errors.Is(processCtx.Err(), context.Canceled) {
		t.Fatalf("runtime after explicit stop = %v, want canceled", processCtx.Err())
	}
}

type shutdownRetryProxy struct {
	cancel context.CancelFunc
	calls  int
}

func (proxy *shutdownRetryProxy) Close(ctx context.Context) error {
	proxy.calls++
	if proxy.calls == 1 {
		// 模拟 graceful Shutdown 运行期间到期；第二次调用应进入强制 Close 路径。
		proxy.cancel()
		return ctx.Err()
	}
	return nil
}

func TestCloseSandboxProxyForcesCloseAfterGracefulTimeout(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	proxy := &shutdownRetryProxy{cancel: cancel}
	err := closeSandboxProxy(ctx, proxy)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("close error = %v, want canceled graceful shutdown", err)
	}
	if proxy.calls != 2 {
		t.Fatalf("proxy close calls = %d, want graceful then forced close", proxy.calls)
	}
}
