//go:build linux

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
	"fmt"
	"log/slog"
	"sync"
	"time"

	"kunpeng.huawei.com/kunpeng-cloud-computing/pkg/kunpeng-perf-monitor/libkperf/kperf"

	"github.com/prometheus/client_golang/prometheus"
)

// if you want to enable this collector, here are the steps to do beforehand:
// 1. get libkperf code:
// 		git clone git@gitee.com:openeuler/libkperf.git
// 2. build go pkg of libkperf:
// 		cd libkperf
//      git submodule update --init
//      bash build.sh go=true
// 3. copy the files to std:
// 		cp -r go/src/libkperf /usr/local/go/src
// 	where /usr/local/go is the installation path of golang
// 4. set env var:
//  	export LD_LIBRARY_PATH=/usr/local/go/src/libkperf/lib:$LD_LIBRARY_PATH

type pmuCollector struct {
	hhaCrossNumaOpRatio   *prometheus.Desc // metric for HHA cross-numa operations ratio
	hhaCrossSocketOpRatio *prometheus.Desc // metric for HHA cross-socket operations ratio
	logger                *slog.Logger
}

var samplingTime = 1 * time.Second

// PMUCollectorInterface defines the methods required for PMU collector operations.
type PMUCollectorInterface interface {
	PmuDeviceOpen(attrs []kperf.PmuDeviceAttr) (int, error)
	PmuClose(fd int)
	PmuEnable(fd int) error
	PmuDisable(fd int) error
	PmuRead(fd int) (kperf.PmuDataVo, error)
	PmuDataFree(dataVo kperf.PmuDataVo)
	PmuGetDevMetric(dataVo kperf.PmuDataVo, attrs []kperf.PmuDeviceAttr) (kperf.PmuDeviceDataVo, error)
	DevDataFree(deviceData kperf.PmuDeviceDataVo)
}

// RealPMUCollector 单例模式实现
type RealPMUCollector struct{}

var (
	realPMUCollectorInstance *RealPMUCollector
	realPMUCollectorOnce     sync.Once
)

// GetRealPMUCollector 获取 RealPMUCollector 的单例实例
func GetRealPMUCollector() *RealPMUCollector {
	realPMUCollectorOnce.Do(func() {
		realPMUCollectorInstance = &RealPMUCollector{}
	})
	return realPMUCollectorInstance
}

func (r *RealPMUCollector) PmuDeviceOpen(attrs []kperf.PmuDeviceAttr) (int, error) {
	return kperf.PmuDeviceOpen(attrs)
}

func (r *RealPMUCollector) PmuClose(fd int) {
	kperf.PmuClose(fd)
}

func (r *RealPMUCollector) PmuEnable(fd int) error {
	return kperf.PmuEnable(fd)
}

func (r *RealPMUCollector) PmuDisable(fd int) error {
	return kperf.PmuDisable(fd)
}

func (r *RealPMUCollector) PmuRead(fd int) (kperf.PmuDataVo, error) {
	return kperf.PmuRead(fd)
}

func (r *RealPMUCollector) PmuDataFree(dataVo kperf.PmuDataVo) {
	kperf.PmuDataFree(dataVo)
}

func (r *RealPMUCollector) PmuGetDevMetric(dataVo kperf.PmuDataVo, attrs []kperf.PmuDeviceAttr) (kperf.PmuDeviceDataVo, error) {
	return kperf.PmuGetDevMetric(dataVo, attrs)
}

func (r *RealPMUCollector) DevDataFree(deviceData kperf.PmuDeviceDataVo) {
	kperf.DevDataFree(deviceData)
}

func init() {
	RegisterCollector("pmu", defaultEnabled, NewPMUCollector)
}

// NewPMUCollector() returns a collector instance
func NewPMUCollector(logger *slog.Logger) (Collector, error) {
	return &pmuCollector{
		hhaCrossNumaOpRatio: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "pmu", "HHA_cross_numa_operations_ratio"),
			"PMU HHA cross-numa operations ratio.",
			[]string{"numaID"},
			nil,
		),
		hhaCrossSocketOpRatio: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "pmu", "HHA_cross_socket_operations_ratio"),
			"PMU HHA cross-socket operations ratio.",
			[]string{"numaID"},
			nil,
		),
		logger: logger,
	}, nil
}

// getHHACrossNumaAndSocketDataWithCollector collects HHA cross-numa and socket operations ratio using the provided PMU collector.
func getHHACrossNumaAndSocketDataWithCollector(pmuCollector PMUCollectorInterface) (kperf.PmuDeviceDataVo, error) {
	deviceAttrs := []kperf.PmuDeviceAttr{
		{Metric: kperf.PMU_HHA_CROSS_NUMA},
		{Metric: kperf.PMU_HHA_CROSS_SOCKET}}

	// initialize PMU events for devices like L3 cache, DDRC and PCIe
	fd, err := pmuCollector.PmuDeviceOpen(deviceAttrs)
	if err != nil {
		return kperf.PmuDeviceDataVo{}, fmt.Errorf("failed to run PmuDeviceOpen(): %w", err)
	}
	defer func() {
		if fd != -1 {
			pmuCollector.PmuClose(fd)
		}
	}()

	// start to sample PMU events for <fd>
	// enable counting or sampling of task <fd>
	err = pmuCollector.PmuEnable(fd)
	if err != nil {
		return kperf.PmuDeviceDataVo{}, fmt.Errorf("failed to run PmuEnable(): %w", err)
	}
	time.Sleep(samplingTime)
	// disable counting or sampling of task <fd>
	err = pmuCollector.PmuDisable(fd)
	if err != nil {
		return kperf.PmuDeviceDataVo{}, fmt.Errorf("failed to run PmuDisable(): %w", err)
	}

	// collect data
	dataVo, err := pmuCollector.PmuRead(fd)
	if err != nil {
		return kperf.PmuDeviceDataVo{}, fmt.Errorf("failed to run PmuRead(): %w", err)
	}
	defer pmuCollector.PmuDataFree(dataVo)

	deviceDataVo, err := pmuCollector.PmuGetDevMetric(dataVo, deviceAttrs)
	if err != nil {
		return kperf.PmuDeviceDataVo{}, fmt.Errorf("failed to run PmuGetDevMetric(): %w", err)
	}
	return deviceDataVo, nil
}

func (c *pmuCollector) updateHHACrossNumaAndSocketOpRatio(ch chan<- prometheus.Metric, pmuCollector PMUCollectorInterface) error {
	deviceData, err := getHHACrossNumaAndSocketDataWithCollector(pmuCollector)
	defer pmuCollector.DevDataFree(deviceData)
	if err != nil {
		return fmt.Errorf("failed to get HHA cross-numa and socket data: %w", err)
	}
	for _, v := range deviceData.GoDeviceData {
		if v.Metric == kperf.PMU_HHA_CROSS_NUMA {
			ch <- prometheus.MustNewConstMetric(c.hhaCrossNumaOpRatio, prometheus.GaugeValue, v.Count, fmt.Sprintf("%v", v.NumaId))
		}
		if v.Metric == kperf.PMU_HHA_CROSS_SOCKET {
			ch <- prometheus.MustNewConstMetric(c.hhaCrossSocketOpRatio, prometheus.GaugeValue, v.Count, fmt.Sprintf("%v", v.NumaId))
		}
	}
	return nil
}

// update pmu metrics
func (c *pmuCollector) Update(ch chan<- prometheus.Metric) error {
	// 使用单例模式获取 RealPMUCollector 实例
	realCollector := GetRealPMUCollector()
	err := c.updateHHACrossNumaAndSocketOpRatio(ch, realCollector)
	if err != nil {
		return fmt.Errorf("failed to update HHA cross-numa and socket op ratio: %w", err)
	}
	return nil
}
