package runtime

import (
	"context"
	"errors"
	"sync"
)

// resource 表示 managedRuntime 能按统一顺序释放的最小资源。
type resource interface {
	Close(context.Context) error
}

// resourceInitializer 创建一个资源；成功返回的资源所有权转交给 managedRuntime。
type resourceInitializer func(context.Context) (resource, error)

// managedRuntime 是不依赖 Linux privileged resource 的小型生命周期模型，供 rollback 测试使用。
type managedRuntime struct {
	resources []resource
	closeOnce sync.Once
	closeErr  error
}

// newManagedRuntime 按顺序初始化资源；任何一步失败或 context 取消都会逆序回滚已完成项。
func newManagedRuntime(ctx context.Context, initializers []resourceInitializer) (*managedRuntime, error) {
	runtime := &managedRuntime{resources: make([]resource, 0, len(initializers))}
	for _, initialize := range initializers {
		if err := ctx.Err(); err != nil {
			return nil, errors.Join(err, closeResources(context.Background(), runtime.resources))
		}
		initialized, err := initialize(ctx)
		if err != nil {
			return nil, errors.Join(err, closeResources(context.Background(), runtime.resources))
		}
		if initialized != nil {
			runtime.resources = append(runtime.resources, initialized)
		}
	}

	return runtime, nil
}

// Close 逆序释放全部资源，并保证重复调用只执行一次真实关闭。
func (runtime *managedRuntime) Close(ctx context.Context) error {
	runtime.closeOnce.Do(func() {
		runtime.closeErr = closeResources(ctx, runtime.resources)
	})

	return runtime.closeErr
}

// closeResources 像展开调用栈一样逆序关闭资源，并保留所有关闭错误。
func closeResources(ctx context.Context, resources []resource) error {
	var result error
	// 逆序能够尊重资源间的初始化依赖；errors.Join 防止一个错误跳过后续 cleanup。
	for i := len(resources) - 1; i >= 0; i-- {
		result = errors.Join(result, resources[i].Close(ctx))
	}

	return result
}
