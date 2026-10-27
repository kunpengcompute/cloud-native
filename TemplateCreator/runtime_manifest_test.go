package templatecreator

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRuntimeManifestFixtureIsValidForSupportedArchitectures(t *testing.T) {
	// 在临时目录构造独立 manifest，避免正式 runtime 资产升级影响解析器测试。
	fixtureRoot := t.TempDir()
	bundledRoot := filepath.Join(fixtureRoot, "bundled")
	if err := os.MkdirAll(bundledRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	bundledContent := []byte("fixture envd executable\n")
	for _, name := range []string{"envd-amd64", "envd-arm64"} {
		if err := os.WriteFile(filepath.Join(bundledRoot, name), bundledContent, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	checksum := fmt.Sprintf("%x", sha256.Sum256(bundledContent))
	fixtureManifest := runtimeManifestFixtureJSON(checksum)
	manifestPath := filepath.Join(fixtureRoot, "manifest.json")
	if err := os.WriteFile(manifestPath, []byte(fixtureManifest), 0o644); err != nil {
		t.Fatal(err)
	}

	file, err := os.Open(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	manifest, err := parseRuntimeManifest(file)
	if err != nil {
		t.Fatalf("parseRuntimeManifest(fixture) error = %v", err)
	}
	for _, architecture := range []string{"amd64", "arm64"} {
		assets, err := manifest.selectArchitecture(architecture)
		if err != nil {
			t.Fatalf("selectArchitecture(%s) error = %v", architecture, err)
		}
		if len(assets) != len(requiredRuntimeAssetTypes) {
			t.Fatalf("selectArchitecture(%s) assets = %d, want %d", architecture, len(assets), len(requiredRuntimeAssetTypes))
		}
		for _, asset := range assets {
			if asset.Architecture != architecture {
				t.Fatalf("selected %s asset architecture = %q, want %q", asset.Type, asset.Architecture, architecture)
			}
		}
	}
	for _, asset := range manifest.Assets {
		if asset.BundledPath == "" {
			continue
		}
		content, err := os.ReadFile(filepath.Join(fixtureRoot, filepath.FromSlash(asset.BundledPath)))
		if err != nil {
			t.Fatalf("read bundled %s/%s: %v", asset.Type, asset.Architecture, err)
		}
		if got := fmt.Sprintf("%x", sha256.Sum256(content)); got != asset.SHA256 {
			t.Fatalf("bundled %s/%s sha256 = %s, want %s", asset.Type, asset.Architecture, got, asset.SHA256)
		}
	}
}

func TestRuntimeManifestParsesStrictSchema(t *testing.T) {
	manifest, err := parseRuntimeManifest(strings.NewReader(validRuntimeManifestJSON()))
	if err != nil {
		t.Fatalf("parseRuntimeManifest() error = %v", err)
	}
	if manifest.SchemaVersion != 1 {
		t.Fatalf("SchemaVersion = %d, want 1", manifest.SchemaVersion)
	}
	if manifest.TemplateVersion != "v2" {
		t.Fatalf("TemplateVersion = %q, want v2", manifest.TemplateVersion)
	}
	if len(manifest.Assets) != 8 {
		t.Fatalf("len(Assets) = %d, want 8", len(manifest.Assets))
	}
}

func TestRuntimeManifestRejectsUnknownField(t *testing.T) {
	raw := strings.Replace(validRuntimeManifestJSON(), `"schema_version": 1`, `"schema_version": 1, "unexpected": true`, 1)

	_, err := parseRuntimeManifest(strings.NewReader(raw))
	if err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("parseRuntimeManifest() error = %v, want unknown field error", err)
	}
}

func TestRuntimeManifestRejectsUnsupportedSchemaVersion(t *testing.T) {
	raw := strings.Replace(validRuntimeManifestJSON(), `"schema_version": 1`, `"schema_version": 2`, 1)

	_, err := parseRuntimeManifest(strings.NewReader(raw))
	if err == nil || !strings.Contains(err.Error(), "schema_version") {
		t.Fatalf("parseRuntimeManifest() error = %v, want schema_version error", err)
	}
}

func TestRuntimeManifestSelectsExactArchitectureInStableOrder(t *testing.T) {
	manifest, err := parseRuntimeManifest(strings.NewReader(validRuntimeManifestJSON()))
	if err != nil {
		t.Fatalf("parseRuntimeManifest() error = %v", err)
	}

	assets, err := manifest.selectArchitecture("amd64")
	if err != nil {
		t.Fatalf("selectArchitecture() error = %v", err)
	}
	wantTypes := []runtimeAssetType{assetBusybox, assetEnvd, assetFirecracker, assetKernel}
	if len(assets) != len(wantTypes) {
		t.Fatalf("len(assets) = %d, want %d", len(assets), len(wantTypes))
	}
	for i, want := range wantTypes {
		if assets[i].Type != want {
			t.Fatalf("assets[%d].Type = %q, want %q", i, assets[i].Type, want)
		}
		if assets[i].Architecture != "amd64" {
			t.Fatalf("assets[%d].Architecture = %q, want amd64", i, assets[i].Architecture)
		}
	}
}

func TestRuntimeManifestRejectsDuplicateAssetForArchitecture(t *testing.T) {
	raw := strings.Replace(
		validRuntimeManifestJSON(),
		`"type": "kernel", "version": "vmlinux-test", "architecture": "amd64"`,
		`"type": "envd", "version": "0.6.10", "architecture": "amd64"`,
		1,
	)
	raw = strings.Replace(
		raw,
		`"destination": "kernels/vmlinux-test/amd64/vmlinux.bin"`,
		`"destination": "envd/0.6.10/amd64/envd"`,
		1,
	)

	_, err := parseRuntimeManifest(strings.NewReader(raw))
	if err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("parseRuntimeManifest() error = %v, want duplicate error", err)
	}
}

func TestRuntimeManifestRejectsInvalidSHA256(t *testing.T) {
	raw := strings.Replace(validRuntimeManifestJSON(), strings.Repeat("a", 64), "not-a-sha256", 1)

	_, err := parseRuntimeManifest(strings.NewReader(raw))
	if err == nil || !strings.Contains(err.Error(), "sha256") {
		t.Fatalf("parseRuntimeManifest() error = %v, want sha256 error", err)
	}
}

func TestRuntimeManifestRejectsLatestURL(t *testing.T) {
	raw := strings.Replace(validRuntimeManifestJSON(), "/vmlinux-test/", "/latest/", 1)

	_, err := parseRuntimeManifest(strings.NewReader(raw))
	if err == nil || !strings.Contains(err.Error(), "latest") {
		t.Fatalf("parseRuntimeManifest() error = %v, want latest URL error", err)
	}
}

func TestRuntimeManifestRejectsURLQueryThatCouldContainSecrets(t *testing.T) {
	raw := strings.Replace(validRuntimeManifestJSON(), "/vmlinux.bin\"", "/vmlinux.bin?token=secret\"", 1)

	_, err := parseRuntimeManifest(strings.NewReader(raw))
	if err == nil || !strings.Contains(err.Error(), "query") {
		t.Fatalf("parseRuntimeManifest() error = %v, want URL query error", err)
	}
}

func TestRuntimeManifestRejectsUnsafeDestination(t *testing.T) {
	raw := strings.Replace(validRuntimeManifestJSON(), `"destination": "kernels/vmlinux-test/amd64/vmlinux.bin"`, `"destination": "../outside"`, 1)

	_, err := parseRuntimeManifest(strings.NewReader(raw))
	if err == nil || !strings.Contains(err.Error(), "destination") {
		t.Fatalf("parseRuntimeManifest() error = %v, want destination error", err)
	}
}

func TestRuntimeManifestRejectsUnsafeBundledPath(t *testing.T) {
	raw := strings.Replace(validRuntimeManifestJSON(), `"bundled_path": "bundled/envd-amd64"`, `"bundled_path": "../envd"`, 1)

	_, err := parseRuntimeManifest(strings.NewReader(raw))
	if err == nil || !strings.Contains(err.Error(), "bundled_path") {
		t.Fatalf("parseRuntimeManifest() error = %v, want bundled_path error", err)
	}
}

func TestRuntimeManifestRejectsBuilderIncompatibleDestinationLayout(t *testing.T) {
	raw := strings.Replace(
		validRuntimeManifestJSON(),
		`"destination": "busybox/1.36.1/amd64/busybox"`,
		`"destination": "bin/busybox"`,
		1,
	)

	_, err := parseRuntimeManifest(strings.NewReader(raw))
	if err == nil || !strings.Contains(err.Error(), "destination") {
		t.Fatalf("parseRuntimeManifest() error = %v, want Builder-compatible destination error", err)
	}
}

func TestRuntimeManifestRejectsUnknownAssetType(t *testing.T) {
	raw := strings.Replace(validRuntimeManifestJSON(), `"type": "kernel"`, `"type": "initrd"`, 1)

	_, err := parseRuntimeManifest(strings.NewReader(raw))
	if err == nil || !strings.Contains(err.Error(), "type") {
		t.Fatalf("parseRuntimeManifest() error = %v, want type error", err)
	}
}

func TestRuntimeManifestRequiresEveryAssetForSelectedArchitecture(t *testing.T) {
	raw := strings.Replace(
		validRuntimeManifestJSON(),
		`"type": "kernel", "version": "vmlinux-test", "architecture": "arm64"`,
		`"type": "kernel", "version": "vmlinux-test", "architecture": "ppc64le"`,
		1,
	)
	raw = strings.Replace(
		raw,
		`"destination": "kernels/vmlinux-test/arm64/vmlinux.bin"`,
		`"destination": "kernels/vmlinux-test/ppc64le/vmlinux.bin"`,
		1,
	)
	manifest, err := parseRuntimeManifest(strings.NewReader(raw))
	if err != nil {
		t.Fatalf("parseRuntimeManifest() error = %v", err)
	}

	_, err = manifest.selectArchitecture("arm64")
	if err == nil || !strings.Contains(err.Error(), "kernel") {
		t.Fatalf("selectArchitecture() error = %v, want missing kernel error", err)
	}
}

func validRuntimeManifestJSON() string {
	return runtimeManifestFixtureJSON(strings.Repeat("a", 64))
}

func runtimeManifestFixtureJSON(sha string) string {
	return `{
  "schema_version": 1,
  "infra_commit": "557445ffddda8d9a27f6f529a3f4d7732cf81a13",
  "template_version": "v2",
  "assets": [
    {"type": "envd", "version": "0.6.10", "architecture": "amd64", "url": "", "bundled_path": "bundled/envd-amd64", "reuse_paths": ["/fc-envd/envd"], "sha256": "` + sha + `", "destination": "envd/0.6.10/amd64/envd", "mode": "0755"},
    {"type": "kernel", "version": "vmlinux-test", "architecture": "amd64", "url": "https://example.test/kernels/vmlinux-test/amd64/vmlinux.bin", "reuse_paths": ["/fc-kernels/vmlinux-test/vmlinux.bin"], "sha256": "` + sha + `", "destination": "kernels/vmlinux-test/amd64/vmlinux.bin", "mode": "0644"},
    {"type": "firecracker", "version": "v1.14.1_test", "architecture": "amd64", "url": "https://example.test/firecrackers/v1.14.1_test/amd64/firecracker", "reuse_paths": ["/fc-versions/v1.14.1_test/firecracker"], "sha256": "` + sha + `", "destination": "firecrackers/v1.14.1_test/amd64/firecracker", "mode": "0755"},
    {"type": "busybox", "version": "1.36.1", "architecture": "amd64", "url": "https://example.test/busybox/1.36.1/amd64/busybox", "reuse_paths": [], "sha256": "` + sha + `", "destination": "busybox/1.36.1/amd64/busybox", "mode": "0755"},
    {"type": "busybox", "version": "1.36.1", "architecture": "arm64", "url": "https://example.test/busybox/1.36.1/arm64/busybox", "reuse_paths": [], "sha256": "` + sha + `", "destination": "busybox/1.36.1/arm64/busybox", "mode": "0755"},
    {"type": "firecracker", "version": "v1.14.1_test", "architecture": "arm64", "url": "https://example.test/firecrackers/v1.14.1_test/arm64/firecracker", "reuse_paths": [], "sha256": "` + sha + `", "destination": "firecrackers/v1.14.1_test/arm64/firecracker", "mode": "0755"},
    {"type": "envd", "version": "0.6.10", "architecture": "arm64", "url": "", "bundled_path": "bundled/envd-arm64", "reuse_paths": ["/fc-envd/envd"], "sha256": "` + sha + `", "destination": "envd/0.6.10/arm64/envd", "mode": "0755"},
    {"type": "kernel", "version": "vmlinux-test", "architecture": "arm64", "url": "https://example.test/kernels/vmlinux-test/arm64/vmlinux.bin", "reuse_paths": [], "sha256": "` + sha + `", "destination": "kernels/vmlinux-test/arm64/vmlinux.bin", "mode": "0644"}
  ]
}`
}
