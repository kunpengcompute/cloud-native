// copyright

// Copyright 2017 The Prometheus Authors
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

package collector

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// getAllTargetDirs() scans the rootDirPath recursively to find all the dirs whose name is TargetSubDir,
// and return a map of target_dir_name to target_dir_path relative to rootDirPath.
// The target_dir_name is the name of the parent dir of the TargetSubDir dir.
// For dir structure below, the result should be:
//  {"group1" "relative-path/to/group", "group2" "relative-path/to/group"}
//
/*
tmpRoot/
 ├── group1/
 │  └── l3_cache/
 └── group2/
	└── l3_cache/
*/
func getAllTargetDirs(rootDirPath string, TargetSubDir string) (map[string]string, error) {
	targetDirs := make(map[string]string)

	err := filepath.WalkDir(rootDirPath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		// when we find a TargetSubDir dir,
		// we get the name and path of the parent dir
		if d.IsDir() && d.Name() == TargetSubDir {
			parentDirPath := filepath.Dir(path)
			baseName := filepath.Base(parentDirPath)
			targetDirPath, err := filepath.Rel(rootDirPath, parentDirPath)
			if err != nil {
				return fmt.Errorf("failed to get relative path: %w", err)
			}
			targetDirs[baseName] = targetDirPath
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to walk dir: %w", err)
	}
	return targetDirs, nil
}

// list all the sub dirs in "path" that start with cacheUsageDirPrefix or memUsageDirPrefix.
// It return the names of the dirs that start with cacheUsageDirPrefix and memUsageDirPrefix respectively.
func listResInfoSubDirs(path string, cacheUsageDirPrefix string, memUsageDirPrefix string) ([]string, []string, error) {
	// requires read and exection permission
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read directory %s: %w", path, err)
	}

	var l3cacheUsageDirs []string
	var memUsageDirs []string
	for _, entry := range entries {
		if entry.IsDir() {
			if strings.HasPrefix(entry.Name(), cacheUsageDirPrefix) {
				l3cacheUsageDirs = append(l3cacheUsageDirs, entry.Name())
			}
			if strings.HasPrefix(entry.Name(), memUsageDirPrefix) {
				memUsageDirs = append(memUsageDirs, entry.Name())
			}
		}
	}
	return l3cacheUsageDirs, memUsageDirs, nil
}

// read the file
func getFileContent(filePath string) (string, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}
	return strings.TrimSpace(string(content)), nil
}
