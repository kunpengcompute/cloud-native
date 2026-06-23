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
	"os"
)

// File permission
const (
	// DefaultUmask is default umask
	DefaultUmask = 0077
	// DefaultFileMode is file mode for cgroup files
	DefaultFileMode os.FileMode = 0600
	// DefaultDirMode is dir default mode
	DefaultDirMode os.FileMode = 0700
	// DefaultFileWriteMode is the default mode for write file
	DefaultFileWriteMode os.FileMode = 0644
)

// ReadFile 将文件中的内容读取成byte并返回
func ReadFile(path string) ([]byte, error) {
	if IsDir(path) {
		return nil, fmt.Errorf("%s is not a file", path)
	}

	if !PathExist(path) {
		return nil, fmt.Errorf("%s is not exist", path)
	}

	return os.ReadFile(path)
}

// IsDir 判断一个文件是否为目录
func IsDir(path string) bool {
	file, err := os.Lstat(path)
	if err != nil {
		return false
	}
	return file.IsDir()
}

// PathExist returns true if the path exists
func PathExist(path string) bool {
	if _, err := os.Lstat(path); err != nil {
		return false
	}

	return true
}
