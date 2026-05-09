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
	"net/url"
	"path"
	"strings"
	"time"
)

const (
	defaultAgentAPIVersion   = "v1"
	defaultOnlinePodsPath    = "/v1/online-pods"
	defaultInterferencePath  = "/v1/interference"
	defaultAgentHTTPTimeout  = 2 * time.Second
	defaultAgentTCPBaseURL   = "http://127.0.0.1:18080"
	maxAgentErrorBodySnippet = 512
)

type publishOnlinePodsRequest struct {
	Version   string            `json:"version"`
	NodeName  string            `json:"node_name"`
	Timestamp time.Time         `json:"timestamp"`
	Pods      []OnlinePodCgroup `json:"pods"`
}

type publishOnlinePodsResponse struct {
	Accepted bool   `json:"accepted"`
	Message  string `json:"message,omitempty"`
}

type getInterferenceResponse struct {
	Version    string             `json:"version"`
	NodeName   string             `json:"node_name"`
	Reason     InterferenceReason `json:"reason"`
	TTLSeconds int64              `json:"ttl_seconds"`
	Items      []InterferenceItem `json:"items"`
}

// HTTPAgentClient communicates with local agent via HTTP JSON.
// It implements both OnlinePodPublisher and InterferenceResultSource.
type HTTPAgentClient struct {
	BaseURL    string
	Version    string
	HTTPClient *http.Client
}

// NewTCPHTTPAgentClient creates an HTTP-over-TCP client, suitable for
// sidecar/peer container communication in the same Pod (localhost:<port>).
// If baseURL is empty, it defaults to http://127.0.0.1:18080.
func NewTCPHTTPAgentClient(baseURL string) *HTTPAgentClient {
	if strings.TrimSpace(baseURL) == "" {
		baseURL = defaultAgentTCPBaseURL
	}
	return &HTTPAgentClient{
		BaseURL: baseURL,
		Version: defaultAgentAPIVersion,
		HTTPClient: &http.Client{
			Timeout: defaultAgentHTTPTimeout,
		},
	}
}

func (c *HTTPAgentClient) setDefaults() {
	if c.BaseURL == "" {
		c.BaseURL = defaultAgentTCPBaseURL
	}
	if c.Version == "" {
		c.Version = defaultAgentAPIVersion
	}
	if c.HTTPClient == nil {
		c.HTTPClient = &http.Client{Timeout: defaultAgentHTTPTimeout}
	}
}

func (c *HTTPAgentClient) validate() error {
	if c.HTTPClient == nil {
		return fmt.Errorf("http client must not be nil")
	}
	if c.BaseURL == "" {
		return fmt.Errorf("base url must not be empty")
	}
	return nil
}

func (c *HTTPAgentClient) endpoint(p string) (string, error) {
	base, err := url.Parse(c.BaseURL)
	if err != nil {
		return "", fmt.Errorf("parse base url %q failed: %w", c.BaseURL, err)
	}
	base.Path = path.Clean(strings.TrimRight(base.Path, "/") + "/" + strings.TrimLeft(p, "/"))
	return base.String(), nil
}

// PublishOnlinePods sends latest online pod cgroup paths to local agent.
func (c *HTTPAgentClient) PublishOnlinePods(ctx context.Context, req AgentAnalyzeRequest) error {
	c.setDefaults()
	if err := c.validate(); err != nil {
		return err
	}
	if req.NodeName == "" {
		return fmt.Errorf("node name must not be empty")
	}

	endpoint, err := c.endpoint(defaultOnlinePodsPath)
	if err != nil {
		return err
	}

	payload := publishOnlinePodsRequest{
		Version:   c.Version,
		NodeName:  req.NodeName,
		Timestamp: req.Time,
		Pods:      req.Pods,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal publish request failed: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build publish request failed: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("publish online pods failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return decodeAgentHTTPError(resp)
	}

	var ack publishOnlinePodsResponse
	if err := json.NewDecoder(resp.Body).Decode(&ack); err != nil {
		return fmt.Errorf("decode publish response failed: %w", err)
	}
	if !ack.Accepted {
		if ack.Message == "" {
			ack.Message = "agent rejected request"
		}
		return fmt.Errorf("publish rejected: %s", ack.Message)
	}
	return nil
}

// GetInterference fetches latest interference result for one node from local agent.
func (c *HTTPAgentClient) GetInterference(ctx context.Context, nodeName string) (AgentAnalyzeResult, error) {
	c.setDefaults()
	if err := c.validate(); err != nil {
		return AgentAnalyzeResult{}, err
	}
	if nodeName == "" {
		return AgentAnalyzeResult{}, fmt.Errorf("node name must not be empty")
	}

	endpoint, err := c.endpoint(defaultInterferencePath)
	if err != nil {
		return AgentAnalyzeResult{}, err
	}
	queryURL, err := url.Parse(endpoint)
	if err != nil {
		return AgentAnalyzeResult{}, fmt.Errorf("parse endpoint url failed: %w", err)
	}
	q := queryURL.Query()
	q.Set("node_name", nodeName)
	queryURL.RawQuery = q.Encode()

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, queryURL.String(), nil)
	if err != nil {
		return AgentAnalyzeResult{}, fmt.Errorf("build get interference request failed: %w", err)
	}
	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return AgentAnalyzeResult{}, fmt.Errorf("get interference failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return AgentAnalyzeResult{}, decodeAgentHTTPError(resp)
	}

	var out getInterferenceResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return AgentAnalyzeResult{}, fmt.Errorf("decode interference response failed: %w", err)
	}

	return AgentAnalyzeResult{
		Reason:     normalizeInterferenceReason(out.Reason),
		TTLSeconds: out.TTLSeconds,
		Items:      out.Items,
	}, nil
}

func decodeAgentHTTPError(resp *http.Response) error {
	snippet, _ := io.ReadAll(io.LimitReader(resp.Body, maxAgentErrorBodySnippet))
	if len(snippet) == 0 {
		return fmt.Errorf("agent http error: status=%d", resp.StatusCode)
	}
	return fmt.Errorf("agent http error: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(snippet)))
}

func normalizeInterferenceReason(reason InterferenceReason) InterferenceReason {
	switch InterferenceReason(strings.ToLower(string(reason))) {
	case InterferenceReasonL3:
		return InterferenceReasonL3
	case InterferenceReasonMB:
		return InterferenceReasonMB
	case InterferenceReasonCPU:
		return InterferenceReasonCPU
	default:
		return InterferenceReasonUnknown
	}
}
