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

package kaeqos

import (
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
)

var kaeQosKey = []string{
	"qos.kae.kunpeng.com/hisi_hpre",
	"qos.kae.kunpeng.com/hisi_zip",
	"qos.kae.kunpeng.com/hisi_sec2",
}

type QosManager struct {
	podResourceManager *podResourceManager
}

func NewQosManager(syncPeriod time.Duration) (*QosManager, error) {
	podResourceManager, err := NewPodResourceManager(syncPeriod)
	if err != nil {
		return nil, err
	}

	return &QosManager{
		podResourceManager: podResourceManager,
	}, nil
}

func (qm *QosManager) updateQos(pod *corev1.Pod) error {
	annotation := pod.GetAnnotations()
	if annotation == nil {
		return nil
	}

	namespaceName := pod.Namespace + "/" + pod.Name
	for _, qosKey := range kaeQosKey {
		if qos, ok := annotation[qosKey]; ok {
			resourceName := qosKeyToResourceName(qosKey)
			devices := qm.podResourceManager.getDeviceIds(namespaceName, resourceName)
			err := setKaeQos(devices, qos)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func qosKeyToResourceName(qosKey string) string {
	data := strings.Split(qosKey, "/")
	if len(data) != 2 {
		return ""
	}

	resourceType := data[1]
	resourceName := "kae.kunpeng.com" + "/" + resourceType

	return resourceName
}

func setKaeQos(devices []string, qos string) error {
	return nil
}
