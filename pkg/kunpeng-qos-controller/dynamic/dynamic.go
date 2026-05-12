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
// Description: This file is used for dynamic cache limit level setting

// Package dyncache is the service used for cache limit setting
package dynamic

import (
	"fmt"
	"time"

	"k8s.io/klog"

	"kunpeng.huawei.com/kunpeng-cloud-computing/pkg/kunpeng-qos-controller/collector"
	"kunpeng.huawei.com/kunpeng-cloud-computing/pkg/kunpeng-qos-controller/typedef"
	"kunpeng.huawei.com/kunpeng-cloud-computing/pkg/kunpeng-qos-controller/util"
)

func (c *DynCache) startDynamic() {
	stepMore, stepLess := 5, -50
	needMore := true
	limiter := c.newCacheLimitSet(levelDynamic, c.Attr.L3PercentDynamic, c.Attr.MemBandPercentDynamic)

	for _, p := range c.listOnlinePods() {
		cacheMiss := getPodCacheMiss(p, c.config.PerfDuration)
		if cacheMiss >= c.Attr.MaxMiss {
			klog.Infof("online pod %v cache miss: %v exceeds maxmiss, lower offline cache limit",
				p.Name, cacheMiss)

			if err := c.flush(limiter, stepLess); err != nil {
				klog.Errorf("%v", err)
			}
			return
		}
		if cacheMiss >= c.Attr.MinMiss {
			needMore = false
		}
	}

	if needMore {
		if err := c.flush(limiter, stepMore); err != nil {
			klog.Errorf("%v", err)
		}
	}
}

func getPodCacheMiss(pod *typedef.PodInfo, perfDu int) int {
	cgroupPath := typedef.AbsoluteCgroupPath("perf_event", pod.Path, "")
	if !util.PathExist(cgroupPath) {
		klog.Errorf("pod: %s cgroup perf_event path: %s is not exist", pod.Name, cgroupPath)
		return 0
	}

	metrics := []string{"llcmr"}
	p, err := collector.NewPerfGroupCollectorSimple(cgroupPath, metrics)
	if err != nil {
		klog.Errorf("create perf group collector failed, err: %s", err)
		return 0
	}

	defer p.Close()

	err = p.Collect(time.Duration(perfDu) * time.Millisecond)
	if err != nil {
		klog.Errorf("collect cacheMiss failed, err: %s", err)
		return 0
	}
	const (
		probability = 100.0
		bias        = 1.0
	)

	result := p.GetResult()
	cr, cm := result["llcache-references"], result["llcache-misses"]
	return int(probability * float64(cm) / (bias + float64(cr)))
}

func (c *DynCache) listOnlinePods() map[string]*typedef.PodInfo {
	return c.podmanager.ListPodsWithOptions(func(pi *typedef.PodInfo) bool {
		return pi.Online()
	})
}

func (c *DynCache) flush(limitSet *limitSet, step int) error {
	var nextPercent = func(value, min, max, step int) int {
		value += step
		if value < min {
			return min
		}
		if value > max {
			return max
		}
		return value

	}
	l3 := nextPercent(c.Attr.L3PercentDynamic, c.config.L3Percent.Low, c.config.L3Percent.High, step)
	mb := nextPercent(c.Attr.MemBandPercentDynamic, c.config.MemBandPercent.Low, c.config.MemBandPercent.High, step)
	if c.Attr.L3PercentDynamic == l3 && c.Attr.MemBandPercentDynamic == mb {
		return nil
	}
	klog.Infof("flush L3 from %v to %v, Mb from %v to %v", limitSet.l3Percent, l3, limitSet.mbPercent, mb)
	limitSet.l3Percent, limitSet.mbPercent = l3, mb
	return c.doFlush(limitSet)
}

func (c *DynCache) doFlush(limitSet *limitSet) error {
	if err := limitSet.writeResctrlSchemata(c.Attr.NumaNum, c.Attr.L3IdList); err != nil {
		return fmt.Errorf("adjust dynamic cache limit to l3:%v mb:%v error: %v",
			limitSet.l3Percent, limitSet.mbPercent, err)
	}
	c.Attr.L3PercentDynamic = limitSet.l3Percent
	c.Attr.MemBandPercentDynamic = limitSet.mbPercent

	return nil
}
