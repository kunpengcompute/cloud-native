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
	"strconv"
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

// ReadIntFromFile 从文件中读取内容并将其转换为int类型
func ReadIntFromFile(file string) (int, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return 0, fmt.Errorf("read file failed, file: %s, err: %s", file, err)
	}

	res, err := strconv.Atoi(string(data))
	if err != nil {
		return 0, fmt.Errorf("string to int failed, string: %s, err: %s", string(data), err)
	}

	return res, nil
}

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

// WriteFile 将指定内容写入到指定文件
func WriteFile(path string, data string) error {
	if IsDir(path) {
		return fmt.Errorf("%s is not a file", path)
	}

	if !PathExist(path) {
		return fmt.Errorf("%s is not exist", path)
	}

	return os.WriteFile(path, []byte(data), DefaultFileWriteMode)
}

// IsDir 判断一个文件是否为目录
func IsDir(path string) bool {
	file, err := os.Lstat(path)
	if err != nil {
		return false
	}
	return file.IsDir()
}

// Mkdir 在指定路径创建目录
func Mkdir(name string, perm os.FileMode) error {
	if err := os.Mkdir(name, perm); err != nil && !os.IsExist(err) {
		return fmt.Errorf("failed to create directory: %s, err: %s", name, err)
	}

	return nil
}

// PathExist returns true if the path exists
func PathExist(path string) bool {
	if _, err := os.Lstat(path); err != nil {
		return false
	}

	return true
}
