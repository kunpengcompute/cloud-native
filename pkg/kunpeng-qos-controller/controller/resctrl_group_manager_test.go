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
	"os"
	"path/filepath"
	"testing"
)

func TestLocalResctrlGroupManagerEnsureGroup(t *testing.T) {
	root := t.TempDir()
	mgr := LocalResctrlGroupManager{RootDir: root, NumaIDs: []int{0}, CacheIDs: []int{0}}

	if err := mgr.EnsureGroup(context.Background(), "group-a"); err != nil {
		t.Fatalf("EnsureGroup() unexpected error: %v", err)
	}
	if err := mgr.EnsureGroup(context.Background(), "group-a"); err != nil {
		t.Fatalf("EnsureGroup() should be idempotent: %v", err)
	}

	if _, err := os.Stat(filepath.Join(root, "group-a")); err != nil {
		t.Fatalf("group dir should exist, got stat error: %v", err)
	}
}

func TestLocalResctrlGroupManagerDeleteGroup(t *testing.T) {
	root := t.TempDir()
	mgr := LocalResctrlGroupManager{RootDir: root, NumaIDs: []int{0}, CacheIDs: []int{0}}

	if err := mgr.EnsureGroup(context.Background(), "group-b"); err != nil {
		t.Fatalf("EnsureGroup() unexpected error: %v", err)
	}
	if err := mgr.DeleteGroup(context.Background(), "group-b"); err != nil {
		t.Fatalf("DeleteGroup() unexpected error: %v", err)
	}
	if err := mgr.DeleteGroup(context.Background(), "group-b"); err != nil {
		t.Fatalf("DeleteGroup() should be idempotent for missing dir: %v", err)
	}
}

func TestLocalResctrlGroupManagerApplyConfig(t *testing.T) {
	root := t.TempDir()
	var wrotePath []string
	var wroteItem []string
	mgr := LocalResctrlGroupManager{
		RootDir:  root,
		NumaIDs:  []int{1, 0},
		CacheIDs: []int{3, 1},
		SupportedSchemataKeys: []string{
			"MBHDL", "MBPRI", "L3PRI", "MBMIN", "L3MIN", "L3MAX", "MB", "L3",
		},
		WriteSchemataItemFunc: func(path string, item string) error {
			wrotePath = append(wrotePath, path)
			wroteItem = append(wroteItem, item)
			return nil
		},
	}

	cfg := ResctrlConfig{
		MBHDL: 1,
		MBPRI: 3,
		L3PRI: 0,
		MBMIN: 10,
		L3MIN: 20,
		L3MAX: 80,
		MB:    90,
		L3:    4,
	}
	if err := mgr.ApplyConfig(context.Background(), "group-c", cfg); err != nil {
		t.Fatalf("ApplyConfig() unexpected error: %v", err)
	}

	expectedPath := filepath.Join(root, "group-c", schemataFileName)
	expectedItems := []string{
		"MBHDL:1=1;0=1",
		"MBPRI:1=3;0=3",
		"L3PRI:3=0;1=0",
		"MBMIN:1=10;0=10",
		"L3MIN:3=20;1=20",
		"L3MAX:3=80;1=80",
		"MB:1=90;0=90",
		"L3:3=f;1=f",
	}

	if len(wroteItem) != len(expectedItems) {
		t.Fatalf("write call count mismatch: got %d want %d", len(wroteItem), len(expectedItems))
	}
	for i := range expectedItems {
		if wrotePath[i] != expectedPath {
			t.Fatalf("write path[%d] = %q, want %q", i, wrotePath[i], expectedPath)
		}
		if wroteItem[i] != expectedItems[i] {
			t.Fatalf("write item[%d] = %q, want %q", i, wroteItem[i], expectedItems[i])
		}
	}
}

func TestLocalResctrlGroupManagerApplyConfigInvalidWays(t *testing.T) {
	root := t.TempDir()
	mgr := LocalResctrlGroupManager{RootDir: root, NumaIDs: []int{0}, CacheIDs: []int{0}}

	cfg := ResctrlConfig{
		MBHDL: 1, MBPRI: 3, L3PRI: 0, MBMIN: 0, L3MIN: 0, L3MAX: 100, MB: 100, L3: 0,
	}
	if err := mgr.ApplyConfig(context.Background(), "group-d", cfg); err == nil {
		t.Fatalf("ApplyConfig() expected error for invalid L3 ways")
	}
}

func TestLocalResctrlGroupManagerApplyConfigMissingTopologyIDs(t *testing.T) {
	root := t.TempDir()
	mgr := LocalResctrlGroupManager{RootDir: root}

	cfg := ResctrlConfig{
		MBHDL: 1, MBPRI: 3, L3PRI: 0, MBMIN: 0, L3MIN: 0, L3MAX: 100, MB: 100, L3: 1,
	}
	if err := mgr.ApplyConfig(context.Background(), "group-e", cfg); err == nil {
		t.Fatalf("ApplyConfig() expected error when NUMAIDs/CacheIDs are not configured")
	}
}

func TestBuildSchemataItems(t *testing.T) {
	cfg := ResctrlConfig{
		MBHDL: 1,
		MBPRI: 3,
		L3PRI: 0,
		MBMIN: 10,
		L3MIN: 20,
		L3MAX: 80,
		MB:    90,
		L3:    4,
	}

	items, err := buildSchemataItems(
		cfg,
		[]int{1, 0},
		[]int{3, 1},
		makeSchemataKeySet([]string{"MBHDL", "MBPRI", "L3PRI", "MBMIN", "L3MIN", "L3MAX", "MB", "L3"}),
	)
	if err != nil {
		t.Fatalf("buildSchemataItems() unexpected error: %v", err)
	}

	expected := []string{
		"MBHDL:1=1;0=1",
		"MBPRI:1=3;0=3",
		"L3PRI:3=0;1=0",
		"MBMIN:1=10;0=10",
		"L3MIN:3=20;1=20",
		"L3MAX:3=80;1=80",
		"MB:1=90;0=90",
		"L3:3=f;1=f",
	}

	if len(items) != len(expected) {
		t.Fatalf("item count mismatch: got %d want %d", len(items), len(expected))
	}
	for i := range expected {
		if items[i] != expected[i] {
			t.Fatalf("items[%d] = %q, want %q", i, items[i], expected[i])
		}
	}
}

func TestBuildSchemataItemsSkipUnsupportedKeys(t *testing.T) {
	cfg := ResctrlConfig{
		MBHDL: 1,
		MBPRI: 3,
		L3PRI: 0,
		MBMIN: 10,
		L3MIN: 20,
		L3MAX: 80,
		MB:    90,
		L3:    4,
	}

	supported := makeSchemataKeySet([]string{"MBHDL", "MBPRI", "L3PRI", "MBMIN", "MB", "L3"})
	items, err := buildSchemataItems(cfg, []int{1, 0}, []int{3, 1}, supported)
	if err != nil {
		t.Fatalf("buildSchemataItems() unexpected error: %v", err)
	}

	expected := []string{
		"MBHDL:1=1;0=1",
		"MBPRI:1=3;0=3",
		"L3PRI:3=0;1=0",
		"MBMIN:1=10;0=10",
		"MB:1=90;0=90",
		"L3:3=f;1=f",
	}
	if len(items) != len(expected) {
		t.Fatalf("item count mismatch: got %d want %d", len(items), len(expected))
	}
	for i := range expected {
		if items[i] != expected[i] {
			t.Fatalf("items[%d] = %q, want %q", i, items[i], expected[i])
		}
	}
}
