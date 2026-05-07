# kata-cpuset-nri

`kata-cpuset-nri` 是一个基于 NRI 的 Pod 级 cpuset 收敛插件骨架。

当前骨架能力：
- 白名单过滤（namespace/runtimeClass）。
- 扫描所有 Pod 的 `cpuset.cpus` 并收敛。
- 固定目标：将 Pod 绑定到同物理核心的 2 个逻辑 CPU。
- 通过 `/proc/self/mountinfo` 自动发现 cpuset cgroup 根路径，也支持参数覆盖。

运行参数：
- `--nri-socket-path`：NRI socket 路径，默认 `/var/run/nri/nri.sock`。
- `--scan-interval`：周期扫描间隔，默认 `10s`。
- `--cgroup-root`：cpuset cgroup 根路径，默认自动发现。
- `--namespace-whitelist`：逗号分隔 namespace 白名单，默认 `default`。
- `--runtimeclass-whitelist`：逗号分隔 runtimeClass 白名单，默认 `kata`。
- `--dry-run`：只打印计划，不写 cgroup。

目录结构：
- `cmd/`：启动入口。
- `plugin/`：NRI 事件、Pod 快照、过滤和 cpuset 收敛。
- `topology/`：SMT sibling 拓扑发现。

镜像构建：

```bash
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o kata-cpuset-nri ./pkg/kata-cpuset-nri/cmd
docker build -f Dockerfile.kata-cpuset-nri -t kata-cpuset-nri:latest .
rm -f kata-cpuset-nri
```

Kubernetes 部署清单：
- `config/kata-cpuset-nri/daemonset.yaml`
- 默认启用 `--dry-run=true`，用于先验证 DaemonSet 形态启动和 NRI 注册。
- 默认 runtimeClass 白名单包含 `kata,kata-clh`，便于分别测试 QEMU 和 cloud-hypervisor 后端。

测试 Pod 清单：
- `config/kata-cpuset-nri/test-pods.yaml`
- 创建两个 `runtimeClassName: kata`、CPU request/limit 均为 `2` 的 Pod，用于验证 dry-run 模式下目标 sibling 规划不会重复。
- 如果测试环境的 kata 配置不支持 CPU hotplug，Pod 可能进入 `StartError`；此时仍可通过插件日志确认 NRI 事件和 dry-run 目标规划是否生效。
- `config/kata-cpuset-nri/runtimeclass-cloud-hypervisor.yaml` 定义 `kata-clh` RuntimeClass。
- `config/kata-cpuset-nri/test-pods-cloud-hypervisor.yaml` 创建两个 `runtimeClassName: kata-clh` 的测试 Pod，用于 cloud-hypervisor 后端验证。

当前不包含：
- 完整 e2e 测试。
- 与 kubelet CPUManager 冲突检测。
