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
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
)

const (
	kaeDefaultQos   = "1000"
	kernelDebugPath = "/sys/kernel/debug"
	pciDeviceDir    = "/sys/bus/pci/devices"
	algQosFile      = "alg_qos"
	physfnFile      = "physfn"
	kaeQosKeyPrefix = "qos.kae.kunpeng.com"
)

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

func (qm *QosManager) Run(timeout time.Duration) {
	qm.podResourceManager.Run(timeout)
}

func (qm *QosManager) applyQos(pod *corev1.Pod, getQosValue func(resourceName string) string) error {
	resources := pod.Spec.Resources.Limits
	if resources == nil {
		return nil
	}

	allocateResource := []string{}
	for resourceName := range supportDeivce {
		if _, ok := resources[corev1.ResourceName(resourceName)]; ok {
			allocateResource = append(allocateResource, resourceName)
		}
	}

	namespaceName := pod.Namespace + "/" + pod.Name
	for _, resourceName := range allocateResource {
		qos := getQosValue(resourceName)
		devices := qm.podResourceManager.getDeviceIds(namespaceName, resourceName)
		// podResourcesManager not synced, retrying
		if devices == nil {
			return fmt.Errorf("podResourcesManager not synced, retrying after synchronization is complete")
		}

		if err := setKaeQos(resourceName, devices, qos); err != nil {
			return fmt.Errorf("set qos for %s device [%v] failed: %v", resourceName, devices, err)
		}
	}

	return nil
}

func (qm *QosManager) updateQos(pod *corev1.Pod) error {
	return qm.applyQos(pod, func(resourceName string) string {
		return getQosfromPodAnnotation(resourceName, pod.Annotations)
	})
}

func (qm *QosManager) restoreQos(pod *corev1.Pod) error {
	return qm.applyQos(pod, func(resourceName string) string {
		return kaeDefaultQos
	})
}

func getQosfromPodAnnotation(resourceName string, annotation map[string]string) string {
	if annotation == nil {
		return kaeDefaultQos
	}

	qosKey := resourceNameToQosKey(resourceName)
	if qos, ok := annotation[qosKey]; ok {
		return qos
	}

	return kaeDefaultQos
}

func resourceNameToQosKey(resourceName string) string {
	data := strings.Split(resourceName, "/")
	if len(data) != 2 {
		return ""
	}

	qosKey := kaeQosKeyPrefix + "/" + data[1]

	return qosKey
}

func setKaeQos(qosType string, devices []string, qos string) error {
	if !isValidQos(qos) {
		return fmt.Errorf("invalid qos value: %s", qos)
	}

	for _, device := range devices {
		if err := setDeviceKaeQos(qosType, device, qos); err != nil {
			return err
		}
	}

	return nil
}

func setDeviceKaeQos(qosType string, device string, qos string) error {
	qosPath := filepath.Join(kernelDebugPath, qosType)

	pfDevice, err := getPfDeviceId(device)
	if err != nil {
		return fmt.Errorf("get pf device id failed: %v", err)
	}

	qosFile := filepath.Join(qosPath, pfDevice, algQosFile)
	if _, err := os.Stat(qosFile); os.IsNotExist(err) {
		return fmt.Errorf("qos file %s not exist", qosFile)
	}

	// The KAE QoS is set by "echo [pci] [value] > alg_qos"
	writeData := fmt.Sprintf("%s %s", device, qos)
	if err := os.WriteFile(qosFile, []byte(writeData), 0644); err != nil {
		return fmt.Errorf("write qos file %s failed: %v", qosFile, err)
	}

	return nil
}

func getPfDeviceId(device string) (string, error) {
	vfPhysfn := filepath.Join(pciDeviceDir, device, physfnFile)

	pfDevice, err := filepath.EvalSymlinks(vfPhysfn)
	if err != nil {
		return "", fmt.Errorf("eval symlinks %s failed: %v", vfPhysfn, err)
	}
	pfDevice = filepath.Base(pfDevice)

	return pfDevice, nil
}

// isValidQos checks if the qos value is valid.
// The qos value must be an integer between 1 and 1000.
func isValidQos(qos string) bool {
	value, err := strconv.Atoi(qos)
	if err != nil {
		return false
	}

	return value >= 1 && value <= 1000
}
