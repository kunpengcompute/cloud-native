// copyright to add

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
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/alecthomas/kingpin/v2"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	resctlMountPath = kingpin.Flag("collector.mpam.resctl.path", "resctl mountpoint.").Default("/sys/fs/resctrl").String()

	// DirNameInAGroup is the directory name in a mpam group.
	// It is used to distinguish a mpam (control or monitor) group.
	DirNameInAGroup = "mon_groups"

	// relative path where the L3 cache usage info and mem usage info is stored in a mpam group
	dirPathForMPAMGroupData = "mon_data"

	// relative filePaths in a mpam group. These helps to get the labels
	filePathForCpuListLabel = "cpus_list"
	filePathForModeLabel    = "mode"

	// key in name of the files/dirs that store the L3 cache usage info in dir: dirPathForMPAMGroupData
	nameKeyForCacheUsageDirs = "mon_L3"
	nameKeyForMemUsageDirs   = "mon_MB"
	// name of the file that store the L3 cache usage info in dir: dirPathForMPAMGroupData/nameKeyForCache+XXX
	fileNameForCache = "llc_occupancy"
	fileNameForMem   = "mbm_total_bytes"
)

type mpamCollector struct {
	cacheUsage  *prometheus.Desc
	cacheConfig *prometheus.Desc
	memUsage    *prometheus.Desc
	memConfig   *prometheus.Desc
	logger      *slog.Logger
}

// labels for the metrics
type mpamMetricsCommonLabels struct {
	groupName string
	cpuList   string
	mode      string
}

func init() {
	RegisterCollector("mpam", defaultEnabled, NewMPAMCollector)
}

// NewMPAMCollector() returns an collector instance
func NewMPAMCollector(logger *slog.Logger) (Collector, error) {
	if _, err := os.Stat(*resctlMountPath); err != nil {
		return nil, fmt.Errorf("failed to stat resctl mount path: %w", err)
	}
	return &mpamCollector{
		cacheUsage: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "mpam", "l3_cache_usage"),
			"MPAM L3 cache usage.",
			[]string{"group", "cpu_list", "mode", "id"},
			nil,
		),
		memUsage: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "mpam", "mem_usage"),
			"MPAM memory bw usage.",
			[]string{"group", "cpu_list", "mode", "id"},
			nil,
		),
		cacheConfig: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "mpam", "l3_cache_config"),
			"MPAM L3 cache config.",
			[]string{"group", "cpu_list", "mode", "item", "id"},
			nil,
		),
		memConfig: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "mpam", "mem_config"),
			"MPAM memory bw config.",
			[]string{"group", "cpu_list", "mode", "item", "id"},
			nil,
		),
		logger: logger,
	}, nil
}

// getMPAMGroups() traverse the resctrl dir to get all mpam groups.
// It returns a map of mpam group name to mpam group path relative to resctl mount path.
func (c *mpamCollector) getMPAMGroups() (map[string]string, error) {
	mpamGroups, err := getAllTargetDirs(*resctlMountPath, DirNameInAGroup)
	if err != nil {
		return nil, fmt.Errorf("failed to get mpam groups: %w", err)
	}
	if len(mpamGroups) == 0 {
		return nil, fmt.Errorf("mpam groups is empty")
	}
	return mpamGroups, err
}

// get all the labels of the target mpam group
// except the group name, it is equal to the dir name of a mpam group.
func (c *mpamCollector) getLabels(mpamGroupName string, mpamGroupRelPath string) (labels mpamMetricsCommonLabels, err error) {
	labels.groupName = mpamGroupName

	cpus, err := getFileContent(filepath.Join(*resctlMountPath, mpamGroupRelPath, filePathForCpuListLabel))
	if err != nil {
		return labels, fmt.Errorf("failed to get cpus_list: %w", err)
	}
	if cpus == "" {
		cpus = "null"
		c.logger.Info("cpus_list is empty, set to null", "group", mpamGroupName)
	}
	labels.cpuList = cpus
	labels.mode, err = getFileContent(filepath.Join(*resctlMountPath, mpamGroupRelPath, filePathForModeLabel))
	if err != nil {
		return labels, fmt.Errorf("failed to get mode: %w", err)
	}

	return labels, nil
}

// update rescource usage info, such as l3 cache usage and memory usage info
func (c *mpamCollector) updateResUsageMetrics(ch chan<- prometheus.Metric, mpamGroupRelPath string, labels mpamMetricsCommonLabels, metricType *prometheus.Desc, targetDirs []string) error {
	if len(targetDirs) == 0 {
		c.logger.Error("non fatal error, targetDir is empty. Skip update mpam resource usage metrics", "metric", metricType, "group", labels.groupName)
		return fmt.Errorf("non fatal error, targetDir is empty. Skip update mpam resource usage metrics, metric: %s, group: %s", metricType, labels.groupName)
	}

	filename := fileNameForCache
	if metricType == c.memUsage {
		filename = fileNameForMem
	}

	// "dir" is the dir name in dirPathForMPAMGroupData,
	// which store the resource usage info.
	// eg: mon_L3_XXX
	for _, dir := range targetDirs {
		path := filepath.Join(*resctlMountPath, mpamGroupRelPath, dirPathForMPAMGroupData, dir, filename)
		if _, err := os.Stat(path); err != nil {
			c.logger.Error("failed to stat file", "path", path, "err", err)
			continue
		}
		resUsage, err := os.ReadFile(path)
		if err != nil {
			c.logger.Error("failed to read file", "path", path, "err", err)
			continue
		}
		strValue := strings.TrimSpace(string(resUsage))
		resUsageData, err := strconv.ParseFloat(strValue, 64)
		if err != nil {
			c.logger.Error("failed to parse resource usage data %s: %w", strValue, err)
			continue
		}
		// set the dir name as id for the metric
		ch <- prometheus.MustNewConstMetric(
			metricType, prometheus.GaugeValue, float64(resUsageData), labels.groupName, labels.cpuList, labels.mode, dir)
		c.logger.Info("add metric", "metric", metricType, "value", resUsageData, "labels", labels)
	}
	return nil
}

func (c *mpamCollector) updateMPAMResUsageMetrics(ch chan<- prometheus.Metric, mpamGroupRelPath string, labels mpamMetricsCommonLabels) error {
	// get the relative paths of the dirs which store mpam resource usage info.
	// the paths are relative to /path/to/resctlMountPath/mpamGroupRelPath/dirPathForMPAMGroupData/
	targetDirPath := filepath.Join(*resctlMountPath, mpamGroupRelPath, dirPathForMPAMGroupData)
	L3CacheUsageDirs, MemUsageDirs, err := listResInfoSubDirs(targetDirPath, nameKeyForCacheUsageDirs, nameKeyForMemUsageDirs)
	if err != nil {
		return fmt.Errorf("failed to get the dirs which store mpam rescource usage info: %w", err)
	}

	if L3CacheUsageDirs == nil {
		c.logger.Error("non fatal error, L3CacheUsageDirs is empty. Skip update mpam l3 cache usage metrics", "group", labels.groupName)
	} else {
		err = c.updateResUsageMetrics(ch, mpamGroupRelPath, labels, c.cacheUsage, L3CacheUsageDirs)
		if err != nil {
			return fmt.Errorf("failed to update mpam l3 cache usage info: %w", err)
		}
	}

	if MemUsageDirs == nil {
		c.logger.Error("non fatal error, MemUsageDirs is empty. Skip update mpam memory usage metrics", "group", labels.groupName)
	} else {
		err = c.updateResUsageMetrics(ch, mpamGroupRelPath, labels, c.memUsage, MemUsageDirs)
		if err != nil {
			return fmt.Errorf("failed to update mpam memory usage info: %w", err)
		}
	}
	return nil
}

func (c *mpamCollector) updateMPAMResConfigMetrics(ch chan<- prometheus.Metric, mpamGroupRelPath string, labels mpamMetricsCommonLabels) error {
	return nil
}

// update mpam resource usage metrics and config metrics
func (c *mpamCollector) Update(ch chan<- prometheus.Metric) error {
	mpamGroups, err := c.getMPAMGroups()
	if err != nil {
		return fmt.Errorf("failed to get mpam groups: %w", err)
	}
	// udpate metrics group by group
	for mpamGroupName, mpamGroupRelPath := range mpamGroups {
		labels, err := c.getLabels(mpamGroupName, mpamGroupRelPath)
		if err != nil {
			return fmt.Errorf("failed to get labels: %w", err)
		}

		err = c.updateMPAMResUsageMetrics(ch, mpamGroupRelPath, labels)
		if err != nil {
			return fmt.Errorf("failed to update resource usage metrics: %w", err)
		}

		err = c.updateMPAMResConfigMetrics(ch, mpamGroupRelPath, labels)
		if err != nil {
			return fmt.Errorf("failed to update resource config metrics: %w", err)
		}

	}

	return nil
}
