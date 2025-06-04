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

package policy

import (
	"kunpeng.huawei.com/kunpeng-cloud-computing/api/kunpeng-tap/policy-manager/v1alpha1"
	"kunpeng.huawei.com/kunpeng-cloud-computing/pkg/kunpeng-tap/cache"
)

type HookManager interface {
	v1alpha1.RuntimeHookServiceClient
}

// HookType identifies the container lifecycle hook point
type HookType string

const (
	PreRunPodSandbox            HookType = "PreRunPodSandbox"
	PostStopPodSandbox          HookType = "PostStopPodSandbox"
	PreRemovePodSandbox         HookType = "PreRemovePodSandbox"
	PreCreateContainer          HookType = "PreCreateContainer"
	PreStartContainer           HookType = "PreStartContainer"
	PostStartContainer          HookType = "PostStartContainer"
	PostStopContainer           HookType = "PostStopContainer"
	PreRemoveContainer          HookType = "PreRemoveContainer"
	PreUpdateContainerResources HookType = "PreUpdateContainerResources"
	NoneHookType                HookType = "NoneHookType"
)

// Policy defines the interface for a resource allocation policy
type Policy interface {
	// Name returns the unique name of the policy
	Name() string

	// Description returns the description of the policy
	Description() string

	// PreRunPodSandboxHook is called before a pod sandbox is created
	PreRunPodSandboxHook(ctx HookContext) (*Allocation, error)

	// PostStopPodSandboxHook is called after a pod sandbox is stopped
	PostStopPodSandboxHook(ctx HookContext) (*Allocation, error)

	// PreRemovePodSandboxHook is called before a pod sandbox is removed
	PreRemovePodSandboxHook(ctx HookContext) (*Allocation, error)

	// PreCreateContainerHook is called before a container is created
	PreCreateContainerHook(ctx HookContext) (*Allocation, error)

	// PreStartContainerHook is called before a container is started
	PreStartContainerHook(ctx HookContext) (*Allocation, error)

	// PostStartContainerHook is called after a container is started
	PostStartContainerHook(ctx HookContext) (*Allocation, error)

	// PostStopContainerHook is called after a container is stopped
	PostStopContainerHook(ctx HookContext) (*Allocation, error)

	// PreRemoveContainerHook is called before a container is removed
	PreRemoveContainerHook(ctx HookContext) (*Allocation, error)

	// PreUpdateContainerResourcesHook is called before container resources are updated
	PreUpdateContainerResourcesHook(ctx HookContext) (*Allocation, error)

	// SetCache sets the shared cache for the policy
	SetCache(cache cache.Cache)
}

// PolicyManager manages all policies and delegates requests to appropriate policies
type PolicyManager interface {
	v1alpha1.RuntimeHookServiceClient

	// RegisterPolicy registers a policy with the manager
	RegisterPolicy(policy Policy)

	// GetPolicy returns a policy by name
	GetPolicy(name string) Policy

	// ListPolicies returns all registered policies
	ListPolicies() []Policy
}

// BasePolicy 提供所有 Policy 接口方法的默认空实现
type BasePolicy struct {
	cache cache.Cache
	name  string
	desc  string
}

// NewBasePolicy 创建一个新的基础策略
func NewBasePolicy(name, description string) *BasePolicy {
	return &BasePolicy{
		name: name,
		desc: description,
	}
}

// Name 返回策略名称
func (p *BasePolicy) Name() string {
	return p.name
}

// Description 返回策略描述
func (p *BasePolicy) Description() string {
	return p.desc
}

// SetCache 设置策略的缓存
func (p *BasePolicy) SetCache(c cache.Cache) {
	p.cache = c
}

// GetCache 获取策略的缓存
func (p *BasePolicy) GetCache() cache.Cache {
	return p.cache
}

// 所有 Hook 方法的默认空实现
func (p *BasePolicy) PreRunPodSandboxHook(ctx HookContext) (*Allocation, error) {
	return nil, nil
}

func (p *BasePolicy) PostStopPodSandboxHook(ctx HookContext) (*Allocation, error) {
	return nil, nil
}

func (p *BasePolicy) PreRemovePodSandboxHook(ctx HookContext) (*Allocation, error) {
	return nil, nil
}

func (p *BasePolicy) PreCreateContainerHook(ctx HookContext) (*Allocation, error) {
	return nil, nil
}

func (p *BasePolicy) PreStartContainerHook(ctx HookContext) (*Allocation, error) {
	return nil, nil
}

func (p *BasePolicy) PostStartContainerHook(ctx HookContext) (*Allocation, error) {
	return nil, nil
}

func (p *BasePolicy) PostStopContainerHook(ctx HookContext) (*Allocation, error) {
	return nil, nil
}

func (p *BasePolicy) PreRemoveContainerHook(ctx HookContext) (*Allocation, error) {
	return nil, nil
}

func (p *BasePolicy) PreUpdateContainerResourcesHook(ctx HookContext) (*Allocation, error) {
	return nil, nil
}
