// Copyright (c) 2025 Huawei Technology corp.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

//go:build linux
// +build linux

package collector

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"strconv"
	"sync"
	"time"
)

// devkitCommandRunner 是 DevKit CLI 的可替换执行边界。正式环境使用
// runDevkitCommand，测试可注入确定性的 runner 而不启动外部进程。
type devkitCommandRunner func(ctx context.Context, binaryPath string, args ...string) (string, error)

// devkitAttemptMetadata 描述一次实际发送给 CLI 的采集目标和 Memory period。
// targetValue 保留无类型前缀的 CPU/PID 值，供 TopDown 报告 scope 交叉校验。
type devkitAttemptMetadata struct {
	targetType         string
	target             string
	targetValue        string
	periodMilliseconds int
}

// parseDevkitAttemptMetadata 从 Collector 已构造完成的 argv 生成指标标签元数据。
// 该函数只接受本 Collector 支持的严格参数集合，参数异常表示内部命令构造合同被破坏。
func parseDevkitAttemptMetadata(args []string) (devkitAttemptMetadata, error) {
	if len(args) < 2 || args[0] != "tuner" {
		return devkitAttemptMetadata{}, fmt.Errorf("DevKit argv is missing the tuner subcommand")
	}

	values := make(map[string]string, (len(args)-2)/2)
	for i := 2; i < len(args); i += 2 {
		if i+1 >= len(args) {
			return devkitAttemptMetadata{}, fmt.Errorf("DevKit argument %s is missing a value", args[i])
		}
		flag, value := args[i], args[i+1]
		if value == "" {
			return devkitAttemptMetadata{}, fmt.Errorf("DevKit argument %s has an empty value", flag)
		}
		if _, exists := values[flag]; exists {
			return devkitAttemptMetadata{}, fmt.Errorf("DevKit argument %s is duplicated", flag)
		}
		values[flag] = value
	}

	duration, err := strconv.Atoi(values["-d"])
	if err != nil || duration < minDevkitDuration || duration > maxDevkitDuration {
		return devkitAttemptMetadata{}, fmt.Errorf("DevKit duration is invalid: %q", values["-d"])
	}

	metadata := devkitAttemptMetadata{targetType: "system", target: "system"}
	switch args[1] {
	case "top-down":
		for flag := range values {
			if flag != "-d" && flag != "-L" && flag != "-c" && flag != "-p" {
				return devkitAttemptMetadata{}, fmt.Errorf("TopDown argv contains unsupported argument: %s", flag)
			}
		}
		if values["-L"] != "0" {
			return devkitAttemptMetadata{}, fmt.Errorf("TopDown profile level must be 0")
		}
		if values["-c"] != "" && values["-p"] != "" {
			return devkitAttemptMetadata{}, fmt.Errorf("TopDown argv cannot contain both CPU and PID")
		}
		if cpu := values["-c"]; cpu != "" {
			metadata.targetType = "cpu"
			metadata.target = "cpu" + cpu
			metadata.targetValue = cpu
		} else if pid := values["-p"]; pid != "" {
			metadata.targetType = "pid"
			metadata.target = "pid" + pid
			metadata.targetValue = pid
		}
	case "memory":
		for flag := range values {
			if flag != "-d" && flag != "-m" && flag != "-P" && flag != "-c" {
				return devkitAttemptMetadata{}, fmt.Errorf("Memory argv contains unsupported argument: %s", flag)
			}
		}
		if values["-m"] != "1" {
			return devkitAttemptMetadata{}, fmt.Errorf("Memory metric must be 1")
		}
		period, parseErr := strconv.Atoi(values["-P"])
		if parseErr != nil || (period != 100 && period != 1000) {
			return devkitAttemptMetadata{}, fmt.Errorf("Memory period is invalid: %q", values["-P"])
		}
		metadata.periodMilliseconds = period
		if cpu := values["-c"]; cpu != "" {
			metadata.targetType = "cpu"
			metadata.target = "cpu" + cpu
			metadata.targetValue = cpu
		}
	default:
		return devkitAttemptMetadata{}, fmt.Errorf("unsupported DevKit subcommand: %s", args[1])
	}

	return metadata, nil
}

// runDevkitCommand 执行 DevKit CLI，并在失败信息中保留 stderr 便于远程诊断。
func runDevkitCommand(ctx context.Context, binaryPath string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, binaryPath, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return "", fmt.Errorf("DevKit CLI timed out: %w (stderr: %s)", ctxErr, stderr.String())
		}
		return "", fmt.Errorf("%w (stderr: %s)", err, stderr.String())
	}
	return stdout.String(), nil
}

// devkitCollectionCoordinator 串行化同一进程内所有 DevKit 采集窗口。
// TopDown 与 Memory 并非 CLI 强制互斥；主动串行用于减少并发采样扰动，
// 保证不同采集轮次的数据精确度与可比性。
type devkitCollectionCoordinator struct {
	mu          sync.Mutex
	nextRoundID uint64
}

var sharedDevkitCollectionCoordinator devkitCollectionCoordinator

// run 在共享采集门内执行一次完整的 CLI 与解析过程。锁的生命周期由本函数
// 管理，调用方不能重复释放；即使日志或采集函数 panic，defer 也会释放全局门。
func (c *devkitCollectionCoordinator) run(
	logger *slog.Logger,
	collector string,
	binaryPath string,
	cliArgs []string,
	collect func(roundID uint64) error,
	configFields ...any,
) (roundID uint64, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.nextRoundID++
	roundID = c.nextRoundID
	startedAt := time.Now()

	fields := []any{
		"collector", collector,
		"round_id", roundID,
		"binary_path", binaryPath,
		"cli_args", append([]string(nil), cliArgs...),
	}
	fields = append(fields, configFields...)
	if logger != nil {
		logger.Info("collection_start", fields...)
	}

	defer func() {
		finishFields := append([]any(nil), fields...)
		finishFields = append(finishFields, "elapsed_ms", time.Since(startedAt).Milliseconds())

		if panicValue := recover(); panicValue != nil {
			finishFields = append(finishFields, "status", "panic", "panic", panicValue)
			if logger != nil {
				logger.Error("collection_finish", finishFields...)
			}
			panic(panicValue)
		}

		status := "success"
		if err != nil {
			status = "failed"
			finishFields = append(finishFields, "error", err)
		}
		finishFields = append(finishFields, "status", status)
		if logger != nil {
			logger.Info("collection_finish", finishFields...)
		}
	}()

	err = collect(roundID)
	return roundID, err
}
