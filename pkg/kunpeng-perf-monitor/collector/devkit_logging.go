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

package collector

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	// 日志状态机的资源上限使用包级变量，生产环境有确定边界，测试可注入短窗口。
	devkitFailureLogWindow          = 5 * time.Minute
	maxDevkitErrorSummaryCharacters = 512
	maxDevkitDebugOutputCharacters  = 2048
)

type devkitCollectionFailure struct {
	stage string
	err   error
}

func newDevkitCollectionFailure(stage string, err error) error {
	// 用 stage 包装错误，让统一日志状态机能区分 argv、CLI、Parser 等失败阶段，
	// 同时通过 Unwrap 保留底层 timeout/exit error 的 errors.Is/As 语义。
	return &devkitCollectionFailure{stage: stage, err: err}
}

func (e *devkitCollectionFailure) Error() string {
	return fmt.Sprintf("%s failed: %v", e.stage, e.err)
}

func (e *devkitCollectionFailure) Unwrap() error {
	return e.err
}

type devkitCollectionLogAttempt struct {
	roundID uint64
	started time.Time
	fields  []any
}

type devkitCollectionLogState struct {
	mu sync.Mutex

	logger    *slog.Logger
	collector string
	now       func() time.Time

	// 参数指纹用于只记录首次参数和真实变化；失败指纹用于抑制同一故障风暴。
	hasParameters         bool
	parametersFingerprint string
	failed                bool
	failureFingerprint    string
	failureLoggedAt       time.Time
}

func newDevkitCollectionLogState(logger *slog.Logger, collector string) *devkitCollectionLogState {
	// now 作为函数注入，既便于限频逻辑测试，也避免生产路径依赖全局时钟状态。
	return &devkitCollectionLogState{logger: logger, collector: collector, now: time.Now}
}

func (s *devkitCollectionLogState) start(
	roundID uint64,
	binaryPath string,
	cliArgs []string,
	configFields ...any,
) devkitCollectionLogAttempt {
	// start 在 INFO 记录稳定参数，在 DEBUG 记录每轮参数；返回的 attempt 保存同一
	// 份字段，保证 finish/失败日志不会重新从可能已变化的配置生成标签。
	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.now()
	fields := []any{
		"collector", s.collector,
		"round_id", roundID,
		"binary_path", binaryPath,
		"cli_args", append([]string(nil), cliArgs...),
	}
	fields = append(fields, configFields...)
	// NUL 分隔避免不同 argv 拼接后产生相同指纹，例如 ["ab", "c"] 与 ["a", "bc"]。
	fingerprint := binaryPath + "\x00" + strings.Join(cliArgs, "\x00")
	if !s.hasParameters {
		s.hasParameters = true
		s.parametersFingerprint = fingerprint
		if s.logger != nil {
			s.logger.Info("devkit_collection_parameters_initialized", fields...)
		}
	} else if fingerprint != s.parametersFingerprint {
		s.parametersFingerprint = fingerprint
		s.resetFailureLocked()
		if s.logger != nil {
			s.logger.Info("devkit_collection_parameters_changed", fields...)
		}
	}
	if s.logger != nil {
		s.logger.Debug("collection_start", fields...)
	}
	return devkitCollectionLogAttempt{roundID: roundID, started: now, fields: fields}
}

func (s *devkitCollectionLogState) finish(attempt devkitCollectionLogAttempt, collectionErr error) {
	// 成功轮只写 DEBUG；失败轮根据 fingerprint 限频，恢复时单独写一次 INFO。
	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.now()
	status := "success"
	if collectionErr != nil {
		status = "failed"
	}
	finishFields := append([]any(nil), attempt.fields...)
	finishFields = append(finishFields, "elapsed_ms", now.Sub(attempt.started).Milliseconds(), "status", status)
	if s.logger != nil {
		s.logger.Debug("collection_finish", finishFields...)
	}
	if collectionErr == nil {
		if s.failed && s.logger != nil {
			s.logger.Info("devkit_collection_recovered", finishFields...)
		}
		s.resetFailureLocked()
		return
	}

	stage := devkitFailureStage(collectionErr)
	summary, _ := truncateDevkitLogText(normalizeDevkitLogText(collectionErr.Error()), maxDevkitErrorSummaryCharacters)
	exitCode, hasExitCode := devkitExitCode(collectionErr)
	timedOut := errors.Is(collectionErr, context.DeadlineExceeded)
	// 故障阶段、退出码、超时状态或摘要任一变化，都应立即形成新日志；
	// 完全相同的故障只在限频窗口到期后重记。
	fingerprint := stage + "\x00" + strconv.Itoa(exitCode) + "\x00" + strconv.FormatBool(timedOut) + "\x00" + summary
	shouldLog := !s.failed || fingerprint != s.failureFingerprint || now.Sub(s.failureLoggedAt) >= devkitFailureLogWindow
	s.failed = true
	s.failureFingerprint = fingerprint
	if !shouldLog || s.logger == nil {
		return
	}
	s.failureLoggedAt = now
	failureFields := append(finishFields,
		"failure_stage", stage,
		"timeout", timedOut,
		"error_summary", summary,
	)
	if hasExitCode {
		failureFields = append(failureFields, "exit_code", exitCode)
	}
	s.logger.Error("devkit_collection_failed", failureFields...)
}

func (s *devkitCollectionLogState) recordPanic(attempt devkitCollectionLogAttempt, panicValue any) {
	// panic 仍沿用失败日志合同，但随后重新 panic，让上层测试/进程生命周期
	// 保持原有故障语义，不把编程错误伪装成普通 CLI 失败。
	if s.logger == nil {
		return
	}
	fields := append([]any(nil), attempt.fields...)
	fields = append(fields, "failure_stage", "panic", "panic", panicValue)
	s.logger.Error("devkit_collection_failed", fields...)
}

func (s *devkitCollectionLogState) debugCLIOutput(roundID uint64, stream, output string) {
	// stdout/stderr 只用于受限 DEBUG 诊断，不进入 Prometheus cache 或 label。
	if s.logger == nil {
		return
	}
	bounded, truncated := truncateDevkitLogText(output, maxDevkitDebugOutputCharacters)
	s.logger.Debug("devkit_cli_output",
		"collector", s.collector,
		"round_id", roundID,
		"stream", stream,
		"output", bounded,
		"truncated", truncated,
	)
}

func (s *devkitCollectionLogState) debugParseSummary(roundID uint64, fields ...any) {
	if s.logger == nil {
		return
	}
	base := []any{"collector", s.collector, "round_id", roundID}
	s.logger.Debug("devkit_parse_summary", append(base, fields...)...)
}

func (s *devkitCollectionLogState) resetFailureLocked() {
	// 参数变化或成功恢复会清空失败状态，使下一次新故障立即可见。
	s.failed = false
	s.failureFingerprint = ""
	s.failureLoggedAt = time.Time{}
}

func devkitFailureStage(err error) string {
	// 允许经过多层 fmt wrapping 后仍提取最内层的业务阶段。
	var failure *devkitCollectionFailure
	if errors.As(err, &failure) {
		return failure.stage
	}
	return "collection"
}

func devkitExitCode(err error) (int, bool) {
	// 非进程退出错误（例如 Parser 或 context 错误）不伪造 exit code 字段。
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ProcessState == nil {
		return 0, false
	}
	return exitErr.ExitCode(), true
}

func normalizeDevkitLogText(raw string) string {
	return strings.Join(strings.Fields(raw), " ")
}

func truncateDevkitLogText(raw string, limit int) (string, bool) {
	// 按 rune 截断，避免在多字节 UTF-8 字符中间切断并生成无效日志文本。
	runes := []rune(raw)
	if limit < 0 || len(runes) <= limit {
		return raw, false
	}
	if limit <= 3 {
		return string(runes[:limit]), true
	}
	return string(runes[:limit-3]) + "...", true
}
