package bundle

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"gitcode.com/boostkit/cloud-native.git/TemplateCreator/internal/publisher"
)

func TestExportIncludesManifestAndCompleteObjectClosure(t *testing.T) {
	storeRoot, manifestPath := writeBundleFixture(t)
	outputPath := filepath.Join(t.TempDir(), "template.tar.gz")

	result, err := Export(context.Background(), Request{
		StoreRoot: storeRoot, ManifestPath: manifestPath, OutputPath: outputPath,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.TemplateID != "tmpl-test" || result.BuildID != "build-test" || result.ObjectCount != 2 {
		t.Fatalf("result = %#v", result)
	}

	entries := readArchive(t, outputPath)
	for _, name := range []string{
		"template-bundle-tmpl-test-build-test/manifest.json",
		"template-bundle-tmpl-test-build-test/objects/ancestor-test/rootfs.ext4",
		"template-bundle-tmpl-test-build-test/objects/build-test/rootfs.ext4",
	} {
		if _, exists := entries[name]; !exists {
			t.Fatalf("archive missing %q; entries=%v", name, entries)
		}
	}
	manifestBytes, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(entries["template-bundle-tmpl-test-build-test/manifest.json"]) != string(manifestBytes) {
		t.Fatal("archive manifest differs from completion manifest")
	}
	checksum, err := os.ReadFile(result.SHA256Path)
	if err != nil {
		t.Fatal(err)
	}
	if len(checksum) == 0 || result.SHA256 == "" {
		t.Fatal("bundle checksum is empty")
	}
}

func TestExportArtifactsCreatesTemplateAndBuildDirectories(t *testing.T) {
	storeRoot, manifestPath := writeBundleFixture(t)
	outputRoot := filepath.Join(t.TempDir(), "exports")

	result, err := ExportArtifacts(context.Background(), Request{StoreRoot: storeRoot, ManifestPath: manifestPath, OutputRoot: outputRoot})
	if err != nil {
		t.Fatal(err)
	}
	if result.TemplateID != "tmpl-test" || result.BuildID != "build-test" || result.ObjectCount != 2 {
		t.Fatalf("result = %#v", result)
	}
	for _, relative := range []string{
		"tmpl-test/manifest.json",
		"tmpl-test/ancestor-test/rootfs.ext4",
		"tmpl-test/build-test/rootfs.ext4",
	} {
		if _, err := os.Stat(filepath.Join(outputRoot, relative)); err != nil {
			t.Fatalf("missing exported artifact %q: %v", relative, err)
		}
	}
	if result.OutputPath != filepath.Join(outputRoot, "tmpl-test") {
		t.Fatalf("OutputPath = %q", result.OutputPath)
	}
}

func TestExportRejectsUnsafeArchiveIdentity(t *testing.T) {
	storeRoot, manifestPath := writeBundleFixture(t)
	manifestBytes, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	var manifest publisher.Manifest
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		t.Fatal(err)
	}
	manifest.TemplateID = "../escape"
	updated, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manifestPath, updated, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Export(context.Background(), Request{
		StoreRoot: storeRoot, ManifestPath: manifestPath, OutputPath: filepath.Join(t.TempDir(), "template.tar.gz"),
	}); err == nil {
		t.Fatal("Export() error = nil, want unsafe archive identity")
	}
}

func TestExportRejectsObjectChecksumMismatchWithoutReplacingOutput(t *testing.T) {
	storeRoot, manifestPath := writeBundleFixture(t)
	if err := os.WriteFile(filepath.Join(storeRoot, "build-test", "rootfs.ext4"), []byte("corrupted"), 0o644); err != nil {
		t.Fatal(err)
	}
	outputPath := filepath.Join(t.TempDir(), "template.tar.gz")
	if err := os.WriteFile(outputPath, []byte("existing-output"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := Export(context.Background(), Request{
		StoreRoot: storeRoot, ManifestPath: manifestPath, OutputPath: outputPath,
	}); err == nil {
		t.Fatal("Export() error = nil, want checksum mismatch")
	}
	content, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "existing-output" {
		t.Fatalf("output content = %q, want original file", content)
	}
}

func writeBundleFixture(t *testing.T) (string, string) {
	t.Helper()
	storeRoot := t.TempDir()
	objects := map[string][]byte{
		"ancestor-test/rootfs.ext4": []byte("ancestor-rootfs"),
		"build-test/rootfs.ext4":    []byte("final-rootfs"),
	}
	records := make([]publisher.ObjectRecord, 0, len(objects))
	for key, content := range objects {
		path := filepath.Join(storeRoot, filepath.FromSlash(key))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, content, 0o644); err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(content)
		records = append(records, publisher.ObjectRecord{Key: key, Size: int64(len(content)), SHA256: hex.EncodeToString(sum[:])})
	}
	manifest := publisher.Manifest{
		SchemaVersion: publisher.ManifestSchemaVersion,
		TemplateID:    "tmpl-test", BuildID: "build-test", ImageRef: "example.com/image:tag",
		Objects: records, AncestorBuildIDs: []string{"ancestor-test"},
		Metadata:    json.RawMessage(`{"Architecture":"amd64","machine_info":{"cpu_architecture":"amd64","cpu_family":"6","cpu_model":"1","cpu_model_name":"test","cpu_flags":[]}}`),
		InfraCommit: "locked-infra", BuiltAt: time.Unix(1, 0).UTC(),
	}
	manifestBytes, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	manifestBytes = append(manifestBytes, '\n')
	manifestPath := filepath.Join(storeRoot, "_templatecreator", "manifests", "tmpl-test", "build-test.json")
	if err := os.MkdirAll(filepath.Dir(manifestPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manifestPath, manifestBytes, 0o644); err != nil {
		t.Fatal(err)
	}

	return storeRoot, manifestPath
}

func readArchive(t *testing.T, path string) map[string][]byte {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		t.Fatal(err)
	}
	defer gzipReader.Close()

	entries := make(map[string][]byte)
	reader := tar.NewReader(gzipReader)
	for {
		header, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		content, err := io.ReadAll(reader)
		if err != nil {
			t.Fatal(err)
		}
		entries[header.Name] = content
	}

	return entries
}
