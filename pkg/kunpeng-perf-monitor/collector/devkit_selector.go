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
	"errors"
	"fmt"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

var (
	// CPU 与 PID 的 raw 文本上限彼此独立，避免后续调整其中一种输入时，
	// 意外放宽另一种输入可占用的资源。变量保留为包级，便于边界测试注入。
	maxDevkitCPUSelectorCharacters        = 1024
	maxDevkitPIDSelectorCharacters        = 512
	maxDevkitCPUSelectorElements          = 512
	maxDevkitPIDSelectorElements          = 32
	fallbackMaxDevkitCPU           uint64 = 1024
	fallbackMaxDevkitPID           uint64 = 4194304

	devkitCPUPossiblePath = "/sys/devices/system/cpu/possible"
	devkitPIDMaxPath      = "/proc/sys/kernel/pid_max"
)

type devkitCPUInterval struct {
	start uint64
	end   uint64
}

func canonicalizeDevkitCPUSelector(raw string, maxCPU uint64) (string, error) {
	// canonical 结果只用于 scope 集合等价比较；ConfigMap、CLI argv、日志和
	// Prometheus target label 始终保留调用方提供的 raw 原文。
	intervals, err := parseDevkitCPUIntervals(raw, maxCPU, true)
	if err != nil {
		return "", err
	}
	parts := make([]string, 0, len(intervals))
	for _, interval := range intervals {
		if interval.start == interval.end {
			parts = append(parts, strconv.FormatUint(interval.start, 10))
			continue
		}
		parts = append(parts, fmt.Sprintf("%d-%d", interval.start, interval.end))
	}
	return strings.Join(parts, ","), nil
}

// parseDevkitCPUIntervals 将 raw CPU 表达式解析为临时区间列表。
// enforceResourceLimits 仅对用户 ConfigMap 输入启用；读取宿主机
// /sys/devices/system/cpu/possible 时关闭它，避免把元数据误当成用户 payload。
func parseDevkitCPUIntervals(raw string, maxCPU uint64, enforceResourceLimits bool) ([]devkitCPUInterval, error) {
	if enforceResourceLimits {
		if err := validateDevkitSelectorSize(raw, maxDevkitCPUSelectorCharacters, maxDevkitCPUSelectorElements); err != nil {
			return nil, err
		}
	}
	elements := strings.Split(raw, ",")
	intervals := make([]devkitCPUInterval, 0, len(elements))
	for _, element := range elements {
		bounds := strings.Split(element, "-")
		if len(bounds) < 1 || len(bounds) > 2 {
			return nil, fmt.Errorf("invalid_format: CPU element %q is not a number or range", element)
		}
		start, err := parseDevkitUnsigned(bounds[0])
		if err != nil {
			return nil, fmt.Errorf("CPU element %q: %w", element, err)
		}
		end := start
		if len(bounds) == 2 {
			end, err = parseDevkitUnsigned(bounds[1])
			if err != nil {
				return nil, fmt.Errorf("CPU element %q: %w", element, err)
			}
		}
		if start > maxCPU || end > maxCPU {
			return nil, fmt.Errorf("value_out_of_range: CPU element %q exceeds maximum %d", element, maxCPU)
		}
		if start > end {
			start, end = end, start
		}
		intervals = append(intervals, devkitCPUInterval{start: start, end: end})
	}

	// 仅在临时区间上排序、合并，不展开区间。这样既能比较降序、乱序、
	// 重复和重叠表达式，又不会因大范围 CPU 区间产生额外内存开销。
	sort.Slice(intervals, func(i, j int) bool {
		if intervals[i].start == intervals[j].start {
			return intervals[i].end < intervals[j].end
		}
		return intervals[i].start < intervals[j].start
	})
	merged := intervals[:0]
	for _, interval := range intervals {
		if len(merged) == 0 {
			merged = append(merged, interval)
			continue
		}
		last := &merged[len(merged)-1]
		adjacent := last.end < math.MaxUint64 && interval.start == last.end+1
		if interval.start <= last.end || adjacent {
			if interval.end > last.end {
				last.end = interval.end
			}
			continue
		}
		merged = append(merged, interval)
	}
	return merged, nil
}

func canonicalizeDevkitPIDSelector(raw string, maxPID uint64) (string, error) {
	// PID canonical 同样是临时值；raw 的顺序和重复写法仍传给 CLI 并进入
	// 可观测性字段，排序和去重不会回写用户配置。
	if err := validateDevkitSelectorSize(raw, maxDevkitPIDSelectorCharacters, maxDevkitPIDSelectorElements); err != nil {
		return "", err
	}
	elements := strings.Split(raw, ",")
	values := make([]uint64, 0, len(elements))
	for _, element := range elements {
		value, err := parseDevkitUnsigned(element)
		if err != nil {
			return "", fmt.Errorf("PID element %q: %w", element, err)
		}
		if value == 0 || value > maxPID {
			return "", fmt.Errorf("value_out_of_range: PID %d must be between 1 and %d", value, maxPID)
		}
		values = append(values, value)
	}
	sort.Slice(values, func(i, j int) bool { return values[i] < values[j] })
	parts := make([]string, 0, len(values))
	var previous uint64
	for index, value := range values {
		if index > 0 && value == previous {
			continue
		}
		parts = append(parts, strconv.FormatUint(value, 10))
		previous = value
	}
	return strings.Join(parts, ","), nil
}

func validateDevkitSelectorSize(raw string, maxCharacters, maxElements int) error {
	// ConfigMap 值属于用户输入，必须先按完整 raw 文本做资源门禁，再解析数值，
	// 保证超限内容不会进入 CLI。合同使用“字符”描述，因此这里按 rune 计数；
	// selector 合法语法仍只接受 ASCII 数字、逗号和连字符。
	if utf8.RuneCountInString(raw) > maxCharacters {
		return fmt.Errorf("length_limit: selector exceeds %d characters", maxCharacters)
	}
	if raw == "" {
		return fmt.Errorf("invalid_format: selector is empty")
	}
	if strings.Count(raw, ",")+1 > maxElements {
		return fmt.Errorf("element_limit: selector exceeds %d elements", maxElements)
	}
	return nil
}

// parseDevkitUnsigned 只接受 ASCII 十进制无符号整数，并把 strconv 的范围错误
// 转成稳定的 overflow reason，供 ConfigMap Warning 和边界测试使用。
func parseDevkitUnsigned(raw string) (uint64, error) {
	if raw == "" {
		return 0, fmt.Errorf("invalid_format: empty number")
	}
	for _, char := range raw {
		if char < '0' || char > '9' {
			return 0, fmt.Errorf("invalid_format: %q is not an unsigned decimal number", raw)
		}
	}
	value, err := strconv.ParseUint(raw, 10, 64)
	if err != nil {
		if errors.Is(err, strconv.ErrRange) {
			return 0, fmt.Errorf("overflow: %q exceeds uint64", raw)
		}
		return 0, fmt.Errorf("invalid_format: %q is not an unsigned decimal number", raw)
	}
	return value, nil
}

func devkitMaxCPU() uint64 {
	// possible 文件是宿主机元数据而非用户 selector，因此解析时不套用用户输入
	// 的字符数和元素数限制；文件缺失或格式异常时使用固定 fallback，保证校验可预测。
	raw, err := os.ReadFile(devkitCPUPossiblePath)
	if err != nil {
		return fallbackMaxDevkitCPU
	}
	intervals, err := parseDevkitCPUIntervals(strings.TrimSpace(string(raw)), math.MaxUint64, false)
	if err != nil || len(intervals) == 0 {
		return fallbackMaxDevkitCPU
	}
	return intervals[len(intervals)-1].end
}

// devkitMaxPID 读取内核 pid_max；只校验数值上限，不判断 PID 当前是否存在，
// 避免配置读取与进程生命周期之间产生竞态。
func devkitMaxPID() uint64 {
	raw, err := os.ReadFile(devkitPIDMaxPath)
	if err != nil {
		return fallbackMaxDevkitPID
	}
	value, err := parseDevkitUnsigned(strings.TrimSpace(string(raw)))
	if err != nil || value == 0 {
		return fallbackMaxDevkitPID
	}
	return value
}

// devkitTopdownScopesEquivalent 比较报告和 argv 的 scope；CPU/PID 使用临时
// canonical 集合，system 直接比较类型，任何未知 target type 都是内部错误。
func devkitTopdownScopesEquivalent(requested, reported topdownScope) (bool, error) {
	if requested.targetType != reported.targetType {
		return false, nil
	}
	switch requested.targetType {
	case "system":
		return true, nil
	case "cpu":
		requestedCanonical, err := canonicalizeDevkitCPUSelector(requested.target, math.MaxUint64)
		if err != nil {
			return false, fmt.Errorf("canonicalize requested CPU scope: %w", err)
		}
		reportedCanonical, err := canonicalizeDevkitCPUSelector(reported.target, math.MaxUint64)
		if err != nil {
			return false, fmt.Errorf("canonicalize reported CPU scope: %w", err)
		}
		return requestedCanonical == reportedCanonical, nil
	case "pid":
		requestedCanonical, err := canonicalizeDevkitPIDSelector(requested.target, math.MaxUint64)
		if err != nil {
			return false, fmt.Errorf("canonicalize requested PID scope: %w", err)
		}
		reportedCanonical, err := canonicalizeDevkitPIDSelector(reported.target, math.MaxUint64)
		if err != nil {
			return false, fmt.Errorf("canonicalize reported PID scope: %w", err)
		}
		return requestedCanonical == reportedCanonical, nil
	default:
		return false, fmt.Errorf("unsupported TopDown target type %q", requested.targetType)
	}
}
