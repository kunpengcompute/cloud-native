# Kunpeng-TAP Pcore Biding 插件设计文档（初稿）

| 标题   | Kunpeng-TAP Pcore Biding 插件 |
|------|------------------------|
| 状态   | Draft                  |
| 作者   | Kunpeng Boostkit Team  |
| 创建日期 | 2026-05-07             |
| 目标版本 | v0.1.0                 |

## 1. 概述

新增一个基于 NRI 的插件项目，用于在 `containerd v2.1 + kata` 场景下，对符合条件的 Pod 进行 cpuset 收敛。  
插件只处理 Pod 层级，目标是把 Pod 的 `2` 个逻辑 CPU 绑定到同一个物理核心（SMT sibling 对）。

## 2. 目标与非目标

### 2.1 目标

1. 仅支持 Pod 层级绑核。
2. 仅处理聚合 CPU limit 恰好为 `2` 的 Pod，CPU request 不参与过滤。
3. 将 2 个逻辑 CPU 绑定到同一物理核心（同 core 的 sibling）。
4. 支持 namespace 白名单和 runtimeClass 白名单（如 `kata`）。
5. 不维护容器生命周期状态；采用扫描式收敛，以当前 cgroup cpuset 为事实来源。
6. 不允许多个符合条件的 Pod 收敛到同一对 sibling。

### 2.2 非目标

1. 不支持 container 层级独立策略。
2. 不支持多种 CPU 分配模式（按数量、按 NUMA 策略等）。
3. 不做容器退出回收逻辑。
4. 不替代 Kubernetes 调度器和 kubelet CPUManager 的上层调度决策。

## 3. 背景与约束

1. 节点需开启 SMT，且存在 sibling 成对可用的物理核心。
2. 插件依赖 NRI 与 containerd 交互，主要在 `Synchronize` 与 Pod 相关事件后触发收敛。
3. 插件写入目标为 Pod 对应 cgroup 的 `cpuset.cpus`（必要时写 `cpuset.mems`）。
4. 如果目标 Pod 当前 cpuset 已满足同核双线程约束，则不重复写。
5. 插件不持久化 sibling 分配结果；每次收敛时从当前 Pod 的 `cpuset.cpus` 重新计算占用情况。

## 4. 架构设计

### 4.1 模块划分

- `cmd/`：进程入口，只负责加载默认配置、发现拓扑并启动插件。
- `plugin/`：NRI 事件适配、Pod 元数据缓存、白名单过滤、cpuset 收敛。
- `topology/`：主机 CPU sibling 拓扑发现。

说明：初始版本不拆分 scanner、allocator、cpuset 等独立包，避免在需求尚未扩展前引入过多抽象。

### 4.2 Pod 元数据缓存

`Pod 元数据缓存` 指插件在内存中保存当前 runtime 视角下的 Pod 基本信息，包括 Pod ID、名称、namespace、runtimeClass 和 Pod cgroup 路径。  
该缓存只用于周期扫描时定位需要处理的 Pod，不保存绑定结果，也不承担资源分配状态持久化。

缓存来源：

- 插件启动或重连时通过 NRI `Synchronize` 接收当前 Pod 列表。
- Pod 创建时通过 `RunPodSandbox` 增量加入。
- Pod 删除时通过 `RemovePodSandbox` 从内存中移除。

### 4.3 收敛流程

1. 拉取当前 Pod 列表（来自 NRI 同步视图）。
2. 过滤非白名单 namespace、runtimeClass 或聚合 CPU limit 不等于 `2` 的 Pod。
3. 定位 Pod cgroup 并读取 `cpuset.cpus`。
4. 在单次扫描过程中维护临时 `occupied` 集合，记录已被本轮 Pod 占用的 sibling 对。
5. 为 Pod 计算未占用的目标 sibling 对（长度固定 2）。
6. 对比当前与目标 cpuset，不一致则执行写入。
7. 写入失败时记录错误并继续处理后续 Pod（单 Pod 故障隔离）。

### 4.4 cgroup 路径解析

插件默认从 `/proc/self/mountinfo` 自动发现 cpuset 控制器挂载点，并将 NRI 提供的 Pod cgroup parent 解析为实际 `cpuset.cpus` 所在目录。

解析顺序：

1. 如果 NRI 提供的是完整路径，且路径下存在 `cpuset.cpus`，直接使用。
2. 将 `cgroup-root` 与 NRI 提供的 cgroup parent 拼接，适配 `/kubepods.slice/...` 形态。
3. 对 `kubepods-*.slice:cri-containerd:<id>` 形态尝试转换为 systemd scope 路径。
4. 如果前面都不命中，在 `cgroup-root` 下按 cgroup basename 做有限 fallback 查找。

在 containerd + kata 场景下，Pod parent cgroup 和 sandbox cgroup 可能不是严格父子目录关系。插件会同时收敛 Pod parent cgroup 以及根据 sandbox ID 推导出的 sandbox cgroup，避免只写 parent 但实际 VM/shim 所在 cgroup 仍保留全量 CPU。

## 5. 核心算法

### 5.1 sibling 拓扑发现

从 `/sys/devices/system/cpu/cpu*/topology/thread_siblings_list` 读取 sibling 对，形成如下结构：

```text
coreID -> [cpuA, cpuB]
```

过滤规则：

- 仅保留恰好 2 个逻辑 CPU 的 sibling 对。
- v0.1 读取 sysfs 拓扑结果，不额外维护复杂 CPU 状态。

### 5.2 同核双线程选择

输入：Pod 当前 cpuset、全局 sibling 列表。  
输出：目标 `cpuset.cpus` 字符串（示例：`4,68`）。

选择策略（v0.1）：

1. 按 namespace、Pod 名称、Pod ID 排序，保证单次扫描顺序稳定。
2. 若当前 cpuset 已是某个 sibling 对，且本轮尚未被其他 Pod 占用，则保持不变。
3. 若当前 cpuset 不是有效 sibling 对，或该 sibling 对已被占用，则选择第一个未占用 sibling 对作为目标。
4. 如果没有未占用 sibling 对，跳过该 Pod 并记录日志。

说明：v0.1 不维护持久化分配状态，但每轮扫描会从当前 `cpuset.cpus` 重新构造临时占用集合，避免多个 Pod 收敛到同一对 sibling。

## 6. 配置设计

```bash
kunpeng-tap-pcore-biding \
  --nri-socket-path=/var/run/nri/nri.sock \
  --scan-interval=10s \
  --cgroup-root="" \
  --namespace-whitelist=default,workloads \
  --runtimeclass-whitelist=kata \
  --dry-run=false
```

字段说明：

- `nri-socket-path`：NRI socket 路径。
- `scan-interval`：周期扫描间隔。
- `cgroup-root`：cpuset cgroup 根路径；为空时插件通过 `/proc/self/mountinfo` 自动发现 cpuset 控制器挂载点。
- `namespace-whitelist`：逗号分隔的 namespace 白名单。
- `runtimeclass-whitelist`：逗号分隔的 runtimeClass 白名单。
- `dry-run`：仅输出计划写入，不实际执行写入。

## 7. 部署方案

插件最终以 Kubernetes DaemonSet 形式部署，每个节点运行一个插件容器。

容器运行约束：

- 需要访问 NRI socket，挂载 `/var/run/nri`。
- 需要读取 CPU 拓扑，挂载 `/sys`。
- 需要读写 Pod cgroup cpuset，挂载 cgroup 文件系统（通常为 `/sys/fs/cgroup`）。
- 容器默认以非特权模式运行，禁止权限提升并丢弃全部 Linux capabilities；只读挂载 NRI socket 目录和 CPU 拓扑，仅对 `/sys/fs/cgroup/cpuset` 保留写权限。

部署边界：

- DaemonSet 只负责节点本地收敛，不做跨节点协调。
- 插件重启后通过 NRI `Synchronize` 恢复 Pod 元数据缓存，并重新扫描 cgroup 当前状态。

## 8. 失败场景与处理

1. **无 SMT sibling 对**：记录错误并跳过节点收敛。
2. **Pod cgroup 路径解析失败**：记录警告并跳过该 Pod。
3. **cpuset 写入失败**：记录错误并继续处理其他 Pod。
4. **白名单为空**：按“全部不匹配”处理，避免误操作。
5. **可用 sibling 对不足**：超过可用 sibling 对数量的 Pod 本轮不收敛，记录日志。

## 9. 测试计划

1. 单元测试

   - 白名单过滤正确性。
   - sibling 对解析正确性。
   - cpuset 比对与格式化正确性。
   - sibling 去重选择正确性。
   - 收敛流程在写入失败或可用 sibling 不足时不中断。

2. 集成测试

   - containerd v2.1 + kata 环境下，验证目标 Pod cpuset 被收敛为同核双线程。
   - 验证多个符合条件的 Pod 不会收敛到同一对 sibling。
   - 验证非白名单 Pod 不受影响。
   - 验证 dry-run 模式不落盘。
   - 验证 DaemonSet 容器重启后可通过 NRI 同步重新收敛。

## 10. 里程碑

1. `v0.1`：设计与骨架代码、基本收敛链路打通。
2. `v0.2`：接入完整 NRI 事件与同步路径，补充集成测试。
3. `v1.0`：稳定性增强与生产化文档。
