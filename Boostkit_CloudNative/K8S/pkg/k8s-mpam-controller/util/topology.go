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
	"path/filepath"
	"runtime"
)

const (
	// NodePath 读取Node数量的路径
	NodePath string = "/sys/devices/system/node"
)

// GetCPUNum 获取当前机器的CPU数量
func GetCPUNum() int {
	return runtime.NumCPU()
}

// GetCPUList 获取指定起始区间内的CPU列表
func GetCPUList(start, end int) ([]int, error) {
	cpuNum := GetCPUNum()
	if start < 0 || end >= cpuNum {
		return nil, fmt.Errorf("start or end is out of CPU range range, start: %d, end: %d, cpuNum: %d", start, end, cpuNum)
	}

	var cpulist []int
	for i := start; i <= end; i++ {
		cpulist = append(cpulist, i)
	}

	return cpulist, nil
}

// GetNUMANum 获取当前机器的NUMA数量
func GetNUMANum() (int, error) {
	files, err := filepath.Glob(filepath.Join(NodePath, "node*"))
	if err != nil {
		return 0, err
	}

	return len(files), nil
}
