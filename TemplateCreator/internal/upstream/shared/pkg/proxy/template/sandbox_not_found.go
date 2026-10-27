// Copyright (c) 2023-2026 e2b Devs Inc.
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

//go:embed browser_sandbox_not_found.html
var sandboxNotFoundHtml string
var sandboxNotFoundHtmlTemplate = template.Must(template.New("sandboxNotFoundHtml").Parse(sandboxNotFoundHtml))

type sandboxNotFoundData struct {
	SandboxId string `json:"sandboxId"`
	Message   string `json:"message"`
	Code      int    `json:"code"`
	Host      string `json:"-"`
}

func (e sandboxNotFoundData) StatusCode() int {
	return e.Code
}

func NewSandboxNotFoundError(sandboxId, host string) *TemplatedError[sandboxNotFoundData] {
	return &TemplatedError[sandboxNotFoundData]{
		template: sandboxNotFoundHtmlTemplate,
		vars: sandboxNotFoundData{
			SandboxId: sandboxId,
			Message:   "The sandbox was not found",
			Host:      host,
			Code:      http.StatusBadGateway,
		},
	}
}
