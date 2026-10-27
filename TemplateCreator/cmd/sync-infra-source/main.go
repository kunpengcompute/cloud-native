package main

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

const (
	lockedInfraCommit                 = "4853232b93a8871b4ded73079184e801d84e0e26"
	templateCreatorModificationNotice = "// TemplateCreator modification: upstream import paths rewritten; no behavioral changes."
)

var importRewrites = []struct{ old, new string }{
	// infra 2026.09 的构建执行面仍位于 orchestrator/internal。同步到本 module 时，
	// 仅机械提升 namespace，使项目自有 runtime adapter 可以访问；源码行为不变。
	{"github.com/e2b-dev/infra/packages/orchestrator/internal", "gitcode.com/cloud-native/cloud-native/TemplateCreator/internal/upstream/orchestrator/pkg"},
	// 后续 infra 已把 feature-flags 目录改名为 featureflags；保持 TemplateCreator
	// 私有 import path 稳定，避免项目自有 logger/runtime 代码感知版本差异。
	{"github.com/e2b-dev/infra/packages/shared/pkg/feature-flags", "gitcode.com/cloud-native/cloud-native/TemplateCreator/internal/upstream/shared/pkg/featureflags"},
	{"github.com/e2b-dev/infra/packages/orchestrator", "gitcode.com/cloud-native/cloud-native/TemplateCreator/internal/upstream/orchestrator"},
	{"github.com/e2b-dev/infra/packages/shared", "gitcode.com/cloud-native/cloud-native/TemplateCreator/internal/upstream/shared"},
	{"github.com/e2b-dev/infra/packages/clickhouse", "gitcode.com/cloud-native/cloud-native/TemplateCreator/internal/upstream/clickhouse"},
}

type syncConfig struct {
	InfraRoot    string
	TargetRoot   string
	Commit       string
	PackagePaths []string
}

type sourceManifest struct {
	SchemaVersion  int                  `json:"schema_version"`
	UpstreamCommit string               `json:"upstream_commit"`
	Files          []sourceManifestFile `json:"files"`
}

type sourceManifestFile struct {
	SourcePath string `json:"source_path"`
	Path       string `json:"path"`
	SHA256     string `json:"sha256"`
	Modified   bool   `json:"modified"`
}

func main() {
	infraRoot := flag.String("infra-root", "", "maintainer-only path to the locked infra checkout")
	packageList := flag.String("package-list", "scripts/infra-packages.txt", "package list relative to the module root")
	targetRoot := flag.String("target", "internal/upstream", "generated snapshot target")
	flag.Parse()
	if *infraRoot == "" {
		fatal(errors.New("-infra-root is required"))
	}
	list, err := os.Open(*packageList)
	if err != nil {
		fatal(err)
	}
	packages, parseErr := parsePackageList(list)
	closeErr := list.Close()
	if parseErr != nil {
		fatal(parseErr)
	}
	if closeErr != nil {
		fatal(closeErr)
	}
	if err := verifyInfraCommit(*infraRoot, lockedInfraCommit); err != nil {
		fatal(err)
	}
	if err := syncPackages(syncConfig{InfraRoot: *infraRoot, TargetRoot: *targetRoot, Commit: lockedInfraCommit, PackagePaths: packages}); err != nil {
		fatal(err)
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}

func parsePackageList(reader io.Reader) ([]string, error) {
	var result []string
	seen := make(map[string]struct{})
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		value := strings.TrimSpace(scanner.Text())
		if value == "" || strings.HasPrefix(value, "#") {
			continue
		}
		if strings.Contains(value, `\`) || path.IsAbs(value) || path.Clean(value) != value || strings.Contains(value, "../") || !validPackagePrefix(value) {
			return nil, fmt.Errorf("unsafe or unsupported package path %q", value)
		}
		if _, exists := seen[value]; exists {
			return nil, fmt.Errorf("duplicate package path %q", value)
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if len(result) == 0 {
		return nil, errors.New("package list is empty")
	}
	sort.Strings(result)
	return result, nil
}

func validPackagePrefix(value string) bool {
	for _, prefix := range []string{"packages/orchestrator/", "packages/shared/", "packages/clickhouse/"} {
		if strings.HasPrefix(value, prefix) {
			return true
		}
	}
	return false
}

func verifyInfraCommit(root, expected string) error {
	actual, err := gitOutput(root, "rev-parse", "HEAD")
	if err != nil {
		return err
	}
	if strings.TrimSpace(actual) != expected {
		return fmt.Errorf("infra commit %s does not match locked commit %s", strings.TrimSpace(actual), expected)
	}
	status, err := gitOutput(root, "status", "--porcelain", "--untracked-files=no")
	if err != nil {
		return err
	}
	if strings.TrimSpace(status) != "" {
		return errors.New("infra worktree is dirty")
	}
	return nil
}

func gitOutput(root string, arguments ...string) (string, error) {
	command := exec.Command("git", append([]string{"-C", root}, arguments...)...)
	output, err := command.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %v: %w: %s", arguments, err, output)
	}
	return string(output), nil
}

func syncPackages(config syncConfig) error {
	parent := filepath.Dir(config.TargetRoot)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return err
	}
	temporary, err := os.MkdirTemp(parent, ".upstream-sync-*")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(temporary) }()

	tracked, err := trackedPackageFiles(config.InfraRoot, config.PackagePaths)
	if err != nil {
		return err
	}
	manifest := sourceManifest{SchemaVersion: 1, UpstreamCommit: config.Commit}
	for _, packagePath := range config.PackagePaths {
		source := filepath.Join(config.InfraRoot, filepath.FromSlash(packagePath))
		targetRelative := targetRelativePackagePath(packagePath)
		if err := copyPackage(source, filepath.Join(temporary, filepath.FromSlash(targetRelative)), packagePath, temporary, tracked, &manifest); err != nil {
			return fmt.Errorf("sync %s: %w", packagePath, err)
		}
	}
	manifest.Files, err = deduplicateManifestFiles(manifest.Files)
	if err != nil {
		return err
	}
	sort.Slice(manifest.Files, func(i, j int) bool { return manifest.Files[i].Path < manifest.Files[j].Path })
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if err := os.WriteFile(filepath.Join(temporary, "SOURCE_MANIFEST.json"), data, 0o644); err != nil {
		return err
	}
	if err := os.RemoveAll(config.TargetRoot); err != nil {
		return err
	}
	return os.Rename(temporary, config.TargetRoot)
}

// trackedPackageFiles 只允许锁定 commit 已跟踪的文件进入 snapshot；工作树中的
// untracked/ignored build 产物即使位于 package 目录也不会污染 source manifest。
func trackedPackageFiles(infraRoot string, packagePaths []string) (map[string]struct{}, error) {
	arguments := append([]string{"ls-files", "-z", "--"}, packagePaths...)
	output, err := gitOutput(infraRoot, arguments...)
	if err != nil {
		return nil, err
	}
	tracked := make(map[string]struct{})
	for _, file := range strings.Split(output, "\x00") {
		if file != "" {
			tracked[filepath.ToSlash(file)] = struct{}{}
		}
	}
	return tracked, nil
}

// targetRelativePackagePath 将 2026.09 的私有目录布局映射到本 module 已有的
// upstream namespace。映射只影响生成路径，不改变 package 内容或公开 API。
func targetRelativePackagePath(packagePath string) string {
	relative := strings.TrimPrefix(packagePath, "packages/")
	if strings.HasPrefix(relative, "orchestrator/internal/") {
		return "orchestrator/pkg/" + strings.TrimPrefix(relative, "orchestrator/internal/")
	}
	if strings.HasPrefix(relative, "shared/pkg/feature-flags") {
		return "shared/pkg/featureflags" + strings.TrimPrefix(relative, "shared/pkg/feature-flags")
	}

	return relative
}

func deduplicateManifestFiles(files []sourceManifestFile) ([]sourceManifestFile, error) {
	byPath := make(map[string]sourceManifestFile, len(files))
	for _, file := range files {
		if existing, found := byPath[file.Path]; found {
			if existing.SourcePath != file.SourcePath || existing.SHA256 != file.SHA256 || existing.Modified != file.Modified {
				return nil, fmt.Errorf("conflicting generated content for %s", file.Path)
			}
			continue
		}
		byPath[file.Path] = file
	}
	result := make([]sourceManifestFile, 0, len(byPath))
	for _, file := range byPath {
		result = append(result, file)
	}
	return result, nil
}

func copyPackage(sourceRoot, targetRoot, packagePath, snapshotRoot string, tracked map[string]struct{}, manifest *sourceManifest) error {
	return filepath.WalkDir(sourceRoot, func(source string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(sourceRoot, source)
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return os.MkdirAll(filepath.Join(targetRoot, relative), 0o755)
		}
		sourceGitPath := path.Join(packagePath, filepath.ToSlash(relative))
		if _, exists := tracked[sourceGitPath]; !exists {
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 || strings.HasSuffix(entry.Name(), "_test.go") {
			return nil
		}
		if filepath.Dir(relative) != "." && isProductionSourceFile(entry.Name()) {
			// Nested Go/C/assembly files belong to a different package and must
			// appear explicitly in the package list. Non-code files may be embeds.
			return nil
		}
		content, err := os.ReadFile(source)
		if err != nil {
			return err
		}
		contentModified := false
		if strings.HasSuffix(entry.Name(), ".go") {
			content, contentModified = rewriteGoSource(content)
		} else {
			content, contentModified = rewriteImportReferences(content)
		}
		target := filepath.Join(targetRoot, relative)
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if err := os.WriteFile(target, content, info.Mode().Perm()); err != nil {
			return err
		}
		manifestPath, err := filepath.Rel(snapshotRoot, target)
		if err != nil {
			return err
		}
		sourceManifestPath := strings.TrimPrefix(sourceGitPath, "packages/")
		generatedPath := filepath.ToSlash(manifestPath)
		manifest.Files = append(manifest.Files, sourceManifestFile{
			SourcePath: sourceManifestPath,
			Path:       generatedPath,
			SHA256:     fmt.Sprintf("%x", sha256.Sum256(content)),
			Modified:   contentModified || sourceManifestPath != generatedPath,
		})
		return nil
	})
}

func isProductionSourceFile(name string) bool {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".go", ".s", ".c", ".cc", ".cpp", ".h":
		return true
	default:
		return false
	}
}

func rewriteGoSource(source []byte) ([]byte, bool) {
	result, modified := rewriteImportReferences(source)
	if !modified {
		return result, false
	}
	packageOffset := packageDeclarationOffset(result)
	if packageOffset < 0 {
		return result, true
	}
	notice := []byte(templateCreatorModificationNotice + "\n\n")
	result = append(result[:packageOffset], append(notice, result[packageOffset:]...)...)
	return result, true
}

func packageDeclarationOffset(source []byte) int {
	offset := 0
	for _, line := range bytes.SplitAfter(source, []byte("\n")) {
		trimmed := bytes.TrimLeft(line, " \t")
		if bytes.HasPrefix(trimmed, []byte("package ")) {
			return offset + len(line) - len(trimmed)
		}
		offset += len(line)
	}
	return -1
}

func rewriteImportReferences(source []byte) ([]byte, bool) {
	result := append([]byte(nil), source...)
	modified := false
	for _, rewrite := range importRewrites {
		updated := bytes.ReplaceAll(result, []byte(rewrite.old), []byte(rewrite.new))
		if !bytes.Equal(updated, result) {
			modified = true
			result = updated
		}
	}
	return result, modified
}
