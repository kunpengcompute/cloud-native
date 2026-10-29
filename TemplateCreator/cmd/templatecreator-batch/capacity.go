package main

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"

	templatecreator "gitcode.com/boostkit/cloud-native.git/TemplateCreator"
)

// capacityWithDefaults 先按公共 API 的零值继承规则解析资源，再估计同时运行的峰值。
func capacityWithDefaults(check func([]batchItem, int) error, items []batchItem, defaults templatecreator.BuildSpec, parallel int) error {
	resolved := make([]batchItem, len(items))
	for i, item := range items {
		resolved[i] = item
		build := resolvedBuild(defaults, item.Request.BuildOverride)
		resolved[i].Request.BuildOverride = &build
	}
	return check(resolved, parallel)
}

func largestSum(items []batchItem, parallel int, measure func(templatecreator.BuildSpec) int64) int64 {
	values := make([]int64, len(items))
	for i, item := range items {
		values[i] = measure(*item.Request.BuildOverride)
	}
	sort.Slice(values, func(i, j int) bool { return values[i] > values[j] })
	if parallel > len(values) {
		parallel = len(values)
	}
	var sum int64
	for _, value := range values[:parallel] {
		if value > math.MaxInt64-sum {
			return math.MaxInt64
		}
		sum += value
	}
	return sum
}

func memInfoValues(path string) (map[string]int64, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	values := make(map[string]int64)
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		n, err := strconv.ParseInt(fields[1], 10, 64)
		if err == nil {
			values[strings.TrimSuffix(fields[0], ":")] = n
		}
	}
	return values, nil
}

func availableBytes(path string) (uint64, error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return 0, err
	}
	return stat.Bavail * uint64(stat.Bsize), nil
}

// sameFilesystem 判断 scratch 与 Store 是否竞争同一份文件系统空闲空间。
func sameFilesystem(left, right string) (bool, error) {
	leftInfo, err := os.Stat(left)
	if err != nil {
		return false, err
	}
	rightInfo, err := os.Stat(right)
	if err != nil {
		return false, err
	}
	leftStat, leftOK := leftInfo.Sys().(*syscall.Stat_t)
	rightStat, rightOK := rightInfo.Sys().(*syscall.Stat_t)
	if !leftOK || !rightOK {
		return false, fmt.Errorf("cannot identify filesystem devices")
	}
	return leftStat.Dev == rightStat.Dev, nil
}

// checkDiskBudget 将 scratch 并发峰值与 Store 的整批累计产物分开估算。
// 同一文件系统上的两项需求必须相加，不能各自与同一份空闲空间比较。
func checkDiskBudget(items []batchItem, parallel int, scratchFreeMiB, storeFreeMiB uint64, shared bool) error {
	scratchNeed := uint64(largestSum(items, parallel, func(b templatecreator.BuildSpec) int64 { return b.DiskSizeMB }))
	storeNeed := uint64(largestSum(items, len(items), func(b templatecreator.BuildSpec) int64 { return b.DiskSizeMB }))
	if shared {
		combined := scratchNeed + storeNeed
		if combined < scratchNeed {
			combined = math.MaxUint64
		}
		if scratchFreeMiB < combined || storeFreeMiB < combined {
			return fmt.Errorf("shared scratch/store free disk is below %d MiB batch budget", combined)
		}
		return nil
	}
	if scratchFreeMiB < scratchNeed {
		return fmt.Errorf("scratch free disk %d MiB is below %d MiB concurrent budget", scratchFreeMiB, scratchNeed)
	}
	if storeFreeMiB < storeNeed {
		return fmt.Errorf("Store free disk %d MiB is below %d MiB whole-batch budget", storeFreeMiB, storeNeed)
	}
	return nil
}

// freeNBDDevices 只计算当前未连接的设备；这是并发数的必要下界，
// 单个 Builder 可能消耗更多设备，实际分配仍由 runtime pool 仲裁。
func freeNBDDevices(root string) (int, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return 0, err
	}
	count := 0
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasPrefix(name, "nbd") {
			continue
		}
		if _, err := strconv.Atoi(strings.TrimPrefix(name, "nbd")); err != nil {
			continue
		}
		data, err := os.ReadFile(filepath.Join(root, name, "size"))
		if err != nil {
			continue
		}
		if strings.TrimSpace(string(data)) == "0" {
			count++
		}
	}
	return count, nil
}

// checkCapacityAt 是保守的只读早期拒绝：按最大并发数取峰值需求。
// 真实构建还会消耗镜像层、snapshot 和缓存，宿主状态也可能在检查后变化。
func checkCapacityAt(items []batchItem, parallel int, scratchRoot, storeRoot string) error {
	if parallel <= 0 {
		return fmt.Errorf("parallelism must be positive")
	}
	if parallel > len(items) {
		parallel = len(items)
	}
	freeNBD, err := freeNBDDevices("/sys/block")
	if err != nil {
		return fmt.Errorf("inspect NBD devices: %w", err)
	}
	if freeNBD < parallel {
		return fmt.Errorf("%d free NBD devices cannot cover %d concurrent builds", freeNBD, parallel)
	}
	values, err := memInfoValues("/proc/meminfo")
	if err != nil {
		return fmt.Errorf("read host memory: %w", err)
	}
	memNeed := largestSum(items, parallel, func(b templatecreator.BuildSpec) int64 {
		if b.HugePages {
			return 0
		}
		return b.MemoryMB
	})
	if available, ok := values["MemAvailable"]; !ok || available/1024 < memNeed {
		return fmt.Errorf("host MemAvailable %d MiB is below %d MiB concurrent guest memory", available/1024, memNeed)
	}
	hugeNeed := largestSum(items, parallel, func(b templatecreator.BuildSpec) int64 {
		if b.HugePages {
			return b.MemoryMB
		}
		return 0
	})
	if hugeNeed > 0 {
		pageKB, hasPage := values["Hugepagesize"]
		free, hasFree := values["HugePages_Free"]
		if !hasPage || !hasFree || pageKB <= 0 || free < 0 {
			return fmt.Errorf("cannot safely evaluate HugePages capacity")
		}
		availableKB := int64(math.MaxInt64)
		if free <= math.MaxInt64/pageKB {
			availableKB = free * pageKB
		}
		if availableKB/1024 < hugeNeed {
			return fmt.Errorf("free HugePages cannot cover %d MiB concurrent guest memory", hugeNeed)
		}
	}
	scratchFree, err := availableBytes(scratchRoot)
	if err != nil {
		return fmt.Errorf("inspect scratch disk: %w", err)
	}
	storeFree, err := availableBytes(storeRoot)
	if err != nil {
		return fmt.Errorf("inspect Store disk: %w", err)
	}
	shared, err := sameFilesystem(scratchRoot, storeRoot)
	if err != nil {
		return fmt.Errorf("compare scratch/Store filesystems: %w", err)
	}
	if err := checkDiskBudget(items, parallel, scratchFree/1024/1024, storeFree/1024/1024, shared); err != nil {
		return err
	}
	return nil
}
