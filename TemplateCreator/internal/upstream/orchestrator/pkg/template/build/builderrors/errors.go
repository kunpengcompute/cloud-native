// Copyright 2023 FoundryLabs, Inc.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// TemplateCreator modification: upstream import paths rewritten; no behavioral changes.

package builderrors

import (
	"context"
	"errors"

	"gitcode.com/cloud-native/cloud-native/TemplateCreator/internal/upstream/orchestrator/pkg/template/build/phases"
	template_manager "gitcode.com/cloud-native/cloud-native/TemplateCreator/internal/upstream/shared/pkg/grpc/template-manager"
)

const (
	InternalErrorMessage = "An internal error occurred. Please try again or contact support with the build ID."
)

// User errors
var (
	ErrCanceled = errors.New("build was cancelled")
	ErrTimeout  = errors.New("build timed out")
)

// IsUserError returns true if the error is a user error (i.e., a PhaseBuildError).
// User errors are caused by the user's configuration and should be shown to them.
func IsUserError(err error) bool {
	return phases.UnwrapPhaseBuildError(err) != nil
}

// WrapContextAsUserError wraps context.Canceled as a user error if no user error already exists.
// This ensures that user-initiated cancellations are tracked as user errors in metrics.
func WrapContextAsUserError(err error) error {
	if err == nil {
		return nil
	}

	// If it's already a user error, return as-is
	if IsUserError(err) {
		return err
	}

	// If it's a canceled context, wrap it as a user error
	if errors.Is(err, context.Canceled) {
		return phases.NewPhaseBuildError(phases.PhaseMeta{}, ErrCanceled)
	}

	// If it's a timeout context, wrap it as a user error
	if errors.Is(err, context.DeadlineExceeded) {
		return phases.NewPhaseBuildError(phases.PhaseMeta{}, ErrTimeout)
	}

	return err
}

func UnwrapUserError(err error) *template_manager.TemplateBuildStatusReason {
	phaseBuildError := phases.UnwrapPhaseBuildError(err)
	if phaseBuildError != nil {
		return &template_manager.TemplateBuildStatusReason{
			Message: phaseBuildError.Error(),
			Step:    &phaseBuildError.Step,
		}
	}

	return &template_manager.TemplateBuildStatusReason{
		Message: InternalErrorMessage,
	}
}
