/*
 * Copyright (c) 2026 Huawei Technology corp.
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

package topology

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// SiblingPair means two logical CPUs on the same physical core.
type SiblingPair struct {
	CPU0 int
	CPU1 int
}

// String returns cpuset format.
func (p SiblingPair) String() string {
	if p.CPU0 < p.CPU1 {
		return fmt.Sprintf("%d,%d", p.CPU0, p.CPU1)
	}
	return fmt.Sprintf("%d,%d", p.CPU1, p.CPU0)
}

// DiscoverSiblingPairs discovers sibling pairs from sysfs.
func DiscoverSiblingPairs() ([]SiblingPair, error) {
	paths, err := filepath.Glob("/sys/devices/system/cpu/cpu*/topology/thread_siblings_list")
	if err != nil {
		return nil, fmt.Errorf("discover sibling files: %w", err)
	}
	if len(paths) == 0 {
		return nil, fmt.Errorf("no sibling topology found")
	}

	seen := map[string]struct{}{}
	out := make([]SiblingPair, 0, len(paths))
	for _, p := range paths {
		raw, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		pair, ok := parsePair(strings.TrimSpace(string(raw)))
		if !ok {
			continue
		}
		key := pair.String()
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, pair)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no valid sibling pair found")
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].CPU0 == out[j].CPU0 {
			return out[i].CPU1 < out[j].CPU1
		}
		return out[i].CPU0 < out[j].CPU0
	})
	return out, nil
}

func parsePair(s string) (SiblingPair, bool) {
	cpus, err := parseCPUList(s)
	if err != nil || len(cpus) != 2 {
		return SiblingPair{}, false
	}
	return SiblingPair{CPU0: cpus[0], CPU1: cpus[1]}, true
}

func parseCPUList(raw string) ([]int, error) {
	var cpus []int
	for _, item := range strings.Split(raw, ",") {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if strings.Contains(item, "-") {
			bounds := strings.Split(item, "-")
			if len(bounds) != 2 {
				return nil, fmt.Errorf("invalid cpu range: %s", item)
			}
			start, err := strconv.Atoi(strings.TrimSpace(bounds[0]))
			if err != nil {
				return nil, err
			}
			end, err := strconv.Atoi(strings.TrimSpace(bounds[1]))
			if err != nil {
				return nil, err
			}
			for cpu := start; cpu <= end; cpu++ {
				cpus = append(cpus, cpu)
			}
			continue
		}
		cpu, err := strconv.Atoi(item)
		if err != nil {
			return nil, err
		}
		cpus = append(cpus, cpu)
	}
	sort.Ints(cpus)
	return cpus, nil
}
