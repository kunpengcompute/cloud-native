package typedef

import (
	"kunpeng.huawei.com/kunpeng-cloud-computing/pkg/k8s-mpam-controller/util"

	"k8s.io/klog"
)

const (
	OfflineKey = "kunpeng.com/offline"
)

type PodInfo struct {
	Hierarchy
	Name            string
	UID             string
	Namespace       string
	IDContainersMap map[string]*ContainerInfo
	Annotations     map[string]string
}
type ContainerInfo struct {
	Hierarchy
	Name string
	ID   string
}

// NewPodInfo creates the PodInfo instance
func NewPodInfo(pod *RawPod) *PodInfo {
	return &PodInfo{
		Name:            pod.Name,
		Namespace:       pod.Namespace,
		UID:             pod.ID(),
		Hierarchy:       Hierarchy{Path: pod.CgroupPath()},
		IDContainersMap: pod.ExtractContainerInfos(),
		Annotations:     pod.DeepCopy().Annotations,
	}
}

func NewContainerInfo(con *RawContainer, podCgroupPath string) *ContainerInfo {
	conInfo := &ContainerInfo{}
	conInfo.ID = con.status.ContainerID
	conInfo.Name = con.status.Name
	cgroupPath, err := util.GetContainerCgroupParentDirByID(podCgroupPath, con.status.ContainerID)
	if err != nil {
		klog.Errorf("get container: %s cgroup path failed, err: %s", con.status.Name, err)
	}
	conInfo.Hierarchy = Hierarchy{Path: cgroupPath}
	return conInfo
}

// DeepCopy returns deepcopy object
func (pod *PodInfo) DeepCopy() *PodInfo {
	if pod == nil {
		return nil
	}

	var copy = *pod
	// nil is different from empty value in golang
	if pod.IDContainersMap != nil {
		contMap := make(map[string]*ContainerInfo)
		for id, cont := range pod.IDContainersMap {
			contMap[id] = cont.DeepCopy()
		}
		copy.IDContainersMap = contMap
	}

	if pod.Annotations != nil {
		annoMap := make(map[string]string)
		for k, v := range pod.Annotations {
			annoMap[k] = v
		}
		copy.Annotations = annoMap
	}

	return &copy
}

// DeepCopy returns deepcopy object.
func (cont *ContainerInfo) DeepCopy() *ContainerInfo {
	copyObject := *cont
	return &copyObject
}

// Online is used to determine whether the pod is online
func (pod *PodInfo) Offline() bool {
	var anno string

	if pod.Annotations != nil {
		anno = pod.Annotations[OfflineKey]
	}

	// Annotations have a higher priority than labels
	return anno == "true"
}

// Offline is used to determine whether the pod is offline
func (pod *PodInfo) Online() bool {
	return !pod.Offline()
}
