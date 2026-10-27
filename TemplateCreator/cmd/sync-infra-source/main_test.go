package main

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestLockedInfraCommitMatches202609(t *testing.T) {
	const want = "4853232b93a8871b4ded73079184e801d84e0e26"
	if lockedInfraCommit != want {
		t.Fatalf("lockedInfraCommit = %q, want infra 2026.09 commit %q", lockedInfraCommit, want)
	}
}

func TestParsePackageListReturnsSortedSafePaths(t *testing.T) {
	raw := `
# production closure
packages/shared/pkg/zeta
packages/orchestrator/pkg/alpha
packages/clickhouse/pkg/batcher
`

	got, err := parsePackageList(strings.NewReader(raw))
	if err != nil {
		t.Fatalf("parsePackageList() error = %v", err)
	}
	want := []string{
		"packages/clickhouse/pkg/batcher",
		"packages/orchestrator/pkg/alpha",
		"packages/shared/pkg/zeta",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parsePackageList() = %#v, want %#v", got, want)
	}
}

func TestParsePackageListRejectsUnsafeDuplicateAndUnknownPaths(t *testing.T) {
	tests := []struct {
		name string
		raw  string
	}{
		{name: "absolute", raw: "/packages/shared/pkg/x"},
		{name: "traversal", raw: "packages/shared/pkg/../x"},
		{name: "backslash", raw: `packages\shared\pkg\x`},
		{name: "unknown module", raw: "packages/api/pkg/x"},
		{name: "duplicate", raw: "packages/shared/pkg/x\npackages/shared/pkg/x"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := parsePackageList(strings.NewReader(test.raw)); err == nil {
				t.Fatal("parsePackageList() error = nil, want validation error")
			}
		})
	}
}

func TestRewriteGoSourcePreservesBuildTagsAndEmbedDirective(t *testing.T) {
	source := []byte(`//go:build linux
// +build linux

// This package keeps Linux fixtures.
package fixture

import (
	"embed"
	"github.com/e2b-dev/infra/packages/shared/pkg/storage"
)

//go:embed assets/*.txt
var assets embed.FS

var _ = storage.Paths{}
`)

	got, modified := rewriteGoSource(source)
	if !modified {
		t.Fatal("rewriteGoSource() modified = false, want true")
	}
	if !bytes.HasPrefix(got, []byte("//go:build linux\n// +build linux\n\n")) {
		t.Fatalf("rewriteGoSource() did not preserve build tag header:\n%s", got)
	}
	if !bytes.Contains(got, []byte(templateCreatorModificationNotice)) {
		t.Fatalf("rewriteGoSource() missing modification notice:\n%s", got)
	}
	if !bytes.Contains(got, []byte(`gitcode.com/cloud-native/cloud-native/TemplateCreator/internal/upstream/shared/pkg/storage`)) {
		t.Fatalf("rewriteGoSource() did not rewrite import:\n%s", got)
	}
	if !bytes.Contains(got, []byte("//go:embed assets/*.txt")) {
		t.Fatalf("rewriteGoSource() lost go:embed directive:\n%s", got)
	}
	if !bytes.Contains(got, []byte("package fixture")) {
		t.Fatalf("rewriteGoSource() corrupted package declaration:\n%s", got)
	}
	if !bytes.Contains(got, []byte("// This package keeps Linux fixtures.")) {
		t.Fatalf("rewriteGoSource() corrupted leading comment:\n%s", got)
	}
}

func TestSyncPackagesRemovesStaleFilesAndIsDeterministic(t *testing.T) {
	infraRoot := t.TempDir()
	packageDir := filepath.Join(infraRoot, "packages", "shared", "pkg", "fixture")
	writeFixtureFile(t, filepath.Join(packageDir, "fixture_linux.go"), []byte(`//go:build linux

package fixture

import "github.com/e2b-dev/infra/packages/shared/pkg/storage"

//go:embed assets/message.txt
var message string

var _ = storage.Paths{}
`), 0o644)
	writeFixtureFile(t, filepath.Join(packageDir, "fixture_other.go"), []byte("//go:build !linux\n\npackage fixture\n"), 0o644)
	writeFixtureFile(t, filepath.Join(packageDir, "fixture_test.go"), []byte("package fixture\n"), 0o644)
	writeFixtureFile(t, filepath.Join(packageDir, "assets", "message.txt"), []byte("github.com/e2b-dev/infra/packages/shared/pkg/storage\n"), 0o644)
	writeFixtureFile(t, filepath.Join(packageDir, "unlisted", "child.go"), []byte("package unlisted\n"), 0o644)
	initFixtureRepository(t, infraRoot)
	writeFixtureFile(t, filepath.Join(packageDir, "untracked.go"), []byte("package fixture\n"), 0o644)

	targetRoot := filepath.Join(t.TempDir(), "upstream")
	writeFixtureFile(t, filepath.Join(targetRoot, "stale.txt"), []byte("stale"), 0o644)
	config := syncConfig{
		InfraRoot:    infraRoot,
		TargetRoot:   targetRoot,
		Commit:       strings.Repeat("a", 40),
		PackagePaths: []string{"packages/shared/pkg/fixture"},
	}

	if err := syncPackages(config); err != nil {
		t.Fatalf("syncPackages() error = %v", err)
	}
	first := hashTree(t, targetRoot)
	if _, err := os.Stat(filepath.Join(targetRoot, "stale.txt")); !os.IsNotExist(err) {
		t.Fatalf("stale file stat error = %v, want not exist", err)
	}
	if _, err := os.Stat(filepath.Join(targetRoot, "shared", "pkg", "fixture", "fixture_test.go")); !os.IsNotExist(err) {
		t.Fatalf("test file stat error = %v, want not exist", err)
	}
	if _, err := os.Stat(filepath.Join(targetRoot, "shared", "pkg", "fixture", "unlisted", "child.go")); !os.IsNotExist(err) {
		t.Fatalf("nested package file stat error = %v, want not exist", err)
	}
	if _, err := os.Stat(filepath.Join(targetRoot, "shared", "pkg", "fixture", "untracked.go")); !os.IsNotExist(err) {
		t.Fatalf("untracked file stat error = %v, want not exist", err)
	}
	for _, relative := range []string{
		"shared/pkg/fixture/fixture_linux.go",
		"shared/pkg/fixture/fixture_other.go",
		"shared/pkg/fixture/assets/message.txt",
		"SOURCE_MANIFEST.json",
	} {
		if _, err := os.Stat(filepath.Join(targetRoot, filepath.FromSlash(relative))); err != nil {
			t.Fatalf("expected synced file %s: %v", relative, err)
		}
	}
	assetContent, err := os.ReadFile(filepath.Join(targetRoot, "shared", "pkg", "fixture", "assets", "message.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(assetContent), "github.com/e2b-dev/infra") {
		t.Fatalf("non-Go import reference was not rewritten: %s", assetContent)
	}

	if err := syncPackages(config); err != nil {
		t.Fatalf("second syncPackages() error = %v", err)
	}
	second := hashTree(t, targetRoot)
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("sync output changed between runs:\nfirst=%v\nsecond=%v", first, second)
	}
}

func TestSyncPackagesMapsInfra202609Namespaces(t *testing.T) {
	infraRoot := t.TempDir()
	orchestratorPackage := filepath.Join(infraRoot, "packages", "orchestrator", "internal", "fixture")
	writeFixtureFile(t, filepath.Join(orchestratorPackage, "fixture.go"), []byte(`package fixture

import featureflags "github.com/e2b-dev/infra/packages/shared/pkg/feature-flags"

var _ *featureflags.Client
`), 0o644)
	featureFlagsPackage := filepath.Join(infraRoot, "packages", "shared", "pkg", "feature-flags")
	writeFixtureFile(t, filepath.Join(featureFlagsPackage, "client.go"), []byte("package featureflags\n\ntype Client struct{}\n"), 0o644)
	initFixtureRepository(t, infraRoot)

	targetRoot := filepath.Join(t.TempDir(), "upstream")
	config := syncConfig{
		InfraRoot:  infraRoot,
		TargetRoot: targetRoot,
		Commit:     strings.Repeat("a", 40),
		PackagePaths: []string{
			"packages/orchestrator/internal/fixture",
			"packages/shared/pkg/feature-flags",
		},
	}
	if err := syncPackages(config); err != nil {
		t.Fatalf("syncPackages() error = %v", err)
	}

	orchestratorTarget := filepath.Join(targetRoot, "orchestrator", "pkg", "fixture", "fixture.go")
	content, err := os.ReadFile(orchestratorTarget)
	if err != nil {
		t.Fatalf("read promoted orchestrator package: %v", err)
	}
	if !bytes.Contains(content, []byte(`gitcode.com/cloud-native/cloud-native/TemplateCreator/internal/upstream/shared/pkg/featureflags`)) {
		t.Fatalf("2026.09 feature-flags import was not mapped to the stable private namespace:\n%s", content)
	}
	if _, err := os.Stat(filepath.Join(targetRoot, "shared", "pkg", "featureflags", "client.go")); err != nil {
		t.Fatalf("feature-flags package was not mapped to featureflags: %v", err)
	}
	if _, err := os.Stat(filepath.Join(targetRoot, "orchestrator", "internal")); !os.IsNotExist(err) {
		t.Fatalf("orchestrator/internal leaked into generated snapshot: %v", err)
	}
	manifestContent, err := os.ReadFile(filepath.Join(targetRoot, "SOURCE_MANIFEST.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(manifestContent, []byte(`"source_path": "orchestrator/internal/fixture/fixture.go"`)) ||
		!bytes.Contains(manifestContent, []byte(`"modified": true`)) {
		t.Fatalf("manifest does not audit namespace mapping:\n%s", manifestContent)
	}
}

func TestVerifyInfraCommitRejectsWrongCommitAndDirtyTree(t *testing.T) {
	repository := t.TempDir()
	runGit(t, repository, "init", "-b", "main")
	runGit(t, repository, "config", "user.email", "fixture@example.test")
	runGit(t, repository, "config", "user.name", "Fixture")
	tracked := filepath.Join(repository, "tracked.txt")
	writeFixtureFile(t, tracked, []byte("clean\n"), 0o644)
	runGit(t, repository, "add", "tracked.txt")
	runGit(t, repository, "commit", "-m", "fixture")
	commit := strings.TrimSpace(runGit(t, repository, "rev-parse", "HEAD"))

	if err := verifyInfraCommit(repository, commit); err != nil {
		t.Fatalf("verifyInfraCommit(clean) error = %v", err)
	}
	if err := verifyInfraCommit(repository, strings.Repeat("0", 40)); err == nil || !strings.Contains(err.Error(), "commit") {
		t.Fatalf("verifyInfraCommit(wrong) error = %v, want commit error", err)
	}
	writeFixtureFile(t, tracked, []byte("dirty\n"), 0o644)
	if err := verifyInfraCommit(repository, commit); err == nil || !strings.Contains(err.Error(), "dirty") {
		t.Fatalf("verifyInfraCommit(dirty) error = %v, want dirty error", err)
	}
}

func TestDeduplicateManifestFilesRejectsConflictingContent(t *testing.T) {
	files := []sourceManifestFile{
		{Path: "shared/pkg/a/data.txt", SHA256: "same", Modified: true},
		{Path: "shared/pkg/a/data.txt", SHA256: "same", Modified: true},
		{Path: "shared/pkg/b/data.txt", SHA256: "other"},
	}
	got, err := deduplicateManifestFiles(files)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("deduplicated length = %d, want 2", len(got))
	}
	files[1].SHA256 = "conflict"
	if _, err := deduplicateManifestFiles(files); err == nil || !strings.Contains(err.Error(), "conflicting") {
		t.Fatalf("conflicting duplicate error = %v", err)
	}
}

func writeFixtureFile(t *testing.T, path string, content []byte, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, content, mode); err != nil {
		t.Fatal(err)
	}
}

func initFixtureRepository(t *testing.T, root string) {
	t.Helper()
	runGit(t, root, "init", "-b", "main")
	runGit(t, root, "config", "user.email", "fixture@example.test")
	runGit(t, root, "config", "user.name", "Fixture")
	runGit(t, root, "add", ".")
	runGit(t, root, "commit", "-m", "fixture")
}

func hashTree(t *testing.T, root string) map[string]string {
	t.Helper()
	result := make(map[string]string)
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		result[filepath.ToSlash(relative)] = fmt.Sprintf("%x", sha256.Sum256(content))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	return result
}

func runGit(t *testing.T, directory string, arguments ...string) string {
	t.Helper()
	command := exec.Command("git", arguments...)
	command.Dir = directory
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v error = %v: %s", arguments, err, output)
	}

	return string(output)
}
