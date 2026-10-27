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

//go:embed browser_port_closed.html
var portClosedHtml string
var portClosedHtmlTemplate = template.Must(template.New("portClosedHtml").Parse(portClosedHtml))

type portClosedError struct {
	SandboxId string `json:"sandboxId"`
	Message   string `json:"message"`
	Port      uint64 `json:"port"`
	Code      int    `json:"code"`
	Host      string `json:"-"`
}

func (e portClosedError) StatusCode() int {
	return e.Code
}

func NewPortClosedError(sandboxId, host string, port uint64) *TemplatedError[portClosedError] {
	return &TemplatedError[portClosedError]{
		template: portClosedHtmlTemplate,
		vars: portClosedError{
			Message:   "The sandbox is running but port is not open",
			SandboxId: sandboxId,
			Host:      host,
			Port:      port,
			Code:      http.StatusBadGateway,
		},
	}
}
