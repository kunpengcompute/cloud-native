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
	"fmt"
	"log/slog"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

// topdownNode 是一条恢复层级后的 TopDown 树节点。
type topdownNode struct {
	name           string
	path           string
	level          int
	preferredEvent string
	value          float64
}

// topdownMetricCache 承载一份完整 TopDown 报告解析后的可导出数据。
type topdownMetricCache struct {
	devkitAttemptMetadata
	cycles          uint64
	instructions    uint64
	ipc             float64
	nodes           []topdownNode
	pmuEvents       []topdownPMUEvent
	success         bool
	lastSuccessTime float64
}

// topdownScope 是报告声明的原始采集范围。target 保留无类型前缀的
// CPU/PID 值，仅用于与 argv 生成的 attempt metadata 交叉校验。
type topdownScope struct {
	targetType string
	target     string
}

// topdownPMUEvent 是 PMU Event 表的一行。
type topdownPMUEvent struct {
	event string
	count uint64
}

// topdownTopLevelNames 是 TopDown 顶层四大类，用于完整性校验。
var topdownTopLevelNames = map[string]struct{}{
	"bad_speculation": {},
	"frontend_bound":  {},
	"retiring":        {},
	"backend_bound":   {},
}

var (
	// topdownSummaryPattern 匹配 Summary 字段，如 "Cycles 14437823235"。
	topdownCommandPattern     = regexp.MustCompile(`(?m)^Command\s*:\s*(.+?)\s*$`)
	topdownScopeLinePattern   = regexp.MustCompile(`(?m)^Top-down metrics of .+:\s*$`)
	topdownSystemScopePattern = regexp.MustCompile(`^Top-down metrics of the system:\s*$`)
	topdownCPUScopePattern    = regexp.MustCompile(`^Top-down metrics of CPU\(s\)\s+(.+):\s*$`)
	topdownPIDScopePattern    = regexp.MustCompile(`^Top-down metrics of process id '([^']+)':\s*$`)
	topdownTreeHeader         = regexp.MustCompile(`(?m)^.*Top-down Metrics.*Bound\(%\).*$`)
	topdownPMUHeader          = regexp.MustCompile(`(?m)^.*PMU Event.*Count.*$`)
	topdownElapsed            = regexp.MustCompile(`(?m)^(\d+(?:\.\d+)?)\s+milliseconds time elapsed\s*$`)
	// topdownRowPattern 匹配一条树行：名字前缀 + 百分比 + Preferred Event(-- 或事件名)。
	topdownRowPattern      = regexp.MustCompile(`^(.+?)\s+(\d+(?:\.\d+)?)\s+(--|[A-Za-z0-9_]+)\s*$`)
	topdownTreeMarker      = regexp.MustCompile("[├└]──")
	topdownPMURowPattern   = regexp.MustCompile(`^\s*(r[0-9A-Fa-f]+)\s+([\d,]+)\s*$`)
	topdownNonAlphaNumeric = regexp.MustCompile(`[^a-z0-9]+`)
)

// normalizeTopdownName 将 CLI 节点名稳定转换为 Prometheus label 使用的 snake_case。
func normalizeTopdownName(raw string) string {
	lowered := strings.ToLower(strings.TrimSpace(raw))
	normalized := topdownNonAlphaNumeric.ReplaceAllString(lowered, "_")
	return strings.Trim(normalized, "_")
}

// parseTopdownOutput 解析 stdin 中最后一份完整 TopDown 报告。
func parseTopdownOutput(output string, attempt devkitAttemptMetadata, logger *slog.Logger) (*topdownMetricCache, error) {
	reportText, reportCount, err := selectLastTopdownReport(output)
	if err != nil {
		return nil, err
	}
	if reportCount > 1 && logger != nil {
		logger.Warn("multiple_topdown_reports", "report_count", reportCount, "selected", "last")
	}
	reported, err := parseTopdownScope(reportText)
	if err != nil {
		return nil, err
	}
	requested := topdownScope{targetType: attempt.targetType, target: attempt.targetValue}
	if requested.targetType != "" {
		// CLI 可能把降序、重复或重叠 CPU raw 规范化后写入报告；这里只比较
		// 临时 canonical 集合。指标 label 仍使用 attempt 中的 raw target。
		scopeMatches, compareErr := devkitTopdownScopesEquivalent(requested, reported)
		if compareErr != nil {
			return nil, fmt.Errorf("compare TopDown scope with current argv: %w", compareErr)
		}
		if !scopeMatches {
			return nil, fmt.Errorf(
				"TopDown scope does not match current argv: requested_type=%s requested_target=%s reported_type=%s reported_target=%s",
				requested.targetType, requested.target, reported.targetType, reported.target,
			)
		}
	}
	cycles, instructions, ipc, err := parseTopdownSummary(reportText)
	if err != nil {
		return nil, err
	}
	nodes, err := parseTopdownTree(reportText)
	if err != nil {
		return nil, err
	}
	pmuEvents, err := parseTopdownPMU(reportText)
	if err != nil {
		return nil, err
	}
	return &topdownMetricCache{
		devkitAttemptMetadata: attempt,
		cycles:                cycles,
		instructions:          instructions,
		ipc:                   ipc,
		nodes:                 nodes,
		pmuEvents:             pmuEvents,
		success:               true,
	}, nil
}

// extractDevkitCommand 读取候选报告元信息中的实际 CLI 命令。
func extractDevkitCommand(text string) (string, error) {
	matches := topdownCommandPattern.FindAllStringSubmatch(text, -1)
	if len(matches) == 0 {
		return "", fmt.Errorf("missing Command metadata")
	}
	return matches[len(matches)-1][1], nil
}

// selectLastTopdownReport 多报告输入只选择最后一份，并校验其业务 anchor 前的元信息。
func selectLastTopdownReport(text string) (string, int, error) {
	locs := topdownScopeLinePattern.FindAllStringIndex(text, -1)
	if len(locs) == 0 {
		return "", 0, fmt.Errorf("missing Top-down metrics of ... scope marker")
	}
	selected := len(locs) - 1
	metadataStart := 0
	if selected > 0 {
		metadataStart = locs[selected-1][0]
	}
	if _, err := extractDevkitCommand(text[metadataStart:locs[selected][0]]); err != nil {
		return "", len(locs), err
	}
	return text[locs[selected][0]:], len(locs), nil
}

func parseTopdownScope(reportText string) (topdownScope, error) {
	// scope 来自所选报告的业务标题，仅用于与 argv attempt 做集合等价比较；
	// 指标 target 仍由 attempt metadata 提供，避免报告 Command 覆盖 raw target。
	line := topdownScopeLinePattern.FindString(reportText)
	if line == "" {
		return topdownScope{}, fmt.Errorf("missing Top-down metrics of ... scope marker")
	}
	line = strings.TrimSpace(line)
	if topdownSystemScopePattern.MatchString(line) {
		return topdownScope{targetType: "system"}, nil
	}
	if match := topdownCPUScopePattern.FindStringSubmatch(line); match != nil {
		return topdownScope{targetType: "cpu", target: strings.TrimSpace(match[1])}, nil
	}
	if match := topdownPIDScopePattern.FindStringSubmatch(line); match != nil {
		return topdownScope{targetType: "pid", target: strings.TrimSpace(match[1])}, nil
	}
	return topdownScope{}, fmt.Errorf("unrecognized TopDown scope marker: %s", line)
}

// parseTopdownSummary 解析报告根层的 Cycles、Instructions 和 IPC。
func parseTopdownSummary(reportText string) (uint64, uint64, float64, error) {
	cycles, err := requiredUintField(reportText, "Cycles")
	if err != nil {
		return 0, 0, 0, err
	}
	instructions, err := requiredUintField(reportText, "Instructions")
	if err != nil {
		return 0, 0, 0, err
	}
	ipc, err := requiredFloatField(reportText, "IPC")
	if err != nil {
		return 0, 0, 0, err
	}
	return cycles, instructions, ipc, nil
}

// requiredUintField 解析允许千位逗号的无符号计数，并保留 ParseUint 的溢出错误。
func requiredUintField(text, field string) (uint64, error) {
	pattern := regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(field) + `\s+([\d,]+(?:\.\d+)?)\s*$`)
	match := pattern.FindStringSubmatch(text)
	if match == nil {
		return 0, fmt.Errorf("missing Summary field %s", field)
	}
	raw := strings.ReplaceAll(match[1], ",", "")
	value, err := strconv.ParseUint(raw, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%s has invalid value: %s", field, raw)
	}
	return value, nil
}

// requiredFloatField 解析 Summary 中的十进制浮点字段；字段缺失与非法值均使整轮失败。
func requiredFloatField(text, field string) (float64, error) {
	pattern := regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(field) + `\s+([\d,]+(?:\.\d+)?)\s*$`)
	match := pattern.FindStringSubmatch(text)
	if match == nil {
		return 0, fmt.Errorf("missing Summary field %s", field)
	}
	raw := strings.ReplaceAll(match[1], ",", "")
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0, fmt.Errorf("%s has invalid value: %s", field, raw)
	}
	if value < 0 {
		return 0, fmt.Errorf("%s must not be negative", field)
	}
	return value, nil
}

// parseTopdownTree 按树符号和缩进恢复 TopDown 节点的 path/level。
func parseTopdownTree(reportText string) ([]topdownNode, error) {
	headerLoc := topdownTreeHeader.FindStringIndex(reportText)
	pmuLoc := topdownPMUHeader.FindStringIndex(reportText)
	if headerLoc == nil || pmuLoc == nil || pmuLoc[0] <= headerLoc[1] {
		return nil, fmt.Errorf("TopDown table or PMU table boundary is missing")
	}

	var (
		stack []string
		nodes []topdownNode
		paths = make(map[string]struct{})
	)
	segment := reportText[headerLoc[1]:pmuLoc[0]]
	for _, rawLine := range strings.Split(segment, "\n") {
		if strings.TrimSpace(rawLine) == "" || isSeparatorLine(rawLine) {
			continue
		}
		match := topdownRowPattern.FindStringSubmatch(rawLine)
		if match == nil {
			continue
		}

		rawName, level := treeNameAndLevel(match[1])
		name := normalizeTopdownName(rawName)
		if name == "" {
			return nil, fmt.Errorf("TopDown node name is empty")
		}
		if strings.Contains(name, ".") {
			// path 以 "." 连接各段，分隔符绝不能落入段内，否则 path 编码与查询期
			// 父子聚合正则都会静默错位。normalizeTopdownName 已把 "." 折叠为 "_"，
			// 此处显式断言把该隐式不变量锁死，防止归一规则日后被放宽时静默破坏。
			return nil, fmt.Errorf("normalized node name still contains path separator '.': %s", name)
		}
		if level < 1 {
			return nil, fmt.Errorf("TopDown node has invalid level: %s", rawLine)
		}
		if len(stack) < level-1 {
			return nil, fmt.Errorf("TopDown tree contains a level jump: %s", rawLine)
		}

		stack = stack[:level-1]
		path := strings.Join(append(append([]string{}, stack...), name), ".")
		if _, dup := paths[path]; dup {
			return nil, fmt.Errorf("duplicate TopDown node path: %s", path)
		}
		paths[path] = struct{}{}

		preferred := match[3]
		if preferred == "--" {
			preferred = ""
		}
		value, err := strconv.ParseFloat(match[2], 64)
		if err != nil {
			return nil, fmt.Errorf("invalid TopDown percentage: %s", match[2])
		}
		nodes = append(nodes, topdownNode{
			name:           name,
			path:           path,
			level:          level,
			preferredEvent: preferred,
			value:          value,
		})
		stack = append(stack, name)
	}

	if len(nodes) == 0 {
		return nil, fmt.Errorf("TopDown table has no valid nodes")
	}
	if err := validateTopdownTree(nodes); err != nil {
		return nil, err
	}
	return nodes, nil
}

// treeNameAndLevel 从节点名前缀计算层级；CLI 的每个树层级占 4 个字符。
func treeNameAndLevel(namePart string) (string, int) {
	markers := topdownTreeMarker.FindAllStringIndex(namePart, -1)
	if len(markers) == 0 {
		return strings.TrimSpace(namePart), 1
	}
	last := markers[len(markers)-1]
	// 顶层固定有两个前导空格，第一层分支标记位于索引 2。
	// regexp 返回 UTF-8 字节偏移；层级宽度是 4 个 Unicode 字符，必须先把
	// marker 前缀换算为 rune 数，否则多字节的树线字符会把 level 算大。
	level := utf8.RuneCountInString(namePart[:last[0]])/4 + 2
	// last[1] 是最后一个树符号的字节结束位置（"──" 为多字节，用字节切片安全）。
	return strings.TrimSpace(namePart[last[1]:]), level
}

// isSeparatorLine 识别 CLI 使用的 Unicode/ASCII 表格分隔线。
func isSeparatorLine(line string) bool {
	stripped := strings.TrimSpace(line)
	if stripped == "" {
		return false
	}
	return strings.Trim(stripped, "─=-") == ""
}

// validateTopdownTree 执行正式合同要求的 TopDown 树完整性校验。
func validateTopdownTree(nodes []topdownNode) error {
	topNames := make(map[string]struct{})
	var topTotal float64
	for _, node := range nodes {
		if node.level == 1 {
			topNames[node.name] = struct{}{}
			topTotal += node.value
		}
	}
	if len(topNames) != len(topdownTopLevelNames) {
		return fmt.Errorf("TopDown top-level nodes are incomplete: %v", keysOf(topNames))
	}
	for name := range topdownTopLevelNames {
		if _, ok := topNames[name]; !ok {
			return fmt.Errorf("TopDown is missing top-level node: %s", name)
		}
	}
	if topTotal < 99.5 || topTotal > 100.5 {
		return fmt.Errorf("TopDown top-level percentage sum is out of range: %.2f", topTotal)
	}

	paths := make(map[string]struct{}, len(nodes))
	for _, node := range nodes {
		paths[node.path] = struct{}{}
	}
	for _, required := range []string{"backend_bound.core_bound", "backend_bound.memory_bound"} {
		if _, ok := paths[required]; !ok {
			return fmt.Errorf("incomplete Backend Bound subtree; missing: %s", required)
		}
	}
	return nil
}

// keysOf 让错误信息中的缺失节点顺序保持稳定，便于日志比对和回归测试。
func keysOf(m map[string]struct{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// parseTopdownPMU 解析 PMU Event 表，事件码统一为小写。
func parseTopdownPMU(reportText string) ([]topdownPMUEvent, error) {
	headerLoc := topdownPMUHeader.FindStringIndex(reportText)
	elapsedLoc := topdownElapsed.FindStringIndex(reportText)
	if headerLoc == nil || elapsedLoc == nil || elapsedLoc[0] <= headerLoc[1] {
		return nil, fmt.Errorf("PMU table or elapsed time is missing")
	}

	var events []topdownPMUEvent
	seen := make(map[string]struct{})
	segment := reportText[headerLoc[1]:elapsedLoc[0]]
	for _, rawLine := range strings.Split(segment, "\n") {
		match := topdownPMURowPattern.FindStringSubmatch(rawLine)
		if match == nil {
			continue
		}
		event := strings.ToLower(match[1])
		if _, dup := seen[event]; dup {
			return nil, fmt.Errorf("duplicate PMU event: %s", event)
		}
		seen[event] = struct{}{}
		count, err := strconv.ParseUint(strings.ReplaceAll(match[2], ",", ""), 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid PMU count: %s", match[2])
		}
		events = append(events, topdownPMUEvent{event: event, count: count})
	}
	if len(events) == 0 {
		return nil, fmt.Errorf("PMU table has no valid events")
	}
	return events, nil
}
