/*
Copyright (c) Huawei Technologies Co., Ltd. 2023-2023. All rights reserved.

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
// Package dynamic implements dynamic control mpam
package dynamic

import (
	"encoding/json"
	"fmt"

	"k8s-mpam-controller/pkg/util"
)

// ConfigPath is mpam config file path
const ConfigPath = "/var/lib/mpam-config/config.json"

/**
{
	"mpamConfig":{
		"adjustInterval": 1000,
		"perfDuration": 1000,
		"l3Percent": {
			"low": 20,
			"high": 50
		},
		"memBandPercent": {
			"low": 10,
			"high": 50
		}
		"cacheMiss": {
			minMiss: "10",
			"maxMiss": "20"
		}
	}
}
**/

// MPAMConfig describes how to control mpam, such as adjust interval,perf interval.
type MPAMConfig struct {
	AdjustInterval int        `json:"adjustInterval"`
	PerfDuration   int        `json:"perfDuration"`
	L3Percent      LevelRange `json:"l3Percent"`
	MemBandPercent LevelRange `json:"memBandPercent"`
	CacheMiss      CacheMiss  `json:"cacheMiss"`
}

// LevelRange defines a numerical range with inclusive lower and upper bounds.
type LevelRange struct {
	Low  int `json:"low"`
	High int `json:"high"`
}

// CacheMiss defines the min cachemiss ratio and the max cachemiss ratio for the online tasks.
type CacheMiss struct {
	MinMiss int `json:"minMiss"`
	MaxMiss int `json:"maxMiss"`
}

// ConfigWrapper serves as a wrapper for MPAM configuration.
type ConfigWrapper struct {
	MPAMConfig MPAMConfig `json:"mpamConfig"`
}

func getMPAMConfig(data []byte) (*MPAMConfig, error) {
	var config ConfigWrapper
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("unmarshal failed, err: %s", err)
	}

	return &config.MPAMConfig, nil
}

func getConfigData() ([]byte, error) {
	data, err := util.ReadFile(ConfigPath)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func existConfigFile() bool {
	return util.PathExist(ConfigPath)
}

func (c *MPAMConfig) validate() error {
	if !inRange(c.AdjustInterval, minAdjustInterval, maxAdjustInterval) {
		return fmt.Errorf(
			"adjustinterval: %d is invalid, should be in [%d, %d]",
			c.AdjustInterval, minAdjustInterval, maxAdjustInterval,
		)
	}

	if !inRange(c.PerfDuration, minPerfDur, maxPerfDur) {
		return fmt.Errorf(
			"perfDuration: %d is invalid, should be in [%d, %d]",
			c.PerfDuration, minPerfDur, maxPerfDur,
		)
	}

	if c.CacheMiss.MinMiss > c.CacheMiss.MaxMiss {
		return fmt.Errorf("minMiss should be less than maxMiss")
	}
	if !inRange(c.CacheMiss.MinMiss, minCacheMiss, maxCacheMiss) ||
		!inRange(c.CacheMiss.MaxMiss, minCacheMiss, maxCacheMiss) {
		return fmt.Errorf(
			"minMiss: %d or maxMiss: %d should be in [%d, %d]",
			c.CacheMiss.MinMiss, c.CacheMiss.MaxMiss, minCacheMiss, maxCacheMiss,
		)
	}

	if c.L3Percent.Low > c.L3Percent.High {
		return fmt.Errorf("l3Percent low should be less than high")
	}
	if !inRange(c.L3Percent.Low, minPercent, maxPercent) ||
		!inRange(c.L3Percent.High, minPercent, maxPercent) {
		return fmt.Errorf("l3Percent low:%d or high: %d should be in [%d, %d]",
			c.L3Percent.Low, c.L3Percent.High, minPercent, maxPercent,
		)
	}

	if c.MemBandPercent.Low > c.MemBandPercent.High {
		return fmt.Errorf("memBandPercent low should be less than high")
	}
	if !inRange(c.MemBandPercent.Low, minPercent, maxPercent) ||
		!inRange(c.MemBandPercent.High, minPercent, maxPercent) {
		return fmt.Errorf("memBandPercent low: %d or high: %d should be in [%d, %d]",
			c.MemBandPercent.Low, c.MemBandPercent.High, minPercent, maxPercent,
		)
	}

	return nil
}

func inRange(target int, start int, end int) bool {
	return target >= start && target <= end
}
