/*
 * Copyright (c) 2026 Huawei Technology corp.
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

package plugin

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/containerd/nri/pkg/api"
	"github.com/containerd/nri/pkg/stub"
	"k8s.io/klog/v2"

	"kunpeng.huawei.com/kunpeng-cloud-computing/pkg/kata-cpuset-nri/topology"
)

const (
	PluginName = "kata-cpuset-nri"
	PluginIdx  = "01"
)

// Config keeps only the settings needed by the initial pod-level binding flow.
type Config struct {
	SocketPath     string
	ScanInterval   time.Duration
	CgroupRoot     string
	Namespaces     []string
	RuntimeClasses []string
	DryRun         bool
}

// DefaultConfig returns conservative defaults for kata workloads.
func DefaultConfig() Config {
	return Config{
		SocketPath:     "/var/run/nri/nri.sock",
		ScanInterval:   10 * time.Second,
		CgroupRoot:     "",
		Namespaces:     []string{"default"},
		RuntimeClasses: []string{"kata"},
	}
}

type podInfo struct {
	id           string
	name         string
	namespace    string
	runtimeClass string
	cgroupPath   string
}

// Agent implements the NRI plugin entry and the scan-based cpuset reconciliation.
type Agent struct {
	mask           api.EventMask
	stub           stub.Stub
	cfg            Config
	siblingPairs   []topology.SiblingPair
	namespaces     map[string]struct{}
	runtimeClasses map[string]struct{}

	mu   sync.RWMutex
	pods map[string]podInfo
}

// New creates an agent.
func New(cfg Config, siblingPairs []topology.SiblingPair) (*Agent, error) {
	if len(siblingPairs) == 0 {
		return nil, fmt.Errorf("no sibling pair available")
	}
	a := &Agent{
		mask:           api.MustParseEventMask("RunPodSandbox,RemovePodSandbox"),
		cfg:            cfg,
		siblingPairs:   siblingPairs,
		namespaces:     toSet(cfg.Namespaces),
		runtimeClasses: toSet(cfg.RuntimeClasses),
		pods:           map[string]podInfo{},
	}

	opts := []stub.Option{
		stub.WithPluginName(PluginName),
		stub.WithPluginIdx(PluginIdx),
	}
	if cfg.SocketPath != "" {
		opts = append(opts, stub.WithSocketPath(cfg.SocketPath))
	}
	s, err := stub.New(a, opts...)
	if err != nil {
		return nil, err
	}
	a.stub = s
	return a, nil
}

// Run starts NRI stub and reconciliation loop.
func (a *Agent) Run(ctx context.Context) error {
	if err := a.stub.Start(ctx); err != nil {
		return err
	}
	ticker := time.NewTicker(a.cfg.ScanInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			a.stub.Stop()
			return nil
		case <-ticker.C:
			if err := a.reconcileOnce(); err != nil {
				klog.ErrorS(err, "Periodic reconciliation failed")
			}
		}
	}
}

// Configure handles NRI configure event.
func (a *Agent) Configure(_ context.Context, _ string, _ string, _ string) (stub.EventMask, error) {
	return a.mask, nil
}

// Synchronize handles initial snapshot.
func (a *Agent) Synchronize(_ context.Context, pods []*api.PodSandbox, _ []*api.Container) ([]*api.ContainerUpdate, error) {
	a.replacePods(pods)
	if err := a.reconcileOnce(); err != nil {
		klog.ErrorS(err, "Reconcile on synchronize failed")
	}
	return nil, nil
}

// RunPodSandbox handles pod creation event.
func (a *Agent) RunPodSandbox(_ context.Context, pod *api.PodSandbox) error {
	a.upsertPod(pod)
	return a.reconcileOnce()
}

// RemovePodSandbox handles pod removal event.
func (a *Agent) RemovePodSandbox(_ context.Context, pod *api.PodSandbox) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	delete(a.pods, pod.Id)
	return nil
}

func (a *Agent) reconcileOnce() error {
	pods := a.listPods()
	occupied := map[string]struct{}{}
	for _, pod := range pods {
		if !a.match(pod) {
			continue
		}
		if pod.cgroupPath == "" {
			klog.InfoS("Skip pod without cgroup path", "pod", pod.name, "namespace", pod.namespace)
			continue
		}
		cgroupPath, err := a.resolveCgroupPath(pod.cgroupPath)
		if err != nil {
			klog.ErrorS(err, "Resolve pod cgroup path failed", "pod", pod.name,
				"namespace", pod.namespace, "cgroupPath", pod.cgroupPath)
			continue
		}
		current, err := readCpuset(cgroupPath)
		if err != nil {
			klog.ErrorS(err, "Read pod cpuset failed", "pod", pod.name, "namespace", pod.namespace)
			continue
		}
		target := a.targetCpuset(current, occupied)
		if target == "" {
			klog.InfoS("No free sibling pair for pod", "pod", pod.name, "namespace", pod.namespace)
			continue
		}
		occupied[normalizeCpuset(target)] = struct{}{}
		if normalizeCpuset(current) == normalizeCpuset(target) {
			continue
		}
		if a.cfg.DryRun {
			klog.InfoS("Dry-run pod cpuset update", "pod", pod.name, "namespace", pod.namespace, "current", current, "target", target)
			continue
		}
		if err := writeCpuset(cgroupPath, target); err != nil {
			klog.ErrorS(err, "Write pod cpuset failed", "pod", pod.name, "namespace", pod.namespace, "target", target)
			continue
		}
		klog.InfoS("Pod cpuset updated", "pod", pod.name, "namespace", pod.namespace, "target", target)
	}
	return nil
}

func (a *Agent) listPods() []podInfo {
	a.mu.RLock()
	defer a.mu.RUnlock()
	out := make([]podInfo, 0, len(a.pods))
	for _, pod := range a.pods {
		out = append(out, pod)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].namespace != out[j].namespace {
			return out[i].namespace < out[j].namespace
		}
		if out[i].name != out[j].name {
			return out[i].name < out[j].name
		}
		return out[i].id < out[j].id
	})
	return out
}

func (a *Agent) match(pod podInfo) bool {
	if _, ok := a.namespaces[pod.namespace]; !ok {
		return false
	}
	if _, ok := a.runtimeClasses[pod.runtimeClass]; !ok {
		return false
	}
	return true
}

func (a *Agent) targetCpuset(current string, occupied map[string]struct{}) string {
	currentSet := normalizeCpuset(current)
	for _, pair := range a.siblingPairs {
		pairSet := normalizeCpuset(pair.String())
		if currentSet == pairSet {
			if _, used := occupied[pairSet]; used {
				break
			}
			return pair.String()
		}
	}
	for _, pair := range a.siblingPairs {
		pairSet := normalizeCpuset(pair.String())
		if _, used := occupied[pairSet]; !used {
			return pair.String()
		}
	}
	return ""
}

func (a *Agent) resolveCgroupPath(cgroupPath string) (string, error) {
	candidates := []string{cgroupPath}
	if a.cfg.CgroupRoot != "" {
		trimmed := strings.TrimPrefix(cgroupPath, "/")
		candidates = append(candidates, filepath.Join(a.cfg.CgroupRoot, trimmed))
		candidates = append(candidates, systemdScopeCandidates(a.cfg.CgroupRoot, trimmed)...)
	}
	for _, candidate := range candidates {
		if hasCpusetFile(candidate) {
			return candidate, nil
		}
	}
	if a.cfg.CgroupRoot != "" {
		if found, ok := findCgroupByBase(a.cfg.CgroupRoot, filepath.Base(cgroupPath)); ok {
			return found, nil
		}
	}
	return "", fmt.Errorf("cpuset.cpus not found for cgroup %q", cgroupPath)
}

func (a *Agent) replacePods(pods []*api.PodSandbox) {
	a.mu.Lock()
	defer a.mu.Unlock()
	next := make(map[string]podInfo, len(pods))
	for _, p := range pods {
		if p == nil {
			continue
		}
		next[p.Id] = convertPod(p)
	}
	a.pods = next
}

func (a *Agent) upsertPod(pod *api.PodSandbox) {
	if pod == nil {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.pods[pod.Id] = convertPod(pod)
}

func convertPod(p *api.PodSandbox) podInfo {
	out := podInfo{
		id:        p.Id,
		name:      p.Name,
		namespace: p.Namespace,
	}
	if p.Linux != nil {
		out.cgroupPath = p.Linux.CgroupParent
	}
	if p.RuntimeHandler != "" {
		out.runtimeClass = p.RuntimeHandler
	}
	return out
}

func toSet(items []string) map[string]struct{} {
	out := make(map[string]struct{}, len(items))
	for _, item := range items {
		if item != "" {
			out[item] = struct{}{}
		}
	}
	return out
}

func systemdScopeCandidates(root, cgroupPath string) []string {
	if !strings.Contains(cgroupPath, ":") {
		return nil
	}
	parts := strings.Split(cgroupPath, ":")
	if len(parts) < 3 {
		return nil
	}
	podSlice := parts[0]
	scopeName := fmt.Sprintf("%s-%s.scope", parts[1], parts[2])
	qosSlice := systemdQoSSlice(podSlice)
	if qosSlice == "" {
		return nil
	}
	return []string{filepath.Join(root, "kubepods.slice", qosSlice, podSlice, scopeName)}
}

func systemdQoSSlice(podSlice string) string {
	switch {
	case strings.HasPrefix(podSlice, "kubepods-burstable-"):
		return "kubepods-burstable.slice"
	case strings.HasPrefix(podSlice, "kubepods-besteffort-"):
		return "kubepods-besteffort.slice"
	case strings.HasPrefix(podSlice, "kubepods-pod"):
		return ""
	default:
		return ""
	}
}

func hasCpusetFile(cgroupPath string) bool {
	info, err := os.Stat(filepath.Join(cgroupPath, "cpuset.cpus"))
	return err == nil && !info.IsDir()
}

func findCgroupByBase(root, base string) (string, bool) {
	if root == "" || base == "" || base == "." || base == "/" {
		return "", false
	}
	var found string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || !d.IsDir() {
			return nil
		}
		if d.Name() != base {
			return nil
		}
		if hasCpusetFile(path) {
			found = path
			return filepath.SkipAll
		}
		return nil
	})
	return found, err == nil && found != ""
}

// DiscoverCpusetRoot discovers the cpuset cgroup mount from the current mount namespace.
func DiscoverCpusetRoot() (string, error) {
	data, err := os.ReadFile("/proc/self/mountinfo")
	if err != nil {
		return "", fmt.Errorf("read mountinfo: %w", err)
	}
	for _, line := range strings.Split(string(data), "\n") {
		if root, ok := parseCpusetMount(line); ok {
			return root, nil
		}
	}
	return "", fmt.Errorf("cpuset cgroup mount not found")
}

func parseCpusetMount(line string) (string, bool) {
	if strings.TrimSpace(line) == "" {
		return "", false
	}
	parts := strings.Split(line, " - ")
	if len(parts) != 2 {
		return "", false
	}
	left := strings.Fields(parts[0])
	right := strings.Fields(parts[1])
	if len(left) < 5 || len(right) < 3 {
		return "", false
	}
	mountPoint := unescapeMountPath(left[4])
	fsType := right[0]
	superOptions := right[2]
	switch fsType {
	case "cgroup":
		return mountPoint, hasMountOption(superOptions, "cpuset")
	case "cgroup2":
		if hasCgroup2Controller(mountPoint, "cpuset") {
			return mountPoint, true
		}
	}
	return "", false
}

func hasMountOption(options, target string) bool {
	for _, option := range strings.Split(options, ",") {
		if option == target {
			return true
		}
	}
	return false
}

func hasCgroup2Controller(root, controller string) bool {
	data, err := os.ReadFile(filepath.Join(root, "cgroup.controllers"))
	if err != nil {
		return false
	}
	for _, item := range strings.Fields(string(data)) {
		if item == controller {
			return true
		}
	}
	return false
}

func unescapeMountPath(path string) string {
	replacer := strings.NewReplacer(
		`\\`, `\`,
		`\040`, " ",
		`\011`, "\t",
		`\012`, "\n",
	)
	return replacer.Replace(path)
}

func readCpuset(cgroupPath string) (string, error) {
	data, err := os.ReadFile(filepath.Join(cgroupPath, "cpuset.cpus"))
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

func writeCpuset(cgroupPath, cpus string) error {
	return os.WriteFile(filepath.Join(cgroupPath, "cpuset.cpus"), []byte(cpus), 0o644)
}

func normalizeCpuset(raw string) string {
	cpus, err := parseCpuset(raw)
	if err != nil {
		return strings.TrimSpace(raw)
	}
	sort.Ints(cpus)
	out := make([]string, 0, len(cpus))
	for _, cpu := range cpus {
		out = append(out, strconv.Itoa(cpu))
	}
	return strings.Join(out, ",")
}

func parseCpuset(raw string) ([]int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	var cpus []int
	for _, item := range strings.Split(raw, ",") {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if strings.Contains(item, "-") {
			bounds := strings.Split(item, "-")
			if len(bounds) != 2 {
				return nil, fmt.Errorf("invalid cpuset range %q", item)
			}
			start, err := strconv.Atoi(strings.TrimSpace(bounds[0]))
			if err != nil {
				return nil, err
			}
			end, err := strconv.Atoi(strings.TrimSpace(bounds[1]))
			if err != nil {
				return nil, err
			}
			for cpu := start; cpu <= end; cpu++ {
				cpus = append(cpus, cpu)
			}
			continue
		}
		cpu, err := strconv.Atoi(item)
		if err != nil {
			return nil, err
		}
		cpus = append(cpus, cpu)
	}
	return cpus, nil
}
