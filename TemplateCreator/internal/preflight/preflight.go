// Package preflight 执行本地 E2B Template 构建前的只读检查。
// 它一次返回全部检查项，便于初学者集中修复调用机问题。
package preflight

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/distribution/reference"
)

// Config 描述 Run 要检查的 host 路径、runtime 文件和构建设置。
type Config struct {
	// GOOS/GOARCH 是调用机事实；Architecture 是准备好的目标 runtime 架构。
	GOOS, GOARCH string
	Architecture string
	// ImageRef 是待拉取的 OCI 镜像引用。
	ImageRef string
	// ImageTarPath 是待读取的 docker save 本地归档路径。
	ImageTarPath string
	// ScratchRoot/StoreRoot 是必须已经存在且可写的本地目录。
	ScratchRoot string
	StoreRoot   string
	// CacheRoot 是可选的持久 base-layer cache 目录；为空表示 cache 关闭。
	CacheRoot string

	// KVMPath/NBDMaxPath/NBDSystemRoot/MemInfoPath 是可替换的 host 检查入口，便于测试。
	KVMPath       string
	NBDMaxPath    string
	NBDSystemRoot string
	MemInfoPath   string
	// EffectiveUID 允许测试覆盖进程身份；nil 时读取当前进程 os.Geteuid()。
	EffectiveUID *int

	// KernelPath/FirecrackerPath/EnvdPath/BusyboxPath 是 runtime-info 指向的实际文件。
	KernelPath      string
	FirecrackerPath string
	EnvdPath        string
	BusyboxPath     string
	// 对应 SHA256 必须与文件内容一致，防止 runtime-info 生成后文件被替换。
	KernelSHA256      string
	FirecrackerSHA256 string
	EnvdSHA256        string
	BusyboxSHA256     string

	// MemoryMB 参与 HugePages 容量计算；HugePages 为 false 时只报告禁用状态。
	MemoryMB  int64
	HugePages bool
}

// Check 是一个既适合 JSON 输出、又适合人工排障的 preflight 结果。
type Check struct {
	// Name 是供程序稳定识别的检查名。
	Name string `json:"name"`
	// OK 表示该检查是否通过。
	OK bool `json:"ok"`
	// Detail 提供实际路径、检测值或失败原因。
	Detail string `json:"detail"`
}

// Report 按稳定执行顺序保存全部 preflight 结果。
type Report struct {
	// Checks 不会在首个失败处截断，因此一次运行可暴露多个 host 问题。
	Checks []Check `json:"checks"`
}

// Ready 仅在每个 preflight 检查都通过时返回 true。
func (report Report) Ready() bool {
	for _, check := range report.Checks {
		if !check.OK {
			return false
		}
	}
	return true
}

// Run 只读取 host 状态，不创建文件、不改权限、不加载 kernel module、
// 不预留 HugePages，也不启动 VM；环境修复必须由调用者显式完成。
func Run(config Config) Report {
	config = normalize(config)
	report := Report{Checks: make([]Check, 0, 14)}
	report.Checks = append(report.Checks,
		checkHostOS(config.GOOS),
		checkPrivileges(*config.EffectiveUID),
		checkArchitecture(config.GOARCH, config.Architecture),
		checkImageInput(config.ImageRef, config.ImageTarPath),
		checkDirectory("scratch_directory", config.ScratchRoot),
		checkDirectory("store_directory", config.StoreRoot),
		checkCacheDirectory(config.CacheRoot, config.ScratchRoot, config.StoreRoot),
		checkRuntimeFile("runtime_kernel", config.KernelPath, config.KernelSHA256, false),
		checkRuntimeFile("runtime_firecracker", config.FirecrackerPath, config.FirecrackerSHA256, true),
		checkRuntimeFile("runtime_envd", config.EnvdPath, config.EnvdSHA256, true),
		checkRuntimeFile("runtime_busybox", config.BusyboxPath, config.BusyboxSHA256, true),
		checkKVM(config.KVMPath),
		checkNBD(config.NBDMaxPath, config.NBDSystemRoot),
		checkHugePages(config.HugePages, config.MemoryMB, config.MemInfoPath),
	)
	return report
}

// checkCacheDirectory 检查可选 cache 目录，并在词法路径层面拒绝与 Scratch/Store 重叠。
// 真实路径的 symlink 风险由 checkDirectory 统一拒绝；该检查只读，不创建目录。
func checkCacheDirectory(cacheRoot, scratchRoot, storeRoot string) Check {
	if strings.TrimSpace(cacheRoot) == "" {
		return Check{Name: "cache_directory", OK: true, Detail: "persistent base cache is disabled"}
	}
	check := checkDirectory("cache_directory", cacheRoot)
	if !check.OK {
		return check
	}
	if filesystemPathsOverlap(cacheRoot, scratchRoot) {
		return Check{Name: "cache_directory", Detail: "CacheRoot and ScratchRoot must not overlap"}
	}
	if filesystemPathsOverlap(cacheRoot, storeRoot) {
		return Check{Name: "cache_directory", Detail: "CacheRoot and StoreRoot must not overlap"}
	}
	return check
}

func filesystemPathsOverlap(first, second string) bool {
	contains := func(parent, child string) bool {
		relative, err := filepath.Rel(filepath.Clean(parent), filepath.Clean(child))
		if err != nil {
			return false
		}
		return relative == "." || (relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)))
	}
	return contains(first, second) || contains(second, first)
}

// normalize 只补测试入口的默认 host 路径，不修改实际系统状态。
func normalize(config Config) Config {
	if config.GOOS == "" {
		config.GOOS = runtime.GOOS
	}
	if config.GOARCH == "" {
		config.GOARCH = runtime.GOARCH
	}
	if config.KVMPath == "" {
		config.KVMPath = "/dev/kvm"
	}
	if config.NBDMaxPath == "" {
		config.NBDMaxPath = "/sys/module/nbd/parameters/nbds_max"
	}
	if config.NBDSystemRoot == "" {
		config.NBDSystemRoot = "/sys/block"
	}
	if config.MemInfoPath == "" {
		config.MemInfoPath = "/proc/meminfo"
	}
	if config.EffectiveUID == nil {
		uid := os.Geteuid()
		config.EffectiveUID = &uid
	}
	return config
}

// checkHostOS 确认真实构建运行在 Linux。
func checkHostOS(goos string) Check {
	if goos != "linux" {
		return Check{Name: "host_os", Detail: fmt.Sprintf("Linux is required; host reports %q", goos)}
	}
	return Check{Name: "host_os", OK: true, Detail: "Linux host detected"}
}

// checkPrivileges 拒绝普通用户进入必须执行 mount、netns 和 iptables 的真实 Builder。
// Demo 当前不尝试推断一组容易漏项的 file capabilities，而采用与 infra create-build
// local mode 相同的明确边界：由 effective UID 0 运行。
func checkPrivileges(effectiveUID int) Check {
	if effectiveUID != 0 {
		return Check{Name: "privileges", Detail: fmt.Sprintf("TemplateCreator build requires root (effective UID 0); current effective UID is %d", effectiveUID)}
	}
	return Check{Name: "privileges", OK: true, Detail: "process runs with effective UID 0 (root)"}
}

// checkArchitecture 拒绝 cross-architecture 构建和当前未支持的架构。
func checkArchitecture(host, target string) Check {
	if target != "amd64" && target != "arm64" {
		return Check{Name: "architecture", Detail: fmt.Sprintf("unsupported runtime architecture %q; use amd64 or arm64", target)}
	}
	if host != target {
		return Check{Name: "architecture", Detail: fmt.Sprintf("host architecture %q does not match runtime architecture %q", host, target)}
	}
	return Check{Name: "architecture", OK: true, Detail: fmt.Sprintf("host and runtime use %s", target)}
}

// checkImageReference 只验证 OCI 引用语法，不进行网络请求或拉取镜像。
func checkImageReference(imageRef string) Check {
	if strings.TrimSpace(imageRef) == "" {
		return Check{Name: "image_reference", Detail: "image reference is required"}
	}
	if _, err := reference.ParseNormalizedNamed(imageRef); err != nil {
		return Check{Name: "image_reference", Detail: fmt.Sprintf("invalid OCI image reference: %v", err)}
	}
	return Check{Name: "image_reference", OK: true, Detail: imageRef}
}

// checkImageInput 在远程引用和本地归档之间保持与 Creator API 相同的互斥约束。
func checkImageInput(imageRef, imageTarPath string) Check {
	if strings.TrimSpace(imageRef) != "" {
		if strings.TrimSpace(imageTarPath) != "" {
			return Check{Name: "image_reference", Detail: "ImageRef and ImageTarPath are mutually exclusive"}
		}
		return checkImageReference(imageRef)
	}
	if strings.TrimSpace(imageTarPath) == "" {
		return Check{Name: "image_reference", Detail: "image reference or image tar path is required"}
	}
	info, err := os.Stat(imageTarPath)
	if err != nil {
		return Check{Name: "image_reference", Detail: fmt.Sprintf("image tar %q is unavailable: %v", imageTarPath, err)}
	}
	if !info.Mode().IsRegular() {
		return Check{Name: "image_reference", Detail: fmt.Sprintf("image tar %q must be a regular file", imageTarPath)}
	}
	return Check{Name: "image_reference", OK: true, Detail: imageTarPath}
}

// checkDirectory 确认目录存在、可搜索、可写且根本身不是 symlink。
func checkDirectory(name, path string) Check {
	// Lstat 有意拒绝可被重新指向的 symlink root；对 StoreRoot 而言，
	// 这也与 Local Store 的发布安全策略一致。
	info, err := os.Lstat(path)
	if err != nil {
		return Check{Name: name, Detail: fmt.Sprintf("directory %q is unavailable: %v", path, err)}
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return Check{Name: name, Detail: fmt.Sprintf("directory %q must not be a symbolic link", path)}
	}
	if !info.IsDir() {
		return Check{Name: name, Detail: fmt.Sprintf("path %q is not a directory", path)}
	}
	if info.Mode().Perm()&0o222 == 0 || info.Mode().Perm()&0o111 == 0 {
		return Check{Name: name, Detail: fmt.Sprintf("directory %q must be writable and searchable", path)}
	}
	return Check{Name: name, OK: true, Detail: path}
}

// checkRuntimeFile 验证文件类型、权限和实际 SHA-256，不能只相信 runtime-info。
func checkRuntimeFile(name, path, expectedSHA256 string, executable bool) Check {
	info, err := os.Stat(path)
	if err != nil {
		return Check{Name: name, Detail: fmt.Sprintf("runtime file %q is missing or unavailable: %v", path, err)}
	}
	if !info.Mode().IsRegular() || info.Size() == 0 {
		return Check{Name: name, Detail: fmt.Sprintf("runtime file %q must be a non-empty regular file", path)}
	}
	if executable && info.Mode().Perm()&0o111 == 0 {
		return Check{Name: name, Detail: fmt.Sprintf("runtime file %q must be executable", path)}
	}
	if !validSHA256(expectedSHA256) {
		return Check{Name: name, Detail: fmt.Sprintf("runtime file %q has no valid expected SHA-256", path)}
	}
	// 在 smoke build 前立即重新 hash；runtime-info 只描述 PrepareRuntime 当时安装的文件，
	// 该文件之后仍可能被外部替换。
	file, err := os.Open(path)
	if err != nil {
		return Check{Name: name, Detail: fmt.Sprintf("open runtime file %q: %v", path, err)}
	}
	hash := sha256.New()
	_, copyErr := io.Copy(hash, file)
	closeErr := file.Close()
	if copyErr != nil {
		return Check{Name: name, Detail: fmt.Sprintf("hash runtime file %q: %v", path, copyErr)}
	}
	if closeErr != nil {
		return Check{Name: name, Detail: fmt.Sprintf("close runtime file %q: %v", path, closeErr)}
	}
	actual := hex.EncodeToString(hash.Sum(nil))
	if actual != expectedSHA256 {
		return Check{Name: name, Detail: fmt.Sprintf("runtime file %q checksum mismatch: got %s, want %s", path, actual, expectedSHA256)}
	}
	return Check{Name: name, OK: true, Detail: path}
}

// validSHA256 要求小写、定长的十六进制 SHA-256，避免模糊输入。
func validSHA256(value string) bool {
	if len(value) != sha256.Size*2 || value != strings.ToLower(value) {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

// checkKVM 只验证 device 可读写，不在 preflight 中执行 privileged KVM ioctl。
func checkKVM(path string) Check {
	// 以 read/write 打开足以证明后续 Firecracker 进程具备基础访问权限。
	file, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		return Check{Name: "kvm", Detail: fmt.Sprintf("KVM device %q is unavailable for read/write access: %v", path, err)}
	}
	if err := file.Close(); err != nil {
		return Check{Name: "kvm", Detail: fmt.Sprintf("close KVM device %q: %v", path, err)}
	}
	return Check{Name: "kvm", OK: true, Detail: path}
}

// checkNBD 确认 module 参数有效且至少存在一个 size 为 0 的空闲 NBD device。
func checkNBD(maxPath, systemRoot string) Check {
	content, err := os.ReadFile(maxPath)
	if err != nil {
		return Check{Name: "nbd", Detail: fmt.Sprintf("NBD module parameter %q is unavailable: %v", maxPath, err)}
	}
	maximum, err := strconv.ParseUint(strings.TrimSpace(string(content)), 10, 32)
	if err != nil || maximum == 0 {
		return Check{Name: "nbd", Detail: fmt.Sprintf("NBD nbds_max must be a positive integer; got %q", strings.TrimSpace(string(content)))}
	}
	sizePaths, err := filepath.Glob(filepath.Join(systemRoot, "nbd*", "size"))
	if err != nil {
		return Check{Name: "nbd", Detail: fmt.Sprintf("inspect NBD devices: %v", err)}
	}
	for _, sizePath := range sizePaths {
		size, readErr := os.ReadFile(sizePath)
		// Linux 用 0 sector 表示未连接的 NBD device；upstream pool 分配 slot 前也使用同一信号。
		if readErr == nil && strings.TrimSpace(string(size)) == "0" {
			return Check{Name: "nbd", OK: true, Detail: fmt.Sprintf("NBD module exposes a free device (%d configured)", maximum)}
		}
	}
	return Check{Name: "nbd", Detail: fmt.Sprintf("NBD module has %d configured devices but no free nbd*/size entry", maximum)}
}

// checkHugePages 在启用时根据真实 page size 计算单次构建所需的最少空闲页数。
func checkHugePages(enabled bool, memoryMB int64, memInfoPath string) Check {
	if !enabled {
		return Check{Name: "hugepages", OK: true, Detail: "HugePages mode is disabled"}
	}
	if memoryMB <= 0 {
		return Check{Name: "hugepages", Detail: "HugePages mode requires positive MemoryMB"}
	}
	content, err := os.ReadFile(memInfoPath)
	if err != nil {
		return Check{Name: "hugepages", Detail: fmt.Sprintf("read %q: %v", memInfoPath, err)}
	}
	values := make(map[string]int64)
	for _, line := range strings.Split(string(content), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		value, parseErr := strconv.ParseInt(fields[1], 10, 64)
		if parseErr == nil {
			values[strings.TrimSuffix(fields[0], ":")] = value
		}
	}
	pageKB, pageOK := values["Hugepagesize"]
	free, freeOK := values["HugePages_Free"]
	if !pageOK || !freeOK || pageKB <= 0 {
		return Check{Name: "hugepages", Detail: "meminfo does not contain valid HugePages_Free and Hugepagesize values"}
	}
	// MiB 转 KiB 后向上取整，因为最后不足一页的内存仍会占用完整 host HugePage。
	required := (memoryMB*1024 + pageKB - 1) / pageKB
	if free < required {
		return Check{Name: "hugepages", Detail: fmt.Sprintf("build requires at least %d free HugePages but host reports %d", required, free)}
	}
	return Check{Name: "hugepages", OK: true, Detail: fmt.Sprintf("%d free HugePages; %d required", free, required)}
}
