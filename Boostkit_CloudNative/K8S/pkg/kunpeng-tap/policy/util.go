/*
 * Copyright 2019 Intel Corporation. All Rights Reserved.
 * Modifications Copyright 2025 Huawei Technology corp.
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
	"strings"

	koor "kunpeng.huawei.com/kunpeng-cloud-computing/api/kunpeng-tap/policy-manager/v1alpha1"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
)

// 计算Pod QOSclass
func ParseCgroupForQOSClass(cgroupPath string) corev1.PodQOSClass {
	dir := cgroupPath
	var qos corev1.PodQOSClass
	switch {
	case strings.Contains(dir, "besteffort"):
		qos = v1.PodQOSBestEffort
	case strings.Contains(dir, "burstable"):
		qos = v1.PodQOSBurstable
	default:
		qos = v1.PodQOSGuaranteed
	}

	if dir == "" {
		return ""
	}
	return qos
}

// cgroupParentToQOS tries to map Pod cgroup parent to QOS class.
func cgroupParentToQOS(dir string) corev1.PodQOSClass {
	var qos corev1.PodQOSClass

	// The parent directory naming scheme depends on the cgroup driver in use.
	// Thus, rely on substring matching
	split := strings.Split(strings.TrimPrefix(dir, "/"), "/")
	switch {
	case len(split) < 2:
		qos = corev1.PodQOSClass("")
	case strings.Index(split[1], strings.ToLower(string(corev1.PodQOSBurstable))) != -1:
		qos = corev1.PodQOSBurstable
	case strings.Index(split[1], strings.ToLower(string(corev1.PodQOSBestEffort))) != -1:
		qos = corev1.PodQOSBestEffort
	default:
		qos = corev1.PodQOSGuaranteed
	}

	return qos
}

func mergeAllocations(allocs []*Allocation, resp *koor.ContainerResourceHookResponse) {
	for _, alloc := range allocs {
		if alloc == nil {
			continue
		}
		alloc.Merge(resp)
	}
	return
}
