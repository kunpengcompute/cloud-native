/*
 * Copyright (c) 2025 Huawei Technology corp.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

// Package util 实现常用的工具函数
package util

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

const (
	// ResctrlSchemataPath is resctrl schemata file path.
	ResctrlSchemataPath = "/sys/fs/resctrl/schemata"
)

// GetResctrlNUMAIDs gets MB domain IDs from resctrl schemata.
// Example line: MB:0=100;1=100
func GetResctrlNUMAIDs() ([]int, error) {
	return getIDsFromSchemata(ResctrlSchemataPath, "MB")
}

// GetResctrlCacheIDs gets L3 domain IDs from resctrl schemata.
// Example line: L3:0=ffff;1=ffff
func GetResctrlCacheIDs() ([]int, error) {
	return getIDsFromSchemata(ResctrlSchemataPath, "L3")
}

// GetResctrlSupportedSchemataKeys gets all supported schemata item keys from
// /sys/fs/resctrl/schemata, e.g. MBHDL/MBPRI/L3PRI/MBMIN/MB/L3.
func GetResctrlSupportedSchemataKeys() ([]string, error) {
	data, err := ReadFile(ResctrlSchemataPath)
	if err != nil {
		return nil, err
	}
	return parseSchemataKeys(string(data))
}

func getIDsFromSchemata(path, item string) ([]int, error) {
	data, err := ReadFile(path)
	if err != nil {
		return nil, err
	}
	return parseSchemataIDs(string(data), item)
}

func parseSchemataIDs(content, item string) ([]int, error) {
	lines := strings.Split(content, "\n")
	prefix := item + ":"

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if !strings.HasPrefix(line, prefix) {
			continue
		}

		assignments := strings.TrimPrefix(line, prefix)
		parts := strings.Split(assignments, ";")
		ids := make([]int, 0, len(parts))
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			kv := strings.SplitN(part, "=", 2)
			if len(kv) != 2 {
				return nil, fmt.Errorf("invalid schemata assignment %q in %s", part, line)
			}
			id, err := strconv.Atoi(strings.TrimSpace(kv[0]))
			if err != nil {
				return nil, fmt.Errorf("invalid id %q in %s: %w", kv[0], line, err)
			}
			ids = append(ids, id)
		}

		if len(ids) == 0 {
			return nil, fmt.Errorf("no ids found in %s line", item)
		}
		sort.Ints(ids)

		return ids, nil
	}

	return nil, fmt.Errorf("%s line not found in schemata", item)
}

func parseSchemataKeys(content string) ([]string, error) {
	lines := strings.Split(content, "\n")
	keys := make([]string, 0, len(lines))
	seen := make(map[string]struct{}, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		key, _, found := strings.Cut(line, ":")
		if !found {
			continue
		}
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		keys = append(keys, key)
	}
	if len(keys) == 0 {
		return nil, fmt.Errorf("no schemata keys found")
	}
	return keys, nil
}
