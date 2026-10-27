package templatecreator

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

func TestValidateCreatorConfigRequiresScratchRoot(t *testing.T) {
	cfg := validCreatorConfig(t)
	cfg.ScratchRoot = ""

	err := validateCreatorConfig(cfg)
	if err == nil || !strings.Contains(err.Error(), "ScratchRoot") {
		t.Fatalf("validateCreatorConfig() error = %v, want ScratchRoot error", err)
	}
}

func TestValidateCreatorConfigAcceptsFileStore(t *testing.T) {
	cfg := validCreatorConfig(t)

	if err := validateCreatorConfig(cfg); err != nil {
		t.Fatalf("validateCreatorConfig() error = %v", err)
	}
}

func TestRuntimeConfigRejectsMismatchedAssetChecksum(t *testing.T) {
	cfg := validCreatorConfig(t)
	cfg.Runtime.KernelSHA256 = strings.Repeat("0", 64)

	err := validateCreatorConfig(cfg)
	if err == nil || !strings.Contains(err.Error(), "kernel") || !strings.Contains(err.Error(), "SHA-256") {
		t.Fatalf("validateCreatorConfig() error = %v, want kernel SHA-256 error", err)
	}
}

func TestValidateCreatorConfigRequiresEnvdVersion(t *testing.T) {
	cfg := validCreatorConfig(t)
	cfg.Runtime.EnvdVersion = ""

	err := validateCreatorConfig(cfg)
	if err == nil || !strings.Contains(err.Error(), "EnvdVersion") {
		t.Fatalf("validateCreatorConfig() error = %v, want EnvdVersion error", err)
	}
}

func TestValidateCreatorConfigRejectsS3InDemo(t *testing.T) {
	cfg := validCreatorConfig(t)
	cfg.Store.URL = "s3://template-bucket?region=example"

	err := validateCreatorConfig(cfg)
	if err == nil || !strings.Contains(err.Error(), "file://") {
		t.Fatalf("validateCreatorConfig() error = %v, want file:// error", err)
	}
}

func TestNormalizeCreatorConfigDefaultsMaxParallel(t *testing.T) {
	cfg := validCreatorConfig(t)
	cfg.MaxParallel = 0

	got := normalizeCreatorConfig(cfg)
	if got.MaxParallel != 1 {
		t.Fatalf("normalizeCreatorConfig() MaxParallel = %d, want 1", got.MaxParallel)
	}
}

func TestValidateCreatorConfigRejectsUnknownLogLevel(t *testing.T) {
	cfg := validCreatorConfig(t)
	cfg.LogLevel = LogLevel("verbose")

	err := validateCreatorConfig(cfg)
	if err == nil || !strings.Contains(err.Error(), "LogLevel") {
		t.Fatalf("validateCreatorConfig() error = %v, want LogLevel error", err)
	}
}

func TestNormalizeCreatorConfigResolvesLogLevel(t *testing.T) {
	tests := []struct {
		name        string
		level       LogLevel
		withWriter  bool
		legacyDebug bool
		wantLevel   LogLevel
		wantWriter  bool
		wantDebug   bool
	}{
		{name: "zero value stays silent", wantLevel: LogLevelOff},
		{name: "legacy writer keeps info", withWriter: true, wantLevel: LogLevelInfo, wantWriter: true},
		{name: "legacy debug keeps debug", withWriter: true, legacyDebug: true, wantLevel: LogLevelDebug, wantWriter: true, wantDebug: true},
		{name: "explicit off wins", level: LogLevelOff, withWriter: true, legacyDebug: true, wantLevel: LogLevelOff},
		{name: "explicit info wins", level: LogLevelInfo, withWriter: true, legacyDebug: true, wantLevel: LogLevelInfo, wantWriter: true},
		{name: "explicit debug", level: LogLevelDebug, withWriter: true, wantLevel: LogLevelDebug, wantWriter: true, wantDebug: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cfg := validCreatorConfig(t)
			var output strings.Builder
			if test.withWriter {
				cfg.LogWriter = &output
			}
			cfg.LogLevel = test.level
			cfg.DebugLogs = test.legacyDebug

			got := normalizeCreatorConfig(cfg)
			if got.LogLevel != test.wantLevel || (got.LogWriter != nil) != test.wantWriter || got.DebugLogs != test.wantDebug {
				t.Fatalf("normalized logging = level %q writer=%t debug=%t, want level %q writer=%t debug=%t", got.LogLevel, got.LogWriter != nil, got.DebugLogs, test.wantLevel, test.wantWriter, test.wantDebug)
			}
		})
	}
}

func TestNormalizeCreatorConfigAddsDefaultProvisioningSteps(t *testing.T) {
	cfg := validCreatorConfig(t)
	cfg.Defaults.Steps = nil
	cfg.Format = BuildFormatFromDockerfile

	got, _, _, err := resolveBuildOptions(cfg.Defaults, cfg.Format, DefaultPublicationMode, nil, "", "")
	if err != nil {
		t.Fatal(err)
	}
	want := []TemplateStep{
		{Type: "USER", Args: []string{"root"}},
		{Type: "WORKDIR", Args: []string{"/"}},
		{Type: "USER", Args: []string{"user"}},
		{Type: "WORKDIR", Args: []string{"/home/user"}},
	}
	if !reflect.DeepEqual(got.Steps, want) {
		t.Fatalf("default steps = %#v, want %#v", got.Steps, want)
	}
}

func TestResolveBuildOptionsUsesFormatSpecificDefaults(t *testing.T) {
	defaults := DefaultBuildSpec()
	defaults.Steps = nil

	imageSpec, imageFormat, imagePublication, err := resolveBuildOptions(defaults, BuildFormatFromImage, PublicationAll, nil, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if imageFormat != BuildFormatFromImage || imagePublication != PublicationAll || imageSpec.Steps == nil || len(imageSpec.Steps) != 0 {
		t.Fatalf("fromImage defaults = format=%q publication=%q steps=%#v", imageFormat, imagePublication, imageSpec.Steps)
	}

	dockerfileSpec, dockerfileFormat, _, err := resolveBuildOptions(defaults, BuildFormatFromImage, PublicationAll, nil, BuildFormatFromDockerfile, "")
	if err != nil {
		t.Fatal(err)
	}
	if dockerfileFormat != BuildFormatFromDockerfile || !reflect.DeepEqual(dockerfileSpec.Steps, defaultProvisioningSteps()) {
		t.Fatalf("fromDockerfile defaults = format=%q steps=%#v", dockerfileFormat, dockerfileSpec.Steps)
	}
}

func TestNormalizeCreateRequestUsesDefaultImage(t *testing.T) {
	got := normalizeCreateRequest(CreateRequest{})
	if got.ImageRef != DefaultImageRef {
		t.Fatalf("normalized empty request image = %q, want %q", got.ImageRef, DefaultImageRef)
	}
}

func TestResolveBuildOptionsRejectsUnknownFormatAndPublication(t *testing.T) {
	if _, _, _, err := resolveBuildOptions(DefaultBuildSpec(), BuildFormatFromImage, PublicationAll, nil, BuildFormat("unknown"), ""); err == nil {
		t.Fatal("unknown format accepted")
	}
	if _, _, _, err := resolveBuildOptions(DefaultBuildSpec(), BuildFormatFromImage, PublicationAll, nil, "", PublicationMode("unknown")); err == nil {
		t.Fatal("unknown publication accepted")
	}
}

func TestNormalizeCreatorConfigKeepsExplicitProvisioningSteps(t *testing.T) {
	tests := []struct {
		name  string
		steps []TemplateStep
	}{
		{name: "custom", steps: []TemplateStep{{Type: "WORKDIR", Args: []string{"/workspace"}}}},
		{name: "empty disables defaults", steps: []TemplateStep{}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cfg := validCreatorConfig(t)
			cfg.Defaults.Steps = test.steps

			got := normalizeCreatorConfig(cfg)
			if !reflect.DeepEqual(got.Defaults.Steps, test.steps) {
				t.Fatalf("normalized steps = %#v, want %#v", got.Defaults.Steps, test.steps)
			}
		})
	}
}

func TestNormalizeCreatorConfigDefaultStepsAreIsolated(t *testing.T) {
	first, _, _, err := resolveBuildOptions(DefaultBuildSpec(), BuildFormatFromDockerfile, DefaultPublicationMode, nil, "", "")
	if err != nil {
		t.Fatal(err)
	}
	second, _, _, err := resolveBuildOptions(DefaultBuildSpec(), BuildFormatFromDockerfile, DefaultPublicationMode, nil, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Steps) == 0 || len(second.Steps) == 0 {
		t.Fatalf("default steps are missing: first=%#v second=%#v", first.Steps, second.Steps)
	}

	first.Steps[0].Args[0] = "changed"
	if second.Steps[0].Args[0] != "root" {
		t.Fatalf("second default steps changed with first: %#v", second.Steps)
	}
}

func TestResolveBuildSpecReplacesDefaultProvisioningSteps(t *testing.T) {
	defaults := normalizeCreatorConfig(validCreatorConfig(t)).Defaults
	override := &BuildSpec{Steps: []TemplateStep{{Type: "USER", Args: []string{"app"}}}}

	got, err := resolveBuildSpec(defaults, override)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.Steps, override.Steps) {
		t.Fatalf("resolved steps = %#v, want replacement %#v", got.Steps, override.Steps)
	}
}

func TestCacheConfigZeroValueKeepsSharedCacheDisabled(t *testing.T) {
	cfg := validCreatorConfig(t)

	if err := validateCreatorConfig(cfg); err != nil {
		t.Fatalf("validateCreatorConfig() error = %v", err)
	}
	if cfg.Cache.Enabled || cfg.Cache.Root != "" || cfg.Cache.AllowMutableImageTags {
		t.Fatalf("zero CacheConfig = %#v, want disabled safe defaults", cfg.Cache)
	}
}

func TestCacheConfigEnabledRequiresAbsoluteRoot(t *testing.T) {
	cfg := validCreatorConfig(t)
	cfg.Cache = CacheConfig{Enabled: true, Root: "relative/cache"}

	err := validateCreatorConfig(cfg)
	if err == nil || !strings.Contains(err.Error(), "Cache.Root") || !strings.Contains(err.Error(), "absolute") {
		t.Fatalf("validateCreatorConfig() error = %v, want absolute Cache.Root error", err)
	}
}

func TestCacheConfigRejectsSharedCacheForInfra202609(t *testing.T) {
	cfg := validCreatorConfig(t)
	cfg.Cache = CacheConfig{Enabled: true, Root: t.TempDir()}

	err := validateCreatorConfig(cfg)
	if err == nil || !strings.Contains(err.Error(), "infra 2026.09") {
		t.Fatalf("validateCreatorConfig() error = %v, want unsupported infra 2026.09 cache error", err)
	}
}

func TestCacheConfigRejectsOverlappingPersistentRoots(t *testing.T) {
	tests := []struct {
		name string
		root func(CreatorConfig) string
	}{
		{name: "same as scratch", root: func(cfg CreatorConfig) string { return cfg.ScratchRoot }},
		{name: "inside scratch", root: func(cfg CreatorConfig) string { return filepath.Join(cfg.ScratchRoot, "cache") }},
		{name: "parent of scratch", root: func(cfg CreatorConfig) string { return filepath.Dir(cfg.ScratchRoot) }},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cfg := validCreatorConfig(t)
			cfg.Cache = CacheConfig{Enabled: true, Root: test.root(cfg)}

			err := validateCreatorConfig(cfg)
			if err == nil || !strings.Contains(err.Error(), "must not overlap") {
				t.Fatalf("validateCreatorConfig() error = %v, want root overlap error", err)
			}
		})
	}
}

func TestValidateCreateRequestRequiresImageRef(t *testing.T) {
	err := validateCreateRequest(CreateRequest{})
	if err == nil || !strings.Contains(err.Error(), "ImageRef") {
		t.Fatalf("validateCreateRequest() error = %v, want ImageRef error", err)
	}
}

func TestValidateCreateRequestAcceptsImageTarPath(t *testing.T) {
	err := validateCreateRequest(CreateRequest{ImageTarPath: "/tmp/image.tar"})
	if err != nil {
		t.Fatalf("validateCreateRequest() error = %v, want tar input accepted", err)
	}
}

func TestValidateCreateRequestRejectsImageRefAndImageTarPathTogether(t *testing.T) {
	err := validateCreateRequest(CreateRequest{ImageRef: "alpine:3.20", ImageTarPath: "/tmp/image.tar"})
	if err == nil || !strings.Contains(err.Error(), "mutually exclusive") {
		t.Fatalf("validateCreateRequest() error = %v, want mutually exclusive error", err)
	}
}

func TestResolveBuildSpecUsesDefaults(t *testing.T) {
	defaults := BuildSpec{
		VCPU:       2,
		MemoryMB:   1024,
		DiskSizeMB: 2048,
		HugePages:  true,
		Steps:      []TemplateStep{},
	}

	got, err := resolveBuildSpec(defaults, nil)
	if err != nil {
		t.Fatalf("resolveBuildSpec() error = %v", err)
	}
	if !reflect.DeepEqual(got, defaults) {
		t.Fatalf("resolveBuildSpec() = %#v, want %#v", got, defaults)
	}
}

func TestResolveBuildSpecAppliesOverride(t *testing.T) {
	defaults := BuildSpec{
		VCPU:       2,
		MemoryMB:   1024,
		DiskSizeMB: 2048,
		HugePages:  true,
	}
	override := &BuildSpec{
		VCPU:         4,
		MemoryMB:     4096,
		DiskSizeMB:   8192,
		HugePages:    false,
		StartCommand: "sleep infinity",
		Steps:        []TemplateStep{},
	}

	got, err := resolveBuildSpec(defaults, override)
	if err != nil {
		t.Fatalf("resolveBuildSpec() error = %v", err)
	}
	if !reflect.DeepEqual(got, *override) {
		t.Fatalf("resolveBuildSpec() = %#v, want %#v", got, *override)
	}
}

func TestResolveBuildSpecMergesNonZeroOverrideFields(t *testing.T) {
	defaults := BuildSpec{
		VCPU:       2,
		MemoryMB:   1024,
		DiskSizeMB: 2048,
		HugePages:  true,
	}
	override := &BuildSpec{VCPU: 4}

	got, err := resolveBuildSpec(defaults, override)
	if err != nil {
		t.Fatalf("resolveBuildSpec() error = %v", err)
	}
	if got.VCPU != 4 || got.MemoryMB != 1024 || got.DiskSizeMB != 2048 {
		t.Fatalf("resolveBuildSpec() = %#v, want VCPU override and inherited memory/disk", got)
	}
	if got.HugePages {
		t.Fatal("resolveBuildSpec() HugePages = true, want explicit false from non-nil override")
	}
}

func TestResolveBuildSpecRejectsNonPositiveResources(t *testing.T) {
	tests := []struct {
		name string
		spec BuildSpec
	}{
		{name: "vcpu", spec: BuildSpec{VCPU: -1, MemoryMB: 1024, DiskSizeMB: 1024}},
		{name: "memory", spec: BuildSpec{VCPU: 1, MemoryMB: -1, DiskSizeMB: 1024}},
		{name: "disk", spec: BuildSpec{VCPU: 1, MemoryMB: 1024, DiskSizeMB: -1}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := resolveBuildSpec(tt.spec, nil); err == nil {
				t.Fatal("resolveBuildSpec() error = nil, want validation error")
			}
		})
	}
}

func validCreatorConfig(t *testing.T) CreatorConfig {
	t.Helper()
	runtimeRoot := t.TempDir()
	kernelPath := filepath.Join(runtimeRoot, "kernels", "vmlinux-test", runtime.GOARCH, "vmlinux.bin")
	firecrackerPath := filepath.Join(runtimeRoot, "firecrackers", "v1.14.1_test", runtime.GOARCH, "firecracker")
	envdPath := filepath.Join(runtimeRoot, "envd", "0.6.10", runtime.GOARCH, "envd")
	busyboxPath := filepath.Join(runtimeRoot, "busybox", "1.36.1", runtime.GOARCH, "busybox")
	assets := map[string]string{
		kernelPath: "kernel", firecrackerPath: "firecracker", envdPath: "envd", busyboxPath: "busybox",
	}
	hashes := make(map[string]string, len(assets))
	for assetPath, content := range assets {
		writeTestFile(t, "", assetPath, []byte(content))
		if err := os.Chmod(assetPath, 0o755); err != nil {
			t.Fatal(err)
		}
		hashes[assetPath] = fmt.Sprintf("%x", sha256.Sum256([]byte(content)))
	}

	return CreatorConfig{
		ScratchRoot: t.TempDir(),
		Store: StoreConfig{
			URL: "file://" + t.TempDir(),
		},
		Runtime: RuntimeConfig{
			RuntimeDir:         runtimeRoot,
			InfraCommit:        "557445ffddda8d9a27f6f529a3f4d7732cf81a13",
			TemplateVersion:    "v2",
			Architecture:       runtime.GOARCH,
			KernelVersion:      "vmlinux-test",
			FirecrackerVersion: "v1.14.1_test",
			EnvdVersion:        "0.6.10",
			EnvdPath:           envdPath,
			BusyboxDir:         filepath.Join(runtimeRoot, "busybox"),
			BusyboxVersion:     "1.36.1",
			KernelSHA256:       hashes[kernelPath],
			FirecrackerSHA256:  hashes[firecrackerPath],
			EnvdSHA256:         hashes[envdPath],
			BusyboxSHA256:      hashes[busyboxPath],
		},
		Defaults: BuildSpec{
			VCPU:       2,
			MemoryMB:   1024,
			DiskSizeMB: 2048,
		},
		MaxParallel: 1,
	}
}
