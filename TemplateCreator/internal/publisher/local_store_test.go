package publisher

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNewLocalStoreRejectsNonFileURL(t *testing.T) {
	if _, err := NewLocalStore("s3://templates"); err == nil {
		t.Fatal("NewLocalStore() error = nil, want unsupported scheme error")
	}
}

func TestPublishPreservesObjectKeysAndRecordsChecksum(t *testing.T) {
	root := t.TempDir()
	store, err := NewLocalStore("file://" + root)
	if err != nil {
		t.Fatalf("NewLocalStore() error = %v", err)
	}

	sourcePath := filepath.Join(t.TempDir(), "rootfs.ext4")
	content := []byte("rootfs-content")
	if err := os.WriteFile(sourcePath, content, 0o600); err != nil {
		t.Fatal(err)
	}

	location, err := store.Publish(context.Background(), PublishRequest{
		Manifest: testManifest(),
		Objects: []ObjectSource{{
			Key:  "build-1/rootfs.ext4",
			Path: sourcePath,
		}},
	})
	if err != nil {
		t.Fatalf("Publish() error = %v", err)
	}

	published, err := os.ReadFile(filepath.Join(root, "build-1", "rootfs.ext4"))
	if err != nil {
		t.Fatalf("read published object: %v", err)
	}
	if string(published) != string(content) {
		t.Fatalf("published content = %q, want %q", published, content)
	}

	manifest := readManifest(t, filepath.Join(root, filepath.FromSlash(location.ManifestKey)))
	if len(manifest.Objects) != 1 {
		t.Fatalf("manifest objects = %d, want 1", len(manifest.Objects))
	}
	wantSum := sha256.Sum256(content)
	if got := manifest.Objects[0]; got.Key != "build-1/rootfs.ext4" || got.Size != int64(len(content)) || got.SHA256 != hex.EncodeToString(wantSum[:]) {
		t.Fatalf("object record = %#v, want key/size/checksum", got)
	}
}

func TestPublishRejectsPathTraversalKey(t *testing.T) {
	store, err := NewLocalStore("file://" + t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	sourcePath := filepath.Join(t.TempDir(), "object")
	if err := os.WriteFile(sourcePath, []byte("data"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err = store.Publish(context.Background(), PublishRequest{
		Manifest: testManifest(),
		Objects:  []ObjectSource{{Key: "../escape", Path: sourcePath}},
	})
	if err == nil || !strings.Contains(err.Error(), "unsafe object key") {
		t.Fatalf("Publish() error = %v, want unsafe object key", err)
	}
}

func TestPublishRejectsSymlinkEscape(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "build-1")); err != nil {
		t.Fatal(err)
	}
	store, err := NewLocalStore("file://" + root)
	if err != nil {
		t.Fatal(err)
	}
	sourcePath := filepath.Join(t.TempDir(), "object")
	if err := os.WriteFile(sourcePath, []byte("data"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err = store.Publish(context.Background(), PublishRequest{
		Manifest: testManifest(),
		Objects:  []ObjectSource{{Key: "build-1/rootfs.ext4", Path: sourcePath}},
	})
	if err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("Publish() error = %v, want symlink error", err)
	}
	if _, statErr := os.Stat(filepath.Join(outside, "rootfs.ext4")); !os.IsNotExist(statErr) {
		t.Fatalf("outside object stat error = %v, want not exists", statErr)
	}
}

func TestPublishLeavesNoManifestWhenObjectCopyFails(t *testing.T) {
	root := t.TempDir()
	store, err := NewLocalStore("file://" + root)
	if err != nil {
		t.Fatal(err)
	}
	sourcePath := filepath.Join(t.TempDir(), "object")
	if err := os.WriteFile(sourcePath, []byte("data"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err = store.Publish(context.Background(), PublishRequest{
		Manifest: testManifest(),
		Objects: []ObjectSource{
			{Key: "build-1/rootfs.ext4", Path: sourcePath},
			{Key: "build-1/memfile", Path: filepath.Join(t.TempDir(), "missing")},
		},
	})
	if err == nil {
		t.Fatal("Publish() error = nil, want object copy failure")
	}

	manifestPath := filepath.Join(root, "_templatecreator", "manifests", "template-1", "build-1.json")
	if _, statErr := os.Stat(manifestPath); !os.IsNotExist(statErr) {
		t.Fatalf("manifest stat error = %v, want not exists", statErr)
	}
}

func TestPublishAtomicallyReplacesManifest(t *testing.T) {
	root := t.TempDir()
	store, err := NewLocalStore("file://" + root)
	if err != nil {
		t.Fatal(err)
	}
	sourcePath := filepath.Join(t.TempDir(), "object")
	if err := os.WriteFile(sourcePath, []byte("data"), 0o600); err != nil {
		t.Fatal(err)
	}
	request := PublishRequest{
		Manifest: testManifest(),
		Objects:  []ObjectSource{{Key: "build-1/rootfs.ext4", Path: sourcePath}},
	}

	location, err := store.Publish(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	request.Manifest.Labels = map[string]string{"revision": "two"}
	if _, err := store.Publish(context.Background(), request); err != nil {
		t.Fatal(err)
	}

	manifest := readManifest(t, filepath.Join(root, filepath.FromSlash(location.ManifestKey)))
	if got := manifest.Labels["revision"]; got != "two" {
		t.Fatalf("manifest revision = %q, want two", got)
	}
	matches, err := filepath.Glob(filepath.Join(filepath.Dir(filepath.Join(root, filepath.FromSlash(location.ManifestKey))), ".manifest-*.tmp"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("temporary manifests = %v, want none", matches)
	}
}

func testManifest() Manifest {
	return Manifest{
		SchemaVersion:    ManifestSchemaVersion,
		TemplateID:       "template-1",
		BuildID:          "build-1",
		ImageRef:         "example.com/image@sha256:test",
		InfraCommit:      "557445ffddda8d9a27f6f529a3f4d7732cf81a13",
		BuiltAt:          time.Unix(1, 0).UTC(),
		Metadata:         json.RawMessage(`{"architecture":"amd64"}`),
		AncestorBuildIDs: []string{"ancestor-1"},
	}
}

func readManifest(t *testing.T, path string) Manifest {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatalf("decode manifest: %v", err)
	}

	return manifest
}
