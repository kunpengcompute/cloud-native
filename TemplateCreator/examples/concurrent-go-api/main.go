// 并发构建参考程序：集中修改下方参数后，在具备 KVM/NBD 的 Linux 构建机运行。
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	templatecreator "gitcode.com/cloud-native/cloud-native/TemplateCreator"
)

// ---------- 基础参数：构建机路径、超时和本地监听端口 ----------
const (
	workRoot     = "/opt/templatecreator"
	manifestPath = "/opt/templatecreator/runtime-assets/manifest.json" // 与目标 E2B runtime SHA 匹配。
	imageTarPath = "/opt/images/ubuntu-2204-full.tar"                  // ImageTarPath 默认空；不提供镜像时默认拉取 DefaultImageRef。
	buildTimeout = templatecreator.DefaultBuildTimeout                 // 30 分钟；Go API 不自动设置 context 截止时间。
	closeTimeout = 2 * time.Minute                                     // Close 没有内置等待超时，由调用方限定。
	proxyPort    = uint16(5109)                                        // 默认 5007；与同机 E2B 服务端口冲突时才覆盖。
)

var runtimeRoot = filepath.Join(workRoot, "runtime")

// ---------- PrepareRuntime 参数：必填路径；架构默认 GOARCH，下载超时默认 5 分钟 ----------
var runtimeRequest = templatecreator.PrepareRuntimeRequest{
	ManifestPath: manifestPath,
	RuntimeRoot:  runtimeRoot,
}

// ---------- NewCreator 参数：仅覆盖并发数和进度日志，其余使用默认值 ----------
var creatorConfig = templatecreator.CreatorConfig{
	MaxParallel: 5,         // 默认 1；本例允许同一 Creator 同时执行五个物理构建。
	LogWriter:   os.Stderr, // 默认 nil 且不输出运行日志；提供 writer 后默认 info 级别。
}

// ---------- CreateTemplate 参数：默认幂等键为空、Labels 为 nil；本例分别设置五份请求 ----------
var requests = []templatecreator.CreateRequest{
	// nil BuildOverride：完全继承 Creator 默认资源、格式和发布模式。
	{
		ImageTarPath:   imageTarPath,
		IdempotencyKey: "example-base-v1",
		Labels:         map[string]string{"profile": "base"},
	},
	// HugePages 默认 false；启用前确认宿主预留页足够。其他资源继承默认 2/1024/4096。
	{
		ImageTarPath: imageTarPath,
		BuildOverride: &templatecreator.BuildSpec{
			HugePages: true,
		},
		IdempotencyKey: "example-huge-v1",
		Labels:         map[string]string{"profile": "huge"},
	},
	// vCPU 默认 2，此处改为 3；数值 0 继承默认值，非 nil override 的 HugePages 零值为 false。
	{
		ImageTarPath: imageTarPath,
		BuildOverride: &templatecreator.BuildSpec{
			VCPU: 3,
		},
		IdempotencyKey: "example-partial-v1",
		Labels:         map[string]string{"profile": "partial"},
	},
	// 内存默认 1024 MiB，空闲根盘默认 4096 MiB；这里分别改为 512 和 2048 MiB。
	{
		ImageTarPath: imageTarPath,
		BuildOverride: &templatecreator.BuildSpec{
			MemoryMB: 512, DiskSizeMB: 2048,
		},
		IdempotencyKey: "example-small-v1",
		Labels:         map[string]string{"profile": "small"},
	},
	// vCPU 默认 2、空闲根盘默认 4096 MiB、命令默认空、Steps 默认由格式决定。
	// Format 默认 fromImage、Publication 默认 all；显式 Steps 会整体替换格式默认步骤。
	{
		ImageTarPath: imageTarPath,
		BuildOverride: &templatecreator.BuildSpec{
			VCPU: 1, DiskSizeMB: 2048,
			StartCommand: "sh -c 'touch /tmp/example-ready; sleep 3600'",
			ReadyCommand: "test -f /tmp/example-ready",
			Steps: []templatecreator.TemplateStep{
				{Type: "USER", Args: []string{"root"}},
				{Type: "WORKDIR", Args: []string{"/"}},
				{Type: "RUN", Args: []string{"printf example-ok > /home/user/example-marker", "root"}},
				{Type: "USER", Args: []string{"user"}},
				{Type: "WORKDIR", Args: []string{"/home/user"}},
			},
		},
		Format:         templatecreator.BuildFormatFromDockerfile,
		Publication:    templatecreator.PublicationSimple,
		IdempotencyKey: "example-steps-v1",
		Labels:         map[string]string{"profile": "steps"},
	},
}

type outcome struct {
	result templatecreator.CreateResult
	err    error
}

// prepareRuntime 创建独立目录，再根据清单校验四项运行资产。
func prepareRuntime(ctx context.Context) (templatecreator.RuntimeConfig, error) {
	for _, root := range []string{runtimeRoot, templatecreator.DefaultScratchRoot, templatecreator.DefaultStoreRoot} {
		if err := os.MkdirAll(root, 0o755); err != nil {
			return templatecreator.RuntimeConfig{}, fmt.Errorf("create %s: %w", root, err)
		}
	}
	prepared, err := templatecreator.PrepareRuntime(ctx, runtimeRequest)
	if err != nil {
		return templatecreator.RuntimeConfig{}, fmt.Errorf("prepare runtime: %w", err)
	}
	return prepared.Runtime, nil
}

// newCreator 把已校验的 RuntimeConfig 注入前面定义的进程级配置。
func newCreator(ctx context.Context, runtimeConfig templatecreator.RuntimeConfig) (*templatecreator.Creator, error) {
	config := creatorConfig
	config.Runtime = runtimeConfig
	config.Runtime.ProxyPort = proxyPort
	return templatecreator.NewCreator(ctx, config)
}

// buildConcurrently 发起所有同步请求，Creator 自己用 MaxParallel 限制物理构建。
func buildConcurrently(ctx context.Context, creator *templatecreator.Creator) []outcome {
	outcomes := make([]outcome, len(requests))
	var group sync.WaitGroup
	for index := range requests {
		group.Add(1)
		go func(index int) {
			defer group.Done()
			outcomes[index].result, outcomes[index].err = creator.CreateTemplate(ctx, requests[index])
		}(index)
	}
	group.Wait()
	return outcomes
}

// reportResults 按请求顺序打印已完成产物，保留每项失败的任务编号。
func reportResults(outcomes []outcome) error {
	var failures []error
	for index, outcome := range outcomes {
		key := requests[index].IdempotencyKey
		if outcome.err != nil {
			failures = append(failures, fmt.Errorf("%s: %w", key, outcome.err))
			continue
		}
		fmt.Fprintf(os.Stderr, "%s: TemplateID=%s BuildID=%s ManifestKey=%s RegistrationSQL=%s\n",
			key, outcome.result.TemplateID, outcome.result.BuildID,
			outcome.result.Artifacts.ManifestKey, outcome.result.Registration.LocalPath)
	}
	return errors.Join(failures...)
}

// run 只按阶段调用公开 API；无论构建是否失败都等待 Creator 关闭。
func run() (runErr error) {
	ctx, cancel := context.WithTimeout(context.Background(), buildTimeout)
	defer cancel()

	// ---------- PrepareRuntime：准备并校验运行资产 ----------
	runtimeConfig, err := prepareRuntime(ctx)
	if err != nil {
		return err
	}
	// ---------- NewCreator：初始化共享的进程级资源 ----------
	creator, err := newCreator(ctx, runtimeConfig)
	if err != nil {
		return fmt.Errorf("new Creator: %w", err)
	}
	defer func() {
		// 关闭时使用独立 context，避免构建超时影响资源清理。
		closeCtx, closeCancel := context.WithTimeout(context.Background(), closeTimeout)
		defer closeCancel()
		runErr = errors.Join(runErr, creator.Close(closeCtx))
	}()

	// ---------- CreateTemplate：并发执行并逐项处理结果 ----------
	return reportResults(buildConcurrently(ctx, creator))
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
