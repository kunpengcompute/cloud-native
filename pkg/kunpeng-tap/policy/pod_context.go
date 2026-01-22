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
	"fmt"

	"k8s.io/klog/v2"
	"kunpeng.huawei.com/kunpeng-cloud-computing/api/kunpeng-tap/policy-manager/v1alpha1"
)

// Define context structs for different hook types
type PodSandboxContext struct {
	Request *v1alpha1.PodSandboxHookRequest
}

func (ctx *PodSandboxContext) GetRequest() interface{} {
	return ctx.Request
}

func (ctx *PodSandboxContext) FromProxy(req interface{}) {
	request, ok := req.(*v1alpha1.PodSandboxHookRequest)
	if !ok {
		klog.ErrorS(fmt.Errorf("invalid request type"), "Failed to get PodSandboxHookRequest")
		return
	}
	// 如果请求为空，则返回错误
	if request == nil {
		klog.ErrorS(fmt.Errorf("request is nil"), "Failed to get PodSandboxHookRequest")
		return
	}
	ctx.Request = request
}
