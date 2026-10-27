//go:build linux

package runtime

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	upstreamstorage "gitcode.com/cloud-native/cloud-native/TemplateCreator/internal/upstream/shared/pkg/storage"
)

func TestPrepareInfra202609RuntimeViewMapsArchitectureAssets(t *testing.T) {
	runtimeRoot := t.TempDir()
	jobRoot := t.TempDir()
	kernelSource := filepath.Join(runtimeRoot, "kernels", "vmlinux-6.1.158", "amd64", "vmlinux.bin")
	fcSource := filepath.Join(runtimeRoot, "firecrackers", "v1.12.1_a41d3fb", "amd64", "firecracker")
	writeRuntimeFixture(t, kernelSource, "kernel", 0o644)
	writeRuntimeFixture(t, fcSource, "firecracker", 0o755)

	view, err := prepareInfra202609RuntimeView(jobRoot, runtimeRoot, "amd64", "vmlinux-6.1.158", "v1.12.1_a41d3fb")
	if err != nil {
		t.Fatalf("prepareInfra202609RuntimeView() error = %v", err)
	}
	assertSymlinkTarget(t, filepath.Join(view.KernelsDir, "vmlinux-6.1.158", "vmlinux.bin"), kernelSource)
	assertSymlinkTarget(t, filepath.Join(view.FirecrackerVersionsDir, "v1.12.1_a41d3fb", "firecracker"), fcSource)
}

func TestPrepareInfra202609RuntimeViewRejectsMissingAsset(t *testing.T) {
	_, err := prepareInfra202609RuntimeView(t.TempDir(), t.TempDir(), "amd64", "vmlinux-6.1.158", "v1.12.1_a41d3fb")
	if err == nil || !strings.Contains(err.Error(), "kernel") {
		t.Fatalf("prepareInfra202609RuntimeView() error = %v, want missing kernel error", err)
	}
}

func TestValidateInfra202609ProcessConfigRejectsSharedBaseCache(t *testing.T) {
	err := validateInfra202609BaseCacheEnabled(true)
	if err == nil || !strings.Contains(err.Error(), "infra 2026.09") {
		t.Fatalf("validateInfra202609BaseCacheEnabled() error = %v, want unsupported cache error", err)
	}
}

func TestInfra202609BusyboxInstallerUsesValidatedExternalBinary(t *testing.T) {
	source := filepath.Join(t.TempDir(), "busybox")
	content := []byte("arm64-busybox")
	writeRuntimeFixture(t, source, string(content), 0o755)
	expected := fmt.Sprintf("%x", sha256.Sum256(content))
	var destination []byte
	installer := infra202609BusyboxInstaller{}

	if err := installer.install(source, expected, &destination); err != nil {
		t.Fatalf("install() error = %v", err)
	}
	if string(destination) != string(content) {
		t.Fatalf("installed BusyBox = %q, want %q", destination, content)
	}
}

func TestInfra202609BusyboxInstallerRejectsProcessWideBinaryChange(t *testing.T) {
	installer := infra202609BusyboxInstaller{}
	var destination []byte
	for index, content := range [][]byte{[]byte("first"), []byte("second")} {
		source := filepath.Join(t.TempDir(), "busybox")
		writeRuntimeFixture(t, source, string(content), 0o755)
		expected := fmt.Sprintf("%x", sha256.Sum256(content))
		err := installer.install(source, expected, &destination)
		if index == 0 && err != nil {
			t.Fatalf("first install() error = %v", err)
		}
		if index == 1 && (err == nil || !strings.Contains(err.Error(), "already initialized")) {
			t.Fatalf("second install() error = %v, want process-wide mismatch", err)
		}
	}
}

func TestJobLocalStorageSupportsInfra202609BlobAndSeekableContracts(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	provider, err := newJobLocalStorage(root)
	if err != nil {
		t.Fatalf("newJobLocalStorage() error = %v", err)
	}

	blob, err := provider.OpenBlob(ctx, "build/metadata.json", upstreamstorage.MetadataObjectType)
	if err != nil {
		t.Fatal(err)
	}
	if err := blob.Put(ctx, []byte("metadata")); err != nil {
		t.Fatal(err)
	}
	var output strings.Builder
	if _, err := blob.WriteTo(ctx, &output); err != nil {
		t.Fatal(err)
	}
	if output.String() != "metadata" {
		t.Fatalf("blob content = %q, want metadata", output.String())
	}

	source := filepath.Join(t.TempDir(), "rootfs.ext4")
	writeRuntimeFixture(t, source, "rootfs", 0o644)
	seekable, err := provider.OpenSeekable(ctx, "build/rootfs.ext4", upstreamstorage.RootFSObjectType)
	if err != nil {
		t.Fatal(err)
	}
	if err := seekable.StoreFile(ctx, source); err != nil {
		t.Fatal(err)
	}
	buffer := make([]byte, len("rootfs"))
	if _, err := seekable.ReadAt(ctx, buffer, 0); err != nil {
		t.Fatal(err)
	}
	if string(buffer) != "rootfs" {
		t.Fatalf("seekable content = %q, want rootfs", buffer)
	}
}

func TestCopyJobLocalFileTruncatesExistingDestination(t *testing.T) {
	destination := filepath.Join(t.TempDir(), "rootfs.ext4")
	writeRuntimeFixture(t, destination, "stale-rootfs-content", 0o644)

	if err := copyJobLocalFile(destination, strings.NewReader("new")); err != nil {
		t.Fatalf("copyJobLocalFile() error = %v", err)
	}
	content, err := os.ReadFile(destination)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "new" {
		t.Fatalf("destination content = %q, want truncated content %q", content, "new")
	}
}

func TestJobLocalStorageRejectsEscapingObjectKey(t *testing.T) {
	provider, err := newJobLocalStorage(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := provider.OpenBlob(context.Background(), "../escape", upstreamstorage.MetadataObjectType); err == nil {
		t.Fatal("OpenBlob() error = nil, want unsafe key error")
	}
}

func writeRuntimeFixture(t *testing.T, path, content string, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		t.Fatal(err)
	}
}

func assertSymlinkTarget(t *testing.T, link, want string) {
	t.Helper()
	target, err := os.Readlink(link)
	if err != nil {
		t.Fatalf("Readlink(%s) error = %v", link, err)
	}
	if target != want {
		t.Fatalf("Readlink(%s) = %q, want %q", link, target, want)
	}
	content, err := os.ReadFile(link)
	if err != nil {
		t.Fatal(err)
	}
	if len(content) == 0 {
		t.Fatal(errors.New("symlink target is empty"))
	}
}
