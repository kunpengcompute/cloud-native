package informer

import (
	"sync"

	"kunpeng.huawei.com/kunpeng-cloud-computing/pkg/k8s-mpam-controller/typedef"

	"k8s.io/klog"
)

// PodCache is used to store PodInfo
type PodCache struct {
	sync.RWMutex
	Pods map[string]*typedef.PodInfo
}

// NewPodCache returns a PodCache object (pointer)
func NewPodCache() *PodCache {
	return &PodCache{
		Pods: make(map[string]*typedef.PodInfo, 0),
	}
}

// getPod returns the deepcopy object of pod
func (cache *PodCache) getPod(podID string) *typedef.PodInfo {
	cache.RLock()
	defer cache.RUnlock()
	return cache.Pods[podID].DeepCopy()
}

// podExist returns true if there is a pod whose key is podID in the pods
func (cache *PodCache) podExist(podID string) bool {
	cache.RLock()
	_, ok := cache.Pods[podID]
	cache.RUnlock()
	return ok
}

// addPod adds pod information
func (cache *PodCache) addPod(pod *typedef.PodInfo) {
	if pod == nil || pod.UID == "" {
		return
	}
	if ok := cache.podExist(pod.UID); ok {
		klog.Infof("pod already exists: %v", pod.Name)
		return
	}
	cache.Lock()
	cache.Pods[pod.UID] = pod
	cache.Unlock()
	klog.Infof("add pod successfully: %v", pod.Name)
}

// delPod deletes pod information
func (cache *PodCache) delPod(podID string) {
	if ok := cache.podExist(podID); !ok {
		klog.Infof("pod does not exist: %v", podID)
		return
	}
	cache.Lock()
	delete(cache.Pods, podID)
	cache.Unlock()
	klog.Infof("delete pod successfully: %v", podID)
}

// updatePod updates pod information
func (cache *PodCache) updatePod(pod *typedef.PodInfo) {
	if pod == nil || pod.UID == "" {
		return
	}
	cache.Lock()
	cache.Pods[pod.UID] = pod
	cache.Unlock()
	klog.Infof("update pod successfully: %v", pod.Name)
}

// substitute replaces all the data in the cache
func (cache *PodCache) substitute(pods []*typedef.PodInfo) {
	cache.Lock()
	defer cache.Unlock()
	cache.Pods = make(map[string]*typedef.PodInfo, 0)
	if len(pods) == 0 {
		return
	}
	for _, pod := range pods {
		if pod == nil || pod.UID == "" {
			continue
		}
		cache.Pods[pod.UID] = pod
		klog.Infof("substituting pod successfully: %v", pod.Name)
	}
}

// listPod returns the deepcopy object of all pod
func (cache *PodCache) listPod() map[string]*typedef.PodInfo {
	res := make(map[string]*typedef.PodInfo, len(cache.Pods))
	cache.RLock()
	for id, pi := range cache.Pods {
		res[id] = pi.DeepCopy()
	}
	cache.RUnlock()
	return res
}
