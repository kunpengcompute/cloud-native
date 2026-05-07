# kata-cpuset-nri

`kata-cpuset-nri` 是一个基于 NRI 的 Pod 级 cpuset 收敛插件骨架。

当前骨架能力：
- 白名单过滤（namespace/runtimeClass）。
- 扫描所有 Pod 的 `cpuset.cpus` 并收敛。
- 固定目标：将 Pod 绑定到同物理核心的 2 个逻辑 CPU。

目录结构：
- `cmd/`：启动入口。
- `plugin/`：NRI 事件、Pod 快照、过滤和 cpuset 收敛。
- `topology/`：SMT sibling 拓扑发现。

当前不包含：
- 生产级配置加载与参数化。
- 完整 e2e 测试。
- 与 kubelet CPUManager 冲突检测。
