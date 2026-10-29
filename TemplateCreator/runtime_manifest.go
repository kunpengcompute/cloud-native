package templatecreator

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

const runtimeManifestSchemaVersion = 1

// runtimeAssetType 限制 manifest 可声明的 Builder runtime 资产种类。
type runtimeAssetType string

const (
	assetBusybox     runtimeAssetType = "busybox"
	assetEnvd        runtimeAssetType = "envd"
	assetFirecracker runtimeAssetType = "firecracker"
	assetKernel      runtimeAssetType = "kernel"
)

// requiredRuntimeAssetTypes 定义每种 architecture 必须恰好具备的完整资产集合。
var requiredRuntimeAssetTypes = []runtimeAssetType{
	assetBusybox,
	assetEnvd,
	assetFirecracker,
	assetKernel,
}

// runtimeManifest 是 PrepareRuntime 接受的严格版本锁定 schema。
type runtimeManifest struct {
	SchemaVersion   int            `json:"schema_version"`
	InfraCommit     string         `json:"infra_commit"`
	TemplateVersion string         `json:"template_version"`
	Assets          []runtimeAsset `json:"assets"`
}

// runtimeAsset 描述一个 architecture 下单个文件的版本、来源、checksum 和安装位置。
type runtimeAsset struct {
	Type         runtimeAssetType `json:"type"`
	Version      string           `json:"version"`
	Architecture string           `json:"architecture"`
	URL          string           `json:"url"`
	BundledPath  string           `json:"bundled_path,omitempty"`
	ReusePaths   []string         `json:"reuse_paths"`
	SHA256       string           `json:"sha256"`
	Destination  string           `json:"destination"`
	Mode         string           `json:"mode"`
}

// parseRuntimeManifest 严格解析并完整验证一个 runtime manifest。
func parseRuntimeManifest(reader io.Reader) (runtimeManifest, error) {
	// 拒绝未知字段和尾随内容，避免 security/version 字段拼错后静默退化为零值。
	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()

	var manifest runtimeManifest
	if err := decoder.Decode(&manifest); err != nil {
		return runtimeManifest{}, fmt.Errorf("decode runtime manifest: %w", err)
	}
	if err := requireJSONEOF(decoder); err != nil {
		return runtimeManifest{}, err
	}
	if err := validateRuntimeManifest(manifest); err != nil {
		return runtimeManifest{}, err
	}

	return manifest, nil
}

// requireJSONEOF 保证输入只包含一个完整 JSON value。
func requireJSONEOF(decoder *json.Decoder) error {
	var trailing any
	if err := decoder.Decode(&trailing); err == io.EOF {
		return nil
	} else if err != nil {
		return fmt.Errorf("decode trailing runtime manifest data: %w", err)
	}

	return fmt.Errorf("runtime manifest contains multiple JSON values")
}

// validateRuntimeManifest 验证 schema、infra commit、Template version 以及全局唯一性约束。
func validateRuntimeManifest(manifest runtimeManifest) error {
	if manifest.SchemaVersion != runtimeManifestSchemaVersion {
		return fmt.Errorf("runtime manifest schema_version = %d, want %d", manifest.SchemaVersion, runtimeManifestSchemaVersion)
	}
	if !isLowerHex(manifest.InfraCommit, 40) {
		return fmt.Errorf("runtime manifest infra_commit must be a 40-character lowercase hex commit")
	}
	if strings.TrimSpace(manifest.TemplateVersion) == "" || strings.EqualFold(manifest.TemplateVersion, "latest") {
		return fmt.Errorf("runtime manifest template_version must be pinned and must not be latest")
	}
	if len(manifest.Assets) == 0 {
		return fmt.Errorf("runtime manifest assets must not be empty")
	}

	// 每个 architecture 的每类资产最多一个，且任意两项不能竞争写入同一 destination。
	identities := make(map[string]struct{}, len(manifest.Assets))
	destinations := make(map[string]struct{}, len(manifest.Assets))
	for i, asset := range manifest.Assets {
		if err := validateRuntimeAsset(asset); err != nil {
			return fmt.Errorf("runtime manifest assets[%d]: %w", i, err)
		}

		identity := string(asset.Type) + "\x00" + asset.Architecture
		if _, exists := identities[identity]; exists {
			return fmt.Errorf("runtime manifest assets[%d]: duplicate %s asset for architecture %s", i, asset.Type, asset.Architecture)
		}
		identities[identity] = struct{}{}

		if _, exists := destinations[asset.Destination]; exists {
			return fmt.Errorf("runtime manifest assets[%d]: duplicate destination %q", i, asset.Destination)
		}
		destinations[asset.Destination] = struct{}{}
	}

	return nil
}

// validateRuntimeAsset 验证单项版本固定、来源、安全路径、checksum、权限和 Builder 布局。
func validateRuntimeAsset(asset runtimeAsset) error {
	if !isKnownRuntimeAssetType(asset.Type) {
		return fmt.Errorf("unknown asset type %q", asset.Type)
	}
	if strings.TrimSpace(asset.Version) == "" || strings.EqualFold(asset.Version, "latest") {
		return fmt.Errorf("version must be pinned and must not be latest")
	}
	if strings.TrimSpace(asset.Architecture) == "" {
		return fmt.Errorf("architecture is required")
	}
	if !isLowerHex(asset.SHA256, 64) {
		return fmt.Errorf("sha256 must be 64 lowercase hex characters")
	}
	if err := validateAssetDestination(asset.Destination); err != nil {
		return err
	}
	if err := validateAssetDestinationLayout(asset); err != nil {
		return err
	}
	if _, err := parseAssetMode(asset.Mode); err != nil {
		return err
	}
	if asset.URL == "" && asset.BundledPath == "" && len(asset.ReusePaths) == 0 {
		return fmt.Errorf("url, bundled_path, or reuse_paths is required")
	}
	if asset.URL != "" {
		if err := validateRuntimeAssetURL(asset.URL); err != nil {
			return err
		}
	}
	if asset.BundledPath != "" {
		if err := validateBundledPath(asset.BundledPath); err != nil {
			return err
		}
	}
	for _, reusePath := range asset.ReusePaths {
		if !filepath.IsAbs(reusePath) || filepath.Clean(reusePath) != reusePath {
			return fmt.Errorf("reuse path %q must be a clean absolute path", reusePath)
		}
	}

	return nil
}

// validateBundledPath 只允许相对 manifest 的干净 slash path，防止 bundle 路径逃逸。
func validateBundledPath(bundledPath string) error {
	if strings.Contains(bundledPath, `\`) || path.IsAbs(bundledPath) || path.Clean(bundledPath) != bundledPath || bundledPath == "." || strings.HasPrefix(bundledPath, "../") {
		return fmt.Errorf("bundled_path %q must be a clean relative slash path", bundledPath)
	}

	return nil
}

// validateAssetDestinationLayout 保证安装位置符合抽取 Builder 的固定查找 contract。
func validateAssetDestinationLayout(asset runtimeAsset) error {
	// 即使下载内容正确，只要放在 Builder 永远不会查找的位置也不可用，因此布局必须严格固定。
	var expected string
	switch asset.Type {
	case assetKernel:
		expected = path.Join("kernels", asset.Version, asset.Architecture, "vmlinux.bin")
	case assetFirecracker:
		expected = path.Join("firecrackers", asset.Version, asset.Architecture, "firecracker")
	case assetEnvd:
		expected = path.Join("envd", asset.Version, asset.Architecture, "envd")
	case assetBusybox:
		expected = path.Join("busybox", asset.Version, asset.Architecture, "busybox")
	}
	if asset.Destination != expected {
		return fmt.Errorf("destination %q is incompatible with Builder layout; want %q", asset.Destination, expected)
	}

	return nil
}

// isKnownRuntimeAssetType 拒绝当前 Builder contract 之外的资产类型。
func isKnownRuntimeAssetType(assetType runtimeAssetType) bool {
	for _, required := range requiredRuntimeAssetTypes {
		if assetType == required {
			return true
		}
	}

	return false
}

// isLowerHex 验证固定长度的小写十六进制摘要。
func isLowerHex(value string, length int) bool {
	if len(value) != length || value != strings.ToLower(value) {
		return false
	}
	_, err := hex.DecodeString(value)

	return err == nil
}

// validateAssetDestination 拒绝绝对路径、反斜杠和 `..` 逃逸。
func validateAssetDestination(destination string) error {
	if destination == "" || strings.Contains(destination, `\`) || path.IsAbs(destination) || path.Clean(destination) != destination || destination == "." || strings.HasPrefix(destination, "../") {
		return fmt.Errorf("destination %q must be a clean relative slash path", destination)
	}

	return nil
}

// parseAssetMode 只接受形如 0755 的权限位，不允许特殊 mode bits。
func parseAssetMode(value string) (uint32, error) {
	if len(value) != 4 || value[0] != '0' {
		return 0, fmt.Errorf("mode %q must use four-digit octal form such as 0755", value)
	}
	mode, err := strconv.ParseUint(value, 8, 32)
	if err != nil || mode > 0o777 {
		return 0, fmt.Errorf("mode %q must contain permission bits only", value)
	}

	return uint32(mode), nil
}

// validateRuntimeAssetURL 拒绝 embedded credential、fragment、query 和 latest 路径。
func validateRuntimeAssetURL(rawURL string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("parse asset URL: %w", err)
	}
	if parsed.Scheme != "https" && parsed.Scheme != "http" {
		return fmt.Errorf("asset URL %q must use http or https", rawURL)
	}
	if parsed.RawQuery != "" {
		return fmt.Errorf("asset URL must not contain a query")
	}
	if parsed.Host == "" || parsed.User != nil || parsed.Fragment != "" {
		return fmt.Errorf("asset URL %q must have a host and no credentials or fragment", rawURL)
	}
	for _, segment := range strings.Split(parsed.EscapedPath(), "/") {
		// 版本化 manifest 必须可复现；即使 URL 当前可解析，包含 latest 也会让内容随时间漂移。
		if strings.EqualFold(segment, "latest") {
			return fmt.Errorf("asset URL %q must not contain a latest path segment", rawURL)
		}
	}

	return nil
}

// selectArchitecture 选择一套完整 architecture 资产，并以稳定顺序返回。
func (manifest runtimeManifest) selectArchitecture(architecture string) ([]runtimeAsset, error) {
	if strings.TrimSpace(architecture) == "" {
		return nil, fmt.Errorf("architecture is required")
	}

	byType := make(map[runtimeAssetType]runtimeAsset, len(requiredRuntimeAssetTypes))
	for _, asset := range manifest.Assets {
		if asset.Architecture == architecture {
			byType[asset.Type] = asset
		}
	}
	for _, required := range requiredRuntimeAssetTypes {
		if _, exists := byType[required]; !exists {
			return nil, fmt.Errorf("runtime manifest has no %s asset for architecture %s", required, architecture)
		}
	}

	assets := make([]runtimeAsset, 0, len(byType))
	for _, asset := range byType {
		assets = append(assets, asset)
	}
	// map 遍历顺序随机；稳定排序保证安装结果和 CLI 输出可复现。
	sort.Slice(assets, func(i, j int) bool {
		return assets[i].Type < assets[j].Type
	})

	return assets, nil
}
