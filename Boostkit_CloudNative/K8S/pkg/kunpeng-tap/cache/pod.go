// Copyright 2019 Intel Corporation. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cache

import (
	"strings"

	"kunpeng.huawei.com/kunpeng-cloud-computing/api/kunpeng-tap/policy-manager/v1alpha1"

	v1 "k8s.io/api/core/v1"
	criv1 "k8s.io/cri-api/pkg/apis/runtime/v1"
	"k8s.io/klog/v2"
)

// Create a pod from a run request.
func (p *pod) fromRunRequest(req *criv1.RunPodSandboxRequest) error {
	// need implementation
	return nil
}

// Create a pod from a list response.
func (p *pod) fromListResponse(pod *criv1.PodSandbox, status *PodStatus) error {
	// need implementation
	return nil
}

// Create a pod from a run request.
func (p *pod) fromDockerRunRequest(pInfo *v1alpha1.PodSandboxHookRequest) error {
	p.containers = make(map[string]string)
	p.UID = pInfo.PodMeta.Uid
	p.Name = pInfo.PodMeta.Name
	p.Namespace = pInfo.PodMeta.Namespace
	p.Labels = pInfo.Labels
	p.Annotations = pInfo.Annotations
	p.State = PodState(int32(PodStateReady))
	p.CgroupParent = pInfo.CgroupParent
	p.LinuxReq = pInfo.Resources
	p.Resources = v1.ResourceRequirements{} // fix this

	klog.V(5).InfoS("PodSandboxHookRequest", "podInfo", pInfo)
	// TODO: check if cgroup parent is right
	if err := p.discoverQOSClass(); err != nil {
		klog.ErrorS(err, "Failed to discover QoS class")
	}

	return nil
}

// Get the normal containers of a pod.
func (p *pod) GetContainers() []Container {
	containers := []Container{}

	for _, c := range p.cache.Containers {
		if c.PodID != p.ID {
			continue
		}
		containers = append(containers, c)
	}

	return containers
}

// Get container pointer by its name.
func (p *pod) getContainer(name string) *container {
	var found *container

	if id, ok := p.containers[name]; ok {
		return p.cache.Containers[id]
	}

	for _, c := range p.GetContainers() {
		cptr := c.(*container)
		p.containers[cptr.Name] = cptr.ID
		if cptr.Name == name {
			found = cptr
		}
	}

	return found
}

// Get container by its name.
func (p *pod) GetContainer(name string) (Container, bool) {
	c := p.getContainer(name)

	return c, c != nil
}

// Get the id of a pod.
func (p *pod) GetID() string {
	return p.ID
}

// Get the (k8s) unique id of a pod.
func (p *pod) GetUID() string {
	return p.UID
}

// Get the name of a pod.
func (p *pod) GetName() string {
	return p.Name
}

// Get the namespace of a pod.
func (p *pod) GetNamespace() string {
	return p.Namespace
}

// Get the PodState of a pod.
func (p *pod) GetState() PodState {
	return p.State
}

// Get the cgroup parent directory of a pod, if known.
func (p *pod) GetCgroupParentDir() string {
	return p.CgroupParent
}

// discover a pod's QoS class by parsing the cgroup parent directory.
func (p *pod) discoverQOSClass() error {
	if p.CgroupParent == "" {
		p.QOSClass = v1.PodQOSBestEffort
		return cacheError("%s: unknown cgroup parent/QoS class, maybe fail", p.ID)
	}
	cgroupParent := strings.TrimPrefix(p.CgroupParent, "/")
	dirs := strings.Split(cgroupParent, "/")
	if len(dirs) < 1 {
		return cacheError("%s: failed to parse %q for QoS class",
			p.ID, p.CgroupParent)

	}

	// consume any potential --cgroup-root passed to kubelet
	if dirs[0] != "kubepods.slice" && dirs[0] != "kubepods" {
		dirs = dirs[1:]
	}
	if len(dirs) < 1 {
		return cacheError("%s: failed to parse %q for QoS class",
			p.ID, p.CgroupParent)
	}

	// consume potential kubepods[.slice]
	if dirs[0] == "kubepods.slice" || dirs[0] == "kubepods" {
		dirs = dirs[1:]
	}
	if len(dirs) < 1 {
		return cacheError("%s: failed to parse %q for QoS class",
			p.ID, p.CgroupParent)
	}

	// check for besteffort, burstable, or lack thereof indicating guaranteed
	klog.V(4).InfoS("Discovered pod QoS class", "podID", p.ID, "qosClass", dirs[0])
	switch dir := dirs[0]; {
	case dir == "kubepods-besteffort.slice" || dir == "besteffort":
		p.QOSClass = v1.PodQOSBestEffort
		return nil
	case dir == "kubepods-burstable.slice" || dir == "burstable":
		p.QOSClass = v1.PodQOSBurstable
		return nil
	case strings.HasPrefix(dir, "kubepods-pod") || strings.HasPrefix(dir, "pod"):
		p.QOSClass = v1.PodQOSGuaranteed
		return nil
	}

	return cacheError("%s: failed to parse %q for QoS class",
		p.ID, p.CgroupParent)
}

// Get the resource requirements of a pod.
func (p *pod) GetPodResourceRequirements() v1.ResourceRequirements {
	return p.Resources
}

// Determine the QoS class of the pod.
func (p *pod) GetQOSClass() v1.PodQOSClass {
	return p.QOSClass
}

// String returns a string representation of pod.
func (p *pod) String() string {
	return p.Name
}

func (p *pod) GetLinuxResources() *v1alpha1.LinuxContainerResources {
	return p.LinuxReq
}

// GetLabels returns the labels of the pod.
func (p *pod) GetLabels() map[string]string {
	return p.Labels
}

// GetAnnotations returns the annotations of the pod.
func (p *pod) GetAnnotations() map[string]string {
	return p.Annotations
}
