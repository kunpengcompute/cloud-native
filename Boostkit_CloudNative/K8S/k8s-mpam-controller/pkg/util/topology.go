package util

import (
	"fmt"
	"path/filepath"
	"runtime"
)

const (
	NodePath string = "/sys/devices/system/node"
)

func GetCPUNum() int {
	return runtime.NumCPU()
}

// [start, end]
func GetCPUList(start, end int) ([]int, error) {
	cpuNum := GetCPUNum()
	if start < 0 || end >= cpuNum {
		return nil, fmt.Errorf("start or end is out of CPU range range, start: %d, end: %d, cpuNum: %d", start, end, cpuNum)
	}

	cpulist := []int{}
	for i := start; i <= end; i++ {
		cpulist = append(cpulist, i)
	}

	return cpulist, nil
}

func GetNUMANum() (int, error) {
	files, err := filepath.Glob(filepath.Join(NodePath, "node*"))
	if err != nil {
		return 0, err
	}

	return len(files), nil
}
