//go:build linux

package runtime

import (
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap/zapcore"

	"gitcode.com/boostkit/cloud-native.git/TemplateCreator/internal/upstream/orchestrator/pkg/sandbox/network"
	build "gitcode.com/boostkit/cloud-native.git/TemplateCreator/internal/upstream/orchestrator/pkg/template/build"
	buildconfig "gitcode.com/boostkit/cloud-native.git/TemplateCreator/internal/upstream/orchestrator/pkg/template/build/config"
	templatemanager "gitcode.com/boostkit/cloud-native.git/TemplateCreator/internal/upstream/shared/pkg/grpc/template-manager"
	upstreamstorage "gitcode.com/boostkit/cloud-native.git/TemplateCreator/internal/upstream/shared/pkg/storage"
)

type recordingBuilder struct {
	files  upstreamstorage.TemplateFiles
	config buildconfig.TemplateConfig
	result *build.Result
	err    error
}

func TestBuilderConfigUsesPortableSandboxAndInfra202609RuntimeView(t *testing.T) {
	request := JobRequest{JobRoot: "/tmp/templatecreator/jobs/build-id", EnvdPath: "/opt/templatecreator/runtime/envd/envd"}
	view := infra202609RuntimeView{
		KernelsDir:             "/tmp/templatecreator/jobs/build-id/runtime-compat/kernels",
		FirecrackerVersionsDir: "/tmp/templatecreator/jobs/build-id/runtime-compat/firecrackers",
	}

	config := builderConfigForJob(request, network.Config{}, view)
	if config.SandboxDir != "/fc-vm" {
		t.Fatalf("SandboxDir = %q, want /fc-vm", config.SandboxDir)
	}
	if config.HostKernelsDir != view.KernelsDir || config.FirecrackerVersionsDir != view.FirecrackerVersionsDir {
		t.Fatalf("runtime view = kernels %q firecrackers %q", config.HostKernelsDir, config.FirecrackerVersionsDir)
	}
	if config.DefaultCacheDir != filepath.Join(request.JobRoot, "orchestrator", "build") {
		t.Fatalf("DefaultCacheDir = %q, want job-local cache", config.DefaultCacheDir)
	}
	if config.StorageConfig.SnapshotCacheDir != filepath.Join(request.JobRoot, "snapshot-cache") {
		t.Fatalf("SnapshotCacheDir = %q, want job-local cache", config.StorageConfig.SnapshotCacheDir)
	}
	if !config.AllowSandboxInternet {
		t.Fatal("AllowSandboxInternet = false, want infra 2026.09 local build default")
	}
	if config.EnvdTimeout != 10*time.Second {
		t.Fatalf("EnvdTimeout = %s, want 10s default", config.EnvdTimeout)
	}
}

func (builder *recordingBuilder) Build(_ context.Context, files upstreamstorage.TemplateFiles, config buildconfig.TemplateConfig, _ zapcore.Core) (*build.Result, error) {
	builder.files = files
	builder.config = config
	return builder.result, builder.err
}

func TestExecuteBuilderUsesInfra202609ContractAndBackfillsPinnedABI(t *testing.T) {
	builder := &recordingBuilder{result: &build.Result{EnvdVersion: "0.5.3", RootfsSizeMB: 2048}}
	request := validJobRequest()
	request.Steps = []JobStep{{Type: "RUN", Args: []string{"apt-get update", "root"}}}

	var logs bytes.Buffer
	core := zapcore.NewCore(zapcore.NewJSONEncoder(zapcore.EncoderConfig{MessageKey: "message", LevelKey: "level", EncodeLevel: zapcore.LowercaseLevelEncoder}), zapcore.AddSync(&logs), zapcore.InfoLevel)
	result, err := executeBuilder(context.Background(), builder, request, core)
	if err != nil {
		t.Fatal(err)
	}
	if builder.files.BuildID != request.BuildID {
		t.Fatalf("BuildID = %q, want %q", builder.files.BuildID, request.BuildID)
	}
	config := builder.config
	if config.FromImage != request.ImageRef || config.FromTemplate != nil {
		t.Fatalf("source config = %#v, want FromImage", config)
	}
	if config.Force == nil || !*config.Force || config.CacheScope == "" {
		t.Fatalf("Force/CacheScope = %v/%q", config.Force, config.CacheScope)
	}
	if config.Version != request.TemplateVersion || config.KernelVersion != request.KernelVersion || config.FirecrackerVersion != request.FirecrackerVersion {
		t.Fatalf("runtime ABI config = %#v", config)
	}
	if len(config.Steps) != 1 || config.Steps[0].Type != "RUN" {
		t.Fatalf("Steps = %#v", config.Steps)
	}
	if result.RootfsSizeMB != 2048 || result.EnvdVersion != "0.5.3" || result.KernelVersion != request.KernelVersion || result.FirecrackerVersion != request.FirecrackerVersion {
		t.Fatalf("result = %#v", result)
	}
	for _, stage := range []string{"upstream_builder_call_started", "upstream_builder_call_completed"} {
		if !strings.Contains(logs.String(), stage) {
			t.Errorf("executeBuilder log missing stage %q:\n%s", stage, logs.String())
		}
	}
}

func TestExecuteBuilderRejectsSharedBaseCache(t *testing.T) {
	request := validJobRequest()
	request.UseBaseCache = true
	_, err := executeBuilder(context.Background(), &recordingBuilder{}, request, zapcore.NewNopCore())
	if err == nil || !strings.Contains(err.Error(), "infra 2026.09") {
		t.Fatalf("executeBuilder() error = %v, want unsupported cache error", err)
	}
}

func TestExecuteBuilderPreservesCancellation(t *testing.T) {
	builder := &recordingBuilder{err: context.Canceled}
	_, err := executeBuilder(context.Background(), builder, validJobRequest(), zapcore.NewNopCore())
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("executeBuilder() error = %v, want context canceled", err)
	}
}

func TestExecuteBuilderRejectsNilResult(t *testing.T) {
	_, err := executeBuilder(context.Background(), &recordingBuilder{}, validJobRequest(), zapcore.NewNopCore())
	if err == nil || !strings.Contains(err.Error(), "nil result") {
		t.Fatalf("executeBuilder() error = %v, want nil result error", err)
	}
}

func TestConvertJobStepsCopiesInput(t *testing.T) {
	input := []JobStep{{Type: "RUN", Args: []string{"command", "root"}}}
	got := convertJobSteps(input)
	input[0].Args[0] = "mutated"
	if len(got) != 1 || got[0].Args[0] != "command" {
		t.Fatalf("convertJobSteps() = %#v", got)
	}
	var _ []*templatemanager.TemplateStep = got
}

func validJobRequest() JobRequest {
	return JobRequest{
		TemplateID: "tmpl_test", BuildID: "00000000-0000-0000-0000-000000000001",
		ImageRef:        "docker.io/library/debian@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		TemplateVersion: "v2.1.0", Architecture: "amd64", KernelVersion: "vmlinux-6.1.158",
		FirecrackerVersion: "v1.12.1_a41d3fb", VCPU: 2, MemoryMB: 1024, DiskSizeMB: 2048,
		TemplateRoot: "/tmp/templatecreator/templates",
	}
}
