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

package proxy

import (
	"errors"
)

var (
	ErrInvalidHost      = errors.New("invalid url host")
	ErrInvalidSandboxID = errors.New("invalid sandbox ID")
)

type InvalidSandboxPortError struct {
	Port    string
	wrapped error
}

func (e InvalidSandboxPortError) Error() string {
	return "invalid sandbox port"
}

func (e InvalidSandboxPortError) Unwrap() error {
	return e.wrapped
}

type SandboxNotFoundError struct {
	SandboxId string
}

func NewErrSandboxNotFound(sandboxId string) *SandboxNotFoundError {
	return &SandboxNotFoundError{
		SandboxId: sandboxId,
	}
}

func (e SandboxNotFoundError) Error() string {
	return "sandbox not found"
}

type SandboxResumePermissionDeniedError struct {
	SandboxId string
}

func NewErrSandboxResumePermissionDenied(sandboxId string) *SandboxResumePermissionDeniedError {
	return &SandboxResumePermissionDeniedError{
		SandboxId: sandboxId,
	}
}

func (e SandboxResumePermissionDeniedError) Error() string {
	return "sandbox resume permission denied"
}

type MissingTrafficAccessTokenError struct {
	SandboxId string
	Header    string
}

func (e MissingTrafficAccessTokenError) Error() string {
	return "missing traffic access token"
}

func NewErrMissingTrafficAccessToken(sandboxId, header string) *MissingTrafficAccessTokenError {
	return &MissingTrafficAccessTokenError{
		SandboxId: sandboxId,
		Header:    header,
	}
}

type InvalidTrafficAccessTokenError struct {
	SandboxId string
	Header    string
}

func (e InvalidTrafficAccessTokenError) Error() string {
	return "invalid traffic access token"
}

func NewErrInvalidTrafficAccessToken(sandboxId string, header string) *InvalidTrafficAccessTokenError {
	return &InvalidTrafficAccessTokenError{
		SandboxId: sandboxId,
		Header:    header,
	}
}
