package dynamic

import (
	"context"
	"fmt"
	"time"

	informer "kunpeng.huawei.com/kunpeng-cloud-computing/pkg/k8s-mpam-controller/infomer"

	"k8s.io/apimachinery/pkg/util/wait"
)

// default value
const (
	defaultAdInt       = 1000
	defaultPerfDur     = 1000
	defaultLowL3       = 20
	defaultMidL3       = 30
	defaultHighL3      = 50
	defaultLowMB       = 10
	defaultMidMB       = 30
	defaultHighMB      = 50
	defaultMaxMiss     = 20
	defaultMinMiss     = 10
	defaultResctrlDir  = "/sys/fs/resctrl"
	defaultNumaNodeDir = "/sys/devices/system/node"
)

// boundary value
const (
	minPercent = 2
	maxPercent = 100
	// minimum perf duration, unit ms
	minPerfDur = 10
	// maximum perf duration, unit ms
	maxPerfDur = 10000
	// min adjust interval, unit ms
	minAdjustInterval = 10
	// max adjust interval, unit ms
	maxAdjustInterval = 10000000

	minCacheMiss = 0

	maxCacheMiss = 100
)

// Config is cache limit config
type Config struct {
	// DefaultLimitMode is default cache limit method(static)
	DefaultLimitMode string `json:"defaultLimitMode,omitempty"`
	// DefaultResctrlDir is the path of resctrl control group
	// the resctrl dir is not supposed to be altered or exposed
	DefaultResctrlDir string `json:"-"`
	// DefaultPidNameSpace is the pid namespace used for test whether share host pid
	// the value is not supposed to be altered or exposed
	AdjustInterval int `json:"adjustInterval,omitempty"`
	// PerfDuration is online pod perf dection duration time
	PerfDuration int `json:"perfDuration,omitempty"`
	// L3Percent is L3 cache percent for each level
	L3Percent MultiLvlPercent `json:"l3Percent,omitempty"`
	// MemBandPercent is memory bandwidth percent for each level
	MemBandPercent MultiLvlPercent `json:"memBandPercent,omitempty"`
}

// DynCache is cache limit service structure
type DynCache struct {
	config     *Config
	Attr       *Attr
	podmanager *informer.PodManager
}

// Attr is cache limit attribute differ from config
type Attr struct {
	// NumaNodeDir is the path for numa node
	NumaNodeDir string
	// NumaNum stores numa number on physical machine
	NumaNum int
	// L3PercentDynamic stores l3 percent for dynamic cache limit setting
	// the value could be changed
	L3PercentDynamic int
	// MemBandPercentDynamic stores memory band percent for dynamic cache limit setting
	// the value could be changed
	MemBandPercentDynamic int
	// MaxMiss is the maximum value of cache miss
	MaxMiss int
	// MinMiss is the minimum value of cache miss
	MinMiss int
}

// MultiLvlPercent define multi level percentage
type MultiLvlPercent struct {
	Low  int `json:"low,omitempty"`
	Mid  int `json:"mid,omitempty"`
	High int `json:"high,omitempty"`
}

func newConfig() *Config {
	return &Config{
		DefaultResctrlDir: defaultResctrlDir,
		AdjustInterval:    defaultAdInt,
		PerfDuration:      defaultPerfDur,
		L3Percent: MultiLvlPercent{
			Low:  defaultLowL3,
			Mid:  defaultMidL3,
			High: defaultHighL3,
		},
		MemBandPercent: MultiLvlPercent{
			Low:  defaultLowMB,
			Mid:  defaultMidMB,
			High: defaultHighMB,
		},
	}
}

// newDynCache return cache limit instance with default settings
func NewDynCache(podManager *informer.PodManager) *DynCache {
	return &DynCache{
		podmanager: podManager,
		Attr: &Attr{
			NumaNodeDir: defaultNumaNodeDir,
			MaxMiss:     defaultMaxMiss,
			MinMiss:     defaultMinMiss,
		},
		config: newConfig(),
	}
}

// GetConfig returns Config
func (c *DynCache) GetConfig() interface{} {
	return c.config
}

// Run implement service run function
func (c *DynCache) Run(ctx context.Context) {
	const ReSyncTime = 10
	go wait.Until(c.syncCacheLimit, ReSyncTime*time.Second, ctx.Done())
	wait.Until(c.startDynamic, time.Millisecond*time.Duration(c.config.AdjustInterval), ctx.Done())
}

func (c *DynCache) PreStart() error {
	if c.podmanager == nil {
		return fmt.Errorf("invalid pod mananger")
	}

	return c.initDynCache()
}
