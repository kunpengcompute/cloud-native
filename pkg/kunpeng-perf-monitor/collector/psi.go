// Copyright 2017 The Prometheus Authors
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
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
)

// psi data for a item (full or some) of a resource (cpu/io/memory/irq) in a cgroup group
type PSICgroupResourceItemData map[string]float64

// psi data for a resource (cpu/io/memory/irq) in a cgroup group
type PSICgroupResourceData map[string]PSICgroupResourceItemData

// e.g. content of file "cpu.pressure" in cgroup cg1 is "full avg10=0.00 avg60=0.00 avg300=0.00 total=123456"
// then PSICgroupResourceData["full"] is:
// PSICgroupResourceItemData{"avg10": "0.00", "avg60": "0.00", "avg300": "0.00", "total": "123456"}

var (
	// A psi cgroup has files like "cpu.pressure", "io.pressure" and "memory.pressure".
	// Here we use cpu.pressure as a feature file to locate the psi cgroups
	psiCgroupFeatureFile       = "cpu.pressure"
	commonPSIResourceFiles     = []string{"cpu.pressure", "io.pressure", "memory.pressure"}
	additionalPSIResourceFiles = []string{"irq.pressure"}

	targetDirForCgroupDrivers = []string{"kubepods", "kubepods.slice"}

	// vars for cgroup version check
	cgroupVersionCheckCmd = "df"
)

type psiCollector struct {
	// root SearchPath for k8s psi cgroups
	cgroupSearchPath string

	pressureMetric *prometheus.Desc
	logger         *slog.Logger
}

var (
	cgroupV1             = 1
	cgroupV2             = 2
	unknownCgroupVersion = -1
)

func init() {
	RegisterCollector("psi", defaultEnabled, NewPSICollector)
}

var execCommand = func(command string, args ...string) ([]byte, error) {
	return exec.Command(command, args...).Output()
}

func checkCgroupVersion(cgroupPath string, logger *slog.Logger) (version int, err error) {
	if cgroupPath == "" {
		return unknownCgroupVersion, fmt.Errorf("cgroupPath is empty")
	}
	// check cgroup version
	// 使用参数列表而非字符串拼接
	// 直接调用stat二进制文件而非通过shell
	output, err := execCommand(cgroupVersionCheckCmd, "-T", cgroupPath)
	if err != nil {
		return unknownCgroupVersion, fmt.Errorf("failed to execute command: %s -T %s, %v", cgroupVersionCheckCmd, cgroupPath, err)
	}
	outputStr := strings.TrimSpace(string(output))
	if strings.Contains(outputStr, "cgroup2") {
		logger.Info("cgroup version is cgroup v2.")
		return cgroupV2, nil
	} else if strings.Contains(outputStr, "tmpfs") {
		logger.Info("cgroup version is cgroup v1.")
		return cgroupV1, nil
	} else {
		logger.Info("unknown cgroup filesystem", "cgroup filesystem", string(output))
		return unknownCgroupVersion, fmt.Errorf("unknown cgroup filesystem: %s", string(output))
	}
}

// getFinalPath() returns the first existing path in the fileCandidates list.
// e.g. rootPath = "/sys/fs/cgroup", fileCandidates = []string{"kubepods", "kubepods.slice"},
// then the finalPath is "/sys/fs/cgroup/kubepods" or "/sys/fs/cgroup/kubepods.slice"
// If none of the files exists, return an error.
// kubepods is a directory in cgroup with cgroup driver "cgroupfs".
// kubepods.slice is a directory in cgroup with cgroup driver "systemd".
func getFinalPath(rootPath string, fileCandidates []string) (finalPath string, err error) {
	// check the path
	if rootPath == "" {
		return "", fmt.Errorf("rootPath is empty")
	}
	// check the files
	if len(fileCandidates) == 0 {
		return "", fmt.Errorf("target file list is empty")
	}

	for _, file := range fileCandidates {
		path := filepath.Join(rootPath, file)
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}
	return "", fmt.Errorf("no target file found in %s, target files: %v", rootPath, fileCandidates)
}

// getCgroupSearchPath() returns the cgroupSearchPath for psi collector.
// cgroupSearchPath = baseSearchPath + targetDirForCgroupDriver
// For cgroup1, the baseSearchPath is "/sys/fs/cgroup/cpu,cpuacct".
// For cgroup2, the baseSearchPath is "/sys/fs/cgroup".
// For cgroup driver "cgroupfs", the targetDirForCgroupDriver is "kubepods".
// For cgroup driver "systemd", the cgroupSearchPath is "kubepods.slice".
func getCgroupSearchPath(logger *slog.Logger) (cgroupSearchPath string, err error) {
	// check the path
	if cgroupMountPath == nil || *cgroupMountPath == "" {
		return "", fmt.Errorf("cgroupMountPath uninitialized")
	}

	// Delay the init of cgroupSearchPath until the first check.
	// This avoids the case that cgroupSearchPath is empty string
	if cgroupSearchPath == "" {
		cgroupSearchPath = *cgroupMountPath
	}

	// check cgroup version
	cgroupVersion, err := checkCgroupVersion(*cgroupMountPath, logger)
	if err != nil {
		return "", fmt.Errorf("failed to check cgroup version: %w", err)
	}

	if cgroupVersion == cgroupV1 {
		cgroupSearchPath = filepath.Join(cgroupSearchPath, "cpu,cpuacct")
	}

	// get the final cgroupSearchPath
	finalCgroupSearchPath, err := getFinalPath(cgroupSearchPath, targetDirForCgroupDrivers)
	if err != nil {
		return "", fmt.Errorf("failed to get cgroupSearchPath path: %w", err)
	}
	cgroupSearchPath = finalCgroupSearchPath
	logger.Info("getted cgroupSearchPath successfully", "cgroupSearchPath", cgroupSearchPath)
	return cgroupSearchPath, nil
}

// NewPSICollector() returns an collector instance
func NewPSICollector(logger *slog.Logger) (Collector, error) {
	if cgroupMountPath == nil || *cgroupMountPath == "" {
		return nil, fmt.Errorf("cgroupMountPath uninitialized")
	}

	if _, err := os.Stat(*cgroupMountPath); err != nil {
		return nil, fmt.Errorf("failed to stat cgroup mount path: %w", err)
	}
	cgroupSearchPath, err := getCgroupSearchPath(logger)
	if err != nil {
		return nil, fmt.Errorf("failed to check cgroup version: %w", err)
	}
	return &psiCollector{
		cgroupSearchPath: cgroupSearchPath,
		pressureMetric: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "psi", "psi metrics"),
			"PSI metrics.",
			[]string{"group", "resource", "item", "id"},
			nil,
		),
		logger: logger,
	}, nil
}

// getPSICgroups() traverse the cgroupSearchPath to get all psi cgroups with a feature file "cpu.pressure".
// It returns a map of psi cgroup name to psi cgroup path relative to cgroupSearchPath.
func (c *psiCollector) getPSICgroups() (PSICgroup map[string]string, err error) {
	// use psiCgroupFeatureFile "cpu.pressure" to locate psi cgroups
	PSICgroup, err = getAllTargetDirs(c.cgroupSearchPath, psiCgroupFeatureFile, false, c.logger)
	if err != nil {
		return nil, fmt.Errorf("failed to get PSI cgroups: %w", err)
	}
	if len(PSICgroup) == 0 {
		return nil, fmt.Errorf("PSI cgroup is empty")
	}
	return PSICgroup, err
}

// parse a line like: full avg10=0.00 avg60=0.00 avg300=0.00 total=123456
func parsePSILine(line string, delimiter string) (psiCgroupResourceData PSICgroupResourceData, err error) {
	fields := strings.SplitN(line, delimiter, 2)
	if len(fields) < 2 || fields[0] == "" || fields[1] == "" {
		return nil, fmt.Errorf("invalid line format: %s", line)
	}
	item := fields[0] // like "full"
	data := fields[1] // like "avg10=0.00 avg60=0.00 avg300=0.00 total=123456"
	psiCgroupResourceData = make(PSICgroupResourceData)
	psiCgroupResourceData[item] = make(PSICgroupResourceItemData)

	// parse the data
	keyValuePairs := strings.Split(data, delimiter)
	for _, pair := range keyValuePairs {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}

		// split the pair
		kv := strings.SplitN(pair, "=", 2)
		if len(kv) != 2 || kv[0] == "" || kv[1] == "" {
			return nil, fmt.Errorf("invalid key-value pair: %s", pair)
		}

		key := strings.TrimSpace(kv[0])
		valueStr := strings.TrimSpace(kv[1])

		// convert the value to int
		value, err := strToFloat64(valueStr, false)
		if err != nil {
			return nil, fmt.Errorf("failed to convert the string to float64: %s, error: %w", valueStr, err)
		}
		psiCgroupResourceData[item][key] = value
	}
	return psiCgroupResourceData, nil
}

// getPSIData() parses the psi file and returns the psi data.
func (c *psiCollector) getPSIData(psiFilePath string) (PSICgroupResourceData, error) {
	// get the fd
	file, err := os.Open(psiFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open psi file: %w", err)
	}
	defer file.Close() // nolint: errcheck

	psiCgroupResourceData := PSICgroupResourceData{}

	// parse the data line by line
	scanner := bufio.NewScanner(file)
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := scanner.Text()
		cgroupData, err := parsePSILine(line, " ")
		if err != nil {
			return nil, fmt.Errorf("error parsing line %d of file %v : %w", lineNumber, psiFilePath, err)
		}
		// add cgroupData to psiCgroupResourceData中
		for item, itemData := range cgroupData {
			// if item not exists, create a new one
			if _, exists := psiCgroupResourceData[item]; !exists {
				psiCgroupResourceData[item] = make(PSICgroupResourceItemData)
			}
			// add itemData to psiCgroupResourceData[item]
			for key, value := range itemData {
				psiCgroupResourceData[item][key] = value
			}
		}

	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return psiCgroupResourceData, nil
}

// getPSIMetric() parses the psi file and updates the metrics.
func (c *psiCollector) getPSIMetric(ch chan<- prometheus.Metric, psiCgroupName string, psiCgroupRelPath, resource string) error {
	filepathToParse := filepath.Join(c.cgroupSearchPath, psiCgroupRelPath, resource)
	psiCgroupResourceData, err := c.getPSIData(filepathToParse)
	if err != nil {
		return fmt.Errorf("failed to get psi data: %w", err)
	}
	if len(psiCgroupResourceData) == 0 {
		return fmt.Errorf("PSI cgroup resource data is empty for file %s", filepathToParse)
	}

	// update metrics item by item
	// e.g. content of file "cpu.pressure" in cgroup cg1 is "full avg10=0.00 avg60=0.00 avg300=0.00 total=123456"
	// then for PSICgroupResourceData:
	// there is a item "full" and itemData {"avg10": "0.00", "avg60": "0.00", "avg300": "0.00", "total": "123456"}
	for item, itemData := range psiCgroupResourceData {
		// for the itemData, "avgXX" and "total" are used as metric labels
		for key, value := range itemData {
			ch <- prometheus.MustNewConstMetric(
				c.pressureMetric,
				prometheus.GaugeValue,
				float64(value),
				psiCgroupName,
				resource,
				item,
				key,
			)
		}
	}

	return nil
}

// updatePSIMetrics() updates the psi metrics for a psi cgroup.
func (c *psiCollector) updatePSIMetrics(ch chan<- prometheus.Metric, psiCgroupName, psiCgroupRelPath string) error {
	// update metrics file by file
	for _, resFile := range commonPSIResourceFiles {
		if err := c.getPSIMetric(ch, psiCgroupName, psiCgroupRelPath, resFile); err != nil {
			return fmt.Errorf("failed to update psi metric from %s: %w", resFile, err)
		}
	}
	for _, resFile := range additionalPSIResourceFiles {
		if err := c.getPSIMetric(ch, psiCgroupName, psiCgroupRelPath, resFile); err != nil {
			c.logger.Error("non-fatal error: failed to update psi metrics from file", "file", resFile, "error", err)
		}
	}
	return nil
}

// Update() updates the metrics for all psi cgroups.
func (c *psiCollector) Update(ch chan<- prometheus.Metric) error {
	psiCgroups, err := c.getPSICgroups()
	if err != nil {
		return fmt.Errorf("failed to get PSI cgroups: %w", err)
	}
	// update metrics group by group
	for psiCgroupName, psiCgroupRelPath := range psiCgroups {
		err = c.updatePSIMetrics(ch, psiCgroupName, psiCgroupRelPath)
		if err != nil {
			groupPath := filepath.Join(c.cgroupSearchPath, psiCgroupRelPath)
			c.logger.Error("failed to update psi metrics for the group", "groupName", psiCgroupName, "groupPath", groupPath, "error", err)
		}
	}
	return nil
}
