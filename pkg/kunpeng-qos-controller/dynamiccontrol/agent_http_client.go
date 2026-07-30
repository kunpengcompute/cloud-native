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
	"net"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"k8s.io/klog/v2"
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
	Version     string          `json:"version"`
	NodeName    string          `json:"node_name"`
	ReasonCodes json.RawMessage `json:"reason_codes"`
}

type interferenceReasonCodeMapping struct {
	name   string
	reason InterferenceReason
}

var interferenceReasonCodeMappings = map[int]interferenceReasonCodeMapping{
	0: {name: "base", reason: InterferenceReasonNone},
	1: {name: "compute", reason: InterferenceReasonCPU},
	2: {name: "l2", reason: InterferenceReasonCPU},
	3: {name: "l3", reason: InterferenceReasonL3},
	4: {name: "membw", reason: InterferenceReasonMB},
	5: {name: "tlb", reason: InterferenceReasonCPU},
	6: {name: "frontend", reason: InterferenceReasonCPU},
}

// AgentClient encapsulates two-way communication with external agent.
// HTTPAgentClient is the default implementation.
type AgentClient interface {
	PublishOnlinePods(ctx context.Context, req AgentAnalyzeRequest) error
	GetInterference(ctx context.Context, nodeName string) (AgentAnalyzeResult, error)
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

// ValidateAgentBaseURL restricts agent endpoint to local loopback HTTP address
// to reduce SSRF risk from unexpected remote targets.
func ValidateAgentBaseURL(baseURL string) error {
	u, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil {
		return fmt.Errorf("parse agent address failed: %w", err)
	}
	if u.Scheme != "http" {
		return fmt.Errorf("agent address must use http scheme")
	}
	if u.User != nil {
		return fmt.Errorf("agent address must not contain user info")
	}
	if u.RawQuery != "" || u.Fragment != "" {
		return fmt.Errorf("agent address must not contain query or fragment")
	}
	if u.Path != "" && u.Path != "/" {
		return fmt.Errorf("agent address path must be empty")
	}
	host := u.Hostname()
	if host == "" {
		return fmt.Errorf("agent address host is empty")
	}
	if u.Port() == "" {
		return fmt.Errorf("agent address port is required")
	}
	if host == "localhost" {
		return nil
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return fmt.Errorf("agent address host must be localhost or loopback ip")
	}
	if !ip.IsLoopback() {
		return fmt.Errorf("agent address host must be loopback ip")
	}
	return nil
}

func (c *HTTPAgentClient) endpoint(p string) (string, error) {
	if err := ValidateAgentBaseURL(c.BaseURL); err != nil {
		return "", err
	}
	base, err := url.Parse(c.BaseURL)
	if err != nil {
		return "", fmt.Errorf("parse base url %q failed: %w", c.BaseURL, err)
	}
	base.Path = path.Clean(strings.TrimRight(base.Path, "/") + "/" + strings.TrimLeft(p, "/"))
	return base.String(), nil
}

// PublishOnlinePods sends latest online pod cgroup paths to local agent.
func (c *HTTPAgentClient) PublishOnlinePods(ctx context.Context, req AgentAnalyzeRequest) error {
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
	defer resp.Body.Close() // nolint: errcheck

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
	defer resp.Body.Close() // nolint: errcheck

	if resp.StatusCode != http.StatusOK {
		return AgentAnalyzeResult{}, decodeAgentHTTPError(resp)
	}

	var out getInterferenceResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return AgentAnalyzeResult{}, fmt.Errorf("decode interference response failed: %w", err)
	}
	if out.Version != defaultAgentAPIVersion {
		return AgentAnalyzeResult{}, fmt.Errorf(
			"unexpected interference response version: got %q, want %q",
			out.Version,
			defaultAgentAPIVersion,
		)
	}
	if out.NodeName != nodeName {
		return AgentAnalyzeResult{}, fmt.Errorf(
			"unexpected interference response node: got %q, want %q",
			out.NodeName,
			nodeName,
		)
	}
	if len(out.ReasonCodes) == 0 {
		return AgentAnalyzeResult{}, fmt.Errorf("interference response reason_codes field is required")
	}
	var reasonCodes []int
	if err := json.Unmarshal(out.ReasonCodes, &reasonCodes); err != nil {
		return AgentAnalyzeResult{}, fmt.Errorf("decode interference response reason_codes failed: %w", err)
	}

	reasonNames := make([]string, 0, len(reasonCodes))
	for _, code := range reasonCodes {
		name := "unknown"
		if mapping, ok := interferenceReasonCodeMappings[code]; ok {
			name = mapping.name
		}
		reasonNames = append(reasonNames, name)
	}
	klog.Infof(
		"dynamic-control received interference reasons: node=%s reasons=%v",
		nodeName,
		reasonNames,
	)

	reasons, ignored := mapInterferenceReasonCodes(reasonCodes)
	for _, code := range ignored {
		klog.Warningf(
			"dynamic-control ignored unsupported interference reason code: node=%s reasonCode=%d",
			nodeName,
			code,
		)
	}
	return AgentAnalyzeResult{Reasons: reasons}, nil
}

func mapInterferenceReasonCodes(codes []int) ([]InterferenceReason, []int) {
	mapped := make([]InterferenceReason, 0, len(codes))
	ignored := make([]int, 0)
	for _, code := range codes {
		mapping, ok := interferenceReasonCodeMappings[code]
		if !ok {
			ignored = append(ignored, code)
			continue
		}
		mapped = append(mapped, mapping.reason)
	}

	reasons, _ := normalizeInterferenceReasons(mapped, true)
	return reasons, ignored
}

func decodeAgentHTTPError(resp *http.Response) error {
	snippet, err := io.ReadAll(io.LimitReader(resp.Body, maxAgentErrorBodySnippet))
	if err != nil {
		return fmt.Errorf("agent http error: status=%d, read body failed: %w", resp.StatusCode, err)
	}
	if len(snippet) == 0 {
		return fmt.Errorf("agent http error: status=%d", resp.StatusCode)
	}
	return fmt.Errorf("agent http error: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(snippet)))
}
