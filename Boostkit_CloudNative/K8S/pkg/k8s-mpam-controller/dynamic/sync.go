// Copyright (c) Huawei Technologies Co., Ltd. 2023. All rights reserved.
// rubik licensed under the Mulan PSL v2.
// You can use this software according to the terms and conditions of the Mulan PSL v2.
// You may obtain a copy of Mulan PSL v2 at:
//     http://license.coscl.org.cn/MulanPSL2
// THIS SOFTWARE IS PROVIDED ON AN "AS IS" BASIS, WITHOUT WARRANTIES OF ANY KIND, EITHER EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO NON-INFRINGEMENT, MERCHANTABILITY OR FIT FOR A PARTICULAR
// PURPOSE.
// See the Mulan PSL v2 for more details.
// Author: Xiang Li
// Create: 2023-02-21
// Description: This file is used for cache limit sync setting

// Package dyncache is the service used for cache limit setting
package dynamic

import (
	"fmt"
	"path/filepath"
	"strings"

	"k8s.io/klog"

	"kunpeng.huawei.com/kunpeng-cloud-computing/pkg/k8s-mpam-controller/typedef"
	"kunpeng.huawei.com/kunpeng-cloud-computing/pkg/k8s-mpam-controller/util"
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
func (c *DynCache) syncCacheLimitAndCPUQOS() {
	for _, p := range c.listOfflinePods() {
		if err := c.writeTasksToResctrl(p); err != nil {
			klog.Errorf("failed to set cache limit for pod %v: %v", p.Name, err)
			continue
		}

		if err := c.writeCPUQOS(p); err != nil {
			klog.Errorf("failed to set cpu qos for pod %v: %v", p.Name, err)
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

func (c *DynCache) writeCPUQOS(pod *typedef.PodInfo) error {
	if !util.PathExist(typedef.AbsoluteCgroupPath("cpu", pod.Path, "")) {
		// just return since pod maybe deleted
		return nil
	}

	for _, container := range pod.IDContainersMap {
		cgroupKey := &typedef.Key{SubSys: "cpu", FileName: "cpu.qos_level"}
		key := container.GetCgroupAttr(cgroupKey)
		if key.Err != nil {
			return fmt.Errorf("get container: %v cpu qos level failed, err: %v", container.Name, key.Err)
		}

		// 当前容器已经设置cpu.qos_level值为-1，无需再进行设置
		if key.Value == "-1" {
			continue
		}

		if err := container.SetCgroupAttr(cgroupKey, "-1"); err != nil {
			return fmt.Errorf("set container: %v cpu qos level failed, err: %v", container.Name, err)
		}
		klog.Infof("set container: %v cpu qos level to -1", container.Name)
	}

	return nil
}
