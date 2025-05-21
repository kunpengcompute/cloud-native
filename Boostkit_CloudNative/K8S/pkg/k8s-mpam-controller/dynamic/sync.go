package dynamic

import (
	"fmt"
	"path/filepath"
	"strings"

	"kunpeng.huawei.com/kunpeng-cloud-computing/pkg/k8s-mpam-controller/typedef"
	"kunpeng.huawei.com/kunpeng-cloud-computing/pkg/k8s-mpam-controller/util"

	"k8s.io/klog"
)

const (
	levelLow     = "low"
	levelMiddle  = "middle"
	levelHigh    = "high"
	levelMax     = "max"
	levelDynamic = "dynamic"

	resctrlDirPrefix = "mpam-controller_"
	schemataFileName = "schemata"
)

var validLevel = map[string]bool{
	levelLow:     true,
	levelMiddle:  true,
	levelHigh:    true,
	levelMax:     true,
	levelDynamic: true,
}

// SyncCacheLimit will continuously set cache limit with corresponding offline pods
func (c *DynCache) syncCacheLimit() {
	for _, p := range c.listOfflinePods() {
		if err := c.writeTasksToResctrl(p); err != nil {
			klog.Errorf("failed to set cache limit for pod %v: %v", p.Name, err)
			continue
		}
	}
}

// writeTasksToResctrl will write tasks running in containers into resctrl group
func (c *DynCache) writeTasksToResctrl(pod *typedef.PodInfo) error {
	if !util.PathExist(typedef.AbsoluteCgroupPath("cpu", pod.Path, "")) {
		// just return since pod maybe deleted
		return nil
	}
	var taskList []string
	cgroupKey := &typedef.Key{SubSys: "cpu", FileName: "cgroup.procs"}
	for _, container := range pod.IDContainersMap {
		key := container.GetCgroupAttr(cgroupKey)
		if key.Err != nil {
			return key.Err
		}
		taskList = append(taskList, strings.Split(key.Value, "\n")...)
	}
	if len(taskList) == 0 {
		return nil
	}

	resctrlTaskFile := filepath.Join(c.config.DefaultResctrlDir,
		resctrlDirPrefix+"dynamic", "tasks")
	for _, task := range taskList {
		if err := util.WriteFile(resctrlTaskFile, task); err != nil {
			if strings.Contains(err.Error(), "no such process") {
				klog.Errorf("pod %s task %s does not exist", pod.Name, task)
				continue
			}
			return fmt.Errorf("failed to add task %v to file %v: %v", task, resctrlTaskFile, err)
		}
	}

	return nil
}

func (c *DynCache) listOfflinePods() map[string]*typedef.PodInfo {
	return c.podmanager.ListPodsWithOptions(func(pi *typedef.PodInfo) bool {
		return pi.Offline()
	})
}
