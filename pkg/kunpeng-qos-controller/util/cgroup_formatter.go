/*
Copyright 2022 The Koordinator Authors.

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

// Package util 实现常用的工具函数
package util

import (
	"fmt"
	"log"
	"path/filepath"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog"
)

// CgroupDriverType presents driver type, Systemd or Cgroupfs
type CgroupDriverType string

const (
	Cgroupfs CgroupDriverType = "cgroupfs"
	Systemd  CgroupDriverType = "systemd"

	KubeRootNameSystemd       = "kubepods.slice/"
	KubeBurstableNameSystemd  = "kubepods-burstable.slice/"
	KubeBesteffortNameSystemd = "kubepods-besteffort.slice/"

	KubeRootNameCgroupfs       = "kubepods/"
	KubeBurstableNameCgroupfs  = "burstable/"
	KubeBesteffortNameCgroupfs = "besteffort/"

	RuntimeTypeDocker     = "docker"
	RuntimeTypeContainerd = "containerd"
	RuntimeTypeUnknown    = "unknown"
)

// Formatter provide formatter function for cgoup path formatting
type Formatter struct {
	ParentDir string
	QOSDirFn  func(qos corev1.PodQOSClass) string
	PodDirFn  func(qos corev1.PodQOSClass, podUID string) string
	// containerID format: "containerd://..." or "docker://...", return (containerd, HashID)
	ContainerDirFn func(id string) (string, string, error)
}

// CgroupPathFormatter is the formatter for current cgroup driver
var CgroupPathFormatter = GetCgroupFormatter()

// GetCgroupFormatter gets a cgroup path formatter for the current cgroup driver.
// It automatically detects the host's cgroup driver type (systemd or cgroupfs) and returns
func GetCgroupFormatter() Formatter {
	driver := GetCgroupDriver()
	if driver == "" {
		klog.Infof("get cgroup driver failed, use systemdfs as default driver")
		driver = Systemd
	}
	return GetCgroupPathFormatter(driver)
}

var cgroupPathFormatterInSystemd = Formatter{
	ParentDir: KubeRootNameSystemd,
	QOSDirFn: func(qos corev1.PodQOSClass) string {
		switch qos {
		case corev1.PodQOSBurstable:
			return KubeBurstableNameSystemd
		case corev1.PodQOSBestEffort:
			return KubeBesteffortNameSystemd
		case corev1.PodQOSGuaranteed:
			return "/"
		}
		return "/"
	},
	PodDirFn: func(qos corev1.PodQOSClass, podUID string) string {
		id := strings.ReplaceAll(podUID, "-", "_")
		switch qos {
		case corev1.PodQOSBurstable:
			return fmt.Sprintf("kubepods-burstable-pod%s.slice/", id)
		case corev1.PodQOSBestEffort:
			return fmt.Sprintf("kubepods-besteffort-pod%s.slice/", id)
		case corev1.PodQOSGuaranteed:
			return fmt.Sprintf("kubepods-pod%s.slice/", id)
		}
		return "/"
	},
	ContainerDirFn: func(id string) (string, string, error) {
		hashID := strings.Split(id, "://")
		if len(hashID) < 2 {
			return RuntimeTypeUnknown, "", fmt.Errorf("parse container id %s failed", id)
		}

		switch hashID[0] {
		case RuntimeTypeDocker:
			return RuntimeTypeDocker, fmt.Sprintf("docker-%s.scope", hashID[1]), nil
		case RuntimeTypeContainerd:
			return RuntimeTypeContainerd, fmt.Sprintf("cri-containerd-%s.scope", hashID[1]), nil
		default:
			return RuntimeTypeUnknown, "", fmt.Errorf("unknown container protocol %s", id)
		}
	},
}

var cgroupPathFormatterInCgroupfs = Formatter{
	ParentDir: KubeRootNameCgroupfs,
	QOSDirFn: func(qos corev1.PodQOSClass) string {
		switch qos {
		case corev1.PodQOSBurstable:
			return KubeBurstableNameCgroupfs
		case corev1.PodQOSBestEffort:
			return KubeBesteffortNameCgroupfs
		case corev1.PodQOSGuaranteed:
			return "/"
		}
		return "/"
	},
	PodDirFn: func(qos corev1.PodQOSClass, podUID string) string {
		return fmt.Sprintf("pod%s/", podUID)
	},
	ContainerDirFn: func(id string) (string, string, error) {
		hashID := strings.Split(id, "://")
		if len(hashID) < 2 {
			return RuntimeTypeUnknown, "", fmt.Errorf("parse container id %s failed", id)
		}
		if hashID[0] == RuntimeTypeDocker || hashID[0] == RuntimeTypeContainerd {
			return hashID[0], hashID[1], nil
		} else {
			return RuntimeTypeUnknown, "", fmt.Errorf("unknown container protocol %s", id)
		}
	},
}

// GetCgroupPathFormatter gets the Formatter for current cgroup driver
func GetCgroupPathFormatter(driver CgroupDriverType) Formatter {
	switch driver {
	case Systemd:
		return cgroupPathFormatterInSystemd
	case Cgroupfs:
		return cgroupPathFormatterInCgroupfs
	default:
		log.Printf("cgroup driver formatter not supported: %s\n", string(driver))
		return cgroupPathFormatterInSystemd
	}
}

// GetCgroupDriver gets current os cgroup driver
func GetCgroupDriver() CgroupDriverType {
	isSystemd := PathExist(filepath.Join("/sys/fs/cgroup/cpu", KubeRootNameSystemd))
	if isSystemd {
		return Systemd
	}

	isCgroupfs := PathExist(filepath.Join("/sys/fs/cgroup/cpu", KubeRootNameCgroupfs))
	if isCgroupfs {
		return Cgroupfs
	}

	return ""
}
