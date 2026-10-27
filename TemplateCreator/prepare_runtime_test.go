package templatecreator

import (
	"context"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPrepareRuntimeInstallsArchitectureAndReturnsRuntimeConfig(t *testing.T) {
	contents := map[string][]byte{
		"busybox":     []byte("busybox-amd64"),
		"envd":        []byte("envd-amd64"),
		"firecracker": []byte("firecracker-amd64"),
		"kernel":      []byte("kernel-amd64"),
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		name := strings.TrimPrefix(request.URL.Path, "/")
		content, exists := contents[name]
		if !exists {
			http.NotFound(w, request)
			return
		}
		_, _ = w.Write(content)
	}))
	defer server.Close()

	manifestPath := writePrepareRuntimeManifest(t, server.URL, contents)
	runtimeRoot := filepath.Join(t.TempDir(), "runtime")
	result, err := PrepareRuntime(context.Background(), PrepareRuntimeRequest{
		ManifestPath: manifestPath,
		RuntimeRoot:  runtimeRoot,
		Architecture: "amd64",
		E2BInfraRoot: t.TempDir(),
		Timeout:      2 * time.Second,
	})
	if err != nil {
		t.Fatalf("PrepareRuntime() error = %v", err)
	}

	if len(result.Assets) != 4 {
		t.Fatalf("len(Assets) = %d, want 4", len(result.Assets))
	}
	if result.Runtime.InfraCommit != "557445ffddda8d9a27f6f529a3f4d7732cf81a13" || result.Runtime.TemplateVersion != "v2" {
		t.Fatalf("Runtime provenance = %#v", result.Runtime)
	}
	if result.Runtime.Architecture != "amd64" || result.Runtime.EnvdVersion != "0.6.10" {
		t.Fatalf("Runtime architecture/envd = %#v", result.Runtime)
	}
	if result.Runtime.KernelVersion != "vmlinux-test" || result.Runtime.FirecrackerVersion != "v1.14.1_test" || result.Runtime.BusyboxVersion != "1.36.1" {
		t.Fatalf("Runtime versions = %#v", result.Runtime)
	}
	if result.Runtime.EnvdPath != filepath.Join(runtimeRoot, "envd", "0.6.10", "amd64", "envd") {
		t.Fatalf("EnvdPath = %q", result.Runtime.EnvdPath)
	}
	if result.Runtime.BusyboxDir != filepath.Join(runtimeRoot, "busybox") {
		t.Fatalf("BusyboxDir = %q", result.Runtime.BusyboxDir)
	}
	for _, asset := range result.Assets {
		if _, err := os.Stat(asset.Path); err != nil {
			t.Fatalf("prepared asset %s stat error = %v", asset.Type, err)
		}
	}
}

func TestPrepareRuntimeRejectsUnavailableArchitectureBeforeDownload(t *testing.T) {
	contents := map[string][]byte{
		"busybox":     []byte("busybox-amd64"),
		"envd":        []byte("envd-amd64"),
		"firecracker": []byte("firecracker-amd64"),
		"kernel":      []byte("kernel-amd64"),
	}
	manifestPath := writePrepareRuntimeManifest(t, "https://example.test", contents)

	_, err := PrepareRuntime(context.Background(), PrepareRuntimeRequest{
		ManifestPath: manifestPath,
		RuntimeRoot:  t.TempDir(),
		Architecture: "arm64",
	})
	if err == nil || !strings.Contains(err.Error(), "arm64") {
		t.Fatalf("PrepareRuntime() error = %v, want unavailable arm64 error", err)
	}
}

func TestPrepareRuntimeRequiresAbsolutePathsAndPositiveTimeout(t *testing.T) {
	tests := []struct {
		name    string
		request PrepareRuntimeRequest
		want    string
	}{
		{name: "manifest", request: PrepareRuntimeRequest{RuntimeRoot: "/runtime"}, want: "ManifestPath"},
		{name: "runtime root", request: PrepareRuntimeRequest{ManifestPath: "/manifest.json", RuntimeRoot: "relative"}, want: "RuntimeRoot"},
		{name: "infra root", request: PrepareRuntimeRequest{ManifestPath: "/manifest.json", RuntimeRoot: "/runtime", E2BInfraRoot: "relative"}, want: "E2BInfraRoot"},
		{name: "timeout", request: PrepareRuntimeRequest{ManifestPath: "/manifest.json", RuntimeRoot: "/runtime", Timeout: -time.Second}, want: "Timeout"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := PrepareRuntime(context.Background(), test.request)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("PrepareRuntime() error = %v, want %s error", err, test.want)
			}
		})
	}
}

func writePrepareRuntimeManifest(t *testing.T, baseURL string, contents map[string][]byte) string {
	t.Helper()
	sha := func(name string) string {
		return fmt.Sprintf("%x", sha256.Sum256(contents[name]))
	}
	manifestDir := t.TempDir()
	bundledPath := filepath.Join(manifestDir, "bundled", "envd-amd64")
	if err := os.MkdirAll(filepath.Dir(bundledPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bundledPath, contents["envd"], 0o755); err != nil {
		t.Fatal(err)
	}
	raw := `{
  "schema_version": 1,
  "infra_commit": "557445ffddda8d9a27f6f529a3f4d7732cf81a13",
  "template_version": "v2",
  "assets": [
    {"type":"kernel","version":"vmlinux-test","architecture":"amd64","url":"` + baseURL + `/kernel","reuse_paths":[],"sha256":"` + sha("kernel") + `","destination":"kernels/vmlinux-test/amd64/vmlinux.bin","mode":"0644"},
    {"type":"firecracker","version":"v1.14.1_test","architecture":"amd64","url":"` + baseURL + `/firecracker","reuse_paths":[],"sha256":"` + sha("firecracker") + `","destination":"firecrackers/v1.14.1_test/amd64/firecracker","mode":"0755"},
    {"type":"envd","version":"0.6.10","architecture":"amd64","url":"","bundled_path":"bundled/envd-amd64","reuse_paths":[],"sha256":"` + sha("envd") + `","destination":"envd/0.6.10/amd64/envd","mode":"0755"},
    {"type":"busybox","version":"1.36.1","architecture":"amd64","url":"` + baseURL + `/busybox","reuse_paths":[],"sha256":"` + sha("busybox") + `","destination":"busybox/1.36.1/amd64/busybox","mode":"0755"}
  ]
}`
	path := filepath.Join(manifestDir, "manifest.json")
	if err := os.WriteFile(path, []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}

	return path
}
