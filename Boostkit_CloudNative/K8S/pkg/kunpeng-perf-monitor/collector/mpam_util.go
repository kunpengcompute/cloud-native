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

package collector

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// configItemData: the configData for a configItem
// e.g. configData [1:16777215 122:16777215 243:16777215 364:16777215] for configItem "L3"
type ConfigItemData map[string]float64

// ConfigData stores the parsed data in a mpam resource config file, including configItem (the keys) and configItemData (the values).
// configItem examples: "MBHDL" "MBPRI" "MBMIN" "MB" "L3PRI" "L3"
type ConfigData map[string]ConfigItemData

func strToFloat64(str string, isInt bool) (float64, error) {
	str = strings.TrimSpace(str)
	if str == "" {
		return 0, fmt.Errorf("failed to parse int/float: empty string")
	}

	if isInt {
		// convert intStr to int64 then to float64
		base := 10
		if strings.ContainsAny(str, "abcdefABCDEF") {
			base = 16
		}
		value, err := strconv.ParseInt(str, base, 64)
		if err != nil {
			return 0, fmt.Errorf("failed to parse int from string: %s", str)
		}
		return float64(value), nil
	} else {
		// if str is not intStr, then treat it as floatStr
		value, err := strconv.ParseFloat(str, 64)
		if err != nil {
			return 0, fmt.Errorf("failed to parse float64 from string: %s", str)
		}
		return value, nil
	}
}

func addToConfigData(configData *ConfigData, item string, key string, value float64) (err error) {
	if configData == nil {
		return fmt.Errorf("configData is nil")
	}
	if *configData == nil {
		*configData = make(ConfigData)
	}
	if _, ok := (*configData)[item]; !ok {
		(*configData)[item] = make(ConfigItemData)
	}
	(*configData)[item][key] = value
	return nil
}

// parse a line
// a link  is like: L3:1=ffffff;122=ffffff;243=ffffff;364=ffffff
func parseLine(line string, configData1 *ConfigData, configKey1 string, configData2 *ConfigData, configKey2 string) error {
	// remove the blank characters
	line = strings.TrimSpace(line)
	if line == "" {
		return nil // skip the empty line
	}

	// split a lint into configItem and dataStr
	// eg: "L3" and "1=ffffff;122=ffffff;243=ffffff;364=ffffff"
	parts := strings.SplitN(line, ":", 2)
	if len(parts) != 2 {
		return fmt.Errorf("invalid line format: %s", line)
	}
	item := strings.TrimSpace(parts[0])
	dataStr := strings.TrimSpace(parts[1])

	// parse the dataStr
	// split the dataStr into pairs like: XX=XXXXXX
	keyValuePairs := strings.Split(dataStr, ";")
	for _, pair := range keyValuePairs {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}

		// split the pair
		kv := strings.SplitN(pair, "=", 2)
		if len(kv) != 2 {
			return fmt.Errorf("invalid key-value pair: %s", pair)
		}

		key := strings.TrimSpace(kv[0])
		valueStr := strings.TrimSpace(kv[1])

		// convert the value to int64
		value, err := strToFloat64(valueStr, true)
		if err != nil {
			return fmt.Errorf("failed to convert the string to float64: %s, error: %w", valueStr, err)
		}

		if strings.Contains(item, configKey1) {
			err = addToConfigData(configData1, item, key, value)
		} else if strings.Contains(item, configKey2) {
			err = addToConfigData(configData2, item, key, value)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func getMPAMConfigData(filename string, configKey1 string, configKey2 string) (configData1 *ConfigData, configData2 *ConfigData, err error) {
	// get the fd
	file, err := os.Open(filename)
	if err != nil {
		return nil, nil, err
	}
	defer file.Close()

	// init configData1 and configData2
	configData1 = &ConfigData{}
	configData2 = &ConfigData{}

	// parse the file content line by line
	scanner := bufio.NewScanner(file)
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := scanner.Text()
		err = parseLine(line, configData1, configKey1, configData2, configKey2)
		if err != nil {
			return nil, nil, fmt.Errorf("error parsing line %d of file %v : %v", lineNumber, filename, err)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, nil, err
	}

	return configData1, configData2, nil
}

// getAllTargetDirs() scans the <rootDirPath> recursively to find all TargetSubDirOrFiles whose name is <TargetSubDirOrFile>,
// and returns a map of <target_dir_name> to <target_dir_path> relative to rootDirPath.
// The <target_dir_name> is the name of the parent dir of the <TargetSubDirOrFile>.
// For dir structure below ("l3_cache"s are dirs), if <targetIsDir> is true and <TargetSubDirOrFile> is "l3_cache", the result should be:
//  {"group1" "relative-path/to/group1", "group2" "relative-path/to/group2"}
/*
tmpRoot/
 ├── group1/
 │  └── l3_cache/
 └── group2/
	└── l3_cache/
*/
// If "l3_cache"s above are files, and <targetIsDir> is false, the result remains the same as before.
func getAllTargetDirs(rootDirPath string, TargetSubDirOrFile string, targetIsDir bool) (map[string]string, error) {
	targetDirs := make(map[string]string)

	err := filepath.WalkDir(rootDirPath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		// when we find a TargetSubDirOrFile ,
		// we get the name and relative path of the parent dir
		if d.IsDir() == targetIsDir && d.Name() == TargetSubDirOrFile {
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
