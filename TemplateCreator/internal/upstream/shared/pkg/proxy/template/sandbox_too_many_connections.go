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
	"fmt"
	"html/template"
	"net/http"
)

//go:embed browser_sandbox_too_many_connections.html
var sandboxTooManyConnectionsHtml string
var sandboxTooManyConnectionsHtmlTemplate = template.Must(template.New("sandboxTooManyConnectionsHtml").Parse(sandboxTooManyConnectionsHtml))

type sandboxTooManyConnectionsData struct {
	SandboxId       string `json:"sandboxId"`
	Message         string `json:"message"`
	Code            int    `json:"code"`
	ConnectionLimit int    `json:"connectionLimit"`
	Host            string `json:"-"`
}

func (e sandboxTooManyConnectionsData) StatusCode() int {
	return e.Code
}

func NewSandboxTooManyConnectionsError(sandboxId, host string, connectionLimit int) *TemplatedError[sandboxTooManyConnectionsData] {
	return &TemplatedError[sandboxTooManyConnectionsData]{
		template: sandboxTooManyConnectionsHtmlTemplate,
		vars: sandboxTooManyConnectionsData{
			SandboxId:       sandboxId,
			Message:         fmt.Sprintf("The sandbox has too many concurrent incoming connections (limit: %d)", connectionLimit),
			Host:            host,
			ConnectionLimit: connectionLimit,
			Code:            http.StatusTooManyRequests,
		},
	}
}
