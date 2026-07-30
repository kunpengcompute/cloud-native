/*
Copyright (c) Huawei Technologies Co., Ltd. 2023-2026. All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package dynamiccontrol

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"k8s.io/klog/v2"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func newMockHTTPClient(fn roundTripFunc) *http.Client {
	return &http.Client{
		Transport: fn,
	}
}

func httpResp(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func TestHTTPAgentClientPublishOnlinePods(t *testing.T) {
	now := time.Date(2026, 4, 17, 8, 0, 0, 0, time.UTC)
	var gotReq publishOnlinePodsRequest

	c := &HTTPAgentClient{
		BaseURL: "http://127.0.0.1:18080",
		Version: "v1",
		HTTPClient: newMockHTTPClient(func(r *http.Request) (*http.Response, error) {
			if r.Method != http.MethodPost {
				t.Fatalf("expected method POST, got %s", r.Method)
			}
			if r.URL.Path != "/v1/online-pods" {
				t.Fatalf("expected path /v1/online-pods, got %s", r.URL.Path)
			}
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("read request body failed: %v", err)
			}
			if err := json.Unmarshal(body, &gotReq); err != nil {
				t.Fatalf("decode request failed: %v", err)
			}
			return httpResp(http.StatusOK, `{"accepted":true,"message":"ok"}`), nil
		}),
	}

	err := c.PublishOnlinePods(context.Background(), AgentAnalyzeRequest{
		NodeName: "node-a",
		Time:     now,
		Pods: []OnlinePodCgroup{
			{
				Namespace:  "default",
				Name:       "online-a",
				UID:        "pod-uid-a",
				CgroupPath: "/kubepods.slice/pod-a",
			},
		},
	})
	if err != nil {
		t.Fatalf("PublishOnlinePods() unexpected error: %v", err)
	}
	if gotReq.NodeName != "node-a" || len(gotReq.Pods) != 1 || gotReq.Pods[0].UID != "pod-uid-a" {
		t.Fatalf("unexpected request payload: %+v", gotReq)
	}
	if !gotReq.Timestamp.Equal(now) {
		t.Fatalf("unexpected timestamp: %v", gotReq.Timestamp)
	}
}

func TestHTTPAgentClientPublishOnlinePodsReject(t *testing.T) {
	c := &HTTPAgentClient{
		BaseURL: "http://127.0.0.1:18080",
		HTTPClient: newMockHTTPClient(func(r *http.Request) (*http.Response, error) {
			return httpResp(http.StatusOK, `{"accepted":false,"message":"invalid payload"}`), nil
		}),
	}
	err := c.PublishOnlinePods(context.Background(), AgentAnalyzeRequest{NodeName: "node-a"})
	if err == nil || !strings.Contains(err.Error(), "invalid payload") {
		t.Fatalf("expected reject error, got %v", err)
	}
}

func TestHTTPAgentClientGetInterference(t *testing.T) {
	c := &HTTPAgentClient{
		BaseURL: "http://127.0.0.1:18080",
		HTTPClient: newMockHTTPClient(func(r *http.Request) (*http.Response, error) {
			if r.Method != http.MethodGet {
				t.Fatalf("expected method GET, got %s", r.Method)
			}
			if r.URL.Path != "/v1/interference" {
				t.Fatalf("expected path /v1/interference, got %s", r.URL.Path)
			}
			if got := r.URL.Query().Get("node_name"); got != "node-a" {
				t.Fatalf("expected node_name=node-a, got %s", got)
			}
			return httpResp(http.StatusOK, `{
				"version":"v1",
				"node_name":"node-a",
				"reason_codes":[1,3,4,2,5,6,0,3,99,-1]
			}`), nil
		}),
	}

	got, err := c.GetInterference(context.Background(), "node-a")
	if err != nil {
		t.Fatalf("GetInterference() unexpected error: %v", err)
	}
	want := []InterferenceReason{
		InterferenceReasonNone,
		InterferenceReasonCPU,
		InterferenceReasonMB,
		InterferenceReasonL3,
	}
	if !reflect.DeepEqual(got.Reasons, want) {
		t.Fatalf("unexpected interference reasons: got %v, want %v", got.Reasons, want)
	}
}

func TestHTTPAgentClientGetInterferenceMapsReasonCodes(t *testing.T) {
	tests := []struct {
		code int
		want InterferenceReason
	}{
		{code: 0, want: InterferenceReasonNone},
		{code: 1, want: InterferenceReasonCPU},
		{code: 2, want: InterferenceReasonCPU},
		{code: 3, want: InterferenceReasonL3},
		{code: 4, want: InterferenceReasonMB},
		{code: 5, want: InterferenceReasonCPU},
		{code: 6, want: InterferenceReasonCPU},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("code-%d", tt.code), func(t *testing.T) {
			body := fmt.Sprintf(
				`{"version":"v1","node_name":"node-a","reason_codes":[%d]}`,
				tt.code,
			)
			c := &HTTPAgentClient{
				BaseURL: "http://127.0.0.1:18080",
				HTTPClient: newMockHTTPClient(func(r *http.Request) (*http.Response, error) {
					return httpResp(http.StatusOK, body), nil
				}),
			}

			got, err := c.GetInterference(context.Background(), "node-a")
			if err != nil {
				t.Fatalf("GetInterference() unexpected error: %v", err)
			}
			want := []InterferenceReason{tt.want}
			if !reflect.DeepEqual(got.Reasons, want) {
				t.Fatalf("unexpected reasons: got %v, want %v", got.Reasons, want)
			}
		})
	}
}

func TestHTTPAgentClientGetInterferenceLogsRawReasons(t *testing.T) {
	var logOutput bytes.Buffer
	klog.LogToStderr(false)
	klog.SetOutput(&logOutput)
	t.Cleanup(func() {
		klog.Flush()
		klog.SetOutput(os.Stderr)
		klog.LogToStderr(true)
	})

	c := &HTTPAgentClient{
		BaseURL: "http://127.0.0.1:18080",
		HTTPClient: newMockHTTPClient(func(r *http.Request) (*http.Response, error) {
			return httpResp(http.StatusOK, `{
				"version":"v1",
				"node_name":"node-a",
				"reason_codes":[0,2,99]
			}`), nil
		}),
	}

	if _, err := c.GetInterference(context.Background(), "node-a"); err != nil {
		t.Fatalf("GetInterference() unexpected error: %v", err)
	}
	klog.Flush()

	want := "node=node-a reasons=[base l2 unknown]"
	if !strings.Contains(logOutput.String(), want) {
		t.Fatalf("expected log to contain %q, got %q", want, logOutput.String())
	}
}

func TestHTTPAgentClientGetInterferenceEmptyReasonCodes(t *testing.T) {
	for _, body := range []string{
		`{"version":"v1","node_name":"node-a","reason_codes":[]}`,
		`{"version":"v1","node_name":"node-a","reason_codes":null}`,
	} {
		c := &HTTPAgentClient{
			BaseURL: "http://127.0.0.1:18080",
			HTTPClient: newMockHTTPClient(func(r *http.Request) (*http.Response, error) {
				return httpResp(http.StatusOK, body), nil
			}),
		}

		got, err := c.GetInterference(context.Background(), "node-a")
		if err != nil {
			t.Fatalf("GetInterference() unexpected error: %v", err)
		}
		if len(got.Reasons) != 0 {
			t.Fatalf("expected empty reasons, got %v", got.Reasons)
		}
	}
}

func TestHTTPAgentClientGetInterferenceValidatesMetadata(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{
			name: "version mismatch",
			body: `{"version":"v2","node_name":"node-a","reason_codes":[]}`,
			want: "version",
		},
		{
			name: "node mismatch",
			body: `{"version":"v1","node_name":"node-b","reason_codes":[]}`,
			want: "node",
		},
		{
			name: "missing reason codes",
			body: `{"version":"v1","node_name":"node-a","reasons":["l3"]}`,
			want: "reason_codes",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &HTTPAgentClient{
				BaseURL: "http://127.0.0.1:18080",
				HTTPClient: newMockHTTPClient(func(r *http.Request) (*http.Response, error) {
					return httpResp(http.StatusOK, tt.body), nil
				}),
			}
			_, err := c.GetInterference(context.Background(), "node-a")
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("expected %s validation error, got %v", tt.want, err)
			}
		})
	}
}

func TestHTTPAgentClientGetInterferenceHTTPError(t *testing.T) {
	c := &HTTPAgentClient{
		BaseURL: "http://127.0.0.1:18080",
		HTTPClient: newMockHTTPClient(func(r *http.Request) (*http.Response, error) {
			return httpResp(http.StatusBadGateway, "backend unavailable"), nil
		}),
	}
	_, err := c.GetInterference(context.Background(), "node-a")
	if err == nil || !strings.Contains(err.Error(), "status=502") {
		t.Fatalf("expected http error, got %v", err)
	}
}

func TestHTTPAgentClientRejectBasePath(t *testing.T) {
	c := &HTTPAgentClient{
		BaseURL: "http://127.0.0.1:18080/custom/base",
		HTTPClient: newMockHTTPClient(func(r *http.Request) (*http.Response, error) {
			return httpResp(http.StatusOK, `{"accepted":true}`), nil
		}),
	}
	err := c.PublishOnlinePods(context.Background(), AgentAnalyzeRequest{NodeName: "node-a"})
	if err == nil {
		t.Fatalf("expected error for non-empty base path")
	}
}

func TestHTTPAgentClientDecodeFailure(t *testing.T) {
	c := &HTTPAgentClient{
		BaseURL: "http://127.0.0.1:18080",
		HTTPClient: newMockHTTPClient(func(r *http.Request) (*http.Response, error) {
			// Invalid JSON response.
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(bytes.NewBufferString("{invalid json")),
			}, nil
		}),
	}
	_, err := c.GetInterference(context.Background(), "node-a")
	if err == nil || !strings.Contains(err.Error(), "decode interference response failed") {
		t.Fatalf("expected decode error, got %v", err)
	}
}

func TestNewTCPHTTPAgentClientDefaults(t *testing.T) {
	c := NewTCPHTTPAgentClient("")
	if c.BaseURL != "http://127.0.0.1:18080" {
		t.Fatalf("unexpected default base url: %s", c.BaseURL)
	}
	if c.HTTPClient == nil {
		t.Fatalf("http client should not be nil")
	}
	if c.Version != "v1" {
		t.Fatalf("unexpected default version: %s", c.Version)
	}
}

func TestNewTCPHTTPAgentClientCustomBaseURL(t *testing.T) {
	c := NewTCPHTTPAgentClient("http://127.0.0.1:19090")
	if c.BaseURL != "http://127.0.0.1:19090" {
		t.Fatalf("unexpected custom base url: %s", c.BaseURL)
	}
}
