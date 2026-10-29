package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	templatecreator "gitcode.com/boostkit/cloud-native.git/TemplateCreator"
	"gitcode.com/boostkit/cloud-native.git/TemplateCreator/internal/preflight"
)

type fakeCreator struct {
	request templatecreator.CreateRequest
	result  templatecreator.CreateResult
	closed  bool
}

func fakeArtifactsResult() templatecreator.ExportTemplateArtifactsResult {
	return templatecreator.ExportTemplateArtifactsResult{
		TemplateID: "tmpl_demo", BuildID: "build-demo", ObjectCount: 6,
		OutputPath: "/exports/tmpl_demo", Size: 123,
	}
}

type errorWriter struct {
	err error
}

func (writer errorWriter) Write([]byte) (int, error) {
	return 0, writer.err
}

func (creator *fakeCreator) CreateTemplate(_ context.Context, request templatecreator.CreateRequest) (templatecreator.CreateResult, error) {
	creator.request = request
	return creator.result, nil
}

func (creator *fakeCreator) Close(context.Context) error {
	creator.closed = true
	return nil
}

func TestRunHelp(t *testing.T) {
	tests := []struct {
		name string
		arg  string
	}{
		{name: "short", arg: "-h"},
		{name: "long", arg: "--help"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			preflightCalls := 0
			newCreatorCalls := 0
			dependencies := dependencies{
				preflight: func(preflight.Config) preflight.Report {
					preflightCalls++
					return preflight.Report{}
				},
				newCreator: func(context.Context, templatecreator.CreatorConfig) (demoCreator, error) {
					newCreatorCalls++
					return nil, errors.New("must not be called")
				},
			}
			var output bytes.Buffer
			err := run(context.Background(), []string{test.arg}, &output, dependencies)
			if err != nil {
				t.Fatalf("run(%q) error = %v", test.arg, err)
			}
			if preflightCalls != 0 || newCreatorCalls != 0 {
				t.Fatalf("run(%q) preflight calls = %d, newCreator calls = %d", test.arg, preflightCalls, newCreatorCalls)
			}
			help := output.String()
			for _, want := range []string{
				"TemplateCreator", "用法", "必填参数", "可选参数",
				"-runtime-info", "-scratch-root", "-store-root", "-image",
				"示例", "结果", "/opt/templatecreator/", "debian:bookworm-slim",
			} {
				if !strings.Contains(help, want) {
					t.Errorf("run(%q) help missing %q:\n%s", test.arg, want, help)
				}
			}
			if strings.Contains(help, "flag: help requested") {
				t.Fatalf("run(%q) help contains internal flag error:\n%s", test.arg, help)
			}
		})
	}
}

func TestRunHelpDocumentsEveryFlagAndDefault(t *testing.T) {
	var output bytes.Buffer
	if err := run(context.Background(), []string{"-h"}, &output, dependencies{}); err != nil {
		t.Fatal(err)
	}
	help := output.String()
	for _, want := range []string{
		"  -runtime-info <路径>",
		"  -scratch-root <目录>",
		"  -store-root <目录>",
		"  -image <镜像>",
		"  -image-tar <文件>",
		"  -idempotency-key <值>",
		"  -vcpu <数量>",
		"  -memory-mb <MiB>",
		"  -disk-size-mb <MiB>",
		"  -hugepages ",
		"  -start-command <命令>",
		"  -ready-command <命令>",
		"  -timeout <时长>",
		"  -max-parallel <数量>",
		"  -proxy-port <端口>",
		"  -log-level <级别>",
		"  -preflight-only ",
		"  -h, --help ",
	} {
		if !strings.Contains(help, want) {
			t.Errorf("help missing public flag %q:\n%s", strings.TrimSpace(want), help)
		}
	}
	for _, want := range []string{
		"虚拟 CPU 数量（默认 2）",
		"guest 内存（默认 1024 MiB）",
		"根文件系统大小（默认 4096 MiB）",
		"整体构建超时（默认 30m）",
		"最大物理构建并发数（默认 1）",
		"sandbox proxy 端口（默认 5007）",
		"运行日志级别（默认 info）",
		"infra 2026.09 不支持持久 base-layer cache",
	} {
		if !strings.Contains(help, want) {
			t.Errorf("help missing documented default %q:\n%s", want, help)
		}
	}
}

func TestRunHelpWriterFailure(t *testing.T) {
	wantErr := errors.New("help writer failed")
	err := run(context.Background(), []string{"-h"}, errorWriter{err: wantErr}, dependencies{})
	if !errors.Is(err, wantErr) {
		t.Fatalf("run(-h) error = %v, want %v", err, wantErr)
	}
}

func TestRunParseErrorKeepsOutputEmpty(t *testing.T) {
	var output bytes.Buffer
	err := run(context.Background(), []string{"-unknown-flag"}, &output, dependencies{})
	if err == nil {
		t.Fatal("run() error = nil, want unknown flag error")
	}
	if output.Len() != 0 {
		t.Fatalf("run() stdout = %q, want empty", output.String())
	}
}

func TestParseFlagsAcceptsSmokeArguments(t *testing.T) {
	options, err := parseFlags([]string{
		"-runtime-info", "/runtime-info.json",
		"-scratch-root", "/scratch",
		"-store-root", "/store",
		"-image", "alpine:3.20",
		"-vcpu", "4", "-memory-mb", "4096", "-disk-size-mb", "8192",
		"-hugepages", "-timeout", "45m", "-idempotency-key", "demo", "-max-parallel", "2", "-proxy-port", "5008", "-log-level", "debug",
	})
	if err != nil {
		t.Fatal(err)
	}
	if options.RuntimeInfoPath != "/runtime-info.json" || options.ScratchRoot != "/scratch" || options.StoreRoot != "/store" || options.ImageRef != "alpine:3.20" {
		t.Fatalf("parseFlags() = %#v", options)
	}
	if options.Build.VCPU != 4 || options.Build.MemoryMB != 4096 || options.Build.DiskSizeMB != 8192 || !options.Build.HugePages || options.Timeout != 45*time.Minute || options.MaxParallel != 2 || options.ProxyPort != 5008 || options.LogLevel != "debug" {
		t.Fatalf("parseFlags() settings = %#v", options)
	}
}

func TestParseFlagsUsesCentralBuildDefaults(t *testing.T) {
	options, err := parseFlags(nil)
	if err != nil {
		t.Fatal(err)
	}
	if options.ImageRef != templatecreator.DefaultImageRef {
		t.Fatalf("default image = %q, want %q", options.ImageRef, templatecreator.DefaultImageRef)
	}
	if options.Format != string(templatecreator.DefaultBuildFormat) {
		t.Fatalf("default format = %q, want %q", options.Format, templatecreator.DefaultBuildFormat)
	}
	if options.Publication != string(templatecreator.DefaultPublicationMode) {
		t.Fatalf("default publication = %q, want %q", options.Publication, templatecreator.DefaultPublicationMode)
	}
	if options.RuntimeInfoPath != templatecreator.DefaultRuntimeInfoPath || options.ScratchRoot != templatecreator.DefaultScratchRoot || options.StoreRoot != templatecreator.DefaultStoreRoot {
		t.Fatalf("default paths = runtime=%q scratch=%q store=%q", options.RuntimeInfoPath, options.ScratchRoot, options.StoreRoot)
	}
}

func TestParseFlagsAcceptsFormatAndPublicationOverrides(t *testing.T) {
	options, err := parseFlags([]string{"-format", "fromDockerfile", "-publication", "simple"})
	if err != nil {
		t.Fatal(err)
	}
	if options.Format != "fromDockerfile" || options.Publication != "simple" {
		t.Fatalf("format/publication = %q/%q", options.Format, options.Publication)
	}
}

func TestParseFlagsAcceptsImageTar(t *testing.T) {
	args := []string{
		"-runtime-info", "/runtime-info.json",
		"-scratch-root", "/scratch",
		"-store-root", "/store",
		"-image-tar", "/images/ubuntu.tar",
	}
	options, err := parseFlags(args)
	if err != nil {
		t.Fatal(err)
	}
	if options.ImageRef != "" || options.ImageTarPath != "/images/ubuntu.tar" {
		t.Fatalf("parseFlags() image inputs = %#v, want tar path only", options)
	}
}

func TestParseFlagsDerivesSimpleWorkDirMode(t *testing.T) {
	options, err := parseFlags([]string{"-image-id", "redis:e2b-x86", "-work-dir", "/opt/templatecreator"})
	if err != nil {
		t.Fatal(err)
	}
	if options.ImageTarPath != "/opt/templatecreator/redis:e2b-x86.tar" || options.RuntimeInfoPath != "/opt/templatecreator/runtime-info.json" || options.ScratchRoot != "/opt/templatecreator/scratch" || options.StoreRoot != "/opt/templatecreator/store" || options.ExportRoot != "/opt/templatecreator" {
		t.Fatalf("simple mode options = %#v", options)
	}
}

func TestParseFlagsSimpleModeKeepsExplicitExportRoot(t *testing.T) {
	options, err := parseFlags([]string{
		"-image-id", "redis:e2b-x86",
		"-work-dir", "/opt/templatecreator",
		"-export-root", "/mnt/template-exports",
	})
	if err != nil {
		t.Fatal(err)
	}
	if options.ExportRoot != "/mnt/template-exports" {
		t.Fatalf("ExportRoot = %q, want explicit path", options.ExportRoot)
	}
}

func TestParseFlagsRejectsImageAndImageTarTogether(t *testing.T) {
	_, err := parseFlags([]string{
		"-runtime-info", "/runtime-info.json",
		"-scratch-root", "/scratch",
		"-store-root", "/store",
		"-image", "alpine:3.20",
		"-image-tar", "/images/alpine.tar",
	})
	if err == nil || !strings.Contains(err.Error(), "mutually exclusive") {
		t.Fatalf("parseFlags() error = %v, want mutually exclusive error", err)
	}
}

func TestParseFlagsRejectsConflictingBaseCacheOptions(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "disable with root", args: []string{"-cache-root", "/cache", "-disable-base-cache"}, want: "cannot be combined"},
		{name: "mutable without cache", args: []string{"-allow-mutable-image-tag-cache"}, want: "requires -cache-root"},
		{name: "bypass without cache", args: []string{"-bypass-base-cache"}, want: "requires -cache-root"},
		{name: "relative root", args: []string{"-cache-root", "cache"}, want: "absolute"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := parseFlags(append(coreArguments(t), test.args...))
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("parseFlags() error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestParseFlagsRejectsSharedBaseCacheForInfra202609(t *testing.T) {
	_, err := parseFlags(append(coreArguments(t), "-cache-root", "/cache"))
	if err == nil || !strings.Contains(err.Error(), "infra 2026.09") {
		t.Fatalf("parseFlags() error = %v, want unsupported infra 2026.09 cache error", err)
	}
}

func TestParseFlagsRejectsUnknownLogLevel(t *testing.T) {
	_, err := parseFlags(append(coreArguments(t), "-log-level", "verbose"))
	if err == nil || !strings.Contains(err.Error(), "-log-level") {
		t.Fatalf("parseFlags() error = %v, want invalid -log-level", err)
	}
}

func TestParseFlagsAllowsOmittedCoreArguments(t *testing.T) {
	if _, err := parseFlags(nil); err != nil {
		t.Fatalf("parseFlags() error = %v, want central defaults", err)
	}
}

func TestLoadRuntimeInfoBuildsRuntimeConfig(t *testing.T) {
	path := writeRuntimeInfo(t)
	config, paths, err := loadRuntimeInfo(path)
	if err != nil {
		t.Fatal(err)
	}
	if config.Architecture != runtime.GOARCH || config.InfraCommit != "557445ffddda8d9a27f6f529a3f4d7732cf81a13" || config.KernelVersion != "vmlinux-test" || config.FirecrackerVersion != "v1.14.1_test" || config.EnvdVersion != "0.6.10" || config.BusyboxVersion != "1.36.1" {
		t.Fatalf("RuntimeConfig = %#v", config)
	}
	if paths.Kernel == "" || paths.Firecracker == "" || paths.Envd == "" || paths.Busybox == "" || paths.KernelSHA256 == "" || paths.EnvdSHA256 == "" {
		t.Fatalf("runtime paths = %#v", paths)
	}
}

func TestLoadRuntimeInfoRejectsMissingAsset(t *testing.T) {
	path := writeRuntimeInfoWithAssets(t, []runtimeInfoAsset{{Type: "kernel", Version: "vmlinux-test", Path: "/runtime/kernel"}})
	_, _, err := loadRuntimeInfo(path)
	if err == nil || !strings.Contains(err.Error(), "firecracker") {
		t.Fatalf("loadRuntimeInfo() error = %v, want missing firecracker", err)
	}
}

func TestRunPreflightOnlyDoesNotInitializeCreator(t *testing.T) {
	called := false
	dependencies := dependencies{
		preflight: func(config preflight.Config) preflight.Report {
			return preflight.Report{Checks: []preflight.Check{{Name: "test", OK: true, Detail: config.ImageRef}}}
		},
		newCreator: func(context.Context, templatecreator.CreatorConfig) (demoCreator, error) {
			called = true
			return nil, errors.New("must not be called")
		},
	}
	var output bytes.Buffer
	err := run(context.Background(), coreArguments(t, "-preflight-only"), &output, dependencies)
	if err != nil {
		t.Fatal(err)
	}
	if called {
		t.Fatal("preflight-only initialized Creator")
	}
	var response commandOutput
	if err := json.Unmarshal(output.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if !response.Ready || response.Result != nil {
		t.Fatalf("response = %#v", response)
	}
}

func TestRunStopsBeforeCreatorWhenPreflightFails(t *testing.T) {
	called := false
	dependencies := dependencies{
		preflight: func(preflight.Config) preflight.Report {
			return preflight.Report{Checks: []preflight.Check{{Name: "kvm", Detail: "missing"}}}
		},
		newCreator: func(context.Context, templatecreator.CreatorConfig) (demoCreator, error) {
			called = true
			return nil, nil
		},
	}
	var output bytes.Buffer
	err := run(context.Background(), coreArguments(t), &output, dependencies)
	if err == nil || !strings.Contains(err.Error(), "preflight") || called {
		t.Fatalf("run() error = %v, newCreator called = %v", err, called)
	}
}

func TestRunCreatesTemplateAndExportsByDefault(t *testing.T) {
	fake := &fakeCreator{result: templatecreator.CreateResult{TemplateID: "tmpl_demo", BuildID: "build-demo", Artifacts: templatecreator.ArtifactLocation{StoreURL: "file:///store", ManifestKey: "manifest.json"}}}
	var exportRequest templatecreator.ExportTemplateArtifactsRequest
	var creatorConfig templatecreator.CreatorConfig
	var output bytes.Buffer
	dependencies := dependencies{
		preflight: func(preflight.Config) preflight.Report {
			return preflight.Report{Checks: []preflight.Check{{Name: "all", OK: true, Detail: "ready"}}}
		},
		newCreator: func(_ context.Context, config templatecreator.CreatorConfig) (demoCreator, error) {
			creatorConfig = config
			return fake, nil
		},
		exportArtifacts: func(_ context.Context, request templatecreator.ExportTemplateArtifactsRequest) (templatecreator.ExportTemplateArtifactsResult, error) {
			exportRequest = request
			return fakeArtifactsResult(), nil
		},
	}
	err := run(context.Background(), coreArguments(t, "-idempotency-key", "stable"), &output, dependencies)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(exportRequest.OutputRoot, filepath.Join("store", "exports")) {
		t.Fatalf("default export output root = %q", exportRequest.OutputRoot)
	}
	if creatorConfig.Store.URL == "" || creatorConfig.Runtime.EnvdPath == "" || fake.request.ImageRef != "alpine:3.20" || fake.request.IdempotencyKey != "stable" || !fake.closed {
		t.Fatalf("CreatorConfig=%#v request=%#v closed=%v", creatorConfig, fake.request, fake.closed)
	}
	if creatorConfig.Cache.Enabled || fake.request.BypassBaseCache {
		t.Fatalf("cache config/request = %#v / %#v, want disabled infra 2026.09 cache", creatorConfig.Cache, fake.request)
	}
	var response commandOutput
	if err := json.Unmarshal(output.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Result == nil || response.Result.BuildID != "build-demo" {
		t.Fatalf("response = %#v", response)
	}
	if response.TemplateID != "tmpl_demo" || response.FinalID != "build-demo" {
		t.Fatalf("output IDs = %q/%q", response.TemplateID, response.FinalID)
	}
}

func TestRunUsesExplicitExportRoot(t *testing.T) {
	fake := &fakeCreator{result: templatecreator.CreateResult{TemplateID: "tmpl_demo", BuildID: "build-demo", Artifacts: templatecreator.ArtifactLocation{StoreURL: "file:///store", ManifestKey: "manifest.json"}}}
	var exportRequest templatecreator.ExportTemplateArtifactsRequest
	dependencies := dependencies{
		preflight: func(preflight.Config) preflight.Report {
			return preflight.Report{Checks: []preflight.Check{{Name: "all", OK: true}}}
		},
		newCreator: func(context.Context, templatecreator.CreatorConfig) (demoCreator, error) { return fake, nil },
		exportArtifacts: func(_ context.Context, request templatecreator.ExportTemplateArtifactsRequest) (templatecreator.ExportTemplateArtifactsResult, error) {
			exportRequest = request
			return fakeArtifactsResult(), nil
		},
	}
	var output bytes.Buffer
	if err := run(context.Background(), coreArguments(t, "-export-root", "/custom/exports"), &output, dependencies); err != nil {
		t.Fatal(err)
	}
	if exportRequest.OutputRoot != "/custom/exports" {
		t.Fatalf("export output root = %q", exportRequest.OutputRoot)
	}
}

func TestRunReturnsExportErrorAfterTemplateCreation(t *testing.T) {
	fake := &fakeCreator{result: templatecreator.CreateResult{TemplateID: "tmpl_demo", BuildID: "build-demo", Artifacts: templatecreator.ArtifactLocation{StoreURL: "file:///store", ManifestKey: "manifest.json"}}}
	want := errors.New("artifact export failed")
	dependencies := dependencies{
		preflight: func(preflight.Config) preflight.Report {
			return preflight.Report{Checks: []preflight.Check{{Name: "all", OK: true}}}
		},
		newCreator: func(context.Context, templatecreator.CreatorConfig) (demoCreator, error) { return fake, nil },
		exportArtifacts: func(context.Context, templatecreator.ExportTemplateArtifactsRequest) (templatecreator.ExportTemplateArtifactsResult, error) {
			return templatecreator.ExportTemplateArtifactsResult{}, want
		},
	}
	var output bytes.Buffer
	err := run(context.Background(), coreArguments(t, "-export-root", "/exports"), &output, dependencies)
	if !errors.Is(err, want) || !strings.Contains(err.Error(), "export") {
		t.Fatalf("run() error = %v, want export error", err)
	}
}

func TestRunPrintsExportPathsWhenProgressLogsAreDisabled(t *testing.T) {
	fake := &fakeCreator{result: templatecreator.CreateResult{TemplateID: "tmpl_demo", BuildID: "build-demo", Artifacts: templatecreator.ArtifactLocation{StoreURL: "file:///store", ManifestKey: "manifest.json"}}}
	var logs bytes.Buffer
	dependencies := dependencies{
		logOutput: &logs,
		preflight: func(preflight.Config) preflight.Report {
			return preflight.Report{Checks: []preflight.Check{{Name: "all", OK: true}}}
		},
		newCreator: func(context.Context, templatecreator.CreatorConfig) (demoCreator, error) { return fake, nil },
		exportArtifacts: func(context.Context, templatecreator.ExportTemplateArtifactsRequest) (templatecreator.ExportTemplateArtifactsResult, error) {
			return fakeArtifactsResult(), nil
		},
	}
	var output bytes.Buffer
	if err := run(context.Background(), coreArguments(t, "-log-level", logLevelOff, "-export-root", "/exports"), &output, dependencies); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(logs.String(), "Template 原始产物目录：/exports/tmpl_demo") {
		t.Fatalf("export paths missing with progress logs disabled: %q", logs.String())
	}
}

func TestRunPrintsRegistrationSQLNoticeWhenProgressLogsAreDisabled(t *testing.T) {
	fake := &fakeCreator{result: templatecreator.CreateResult{
		TemplateID: "tmpl_demo", BuildID: "build-demo",
		Registration: templatecreator.RegistrationSQLResult{
			LocalPath: "/store/build-demo/registration.sql",
			Warnings: []string{
				"team_id 当前为 REPLACE_WITH_TEAM_ID，执行前必须替换。",
				"namespace、cluster_node_id 和 template_alias 必要时修改。",
			},
		},
	}}
	var logs bytes.Buffer
	dependencies := dependencies{
		logOutput: &logs,
		preflight: func(preflight.Config) preflight.Report {
			return preflight.Report{Checks: []preflight.Check{{Name: "all", OK: true}}}
		},
		newCreator: func(context.Context, templatecreator.CreatorConfig) (demoCreator, error) { return fake, nil },
	}
	var output bytes.Buffer
	if err := run(context.Background(), coreArguments(t, "-log-level", logLevelOff), &output, dependencies); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"PostgreSQL 注册 SQL：/store/build-demo/registration.sql",
		"REPLACE_WITH_TEAM_ID",
		"必要时修改",
	} {
		if !strings.Contains(logs.String(), want) {
			t.Fatalf("registration SQL notice missing %q: %q", want, logs.String())
		}
	}
}

func TestRunWritesProgressToLogOutputAndKeepsStdoutJSON(t *testing.T) {
	fake := &fakeCreator{result: templatecreator.CreateResult{TemplateID: "tmpl_demo", BuildID: "build-demo"}}
	var creatorConfig templatecreator.CreatorConfig
	var logs bytes.Buffer
	dependencies := dependencies{
		logOutput: &logs,
		preflight: func(preflight.Config) preflight.Report {
			return preflight.Report{Checks: []preflight.Check{{Name: "all", OK: true, Detail: "ready"}}}
		},
		newCreator: func(_ context.Context, config templatecreator.CreatorConfig) (demoCreator, error) {
			creatorConfig = config
			return fake, nil
		},
	}
	var stdout bytes.Buffer
	if err := run(context.Background(), coreArguments(t, "-log-level", "debug"), &stdout, dependencies); err != nil {
		t.Fatal(err)
	}

	var response commandOutput
	if err := json.Unmarshal(stdout.Bytes(), &response); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\n%s", err, stdout.String())
	}
	if creatorConfig.LogWriter != &logs || creatorConfig.LogLevel != templatecreator.LogLevelDebug {
		t.Fatalf("CreatorConfig logging = writer %T level=%q", creatorConfig.LogWriter, creatorConfig.LogLevel)
	}
	for _, stage := range []string{
		"arguments_parsed", "runtime_info_loaded", "preflight_started", "preflight_passed",
		"creator_initialization_started", "creator_initialization_completed", "creator_close_started", "creator_close_completed",
	} {
		if !strings.Contains(logs.String(), stage) {
			t.Errorf("stderr log missing stage %q:\n%s", stage, logs.String())
		}
	}
	if strings.Contains(stdout.String(), "arguments_parsed") {
		t.Fatalf("stdout was polluted by progress logs:\n%s", stdout.String())
	}
}

func coreArguments(t *testing.T, extra ...string) []string {
	t.Helper()
	root := t.TempDir()
	for _, directory := range []string{filepath.Join(root, "scratch"), filepath.Join(root, "store")} {
		if err := os.MkdirAll(directory, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	args := []string{
		"-runtime-info", writeRuntimeInfo(t),
		"-scratch-root", filepath.Join(root, "scratch"),
		"-store-root", filepath.Join(root, "store"),
		"-image", "alpine:3.20",
	}
	return append(args, extra...)
}

func writeRuntimeInfo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	assets := []runtimeInfoAsset{
		{Type: "kernel", Version: "vmlinux-test", Path: filepath.Join(root, "kernels", "vmlinux-test", runtime.GOARCH, "vmlinux.bin"), SHA256: strings.Repeat("1", 64)},
		{Type: "firecracker", Version: "v1.14.1_test", Path: filepath.Join(root, "firecrackers", "v1.14.1_test", runtime.GOARCH, "firecracker"), SHA256: strings.Repeat("2", 64)},
		{Type: "envd", Version: "0.6.10", Path: filepath.Join(root, "envd", "0.6.10", runtime.GOARCH, "envd"), SHA256: strings.Repeat("3", 64)},
		{Type: "busybox", Version: "1.36.1", Path: filepath.Join(root, "busybox", "1.36.1", runtime.GOARCH, "busybox"), SHA256: strings.Repeat("4", 64)},
	}
	return writeRuntimeInfoWithRoot(t, root, assets)
}

func writeRuntimeInfoWithAssets(t *testing.T, assets []runtimeInfoAsset) string {
	t.Helper()
	return writeRuntimeInfoWithRoot(t, "/runtime", assets)
}

func writeRuntimeInfoWithRoot(t *testing.T, root string, assets []runtimeInfoAsset) string {
	t.Helper()
	info := runtimeInfo{
		Architecture: runtime.GOARCH, InfraCommit: "557445ffddda8d9a27f6f529a3f4d7732cf81a13",
		TemplateVersion: "v2.1.0", RuntimeRoot: root, Assets: assets,
	}
	data, err := json.Marshal(info)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "runtime-info.json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}
