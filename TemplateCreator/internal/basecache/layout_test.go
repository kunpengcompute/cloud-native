package basecache

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLayoutNamespaceIsStableAndSensitiveToEveryABIField(t *testing.T) {
	abi := testRuntimeABI()
	first, err := NewLayout(t.TempDir(), abi)
	if err != nil {
		t.Fatal(err)
	}
	second, err := NewLayout(first.Root, abi)
	if err != nil {
		t.Fatal(err)
	}
	if first.Namespace != second.Namespace || len(first.Namespace) != 64 {
		t.Fatalf("namespace is not stable SHA-256: %q != %q", first.Namespace, second.Namespace)
	}

	mutations := []struct {
		name   string
		mutate func(*RuntimeABI)
	}{
		{name: "infra commit", mutate: func(value *RuntimeABI) { value.InfraCommit += "-new" }},
		{name: "architecture", mutate: func(value *RuntimeABI) { value.Architecture = "arm64" }},
		{name: "template version", mutate: func(value *RuntimeABI) { value.TemplateVersion += "-new" }},
		{name: "kernel version", mutate: func(value *RuntimeABI) { value.KernelVersion += "-new" }},
		{name: "firecracker version", mutate: func(value *RuntimeABI) { value.FirecrackerVersion += "-new" }},
		{name: "envd version", mutate: func(value *RuntimeABI) { value.EnvdVersion += "-new" }},
		{name: "busybox version", mutate: func(value *RuntimeABI) { value.BusyboxVersion += "-new" }},
	}
	for _, mutation := range mutations {
		t.Run(mutation.name, func(t *testing.T) {
			changed := abi
			mutation.mutate(&changed)
			layout, err := NewLayout(first.Root, changed)
			if err != nil {
				t.Fatal(err)
			}
			if layout.Namespace == first.Namespace {
				t.Fatalf("namespace did not change after %s mutation", mutation.name)
			}
		})
	}
}

func TestLayoutUsesVersionedPersistentDirectories(t *testing.T) {
	root := t.TempDir()
	layout, err := NewLayout(root, testRuntimeABI())
	if err != nil {
		t.Fatal(err)
	}
	wantNamespaceRoot := filepath.Join(root, CacheSchemaVersion, layout.Namespace)
	if layout.NamespaceRoot != wantNamespaceRoot ||
		layout.TemplateRoot != filepath.Join(wantNamespaceRoot, "templates") ||
		layout.BuildIndexRoot != filepath.Join(wantNamespaceRoot, "build-index") ||
		layout.CorruptRoot != filepath.Join(wantNamespaceRoot, "corrupt") ||
		layout.LockPath != filepath.Join(wantNamespaceRoot, "cache.lock") ||
		layout.MetadataPath != filepath.Join(wantNamespaceRoot, "namespace.json") {
		t.Fatalf("layout = %#v", layout)
	}
	if layout.CacheScope != "templatecreator-base-v1-"+layout.Namespace {
		t.Fatalf("CacheScope = %q", layout.CacheScope)
	}
}

func TestPrepareCreatesLayoutAndRejectsMetadataMismatch(t *testing.T) {
	layout, err := NewLayout(t.TempDir(), testRuntimeABI())
	if err != nil {
		t.Fatal(err)
	}
	if err := Prepare(layout); err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}
	for _, path := range []string{layout.TemplateRoot, layout.BuildIndexRoot, layout.CorruptRoot} {
		info, err := os.Stat(path)
		if err != nil || !info.IsDir() {
			t.Fatalf("prepared directory %q: info=%v error=%v", path, info, err)
		}
	}
	metadata, err := os.ReadFile(layout.MetadataPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(metadata), layout.Namespace) || !strings.Contains(string(metadata), layout.ABI.EnvdVersion) {
		t.Fatalf("namespace metadata = %s", metadata)
	}

	inconsistent := layout
	inconsistent.ABI.EnvdVersion = "different-envd"
	if err := Prepare(inconsistent); err == nil || !strings.Contains(err.Error(), "metadata mismatch") {
		t.Fatalf("Prepare(inconsistent) error = %v, want metadata mismatch", err)
	}

	metadataAfter, err := os.ReadFile(layout.MetadataPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(metadataAfter) != string(metadata) {
		t.Fatal("Prepare() overwrote existing namespace metadata after mismatch")
	}
}

func TestLayoutRejectsRelativeRoot(t *testing.T) {
	if _, err := NewLayout("relative/cache", testRuntimeABI()); err == nil {
		t.Fatal("NewLayout() error = nil, want absolute root error")
	}
}

func testRuntimeABI() RuntimeABI {
	return RuntimeABI{
		InfraCommit:        "557445ffddda8d9a27f6f529a3f4d7732cf81a13",
		Architecture:       "amd64",
		TemplateVersion:    "v2",
		KernelVersion:      "vmlinux-test",
		FirecrackerVersion: "v1.14.1_test",
		EnvdVersion:        "0.6.10",
		BusyboxVersion:     "1.36.1",
	}
}
