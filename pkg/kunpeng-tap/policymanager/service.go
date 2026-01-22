/*
 * Copyright (c) 2025 Huawei Technology corp.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package policymanager

import (
	"context"

	"k8s.io/klog/v2"

	runtimeapi "kunpeng.huawei.com/kunpeng-cloud-computing/api/kunpeng-tap/policy-manager/v1alpha1"
)

func (p *policyManager) PreRunPodSandboxHook(ctx context.Context,
	req *runtimeapi.PodSandboxHookRequest) (*runtimeapi.PodSandboxHookResponse, error) {
	klog.V(5).InfoS("Received PreRunPodSandboxHook request", "request", req.String())

	resp, err := p.Hooks.PreRunPodSandboxHook(ctx, req)
	if err != nil {
		klog.ErrorS(err, "Error in PreRunPodSandboxHook")
		return nil, err
	}

	klog.V(5).InfoS("Sending PreRunPodSandboxHook response", "pod", req.PodMeta.String(), "response", resp.String())
	return resp, err
}

func (p *policyManager) PostStopPodSandboxHook(ctx context.Context,
	req *runtimeapi.PodSandboxHookRequest) (*runtimeapi.PodSandboxHookResponse, error) {
	klog.V(5).InfoS("Received PostStopPodSandboxHook request", "request", req.String())

	resp, err := p.Hooks.PostStopPodSandboxHook(ctx, req)
	if err != nil {
		klog.ErrorS(err, "Error in PostStopPodSandboxHook")
		return nil, err
	}

	klog.V(5).InfoS("Sending PostStopPodSandboxHook response", "pod", req.PodMeta.String(), "response", resp.String())
	return resp, err
}

func (p *policyManager) PreRemovePodSandboxHook(ctx context.Context,
	req *runtimeapi.PodSandboxHookRequest) (*runtimeapi.PodSandboxHookResponse, error) {
	klog.V(5).InfoS("Received PreRemovePodSandboxHook request", "request", req.String())

	resp, err := p.Hooks.PreRemovePodSandboxHook(ctx, req)
	if err != nil {
		klog.ErrorS(err, "Error in PreRemovePodSandboxHook")
		return nil, err
	}

	klog.V(5).InfoS("Sending PreRemovePodSandboxHook response", "pod", req.PodMeta.String(), "response", resp.String())
	return resp, err
}

func (p *policyManager) PreCreateContainerHook(ctx context.Context,
	req *runtimeapi.ContainerResourceHookRequest) (*runtimeapi.ContainerResourceHookResponse, error) {
	klog.V(5).InfoS("Received PreCreateContainerHook request", "request", req.String())

	resp, err := p.Hooks.PreCreateContainerHook(ctx, req)
	if err != nil {
		klog.ErrorS(err, "Error in PreCreateContainerHook")
		return nil, err
	}

	klog.V(5).InfoS("Sending PreCreateContainerHook response",
		"pod", req.PodMeta.String(),
		"container", req.ContainerMeta.String(),
		"response", resp.String())
	return resp, err
}

func (p *policyManager) PreStartContainerHook(ctx context.Context,
	req *runtimeapi.ContainerResourceHookRequest) (*runtimeapi.ContainerResourceHookResponse, error) {
	klog.V(5).InfoS("Received PreStartContainerHook request", "request", req.String())

	resp, err := p.Hooks.PreStartContainerHook(ctx, req)
	if err != nil {
		klog.ErrorS(err, "Error in PreStartContainerHook")
		return nil, err
	}

	klog.V(5).InfoS("Sending PreStartContainerHook response",
		"pod", req.PodMeta.String(),
		"container", req.ContainerMeta.String(),
		"response", resp.String())
	return resp, err
}

func (p *policyManager) PostStartContainerHook(ctx context.Context,
	req *runtimeapi.ContainerResourceHookRequest) (*runtimeapi.ContainerResourceHookResponse, error) {
	klog.V(5).InfoS("Received PostStartContainerHook request", "request", req.String())

	resp, err := p.Hooks.PostStartContainerHook(ctx, req)
	if err != nil {
		klog.ErrorS(err, "Error in PostStartContainerHook")
		return nil, err
	}

	klog.V(5).InfoS("Sending PostStartContainerHook response",
		"pod", req.PodMeta.String(),
		"container", req.ContainerMeta.String(),
		"response", resp.String())
	return resp, err
}

func (p *policyManager) PostStopContainerHook(ctx context.Context,
	req *runtimeapi.ContainerResourceHookRequest) (*runtimeapi.ContainerResourceHookResponse, error) {
	klog.V(5).InfoS("Received PostStopContainerHook request", "request", req.String())

	resp, err := p.Hooks.PostStopContainerHook(ctx, req)
	if err != nil {
		klog.ErrorS(err, "Error in PostStopContainerHook")
		return nil, err
	}

	klog.V(5).InfoS("Sending PostStopContainerHook response",
		"pod", req.PodMeta.String(),
		"container", req.ContainerMeta.String(),
		"response", resp.String())
	return resp, err
}

func (p *policyManager) PreUpdateContainerResourcesHook(ctx context.Context,
	req *runtimeapi.ContainerResourceHookRequest) (*runtimeapi.ContainerResourceHookResponse, error) {
	klog.V(5).InfoS("Received PreUpdateContainerResourcesHook request", "request", req.String())

	resp, err := p.Hooks.PreUpdateContainerResourcesHook(ctx, req)
	if err != nil {
		klog.ErrorS(err, "Error in PreUpdateContainerResourcesHook")
		return nil, err
	}

	klog.V(5).InfoS("Sending PreUpdateContainerResourcesHook response",
		"pod", req.PodMeta.String(),
		"container", req.ContainerMeta.String(),
		"response", resp.String())
	return resp, err
}

func (p *policyManager) PreRemoveContainerHook(ctx context.Context,
	req *runtimeapi.ContainerResourceHookRequest) (*runtimeapi.ContainerResourceHookResponse, error) {
	klog.V(5).InfoS("Received PreRemoveContainerHook request", "request", req.String())

	resp, err := p.Hooks.PreRemoveContainerHook(ctx, req)
	if err != nil {
		klog.ErrorS(err, "Error in PreRemoveContainerHook")
		return nil, err
	}

	klog.V(5).InfoS("Sending PreRemoveContainerHook response",
		"pod", req.PodMeta.String(),
		"container", req.ContainerMeta.String(),
		"response", resp.String())
	return resp, err
}
