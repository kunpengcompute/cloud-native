package main

import (
	"math"
	"os"
	"path/filepath"
	"testing"

	templatecreator "gitcode.com/boostkit/cloud-native.git/TemplateCreator"
)

func TestFreeNBDDevicesCountsIdleOnly(t *testing.T) {
	root := t.TempDir()
	for name, size := range map[string]string{"nbd0": "0\n", "nbd1": "16\n", "nbd2": "0\n", "sda": "0\n"} {
		path := filepath.Join(root, name)
		if err := os.Mkdir(path, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(path, "size"), []byte(size), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	count, err := freeNBDDevices(root)
	if err != nil || count != 2 {
		t.Fatalf("free NBD count=%d err=%v, want 2", count, err)
	}
}

func TestLargestSumSaturatesInsteadOfOverflowing(t *testing.T) {
	items := []batchItem{{Request: templatecreator.CreateRequest{BuildOverride: &templatecreator.BuildSpec{MemoryMB: math.MaxInt64}}}, {Request: templatecreator.CreateRequest{BuildOverride: &templatecreator.BuildSpec{MemoryMB: math.MaxInt64}}}}
	if got := largestSum(items, 2, func(spec templatecreator.BuildSpec) int64 { return spec.MemoryMB }); got != math.MaxInt64 {
		t.Fatalf("overflowed sum: %d", got)
	}
}

func TestDiskBudgetAccountsForSharedFilesystemAndWholeBatch(t *testing.T) {
	items := []batchItem{
		{Request: templatecreator.CreateRequest{BuildOverride: &templatecreator.BuildSpec{DiskSizeMB: 4096}}},
		{Request: templatecreator.CreateRequest{BuildOverride: &templatecreator.BuildSpec{DiskSizeMB: 4096}}},
		{Request: templatecreator.CreateRequest{BuildOverride: &templatecreator.BuildSpec{DiskSizeMB: 4096}}},
	}
	if err := checkDiskBudget(items, 2, 15000, 15000, true); err == nil {
		t.Fatal("shared 15 GiB filesystem accepted 8 GiB scratch peak plus 12 GiB Store batch")
	}
	if err := checkDiskBudget(items, 2, 9000, 13000, false); err != nil {
		t.Fatalf("separate filesystems with enough space rejected: %v", err)
	}
	if err := checkDiskBudget(items, 2, 9000, 10000, false); err == nil {
		t.Fatal("Store budget ignored the third completed Template")
	}
}
