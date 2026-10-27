package main

import (
	"fmt"
	"os"
	"path/filepath"

	templatecreator "gitcode.com/cloud-native/cloud-native/TemplateCreator"
)

type runtimeInfo struct {
	Architecture    string         `json:"architecture"`
	InfraCommit     string         `json:"infra_commit"`
	TemplateVersion string         `json:"template_version"`
	RuntimeRoot     string         `json:"runtime_root"`
	Assets          []runtimeAsset `json:"assets"`
}
type runtimeAsset struct {
	Type    string `json:"type"`
	Version string `json:"version"`
	Path    string `json:"path"`
	SHA256  string `json:"sha256"`
	Source  string `json:"source,omitempty"`
}
type runtimePaths struct {
	Kernel, Firecracker, Envd, Busybox                         string
	KernelSHA256, FirecrackerSHA256, EnvdSHA256, BusyboxSHA256 string
}

// loadRuntimeInfo 只接受 prepare-runtime 的固定目录布局，拒绝资产路径被 JSON 重定向。
func loadRuntimeInfo(path string) (templatecreator.RuntimeConfig, runtimePaths, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return templatecreator.RuntimeConfig{}, runtimePaths{}, fmt.Errorf("read runtime info: %w", err)
	}
	var info runtimeInfo
	if err := decodeStrict(data, &info); err != nil {
		return templatecreator.RuntimeConfig{}, runtimePaths{}, fmt.Errorf("decode runtime info: %w", err)
	}
	if !filepath.IsAbs(info.RuntimeRoot) || info.Architecture == "" || info.InfraCommit == "" || info.TemplateVersion == "" {
		return templatecreator.RuntimeConfig{}, runtimePaths{}, fmt.Errorf("runtime info requires absolute runtime_root, architecture, infra_commit, template_version")
	}
	assets := make(map[string]runtimeAsset, len(info.Assets))
	for _, asset := range info.Assets {
		if _, exists := assets[asset.Type]; exists {
			return templatecreator.RuntimeConfig{}, runtimePaths{}, fmt.Errorf("duplicate %s runtime asset", asset.Type)
		}
		assets[asset.Type] = asset
	}
	pathFor := func(name, subdir, filename string) (runtimeAsset, string, error) {
		asset, ok := assets[name]
		if !ok || asset.Version == "" || asset.SHA256 == "" || !filepath.IsAbs(asset.Path) {
			return runtimeAsset{}, "", fmt.Errorf("missing or incomplete %s asset", name)
		}
		expected := filepath.Join(info.RuntimeRoot, subdir, asset.Version, info.Architecture, filename)
		if filepath.Clean(asset.Path) != expected {
			return runtimeAsset{}, "", fmt.Errorf("%s asset path does not match Builder layout", name)
		}
		return asset, expected, nil
	}
	k, kp, err := pathFor("kernel", "kernels", "vmlinux.bin")
	if err != nil {
		return templatecreator.RuntimeConfig{}, runtimePaths{}, err
	}
	f, fp, err := pathFor("firecracker", "firecrackers", "firecracker")
	if err != nil {
		return templatecreator.RuntimeConfig{}, runtimePaths{}, err
	}
	e, ep, err := pathFor("envd", "envd", "envd")
	if err != nil {
		return templatecreator.RuntimeConfig{}, runtimePaths{}, err
	}
	b, bp, err := pathFor("busybox", "busybox", "busybox")
	if err != nil {
		return templatecreator.RuntimeConfig{}, runtimePaths{}, err
	}
	return templatecreator.RuntimeConfig{RuntimeDir: info.RuntimeRoot, InfraCommit: info.InfraCommit, TemplateVersion: info.TemplateVersion, Architecture: info.Architecture,
			KernelVersion: k.Version, KernelSHA256: k.SHA256, FirecrackerVersion: f.Version, FirecrackerSHA256: f.SHA256,
			EnvdVersion: e.Version, EnvdPath: ep, EnvdSHA256: e.SHA256, BusyboxDir: filepath.Join(info.RuntimeRoot, "busybox"), BusyboxVersion: b.Version, BusyboxSHA256: b.SHA256},
		runtimePaths{Kernel: kp, Firecracker: fp, Envd: ep, Busybox: bp, KernelSHA256: k.SHA256, FirecrackerSHA256: f.SHA256, EnvdSHA256: e.SHA256, BusyboxSHA256: b.SHA256}, nil
}
