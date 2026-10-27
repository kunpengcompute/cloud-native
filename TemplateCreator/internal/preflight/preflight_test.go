package preflight

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestRunAcceptsCompleteEnvironment(t *testing.T) {
	config := completeConfig(t)
	report := Run(config)
	if !report.Ready() {
		t.Fatalf("Run() report = %#v, want ready", report)
	}
	if len(report.Checks) != 14 {
		t.Fatalf("len(Checks) = %d, want 14", len(report.Checks))
	}
}

func TestRunChecksPersistentCacheDirectoryAndIsolation(t *testing.T) {
	config := completeConfig(t)
	config.CacheRoot = filepath.Join(t.TempDir(), "cache")
	if err := os.Mkdir(config.CacheRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	report := Run(config)
	check := findCheck(t, report, "cache_directory")
	if !check.OK {
		t.Fatalf("cache_directory check = %#v", check)
	}

	config.CacheRoot = config.ScratchRoot
	assertFailedCheck(t, Run(config), "cache_directory", "must not overlap")
}

func TestRunAllowsDisabledPersistentCacheWithoutCacheDirectory(t *testing.T) {
	config := completeConfig(t)
	config.CacheRoot = ""
	config.ScratchRoot = filepath.Join(t.TempDir(), "scratch")
	report := Run(config)
	check := findCheck(t, report, "cache_directory")
	if !check.OK || !strings.Contains(check.Detail, "disabled") {
		t.Fatalf("cache_directory check = %#v", check)
	}
}

func TestRunRejectsNonRootProcessBeforePrivilegedBuild(t *testing.T) {
	config := completeConfig(t)
	uid := 1000
	config.EffectiveUID = &uid
	assertFailedCheck(t, Run(config), "privileges", "root")
}

func TestRunRejectsNonLinuxHost(t *testing.T) {
	config := completeConfig(t)
	config.GOOS = "darwin"
	assertFailedCheck(t, Run(config), "host_os", "Linux")
}

func TestRunRejectsArchitectureMismatch(t *testing.T) {
	config := completeConfig(t)
	config.GOARCH = "arm64"
	config.Architecture = "amd64"
	assertFailedCheck(t, Run(config), "architecture", "does not match")
}

func TestRunRejectsUnavailableKVM(t *testing.T) {
	config := completeConfig(t)
	config.KVMPath = filepath.Join(t.TempDir(), "missing-kvm")
	assertFailedCheck(t, Run(config), "kvm", "unavailable")
}

func TestRunRejectsUnavailableNBD(t *testing.T) {
	config := completeConfig(t)
	if err := os.WriteFile(config.NBDMaxPath, []byte("0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	assertFailedCheck(t, Run(config), "nbd", "positive")
}

func TestRunRejectsInsufficientHugePagesWhenEnabled(t *testing.T) {
	config := completeConfig(t)
	config.HugePages = true
	config.MemoryMB = 1024
	if err := os.WriteFile(config.MemInfoPath, []byte("HugePages_Free: 1\nHugepagesize: 2048 kB\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	assertFailedCheck(t, Run(config), "hugepages", "requires")
}

func TestRunSkipsHugePageCapacityWhenDisabled(t *testing.T) {
	config := completeConfig(t)
	config.HugePages = false
	config.MemInfoPath = filepath.Join(t.TempDir(), "missing-meminfo")
	report := Run(config)
	check := findCheck(t, report, "hugepages")
	if !check.OK || !strings.Contains(check.Detail, "disabled") {
		t.Fatalf("hugepages check = %#v", check)
	}
}

func TestRunRejectsMissingRuntimeFile(t *testing.T) {
	config := completeConfig(t)
	if err := os.Remove(config.EnvdPath); err != nil {
		t.Fatal(err)
	}
	assertFailedCheck(t, Run(config), "runtime_envd", "missing")
}

func TestRunRejectsNonExecutableRuntimeBinary(t *testing.T) {
	config := completeConfig(t)
	if err := os.Chmod(config.FirecrackerPath, 0o644); err != nil {
		t.Fatal(err)
	}
	assertFailedCheck(t, Run(config), "runtime_firecracker", "executable")
}

func TestRunRejectsUnwritableDirectory(t *testing.T) {
	config := completeConfig(t)
	if err := os.Chmod(config.StoreRoot, 0o555); err != nil {
		t.Fatal(err)
	}
	assertFailedCheck(t, Run(config), "store_directory", "writable")
}

func TestRunRejectsSymlinkDirectory(t *testing.T) {
	config := completeConfig(t)
	link := filepath.Join(t.TempDir(), "store-link")
	if err := os.Symlink(config.StoreRoot, link); err != nil {
		t.Fatal(err)
	}
	config.StoreRoot = link
	assertFailedCheck(t, Run(config), "store_directory", "symbolic link")
}

func TestRunRejectsRuntimeChecksumMismatch(t *testing.T) {
	config := completeConfig(t)
	config.EnvdSHA256 = strings.Repeat("0", 64)
	assertFailedCheck(t, Run(config), "runtime_envd", "checksum mismatch")
}

func TestRunRejectsInvalidImageReference(t *testing.T) {
	config := completeConfig(t)
	config.ImageRef = "UPPER CASE/invalid image"
	assertFailedCheck(t, Run(config), "image_reference", "invalid")
}

func completeConfig(t *testing.T) Config {
	t.Helper()
	root := t.TempDir()
	writeFile := func(relative string, mode os.FileMode, content string) string {
		t.Helper()
		path := filepath.Join(root, relative)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), mode); err != nil {
			t.Fatal(err)
		}
		return path
	}
	rootUID := 0
	config := Config{
		GOOS:            "linux",
		GOARCH:          runtime.GOARCH,
		Architecture:    runtime.GOARCH,
		ImageRef:        "docker.io/library/alpine:3.20",
		ScratchRoot:     filepath.Join(root, "scratch"),
		StoreRoot:       filepath.Join(root, "store"),
		KVMPath:         writeFile("dev/kvm", 0o660, ""),
		NBDMaxPath:      writeFile("sys/module/nbd/parameters/nbds_max", 0o644, "8\n"),
		NBDSystemRoot:   filepath.Join(root, "sys/block"),
		MemInfoPath:     writeFile("proc/meminfo", 0o644, "HugePages_Free: 1024\nHugepagesize: 2048 kB\n"),
		KernelPath:      writeFile("runtime/kernel", 0o644, "kernel"),
		FirecrackerPath: writeFile("runtime/firecracker", 0o755, "firecracker"),
		EnvdPath:        writeFile("runtime/envd", 0o755, "envd"),
		BusyboxPath:     writeFile("runtime/busybox", 0o755, "busybox"),
		MemoryMB:        512,
		HugePages:       true,
		EffectiveUID:    &rootUID,
	}
	config.KernelSHA256 = fileSHA256(t, config.KernelPath)
	config.FirecrackerSHA256 = fileSHA256(t, config.FirecrackerPath)
	config.EnvdSHA256 = fileSHA256(t, config.EnvdPath)
	config.BusyboxSHA256 = fileSHA256(t, config.BusyboxPath)
	for _, directory := range []string{config.ScratchRoot, config.StoreRoot} {
		if err := os.MkdirAll(directory, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(filepath.Join(config.NBDSystemRoot, "nbd0"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(config.NBDSystemRoot, "nbd0", "size"), []byte("0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return config
}

func fileSHA256(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return fmt.Sprintf("%x", sha256.Sum256(content))
}

func assertFailedCheck(t *testing.T, report Report, name, detail string) {
	t.Helper()
	check := findCheck(t, report, name)
	if check.OK || !strings.Contains(check.Detail, detail) {
		t.Fatalf("check %q = %#v, want failure containing %q", name, check, detail)
	}
}

func findCheck(t *testing.T, report Report, name string) Check {
	t.Helper()
	for _, check := range report.Checks {
		if check.Name == name {
			return check
		}
	}
	t.Fatalf("check %q not found in %#v", name, report)
	return Check{}
}
