package templatecreator

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	buildruntime "gitcode.com/cloud-native/cloud-native/TemplateCreator/internal/runtime"
)

type deadlineCheckingRuntime struct{}

func (*deadlineCheckingRuntime) Close(ctx context.Context) error {
	if _, ok := ctx.Deadline(); !ok {
		return errors.New("runtime cleanup has no deadline")
	}
	return nil
}

func TestCreatorCloseUsesBoundedCleanupContext(t *testing.T) {
	creator, err := newCreatorWithFactory(context.Background(), validCreatorConfig(t), func(context.Context, CreatorConfig) (creatorRuntime, error) {
		return &deadlineCheckingRuntime{}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := creator.Close(context.Background()); err != nil {
		t.Fatalf("Close error = %v, want bounded cleanup context", err)
	}
}

func TestCreatorCanceledContextNeverStartsBuildWithFreeSlot(t *testing.T) {
	cfg := validCreatorConfig(t)
	var buildCalls atomic.Int32
	fake := &fakeCreatorRuntime{buildFn: func(buildruntime.JobRequest) (buildruntime.JobResult, error) {
		buildCalls.Add(1)
		return buildruntime.JobResult{}, context.Canceled
	}}
	creator, err := newCreatorWithFactory(context.Background(), cfg, func(context.Context, CreatorConfig) (creatorRuntime, error) { return fake, nil })
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	// context 和空 semaphore 同时就绪时，select 可能随机选择槽位；多次调用覆盖该竞争。
	for range 256 {
		_, err := creator.CreateTemplate(ctx, CreateRequest{ImageRef: "alpine"})
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("canceled CreateTemplate error = %v", err)
		}
	}
	if got := buildCalls.Load(); got != 0 {
		t.Fatalf("canceled requests started %d physical builds", got)
	}
}

func TestCreatorMaxParallelTwoBuildsOverlapAndThirdWaits(t *testing.T) {
	cfg := validCreatorConfig(t)
	cfg.MaxParallel = 2
	started := make(chan string, 3)
	release := make(chan struct{})
	fake := &fakeCreatorRuntime{buildFn: func(request buildruntime.JobRequest) (buildruntime.JobResult, error) {
		started <- request.BuildID
		<-release
		writeFakeTemplateArtifacts(t, filepath.Join(request.JobRoot, "templates"), request.BuildID)
		return buildruntime.JobResult{EnvdVersion: "0.6.10", KernelVersion: request.KernelVersion, FirecrackerVersion: request.FirecrackerVersion}, nil
	}}
	creator, err := newCreatorWithFactory(context.Background(), cfg, func(context.Context, CreatorConfig) (creatorRuntime, error) { return fake, nil })
	if err != nil {
		t.Fatal(err)
	}
	results := make(chan CreateResult, 3)
	errorsCh := make(chan error, 3)
	for range 3 {
		go func() {
			result, err := creator.CreateTemplate(context.Background(), CreateRequest{ImageRef: "alpine"})
			results <- result
			errorsCh <- err
		}()
	}
	first, second := <-started, <-started
	if first == second {
		t.Fatalf("simultaneous builds share BuildID %q", first)
	}
	select {
	case third := <-started:
		close(release)
		t.Fatalf("third build %q started before a slot was released", third)
	case <-time.After(50 * time.Millisecond):
	}
	close(release)
	seen := map[string]bool{}
	for range 3 {
		result := <-results
		if err := <-errorsCh; err != nil {
			t.Fatal(err)
		}
		if seen[result.BuildID] || result.Artifacts.ManifestKey == "" {
			t.Fatalf("duplicate or incomplete result: %#v", result)
		}
		seen[result.BuildID] = true
	}
	if len(seen) != 3 {
		t.Fatalf("completed BuildIDs = %v, want 3", seen)
	}
}

func TestCreatorConcurrentBuildFailureDoesNotCancelOtherBuild(t *testing.T) {
	cfg := validCreatorConfig(t)
	cfg.MaxParallel = 2
	started := make(chan struct{}, 2)
	release := make(chan struct{})
	buildErr := errors.New("one build failed")
	fake := &fakeCreatorRuntime{buildFn: func(request buildruntime.JobRequest) (buildruntime.JobResult, error) {
		started <- struct{}{}
		<-release
		if request.ImageRef == "broken" {
			return buildruntime.JobResult{}, buildErr
		}
		writeFakeTemplateArtifacts(t, filepath.Join(request.JobRoot, "templates"), request.BuildID)
		return buildruntime.JobResult{EnvdVersion: "0.6.10", KernelVersion: request.KernelVersion, FirecrackerVersion: request.FirecrackerVersion}, nil
	}}
	creator, err := newCreatorWithFactory(context.Background(), cfg, func(context.Context, CreatorConfig) (creatorRuntime, error) { return fake, nil })
	if err != nil {
		t.Fatal(err)
	}
	type outcome struct {
		image  string
		result CreateResult
		err    error
	}
	done := make(chan outcome, 2)
	for _, image := range []string{"broken", "working"} {
		go func() {
			result, err := creator.CreateTemplate(context.Background(), CreateRequest{ImageRef: image})
			done <- outcome{image: image, result: result, err: err}
		}()
	}
	<-started
	<-started
	close(release)
	for range 2 {
		got := <-done
		switch got.image {
		case "broken":
			if !errors.Is(got.err, buildErr) {
				t.Fatalf("broken build error = %v, want %v", got.err, buildErr)
			}
		case "working":
			if got.err != nil || got.result.Artifacts.ManifestKey == "" {
				t.Fatalf("working build result = %#v, error = %v", got.result, got.err)
			}
		}
	}
}

func TestCreatorCloseWaitsForActiveBuild(t *testing.T) {
	cfg := validCreatorConfig(t)
	started := make(chan struct{})
	release := make(chan struct{})
	fake := &fakeCreatorRuntime{buildFn: func(request buildruntime.JobRequest) (buildruntime.JobResult, error) {
		close(started)
		<-release
		writeFakeTemplateArtifacts(t, filepath.Join(request.JobRoot, "templates"), request.BuildID)
		return buildruntime.JobResult{EnvdVersion: "0.6.10", KernelVersion: request.KernelVersion, FirecrackerVersion: request.FirecrackerVersion}, nil
	}}
	creator, err := newCreatorWithFactory(context.Background(), cfg, func(context.Context, CreatorConfig) (creatorRuntime, error) { return fake, nil })
	if err != nil {
		t.Fatal(err)
	}
	buildDone := make(chan error, 1)
	go func() {
		_, err := creator.CreateTemplate(context.Background(), CreateRequest{ImageRef: "alpine"})
		buildDone <- err
	}()
	<-started
	closeDone := make(chan error, 1)
	go func() { closeDone <- creator.Close(context.Background()) }()
	select {
	case err := <-closeDone:
		close(release)
		<-buildDone
		t.Fatalf("Close returned while a build was active: %v", err)
	case <-time.After(50 * time.Millisecond):
	}
	close(release)
	if err := <-buildDone; err != nil {
		t.Fatal(err)
	}
	if err := <-closeDone; err != nil {
		t.Fatal(err)
	}
	if fake.closeCalls != 1 {
		t.Fatalf("runtime Close calls = %d, want 1", fake.closeCalls)
	}
}

func TestCreatorCloseTimeoutKeepsActiveBuildAndRejectsNewCalls(t *testing.T) {
	cfg := validCreatorConfig(t)
	started := make(chan struct{})
	release := make(chan struct{})
	fake := &fakeCreatorRuntime{buildFn: func(request buildruntime.JobRequest) (buildruntime.JobResult, error) {
		close(started)
		<-release
		writeFakeTemplateArtifacts(t, filepath.Join(request.JobRoot, "templates"), request.BuildID)
		return buildruntime.JobResult{EnvdVersion: "0.6.10", KernelVersion: request.KernelVersion, FirecrackerVersion: request.FirecrackerVersion}, nil
	}}
	creator, err := newCreatorWithFactory(context.Background(), cfg, func(context.Context, CreatorConfig) (creatorRuntime, error) { return fake, nil })
	if err != nil {
		t.Fatal(err)
	}
	buildDone := make(chan error, 1)
	go func() {
		_, err := creator.CreateTemplate(context.Background(), CreateRequest{ImageRef: "alpine"})
		buildDone <- err
	}()
	<-started
	closeCtx, cancelClose := context.WithCancel(context.Background())
	cancelClose()
	closeErr := creator.Close(closeCtx)
	callCtx, cancelCall := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancelCall()
	_, newCallErr := creator.CreateTemplate(callCtx, CreateRequest{ImageRef: "second"})
	close(release)
	buildErr := <-buildDone
	finalCloseErr := creator.Close(context.Background())
	if !errors.Is(closeErr, context.Canceled) {
		t.Fatalf("first Close error = %v, want canceled", closeErr)
	}
	if newCallErr == nil || !strings.Contains(newCallErr.Error(), "closed") {
		t.Fatalf("new call after Close error = %v, want creator closed", newCallErr)
	}
	if buildErr != nil || finalCloseErr != nil {
		t.Fatalf("active build error = %v, final Close error = %v", buildErr, finalCloseErr)
	}
	if fake.closeCalls != 1 {
		t.Fatalf("runtime Close calls = %d, want 1", fake.closeCalls)
	}
}

func TestCreatorMaxParallelLimitsPhysicalBuilds(t *testing.T) {
	cfg := validCreatorConfig(t)
	cfg.MaxParallel = 1
	cfg.Store.URL = "file://" + t.TempDir()
	started := make(chan struct{}, 2)
	release := make(chan struct{})
	var jobRoots []string
	fake := &fakeCreatorRuntime{buildFn: func(request buildruntime.JobRequest) (buildruntime.JobResult, error) {
		jobRoots = append(jobRoots, request.JobRoot)
		started <- struct{}{}
		<-release
		writeFakeTemplateArtifacts(t, filepath.Join(request.JobRoot, "templates"), request.BuildID)
		return buildruntime.JobResult{EnvdVersion: "0.6.10", KernelVersion: request.KernelVersion, FirecrackerVersion: request.FirecrackerVersion}, nil
	}}
	creator, _ := newCreatorWithFactory(context.Background(), cfg, func(context.Context, CreatorConfig) (creatorRuntime, error) { return fake, nil })
	var group sync.WaitGroup
	errorsCh := make(chan error, 2)
	for range 2 {
		group.Add(1)
		go func() {
			defer group.Done()
			_, err := creator.CreateTemplate(context.Background(), CreateRequest{ImageRef: "alpine"})
			errorsCh <- err
		}()
	}
	<-started
	select {
	case <-started:
		t.Fatal("second physical build started before first released semaphore")
	case <-time.After(50 * time.Millisecond):
	}
	close(release)
	group.Wait()
	close(errorsCh)
	for err := range errorsCh {
		if err != nil {
			t.Fatal(err)
		}
	}
	if len(jobRoots) != 2 || jobRoots[0] == jobRoots[1] {
		t.Fatalf("job roots = %v, want two isolated directories", jobRoots)
	}
}

func TestCreatorCanceledSemaphoreWaiterDoesNotBuild(t *testing.T) {
	cfg := validCreatorConfig(t)
	cfg.MaxParallel = 1
	cfg.Store.URL = "file://" + t.TempDir()
	started := make(chan struct{}, 2)
	release := make(chan struct{})
	fake := &fakeCreatorRuntime{buildFn: func(request buildruntime.JobRequest) (buildruntime.JobResult, error) {
		started <- struct{}{}
		<-release
		writeFakeTemplateArtifacts(t, filepath.Join(request.JobRoot, "templates"), request.BuildID)
		return buildruntime.JobResult{EnvdVersion: "0.6.10", KernelVersion: request.KernelVersion, FirecrackerVersion: request.FirecrackerVersion}, nil
	}}
	creator, _ := newCreatorWithFactory(context.Background(), cfg, func(context.Context, CreatorConfig) (creatorRuntime, error) { return fake, nil })
	firstDone := make(chan error, 1)
	go func() {
		_, err := creator.CreateTemplate(context.Background(), CreateRequest{ImageRef: "first"})
		firstDone <- err
	}()
	<-started
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := creator.CreateTemplate(ctx, CreateRequest{ImageRef: "second"})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("waiter error = %v", err)
	}
	if len(started) != 0 {
		t.Fatal("canceled waiter started a build")
	}
	close(release)
	if err := <-firstDone; err != nil {
		t.Fatal(err)
	}
}

func TestCreatorCleansJobDirectoryAfterBuildFailure(t *testing.T) {
	cfg := validCreatorConfig(t)
	cfg.Store.URL = "file://" + t.TempDir()
	var jobRoot string
	want := errors.New("physical build failed")
	fake := &fakeCreatorRuntime{buildFn: func(request buildruntime.JobRequest) (buildruntime.JobResult, error) {
		jobRoot = request.JobRoot
		if err := os.MkdirAll(jobRoot, 0o755); err != nil {
			t.Fatal(err)
		}
		return buildruntime.JobResult{}, want
	}}
	creator, _ := newCreatorWithFactory(context.Background(), cfg, func(context.Context, CreatorConfig) (creatorRuntime, error) { return fake, nil })
	_, err := creator.CreateTemplate(context.Background(), CreateRequest{ImageRef: "alpine"})
	if !errors.Is(err, want) {
		t.Fatalf("build error = %v", err)
	}
	if _, err := os.Stat(jobRoot); !os.IsNotExist(err) {
		t.Fatalf("job root stat = %v, want not exist", err)
	}
}

func TestCreatorJoinsBuildAndCleanupErrors(t *testing.T) {
	cfg := validCreatorConfig(t)
	cfg.Store.URL = "file://" + t.TempDir()
	buildErr := errors.New("build failed")
	cleanupErr := errors.New("cleanup failed")
	fake := &fakeCreatorRuntime{jobErr: buildErr}
	creator, _ := newCreatorWithFactory(context.Background(), cfg, func(context.Context, CreatorConfig) (creatorRuntime, error) { return fake, nil })
	creator.removeAll = func(string) error { return cleanupErr }
	_, err := creator.CreateTemplate(context.Background(), CreateRequest{ImageRef: "alpine"})
	if !errors.Is(err, buildErr) || !errors.Is(err, cleanupErr) {
		t.Fatalf("CreateTemplate() error = %v, want joined errors", err)
	}
}

func TestCreatorCleansJobDirectoryAfterPublishFailure(t *testing.T) {
	actualStore := t.TempDir()
	linkedStore := filepath.Join(t.TempDir(), "store-link")
	if err := os.Symlink(actualStore, linkedStore); err != nil {
		t.Fatal(err)
	}
	cfg := validCreatorConfig(t)
	cfg.Store.URL = "file://" + linkedStore
	var jobRoot string
	fake := &fakeCreatorRuntime{buildFn: func(request buildruntime.JobRequest) (buildruntime.JobResult, error) {
		jobRoot = request.JobRoot
		writeFakeTemplateArtifacts(t, filepath.Join(request.JobRoot, "templates"), request.BuildID)
		return buildruntime.JobResult{EnvdVersion: "0.6.10", KernelVersion: request.KernelVersion, FirecrackerVersion: request.FirecrackerVersion}, nil
	}}
	creator, err := newCreatorWithFactory(context.Background(), cfg, func(context.Context, CreatorConfig) (creatorRuntime, error) { return fake, nil })
	if err != nil {
		t.Fatal(err)
	}
	_, err = creator.CreateTemplate(context.Background(), CreateRequest{ImageRef: "alpine"})
	if err == nil {
		t.Fatal("CreateTemplate() error = nil, want publish failure")
	}
	if _, statErr := os.Stat(jobRoot); !os.IsNotExist(statErr) {
		t.Fatalf("job root stat = %v, want not exist", statErr)
	}
}
