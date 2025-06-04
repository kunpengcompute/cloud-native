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

package cache

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"sync"

	"github.com/docker/docker/client"
	v1 "k8s.io/api/core/v1"
	criv1 "k8s.io/cri-api/pkg/apis/runtime/v1"
	"k8s.io/klog/v2"

	"kunpeng.huawei.com/kunpeng-cloud-computing/api/kunpeng-tap/policy-manager/v1alpha1"
	"kunpeng.huawei.com/kunpeng-cloud-computing/pkg/kunpeng-tap/sysfs"
)

const (
	KeyPod       = "pod"
	KeyID        = "id"
	KeyUID       = "uid"
	KeyName      = "name"
	KeyNamespace = "namespace"
	KeyQOSClass  = "qosclass"
)

const (
	// CPU marks changes that can be applied by the CPU controller.
	CPU = "cpu"
	// CRI marks changes that can be applied by the CRI controller.
	CRI = "cri"
	// Memory marks changes that can be applied by the Memory controller.
	Memory = "memory"
)

// PodState is the pod state in the runtime.
type PodState int32

const (
	// PodStateReady marks a pod ready.
	PodStateReady = PodState(int32(criv1.PodSandboxState_SANDBOX_READY))
	// PodStateNotReady marks a pod as not ready.
	PodStateNotReady = PodState(int32(criv1.PodSandboxState_SANDBOX_NOTREADY))
	// PodStateStale marks a pod as removed.
	PodStateStale = PodState(int32(PodStateNotReady) + 1)
)

// PodResourceRequirements are per container resource requirements, annotated by our webhook.
type PodResourceRequirements struct {
	// InitContainers is the resource requirements by init containers.
	InitContainers map[string]v1.ResourceRequirements `json:"initContainers"`
	// Containers is the resource requirements by normal container.
	Containers map[string]v1.ResourceRequirements `json:"containers"`
}

// PodStatus wraps a PodSandboxStatus response for data extraction.
type PodStatus struct {
	CgroupParent string // extracted CgroupParent
}

type NumaNodeResources struct {
	CpuFree  float64
	CpuUsed  float64
	CpuTotal float64
	MemFree  float64
	MemUsed  float64
	MemTotal float64
}

// Pod is the exposed interface from a cached pod.
type Pod interface {
	fmt.Stringer
	// GetContainers returns the (non-init) containers of the pod.
	GetContainers() []Container
	// GetContainer returns the named container of the pod.
	GetContainer(string) (Container, bool)
	// GetId returns the pod id of the pod.
	GetID() string
	// GetUID returns the (kubernetes) unique id of the pod.
	GetUID() string
	// GetName returns the name of the pod.
	GetName() string
	// GetNamespace returns the namespace of the pod.
	GetNamespace() string
	// GetLabels returns the labels of the pod.
	GetLabels() map[string]string
	// GetAnnotations returns the annotations of the pod.
	GetAnnotations() map[string]string
	// GetState returns the PodState of the pod.
	GetState() PodState
	// GetQOSClass returns the PodQOSClass of the pod.
	GetQOSClass() v1.PodQOSClass

	// GetCgroupParentDir returns the pods cgroup parent directory.
	GetCgroupParentDir() string
	// GetPodResourceRequirements returns container resource requirements
	GetPodResourceRequirements() v1.ResourceRequirements

	GetLinuxResources() *v1alpha1.LinuxContainerResources
}

// A cached pod.
type pod struct {
	cache       *cache // our cache of object
	ID          string // pod sandbox runtime id
	UID         string // (k8s) unique id
	Name        string // pod sandbox name
	Namespace   string // pod namespace
	Annotations map[string]string
	Labels      map[string]string
	State       PodState                          // ready/not ready
	QOSClass    v1.PodQOSClass                    // pod QoS class
	Resources   v1.ResourceRequirements           // use total resources requirement for now
	LinuxReq    *v1alpha1.LinuxContainerResources // used to estimate Resources if we lack annotations

	CgroupParent string            // cgroup parent directory
	containers   map[string]string // container name to ID map
}

// ContainerState is the container state in the runtime.
type ContainerState int32

const (
	// ContainerStateCreated marks a container created, not running.
	ContainerStateCreated = ContainerState(int32(criv1.ContainerState_CONTAINER_CREATED))
	// ContainerStateRunning marks a container created, running.
	ContainerStateRunning = ContainerState(int32(criv1.ContainerState_CONTAINER_RUNNING))
	// ContainerStateExited marks a container exited.
	ContainerStateExited = ContainerState(int32(criv1.ContainerState_CONTAINER_EXITED))
	// ContainerStateUnknown marks a container to be in an unknown state.
	ContainerStateUnknown = ContainerState(int32(criv1.ContainerState_CONTAINER_UNKNOWN))
	// ContainerStateCreating marks a container as being created.
	ContainerStateCreating = ContainerState(int32(ContainerStateUnknown) + 1)
	// ContainerStateStale marks a container removed.
	ContainerStateStale = ContainerState(int32(ContainerStateUnknown) + 2)
)

// Container is the exposed interface from a cached container.
type Container interface {
	fmt.Stringer
	// PrettyName returns the user-friendly <podname>:<containername> for the container.
	PrettyName() string
	// GetPod returns the pod of the container and a boolean indicating if there was one.
	GetPod() (Pod, bool)
	// GetID returns the ID of the container.
	GetID() string
	// GetPodID returns the pod ID of the container.
	GetPodID() string
	// GetCacheID returns the cacheID of the container.
	GetCacheID() string
	// GetName returns the name of the container.
	GetName() string
	// GetNamespace returns the namespace of the container.
	GetNamespace() string
	// UpdateState updates the state of the container.
	UpdateState(ContainerState)
	// GetState returns the ContainerState of the container.
	GetState() ContainerState
	// GetQOSClass returns the QoS class the pod would have if this was its only container.
	GetQOSClass() v1.PodQOSClass

	// GetResourceRequirements returns the webhook-annotated requirements for ths container.
	GetResourceRequirements() v1.ResourceRequirements
	// GetLinuxResources returns the CRI linux resource request of the container.
	GetLinuxResources() *v1alpha1.LinuxContainerResources
	// GetCPUPeriod gets the CFS CPU period of the container.
	GetCPUPeriod() int64
	// GetCpuQuota gets the CFS CPU quota of the container.
	GetCPUQuota() int64
	// GetCPUShares gets the CFS CPU shares of the container.
	GetCPUShares() int64
	// GetmemoryLimit gets the memory limit in bytes for the container.
	GetMemoryLimit() int64
	// GetOomScoreAdj gets the OOM score adjustment for the container.
	GetOomScoreAdj() int64
	// GetCpusetCPUs gets the cgroup cpuset.cpus of the container.
	GetCpusetCpus() string
	// GetCpusetMems gets the cgroup cpuset.mems of the container.
	GetCpusetMems() string

	// SetLinuxResources sets the Linux-specific resource request of the container.
	SetLinuxResources(*v1alpha1.LinuxContainerResources)
	// SetCPUPeriod sets the CFS CPU period of the container.
	SetCPUPeriod(int64)
	// SetCPUQuota sets the CFS CPU quota of the container.
	SetCPUQuota(int64)
	// SetCPUShares sets the CFS CPU shares of the container.
	SetCPUShares(int64)
	// SetmemoryLimit sets the memory limit in bytes for the container.
	SetMemoryLimit(int64)
	// SetCpusetCpu sets the cgroup cpuset.cpus of the container.
	SetCpusetCpus(string)
	// SetCpusetMems sets the cgroup cpuset.mems of the container.
	SetCpusetMems(string)

	// GetPending gets the names of the controllers with pending changes.
	GetPending() []string
	// HasPending checks if the container has pending chanhes for the given controller.
	HasPending(string) bool
	// ClearPending clears the pending change marker for the given controller.
	ClearPending(string)
}

// A cached container.
type container struct {
	cache     *cache         // our cache of objects
	ID        string         // container runtime id
	PodID     string         // associate pods runtime id
	CacheID   string         // our cache id
	Name      string         // container name
	Namespace string         // container namespace
	State     ContainerState // created/running/exited/unknown

	Resources v1.ResourceRequirements           // container resources (from webhook annotation)
	LinuxReq  *v1alpha1.LinuxContainerResources // used to estimate Resources if we lack annotations

	pending map[string]struct{} // controllers with pending changes for this container

	prettyName string // cached PrettyName()
}

// Cache is the primary interface exposed for tracking pods and containers.
//
// Cache tracks pods and containers in the runtime, mostly by processing CRI
// requests and responses which the cache is fed as these are being procesed.
// Cache also saves its state upon changes to secondary storage and restores
// itself upon startup.
// Cache can validate its state against the actual system state by checking
// if container cgroups still exist.
type Cache interface {
	// InsertPod inserts a pod into the cache, using a runtime request or reply.
	InsertPod(id string, msg interface{}, status *PodStatus) (Pod, error)
	// DeletePod deletes a pod from the cache.
	DeletePod(id string) Pod
	// LookupPod looks up a pod in the cache.
	LookupPod(id string) (Pod, bool)
	// InsertContainer inserts a container into the cache, using a runtime request or reply.
	InsertContainer(containerId string, msg interface{}) (Container, error)
	// UpdateContainerID updates a containers runtime id.
	UpdateContainerID(cacheID string, msg interface{}) (Container, error)
	// DeleteContainer deletes a container from the cache.
	DeleteContainer(id string) Container
	// LookupContainer looks up a container in the cache.
	LookupContainer(id string) (Container, bool)

	// GetPendingContainers returs all containers with pending changes.
	GetPendingContainers() []Container

	// GetPods returns all the pods known to the cache.
	GetPods() []Pod
	// GetContainers returns all the containers known to the cache.
	GetContainers() []Container

	// GetContainerCacheIds returns the cache ids of all containers.
	GetContainerCacheIds() []string
	// GetContaineIds return the ids of all containers.
	GetContainerIds() []string

	// Save requests a cache save.
	//Save() error

	// RefreshPods purges/inserts stale/new pods/containers using a pod sandbox list response.
	RefreshPods(*criv1.ListPodSandboxResponse, map[string]*PodStatus) ([]Pod, []Pod, []Container)
	// RefreshContainers purges/inserts stale/new containers using a container list response.
	RefreshContainers(*criv1.ListContainersResponse) ([]Container, []Container)

	GetNodeResources() []NumaNodeResources

	// ValidateCachedContainers checks if containers in cache still exist in the system
	// by verifying their cgroup directories, and returns a list of container IDs that
	// should be removed from the cache
	ValidateCachedContainers(containerIds []string) []string

	// CleanupStaleContainers removes containers from cache that no longer exist in the system
	// and returns the number of containers removed
	CleanupStaleContainers(staleContainerIds []string) int

	LoadStoreDocker(dockerClient client.CommonAPIClient, cgroupDriver string) error

	LoadStoreContainerd(backendRuntimeServiceClient criv1.RuntimeServiceClient) error
}

const (
	// CacheVersion is the running version of the cache.
	CacheVersion = "1"
)

// permissions describe preferred/expected ownership and permissions for a file or directory.
type permissions struct {
	prefer os.FileMode // permissions to create file/directory with
	reject os.FileMode // bits that cause rejection to use an existing entry
}

// permissions to create with/check against
var (
	cacheDirPerm  = &permissions{prefer: 0710, reject: 0022}
	cacheFilePerm = &permissions{prefer: 0644, reject: 0022}
	dataDirPerm   = &permissions{prefer: 0755, reject: 0022}
	dataFilePerm  = &permissions{prefer: 0644, reject: 0022}
)

// Our cache of objects.
type cache struct {
	sync.Mutex `json:"-"` // we're lockable
	//logger.Logger `json:"-"` // cache logger instance
	filePath string // where to store to/load from
	dataDir  string // container data directory

	Pods       map[string]*pod       // known/cached pods
	Containers map[string]*container // known/cache containers
	NextID     uint64                // next container cache id to use

	pending map[string]struct{} // cache IDs of containers with pending changes

	CpuInfo *sysfs.LocalCPUInfo
}

// Make sure cache implements Cache.
var _ Cache = &cache{}

// Options contains the configurable cache options.
type Options struct {
	// CacheDir is the directory the cache should save its state in.
	CacheDir string
}

func cacheError(format string, args ...interface{}) error {
	return fmt.Errorf("cache: "+format, args...)
}

// NewCache instantiates a new cache. Load it from the given path if it exists.
func NewCache(options Options) (Cache, error) {
	cch := &cache{
		filePath: filepath.Join(options.CacheDir, "cache"),
		dataDir:  filepath.Join(options.CacheDir, "containers"),
		//Logger:     logger.NewLogger("cache"),
		Pods:       make(map[string]*pod),
		Containers: make(map[string]*container),
		NextID:     1,
	}

	if _, err := cch.checkPerm("cache", cch.filePath, false, cacheFilePerm); err != nil {
		return nil, cacheError("refusing to use existing cache file: %v", err)
	}
	if err := cch.mkdirAll("cache", options.CacheDir, cacheDirPerm); err != nil {
		return nil, err
	}
	if err := cch.mkdirAll("container", cch.dataDir, dataDirPerm); err != nil {
		return nil, err
	}
	//if err := cch.Load(); err != nil {
	//	return nil, err
	//}
	if cpuInfo, err := sysfs.GetLocalCPUInfo(); err != nil {
		return nil, err
	} else {
		cch.CpuInfo = cpuInfo
	}
	return cch, nil
}

// Derive cache id using pod uid, or allocate a new unused local cache id.
func (cch *cache) createCacheID(c *container) string {
	if pod, ok := c.cache.LookupPod(c.PodID); ok {
		uid := pod.GetUID()
		if uid != "" {
			return uid + ":" + c.Name
		}
	}

	klog.InfoS("Couldn't find unique id for", "pod", c.PodID, "assigning local cache id")
	id := "cache:" + strconv.FormatUint(cch.NextID, 16)
	cch.NextID++

	return id
}

// Insert a pod into the cache.
func (cch *cache) InsertPod(id string, msg interface{}, status *PodStatus) (Pod, error) {
	var err error
	if id == "" {
		klog.ErrorS(err, "Failed to insert pod", "id", id)
		return nil, fmt.Errorf("pod ID is empty")
	}

	p := &pod{cache: cch, ID: id}

	cch.Lock()
	defer cch.Unlock()

	switch msg.(type) {
	case *criv1.RunPodSandboxRequest:
		err = p.fromRunRequest(msg.(*criv1.RunPodSandboxRequest))
	case *criv1.PodSandbox:
		err = p.fromListResponse(msg.(*criv1.PodSandbox), status)
	case *v1alpha1.PodSandboxHookRequest:
		klog.V(5).InfoS("inserting pod", "id", id, "name", msg.(*v1alpha1.PodSandboxHookRequest).PodMeta.Name)
		err = p.fromDockerRunRequest(msg.(*v1alpha1.PodSandboxHookRequest))
	default:
		err = fmt.Errorf("cannot create pod from message %T", msg)
	}

	if err != nil {
		klog.ErrorS(err, "Failed to insert pod", "id", id)
		return nil, err
	}

	cch.Pods[p.ID] = p

	// cch.Save()

	return p, nil
}

// Delete a pod from the cache.
func (cch *cache) DeletePod(id string) Pod {
	cch.Lock()
	defer cch.Unlock()

	p, ok := cch.Pods[id]
	if !ok {
		return nil
	}

	klog.V(5).InfoS("Removing pod", "name", p.Name, "id", p.ID)
	delete(cch.Pods, id)

	// cch.Save()

	return p
}

// Look up a pod in the cache.
func (cch *cache) LookupPod(id string) (Pod, bool) {
	cch.Lock()
	defer cch.Unlock()

	p, ok := cch.Pods[id]
	return p, ok
}

// Insert a container into the cache.
func (cch *cache) InsertContainer(containerId string, msg interface{}) (Container, error) {
	cch.Lock()
	defer cch.Unlock()

	var err error

	c := &container{
		cache: cch,
	}

	switch msg.(type) {
	case *criv1.CreateContainerRequest:
		err = c.fromCreateRequest(msg.(*criv1.CreateContainerRequest))
	case *criv1.Container:
		err = c.fromListResponse(msg.(*criv1.Container))
	case *v1alpha1.ContainerResourceHookRequest:
		klog.InfoS("Inserting container", "id", containerId, "name", msg.(*v1alpha1.ContainerResourceHookRequest).ContainerMeta.Name)
		err = c.fromDockerRunRequest(msg.(*v1alpha1.ContainerResourceHookRequest))
	default:
		err = fmt.Errorf("cannot create container from message %T", msg)
	}

	if err != nil {
		return nil, cacheError("failed to insert container %s: %v", c.CacheID, err)
	}

	// c.CacheID = cch.createCacheID(c)

	// cch.Containers[c.CacheID] = c
	// if c.ID != "" {
	// 	cch.Containers[containerId] = c
	// }
	c.ID = containerId
	cch.Containers[containerId] = c

	// cch.Save()

	return c, nil
}

// UpdateContainerID updates a containers runtime id.
func (cch *cache) UpdateContainerID(cacheID string, msg interface{}) (Container, error) {
	cch.Lock()
	defer cch.Unlock()

	c, ok := cch.Containers[cacheID]
	if !ok {
		return nil, cacheError("%s: failed to update ID, container not found",
			cacheID)
	}

	reply, ok := msg.(*criv1.CreateContainerResponse)
	if !ok {
		return nil, cacheError("%s: failed to update ID from message %T",
			c.PrettyName(), msg)
	}

	c.ID = reply.ContainerId
	cch.Containers[c.ID] = c

	// cch.Save()

	return c, nil
}

// Delete a pod from the cache.
func (cch *cache) DeleteContainer(id string) Container {
	cch.Lock()
	defer cch.Unlock()

	c, ok := cch.Containers[id]
	if !ok {
		return nil
	}

	klog.V(5).InfoS("Removing container", "name", c.PrettyName())
	delete(cch.Containers, c.ID)
	delete(cch.Containers, c.CacheID)

	// cch.Save()

	return c
}

// Look up a container in the cache.
func (cch *cache) LookupContainer(id string) (Container, bool) {
	cch.Lock()
	defer cch.Unlock()
	c, ok := cch.Containers[id]
	return c, ok
}

// RefreshPods purges/inserts stale/new pods/containers using a pod sandbox list response.
func (cch *cache) RefreshPods(msg *criv1.ListPodSandboxResponse, status map[string]*PodStatus) ([]Pod, []Pod, []Container) {
	valid := make(map[string]struct{})

	add := []Pod{}
	del := []Pod{}
	containers := []Container{}

	cch.Lock()
	defer cch.Unlock()

	for _, item := range msg.Items {
		valid[item.Id] = struct{}{}
		if _, ok := cch.Pods[item.Id]; !ok {
			klog.V(5).InfoS("Inserting discovered pod", "id", item.Id)
			pod, err := cch.InsertPod(item.Id, item, status[item.Id])
			if err != nil {
				klog.ErrorS(err, "Failed to insert discovered pod to cache", "id", item.Id)
			} else {
				add = append(add, pod)
			}
		}
	}

	for _, pod := range cch.Pods {
		if _, ok := valid[pod.ID]; !ok {
			klog.V(5).InfoS("Purging stale pod", "id", pod.ID)
			pod.State = PodStateStale
			del = append(del, cch.DeletePod(pod.ID))
		}
	}

	for id, c := range cch.Containers {
		if _, ok := valid[c.PodID]; !ok {
			klog.V(5).InfoS("Purging container of stale pod", "containerID", c.CacheID, "podID", c.PodID)
			cch.DeleteContainer(c.CacheID)
			c.State = ContainerStateStale
			if id == c.CacheID {
				containers = append(containers, c)
			}
		}
	}

	return add, del, containers
}

// RefreshContainers purges/inserts stale/new containers using a container list response.
func (cch *cache) RefreshContainers(msg *criv1.ListContainersResponse) ([]Container, []Container) {
	valid := make(map[string]struct{})

	cch.Lock()
	defer cch.Unlock()

	add := []Container{}
	del := []Container{}

	for _, c := range msg.Containers {
		if ContainerState(c.State) == ContainerStateExited {
			continue
		}

		valid[c.Id] = struct{}{}
		if _, ok := cch.Containers[c.Id]; !ok {
			klog.V(5).InfoS("Inserting discovered container", "id", c.Id)
			inserted, err := cch.InsertContainer(c.Id, c)
			if err != nil {
				klog.ErrorS(err, "Failed to insert discovered container to cache", "id", c.Id)
			} else {
				add = append(add, inserted)
			}
		}
	}

	for id, c := range cch.Containers {
		if _, ok := valid[c.ID]; !ok {
			klog.V(5).InfoS("Purging stale container", "id", c.CacheID, "state", c.GetState())
			cch.DeleteContainer(c.CacheID)
			c.State = ContainerStateStale
			if id == c.CacheID {
				del = append(del, c)
			}
		}
	}

	return add, del
}

// Mark a container as having pending changes.
func (cch *cache) markPending(c *container) {
	if cch.pending == nil {
		cch.pending = make(map[string]struct{})
	}
	cch.pending[c.CacheID] = struct{}{}
}

// Get all containers with pending changes.
func (cch *cache) GetPendingContainers() []Container {
	pending := make([]Container, 0, len(cch.pending))
	for id := range cch.pending {
		c, ok := cch.LookupContainer(id)
		if ok {
			pending = append(pending, c)
		}
	}
	return pending
}

// clear the pending state of the given container.
func (cch *cache) clearPending(c *container) {
	delete(cch.pending, c.CacheID)
}

// Get the cache ids of all cached containers.
func (cch *cache) GetContainerCacheIds() []string {
	ids := make([]string, len(cch.Containers))

	idx := 0
	for id, c := range cch.Containers {
		if id != c.CacheID {
			continue
		}
		ids[idx] = c.CacheID
		idx++
	}

	return ids[0:idx]
}

// Get the ids of all cached containers.
func (cch *cache) GetContainerIds() []string {
	ids := make([]string, len(cch.Containers))

	idx := 0
	for id, c := range cch.Containers {
		if id == c.CacheID {
			continue
		}
		ids[idx] = c.ID
		idx++
	}

	return ids[0:idx]
}

// GetPods returns all pods present in the cache.
func (cch *cache) GetPods() []Pod {
	cch.Lock()
	defer cch.Unlock()

	pods := make([]Pod, 0, len(cch.Pods))
	for _, pod := range cch.Pods {
		pods = append(pods, pod)
	}
	return pods
}

// GetContainers returns all the containers present in the cache.
func (cch *cache) GetContainers() []Container {
	cch.Lock()
	defer cch.Unlock()

	containers := make([]Container, 0, len(cch.Containers)/2)
	for _, container := range cch.Containers {
		//if id != container.CacheID {
		//	continue
		//}
		containers = append(containers, container)
	}
	return containers
}

func (cch *cache) GetNodeResources() []NumaNodeResources {
	cch.Lock()
	defer cch.Unlock()

	// TODO: 计算 Reserved CPU 数量
	nodes := make([]NumaNodeResources, len(cch.CpuInfo.TotalInfo.NodeToCPU))
	for i := 0; i < len(cch.CpuInfo.TotalInfo.NodeToCPU); i++ {
		nodes[i].CpuTotal = float64(len(cch.CpuInfo.TotalInfo.NodeToCPU[int32(i)]))
		nodes[i].CpuFree = nodes[i].CpuTotal
		nodes[i].CpuUsed = 0.0
	}

	for _, ctr := range cch.Containers {
		res := ctr.GetLinuxResources()
		if res.CpuPeriod == 0 {
			klog.V(3).InfoS("Container has no CPU period", "container", ctr.PrettyName())
		}
		// 按照 limits 计算 CPU 使用量
		cpuTime := float64(res.CpuQuota) / float64(res.CpuPeriod)
		// 只计算 NUMA Node 亲和的 CPU 使用量
		klog.V(5).InfoS("Getting Linux resources", "resources", res, "container", ctr, "cpuTime", cpuTime)
		i := sysfs.GetNumaNodeFromCpuSet(cch.CpuInfo, ctr.GetLinuxResources().GetCpusetCpus())
		klog.V(5).InfoS("Getting NUMA node from CPU set", "nodeID", i, "cpuSet", ctr.GetLinuxResources().GetCpusetCpus())
		if i >= 0 {
			// 依据使用量计算
			nodes[i].CpuUsed += cpuTime
		}
	}
	return nodes
}

// checkPerm checks permissions of an already existing file or directory.
func (cch *cache) checkPerm(what, path string, isDir bool, p *permissions) (bool, error) {
	if isDir {
		what += " directory"
	}

	info, err := os.Stat(path)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return true, cacheError("failed to os.Stat() %s %q: %v", what, path, err)
		}
		return false, nil
	}

	// check expected file type
	if isDir {
		if !info.IsDir() {
			return true, cacheError("%s %q exists, but is not a directory", what, path)
		}
	} else {
		if info.Mode()&os.ModeType != 0 {
			return true, cacheError("%s %q exists, but is not a regular file", what, path)
		}
	}

	existing := info.Mode().Perm()
	expected := p.prefer
	rejected := p.reject
	if ((expected | rejected) &^ os.ModePerm) != 0 {
		klog.ErrorS(fmt.Errorf("current permissions check only handles permission bits (rwx)"), "Internal error in permissions check")
		panic("internal error: current permissions check only handles permission bits (rwx)")
	}

	// check that we don't have any of the rejectable permission bits set
	if existing&rejected != 0 {
		return true, cacheError("existing %s %q has disallowed permissions set: %v",
			what, path, existing&rejected)
	}

	// warn if permissions are less strict than the preferred defaults
	if (existing | expected) != expected {
		klog.V(0).InfoS("Existing path has less strict permissions than expected",
			"type", what,
			"path", path,
			"actualPermissions", existing,
			"expectedPermissions", expected)
	}

	return true, nil
}

// mkdirAll creates a directory, checking permissions if it already exists.
func (cch *cache) mkdirAll(what, path string, p *permissions) error {
	exists, err := cch.checkPerm(what, path, true, p)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}

	if err := os.MkdirAll(path, p.prefer); err != nil {
		return cacheError("failed to create %s directory %q: %v", what, path, err)
	}

	return nil
}

// snapshot is used to serialize the cache into a saveable/loadable state.
type snapshot struct {
	Version    string
	Pods       map[string]*pod
	Containers map[string]*container
	NextID     uint64
}

// Snapshot takes a restorable snapshot of the current state of the cache.
func (cch *cache) Snapshot() ([]byte, error) {
	s := snapshot{
		Version:    CacheVersion,
		Pods:       make(map[string]*pod),
		Containers: make(map[string]*container),
		NextID:     cch.NextID,
	}

	for id, p := range cch.Pods {
		s.Pods[id] = p
	}

	for id, c := range cch.Containers {
		if id == c.CacheID {
			s.Containers[c.CacheID] = c
		}
	}

	data, err := json.Marshal(s)
	if err != nil {
		return nil, cacheError("failed to marshal cache: %v", err)
	}

	return data, nil
}

// Restore restores a previously takes snapshot of the cache.
func (cch *cache) Restore(data []byte) error {
	s := snapshot{
		Pods:       make(map[string]*pod),
		Containers: make(map[string]*container),
	}

	if err := json.Unmarshal(data, &s); err != nil {
		return cacheError("failed to unmarshal snapshot data: %v", err)
	}

	if s.Version != CacheVersion {
		return cacheError("can't restore snapshot, version '%s' != running version %s",
			s.Version, CacheVersion)
	}

	cch.Pods = s.Pods
	cch.Containers = s.Containers
	cch.NextID = s.NextID

	for _, p := range cch.Pods {
		p.cache = cch
		p.containers = make(map[string]string)
	}
	for _, c := range cch.Containers {
		c.cache = cch
		cch.Containers[c.CacheID] = c
		if c.ID != "" {
			cch.Containers[c.ID] = c
		}
	}

	return nil
}

// Save the state of the cache.
func (cch *cache) Save() error {
	klog.InfoS("Saving cache to file", "path", cch.filePath)

	data, err := cch.Snapshot()
	if err != nil {
		return cacheError("failed to save cache: %v", err)
	}

	tmpPath := cch.filePath + ".saving"
	if err = os.WriteFile(tmpPath, data, cacheFilePerm.prefer); err != nil {
		return cacheError("failed to write cache to file %q: %v", tmpPath, err)
	}
	if err := os.Rename(tmpPath, cch.filePath); err != nil {
		return cacheError("failed to rename %q to %q: %v",
			tmpPath, cch.filePath, err)
	}

	return nil
}

// Load loads the last saved state of the cache.
func (cch *cache) Load() error {
	klog.InfoS("Loading cache from file", "path", cch.filePath)

	data, err := os.ReadFile(cch.filePath)

	switch {
	case os.IsNotExist(err):
		klog.ErrorS(fmt.Errorf("no cache file to restore"), "Cache file not found", "path", cch.filePath)
		return nil
	case len(data) == 0:
		klog.ErrorS(fmt.Errorf("empty cache file"), "Empty cache file, nothing to restore", "path", cch.filePath)
		return nil
	case err != nil:
		return cacheError("failed to load cache from file '%s': %v", cch.filePath, err)
	}

	return cch.Restore(data)
}
