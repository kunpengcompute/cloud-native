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
)

// memory 报告各 section 的锚点：每个 section 由“进入标志”到“下一 section 进入标志”界定。
const (
	memAnchorReport       = "Memory Summary Report"
	memAnchorCacheMiss    = "Percentage of core Cache miss"
	memAnchorDDRSystem    = "DDR Bandwidth (system wide)"
	memAnchorCacheMetrics = "Memory metrics of the Cache"
	memAnchorAccess       = "L1/L2/TLB Access Bandwidth and Hit Rate"
	memAnchorL3           = "L3 Read Bandwidth and Hit Rate"
	memAnchorDDRCMetrics  = "Memory metrics of the DDRC"
	memAnchorDDRCTable    = "DDRC_ACCESS_BANDWIDTH"
)

// memSumTolerance 是交叉校验的带宽求和容差(MB/s)：6 个保留两位小数的带宽累计
// 舍入误差可达约 0.06MB/s，取 0.5MB/s 留出安全裕度，避免真实数据被误判失败。
const memSumTolerance = 0.5

var (
	// memAccessCell 复合单元格：带宽|命中率；竖线两侧可能带空格（如 "N/A| 82.00%"）。
	memAccessCell = regexp.MustCompile(`(N/A|[\d.]+MB/s)\s*\|\s*(N/A|[\d.]+%)`)
	// memDDRCCell 复合单元格：DDR read|DDR write。
	memDDRCCell        = regexp.MustCompile(`([\d.]+)MB/s\s*\|\s*([\d.]+)MB/s`)
	memCacheMissRow    = regexp.MustCompile(`(?m)^\s*(L1D|L1I|L2D|L2I)\s+([\d.]+)%\s*$`)
	memDDRSystemRow    = regexp.MustCompile(`(?m)^\s*(ddrc_read|ddrc_write)\s+([\d.]+)MB/s\s*$`)
	memL3Row           = regexp.MustCompile(`(?m)^\s*(\d+)\s+(--|\d+)\s+([\d.]+)MB/s\s+([\d.]+)MB/s\s+([\d.]+)%\s*$`)
	memAccessCPUPrefix = regexp.MustCompile(`^\s*(\S+)\s+(.*)$`)
	memDDRCNodePrefix  = regexp.MustCompile(`^\s*(\d+)\s+(.*)$`)
)

// memoryAccessCell 是 L1/L2/TLB Access 表的一格：带宽与命中率相互独立，可各自缺失。
type memoryAccessCell struct {
	cpu        string
	component  string
	bandwidth  float64
	hasBW      bool
	hitPercent float64
	hasHit     bool
}

// memoryL3Row 是 L3 表的一行：NODE + CCL 双维度。
type memoryL3Row struct {
	node             string
	ccl              string
	readHitBandwidth float64
	readBandwidth    float64
	readHitPercent   float64
}

// memoryDDRCCell 是 DDRC 表的一格：同一 (node, ddrc) 下 read/write 两个带宽。
type memoryDDRCCell struct {
	node  string
	ddrc  string
	read  float64
	write float64
}

// memoryCacheMiss 是 Cache Miss 段的一项。
type memoryCacheMiss struct {
	component string
	percent   float64
}

// memoryDDRSystem 是 DDR system-wide 段的一项。
type memoryDDRSystem struct {
	operation string
	value     float64
}

// memoryMetricCache 承载一份完整 Memory 报告解析后的可导出数据。
type memoryMetricCache struct {
	devkitAttemptMetadata
	cacheMiss       []memoryCacheMiss
	ddrSystem       []memoryDDRSystem
	access          []memoryAccessCell
	l3              []memoryL3Row
	ddrc            []memoryDDRCCell
	success         bool
	lastSuccessTime float64
}

// parseMemoryOutput 解析 stdin 中最后一份完整 Memory 报告（metric=1 ALL 契约）。
func parseMemoryOutput(output string, attempt devkitAttemptMetadata, logger *slog.Logger) (*memoryMetricCache, error) {
	reportText, reportCount, err := selectLastMemoryReport(output)
	if err != nil {
		return nil, err
	}
	if reportCount > 1 && logger != nil {
		logger.Warn("multiple_memory_reports", "report_count", reportCount, "selected", "last")
	}

	cacheMiss, err := parseMemoryCacheMiss(reportText)
	if err != nil {
		return nil, err
	}
	ddrSystem, err := parseMemoryDDRSystem(reportText)
	if err != nil {
		return nil, err
	}
	access, err := parseMemoryAccess(reportText)
	if err != nil {
		return nil, err
	}
	l3, err := parseMemoryL3(reportText)
	if err != nil {
		return nil, err
	}
	ddrc, err := parseMemoryDDRC(reportText)
	if err != nil {
		return nil, err
	}

	return &memoryMetricCache{
		devkitAttemptMetadata: attempt,
		cacheMiss:             cacheMiss,
		ddrSystem:             ddrSystem,
		access:                access,
		l3:                    l3,
		ddrc:                  ddrc,
		success:               true,
	}, nil
}

// selectLastMemoryReport 多报告输入只选择最后一份，校验对应元信息并返回报告数量。
func selectLastMemoryReport(text string) (string, int, error) {
	reportCount := strings.Count(text, memAnchorReport)
	if reportCount == 0 {
		return "", 0, fmt.Errorf("missing Memory Summary Report")
	}
	idx := strings.LastIndex(text, memAnchorReport)
	metadataStart := 0
	if previous := strings.LastIndex(text[:idx], memAnchorReport); previous != -1 {
		metadataStart = previous
	}
	if _, err := extractDevkitCommand(text[metadataStart:idx]); err != nil {
		return "", reportCount, err
	}
	// 回退到该行行首，保证后续 section 锚点定位不受行内偏移影响。
	lineStart := strings.LastIndex(text[:idx], "\n")
	if lineStart == -1 {
		return text, reportCount, nil
	}
	return text[lineStart+1:], reportCount, nil
}

// memorySection 截取 [startAnchor, endAnchor) 之间的文本；endAnchor 为空时到 EOF。
func memorySection(reportText, startAnchor, endAnchor string) (string, error) {
	start := strings.Index(reportText, startAnchor)
	if start == -1 {
		return "", fmt.Errorf("missing section: %s", startAnchor)
	}
	if endAnchor == "" {
		return reportText[start:], nil
	}
	end := strings.Index(reportText[start+len(startAnchor):], endAnchor)
	if end == -1 {
		return "", fmt.Errorf("section %s is missing end marker %s", startAnchor, endAnchor)
	}
	return reportText[start : start+len(startAnchor)+end], nil
}

// parseMemoryCacheMiss 解析 Cache Miss 段的 L1D/L1I/L2D/L2I 未命中百分比。
func parseMemoryCacheMiss(reportText string) ([]memoryCacheMiss, error) {
	segment, err := memorySection(reportText, memAnchorCacheMiss, memAnchorDDRSystem)
	if err != nil {
		return nil, err
	}
	var result []memoryCacheMiss
	seen := make(map[string]struct{})
	for _, match := range memCacheMissRow.FindAllStringSubmatch(segment, -1) {
		component := strings.ToLower(match[1])
		if _, dup := seen[component]; dup {
			return nil, fmt.Errorf("duplicate Cache Miss component: %s", component)
		}
		seen[component] = struct{}{}
		percent, err := parsePercent(match[2])
		if err != nil {
			return nil, err
		}
		result = append(result, memoryCacheMiss{component: component, percent: percent})
	}
	for _, component := range []string{"l1d", "l1i", "l2d", "l2i"} {
		if _, ok := seen[component]; !ok {
			return nil, fmt.Errorf("required Cache Miss component is missing: %s", component)
		}
	}
	return result, nil
}

// parseMemoryDDRSystem 解析 DDR Bandwidth (system wide) 段。
// 行首 token 字面即 ddrc_write / ddrc_read（方案甲：直接作为 operation 取值）。
func parseMemoryDDRSystem(reportText string) ([]memoryDDRSystem, error) {
	segment, err := memorySection(reportText, memAnchorDDRSystem, memAnchorCacheMetrics)
	if err != nil {
		return nil, err
	}
	var result []memoryDDRSystem
	seen := make(map[string]struct{})
	for _, match := range memDDRSystemRow.FindAllStringSubmatch(segment, -1) {
		operation := match[1]
		if _, dup := seen[operation]; dup {
			return nil, fmt.Errorf("duplicate DDR system operation: %s", operation)
		}
		seen[operation] = struct{}{}
		value, err := strconv.ParseFloat(match[2], 64)
		if err != nil {
			return nil, fmt.Errorf("invalid DDR system bandwidth: %s", match[2])
		}
		result = append(result, memoryDDRSystem{operation: operation, value: value})
	}
	for _, operation := range []string{"ddrc_read", "ddrc_write"} {
		if _, ok := seen[operation]; !ok {
			return nil, fmt.Errorf("DDR system is missing required operation: %s", operation)
		}
	}
	return result, nil
}

// parseMemoryAccess 解析 L1/L2/TLB Access 表：动态 component 列 + 复合单元格 X|Y。
func parseMemoryAccess(reportText string) ([]memoryAccessCell, error) {
	segment, err := memorySection(reportText, memAnchorAccess, memAnchorL3)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(segment, "\n")
	headerIndex, components, err := memoryAccessHeader(lines)
	if err != nil {
		return nil, err
	}

	var cells []memoryAccessCell
	seen := make(map[string]struct{})
	for _, rawLine := range lines[headerIndex+1:] {
		if strings.TrimSpace(rawLine) == "" || isSeparatorLine(rawLine) {
			continue
		}
		// Access 数据行必含 "|"；借此跳过截取时混入的下一 section 标题前缀。
		if !strings.Contains(rawLine, "|") {
			continue
		}
		prefix := memAccessCPUPrefix.FindStringSubmatch(rawLine)
		if prefix == nil {
			continue
		}
		cpu := prefix[1]
		matches := memAccessCell.FindAllStringSubmatch(prefix[2], -1)
		if len(matches) != len(components) {
			return nil, fmt.Errorf("invalid Access row: got %d cells, expected %d: %q", len(matches), len(components), rawLine)
		}
		for i, component := range components {
			key := cpu + "|" + component
			if _, dup := seen[key]; dup {
				return nil, fmt.Errorf("duplicate Access cell: %s", key)
			}
			seen[key] = struct{}{}
			cell := memoryAccessCell{cpu: cpu, component: component}
			if matches[i][1] != "N/A" {
				bw, err := parseMBps(matches[i][1])
				if err != nil {
					return nil, err
				}
				cell.bandwidth = bw
				cell.hasBW = true
			}
			if matches[i][2] != "N/A" {
				hit, err := parsePercent(matches[i][2])
				if err != nil {
					return nil, err
				}
				cell.hitPercent = hit
				cell.hasHit = true
			}
			cells = append(cells, cell)
		}
	}
	if len(cells) == 0 {
		return nil, fmt.Errorf("no valid data rows in Access table")
	}
	return cells, nil
}

// memoryAccessHeader 定位 Access 表头行，返回其索引与归一后的 component 列名。
func memoryAccessHeader(lines []string) (int, []string, error) {
	for index, line := range lines {
		stripped := strings.TrimSpace(line)
		if strings.HasPrefix(stripped, "CPU") {
			columns := strings.Fields(stripped)
			if len(columns) < 2 {
				return 0, nil, fmt.Errorf("component columns are missing from Access header")
			}
			components := make([]string, 0, len(columns)-1)
			for _, col := range columns[1:] {
				components = append(components, strings.ToLower(col))
			}
			return index, components, nil
		}
	}
	return 0, nil, fmt.Errorf("missing CPU header in Access table")
}

// parseMemoryL3 解析 L3 表：NODE + CCL（-- 规范为 all）。
func parseMemoryL3(reportText string) ([]memoryL3Row, error) {
	segment, err := memorySection(reportText, memAnchorL3, memAnchorDDRCMetrics)
	if err != nil {
		return nil, err
	}
	var rows []memoryL3Row
	seen := make(map[string]struct{})
	for _, match := range memL3Row.FindAllStringSubmatch(segment, -1) {
		node := match[1]
		ccl := match[2]
		if ccl == "--" {
			ccl = "all"
		}
		key := node + "|" + ccl
		if _, dup := seen[key]; dup {
			return nil, fmt.Errorf("duplicate L3 row: %s", key)
		}
		seen[key] = struct{}{}
		readHitBW, err := strconv.ParseFloat(match[3], 64)
		if err != nil {
			return nil, fmt.Errorf("invalid L3 read_hit_bandwidth: %s", match[3])
		}
		readBW, err := strconv.ParseFloat(match[4], 64)
		if err != nil {
			return nil, fmt.Errorf("invalid L3 read_bandwidth: %s", match[4])
		}
		hit, err := parsePercent(match[5])
		if err != nil {
			return nil, err
		}
		rows = append(rows, memoryL3Row{
			node:             node,
			ccl:              ccl,
			readHitBandwidth: readHitBW,
			readBandwidth:    readBW,
			readHitPercent:   hit,
		})
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("L3 table has no valid data rows")
	}
	return rows, nil
}

// parseMemoryDDRC 解析 DDRC 表：动态 DDRC 列 + Total（规范为 total），每格 read|write。
func parseMemoryDDRC(reportText string) ([]memoryDDRCCell, error) {
	segment, err := memorySection(reportText, memAnchorDDRCTable, "")
	if err != nil {
		return nil, err
	}
	lines := strings.Split(segment, "\n")
	headerIndex, ddrcLabels, err := memoryDDRCHeader(lines)
	if err != nil {
		return nil, err
	}

	var cells []memoryDDRCCell
	seen := make(map[string]struct{})
	for _, rawLine := range lines[headerIndex+1:] {
		if strings.TrimSpace(rawLine) == "" || isSeparatorLine(rawLine) {
			continue
		}
		if !strings.Contains(rawLine, "|") {
			continue
		}
		prefix := memDDRCNodePrefix.FindStringSubmatch(rawLine)
		if prefix == nil {
			continue
		}
		node := prefix[1]
		matches := memDDRCCell.FindAllStringSubmatch(prefix[2], -1)
		if len(matches) != len(ddrcLabels) {
			return nil, fmt.Errorf("DDRC row has %d cells, expected %d: %q", len(matches), len(ddrcLabels), rawLine)
		}
		for i, ddrc := range ddrcLabels {
			key := node + "|" + ddrc
			if _, dup := seen[key]; dup {
				return nil, fmt.Errorf("duplicate DDRC cell: %s", key)
			}
			seen[key] = struct{}{}
			read, err := strconv.ParseFloat(matches[i][1], 64)
			if err != nil {
				return nil, fmt.Errorf("invalid DDRC read value: %s", matches[i][1])
			}
			write, err := strconv.ParseFloat(matches[i][2], 64)
			if err != nil {
				return nil, fmt.Errorf("invalid DDRC write value: %s", matches[i][2])
			}
			cells = append(cells, memoryDDRCCell{node: node, ddrc: ddrc, read: read, write: write})
		}
	}
	if len(cells) == 0 {
		return nil, fmt.Errorf("DDRC table has no valid data rows")
	}
	return cells, nil
}

// memoryDDRCHeader 定位 DDRC 表头行，返回其索引与归一后的 ddrc 列名（Total→total）。
func memoryDDRCHeader(lines []string) (int, []string, error) {
	for index, line := range lines {
		stripped := strings.TrimSpace(line)
		if strings.HasPrefix(stripped, "NODE") && strings.Contains(stripped, "DDRC") {
			columns := strings.Fields(stripped)
			if len(columns) < 2 {
				return 0, nil, fmt.Errorf("DDRC header is missing data columns")
			}
			labels := make([]string, 0, len(columns)-1)
			for _, col := range columns[1:] {
				if strings.EqualFold(col, "total") {
					labels = append(labels, "total")
					continue
				}
				// 形如 DDRC_0 → 取编号 0。
				parts := strings.Split(col, "_")
				labels = append(labels, parts[len(parts)-1])
			}
			return index, labels, nil
		}
	}
	return 0, nil, fmt.Errorf("DDRC table is missing NODE header")
}

// parseMBps 剥离 MB/s 后缀并转 float。
func parseMBps(token string) (float64, error) {
	if !strings.HasSuffix(token, "MB/s") {
		return 0, fmt.Errorf("bandwidth is missing MB/s suffix: %s", token)
	}
	return strconv.ParseFloat(strings.TrimSuffix(token, "MB/s"), 64)
}

// parsePercent 剥离可选的 % 后缀并转 float，校验落在 0~100。
func parsePercent(token string) (float64, error) {
	trimmed := strings.TrimSuffix(token, "%")
	value, err := strconv.ParseFloat(trimmed, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid percentage: %s", token)
	}
	if value < 0 || value > 100 {
		return 0, fmt.Errorf("percentage is out of range: %s", token)
	}
	return value, nil
}

// crossCheckMemory 做带宽求和一致性校验；不一致只记 Warning，不阻断指标发布
// （见设计方案 7.6.4）。
func crossCheckMemory(cache *memoryMetricCache, logger *slog.Logger) {
	// L3：同 NODE 的 ccl=all 两个带宽字段应等于各 CCL 明细之和；hit_percent 不参与。
	nodes := make(map[string]struct{})
	for _, row := range cache.l3 {
		nodes[row.node] = struct{}{}
	}
	for node := range nodes {
		var sumHitBW, sumBW, allHitBW, allBW float64
		var hasAll, hasDetail bool
		for _, row := range cache.l3 {
			if row.node != node {
				continue
			}
			if row.ccl == "all" {
				allHitBW, allBW, hasAll = row.readHitBandwidth, row.readBandwidth, true
			} else {
				sumHitBW += row.readHitBandwidth
				sumBW += row.readBandwidth
				hasDetail = true
			}
		}
		if !hasAll || !hasDetail {
			continue
		}
		if absFloat(sumBW-allBW) > memSumTolerance {
			warnMemoryAggregateMismatch(logger, "L3", node, "read_bandwidth", allBW, sumBW)
		}
		if absFloat(sumHitBW-allHitBW) > memSumTolerance {
			warnMemoryAggregateMismatch(logger, "L3", node, "read_hit_bandwidth", allHitBW, sumHitBW)
		}
	}

	// DDRC：同 NODE 的 ddrc=total 应等于各控制器之和。
	ddrcNodes := make(map[string]struct{})
	for _, cell := range cache.ddrc {
		ddrcNodes[cell.node] = struct{}{}
	}
	for node := range ddrcNodes {
		var sumRead, sumWrite, totalRead, totalWrite float64
		var hasTotal, hasDetail bool
		for _, cell := range cache.ddrc {
			if cell.node != node {
				continue
			}
			if cell.ddrc == "total" {
				totalRead, totalWrite, hasTotal = cell.read, cell.write, true
			} else {
				sumRead += cell.read
				sumWrite += cell.write
				hasDetail = true
			}
		}
		if !hasTotal || !hasDetail {
			continue
		}
		if absFloat(sumRead-totalRead) > memSumTolerance {
			warnMemoryAggregateMismatch(logger, "DDRC", node, "read", totalRead, sumRead)
		}
		if absFloat(sumWrite-totalWrite) > memSumTolerance {
			warnMemoryAggregateMismatch(logger, "DDRC", node, "write", totalWrite, sumWrite)
		}
	}
}

func warnMemoryAggregateMismatch(logger *slog.Logger, table, node, metric string, total, sum float64) {
	if logger == nil {
		return
	}
	logger.Warn("memory_aggregate_mismatch",
		"table", table,
		"node", node,
		"metric", metric,
		"total", total,
		"sum", sum,
		"difference", absFloat(sum-total),
		"tolerance", memSumTolerance,
	)
}

func absFloat(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
