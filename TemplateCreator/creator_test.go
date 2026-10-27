package templatecreator

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"

	buildruntime "gitcode.com/cloud-native/cloud-native/TemplateCreator/internal/runtime"
	upstreamstorage "gitcode.com/cloud-native/cloud-native/TemplateCreator/internal/upstream/shared/pkg/storage"
	upstreamheader "gitcode.com/cloud-native/cloud-native/TemplateCreator/internal/upstream/shared/pkg/storage/header"
)

type fakeCreatorRuntime struct {
	mu         sync.Mutex
	closeCalls int
	closeErr   error
	jobRequest buildruntime.JobRequest
	jobResult  buildruntime.JobResult
	jobErr     error
	buildFn    func(buildruntime.JobRequest) (buildruntime.JobResult, error)
}

type staticRegistryAuth struct{ password string }

func (provider staticRegistryAuth) Credentials(context.Context, string) (RegistryCredentials, error) {
	return RegistryCredentials{Username: "user", Password: provider.password}, nil
}

func (runtime *fakeCreatorRuntime) Close(context.Context) error {
	runtime.closeCalls++
	return runtime.closeErr
}

func (runtime *fakeCreatorRuntime) BuildJob(_ context.Context, request buildruntime.JobRequest) (buildruntime.JobResult, error) {
	runtime.mu.Lock()
	runtime.jobRequest = request
	runtime.mu.Unlock()
	if runtime.buildFn != nil {
		result, err := runtime.buildFn(request)
		if result.TemplateRoot == "" {
			result.TemplateRoot = filepath.Join(request.JobRoot, "templates")
		}
		return result, err
	}
	result := runtime.jobResult
	if result.TemplateRoot == "" {
		result.TemplateRoot = filepath.Join(request.JobRoot, "templates")
	}
	return result, runtime.jobErr
}

func TestNewCreatorWithFactoryNormalizesConfig(t *testing.T) {
	cfg := validCreatorConfig(t)
	cfg.MaxParallel = 0
	fake := &fakeCreatorRuntime{}
	var factoryLogger any
	creator, err := newCreatorWithFactory(context.Background(), cfg, func(_ context.Context, got CreatorConfig) (creatorRuntime, error) {
		if got.MaxParallel != 1 {
			t.Fatalf("factory MaxParallel = %d, want 1", got.MaxParallel)
		}
		factoryLogger = got.progressLogger
		return fake, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if creator == nil {
		t.Fatal("creator = nil")
	}
	if factoryLogger == nil || factoryLogger != creator.logger {
		t.Fatalf("factory logger = %T, Creator logger = %T, want same instance", factoryLogger, creator.logger)
	}
}

func TestNewCreatorWithFactoryAppliesCentralDefaults(t *testing.T) {
	cfg := validCreatorConfig(t)
	cfg.ScratchRoot = ""
	cfg.Store.URL = ""
	cfg.Defaults = BuildSpec{}
	cfg.Format = ""
	cfg.Publication = ""
	fake := &fakeCreatorRuntime{}
	creator, err := newCreatorWithFactory(context.Background(), cfg, func(_ context.Context, got CreatorConfig) (creatorRuntime, error) {
		if got.ScratchRoot != DefaultScratchRoot || got.Store.URL != DefaultStoreURL() {
			t.Fatalf("default roots = scratch=%q store=%q", got.ScratchRoot, got.Store.URL)
		}
		if got.Defaults.VCPU != DefaultVCPU || got.Defaults.MemoryMB != DefaultMemoryMB || got.Defaults.DiskSizeMB != DefaultDiskSizeMB {
			t.Fatalf("default build resources = %#v", got.Defaults)
		}
		if got.Format != DefaultBuildFormat || got.Publication != DefaultPublicationMode {
			t.Fatalf("default profiles = format=%q publication=%q", got.Format, got.Publication)
		}
		return fake, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if creator == nil {
		t.Fatal("creator = nil")
	}
}

func TestProcessConfigFromCreatorConfigMapsLogging(t *testing.T) {
	var output bytes.Buffer
	cfg := validCreatorConfig(t)
	cfg.LogWriter = &output
	cfg.LogLevel = LogLevelDebug
	cfg = normalizeCreatorConfig(cfg)

	processConfig := processConfigFromCreatorConfig(cfg)
	if processConfig.ScratchRoot != cfg.ScratchRoot || processConfig.ProxyPort != cfg.Runtime.ProxyPort {
		t.Fatalf("process config paths = %#v", processConfig)
	}
	if processConfig.LogWriter != &output || !processConfig.DebugLogs {
		t.Fatalf("process config logging = %#v", processConfig)
	}
}

func TestProcessConfigFromCreatorConfigMapsBaseCache(t *testing.T) {
	cfg := validCreatorConfig(t)
	cfg.Cache = CacheConfig{Enabled: true, Root: t.TempDir(), AllowMutableImageTags: true}

	processConfig := processConfigFromCreatorConfig(cfg)
	if !processConfig.BaseCache.Enabled || processConfig.BaseCache.Root != cfg.Cache.Root || !processConfig.BaseCache.AllowMutableImageTags {
		t.Fatalf("process base cache config = %#v", processConfig.BaseCache)
	}
	abi := processConfig.BaseCache.ABI
	if abi.InfraCommit != cfg.Runtime.InfraCommit || abi.Architecture != cfg.Runtime.Architecture ||
		abi.KernelVersion != cfg.Runtime.KernelVersion || abi.EnvdVersion != cfg.Runtime.EnvdVersion || abi.BusyboxVersion != cfg.Runtime.BusyboxVersion {
		t.Fatalf("process base cache ABI = %#v", abi)
	}
}

func TestCreatorBuildTemplateMapsRequestAndResult(t *testing.T) {
	fake := &fakeCreatorRuntime{jobResult: buildruntime.JobResult{
		EnvdVersion: "0.6.10", KernelVersion: "vmlinux-test", FirecrackerVersion: "v1.14.1_test", RootfsSizeMB: 2048,
	}}
	creator, err := newCreatorWithFactory(context.Background(), validCreatorConfig(t), func(context.Context, CreatorConfig) (creatorRuntime, error) {
		return fake, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	request := CreateRequest{ImageRef: "docker.io/library/alpine:3.20", BypassBaseCache: true}
	outcome, err := creator.buildTemplate(context.Background(), request, "tmpl_fixed", "build-fixed")
	if err != nil {
		t.Fatal(err)
	}
	if fake.jobRequest.TemplateID != "tmpl_fixed" || fake.jobRequest.BuildID != "build-fixed" || fake.jobRequest.ImageRef != request.ImageRef {
		t.Fatalf("job request = %#v", fake.jobRequest)
	}
	if !fake.jobRequest.BypassBaseCache {
		t.Fatal("BuildJob request did not receive BypassBaseCache")
	}
	if fake.jobRequest.JobRoot == "" || fake.jobRequest.TemplateVersion != creator.config.Runtime.TemplateVersion {
		t.Fatalf("job paths/runtime = %#v", fake.jobRequest)
	}
	if outcome.Metadata.EnvdVersion != "0.6.10" || outcome.Metadata.RootfsSizeMB != 2048 {
		t.Fatalf("outcome = %#v", outcome)
	}
}

func TestCreatorBuildTemplateRejectsRuntimeABIMismatch(t *testing.T) {
	fake := &fakeCreatorRuntime{jobResult: buildruntime.JobResult{
		EnvdVersion: "0.0.0-wrong", KernelVersion: "vmlinux-test", FirecrackerVersion: "v1.14.1_test",
	}}
	creator, err := newCreatorWithFactory(context.Background(), validCreatorConfig(t), func(context.Context, CreatorConfig) (creatorRuntime, error) { return fake, nil })
	if err != nil {
		t.Fatal(err)
	}
	_, err = creator.buildTemplate(context.Background(), CreateRequest{ImageRef: "alpine"}, "tmpl_fixed", "build-fixed")
	if err == nil || !strings.Contains(err.Error(), "envd") {
		t.Fatalf("buildTemplate() error = %v, want envd ABI mismatch", err)
	}
}

func TestCreateTemplatePublishesManifestAfterCompleteObjectClosure(t *testing.T) {
	cfg := validCreatorConfig(t)
	var logs bytes.Buffer
	cfg.LogWriter = &logs
	cfg.DebugLogs = true
	storeRoot := t.TempDir()
	cfg.Store.URL = "file://" + storeRoot
	fake := &fakeCreatorRuntime{}
	fake.buildFn = func(request buildruntime.JobRequest) (buildruntime.JobResult, error) {
		writeFakeTemplateArtifacts(t, filepath.Join(request.JobRoot, "templates"), request.BuildID)
		return buildruntime.JobResult{EnvdVersion: "0.6.10", KernelVersion: request.KernelVersion, FirecrackerVersion: request.FirecrackerVersion, RootfsSizeMB: 2048}, nil
	}
	creator, err := newCreatorWithFactory(context.Background(), cfg, func(context.Context, CreatorConfig) (creatorRuntime, error) { return fake, nil })
	if err != nil {
		t.Fatal(err)
	}

	result, err := creator.CreateTemplate(context.Background(), CreateRequest{
		ImageRef: "docker.io/library/alpine:3.20", IdempotencyKey: "task-registration-v1",
		Labels: map[string]string{"purpose": "test"},
	})
	if err != nil {
		t.Fatalf("CreateTemplate() error = %v", err)
	}
	manifestPath := filepath.Join(storeRoot, filepath.FromSlash(result.Artifacts.ManifestKey))
	content, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	var manifest ArtifactManifest
	if err := json.Unmarshal(content, &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.TemplateID != result.TemplateID || manifest.BuildID != result.BuildID || len(manifest.Objects) != 6 {
		t.Fatalf("manifest = %#v", manifest)
	}
	if manifest.IdempotencyDigest == "" {
		t.Fatal("manifest idempotency digest is empty")
	}
	for _, object := range manifest.Objects {
		if object.Key == result.Registration.ObjectKey {
			t.Fatalf("registration.sql must not enter completion manifest: %#v", object)
		}
		if _, err := os.Stat(filepath.Join(storeRoot, filepath.FromSlash(object.Key))); err != nil {
			t.Fatalf("published object %s: %v", object.Key, err)
		}
	}
	if result.Registration.ObjectKey != filepath.ToSlash(filepath.Join(result.BuildID, "registration.sql")) {
		t.Fatalf("Registration = %#v", result.Registration)
	}
	registrationContent, err := os.ReadFile(filepath.Join(storeRoot, result.BuildID, "registration.sql"))
	if err != nil {
		t.Fatalf("read registration.sql: %v", err)
	}
	if !strings.Contains(string(registrationContent), `\set template_alias 'task-registration-v1'`) {
		t.Fatalf("registration.sql does not contain task alias:\n%s", registrationContent)
	}
	for _, stage := range []string{
		"creator_initialized", "build_slot_waiting", "build_slot_acquired", "identifiers_allocated",
		"builder_started", "builder_completed", "runtime_abi_validated", "closure_resolve_started",
		"closure_resolve_completed", "publication_started", "publication_completed", "job_cleanup_started", "job_cleanup_completed",
	} {
		if !strings.Contains(logs.String(), stage) {
			t.Errorf("Creator log missing stage %q:\n%s", stage, logs.String())
		}
	}
}

func TestCreateTemplatePublicationAllKeepsStagingObjects(t *testing.T) {
	cfg := validCreatorConfig(t)
	storeRoot := t.TempDir()
	cfg.Store.URL = "file://" + storeRoot
	fake := &fakeCreatorRuntime{buildFn: func(request buildruntime.JobRequest) (buildruntime.JobResult, error) {
		root := filepath.Join(request.JobRoot, "templates")
		writeFakeTemplateArtifacts(t, root, request.BuildID)
		ancestor := uuid.NewString()
		writeTestFile(t, root, filepath.Join(ancestor, "rootfs.ext4"), []byte("historical"))
		writeTestFile(t, root, filepath.Join(ancestor, "metadata.json"), []byte(`{"historical":true}`))
		return buildruntime.JobResult{EnvdVersion: "0.6.10", KernelVersion: request.KernelVersion, FirecrackerVersion: request.FirecrackerVersion}, nil
	}}
	creator, err := newCreatorWithFactory(context.Background(), cfg, func(context.Context, CreatorConfig) (creatorRuntime, error) { return fake, nil })
	if err != nil {
		t.Fatal(err)
	}

	result, err := creator.CreateTemplate(context.Background(), CreateRequest{Publication: PublicationAll})
	if err != nil {
		t.Fatal(err)
	}
	manifestBytes, err := os.ReadFile(filepath.Join(storeRoot, filepath.FromSlash(result.Artifacts.ManifestKey)))
	if err != nil {
		t.Fatal(err)
	}
	var manifest ArtifactManifest
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		t.Fatal(err)
	}
	var foundHistorical bool
	for _, object := range manifest.Objects {
		if strings.HasSuffix(object.Key, "/metadata.json") && !strings.HasPrefix(object.Key, result.BuildID+"/") {
			foundHistorical = true
		}
	}
	if !foundHistorical {
		t.Fatalf("publication=all did not retain staging object: %#v", manifest.Objects)
	}
}

func TestCreateTemplateWritesResultInfoFile(t *testing.T) {
	cfg := validCreatorConfig(t)
	storeRoot := t.TempDir()
	cfg.Store.URL = "file://" + storeRoot
	fake := &fakeCreatorRuntime{buildFn: func(request buildruntime.JobRequest) (buildruntime.JobResult, error) {
		writeFakeTemplateArtifacts(t, filepath.Join(request.JobRoot, "templates"), request.BuildID)
		return buildruntime.JobResult{EnvdVersion: "0.6.10", KernelVersion: request.KernelVersion, FirecrackerVersion: request.FirecrackerVersion}, nil
	}}
	creator, err := newCreatorWithFactory(context.Background(), cfg, func(context.Context, CreatorConfig) (creatorRuntime, error) { return fake, nil })
	if err != nil {
		t.Fatal(err)
	}

	result, err := creator.CreateTemplate(context.Background(), CreateRequest{ImageRef: "alpine:3.20"})
	if err != nil {
		t.Fatalf("CreateTemplate() error = %v", err)
	}
	infoPath := filepath.Join(storeRoot, result.TemplateID, "info.json")
	content, err := os.ReadFile(infoPath)
	if err != nil {
		t.Fatalf("read Template info.json: %v", err)
	}
	if bytes.Contains(content, []byte(`"SQL"`)) {
		t.Fatalf("info.json must not embed registration SQL:\n%s", content)
	}
	var got CreateResult
	if err := json.Unmarshal(content, &got); err != nil {
		t.Fatalf("decode Template info.json: %v", err)
	}
	wantJSON, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	gotJSON, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(gotJSON, wantJSON) {
		t.Fatalf("info.json result = %s, want %s", gotJSON, wantJSON)
	}
}

func TestNewCreatorRejectsSharedCacheForInfra202609(t *testing.T) {
	cfg := validCreatorConfig(t)
	cacheRoot := t.TempDir()
	cfg.Cache = CacheConfig{Enabled: true, Root: cacheRoot}
	factoryCalled := false
	_, err := newCreatorWithFactory(context.Background(), cfg, func(context.Context, CreatorConfig) (creatorRuntime, error) {
		factoryCalled = true
		return &fakeCreatorRuntime{}, nil
	})
	if err == nil || !strings.Contains(err.Error(), "infra 2026.09") {
		t.Fatalf("newCreatorWithFactory() error = %v, want unsupported infra 2026.09 cache error", err)
	}
	if factoryCalled {
		t.Fatal("runtime factory called for unsupported shared cache configuration")
	}
}

func TestCreatorRejectsTemplateRootOutsideOwnedStorage(t *testing.T) {
	cfg := validCreatorConfig(t)
	fake := &fakeCreatorRuntime{buildFn: func(request buildruntime.JobRequest) (buildruntime.JobResult, error) {
		return buildruntime.JobResult{
			EnvdVersion: "0.6.10", KernelVersion: request.KernelVersion,
			FirecrackerVersion: request.FirecrackerVersion, RootfsSizeMB: 2048,
			TemplateRoot: filepath.Join(t.TempDir(), "foreign"),
		}, nil
	}}
	creator, err := newCreatorWithFactory(context.Background(), cfg, func(context.Context, CreatorConfig) (creatorRuntime, error) { return fake, nil })
	if err != nil {
		t.Fatal(err)
	}
	_, err = creator.buildTemplate(context.Background(), CreateRequest{ImageRef: "docker.io/library/debian:bookworm-slim"}, "tmpl_fixed", "build-fixed")
	if err == nil || !strings.Contains(err.Error(), "TemplateRoot") {
		t.Fatalf("buildTemplate() error = %v, want owned TemplateRoot failure", err)
	}
}

func TestCreateTemplatePublishesMachineInfo(t *testing.T) {
	cfg := validCreatorConfig(t)
	storeRoot := t.TempDir()
	cfg.Store.URL = "file://" + storeRoot
	fake := &fakeCreatorRuntime{buildFn: func(request buildruntime.JobRequest) (buildruntime.JobResult, error) {
		writeFakeTemplateArtifacts(t, filepath.Join(request.JobRoot, "templates"), request.BuildID)
		return buildruntime.JobResult{
			EnvdVersion: "0.6.10", KernelVersion: request.KernelVersion,
			FirecrackerVersion: request.FirecrackerVersion, RootfsSizeMB: 2048,
		}, nil
	}}
	creator, err := newCreatorWithFactory(context.Background(), cfg, func(context.Context, CreatorConfig) (creatorRuntime, error) { return fake, nil })
	if err != nil {
		t.Fatal(err)
	}
	expected := MachineInfo{
		CPUArchitecture: "amd64", CPUFamily: "6", CPUModel: "165",
		CPUModelName: "Intel Test CPU", CPUFlags: []string{},
	}
	creator.machineInfo = expected

	result, err := creator.CreateTemplate(context.Background(), CreateRequest{ImageRef: "alpine:3.20"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Metadata.MachineInfo.CPUModel != expected.CPUModel {
		t.Fatalf("result machine info = %#v, want %#v", result.Metadata.MachineInfo, expected)
	}

	content, err := os.ReadFile(filepath.Join(storeRoot, filepath.FromSlash(result.Artifacts.ManifestKey)))
	if err != nil {
		t.Fatal(err)
	}
	var manifest ArtifactManifest
	if err := json.Unmarshal(content, &manifest); err != nil {
		t.Fatal(err)
	}
	var metadata BuildMetadata
	if err := json.Unmarshal(manifest.Metadata, &metadata); err != nil {
		t.Fatal(err)
	}
	if metadata.MachineInfo.CPUArchitecture != expected.CPUArchitecture || metadata.MachineInfo.CPUFlags == nil {
		t.Fatalf("manifest machine info = %#v, want %#v", metadata.MachineInfo, expected)
	}
}

func TestCreateTemplateIdempotencyReusesCompletedResult(t *testing.T) {
	cfg := validCreatorConfig(t)
	var logs bytes.Buffer
	cfg.LogWriter = &logs
	cfg.Store.URL = "file://" + t.TempDir()
	builds := 0
	fake := &fakeCreatorRuntime{buildFn: func(request buildruntime.JobRequest) (buildruntime.JobResult, error) {
		builds++
		writeFakeTemplateArtifacts(t, filepath.Join(request.JobRoot, "templates"), request.BuildID)
		return buildruntime.JobResult{EnvdVersion: "0.6.10", KernelVersion: request.KernelVersion, FirecrackerVersion: request.FirecrackerVersion}, nil
	}}
	creator, err := newCreatorWithFactory(context.Background(), cfg, func(context.Context, CreatorConfig) (creatorRuntime, error) { return fake, nil })
	if err != nil {
		t.Fatal(err)
	}
	request := CreateRequest{ImageRef: "alpine:3.20", IdempotencyKey: "same-request"}
	first, err := creator.CreateTemplate(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	second, err := creator.CreateTemplate(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if first.BuildID != second.BuildID || builds != 1 {
		t.Fatalf("first=%#v second=%#v builds=%d", first, second, builds)
	}
	content, err := os.ReadFile(filepath.Join(strings.TrimPrefix(cfg.Store.URL, "file://"), filepath.FromSlash(first.Artifacts.ManifestKey)))
	if err != nil {
		t.Fatal(err)
	}
	var manifest ArtifactManifest
	if err := json.Unmarshal(content, &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.IdempotencyDigest == "" {
		t.Fatal("manifest idempotency digest is empty")
	}
	if !strings.Contains(logs.String(), "idempotency_result_recorded") || !strings.Contains(logs.String(), "idempotency_result_reused") {
		t.Fatalf("idempotency stages missing from logs:\n%s", logs.String())
	}
}

func TestCreateTemplateIdempotencyRejectsConflict(t *testing.T) {
	cfg := validCreatorConfig(t)
	cfg.Store.URL = "file://" + t.TempDir()
	fake := &fakeCreatorRuntime{buildFn: func(request buildruntime.JobRequest) (buildruntime.JobResult, error) {
		writeFakeTemplateArtifacts(t, filepath.Join(request.JobRoot, "templates"), request.BuildID)
		return buildruntime.JobResult{EnvdVersion: "0.6.10", KernelVersion: request.KernelVersion, FirecrackerVersion: request.FirecrackerVersion}, nil
	}}
	creator, _ := newCreatorWithFactory(context.Background(), cfg, func(context.Context, CreatorConfig) (creatorRuntime, error) { return fake, nil })
	_, err := creator.CreateTemplate(context.Background(), CreateRequest{ImageRef: "alpine:3.20", IdempotencyKey: "key"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = creator.CreateTemplate(context.Background(), CreateRequest{ImageRef: "ubuntu:24.04", IdempotencyKey: "key"})
	if !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatalf("conflict error = %v", err)
	}
}

func TestIdempotencyDigestIgnoresBaseCacheBypass(t *testing.T) {
	creator := &Creator{config: normalizeCreatorConfig(validCreatorConfig(t))}
	request := CreateRequest{ImageRef: "docker.io/library/debian@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}

	withoutBypass, err := creator.idempotencyDigest(request)
	if err != nil {
		t.Fatal(err)
	}
	request.BypassBaseCache = true
	withBypass, err := creator.idempotencyDigest(request)
	if err != nil {
		t.Fatal(err)
	}
	if withoutBypass != withBypass {
		t.Fatalf("idempotency digest changed for cache bypass: %s != %s", withoutBypass, withBypass)
	}
}

func TestCreateTemplateIdempotencyDoesNotPersistRegistrySecret(t *testing.T) {
	storeRoot := t.TempDir()
	cfg := validCreatorConfig(t)
	cfg.Store.URL = "file://" + storeRoot
	fake := &fakeCreatorRuntime{buildFn: func(request buildruntime.JobRequest) (buildruntime.JobResult, error) {
		writeFakeTemplateArtifacts(t, filepath.Join(request.JobRoot, "templates"), request.BuildID)
		return buildruntime.JobResult{EnvdVersion: "0.6.10", KernelVersion: request.KernelVersion, FirecrackerVersion: request.FirecrackerVersion}, nil
	}}
	creator, _ := newCreatorWithFactory(context.Background(), cfg, func(context.Context, CreatorConfig) (creatorRuntime, error) { return fake, nil })
	secret := "registry-secret-must-not-persist"
	_, err := creator.CreateTemplate(context.Background(), CreateRequest{ImageRef: "private/image:tag", IdempotencyKey: "secret-test", RegistryAuth: staticRegistryAuth{password: secret}})
	if err != nil {
		t.Fatal(err)
	}
	err = filepath.WalkDir(storeRoot, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		content, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		if strings.Contains(string(content), secret) {
			return fmt.Errorf("secret found in %s", path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func writeFakeTemplateArtifacts(t *testing.T, root, buildID string) {
	t.Helper()
	id := uuid.MustParse(buildID)
	files := upstreamstorage.TemplateFiles{BuildID: buildID}
	for _, name := range []string{upstreamstorage.MemfileName, upstreamstorage.RootfsName} {
		data, err := upstreamheader.Serialize(
			&upstreamheader.Metadata{Version: 3, BlockSize: 4096, Size: 4096, BuildId: id, BaseBuildId: id},
			[]*upstreamheader.BuildMap{{Offset: 0, Length: 4096, BuildId: id}},
		)
		if err != nil {
			t.Fatal(err)
		}
		headerKey := files.StorageMemfileHeaderPath()
		dataKey := files.StorageMemfilePath()
		if name == upstreamstorage.RootfsName {
			headerKey = files.StorageRootfsHeaderPath()
			dataKey = files.StorageRootfsPath()
		}
		writeTestFile(t, root, headerKey, data)
		writeTestFile(t, root, dataKey, []byte("data"))
	}
	writeTestFile(t, root, files.StorageSnapfilePath(), []byte("snapshot"))
	writeTestFile(t, root, files.StorageMetadataPath(), []byte(`{"template":true}`))
}

func writeTestFile(t *testing.T, root, key string, data []byte) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(key))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestNewCreatorWithFactoryPreservesInitializationError(t *testing.T) {
	want := errors.New("runtime initialization failed")
	_, err := newCreatorWithFactory(context.Background(), validCreatorConfig(t), func(context.Context, CreatorConfig) (creatorRuntime, error) {
		return nil, want
	})
	if !errors.Is(err, want) {
		t.Fatalf("newCreatorWithFactory() error = %v, want %v", err, want)
	}
}

func TestNewCreatorWithFactoryStopsOnCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	called := false
	_, err := newCreatorWithFactory(ctx, validCreatorConfig(t), func(context.Context, CreatorConfig) (creatorRuntime, error) {
		called = true
		return &fakeCreatorRuntime{}, nil
	})
	if !errors.Is(err, context.Canceled) || called {
		t.Fatalf("error = %v, factory called = %v", err, called)
	}
}

func TestCreatorCloseIsIdempotent(t *testing.T) {
	fake := &fakeCreatorRuntime{}
	creator, err := newCreatorWithFactory(context.Background(), validCreatorConfig(t), func(context.Context, CreatorConfig) (creatorRuntime, error) {
		return fake, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := creator.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := creator.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	if fake.closeCalls != 1 {
		t.Fatalf("runtime close calls = %d, want 1", fake.closeCalls)
	}
}
