/*
Copyright 2022 The Koordinator Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package collector 提供采集Pod底层指标相关功能
package collector

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"
	"k8s.io/klog"

	"kunpeng.huawei.com/kunpeng-cloud-computing/pkg/k8s-mpam-controller/util"
)

var (
	// EventsMap records the event that perf can collect
	EventsMap = map[string][]string{
		"cpi":   {"cycles", "instructions"},
		"cmr":   {"cache-misses", "cache-references"},     // cmr means Cache Miss Ratio
		"llcmr": {"llcache-misses", "llcache-references"}, // llcmr means LLC Cache Miss Ratio
	}
	attrMap = make(map[string]*unix.PerfEventAttr)
)

func init() {
	for _, events := range EventsMap {
		for _, event := range events {
			attr, err := getConfigAndType(event)
			if err != nil {
				panic(err)
			}

			attr.Read_format = unix.PERF_FORMAT_GROUP |
				unix.PERF_FORMAT_TOTAL_TIME_ENABLED |
				unix.PERF_FORMAT_TOTAL_TIME_RUNNING |
				unix.PERF_FORMAT_ID
			attr.Size = uint32(unsafe.Sizeof(unix.PerfEventAttr{}))
			attr.Bits |= unix.PerfBitInherit
			attr.Bits |= unix.PerfBitDisabled
			attrMap[event] = attr
		}
	}
}

// PerfValue is a strurt for perf collect when format is set PERF_FORMAT_ID
type PerfValue struct {
	Value uint64
	ID    uint64
}

type perfValueHeader struct {
	Nr          uint64
	TimeEnabled uint64
	TimeRunning uint64
}

type perfCollector struct {
	cpu      int
	leaderFd *os.File
	fds      []io.ReadCloser
}

// PerfGroupCollector records all perfcollector in all cpus
type PerfGroupCollector struct {
	cgroupFile     *os.File
	cpus           []int
	perfCollectors map[int]*perfCollector
	resultMap      map[string]uint64 // Key: event name
	idEventMap     map[uint64]string // Key: event id, 通过ioctl生成
}

// NewPerfGroupCollectorSimple creates a perfgroupcollector with metrics
func NewPerfGroupCollectorSimple(cgroupPath string, metrics []string) (*PerfGroupCollector, error) {
	cgroupFile, err := os.Open(cgroupPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file, file: %s, err: %s", cgroupPath, err)
	}

	var events []string
	for _, metric := range metrics {
		if _, ok := EventsMap[metric]; ok {
			events = append(events, EventsMap[metric]...)
		} else {
			klog.Errorf("metric: %s is not in EventMap", metric)
		}
	}

	cpuNum := util.GetCPUNum()
	cpus := make([]int, cpuNum)
	for i := range cpus {
		cpus[i] = i
	}

	return newPerfGroupCollector(cgroupFile, cpus, events)
}

func newPerfGroupCollector(
	cgroupFile *os.File,
	cpus []int,
	events []string,
) (collector *PerfGroupCollector, err error) {
	if len(events) == 0 {
		err = fmt.Errorf("events should not be empty")
		return nil, err
	}

	collector = &PerfGroupCollector{
		cgroupFile:     cgroupFile,
		cpus:           cpus,
		perfCollectors: map[int]*perfCollector{},
		idEventMap:     make(map[uint64]string),
		resultMap:      make(map[string]uint64),
	}

	// 每一个CPU上都要创建一个perf_event
	for _, cpu := range cpus {
		pc := perfCollector{}
		pc.fds = make([]io.ReadCloser, 0, len(events)-1)
		pc.cpu = cpu
		attr := attrMap[events[0]]
		defaultFd := -1
		// 初始化perf group的leader
		leaderFd, err := unix.PerfEventOpen(attr, int(cgroupFile.Fd()), cpu, defaultFd,
			unix.PERF_FLAG_PID_CGROUP|unix.PERF_FLAG_FD_CLOEXEC)

		if err != nil {
			klog.Errorf("PerfEventOpen Failed, attr: %v, err: %s", attr, err)
			return nil, err
		}
		pc.leaderFd = os.NewFile(uintptr(leaderFd), fmt.Sprintf("%s_%d", events[0], cpu))
		var id uint64
		id, err = getPerfEventID(leaderFd)
		if err != nil {
			klog.Errorf("Get Perf Event Id Failed, err: %s, cpu: %d, event: %s\n", err, cpu, events[0])
			return nil, err
		}
		collector.idEventMap[id] = events[0]

		// 初始化perf group中其他的事件
		for i := 1; i < len(events); i++ {
			attr := attrMap[events[i]]
			fd, err := unix.PerfEventOpen(attr, int(cgroupFile.Fd()), cpu, leaderFd,
				unix.PERF_FLAG_PID_CGROUP|unix.PERF_FLAG_FD_CLOEXEC)
			if err != nil {
				klog.Errorf("Perf Event Open Failed, err: %s, cpu: %d, events: %s\n", err, cpu, events[i])
				return nil, err
			}

			file := os.NewFile(uintptr(fd), fmt.Sprintf("%s_%d", events[i], cpu))
			pc.fds = append(pc.fds, file)
			id, err := getPerfEventID(fd)
			if err != nil {
				klog.Errorf("Get Perf Event Id Failed, err: %s, cpu: %d, event: %s\n", err, cpu, events[i])
				return nil, err
			}
			collector.idEventMap[id] = events[i]
		}

		collector.perfCollectors[cpu] = &pc
	}

	return collector, nil
}

// Start starts the perfgroupcollector
func (p *PerfGroupCollector) Start() error {
	for _, perfcollector := range p.perfCollectors {
		if err := perfcollector.start(); err != nil {
			return err
		}
	}
	return nil
}

// Stop stops the perfcollector
func (p *PerfGroupCollector) Stop() error {
	for _, perfcollector := range p.perfCollectors {
		if err := perfcollector.stop(); err != nil {
			return err
		}
	}
	return nil
}

// Collect starts the perfcollector
func (p *PerfGroupCollector) Collect(sampleDur time.Duration) error {
	// 清空之前的数据
	clear(p.resultMap)
	// 开始采集
	if err := p.Start(); err != nil {
		return err
	}

	time.Sleep(sampleDur)
	if err := p.Stop(); err != nil {
		return err
	}

	for _, cpu := range p.cpus {
		if pc, ok := p.perfCollectors[cpu]; ok {
			perfvalue, err := pc.collect()
			if err != nil {
				klog.Errorf("Cgroup: %s CPU: %d, perf event collect failed\n", p.cgroupFile.Name(), cpu)
				continue
			}
			for _, v := range perfvalue {
				p.resultMap[p.idEventMap[v.ID]] += v.Value
			}
		}
	}

	return nil
}

// GetResult get the result of perf collect for pod
func (p *PerfGroupCollector) GetResult() map[string]uint64 {
	return p.resultMap
}

func (p *perfCollector) collect() ([]PerfValue, error) {
	buf := make([]byte, 1024)
	_, err := p.leaderFd.Read(buf)
	if err != nil {
		klog.Errorf("Read Perf event Failed, err: %s\n", err)
		return nil, err
	}

	// 读取perfheader内容获取采集的事件数量和实际采集的时间
	header := &perfValueHeader{}
	reader := bytes.NewReader(buf)
	if err := binary.Read(reader, binary.LittleEndian, header); err != nil {
		klog.Errorf("Read Perfheader Failed, err: %s\n", err)
		return nil, err
	}

	// 处理multiplexing
	scalingRatio := 1.0
	if header.TimeRunning != 0 && header.TimeEnabled != 0 {
		scalingRatio = float64(header.TimeRunning) / float64(header.TimeEnabled)
	}

	var res []PerfValue
	for i := 0; i < int(header.Nr); i++ {
		v := PerfValue{}
		if err := binary.Read(reader, binary.LittleEndian, &v); err != nil {
			klog.Errorf("Read Perf Event Value Failed, err: %s\n", err)
			return nil, err
		}

		value := PerfValue{}
		value.Value = uint64(float64(v.Value) / scalingRatio)
		value.ID = v.ID
		res = append(res, value)
	}

	return res, nil
}

func (p *perfCollector) start() error {
	if err := resetPerfGroupEvent(int(p.leaderFd.Fd())); err != nil {
		klog.Errorf("Reset LeaderFd Failed, err: %s\n", err)
		return err
	}

	if err := enablePerfGroupEvent(int(p.leaderFd.Fd())); err != nil {
		klog.Errorf("Enable LeaderFd Failed, err: %s\n", err)
		return err
	}

	return nil
}

func (p *perfCollector) stop() error {
	if err := disablePerfGroupEvent(int(p.leaderFd.Fd())); err != nil {
		klog.Errorf("Disable LeaderFd Failed, err: %s\n", err)
		return err
	}

	return nil
}

func (p *perfCollector) close() error {
	if err := p.leaderFd.Close(); err != nil {
		return fmt.Errorf("close leaderFd failed, err: %s", err)
	}

	for _, fd := range p.fds {
		if err := fd.Close(); err != nil {
			return fmt.Errorf("close group fd failed, err: %s", err)
		}
	}

	return nil
}

func getConfigAndType(event string) (*unix.PerfEventAttr, error) {
	const UNDEFINED = 0xffff
	attr := unix.PerfEventAttr{}
	switch event {
	case "cycles":
		attr.Type = unix.PERF_TYPE_HARDWARE
		attr.Config = unix.PERF_COUNT_HW_CPU_CYCLES
	case "instructions":
		attr.Type = unix.PERF_TYPE_HARDWARE
		attr.Config = unix.PERF_COUNT_HW_INSTRUCTIONS
	case "cache-misses":
		attr.Type = unix.PERF_TYPE_HARDWARE
		attr.Config = unix.PERF_COUNT_HW_CACHE_MISSES
	case "cache-references":
		attr.Type = unix.PERF_TYPE_HARDWARE
		attr.Config = unix.PERF_COUNT_HW_CACHE_REFERENCES
	case "llcache-misses":
		attr.Type = unix.PERF_TYPE_HW_CACHE
		attr.Config = unix.PERF_COUNT_HW_CACHE_LL | unix.PERF_COUNT_HW_CACHE_OP_READ<<8 |
			unix.PERF_COUNT_HW_CACHE_RESULT_MISS<<16
	case "llcache-references":
		attr.Type = unix.PERF_TYPE_HW_CACHE
		attr.Config = unix.PERF_COUNT_HW_CACHE_LL | unix.PERF_COUNT_HW_CACHE_OP_READ<<8 |
			unix.PERF_COUNT_HW_CACHE_RESULT_ACCESS<<16
	default:
		attr.Type = UNDEFINED
		attr.Config = UNDEFINED
	}

	if attr.Config == UNDEFINED {
		return nil, fmt.Errorf("event %s is not supported", event)
	}

	return &attr, nil
}

func getPerfEventID(fd int) (uint64, error) {
	var id uint64
	_, _, errno := unix.Syscall(
		unix.SYS_IOCTL,
		uintptr(fd),
		unix.PERF_EVENT_IOC_ID,
		uintptr(unsafe.Pointer(&id)),
	)

	if errno != 0 {
		return 0, errno
	}

	return id, nil
}

func enablePerfGroupEvent(fd int) error {
	_, _, errno := unix.Syscall(
		unix.SYS_IOCTL,
		uintptr(fd),
		unix.PERF_EVENT_IOC_ENABLE,
		unix.PERF_IOC_FLAG_GROUP,
	)

	if errno != 0 {
		return errno
	}

	return nil
}

func resetPerfGroupEvent(fd int) error {
	_, _, errno := unix.Syscall(
		unix.SYS_IOCTL,
		uintptr(fd),
		unix.PERF_EVENT_IOC_RESET,
		unix.PERF_IOC_FLAG_GROUP,
	)

	if errno != 0 {
		return errno
	}

	return nil
}

func disablePerfGroupEvent(fd int) error {
	_, _, errno := unix.Syscall(
		unix.SYS_IOCTL,
		uintptr(fd),
		unix.PERF_EVENT_IOC_DISABLE,
		unix.PERF_IOC_FLAG_GROUP,
	)

	if errno != 0 {
		return errno
	}

	return nil
}
