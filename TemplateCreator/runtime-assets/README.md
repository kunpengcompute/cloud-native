# TemplateCreator runtime 资产

本目录保存 TemplateCreator 自己管理的 runtime manifest 和无法从可信公共地址下载的必要资产。它不要求调用机已经部署 E2B，也不把目标 E2B 节点的 `/fc-*` 文件作为主依赖。

## 版本基线

默认版本来自 infra tag `2026.09`、commit `4853232b93a8871b4ded73079184e801d84e0e26`：

- Kernel：`vmlinux-6.1.158`；
- Firecracker：`v1.12.1_a41d3fb`；
- envd：`0.5.3`，commit 标识 `4853232`；
- Busybox：`1.36.1`。

Kernel、Firecracker 和 Busybox 使用 manifest 中固定的公开 URL 与 SHA-256 下载。envd 没有可确认的通用公开 URL，因此 amd64 和 arm64 二进制随 module 保存于 `bundled/`。

infra `2026.09` upstream 内嵌的 BusyBox 是 x86-64 binary。TemplateCreator runtime adapter 会使用 manifest 准备并校验的 architecture-specific BusyBox 替换 system-init payload；ARM64 不会复用 upstream 的 x86-64 embed。同一进程首次安装后只接受相同 SHA-256 的 BusyBox。

## envd 可复现构建

两个 envd 文件均使用 Go `1.26.5` 从上述锁定 infra commit 的 `packages/envd` 目录构建，未修改 infra 源码。构建命令为：

```bash
CGO_ENABLED=0 GOOS=linux GOARCH=<amd64|arm64> \
  go build -trimpath -buildvcs=false -a \
  -ldflags '-X=main.commitSHA=4853232 -s -w -buildid=' \
  -o envd .
```

校验值：

```text
amd64  d8f65a67276a13f82f192692da5cdd262649e8c232264a7e620cef0880ac7b2c
arm64  4f9faf21edbc8ededb45ece52e167d2f64af94b88bdb58ecfb2255d1c5c2da8a
```

上游源码采用 Apache-2.0，许可证副本位于 `../LICENSES/e2b-infra-APACHE-2.0.txt`。

## 与目标 E2B 的兼容边界

TemplateCreator 独立准备 runtime，不等于 Firecracker snapshot 可以脱离 runtime ABI。目标 E2B 必须具备 manifest 和最终 BuildMetadata 记录的 Kernel、Firecracker、envd 与 architecture；若目标环境没有相同 ABI，需要先把这一版本集同步到目标 E2B。
