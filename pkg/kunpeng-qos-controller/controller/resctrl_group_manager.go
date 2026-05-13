/*
Copyright (c) Huawei Technologies Co., Ltd. 2023-2026. All rights reserved.

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

package controller

import (
	"context"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"k8s.io/klog/v2"
	"kunpeng.huawei.com/kunpeng-cloud-computing/pkg/kunpeng-qos-controller/util"
)

const (
	defaultResctrlRoot = "/sys/fs/resctrl"
	schemataFileName   = "schemata"
)

// ResctrlConfig is the normalized config written to /sys/fs/resctrl by node agent.
type ResctrlConfig struct {
	MBHDL int32
	MBPRI int32
	L3PRI int32
	MBMIN int32
	L3MIN int32
	L3MAX int32
	MB    int32
	L3    int32 // Number of cache ways.
}

// ResctrlGroupManager performs local /sys/fs/resctrl operations.
type ResctrlGroupManager interface {
	EnsureGroup(ctx context.Context, groupName string) error
	ApplyConfig(ctx context.Context, groupName string, cfg ResctrlConfig) error
	DeleteGroup(ctx context.Context, groupName string) error
}

// LocalResctrlGroupManager is a local-node implementation for /sys/fs/resctrl.
type LocalResctrlGroupManager struct {
	RootDir string
	// NumaIDs and CacheIDs should be injected from discovered topology.
	// This manager does not assume NUMA and cache IDs are identical.
	NumaIDs  []int
	CacheIDs []int
	// WriteSchemataItemFunc allows tests to capture write operations.
	// If nil, default file writer will be used.
	WriteSchemataItemFunc func(path string, item string) error
	// SupportedSchemataKeys is injected from startup probing in main.go.
	// When empty, manager falls back to writing all known items.
	SupportedSchemataKeys []string
}

// NewLocalResctrlGroupManager discovers local resctrl topology from
// /sys/fs/resctrl/schemata and returns a ready-to-use manager.
func NewLocalResctrlGroupManager() (LocalResctrlGroupManager, error) {
	numaIDs, err := util.GetResctrlNUMAIDs()
	if err != nil {
		return LocalResctrlGroupManager{}, fmt.Errorf("discover resctrl NUMA IDs: %w", err)
	}
	cacheIDs, err := util.GetResctrlCacheIDs()
	if err != nil {
		return LocalResctrlGroupManager{}, fmt.Errorf("discover resctrl cache IDs: %w", err)
	}
	supportedKeys, err := util.GetResctrlSupportedSchemataKeys()
	if err != nil {
		return LocalResctrlGroupManager{}, fmt.Errorf("discover supported resctrl schemata keys: %w", err)
	}

	return LocalResctrlGroupManager{
		NumaIDs:               numaIDs,
		CacheIDs:              cacheIDs,
		SupportedSchemataKeys: supportedKeys,
	}, nil
}

// EnsureGroup ensures the control group directory exists.
func (m LocalResctrlGroupManager) EnsureGroup(_ context.Context, groupName string) error {
	groupPath := m.groupDir(groupName)
	if fi, err := os.Stat(groupPath); err == nil && fi.IsDir() {
		klog.V(4).Infof("resctrl group already exists: %s", groupPath)
		return nil
	}
	if err := os.MkdirAll(groupPath, 0o750); err != nil {
		return err
	}
	klog.V(1).Infof("created resctrl group: %s", groupPath)
	return nil
}

// ApplyConfig writes schemata item-by-item to avoid oversized one-shot writes.
func (m LocalResctrlGroupManager) ApplyConfig(ctx context.Context, groupName string, cfg ResctrlConfig) error {
	if err := m.EnsureGroup(ctx, groupName); err != nil {
		return err
	}

	items, err := buildSchemataItems(cfg, m.NumaIDs, m.CacheIDs, m.supportedSchemataKeySet())
	if err != nil {
		return err
	}

	schemataPath := filepath.Join(m.groupDir(groupName), schemataFileName)
	for _, item := range items {
		if err := m.writeSchemataItem(schemataPath, item); err != nil {
			return err
		}
	}
	return nil
}

// DeleteGroup removes a control group directory. Non-existent group is treated as success.
func (m LocalResctrlGroupManager) DeleteGroup(_ context.Context, groupName string) error {
	err := os.Remove(m.groupDir(groupName))
	if err == nil || os.IsNotExist(err) {
		return nil
	}
	return err
}

func (m LocalResctrlGroupManager) rootDir() string {
	if m.RootDir != "" {
		return m.RootDir
	}
	return defaultResctrlRoot
}

func (m LocalResctrlGroupManager) groupDir(groupName string) string {
	return filepath.Join(m.rootDir(), groupName)
}

func (m LocalResctrlGroupManager) writeSchemataItem(path string, item string) error {
	if m.WriteSchemataItemFunc != nil {
		return m.WriteSchemataItemFunc(path, item)
	}
	return writeSchemataItem(path, item)
}

func (m LocalResctrlGroupManager) supportedSchemataKeySet() map[string]struct{} {
	if len(m.SupportedSchemataKeys) > 0 {
		return makeSchemataKeySet(m.SupportedSchemataKeys)
	}
	return nil
}

func buildIDAssignment(ids []int, value string) (string, error) {
	if len(ids) == 0 {
		return "", fmt.Errorf("ids must not be empty")
	}

	pairs := make([]string, 0, len(ids))
	for _, id := range ids {
		pairs = append(pairs, fmt.Sprintf("%d=%s", id, value))
	}
	return strings.Join(pairs, ";"), nil
}

func buildSchemataItems(
	cfg ResctrlConfig,
	numaIDs []int,
	cacheIDs []int,
	supported map[string]struct{},
) ([]string, error) {
	mbhdlAssignment, err := buildIDAssignment(numaIDs, strconv.FormatInt(int64(cfg.MBHDL), 10))
	if err != nil {
		return nil, err
	}
	mbpriAssignment, err := buildIDAssignment(numaIDs, strconv.FormatInt(int64(cfg.MBPRI), 10))
	if err != nil {
		return nil, err
	}
	mbminAssignment, err := buildIDAssignment(numaIDs, strconv.FormatInt(int64(cfg.MBMIN), 10))
	if err != nil {
		return nil, err
	}
	mbAssignment, err := buildIDAssignment(numaIDs, strconv.FormatInt(int64(cfg.MB), 10))
	if err != nil {
		return nil, err
	}

	l3Mask, err := waysToHexMask(cfg.L3)
	if err != nil {
		return nil, err
	}
	l3priAssignment, err := buildIDAssignment(cacheIDs, strconv.FormatInt(int64(cfg.L3PRI), 10))
	if err != nil {
		return nil, err
	}
	l3minAssignment, err := buildIDAssignment(cacheIDs, strconv.FormatInt(int64(cfg.L3MIN), 10))
	if err != nil {
		return nil, err
	}
	l3maxAssignment, err := buildIDAssignment(cacheIDs, strconv.FormatInt(int64(cfg.L3MAX), 10))
	if err != nil {
		return nil, err
	}
	l3Assignment, err := buildIDAssignment(cacheIDs, l3Mask)
	if err != nil {
		return nil, err
	}

	allItems := []string{
		"MBHDL:" + mbhdlAssignment,
		"MBPRI:" + mbpriAssignment,
		"L3PRI:" + l3priAssignment,
		"MBMIN:" + mbminAssignment,
		"L3MIN:" + l3minAssignment,
		"L3MAX:" + l3maxAssignment,
		"MB:" + mbAssignment,
		"L3:" + l3Assignment,
	}
	// supported == nil means no filter information is available (unknown),
	// so keep backward-compatible behavior and write full item set.
	if supported == nil {
		return allItems, nil
	}
	// Different server models expose different schemata keys (for example,
	// some platforms do not provide L3MIN/L3MAX). Filter items by the
	// discovered key set to avoid write failures on unsupported fields.
	items := make([]string, 0, len(allItems))
	for _, item := range allItems {
		key, _, found := strings.Cut(item, ":")
		if !found {
			continue
		}
		if _, ok := supported[key]; !ok {
			klog.V(1).Infof("skip unsupported schemata item: %s", key)
			continue
		}
		items = append(items, item)
	}
	return items, nil
}

func writeSchemataItem(path string, item string) error {
	return os.WriteFile(path, []byte(item+"\n"), 0o600)
}

func waysToHexMask(ways int32) (string, error) {
	if ways < 1 {
		return "", fmt.Errorf("invalid l3 ways: %d, expected >= 1", ways)
	}

	// mask = (1 << ways) - 1, using big.Int to avoid fixed-width overflow.
	mask := new(big.Int).Lsh(big.NewInt(1), uint(ways))
	mask.Sub(mask, big.NewInt(1))
	return strings.ToLower(mask.Text(16)), nil
}

func makeSchemataKeySet(keys []string) map[string]struct{} {
	out := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		out[key] = struct{}{}
	}
	return out
}
