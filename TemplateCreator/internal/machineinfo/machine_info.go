// Package machineinfo 采集创建 Firecracker snapshot 的宿主机 CPU 事实。
package machineinfo

import (
	"errors"
	"fmt"
	"runtime"

	"github.com/shirou/gopsutil/v4/cpu"
)

// Info 使用与 e2b-infra env_builds/machineinfo 相同的字段语义。
// 这些字段描述 snapshot 的构建机，不表示任意目标节点必然兼容。
type Info struct {
	CPUArchitecture string
	CPUFamily       string
	CPUModel        string
	CPUModelName    string
	CPUFlags        []string
}

// cpuInfo 是 gopsutil 输出的最小稳定副本，便于独立测试字段映射和 ARM64 fallback。
type cpuInfo struct {
	Family    string
	Model     string
	ModelName string
	Flags     []string
}

// Detect 读取当前宿主机 CPU；调用方应在构建开始前冻结返回值并写入产物 metadata。
func Detect() (Info, error) {
	stats, err := cpu.Info()
	if err != nil {
		return Info{}, fmt.Errorf("get CPU info: %w", err)
	}
	infos := make([]cpuInfo, len(stats))
	for index, stat := range stats {
		infos[index] = cpuInfo{
			Family: stat.Family, Model: stat.Model, ModelName: stat.ModelName,
			Flags: append([]string(nil), stat.Flags...),
		}
	}

	return fromCPUInfo(runtime.GOARCH, infos)
}

// fromCPUInfo 保持锁定 infra 的 CPU 字段选择规则：使用第一个逻辑 CPU 的信息。
func fromCPUInfo(architecture string, infos []cpuInfo) (Info, error) {
	if len(infos) == 0 {
		return Info{}, errors.New("unable to detect CPU platform from any source")
	}

	first := infos[0]
	family := first.Family
	model := first.Model
	if architecture == "arm64" {
		// gopsutil 在部分 ARM64 主机上不会从 /proc/cpuinfo 填充 family/model。
		// 该通用回退与 infra 一致，但它只适合同型 CPU 或实机验证后的恢复；
		// 不应据此假设不同 ARM 实现之间的 Firecracker snapshot 兼容。
		if family == "" {
			family = "arm64"
		}
		if model == "" {
			model = "0"
		}
	}
	if family == "" || model == "" {
		return Info{}, fmt.Errorf("unable to detect CPU family/model: %+v", first)
	}

	return Info{
		CPUArchitecture: architecture,
		CPUFamily:       family,
		CPUModel:        model,
		CPUModelName:    first.ModelName,
		CPUFlags:        append([]string{}, first.Flags...),
	}, nil
}
