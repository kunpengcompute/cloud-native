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
	"fmt"
	"html/template"
	"net/http"
)

//go:embed browser_traffic_access_token_missing_error.html
var missingTrafficAccessTokenErrHtml string
var missingTrafficAccessTokenErrHtmlTemplate = template.Must(template.New("missingTrafficAccessTokenErrHtml").Parse(missingTrafficAccessTokenErrHtml))

//go:embed browser_traffic_access_token_invalid_error.html
var invalidTrafficAccessTokenErrHtml string
var invalidTrafficAccessTokenErrHtmlTemplate = template.Must(template.New("invalidTrafficAccessTokenErrHtml").Parse(invalidTrafficAccessTokenErrHtml))

type trafficAccessTokenErrData struct {
	SandboxId string `json:"sandboxId"`
	Message   string `json:"message"`
	Code      int    `json:"code"`
	Header    string `json:"-"`
	Host      string `json:"-"`
}

func (e trafficAccessTokenErrData) StatusCode() int {
	return e.Code
}

func NewTrafficAccessTokenMissingHeader(sandboxId, host string, header string) *TemplatedError[trafficAccessTokenErrData] {
	return &TemplatedError[trafficAccessTokenErrData]{
		template: missingTrafficAccessTokenErrHtmlTemplate,
		vars: trafficAccessTokenErrData{
			SandboxId: sandboxId,
			Message:   fmt.Sprintf("Sandbox is secured with traffic access token. Token header '%s' is missing", header),
			Host:      host,
			Header:    header,
			Code:      http.StatusForbidden,
		},
	}
}

func NewTrafficAccessTokenInvalidHeader(sandboxId, host string, header string) *TemplatedError[trafficAccessTokenErrData] {
	return &TemplatedError[trafficAccessTokenErrData]{
		template: invalidTrafficAccessTokenErrHtmlTemplate,
		vars: trafficAccessTokenErrData{
			SandboxId: sandboxId,
			Message:   fmt.Sprintf("Sandbox is secured with traffic access token. Provided token in header '%s' is invalid", header),
			Host:      host,
			Header:    header,
			Code:      http.StatusForbidden,
		},
	}
}
