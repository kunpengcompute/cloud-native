package runtimeassets

import (
	"context"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestInstallerDownloadsAndAtomicallyInstallsAsset(t *testing.T) {
	content := []byte("runtime-asset")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(content)
	}))
	defer server.Close()

	root := t.TempDir()
	installer := Installer{Root: root, Client: server.Client()}
	result, err := installer.Install(context.Background(), []Asset{testAsset(server.URL, content)})
	if err != nil {
		t.Fatalf("Install() error = %v", err)
	}
	if len(result) != 1 || result[0].Source != SourceDownload {
		t.Fatalf("Install() result = %#v, want one downloaded asset", result)
	}
	assertInstalledFile(t, result[0].Path, content, 0o755)
	assertNoTemporaryFiles(t, filepath.Dir(result[0].Path))
}

func TestInstallerRejectsChecksumMismatchAndCleansTemporaryFile(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("wrong-content"))
	}))
	defer server.Close()

	root := t.TempDir()
	asset := testAsset(server.URL, []byte("expected-content"))
	_, err := (Installer{Root: root, Client: server.Client()}).Install(context.Background(), []Asset{asset})
	if err == nil || !strings.Contains(err.Error(), "checksum") {
		t.Fatalf("Install() error = %v, want checksum error", err)
	}
	if _, statErr := os.Stat(filepath.Join(root, filepath.FromSlash(asset.Destination))); !os.IsNotExist(statErr) {
		t.Fatalf("destination stat error = %v, want not exist", statErr)
	}
	assertNoTemporaryFiles(t, filepath.Join(root, "firecrackers", "v-test", "amd64"))
}

func TestInstallerHonorsContextCancellationAndCleansTemporaryFile(t *testing.T) {
	requestStarted := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(requestStarted)
		<-r.Context().Done()
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	root := t.TempDir()
	go func() {
		_, err := (Installer{Root: root, Client: server.Client()}).Install(ctx, []Asset{testAsset(server.URL, []byte("expected"))})
		done <- err
	}()
	<-requestStarted
	cancel()

	select {
	case err := <-done:
		if err == nil || !strings.Contains(err.Error(), context.Canceled.Error()) {
			t.Fatalf("Install() error = %v, want context canceled", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Install() did not return after context cancellation")
	}
	assertNoTemporaryFiles(t, filepath.Join(root, "firecrackers", "v-test", "amd64"))
}

func TestInstallerRejectsHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
	}))
	defer server.Close()

	_, err := (Installer{Root: t.TempDir(), Client: server.Client()}).Install(context.Background(), []Asset{testAsset(server.URL, []byte("expected"))})
	if err == nil || !strings.Contains(err.Error(), "503") {
		t.Fatalf("Install() error = %v, want HTTP 503 error", err)
	}
}

func TestInstallerReusesValidDestinationAndCorrectsMode(t *testing.T) {
	content := []byte("already-installed")
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		_, _ = w.Write(content)
	}))
	defer server.Close()

	root := t.TempDir()
	asset := testAsset(server.URL, content)
	destination := filepath.Join(root, filepath.FromSlash(asset.Destination))
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(destination, content, 0o600); err != nil {
		t.Fatal(err)
	}

	result, err := (Installer{Root: root, Client: server.Client()}).Install(context.Background(), []Asset{asset})
	if err != nil {
		t.Fatalf("Install() error = %v", err)
	}
	if requests.Load() != 0 {
		t.Fatalf("HTTP requests = %d, want 0", requests.Load())
	}
	if result[0].Source != SourceExisting {
		t.Fatalf("Source = %q, want %q", result[0].Source, SourceExisting)
	}
	assertInstalledFile(t, destination, content, 0o755)
}

func TestInstallerReusesE2BInfraAssetUnderSuppliedRoot(t *testing.T) {
	content := []byte("deployed-envd")
	infraRoot := t.TempDir()
	source := filepath.Join(infraRoot, "fc-envd", "envd")
	if err := os.MkdirAll(filepath.Dir(source), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(source, content, 0o755); err != nil {
		t.Fatal(err)
	}

	asset := Asset{
		Type:         "envd",
		Version:      "0.6.10",
		Architecture: "amd64",
		ReusePaths:   []string{"/fc-envd/envd"},
		SHA256:       checksum(content),
		Destination:  "envd/0.6.10/amd64/envd",
		Mode:         0o755,
	}
	result, err := (Installer{Root: t.TempDir(), InfraRoot: infraRoot}).Install(context.Background(), []Asset{asset})
	if err != nil {
		t.Fatalf("Install() error = %v", err)
	}
	if result[0].Source != SourceLocalReuse || result[0].SourcePath != source {
		t.Fatalf("Install() result = %#v, want reuse of %s", result[0], source)
	}
	assertInstalledFile(t, result[0].Path, content, 0o755)
}

func TestInstallerRejectsDestinationSymlinkEscape(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "firecrackers")); err != nil {
		t.Fatal(err)
	}

	asset := testAsset("https://example.test/asset", []byte("expected"))
	_, err := (Installer{Root: root}).Install(context.Background(), []Asset{asset})
	if err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("Install() error = %v, want symlink error", err)
	}
}

func testAsset(rawURL string, content []byte) Asset {
	return Asset{
		Type:         "firecracker",
		Version:      "v-test",
		Architecture: "amd64",
		URL:          rawURL,
		SHA256:       checksum(content),
		Destination:  "firecrackers/v-test/amd64/firecracker",
		Mode:         0o755,
	}
}

func checksum(content []byte) string {
	return fmt.Sprintf("%x", sha256.Sum256(content))
}

func assertInstalledFile(t *testing.T, path string, wantContent []byte, wantMode os.FileMode) {
	t.Helper()
	gotContent, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", path, err)
	}
	if string(gotContent) != string(wantContent) {
		t.Fatalf("ReadFile(%q) = %q, want %q", path, gotContent, wantContent)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != wantMode {
		t.Fatalf("mode = %o, want %o", info.Mode().Perm(), wantMode)
	}
}

func assertNoTemporaryFiles(t *testing.T, directory string) {
	t.Helper()
	entries, err := os.ReadDir(directory)
	if os.IsNotExist(err) {
		return
	}
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.Contains(entry.Name(), ".partial-") {
			t.Fatalf("temporary file remains: %s", filepath.Join(directory, entry.Name()))
		}
	}
}
