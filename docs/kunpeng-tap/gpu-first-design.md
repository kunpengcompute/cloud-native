# GPU优先资源分配策略需求文档

## 1. 背景与问题陈述

### 1.1 当前问题

在现代异构计算集群中，GPU密集型工作负载（如机器学习训练、深度学习推理）面临以下问题：

1. **GPU与CPU跨NUMA访问延迟高**：当容器被调度到远离GPU设备的NUMA节点时，数据通过PCIe传输需要跨NUMA/Socket访问，导致延迟增加50%-100%
2. **GPU利用率下降**：由于数据供给速度不足，GPU出现"饥饿"状态，无法充分利用计算能力
3. **现有策略不感知GPU拓扑**：默认的CPU优先策略仅考虑CPU容量，忽略了GPU设备的物理位置

### 1.2 业务需求

用户需要一种资源分配策略，能够：
- 将GPU容器优先调度到GPU设备所在的NUMA节点
- 最小化GPU与系统内存之间的数据传输延迟
- 在GPU NUMA节点资源不足时，提供合理的回退机制

---

## 2. 用户故事

以下从不同角色的视角描述了对GPU优先策略的核心诉求：

| ID | 用户故事 |
|----|----------|
| US-01 | 作为**集群管理员**，我希望能够**配置GPU优先的资源分配策略**，以便**GPU工作负载获得最优的硬件亲和性** |
| US-02 | 作为**ML工程师**，我希望**我的深度学习训练任务自动调度到GPU所在的NUMA节点**，以便**获得最佳的数据传输性能** |
| US-03 | 作为**集群管理员**，我希望**当GPU NUMA节点资源不足时系统能智能回退**，以便**保证任务能够成功调度** |
| US-04 | 作为**运维人员**，我希望**能够通过命令行参数启用GPU优先策略**，以便**灵活控制调度行为** |

---

## 3. 功能需求

本节定义了GPU优先策略的核心功能需求，按功能领域进行分类。优先级说明：P0为必须实现，P1为建议实现。

### 3.1 配置需求

系统需要提供灵活的配置方式，允许管理员启用GPU优先策略：

| 需求ID | 需求描述 | 优先级 |
|--------|----------|--------|
| FR-CFG-01 | 系统**应**支持通过命令行参数 `--resource-priority=gpu-first` 启用GPU优先策略 | P0 |
| FR-CFG-02 | GPU优先策略的配置值**应**为 `gpu-first`（字符串常量） | P0 |
| FR-CFG-03 | 系统**应**默认使用CPU优先策略（`cpu-first`），GPU优先策略为可选配置 | P0 |

### 3.2 GPU设备识别需求

系统需要能够从容器配置中识别GPU设备请求，以便进行亲和性调度：

| 需求ID | 需求描述 | 优先级 |
|--------|----------|--------|
| FR-GPU-01 | 系统**应**能够通过容器环境变量 `VA_VISIBLE_DEVICES` 识别GPU设备请求 | P0 |
| FR-GPU-02 | 系统**应**能够通过容器环境变量 `VA_ALLOCATE_DEVICES` 识别GPU设备请求 | P0 |
| FR-GPU-03 | 系统**应**能够解析设备路径格式 `/dev/vaccX`（X为GPU ID） | P0 |
| FR-GPU-04 | 系统**应**支持解析多个GPU设备（逗号分隔格式） | P1 |

### 3.3 资源分配需求

以下定义了GPU优先策略下的资源分配行为，包括正常分配和资源不足时的回退机制：

| 需求ID | 需求描述 | 优先级 |
|--------|----------|--------|
| FR-ALLOC-01 | GPU容器**应**优先分配到请求的GPU设备所在的NUMA节点 | P0 |
| FR-ALLOC-02 | 当GPU所在NUMA节点资源不足时，系统**应**尝试分配到其他有GPU的NUMA节点 | P0 |
| FR-ALLOC-03 | 当所有GPU NUMA节点资源都不足时，系统**应**回退到常规CPU分配策略 | P1 |
| FR-ALLOC-04 | 分配**应**约束在单个NUMA节点级别（不跨NUMA分配） | P0 |
| FR-ALLOC-05 | 对于非GPU容器（纯CPU工作负载），系统**应**回退到CPU优先策略行为 | P1 |



---

## 4. 非功能需求

除功能需求外，系统还需满足以下性能、兼容性方面的要求：

| 需求ID | 需求描述 | 验收标准 |
|--------|----------|----------|
| NFR-01 | 策略配置变更**应**在服务重启后生效 | 重启后策略生效 |
| NFR-02 | GPU优先策略**应**与现有topology-aware框架兼容 | 现有测试用例通过 |
| NFR-03 | 策略**应**支持2NUMA和4NUMA拓扑配置 | 测试覆盖验证 |

---

## 5. 验收标准

以下验收标准用于确认功能需求是否正确实现。每个验收标准都关联到具体的功能需求，确保可追溯性。

| AC-ID | 验收标准 | 对应需求 |
|-------|----------|----------|
| AC-01 | 配置 `--resource-priority=gpu-first` 后，GPU容器分配到GPU所在NUMA | FR-CFG-01, FR-ALLOC-01 |
| AC-02 | 请求 GPU 0（位于NUMA 1）的容器被分配到 NUMA 1 | FR-ALLOC-01 |
| AC-03 | NUMA 1 资源不足时，GPU容器分配到其他有GPU的NUMA节点 | FR-ALLOC-02 |
| AC-04 | 所有GPU NUMA资源不足时，容器仍能成功分配（回退机制） | FR-ALLOC-03 |
| AC-05 | 纯CPU容器在GPU优先策略下回退到NUMA 0（最佳CPU可用性） | FR-ALLOC-05 |
| AC-06 | 环境变量 `VA_VISIBLE_DEVICES=/dev/vacc0,/dev/vacc1` 正确解析为 GPU 0, 1 | FR-GPU-01, FR-GPU-04 |

---

## 6. 设计与实现方案

本节简要描述为实现上述需求所采用的设计方案和主要代码变更点。

### 6.1 整体设计思路

GPU优先策略通过修改资源池排序逻辑来实现，核心思想是：**在调度决策时，将GPU亲和性作为首要考量因素，CPU容量作为次要因素**。

### 6.2 两阶段分配机制

为满足 FR-ALLOC-01 至 FR-ALLOC-03 的需求，采用两阶段分配策略：

| 阶段 | 行为 | 对应需求 |
|------|------|----------|
| **阶段1：特定GPU亲和** | 尝试将容器分配到请求的GPU设备所在的NUMA节点 | FR-ALLOC-01 |
| **阶段2：通用GPU回退** | 若阶段1失败，尝试分配到其他有GPU的NUMA节点 | FR-ALLOC-02 |
| **阶段3：常规回退** | 若阶段2也失败，回退到常规CPU优先分配 | FR-ALLOC-03 |

### 6.3 主要代码变更

以下是实现GPU优先策略的主要变更文件和变更内容：

| 变更文件 | 变更描述 |
|----------|----------|
| `pkg/kunpeng-tap/policy/policy.go` | 新增 `ResourcePriorityGPUFirst` 常量定义 |
| `pkg/kunpeng-tap/policy/topology-aware/resources.go` | 新增GPU设备识别逻辑（`checkDeviceRequest`、`extractVisibleDeviceIDs`） |
| `pkg/kunpeng-tap/policy/topology-aware/topology_aware.go` | 新增 `findGPUAffinityPool`、`calculateSpecificGPUAffinity`、`calculateGeneralGPUAffinity` 方法 |
| `cmd/kunpeng-tap/proxy/options/options.go` | 新增 `--resource-priority` 命令行参数 |

### 6.4 关键实现逻辑

**GPU设备识别**：通过解析容器环境变量 `VA_VISIBLE_DEVICES` 和 `VA_ALLOCATE_DEVICES`，提取请求的GPU设备ID列表。

**亲和性计算**：根据GPU设备与NUMA节点的映射关系，为每个NUMA节点计算亲和性分数。请求的GPU所在NUMA节点获得更高的分数。

**资源池排序**：在GPU优先策略下，首先按GPU亲和性分数排序，亲和性相同时再按CPU可用容量排序。

---

## 7. 约束与假设

### 7.1 约束

以下是系统设计和实现时必须遵守的限制条件：

| ID | 约束描述 |
|----|----------|
| CON-01 | GPU设备必须通过环境变量方式请求（`VA_VISIBLE_DEVICES` 或 `VA_ALLOCATE_DEVICES`） |
| CON-02 | GPU设备路径格式必须为 `/dev/vaccX` |
| CON-03 | 系统必须能够通过 sysfs 获取GPU到NUMA的映射关系 |

### 7.2 假设

以下是本需求成立的前提假设，如果假设不成立，可能需要重新评估需求：

| ID | 假设描述 |
|----|----------|
| ASM-01 | 每个NUMA节点的CPU核心数量相同 |
| ASM-02 | GPU设备均匀或已知分布在各NUMA节点上 |
| ASM-03 | 容器运行时（containerd/docker）会传递GPU环境变量 |

