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
	"context"
	"fmt"
	"sort"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"kunpeng.huawei.com/kunpeng-cloud-computing/pkg/k8s-mpam-controller/util"
)

// OnlinePodSource provides online pod cgroup paths for local node.
type OnlinePodSource interface {
	ListOnlinePodCgroups(ctx context.Context, nodeName string) ([]OnlinePodCgroup, error)
}

// LocalOnlinePodSource lists online pod cgroup paths from kube-apiserver.
type LocalOnlinePodSource struct {
	Client client.Client
}

// NewLocalOnlinePodSource creates online pod source from controller-runtime client.
func NewLocalOnlinePodSource(c client.Client) *LocalOnlinePodSource {
	return &LocalOnlinePodSource{Client: c}
}

// ListOnlinePodCgroups returns online pod cgroup paths on one node.
func (s *LocalOnlinePodSource) ListOnlinePodCgroups(ctx context.Context, nodeName string) ([]OnlinePodCgroup, error) {
	if nodeName == "" {
		return nil, fmt.Errorf("node name must not be empty")
	}

	var podList corev1.PodList
	if err := s.Client.List(ctx, &podList, client.MatchingLabels{
		WorkloadClassLabelKey: WorkloadClassOnline,
	}); err != nil {
		return nil, err
	}

	out := make([]OnlinePodCgroup, 0)
	for i := range podList.Items {
		pod := &podList.Items[i]
		if pod.Spec.NodeName != nodeName {
			continue
		}
		if pod.Status.Phase != corev1.PodRunning {
			continue
		}
		if pod.UID == "" {
			continue
		}

		out = append(out, OnlinePodCgroup{
			Namespace:  pod.Namespace,
			Name:       pod.Name,
			UID:        string(pod.UID),
			CgroupPath: util.GetPodCgroupParentDir(pod),
		})
	}

	// Keep output stable for tests/logging.
	sort.Slice(out, func(i, j int) bool {
		if out[i].Namespace != out[j].Namespace {
			return out[i].Namespace < out[j].Namespace
		}
		return out[i].Name < out[j].Name
	})

	klog.Infof("dynamic-control discovered online pods on node %s: count=%d", nodeName, len(out))
	for _, pod := range out {
		klog.V(2).Infof(
			"dynamic-control online pod: node=%s pod=%s/%s uid=%s cgroup_path=%s",
			nodeName, pod.Namespace, pod.Name, pod.UID, pod.CgroupPath,
		)
	}

	return out, nil
}
