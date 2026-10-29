package templatecreator

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"gitcode.com/boostkit/cloud-native.git/TemplateCreator/internal/publisher"
)

func TestExportTemplateBundleUsesArtifactLocation(t *testing.T) {
	storeRoot := t.TempDir()
	sourcePath := filepath.Join(t.TempDir(), "rootfs.ext4")
	if err := os.WriteFile(sourcePath, []byte("rootfs"), 0o644); err != nil {
		t.Fatal(err)
	}
	store, err := publisher.NewLocalStore("file://" + storeRoot)
	if err != nil {
		t.Fatal(err)
	}
	location, err := store.Publish(context.Background(), publisher.PublishRequest{
		Manifest: publisher.Manifest{
			SchemaVersion: publisher.ManifestSchemaVersion,
			TemplateID:    "tmpl-test", BuildID: "build-test", ImageRef: "example.com/image:tag",
			Metadata: json.RawMessage(`{"Architecture":"amd64"}`), InfraCommit: "locked-infra", BuiltAt: time.Unix(1, 0).UTC(),
		},
		Objects: []publisher.ObjectSource{{Key: "build-test/rootfs.ext4", Path: sourcePath}},
	})
	if err != nil {
		t.Fatal(err)
	}
	outputPath := filepath.Join(t.TempDir(), "bundle.tar.gz")

	result, err := ExportTemplateBundle(context.Background(), ExportBundleRequest{
		Artifacts:  ArtifactLocation{StoreURL: location.StoreURL, ManifestKey: location.ManifestKey},
		OutputPath: outputPath,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.BuildID != "build-test" || result.OutputPath != outputPath || result.SHA256 == "" {
		t.Fatalf("result = %#v", result)
	}
}
