package dynamic

import (
	"fmt"
	"k8s-mpam-controller/pkg/collector"
	"k8s-mpam-controller/pkg/typedef"
	"k8s-mpam-controller/pkg/util"
	"time"

	"k8s.io/klog"
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
				klog.Errorf(err.Error())
			}
			return
		}
		if cacheMiss >= c.Attr.MinMiss {
			needMore = false
		}
	}

	if needMore {
		if err := c.flush(limiter, stepMore); err != nil {
			klog.Errorf(err.Error())
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
	if err := limitSet.writeResctrlSchemata(c.Attr.NumaNum); err != nil {
		return fmt.Errorf("adjust dynamic cache limit to l3:%v mb:%v error: %v",
			limitSet.l3Percent, limitSet.mbPercent, err)
	}
	c.Attr.L3PercentDynamic = limitSet.l3Percent
	c.Attr.MemBandPercentDynamic = limitSet.mbPercent

	return nil
}
