package templatecreator

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestBuildErrorUnwrapsCause(t *testing.T) {
	cause := errors.New("builder failed")
	err := &BuildError{
		TemplateID: "template-1",
		BuildID:    "build-1",
		Phase:      BuildPhaseBuild,
		Cause:      cause,
	}

	if !errors.Is(err, cause) {
		t.Fatalf("errors.Is(%v, cause) = false, want true", err)
	}
}

func TestBuildErrorPreservesContextCanceled(t *testing.T) {
	err := &BuildError{Phase: BuildPhaseBuild, Cause: context.Canceled}

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("errors.Is(%v, context.Canceled) = false, want true", err)
	}
}

func TestBuildErrorPreservesDeadlineExceeded(t *testing.T) {
	err := &BuildError{Phase: BuildPhaseBuild, Cause: context.DeadlineExceeded}

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("errors.Is(%v, context.DeadlineExceeded) = false, want true", err)
	}
}

func TestBuildErrorFormatsPhaseAndIDsWithoutCredentials(t *testing.T) {
	err := &BuildError{
		TemplateID:          "template-1",
		BuildID:             "build-1",
		Phase:               BuildPhasePublishManifest,
		DiagnosticsLocation: "file:///tmp/diagnostics/build-1",
		Cause:               errors.New("write failed"),
	}

	message := err.Error()
	for _, want := range []string{"template-1", "build-1", "publish_manifest", "write failed"} {
		if !strings.Contains(message, want) {
			t.Fatalf("BuildError.Error() = %q, want %q", message, want)
		}
	}
	for _, secretField := range []string{"Password", "Username", "RegistryAuth"} {
		if strings.Contains(message, secretField) {
			t.Fatalf("BuildError.Error() = %q, unexpectedly contains %q", message, secretField)
		}
	}
}

func TestBuildErrorWithoutCauseStillDescribesFailure(t *testing.T) {
	err := &BuildError{BuildID: "build-1", Phase: BuildPhaseValidate}

	if got := err.Error(); !strings.Contains(got, "build-1") || !strings.Contains(got, "validate") {
		t.Fatalf("BuildError.Error() = %q, want build and phase", got)
	}
}
