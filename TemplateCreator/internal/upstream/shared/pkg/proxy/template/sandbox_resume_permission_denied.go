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

package template

import (
	_ "embed"
	"html/template"
	"net/http"
)

//go:embed browser_sandbox_resume_permission_denied.html
var sandboxResumePermissionDeniedHtml string
var sandboxResumePermissionDeniedHtmlTemplate = template.Must(template.New("sandboxResumePermissionDeniedHtml").Parse(sandboxResumePermissionDeniedHtml))

type sandboxResumePermissionDeniedData struct {
	SandboxId string `json:"sandboxId"`
	Message   string `json:"message"`
	Code      int    `json:"code"`
	Host      string `json:"-"`
}

func (e sandboxResumePermissionDeniedData) StatusCode() int {
	return e.Code
}

func NewSandboxResumePermissionDeniedError(sandboxId, host string) *TemplatedError[sandboxResumePermissionDeniedData] {
	return &TemplatedError[sandboxResumePermissionDeniedData]{
		template: sandboxResumePermissionDeniedHtmlTemplate,
		vars: sandboxResumePermissionDeniedData{
			SandboxId: sandboxId,
			Message:   "This sandbox can't be resumed with the credentials provided. Check your access token and try again.",
			Host:      host,
			Code:      http.StatusForbidden,
		},
	}
}
