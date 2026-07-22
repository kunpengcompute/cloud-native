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
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"k8s.io/klog/v2"
)

type fakeNodeIdentity struct {
	name string
}

func (n fakeNodeIdentity) NodeName() string { return n.name }

type fakeOnlineSource struct {
	pods []OnlinePodCgroup
	err  error
}

func (s fakeOnlineSource) ListOnlinePodCgroups(_ context.Context, _ string) ([]OnlinePodCgroup, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.pods, nil
}

type fakeAgentClient struct {
	gotPublish AgentAnalyzeRequest
	publishErr error
	gotNode    string
	resp       AgentAnalyzeResult
	getErr     error
}

func (s *fakeAgentClient) PublishOnlinePods(_ context.Context, req AgentAnalyzeRequest) error {
	s.gotPublish = req
	return s.publishErr
}

func (s *fakeAgentClient) GetInterference(_ context.Context, nodeName string) (AgentAnalyzeResult, error) {
	s.gotNode = nodeName
	if s.getErr != nil {
		return AgentAnalyzeResult{}, s.getErr
	}
	return s.resp, nil
}

type fakeEngine struct {
	calls  int
	node   string
	result AgentAnalyzeResult
	err    error
}

func (e *fakeEngine) HandleInterference(_ context.Context, nodeName string, result AgentAnalyzeResult) error {
	e.calls++
	e.node = nodeName
	e.result = result
	if e.err != nil {
		return e.err
	}
	return nil
}

func TestCoordinator_PublishOnlinePodsOnce(t *testing.T) {
	now := time.Date(2026, 4, 16, 10, 0, 0, 0, time.UTC)
	agent := &fakeAgentClient{}

	c := &Coordinator{
		NodeIdentity: fakeNodeIdentity{name: "node-a"},
		OnlineSource: fakeOnlineSource{
			pods: []OnlinePodCgroup{
				{
					Namespace:  "default",
					Name:       "online-a",
					UID:        "pod-uid-a",
					CgroupPath: "/kubepods.slice/pod-a",
				},
			},
		},
		Agent:  agent,
		Engine: &fakeEngine{},
		Clock: func() time.Time {
			return now
		},
	}

	if err := c.PublishOnlinePodsOnce(context.Background()); err != nil {
		t.Fatalf("PublishOnlinePodsOnce() unexpected error: %v", err)
	}
	if agent.gotPublish.NodeName != "node-a" {
		t.Fatalf("publisher got wrong node name: %s", agent.gotPublish.NodeName)
	}
	if !agent.gotPublish.Time.Equal(now) {
		t.Fatalf("publisher got wrong timestamp: %v", agent.gotPublish.Time)
	}
	if len(agent.gotPublish.Pods) != 1 || agent.gotPublish.Pods[0].Name != "online-a" {
		t.Fatalf("publisher got wrong pod list: %+v", agent.gotPublish.Pods)
	}
}

func TestCoordinator_ApplyInterferenceOnce(t *testing.T) {
	engine := &fakeEngine{}
	c := &Coordinator{
		NodeIdentity: fakeNodeIdentity{name: "node-a"},
		OnlineSource: fakeOnlineSource{},
		Agent:        &fakeAgentClient{resp: AgentAnalyzeResult{Reason: InterferenceReasonL3}},
		Engine:       engine,
	}

	if err := c.ApplyInterferenceOnce(context.Background()); err != nil {
		t.Fatalf("ApplyInterferenceOnce() unexpected error: %v", err)
	}
	if engine.calls != 1 || engine.node != "node-a" || engine.result.Reason != InterferenceReasonL3 {
		t.Fatalf("unexpected engine call: %+v", engine)
	}
}

func TestCoordinator_ApplyInterferenceOnceLogsResult(t *testing.T) {
	var logOutput bytes.Buffer
	klog.LogToStderr(false)
	klog.SetOutput(&logOutput)
	t.Cleanup(func() {
		klog.Flush()
		klog.SetOutput(os.Stderr)
		klog.LogToStderr(true)
	})

	result := AgentAnalyzeResult{
		Reason:     InterferenceReasonL3,
		TTLSeconds: 15,
		Items:      []InterferenceItem{{PodUID: "pod-uid-a", Score: 0.8}},
	}
	c := &Coordinator{
		NodeIdentity: fakeNodeIdentity{name: "node-a"},
		OnlineSource: fakeOnlineSource{},
		Agent:        &fakeAgentClient{resp: result},
		Engine:       &fakeEngine{},
	}

	if err := c.ApplyInterferenceOnce(context.Background()); err != nil {
		t.Fatalf("ApplyInterferenceOnce() unexpected error: %v", err)
	}
	klog.Flush()

	for _, want := range []string{"node=node-a", "reason=l3", "ttlSeconds=15", "items=1"} {
		if !strings.Contains(logOutput.String(), want) {
			t.Fatalf("expected log to contain %q, got %q", want, logOutput.String())
		}
	}
}

func TestCoordinator_ApplyInterferenceOncePropagatesError(t *testing.T) {
	expectErr := errors.New("agent unavailable")
	c := &Coordinator{
		NodeIdentity: fakeNodeIdentity{name: "node-a"},
		OnlineSource: fakeOnlineSource{},
		Agent:        &fakeAgentClient{getErr: expectErr},
		Engine:       &fakeEngine{},
	}

	err := c.ApplyInterferenceOnce(context.Background())
	if !errors.Is(err, expectErr) {
		t.Fatalf("ApplyInterferenceOnce() expected err %v, got %v", expectErr, err)
	}
}
