/*
 * Copyright 2022 The Koordinator Authors.
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
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"go.uber.org/atomic"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"

	koor "kunpeng.huawei.com/kunpeng-cloud-computing/api/kunpeng-tap/policy-manager/v1alpha1"
	"kunpeng.huawei.com/kunpeng-cloud-computing/pkg/kunpeng-tap/cache"
)

type ContainerContext struct {
	Request  ContainerRequest
	Response ContainerResponse
}

func (cc *ContainerContext) GetContainerID() string {
	return cc.Request.ContainerMeta.ID
}

func (cc *ContainerContext) FromProxy(req interface{}) {
	request, ok := req.(*koor.ContainerResourceHookRequest)
	if !ok {
		klog.ErrorS(fmt.Errorf("request is not ContainerResourceHookRequest"), "Failed to get ContainerResourceHookRequest")
		return
	}
	if request == nil {
		klog.ErrorS(fmt.Errorf("request is nil"), "Failed to get ContainerResourceHookRequest")
		return
	}
	cc.Request.FromProxy(request)
}

func (cc *ContainerContext) Update() {
	// Not Implemented
}

func (cc *ContainerContext) ToProxy(resp *koor.ContainerResourceHookResponse) {
	cc.Response.ProxyDone(resp)
}

const (
	CgroupVersionV1 CgroupVersion = 1
	CgroupVersionV2 CgroupVersion = 2

	CgroupCPUDir string = "cpu/"

	// NodeDomainPrefix represents the node domain prefix
	NodeDomainPrefix = "node.koordinator.sh"

	// AnnotationExtendedResourceSpec specifies the resource requirements of extended resources for internal usage.
	// It annotates the requests/limits of extended resources and can be used by runtime proxy and koordlet that
	// cannot get the original pod spec in CRI requests.
	AnnotationExtendedResourceSpec = NodeDomainPrefix + "/extended-resource-spec"

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
	RuntimeTypePouch      = "pouch"
	RuntimeTypeCrio       = "cri-o"
	RuntimeTypeUnknown    = "unknown"
)

var (
	Conf         = NewDsModeConfig()
	UseCgroupsV2 = atomic.NewBool(false)

	// CgroupPathFormatter is the cgroup driver formatter.
	// It is initialized with a fastly looked-up type and will be slowly detected with the kubelet when the daemon starts.
	CgroupPathFormatter = GetCgroupFormatter()

	cgroupPathFormatterInSystemd = Formatter{
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
			case RuntimeTypePouch:
				return RuntimeTypePouch, fmt.Sprintf("pouch-%s.scope", hashID[1]), nil
			case RuntimeTypeCrio:
				return RuntimeTypeCrio, fmt.Sprintf("crio-%s.scope", hashID[1]), nil
			default:
				return RuntimeTypeUnknown, "", fmt.Errorf("unknown container protocol %s", id)
			}
		},
		PodIDParser: func(basename string) (string, error) {
			patterns := []struct {
				prefix string
				suffix string
			}{
				{
					prefix: "kubepods-besteffort-pod",
					suffix: ".slice",
				},
				{
					prefix: "kubepods-burstable-pod",
					suffix: ".slice",
				},
				{
					prefix: "kubepods-pod",
					suffix: ".slice",
				},
			}

			for i := range patterns {
				if strings.HasPrefix(basename, patterns[i].prefix) && strings.HasSuffix(basename, patterns[i].suffix) {
					return basename[len(patterns[i].prefix) : len(basename)-len(patterns[i].suffix)], nil
				}
			}
			return "", fmt.Errorf("fail to parse pod id: %v", basename)
		},
		ContainerIDParser: func(basename string) (string, error) {
			patterns := []struct {
				prefix string
				suffix string
			}{
				{
					prefix: "docker-",
					suffix: ".scope",
				},
				{
					prefix: "cri-containerd-",
					suffix: ".scope",
				},
				{
					prefix: "crio-",
					suffix: ".scope",
				},
			}

			for i := range patterns {
				if strings.HasPrefix(basename, patterns[i].prefix) && strings.HasSuffix(basename, patterns[i].suffix) {
					return basename[len(patterns[i].prefix) : len(basename)-len(patterns[i].suffix)], nil
				}
			}
			if strings.HasPrefix(basename, "crio-") {
				return basename[len("crio-"):], nil
			}
			err := fmt.Errorf("fail to parse container id: %v", basename)
			klog.ErrorS(err, "Failed to parse container ID", "basename", basename)
			return "", err
		},
	}

	cgroupPathFormatterInCgroupfs = Formatter{
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
				err := fmt.Errorf("parse container id %s failed", id)
				klog.ErrorS(err, "Failed to parse container ID", "id", id)
				return RuntimeTypeUnknown, "", err
			}
			if hashID[0] == RuntimeTypeDocker || hashID[0] == RuntimeTypeContainerd || hashID[0] == RuntimeTypePouch || hashID[0] == RuntimeTypeCrio {
				return hashID[0], hashID[1], nil
			} else {
				err := fmt.Errorf("unknown container protocol %s", id)
				klog.ErrorS(err, "Unknown container protocol", "id", id)
				return RuntimeTypeUnknown, "", err
			}
		},
		PodIDParser: func(basename string) (string, error) {
			if strings.HasPrefix(basename, "pod") {
				return basename[len("pod"):], nil
			}
			err := fmt.Errorf("fail to parse pod id: %v", basename)
			klog.ErrorS(err, "Failed to parse pod ID", "basename", basename)
			return "", err
		},
		ContainerIDParser: func(basename string) (string, error) {
			return basename, nil
		},
	}
)

func NewDsModeConfig() *Config {
	return &Config{
		CgroupRootDir: "/host-cgroup/",
		// some dirs are not covered by ns, or unused with `hostPID` is on
		ProcRootDir:           "/proc/",
		SysRootDir:            "/host-sys/",
		SysFSRootDir:          "/host-sys-fs/",
		VarRunRootDir:         "/host-var-run/",
		VarLibKubeletRootDir:  "/var/lib/kubelet/",
		RunRootDir:            "/host-run/",
		RuntimeHooksConfigDir: "/host-etc-hookserver/",
		DefaultRuntimeType:    "containerd",
	}
}

type Config struct {
	CgroupRootDir         string
	SysRootDir            string
	SysFSRootDir          string
	ProcRootDir           string
	VarRunRootDir         string
	VarLibKubeletRootDir  string
	RunRootDir            string
	RuntimeHooksConfigDir string

	ContainerdEndPoint string
	PouchEndpoint      string
	DockerEndPoint     string
	CrioEndPoint       string
	DefaultRuntimeType string
}

type ContainerMeta struct {
	Name string
	ID   string // docker://xxx; containerd://
	// is sandbox container
	Sandbox bool
}

func (c *ContainerMeta) FromProxy(containerMeta *koor.ContainerMetadata, podAnnotations map[string]string) {
	c.Name = containerMeta.GetName()
	uid := containerMeta.GetId()
	c.ID = getContainerID(podAnnotations, uid)
}

func getContainerID(podAnnotations map[string]string, containerUID string) string {
	// TODO parse from runtime hook request directly such as cgroup path format
	runtimeType := Conf.DefaultRuntimeType
	if _, exist := podAnnotations["io.kubernetes.docker.type"]; exist {
		runtimeType = RuntimeTypeDocker
	}
	return fmt.Sprintf("%s://%s", runtimeType, containerUID)
}

type PodMeta struct {
	Namespace string
	Name      string
	UID       string
}

func (p *PodMeta) FromProxy(meta *koor.PodSandboxMetadata) {
	p.Namespace = meta.GetNamespace()
	p.Name = meta.GetName()
	p.UID = meta.GetUid()
}

type ContainerRequest struct {
	PodMeta           PodMeta
	ContainerMeta     ContainerMeta
	PodLabels         map[string]string
	PodAnnotations    map[string]string
	CgroupParent      string
	ContainerEnvs     map[string]string
	Resources         *Resources // TODO: support proxy & nri mode
	ExtendedResources *ExtendedResourceContainerSpec
}

func (c *ContainerRequest) FromProxy(req *koor.ContainerResourceHookRequest) {
	c.PodMeta.FromProxy(req.PodMeta)
	c.ContainerMeta.FromProxy(req.ContainerMeta, req.PodAnnotations)
	c.PodLabels = req.GetPodLabels()
	c.PodAnnotations = req.GetPodAnnotations()
	var err error
	c.CgroupParent, err = GetContainerCgroupParentDirByID(req.GetPodCgroupParent(), c.ContainerMeta.ID)
	if err != nil {
		klog.ErrorS(err, "Failed to get container cgroup parent dir",
			"podCgroupParent", req.GetPodCgroupParent(),
			"containerID", c.ContainerMeta.ID)
		// Set a fallback value or keep the original pod cgroup parent
		c.CgroupParent = req.GetPodCgroupParent()
	}

	c.ContainerEnvs = req.GetContainerEnvs()

	if c.Resources == nil {
		c.Resources = &Resources{}
	}

	c.Resources.FromProxy(req)

	// retrieve ExtendedResources from pod annotations
	spec, err := GetExtendedResourceSpec(req.GetPodAnnotations())
	if err != nil {
		klog.V(5).InfoS("Failed to get ExtendedResourceSpec from proxy via annotation",
			"namespace", c.PodMeta.Namespace,
			"podName", c.PodMeta.Name,
			"containerName", c.ContainerMeta.Name,
			"error", err)
	}
	if spec != nil && spec.Containers != nil {
		if containerSpec, ok := spec.Containers[c.ContainerMeta.Name]; ok {
			c.ExtendedResources = &containerSpec
		}
	}
}

// GetContainerCgroupParentDirByID gets the full container cgroup parent dir with the podParentDir and the container ID.
// @parentDir kubepods.slice/kubepods.slice/kubepods-burstable.slice/kubepods-burstable-pod7712555c_ce62_454a_9e18_9ff0217b8941.slice/
// @return kubepods.slice/kubepods-burstable.slice/kubepods-burstable-pod7712555c_ce62_454a_9e18_9ff0217b8941.slice/****.scope
func GetContainerCgroupParentDirByID(podParentDir string, containerID string) (string, error) {
	_, containerDir, err := CgroupPathFormatter.ContainerDirFn(containerID)
	if err != nil {
		return "", err
	}
	return filepath.Join(
		podParentDir,
		containerDir,
	), nil
}

type Formatter struct {
	ParentDir string
	QOSDirFn  func(qos corev1.PodQOSClass) string
	PodDirFn  func(qos corev1.PodQOSClass, podUID string) string
	// containerID format: "containerd://..." or "docker://...", return (containerd, HashID)
	ContainerDirFn func(id string) (string, string, error)

	PodIDParser       func(basename string) (string, error)
	ContainerIDParser func(basename string) (string, error)
}

// GetCgroupFormatter gets the cgroup formatter simply looking up the cgroup directory names.
func GetCgroupFormatter() Formatter {
	nodeName := os.Getenv("NODE_NAME")
	// setup cgroup path formatter from cgroup driver type
	driver := GetCgroupDriverFromCgroupName()
	if driver.Validate() {
		klog.InfoS("Guessed cgroup driver from cgroup name", "node", nodeName, "driver", string(driver))
		return GetCgroupPathFormatter(driver)
	}
	klog.V(4).InfoS("Couldn't guess cgroup driver from kubepods cgroup name")
	return cgroupPathFormatterInSystemd
}

func (c CgroupDriverType) Validate() bool {
	return c == Cgroupfs || c == Systemd
}

func GetCgroupDriverFromCgroupName() CgroupDriverType {
	isSystemd := FileExists(filepath.Join(GetRootCgroupSubfsDir(CgroupCPUDir), KubeRootNameSystemd))
	if isSystemd {
		return Systemd
	}

	isCgroupfs := FileExists(filepath.Join(GetRootCgroupSubfsDir(CgroupCPUDir), KubeRootNameCgroupfs))
	if isCgroupfs {
		return Cgroupfs
	}

	return ""
}

type CgroupVersion int32

func GetCurrentCgroupVersion() CgroupVersion {
	if UseCgroupsV2.Load() {
		return CgroupVersionV2
	}
	return CgroupVersionV1
}

func GetRootCgroupSubfsDir(subfs string) string {
	if GetCurrentCgroupVersion() == CgroupVersionV2 {
		return filepath.Join(Conf.CgroupRootDir)
	}
	return filepath.Join(Conf.CgroupRootDir, subfs)
}
func FileExists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}

type CgroupDriverType string

func GetCgroupPathFormatter(driver CgroupDriverType) Formatter {
	switch driver {
	case Systemd:
		return cgroupPathFormatterInSystemd
	case Cgroupfs:
		return cgroupPathFormatterInCgroupfs
	default:
		klog.V(0).InfoS("Cgroup driver formatter not supported", "driver", string(driver))
		return cgroupPathFormatterInSystemd
	}
}

// GetExtendedResourceSpec parses ExtendedResourceSpec from annotations
func GetExtendedResourceSpec(annotations map[string]string) (*ExtendedResourceSpec, error) {
	spec := &ExtendedResourceSpec{}
	if annotations == nil {
		return spec, nil
	}
	data, ok := annotations[AnnotationExtendedResourceSpec]
	if !ok {
		return spec, nil
	}
	err := json.Unmarshal([]byte(data), spec)
	if err != nil {
		return nil, err
	}
	return spec, nil
}

type ExtendedResourceSpec struct {
	Containers map[string]ExtendedResourceContainerSpec `json:"containers,omitempty"`
}

type ExtendedResourceContainerSpec struct {
	Limits   corev1.ResourceList `json:"limits,omitempty"`
	Requests corev1.ResourceList `json:"requests,omitempty"`
}

type ContainerResponse struct {
	Resources           Resources
	AddContainerEnvs    map[string]string
	AddContainerMounts  []*Mount
	AddContainerDevices []*LinuxDevice
}

type Resctrl struct {
	Schemata   string
	Closid     string
	NewTaskIds []int32
}

type Resources struct {
	EstimatedRequirements *corev1.ResourceRequirements
	// origin resources
	CpuPeriod          *int64
	CpuShares          *int64
	CpuQuota           *int64
	CpuSetCpus         *string
	CpuSetMems         *string
	MemoryLimitInBytes *int64
	NetClsClassId      *uint32

	// extended resources
	CPUBvt  *int64
	CPUIdle *int64
	Resctrl *Resctrl
}

func (r *Resources) IsOriginResSet() bool {
	return r.CpuPeriod != nil || r.CpuQuota != nil || r.CpuShares != nil ||
		r.CpuSetCpus != nil || r.MemoryLimitInBytes != nil || r.CpuSetMems != nil
}

func (r *Resources) FromProxy(req *koor.ContainerResourceHookRequest) {
	ctrRes := req.ContainerResources
	if ctrRes == nil {
		klog.ErrorS(fmt.Errorf("linux container resources is nil"), "Failed to get Linux Container Resources")
		return
	}
	r.EstimatedRequirements = cache.EstimateRequirements(ctrRes, req.PodCgroupParent)
	r.CpuPeriod = &ctrRes.CpuPeriod
	r.CpuQuota = &ctrRes.CpuQuota
	r.CpuShares = &ctrRes.CpuShares
	r.CpuSetCpus = &ctrRes.CpusetCpus
	r.CpuSetMems = &ctrRes.CpusetMems
	r.MemoryLimitInBytes = &ctrRes.MemoryLimitInBytes
}

func (r *Resources) GetRequests() *corev1.ResourceList {
	if r.EstimatedRequirements != nil && r.EstimatedRequirements.Requests != nil {
		return &r.EstimatedRequirements.Requests
	}
	return nil
}

func (r *Resources) GetLimits() *corev1.ResourceList {
	if r.EstimatedRequirements != nil && r.EstimatedRequirements.Limits != nil {
		return &r.EstimatedRequirements.Limits
	}
	return nil
}

type Mount struct {
	Destination string   `protobuf:"bytes,1,opt,name=destination,proto3" json:"destination,omitempty"`
	Type        string   `protobuf:"bytes,2,opt,name=type,proto3" json:"type,omitempty"`
	Source      string   `protobuf:"bytes,3,opt,name=source,proto3" json:"source,omitempty"`
	Options     []string `protobuf:"bytes,4,rep,name=options,proto3" json:"options,omitempty"`
}

type LinuxDevice struct {
	Path          string
	Type          string
	Major         int64
	Minor         int64
	FileModeValue uint32
}

func (c *ContainerResponse) ProxyDone(resp *koor.ContainerResourceHookResponse) {
	if c.Resources.IsOriginResSet() && resp.ContainerResources == nil {
		// resource value is injected but origin request is nil, init resource response
		resp.ContainerResources = &koor.LinuxContainerResources{}
	}
	if c.Resources.CpuSetCpus != nil {
		resp.ContainerResources.CpusetCpus = *c.Resources.CpuSetCpus
	}
	if c.Resources.CpuQuota != nil {
		resp.ContainerResources.CpuQuota = *c.Resources.CpuQuota
	}
	if c.Resources.CpuShares != nil {
		resp.ContainerResources.CpuShares = *c.Resources.CpuShares
	}
	if c.Resources.MemoryLimitInBytes != nil {
		resp.ContainerResources.MemoryLimitInBytes = *c.Resources.MemoryLimitInBytes
	}
	if c.AddContainerEnvs != nil {
		if resp.ContainerEnvs == nil {
			resp.ContainerEnvs = make(map[string]string)
		}
		for k, v := range c.AddContainerEnvs {
			resp.ContainerEnvs[k] = v
		}
	}
}
