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

// TemplateCreator modification: upstream import paths rewritten; no behavioral changes.

package sandboxtools

import (
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"

	"gitcode.com/cloud-native/cloud-native/TemplateCreator/internal/upstream/orchestrator/pkg/proxy"
	"gitcode.com/cloud-native/cloud-native/TemplateCreator/internal/upstream/orchestrator/pkg/sandbox"
	"gitcode.com/cloud-native/cloud-native/TemplateCreator/internal/upstream/shared/pkg/grpc"
	"gitcode.com/cloud-native/cloud-native/TemplateCreator/internal/upstream/shared/pkg/telemetry"
)

const fileCopyTimeout = 10 * time.Minute

var tracer = otel.Tracer("gitcode.com/cloud-native/cloud-native/TemplateCreator/internal/upstream/orchestrator/pkg/template/build/sandboxtools")

func CopyFile(
	ctx context.Context,
	proxy *proxy.SandboxProxy,
	sandboxID string,
	user string,
	sourcePath string,
	targetPath string,
) error {
	ctx, span := tracer.Start(ctx, "copy-file")
	defer span.End()

	file, err := os.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer file.Close()

	// Pipe to stream data
	pr, pw := io.Pipe()
	writer := multipart.NewWriter(pw)

	errChan := make(chan error, 1)
	go func() {
		var err error
		defer func() {
			writer.Close()
			if err != nil {
				pw.CloseWithError(err)
			} else {
				pw.Close()
			}
			errChan <- err
		}()

		part, err := writer.CreateFormFile("file", filepath.Base(sourcePath))
		if err != nil {
			err = fmt.Errorf("failed to create form file: %w", err)

			return
		}

		if _, errCopy := io.Copy(part, file); errCopy != nil {
			err = fmt.Errorf("failed to copy file: %w", errCopy)

			return
		}
	}()

	// Prepare query parameters
	proxyHost := fmt.Sprintf("http://localhost%s", proxy.GetAddr())
	params := url.Values{}
	params.Add("path", targetPath)
	params.Add("username", user)

	telemetry.ReportEvent(ctx, "copy_file",
		attribute.String("source.path", sourcePath),
		attribute.String("target.path", targetPath),
		attribute.String("proxy.host", proxyHost),
		attribute.String("sandbox.id", sandboxID),
	)
	uploadURL := fmt.Sprintf("%s/files?%s", proxyHost, params.Encode())

	// Create HTTP request with streaming body
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, uploadURL, pr)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	err = grpc.SetSandboxHeader(req.Header, proxyHost, sandboxID)
	if err != nil {
		return fmt.Errorf("failed to set request header: %w", err)
	}
	req.Host = req.Header.Get("Host")

	client := http.Client{
		Timeout:   fileCopyTimeout,
		Transport: sandbox.SandboxHttpTransport,
	}

	// Send request
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if uploadErr := <-errChan; uploadErr != nil {
		io.Copy(io.Discard, resp.Body)

		return fmt.Errorf("file upload failed: %w", uploadErr)
	}

	body, _ := io.ReadAll(resp.Body)
	telemetry.ReportEvent(ctx, "file_upload",
		attribute.Int("response.code", resp.StatusCode),
		attribute.String("response.body", string(body)),
	)

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to upload file (%d): %s", resp.StatusCode, string(body))
	}

	return nil
}
