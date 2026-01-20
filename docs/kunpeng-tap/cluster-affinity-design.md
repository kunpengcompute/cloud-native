# Cluster 亲和资源分配策略设计文档

| 术语 | 说明 |
|------|------|
| **Cluster** | NUMA 节点内部的物理分组，共享 L3 缓存的最小单元 |
| **SMT** | Simultaneous Multi-Threading，同步多线程技术，类似于 Intel 的超线程 |
| **Cluster Affinity** | 优先将容器分配在单个 Cluster 内的资源分配策略 |
| **Guaranteed QoS** | Kubernetes 中的服务质量等级，要求 CPU 和内存的 request 等于 limit |
| **Topology-Aware** | 感知硬件拓扑结构的调度策略 |
| **Resource Pool** | 资源池，用于管理和分配 CPU 资源 |

## 1. 背景与问题陈述

### 1.1 当前问题

在新一代鲲鹏服务器上，传统的 NUMA 级别资源分配策略面临以下挑战：

1. **NUMA 内部跨 Cluster 迁移开销高**：新机型的 NUMA 节点内部包含多个物理 Cluster（每个 Cluster 8-16 个 CPU 核心），容器在 NUMA 内部跨 Cluster 迁移时仍会产生显著的性能损耗
2. **细粒度亲和性缺失**：现有的 NUMA 级别分配无法感知 Cluster 拓扑，导致小容器（CPU 需求 ≤ Cluster 大小）可能被分散到多个 Cluster 上
3. **缓存一致性开销增加**：跨 Cluster 的 CPU 分配会增加 L3 缓存一致性协议的开销，降低缓存命中率

### 1.2 新机型拓扑特点

新一代鲲鹏服务器采用多层次拓扑结构，引入了 Cluster 作为 NUMA 和 CPU 之间的中间层：

```
Socket (192 CPUs with SMT / 96 CPUs without SMT)
  └── NUMA Node (48 CPUs with SMT / 24 CPUs without SMT)
        └── Cluster (16 CPUs with SMT / 8 CPUs without SMT)
              └── CPU Core
```

**关键特性**：
- 每个 Socket 包含多个 NUMA 节点（2/4）
- 每个 NUMA 节点包含多个 Cluster（3/6）
- Cluster 是共享 L3 缓存的最小单元
- 跨 Cluster 访问延迟显著高于 Cluster 内访问

### 1.3 业务需求

用户需要一种资源分配策略，能够：
- 对于小容器（CPU 需求 ≤ Cluster 大小），优先分配在单个 Cluster 内
- 对于中等容器（Cluster < CPU 需求 ≤ NUMA 大小），回退到 NUMA 级别分配
- 对于大容器（CPU 需求 > NUMA 大小），回退到 Socket 级别分配
- 最大化 L3 缓存命中率，减少跨 Cluster 迁移

---

## 2. 用户故事

以下从不同角色的视角描述了对 Cluster 亲和策略的核心诉求：

| ID | 用户故事 |
|----|----------|
| US-01 | 作为**集群管理员**，我希望能够**为支持 Cluster 拓扑的新机型启用 Cluster 亲和策略**，以便**充分利用硬件的多层次拓扑结构** |
| US-02 | 作为**性能工程师**，我希望**微服务容器（4-8 核）能够分配在单个 Cluster 内**，以便**获得最佳的缓存局部性和最低的迁移开销** |
| US-03 | 作为**性能工程师**，我希望**系统能够根据容器大小自动选择合适的分配层次**，以便**在不同规模的工作负载下都能获得最优性能** |

---

## 3. 功能需求

本节定义了 Cluster 亲和策略的核心功能需求，按功能领域进行分类。

### 3.1 机型检测需求

系统需要能够自动检测新机型，并根据机型特性启用相应功能：

| 需求ID | 需求描述 |
|--------|----------|
| FR-DET-01 | 系统应能够通过 CPU part ID 检测特定机型，判断该机型是否支持 Cluster 拓扑 |
| FR-DET-02 | 系统应能够从 `/proc/cpuinfo` 读取 CPU part ID 信息 |
| FR-DET-03 | 不支持 Cluster 拓扑的机型应自动禁用 Cluster 亲和功能，回退到 NUMA 级别分配 |

### 3.2 Cluster 拓扑发现需求

系统需要能够从 sysfs 发现 Cluster 拓扑信息：

| 需求ID | 需求描述 |
|--------|----------|
| FR-TOPO-01 | 系统应能够从 `/sys/devices/system/cpu/cpu*/topology/cluster_cpus_list` 读取 Cluster 拓扑 |
| FR-TOPO-02 | 系统应能够动态计算 Cluster 大小（支持 SMT 开启/关闭） |
| FR-TOPO-03 | 系统应能够构建 Cluster 到 NUMA 的映射关系 |

### 3.3 配置需求

系统需要提供灵活的配置方式，允许管理员控制 Cluster 亲和功能：

| 需求ID | 需求描述 |
|--------|----------|
| FR-CFG-01 | 系统应支持通过命令行参数 `--topology-cluster-affinity` 启用 Cluster 亲和策略 |
| FR-CFG-02 | Cluster 亲和策略应默认禁用，需要显式启用 |
| FR-CFG-03 | 配置变更应在 kunpeng-tap 重启后生效 |

### 3.4 资源分配需求

以下定义了 Cluster 亲和策略下的资源分配行为，包括三层次分配逻辑：

| 需求ID | 需求描述 |
|--------|----------|
| FR-ALLOC-01 | 当容器 CPU 需求 ≤ Cluster 大小时，应优先尝试单 Cluster 分配 |
| FR-ALLOC-02 | 当容器 CPU 需求 > Cluster 大小且 ≤ NUMA 大小时，应回退到 NUMA 级别分配 |
| FR-ALLOC-03 | 当容器 CPU 需求 > NUMA 大小时，应回退到 Socket 级别分配 |
| FR-ALLOC-04 | Cluster 级别分配应约束在单个 Cluster 内（不跨 Cluster） |
| FR-ALLOC-05 | 当目标 Cluster 资源不足时，应尝试其他 Cluster |
| FR-ALLOC-06 | 对于 Guaranteed QoS 的 Pod，应自动应用 Cluster 亲和策略 |
| FR-ALLOC-07 | 对于 Burstable QoS 的 Pod，应基于 limit 值应用 Cluster 亲和策略 |

### 3.5 监控需求

系统需要提供 Prometheus 指标，用于监控 Cluster 亲和功能的运行状态：

| 需求ID | 需求描述 |
|--------|----------|
| FR-MON-01 | 系统应提供 Cluster 级别分配的计数指标 |

---

## 4. 非功能需求

除功能需求外，系统还需满足以下性能、兼容性方面的要求：

| 需求ID | 需求描述 |
|--------|----------|
| NFR-01 | Cluster 拓扑发现应在 kunpeng-tap 启动时完成，不应影响启动时间 | 启动时间增加 < 250ms |
| NFR-02 | Cluster 亲和策略应与现有 topology-aware 框架兼容 | 现有测试用例通过 |
| NFR-03 | 策略应支持 SMT 开启和关闭两种配置 | 测试覆盖验证 |
| NFR-04 | 非新机型应能够正常运行，不受 Cluster 亲和代码影响 | 在非新机型上测试通过 |

---

## 5. 验收标准

以下验收标准用于确认功能需求是否正确实现。每个验收标准都关联到具体的功能需求，确保可追溯性。

| AC-ID | 验收标准 |
|-------|----------|
| AC-01 | 在支持 Cluster 拓扑的新机型上启用 `--topology-cluster-affinity` 后，4 核容器分配在单个 Cluster 内 |
| AC-02 | 在新机型上，20 核容器（> Cluster 大小）分配在单个 NUMA 内 |
| AC-03 | 在新机型上，50 核容器（> NUMA 大小）分配在单个 Socket 内 |
| AC-04 | 不支持 Cluster 拓扑的机型上启用 Cluster 亲和后，功能自动禁用，回退到 NUMA 级别分配 |
| AC-05 | Cluster 资源不足时，容器能够分配到其他 Cluster |
| AC-06 | 系统能够正确识别 SMT 开启和关闭两种配置 |
| AC-07 | Prometheus 指标能够正确统计 Cluster 级别分配的计数 |

---

## 6. 设计与实现方案

本节详细描述为实现上述需求所采用的设计方案和主要代码变更点。

### 6.1 整体架构设计

Cluster 亲和策略通过扩展现有的 topology-aware 框架实现，在资源分配决策中引入 Cluster 层次感知。核心思想是：**将 Cluster 作为独立的资源池节点加入资源树，通过节点深度自动实现优先级排序，优先使用细粒度（Cluster）分配，必要时自动回退到粗粒度（NUMA/Socket）分配**。

#### 关键设计变更

相比设计文档的原始方案，当前实现有以下重大改进：

1. **从策略选择到节点优先**：不再使用显式的条件判断选择分配层次，而是通过节点深度自动实现优先级
2. **统一接口设计**：Cluster 节点实现完整的 Node 接口，与其他节点类型平等参与资源分配
3. **简化决策逻辑**：资源池排序算法无需特殊处理 Cluster，深度排序自动保证优先级
4. **增强可扩展性**：未来添加新的拓扑层次（如 Die、Core）无需修改分配逻辑

#### 6.1.1 系统架构图

```
┌───────────────────────────────────────────────────────────────────┐
│                      kunpeng-tap Proxy                            │
│                                                                   │
│  ┌─────────────────────────────────────────────────────────────┐  │
│  │              拓扑感知策略 (Topology-Aware Policy)             │  │
│  │                                                             │  │
│  │  ┌───────────────────────────────────────────────────────┐  │  │
│  │  │         Cluster 亲和模块                               │  │  │
│  │  │                                                       │  │  │
│  │  │  ┌──────────────┐      ┌────────────────────────┐     │  │  │
│  │  │  │ 机型检测       │     │ Cluster 拓扑发现         │     │  │  │
│  │  │  │ SupportsCluster│    │ DiscoverClusters()     │     │  │  │
│  │  │  │ Feature()    │─────▶│                        │     │  │  │
│  │  │  └──────────────┘      └────────────────────────┘     │  │  │
│  │  │         │                        │                    │  │  │
│  │  │         ▼                        ▼                    │  │  │
│  │  │  ┌────────────────────────────────────────────┐       │  │  │
│  │  │  │   资源树构建                                 │       │  │  │
│  │  │  │                                            |       │  │  │
│  │  │  │  Socket ──▶ NUMA ──▶ Cluster (新增)         │       │  │  │
│  │  │  │                                            │       │  │  │
│  │  │  │  Cluster 作为独立资源池参与分配                │       │  │  │
│  │  │  └────────────────────────────────────────────┘       │  │  │
│  │  │         │                                             │  │  │
│  │  │         ▼                                             │  │  │
│  │  │  ┌────────────────────────────────────────────┐       │  │  │
│  │  │  │   基于深度的资源池排序                         │       │  │  │
│  │  │  │                                             │       │  │  │
│  │  │  │  1. 深度优先 (Cluster > NUMA > Socket)       │       │  │  │
│  │  │  │  2. 容量检查                                 │       │  │  │
│  │  │  │  3. 共享容量比较                             │       │  │  │
│  │  │  │  4. 容器共置比较                             │       │  │  │
│  │  │  └────────────────────────────────────────────┘       │  │  │
│  │  │         │                                             │  │  │
│  │  │         ▼                                             │  │  │
│  │  │  ┌────────────────────────────────────────────┐       │  │  │
│  │  │  │   资源分配与监控                              │       │  │  │
│  │  │  │   - 自动优先选择 Cluster                      │       │  │  │
│  │  │  │   - 资源不足时自动回退到 NUMA/Socket           │       │  │  │
│  │  │  │   - Prometheus 监控指标                      │       │  │  │
│  │  │  └────────────────────────────────────────────┘       │  │  │
│  │  └───────────────────────────────────────────────────────|  │  │
│  └─────────────────────────────────────────────────────────────┘  │
│                                                                   │
│  ┌─────────────────────────────────────────────────────────┐      │
│  │              系统接口                                     │      │
│  │                                                         │      │
│  │  /proc/cpuinfo  ←─  CPU 机型检测                          │      │
│  │  /sys/.../cluster_cpus_list  ←─  Cluster 拓扑            │      │
│  └─────────────────────────────────────────────────────────┘      │
└───────────────────────────────────────────────────────────────────┘
```

**核心设计原则**：
1. **统一接口**：Cluster 节点实现与其他节点相同的 Node 接口
2. **深度优先**：通过节点深度自动实现优先级，无需特殊逻辑
3. **自动回退**：资源不足时自动选择父节点（NUMA/Socket）
4. **统一监控**：所有节点类型使用相同的监控指标体系

#### 6.1.2 拓扑层次结构

新机型的拓扑层次结构如下（以典型配置为例）：

```
机器 (2 个 Socket)
│
├── Socket 0
│   ├── NUMA 0
│   │   ├── Cluster 0
│   │   │   └── CPU: 0-7 (SMT 关闭) / 0-15 (SMT 开启)
│   │   ├── Cluster 1
│   │   │   └── CPU: 8-15 (SMT 关闭) / 16-31 (SMT 开启)
│   │   └── Cluster 2
│   │       └── CPU: 16-23 (SMT 关闭) / 32-47 (SMT 开启)
│   │
│   └── NUMA 1
│       ├── Cluster 3, 4, 5
│       └── ...
│
└── Socket 1
    ├── NUMA 2
    │   ├── Cluster 6, 7, 8
    │   └── ...
    └── NUMA 3
        ├── Cluster 9, 10, 11
        └── ...
```

### 6.2 基于资源池深度的自适应分配策略

为满足 FR-ALLOC-01 至 FR-ALLOC-03 的需求，当前实现采用统一的资源池排序和选择机制。核心思想是：**所有节点类型（Socket、NUMA、Cluster）都实现相同的 Node 接口，通过深度优先的排序算法自动实现分配层次的选择**。

#### 6.2.1 分配决策流程

```
┌────────────────────────────────────────────────────────────────┐
│                   1. 容器资源请求到达                             │
│                   (CPU request/limit, Memory limit)            │
└─────────────────────────────┬──────────────────────────────────┘
                              │
                              ▼
┌────────────────────────────────────────────────────────────────┐
│                   2. 资源池评分 (sortPoolsByScore)               │
│                   - 遍历所有资源池节点                            │
│                   - 计算每个池的得分 (GetScore)                   │
│                   - 考虑 GPU 亲和性权重（如果适用）                 │
└─────────────────────────────┬──────────────────────────────────┘
                              │
                              ▼
┌────────────────────────────────────────────────────────────────┐
│                   3. 资源池排序 (compare)                        │
│                   按优先级顺序比较两个资源池：                      │
│                                                                │
│  ① 资源容量检查                                                 │
│     - SharedCapacity >= 0 的池优先                              │
│     - 不满足需求的池被淘汰                                        │
│                                                                │
│  ② 深度比较  ★关键★                                             │
│     - 深度更大的池优先（RootDistance）                            │
│     - Cluster > NUMA > Socket（深度：3 > 2 > 1）                 │
│                                                                │
│  ③ 共享容量比较                                                 │
│     - 共享容量更大的池优先                                        │
│                                                                │
│  ④ 容器共置数比较                                               │
│     - 共置容器更少的池优先                                        │
│                                                                │
│  ⑤ 资源优先级策略                                               │
│     - GPU-First: GPU 亲和性高的优先                              │
│     - CPU-First: CPU 容量大的优先                                │
│                                                                │
│  ⑥ 最终决胜                                                     │
│     - 节点 ID 更小的池优先                                        │
└─────────────────────────────┬──────────────────────────────────┘
                              │
                              ▼
┌────────────────────────────────────────────────────────────────┐
│                   4. 选择最优资源池                              │
│                   (findBestAvailablePool)                      │
│                   - 从排序后的池中选择第一个                       │
│                   - 自动选择最深层级的可用池                      │
│                   - 如 Cluster 可用 → 选择 Cluster              │
│                   - 如 Cluster 不可用 → 回退到 NUMA              │
│                   - 如 NUMA 不可用 → 回退到 Socket               │
└─────────────────────────────┬──────────────────────────────────┘
                              │
                              ▼
┌────────────────────────────────────────────────────────────────┐
│                   5. 资源分配                                   │
│                   - 从选定池分配资源 (Allocate)                  │
│                   - 更新资源池的可用容量                          │
│                   - 向上传播资源使用情况到父节点                    │
└────────────────────────────────────────────────────────────────┘
```

#### 6.2.2 关键实现机制

##### 6.2.2.1 深度优先排序

深度优先是实现三层次自动分配的核心机制：

```go
// topology_aware.go:compareDepth
func (p *TopologyAwarePolicy) compareDepth(node1, node2 Node) (bool, bool) {
    depth1, depth2 := node1.RootDistance(), node2.RootDistance()

    if depth1 > depth2 {
        return true, true  // node1 wins (更深的节点优先)
    }
    if depth2 > depth1 {
        return false, true // node2 wins
    }
    return false, false   // tie, 继续其他比较
}
```

**深度示例**（950 机器）：
```
Socket 节点:   RootDistance = 1
NUMA 节点:     RootDistance = 2
Cluster 节点:   RootDistance = 3  ← 最深，优先级最高
```

##### 6.2.2.2 容量检查与自动回退

容量检查确保只有满足资源需求的池被考虑：

```go
// topology_aware.go:checkCapacityByRequest
func checkCapacityByRequest(request Request, root, pool Node) bool {
    // 不断向上遍历，确认是否满足容量条件
    for pool != nil && pool != root {
        freeResource := pool.FreeResource()
        score := freeResource.GetScore(request)

        // SharedCapacity < 0 表示不满足需求
        if score.SharedCapacity() < 0 {
            return false  // 池被淘汰
        }
        pool = pool.Parent()
    }
    return true
}
```

**自动回退机制**：
1. Cluster 节点容量不足 → `SharedCapacity < 0` → 被淘汰
2. NUMA 节点容量充足 → 继续参与排序
3. 最终选择 NUMA 节点（剩余池中最深的）

##### 6.2.2.3 资源分配与传播

分配资源时，从选定池分配并向上传播：

```go
// topology_aware.go:allocatePool
supply := pool.FreeResource()
grant, err := supply.Allocate(request)  // 从池分配资源

// 向上传播资源使用情况
p.propagateResourceUsageToParent(grant)
```

### 6.3 机型检测机制

#### 6.3.1 检测流程

```
┌─────────────────────────────────────────────────────────┐
│              Cluster 支持检测                             │
└─────────────────────────────────────────────────────────┘
                           │
                           ▼
              ┌────────────────────────┐
              │ 读取 /proc/cpuinfo      │
              └────────────┬───────────┘
                           │
                           ▼
              ┌────────────────────────┐
              │ 提取 "CPU part"         │
              │ 字段值                  │
              └────────────┬───────────┘
                           │
                           ▼
              ┌────────────────────────┐
              │ 匹配支持的               │
              │ CPU part ID?           │
              └──┬──────────────────┬──┘
                 │ 是               │ 否
                 │                  │
                 ▼                  ▼
    ┌────────────────────┐   ┌──────────────────┐
    │ 返回 true           │   │ 返回 false        │
    │ (支持)              │   │ (不支持)          │
    └────────────────────┘   └──────────────────┘
```

#### 6.3.2 CPU Part ID

系统通过读取 `/proc/cpuinfo` 中的 CPU part ID 字段来检测特定机型，从而判断该机型是否支持 Cluster 拓扑。不同的 CPU part ID 对应不同的处理器型号，只有特定型号的处理器才支持 Cluster 拓扑结构。代码实现在 `pkg/kunpeng-tap/sysfs/system/system.go`中。


### 6.4 Cluster 拓扑发现

#### 6.4.1 发现流程

```
┌─────────────────────────────────────────────────────────┐
│              Cluster 拓扑发现                            │
└─────────────────────────────────────────────────────────┘
                           │
                           ▼
        ┌──────────────────────────────────────┐
        │ 遍历系统中的每个 CPU                    │
        │ 从 sysfs 读取 cluster_cpus_list       │
        └────────────────┬─────────────────────┘
                         │
                         ▼
        ┌──────────────────────────────────────┐
        │ 解析 cluster_cpus_list                │
        │ 示例: "0-7" 或 "0,1,2,3,4,5,6,7"      │
        └────────────────┬─────────────────────┘
                         │
                         ▼
        ┌──────────────────────────────────────┐
        │ 按 cluster_cpus_list 分组 CPU         │
        │ 构建 Cluster ID → CPU Set 映射        │
        └────────────────┬─────────────────────┘
                         │
                         ▼
        ┌──────────────────────────────────────┐
        │ 计算最大 Cluster 大小                  │ 
        │ (用于动态 Cluster 大小检测)             │
        └────────────────┬─────────────────────┘
                         │
                         ▼
        ┌──────────────────────────────────────┐
        │ 构建 Cluster → NUMA 映射               │
        │ (通过 CPU → NUMA 关系)                 │
        └──────────────────────────────────────┘
```

#### 6.4.2 Sysfs 路径

Cluster 拓扑信息存储在 sysfs 的 `/sys/devices/system/cpu/cpu*/topology/cluster_cpus_list` 文件中，每个 CPU 都有一个对应的文件。

```
/sys/devices/system/cpu/cpu0/topology/cluster_cpus_list
/sys/devices/system/cpu/cpu1/topology/cluster_cpus_list
...
/sys/devices/system/cpu/cpu95/topology/cluster_cpus_list
```

**示例输出**（SMT 关闭）：
```
cpu0:  cluster_cpus_list = 0-7      (Cluster 0)
cpu8:  cluster_cpus_list = 8-15     (Cluster 1)
cpu16: cluster_cpus_list = 16-23    (Cluster 2)
cpu24: cluster_cpus_list = 24-31    (Cluster 3)
...
```

### 6.5 资源树扩展

为支持 Cluster 层次，当前实现将 Cluster 作为 NUMA 节点下的子节点加入资源树，参与资源分配计算。Cluster 节点成为资源池（pool）的一种，与 NUMA 节点、Socket 节点等共同参与资源分配决策。

#### 6.5.1 资源树结构

```
资源树 (ResourceTree)
│
└── Socket 节点 (Socket 0)
      │
      ├── NUMA 节点 (NUMA 0)
      │     │
      │     ├── Cluster 节点 (Cluster 0)  ← 新增层次，作为资源池
      │     │     └── CPUs: 0-7
      │     │
      │     ├── Cluster 节点 (Cluster 1)  ← 新增层次，作为资源池
      │     │     └── CPUs: 8-15
      │     │
      │     └── Cluster 节点 (Cluster 2)  ← 新增层次，作为资源池
      │           └── CPUs: 16-23
      │
      └── NUMA 节点 (NUMA 1)
            └── ...
```

**关键设计变更**：
- Cluster 节点作为独立的资源池节点，直接参与资源分配
- Cluster 节点具有比 NUMA 节点更深的深度（RootDistance），因此在资源池排序中具有更高优先级
- 资源分配算法会自动优先选择 Cluster 层次的资源池，只有在 Cluster 资源不足时才回退到 NUMA 层次

#### 6.5.2 ClusterNode 类型定义

ClusterNode 实现了 Node 接口，作为 NUMA 节点的子节点参与资源树构建。

```go
// pkg/kunpeng-tap/policy/topology-aware/node.go
type clusterNode struct {
    baseNode
    clusterID  system.ID      // Cluster ID
    numaNodeID system.ID      // Parent NUMA node ID
    cpus       cpuset.CPUSet  // CPUs in this cluster
    sysCluster system.Cluster // System cluster info
}
```

**核心特性**：
- 实现完整的 Node 接口，包括资源发现、评分、分配等功能
- 支持内存信息查询（通过父 NUMA 节点）
- 支持资源分配和释放
- 具有独立的资源供应（Supply）管理

### 6.6 配置与启用

#### 6.6.1 命令行参数

Cluster 亲和功能通过 kunpeng-tap 的命令行参数启用：

```bash
kunpeng-tap \
  --topology-cluster-affinity=true \
  --other-options...
```

## 7. 约束与假设

本节列出实现 Cluster 亲和功能时的关键约束条件和设计假设。

### 7.1 硬件约束

| 约束项 | 说明 |
|--------|------|
| **机型限制** | 仅支持具有 Cluster 拓扑的新机型，其他机型自动禁用 Cluster 亲和功能 |
| **内核支持** | 需要 Linux 内核提供 `/sys/devices/system/cpu/cpu*/topology/cluster_cpus_list` 接口 |
| **拓扑结构** | 假设拓扑结构为：Socket → NUMA → Cluster → CPU，每个 NUMA 包含多个 Cluster |
| **Cluster 大小** | Cluster 大小通过 sysfs 动态获取，典型值为 8 CPUs（SMT 关闭）或 16 CPUs（SMT 开启） |

### 7.2 软件约束

| 约束项 | 说明 |
|--------|------|
| **QoS 类型** | Cluster 亲和策略对 Guaranteed 和 Burstable QoS 的 Pod 生效，BestEffort 不受影响 |
| **配置方式** | Cluster 亲和功能通过 kunpeng-tap 命令行参数全局启用，不支持 Pod 级别的注解控制 |
| **重启要求** | 配置变更需要重启 kunpeng-tap 服务才能生效 |

### 7.3 设计假设

| 假设项 | 说明 |
|--------|------|
| **拓扑稳定性** | 假设系统运行期间 CPU 拓扑不会发生变化（不考虑 CPU 热插拔） |
| **缓存局部性** | 假设 Cluster 内的 CPU 共享 L3 缓存，跨 Cluster 访问会产生显著的性能损耗 |
| **分配优先级** | 假设优先使用细粒度分配（Cluster）能够获得更好的性能，仅在资源不足时才回退到粗粒度（NUMA/Socket） |

### 7.4 已知限制

#### 7.4.1 不支持 CPU 热插拔

系统启动后不会重新扫描 Cluster 拓扑。如果运行时添加或移除 CPU，需要重启 kunpeng-tap 服务以重新发现拓扑结构。这是因为拓扑信息在服务启动时一次性读取并缓存，不会动态更新。

#### 7.4.2 不支持异构 Cluster

当前实现假设所有 Cluster 大小相同。如果系统中存在不同大小的 Cluster，可能导致资源分配不均衡。分配策略使用 `GetMaxClusterSize()` 作为统一的 Cluster 大小阈值，无法针对不同大小的 Cluster 进行差异化处理。

## 8. 测试计划

为确保 Cluster 亲和功能的正确性和稳定性，采用多层次的测试策略。

### 8.1 单元测试

**测试框架**：Ginkgo + Gomega

**测试覆盖范围**：
- 机型检测逻辑
- Cluster 拓扑发现
- 三层次分配策略
- 资源池管理
- 回退机制

**Mock 策略**：
- 使用 `MockSystem` 模拟机型检测
- 使用 `MockClusterInfo` 模拟 Cluster 拓扑
- 使用 `MockResourceTree` 模拟资源树

### 8.2 BDD 测试

**测试环境**：
- 支持 Cluster 拓扑的服务器
- Kubernetes 集群
- kunpeng-tap 部署

**测试场景**：
- 端到端容器调度
- 长时间稳定性测试
- 故障恢复测试

**测试框架**：pytest-bdd

**测试覆盖范围**：
- 基础分配场景（小、中、大容器）
- 边界条件（Cluster 边界、NUMA 边界、Socket 边界）
- 资源竞争场景（Cluster 耗尽、NUMA 耗尽）
- QoS 行为（Guaranteed vs Burstable）
- SMT 配置（开启 vs 关闭）
- 混合部署场景（多种大小容器混合）

## 参考资料

- [Linux CPU Topology](https://www.kernel.org/doc/Documentation/cputopology.txt)
- [ARM Neoverse V2 Architecture](https://developer.arm.com/Processors/Neoverse%20V2)
- [NUMA Aware Scheduling](https://www.kernel.org/doc/html/latest/scheduler/numa.html)

