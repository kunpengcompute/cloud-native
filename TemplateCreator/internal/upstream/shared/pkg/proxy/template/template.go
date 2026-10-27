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
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"regexp"
)

var browserRegex = regexp.MustCompile(`(?i)mozilla|chrome|safari|firefox|edge|opera|msie`)

type jsonErrorMessage interface {
	StatusCode() int
}

type TemplatedError[T jsonErrorMessage] struct {
	template *template.Template
	vars     T
}

func (e *TemplatedError[T]) buildHtml() ([]byte, error) {
	var html bytes.Buffer

	err := e.template.Execute(&html, e.vars)
	if err != nil {
		return nil, err
	}

	return html.Bytes(), nil
}

func (e *TemplatedError[T]) buildJson() ([]byte, error) {
	return json.Marshal(e.vars)
}

func (e *TemplatedError[T]) HandleError(
	w http.ResponseWriter,
	r *http.Request,
) error {
	if e.vars.StatusCode() <= 0 {
		return fmt.Errorf("invalid status code: %d", e.vars.StatusCode())
	}

	if isBrowser(r) {
		body, buildErr := e.buildHtml()
		if buildErr != nil {
			return buildErr
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(e.vars.StatusCode())
		_, writeErr := w.Write(body)
		if writeErr != nil {
			return writeErr
		}

		return nil
	}

	body, buildErr := e.buildJson()
	if buildErr != nil {
		return buildErr
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(e.vars.StatusCode())

	_, writeErr := w.Write(body)
	if writeErr != nil {
		return writeErr
	}

	return nil
}

func isBrowser(r *http.Request) bool {
	return browserRegex.MatchString(r.UserAgent())
}
