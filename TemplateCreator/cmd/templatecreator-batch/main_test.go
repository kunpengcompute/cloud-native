package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	templatecreator "gitcode.com/boostkit/cloud-native.git/TemplateCreator"
	"gitcode.com/boostkit/cloud-native.git/TemplateCreator/internal/preflight"
)

func writeRequests(t *testing.T, contents string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "requests.json")
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestRedirectProcessStdoutKeepsBuilderOutputOutOfJSON(t *testing.T) {
	stdoutFile, err := os.CreateTemp(t.TempDir(), "stdout-*.txt")
	if err != nil {
		t.Fatal(err)
	}
	stderrFile, err := os.CreateTemp(t.TempDir(), "stderr-*.txt")
	if err != nil {
		t.Fatal(err)
	}
	oldStdout, oldStderr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = stdoutFile, stderrFile
	t.Cleanup(func() {
		os.Stdout, os.Stderr = oldStdout, oldStderr
		_ = stdoutFile.Close()
		_ = stderrFile.Close()
	})
	jsonOutput := redirectProcessStdout()
	_, _ = fmt.Fprintln(os.Stdout, "upstream builder message")
	_, _ = fmt.Fprintln(jsonOutput, `{"schema_version":1}`)
	outBytes, err := os.ReadFile(stdoutFile.Name())
	if err != nil {
		t.Fatal(err)
	}
	errBytes, err := os.ReadFile(stderrFile.Name())
	if err != nil {
		t.Fatal(err)
	}
	if string(outBytes) != "{\"schema_version\":1}\n" || string(errBytes) != "upstream builder message\n" {
		t.Fatalf("stdout=%q stderr=%q", outBytes, errBytes)
	}
}

func TestLoadRequestsStrictSchema(t *testing.T) {
	for name, content := range map[string]string{
		"unknown top field":    `{"schema_version":1,"requests":[{"request_id":"a","image_ref":"repo:tag"}],"extra":1}`,
		"unknown nested field": `{"schema_version":1,"requests":[{"request_id":"a","image_ref":"repo:tag","build_override":{"wat":1}}]}`,
		"wrong version":        `{"schema_version":2,"requests":[{"request_id":"a","image_ref":"repo:tag"}]}`,
		"duplicate id":         `{"schema_version":1,"requests":[{"request_id":"a","image_ref":"repo:tag"},{"request_id":"a","image_ref":"repo:tag"}]}`,
		"missing image":        `{"schema_version":1,"requests":[{"request_id":"a"}]}`,
		"two images":           `{"schema_version":1,"requests":[{"request_id":"a","image_ref":"repo:tag","image_tar_path":"/tmp/x"}]}`,
		"relative tar":         `{"schema_version":1,"requests":[{"request_id":"a","image_tar_path":"x.tar"}]}`,
		"empty batch":          `{"schema_version":1,"requests":[]}`,
		"trailing json":        `{"schema_version":1,"requests":[{"request_id":"a","image_ref":"repo:tag"}]} {}`,
		"duplicate JSON key":   `{"schema_version":1,"requests":[{"request_id":"a","request_id":"b","image_ref":"repo:tag"}]}`,
		"case alias duplicate": `{"schema_version":1,"requests":[{"request_id":"a","REQUEST_ID":"b","image_ref":"repo:tag"}]}`,
		"invalid UTF-8":        string([]byte{'{', '"', 's', 'c', 'h', 'e', 'm', 'a', '_', 'v', 'e', 'r', 's', 'i', 'o', 'n', '"', ':', '1', ',', '"', 'x', '"', ':', '"', 0xff, '"', '}'}),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := loadRequests(writeRequests(t, content)); err == nil {
				t.Fatal("accepted invalid request file")
			}
		})
	}
}

func TestLoadRequestsPreservesEmptyStepsAndHugePagesFalse(t *testing.T) {
	items, err := loadRequests(writeRequests(t, `{"schema_version":1,"requests":[{"request_id":"a","image_ref":"repo:tag","format":"fromDockerfile","publication":"simple","build_override":{"huge_pages":false,"steps":[]},"labels":{"env":"test"}}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Request.BuildOverride == nil || items[0].Request.BuildOverride.Steps == nil || len(items[0].Request.BuildOverride.Steps) != 0 || items[0].Request.Format != templatecreator.BuildFormatFromDockerfile || items[0].Request.Publication != templatecreator.PublicationSimple || items[0].Request.Labels["env"] != "test" {
		t.Fatalf("incorrect decoded request: %+v", items)
	}
}

type fakeCreator struct {
	create func(context.Context, templatecreator.CreateRequest) (templatecreator.CreateResult, error)
	close  func(context.Context) error
}

func (f *fakeCreator) CreateTemplate(ctx context.Context, req templatecreator.CreateRequest) (templatecreator.CreateResult, error) {
	return f.create(ctx, req)
}
func (f *fakeCreator) Close(ctx context.Context) error {
	if f.close != nil {
		return f.close(ctx)
	}
	return nil
}

func TestExecuteBatchBoundedAndOrdered(t *testing.T) {
	var active, peak atomic.Int32
	items := make([]batchItem, 5)
	for i := range items {
		items[i] = batchItem{RequestID: string(rune('a' + i)), Request: templatecreator.CreateRequest{ImageRef: string(rune('a' + i))}}
	}
	creator := &fakeCreator{create: func(ctx context.Context, req templatecreator.CreateRequest) (templatecreator.CreateResult, error) {
		n := active.Add(1)
		for {
			old := peak.Load()
			if old >= n || peak.CompareAndSwap(old, n) {
				break
			}
		}
		time.Sleep(20 * time.Millisecond)
		active.Add(-1)
		return templatecreator.CreateResult{TemplateID: req.ImageRef, BuildID: req.ImageRef}, nil
	}}
	var logs bytes.Buffer
	output := executeBatch(context.Background(), creator, items, 2, time.Second, "", nil, &logs)
	if peak.Load() != 2 || output.Succeeded != 5 || output.Failed != 0 {
		t.Fatalf("bad concurrency/counts: peak=%d output=%+v", peak.Load(), output)
	}
	for i, item := range output.Results {
		if item.RequestID != items[i].RequestID || item.Result == nil || item.Result.TemplateID != items[i].RequestID {
			t.Fatalf("result %d out of order: %+v", i, item)
		}
	}
	if !strings.Contains(logs.String(), `"request_id":"a"`) || !strings.Contains(logs.String(), `"waiting_requests":`) || !strings.Contains(logs.String(), `"active_workers":`) || !strings.Contains(logs.String(), `"finished_requests":`) || strings.Contains(logs.String(), `"occupied":`) {
		t.Fatalf("missing request/counter events: %q", logs.String())
	}
}

func TestExecuteBatchFailureTimeoutAndExport(t *testing.T) {
	items := []batchItem{{RequestID: "bad", Request: templatecreator.CreateRequest{ImageRef: "bad"}}, {RequestID: "slow", Request: templatecreator.CreateRequest{ImageRef: "slow"}}, {RequestID: "good", Request: templatecreator.CreateRequest{ImageRef: "good"}}}
	creator := &fakeCreator{create: func(ctx context.Context, req templatecreator.CreateRequest) (templatecreator.CreateResult, error) {
		if req.ImageRef == "bad" {
			return templatecreator.CreateResult{}, &templatecreator.BuildError{Phase: templatecreator.BuildPhaseBuild, BuildID: "bad-build", Cause: errors.New("boom")}
		}
		if req.ImageRef == "slow" {
			<-ctx.Done()
			return templatecreator.CreateResult{}, ctx.Err()
		}
		return templatecreator.CreateResult{TemplateID: "good-template", BuildID: "good-build", Artifacts: templatecreator.ArtifactLocation{ManifestKey: "manifest"}}, nil
	}}
	exporter := func(context.Context, templatecreator.ExportTemplateArtifactsRequest) (templatecreator.ExportTemplateArtifactsResult, error) {
		return templatecreator.ExportTemplateArtifactsResult{}, errors.New("export broken")
	}
	output := executeBatch(context.Background(), creator, items, 3, 10*time.Millisecond, "/tmp/export", exporter, nil)
	if output.Succeeded != 0 || output.Failed != 2 || output.Canceled != 1 {
		t.Fatalf("bad counts: %+v", output)
	}
	if output.Results[0].Stage != "build" || output.Results[0].BuildID != "bad-build" {
		t.Fatalf("lost BuildError details: %+v", output.Results[0])
	}
	if output.Results[1].Stage != "canceled" {
		t.Fatalf("timeout not canceled: %+v", output.Results[1])
	}
	if output.Results[2].Stage != "export" || output.Results[2].Result == nil || output.Results[2].Result.TemplateID != "good-template" {
		t.Fatalf("export lost published result: %+v", output.Results[2])
	}
}

func TestExecuteBatchCancellationMarksPending(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var calls atomic.Int32
	creator := &fakeCreator{create: func(ctx context.Context, _ templatecreator.CreateRequest) (templatecreator.CreateResult, error) {
		calls.Add(1)
		cancel()
		<-ctx.Done()
		return templatecreator.CreateResult{}, ctx.Err()
	}}
	items := []batchItem{{RequestID: "a"}, {RequestID: "b"}, {RequestID: "c"}}
	output := executeBatch(ctx, creator, items, 1, time.Second, "", nil, nil)
	if calls.Load() != 1 || output.Canceled != 3 {
		t.Fatalf("pending items were started or omitted: calls=%d output=%+v", calls.Load(), output)
	}
}

func TestRunPreflightFailureDoesNotInitializeCreator(t *testing.T) {
	path := writeRequests(t, `{"schema_version":1,"requests":[{"request_id":"a","image_ref":"repo:tag"}]}`)
	var initialized atomic.Bool
	deps := dependencies{
		loadRuntime: func(string) (templatecreator.RuntimeConfig, runtimePaths, error) {
			return templatecreator.RuntimeConfig{}, runtimePaths{}, nil
		},
		preflight: func(preflight.Config) preflight.Report {
			return preflight.Report{Checks: []preflight.Check{{Name: "kvm", OK: false}}}
		},
		newCreator: func(context.Context, templatecreator.CreatorConfig) (batchCreator, error) {
			initialized.Store(true)
			return nil, nil
		},
	}
	var stdout, stderr bytes.Buffer
	err := run(context.Background(), []string{"-requests", path, "-runtime-info", "/tmp/runtime.json", "-scratch-root", "/tmp/scratch", "-store-root", "/tmp/store"}, &stdout, &stderr, deps)
	if err == nil || initialized.Load() || stdout.Len() != 0 {
		t.Fatalf("preflight failure leaked into build: err=%v initialized=%t stdout=%q", err, initialized.Load(), stdout.String())
	}
}

func TestRunCapacityFailureDoesNotInitializeCreator(t *testing.T) {
	path := writeRequests(t, `{"schema_version":1,"requests":[{"request_id":"a","image_ref":"repo:tag","build_override":{"memory_mb":2048}},{"request_id":"b","image_ref":"repo:tag"}]}`)
	var initialized atomic.Bool
	deps := dependencies{
		loadRuntime: func(string) (templatecreator.RuntimeConfig, runtimePaths, error) {
			return templatecreator.RuntimeConfig{}, runtimePaths{}, nil
		},
		preflight: func(preflight.Config) preflight.Report {
			return preflight.Report{Checks: []preflight.Check{{Name: "fake", OK: true}}}
		},
		capacity: func(items []batchItem, parallel int) error {
			if parallel != 2 || items[0].Request.BuildOverride.MemoryMB != 2048 || items[1].Request.BuildOverride.MemoryMB != templatecreator.DefaultMemoryMB {
				t.Fatalf("wrong peak inputs: parallel=%d items=%+v", parallel, items)
			}
			return errors.New("insufficient capacity")
		},
		newCreator: func(context.Context, templatecreator.CreatorConfig) (batchCreator, error) {
			initialized.Store(true)
			return nil, nil
		},
	}
	var stdout, stderr bytes.Buffer
	err := run(context.Background(), []string{"-requests", path, "-runtime-info", "/tmp/runtime.json", "-scratch-root", "/tmp/scratch", "-store-root", "/tmp/store", "-max-parallel", "2"}, &stdout, &stderr, deps)
	if err == nil || !strings.Contains(err.Error(), "insufficient capacity") || initialized.Load() || stdout.Len() != 0 {
		t.Fatalf("capacity failure leaked into build: err=%v initialized=%t stdout=%q", err, initialized.Load(), stdout.String())
	}
}

func TestRunWritesSingleJSONAndReportsCloseFailure(t *testing.T) {
	path := writeRequests(t, `{"schema_version":1,"requests":[{"request_id":"a","image_ref":"repo:tag"}]}`)
	var mu sync.Mutex
	createCount := 0
	deps := dependencies{
		loadRuntime: func(string) (templatecreator.RuntimeConfig, runtimePaths, error) {
			return templatecreator.RuntimeConfig{}, runtimePaths{}, nil
		},
		preflight: func(preflight.Config) preflight.Report {
			return preflight.Report{Checks: []preflight.Check{{Name: "fake", OK: true}}}
		},
		capacity: func([]batchItem, int) error { return nil },
		newCreator: func(context.Context, templatecreator.CreatorConfig) (batchCreator, error) {
			return &fakeCreator{create: func(context.Context, templatecreator.CreateRequest) (templatecreator.CreateResult, error) {
				mu.Lock()
				createCount++
				mu.Unlock()
				return templatecreator.CreateResult{TemplateID: "t", BuildID: "b"}, nil
			}, close: func(context.Context) error { return errors.New("close broken") }}, nil
		},
	}
	var stdout, stderr bytes.Buffer
	err := run(context.Background(), []string{"-requests", path, "-runtime-info", "/tmp/runtime.json", "-scratch-root", "/tmp/scratch", "-store-root", "/tmp/store"}, &stdout, &stderr, deps)
	if err == nil || !strings.Contains(err.Error(), "close broken") || createCount != 1 || strings.Count(stdout.String(), "schema_version") != 1 || !strings.Contains(stdout.String(), "close broken") {
		t.Fatalf("bad close result: err=%v calls=%d stdout=%q", err, createCount, stdout.String())
	}
}
