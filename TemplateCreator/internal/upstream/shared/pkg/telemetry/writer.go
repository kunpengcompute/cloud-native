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

package telemetry

import (
	"context"
	"fmt"
	"io"
	"strings"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

type EventWriter struct {
	span trace.Span
	name string
}

func (w *EventWriter) Write(p []byte) (n int, err error) {
	fmt.Printf("->> [%s] %s\n", w.name, strings.Trim(string(p), " \t\n"))

	w.span.AddEvent(w.name,
		trace.WithAttributes(
			attribute.String("content", string(p)),
		),
	)

	return len(p), nil
}

func NewEventWriter(ctx context.Context, name string) io.Writer {
	span := trace.SpanFromContext(ctx)

	return &EventWriter{
		name: name,
		span: span,
	}
}
