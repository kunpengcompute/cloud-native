# Kunpeng-TAP NRI-Server 特性设计文档

| 标题         | Kunpeng-TAP NRI-Server 特性设计                    |
|------------|-----------------------------------------------|
| 状态         | Implemented                                   |
| 作者         | Kunpeng Boostkit Team                         |
| 创建日期       | 2025-01-01                                    |
| 最后更新       | 2025-12-03                                    |
| 目标版本       | v0.3.0                                        |

## 目录

- [概述](#概述)
- [动机](#动机)
  - [目标](#目标)
  - [非目标](#非目标)
- [背景知识](#背景知识)
- [设计详情](#设计详情)
  - [整体架构](#整体架构)
  - [核心组件](#核心组件)
  - [NRI 事件处理流程](#nri-事件处理流程)
  - [数据结构设计](#数据结构设计)
  - [接口定义](#接口定义)
- [部署方案](#部署方案)
- [配置说明](#配置说明)
- [测试计划](#测试计划)
- [参考资料](#参考资料)

---

## 概述

Kunpeng-TAP (Topology-Affinity Plugin) NRI-Server 是一个基于 Node Resource Interface (NRI) 的容器运行时插件，专为鲲鹏处理器优化设计。它通过 NRI 接口与 containerd 运行时集成，实现容器资源的拓扑感知调度，支持 NUMA 感知和拓扑感知的容器资源分配策略。

### 核心特性

- **NRI 原生集成**：通过 NRI 标准接口与 containerd ≥ v1.7.0 无缝集成
- **拓扑感知资源分配**：基于硬件拓扑结构（Socket/Die/NUMA）进行智能资源分配
- **容器生命周期管理**：完整支持 Pod 和容器的创建、启动、停止、删除等生命周期事件
- **灵活的策略框架**：支持 `topology-aware` 和 `numa-aware` 两种资源分配策略
- **容器化部署**：以 DaemonSet 方式部署到 Kubernetes 集群

---

## 动机

### 目标

1. **简化部署流程**：通过 NRI 插件模式替代传统的代理模式，无需修改容器运行时配置
2. **提升资源利用率**：通过拓扑感知的资源分配策略，优化容器在多 NUMA 节点系统上的性能
3. **降低延迟**：减少内存跨 NUMA 节点访问，降低容器运行时的内存访问延迟
4. **标准化集成**：遵循 NRI 标准接口规范，确保与容器生态系统的兼容性

### 非目标

1. 不支持 containerd 版本低于 1.7.0 的环境
2. 不替代 Kubernetes 调度器的节点级调度决策
3. 不提供跨节点的资源调度能力

---

## 背景知识

### NRI (Node Resource Interface)

NRI 是由 containerd 项目定义的一套标准化接口，允许外部插件在容器生命周期的关键节点介入并调整容器配置。NRI 插件通过 Unix Socket 与容器运行时通信，可以：

- 订阅感兴趣的容器生命周期事件
- 在容器创建时修改容器配置（如 CPU 亲和性、内存限制等）
- 在容器停止/删除时执行清理操作

### 拓扑感知调度

现代服务器通常采用 NUMA (Non-Uniform Memory Access) 架构，具有以下层次结构：

```
System
└── Socket (物理 CPU 插槽)
    └── Die (芯片)
        └── NUMA Node (NUMA 节点)
            └── CPU Cores (CPU 核心)
```

拓扑感知调度确保容器的 CPU 和内存资源尽可能分配在同一 NUMA 节点或相近的拓扑位置，从而减少跨节点访问延迟。

---

## 设计详情

### 整体架构

```
┌─────────────────────────────────────────────────────────────────────┐
│                         Kubernetes Cluster                          │
│  ┌───────────────────────────────────────────────────────────────┐  │
│  │                        Worker Node                             │  │
│  │  ┌─────────────────────────────────────────────────────────┐  │  │
│  │  │                    containerd                            │  │  │
│  │  │  ┌─────────┐    ┌──────────────────────────────────────┐│  │  │
│  │  │  │ Runtime │◄──►│      NRI Plugin Interface            ││  │  │
│  │  │  └─────────┘    └──────────────┬───────────────────────┘│  │  │
│  │  └────────────────────────────────│────────────────────────┘  │  │
│  │                                   │ Unix Socket               │  │
│  │                                   │ (/var/run/nri/nri.sock)   │  │
│  │  ┌────────────────────────────────▼───────────────────────┐   │  │
│  │  │              Kunpeng-TAP NRI Plugin                    │   │  │
│  │  │  ┌──────────────────────────────────────────────────┐  │   │  │
│  │  │  │                  NRI Server                      │  │   │  │
│  │  │  │  • Configure()  • Synchronize()                  │  │   │  │
│  │  │  │  • RunPodSandbox()  • CreateContainer()          │  │   │  │
│  │  │  │  • StopContainer()  • RemoveContainer()          │  │   │  │
│  │  │  └──────────────────────────────────────────────────┘  │   │  │
│  │  │  ┌──────────────────────────────────────────────────┐  │   │  │
│  │  │  │               Policy Manager                     │  │   │  │
│  │  │  │  • TopologyAwarePolicy                           │  │   │  │
│  │  │  │  • NumaAwarePolicy                               │  │   │  │
│  │  │  └──────────────────────────────────────────────────┘  │   │  │
│  │  │  ┌──────────────────────────────────────────────────┐  │   │  │
│  │  │  │                  Cache                           │  │   │  │
│  │  │  │  • Pods Map  • Containers Map                    │  │   │  │
│  │  │  └──────────────────────────────────────────────────┘  │   │  │
│  │  └────────────────────────────────────────────────────────┘   │  │
│  └───────────────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────────┘
```

### 核心组件

#### 1. NRI Plugin (`pkg/kunpeng-tap/server/nri/server.go`)

NRI Plugin 是整个 NRI-Server 的核心，实现了 NRI 插件接口：

```go
type Plugin struct {
    cache       cache.Cache           // Pod/Container 缓存
    hookManager policy.HookManager    // 策略钩子管理器
    stub        stub.Stub             // NRI 存根接口
    mask        api.EventMask         // 订阅的事件掩码
    socketPath  string                // NRI Socket 路径
}
```

**关键配置**：
- `PluginName`: `"kunpeng-tap"` - NRI 插件名称
- `PluginIdx`: `"00"` - 插件优先级索引（数值越小优先级越高）
- `EventMask`: 订阅的事件类型

**订阅的 NRI 事件**：
```go
mask: api.MustParseEventMask(
    "RunPodSandbox,StopPodSandbox,RemovePodSandbox," +
    "CreateContainer,PostCreateContainer,StartContainer," +
    "StopContainer,UpdateContainer,RemoveContainer")
```

#### 2. Policy Manager (`pkg/kunpeng-tap/policy/`)

策略管理器负责容器资源分配决策，支持两种策略：

| 策略名称 | 描述 | 适用场景 |
|---------|------|---------|
| `topology-aware` | 基于完整硬件拓扑（Socket/Die/NUMA）的资源分配 | 复杂多路服务器环境 |
| `numa-aware` | 基于 NUMA 节点的资源分配 | 标准 NUMA 架构 |

**策略接口定义**：
```go
type Policy interface {
    Name() string
    Description() string
    PreRunPodSandboxHook(ctx HookContext) (*Allocation, error)
    PostStopPodSandboxHook(ctx HookContext) (*Allocation, error)
    PreCreateContainerHook(ctx HookContext) (*Allocation, error)
    PostStopContainerHook(ctx HookContext) (*Allocation, error)
    PreRemoveContainerHook(ctx HookContext) (*Allocation, error)
    SetCache(cache cache.Cache)
}
```

#### 3. Cache (`pkg/kunpeng-tap/cache/`)

缓存组件维护运行时 Pod 和容器的状态信息：

```go
type Cache interface {
    // Pod 操作
    InsertPod(id string, msg interface{}, status *PodStatus) (Pod, error)
    DeletePod(id string) Pod
    LookupPod(id string) (Pod, bool)

    // Container 操作
    InsertContainer(containerId string, msg interface{}) (Container, error)
    DeleteContainer(id string) Container
    LookupContainer(id string) (Container, bool)
}
```

#### 4. System Topology (`pkg/kunpeng-tap/sysfs/system/`)

系统拓扑发现模块，通过读取 `/sys` 文件系统获取硬件拓扑信息：

```go
type System interface {
    Discover() error                    // 发现系统拓扑
    PackageIDs() []ID                   // 获取所有 Socket ID
    NodeIDs() []ID                      // 获取所有 NUMA 节点 ID
    Package(id ID) CPUPackage           // 获取指定 Socket
    Node(id ID) Node                    // 获取指定 NUMA 节点
    NodeDistance(from, to ID) int       // 获取节点间距离
    MemoryInfo() (*MemInfo, error)      // 获取内存信息
}
```

### NRI 事件处理流程

#### Pod 生命周期

```
┌─────────────────┐     ┌──────────────────┐     ┌─────────────────┐
│  RunPodSandbox  │────►│   Insert Pod     │────►│  Cache Updated  │
│     Event       │     │   to Cache       │     │                 │
└─────────────────┘     └──────────────────┘     └─────────────────┘

┌─────────────────┐     ┌──────────────────┐     ┌─────────────────┐
│ StopPodSandbox  │────►│   Log Event      │────►│   (No Action)   │
│     Event       │     │                  │     │                 │
└─────────────────┘     └──────────────────┘     └─────────────────┘

┌─────────────────┐     ┌──────────────────┐     ┌─────────────────┐
│RemovePodSandbox │────►│  Delete Pod      │────►│  Cache Updated  │
│     Event       │     │  from Cache      │     │                 │
└─────────────────┘     └──────────────────┘     └─────────────────┘
```

#### Container 创建流程（关键路径）

```
┌───────────────────────────────────────────────────────────────────────────┐
│                         CreateContainer Event                              │
└───────────────────────────────────────┬───────────────────────────────────┘
                                        │
                                        ▼
┌───────────────────────────────────────────────────────────────────────────┐
│  1. 确保 Pod 在缓存中（必要时插入）                                          │
└───────────────────────────────────────┬───────────────────────────────────┘
                                        │
                                        ▼
┌───────────────────────────────────────────────────────────────────────────┐
│  2. 转换 NRI 请求为 Hook 请求格式                                           │
│     • PodMeta, ContainerMeta                                              │
│     • ContainerResources (CPU, Memory)                                    │
│     • Annotations, Labels, Environment Variables                          │
└───────────────────────────────────────┬───────────────────────────────────┘
                                        │
                                        ▼
┌───────────────────────────────────────────────────────────────────────────┐
│  3. 调用 PreCreateContainerHook                                            │
│     • Policy Manager 根据策略计算资源分配                                    │
│     • 返回 CPU/Memory 亲和性设置                                            │
└───────────────────────────────────────┬───────────────────────────────────┘
                                        │
                                        ▼
┌───────────────────────────────────────────────────────────────────────────┐
│  4. 转换 Hook 响应为 NRI ContainerAdjustment                               │
│     • SetLinuxCPUSetCPUs (cpuset.cpus)                                    │
│     • SetLinuxCPUSetMems (cpuset.mems)                                    │
│     • SetLinuxMemoryLimit                                                 │
└───────────────────────────────────────┬───────────────────────────────────┘
                                        │
                                        ▼
┌───────────────────────────────────────────────────────────────────────────┐
│  5. 将容器插入缓存，返回调整结果                                              │
└───────────────────────────────────────────────────────────────────────────┘
```

### 数据结构设计

#### ContainerResourceHookRequest

NRI 请求转换为内部 Hook 请求的数据结构：

```go
type ContainerResourceHookRequest struct {
    PodMeta              *PodSandboxMetadata
    ContainerMeta        *ContainerMetadata
    ContainerResources   *LinuxContainerResources
    ContainerAnnotations map[string]string
    ContainerEnvs        map[string]string
    PodCgroupParent      string
    PodAnnotations       map[string]string
    PodLabels            map[string]string
    PodResources         *LinuxContainerResources
}
```

#### LinuxContainerResources

容器资源配置：

```go
type LinuxContainerResources struct {
    CpuPeriod            int64               // CPU CFS 周期
    CpuQuota             int64               // CPU CFS 配额
    CpuShares            int64               // CPU 权重
    MemoryLimitInBytes   int64               // 内存限制
    OomScoreAdj          int64               // OOM 分数调整
    CpusetCpus           string              // CPU 亲和性 (如 "0-3,8-11")
    CpusetMems           string              // 内存节点亲和性 (如 "0,1")
    HugepageLimits       []*HugepageLimit    // 大页内存限制
    Unified              map[string]string   // cgroup v2 统一参数
}
```

### 接口定义

#### ProxyServer 接口

NRI Server 实现的服务器接口：

```go
type ProxyServer interface {
    Run() error                     // 启动服务器
    Shutdown(ctx context.Context)   // 优雅关闭
}
```

#### RuntimeHookService (gRPC)

策略管理器实现的 gRPC 服务接口：

```protobuf
service RuntimeHookService {
    rpc PreRunPodSandboxHook(PodSandboxHookRequest)
        returns (PodSandboxHookResponse);
    rpc PostStopPodSandboxHook(PodSandboxHookRequest)
        returns (PodSandboxHookResponse);
    rpc PreCreateContainerHook(ContainerResourceHookRequest)
        returns (ContainerResourceHookResponse);
    rpc PostStopContainerHook(ContainerResourceHookRequest)
        returns (ContainerResourceHookResponse);
    rpc PreRemoveContainerHook(ContainerResourceHookRequest)
        returns (ContainerResourceHookResponse);
}
```

---

## 部署方案

本节描述 Kunpeng-TAP NRI 插件的部署方式。NRI 插件以 DaemonSet 形式部署到 Kubernetes 集群的每个工作节点上，通过挂载主机的 NRI Socket 与 containerd 运行时进行通信。部署前需确保 containerd 已正确配置并启用 NRI 功能。

### 前置条件

1. **containerd 版本**：≥ v1.7.0
2. **containerd NRI 配置**：确保 NRI 已启用

在 containerd 配置文件 (`/etc/containerd/config.toml`) 中启用 NRI：

```toml
[plugins."io.containerd.nri.v1.nri"]
  disable = false
  disable_connections = false
  plugin_config_path = "/etc/nri/conf.d"
  plugin_path = "/opt/nri/plugins"
  plugin_registration_timeout = "5s"
  plugin_request_timeout = "2s"
  socket_path = "/var/run/nri/nri.sock"
```

### DaemonSet 部署

使用 Kubernetes DaemonSet 在每个工作节点上部署 NRI 插件：

```yaml
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: kunpeng-tap-nri
  namespace: kunpeng-tap
spec:
  selector:
    matchLabels:
      app: kunpeng-tap-nri
  template:
    spec:
      containers:
      - name: kunpeng-tap-nri
        image: kunpeng-tap-nri:latest
        args:
          - "--container-runtime-mode=NRI"
          - "--nri-socket-path=/var/run/nri/nri.sock"
          - "--resource-policy=topology-aware"
        volumeMounts:
        - name: nri-socket
          mountPath: /var/run/nri
        - name: sys
          mountPath: /host/sys
          readOnly: true
      volumes:
      - name: nri-socket
        hostPath:
          path: /var/run/nri
      - name: sys
        hostPath:
          path: /sys
```

### 部署命令

```bash
# 构建 NRI 容器镜像
make -f Makefile.kunpeng-tap nri-build-image

# 部署到集群
make -f Makefile.kunpeng-tap nri-deploy

# 检查部署状态
make -f Makefile.kunpeng-tap nri-status

# 查看日志
make -f Makefile.kunpeng-tap nri-logs
```

---

## 配置说明

本节描述 Kunpeng-TAP NRI 插件的配置方式。插件支持通过命令行参数和环境变量进行配置，用户可根据实际部署环境和需求灵活调整运行时模式、资源分配策略等参数。

### 命令行参数

| 参数 | 默认值 | 描述 |
|-----|--------|-----|
| `--container-runtime-mode` | `Docker` | 运行时模式：`Containerd`, `Docker`, `NRI` |
| `--nri-socket-path` | `/var/run/nri/nri.sock` | NRI Socket 路径 |
| `--resource-policy` | `topology-aware` | 资源策略：`topology-aware`, `numa-aware` |
| `--enable-memory-topology` | `false` | 启用内存拓扑感知 |
| `-v` | `0` | 日志详细级别 |

### 环境变量

| 变量 | 描述 |
|-----|------|
| `NRI_SOCKET_PATH` | NRI Socket 路径 |
| `RESOURCE_POLICY` | 资源分配策略 |
| `LOG_LEVEL` | 日志级别 |
| `CACHE_DIR` | 缓存目录路径 |

---

## 测试计划

本节描述 Kunpeng-TAP NRI 插件的测试策略。测试覆盖集成测试和端到端验证，确保插件在真实 Kubernetes 环境中能够正确处理容器生命周期事件并实施拓扑感知的资源分配。

### 集成测试

```bash
# 运行所有单元测试
make -f Makefile.kunpeng-tap test
```

同时，基于bdd-framework进行端到端测试。

### 验证测试用例

1. **基本功能验证**
   - 部署 NRI 插件后，新创建的 Pod 应获得正确的 CPU 亲和性设置
   - 检查容器的 `cpuset.cpus` 和 `cpuset.mems` cgroup 设置

2. **拓扑感知验证**
   - 在多 NUMA 节点系统上，验证容器资源分配在同一 NUMA 节点
   - 使用 `numactl --hardware` 查看系统拓扑
   - 使用 `cat /sys/fs/cgroup/.../cpuset.cpus` 验证分配结果

3. **生命周期验证**
   - 创建、停止、删除 Pod，验证缓存状态正确更新
   - 验证资源释放后可被新容器使用

---

## 参考资料

1. [NRI (Node Resource Interface) 规范](https://github.com/containerd/nri)
2. [containerd NRI 插件开发指南](https://github.com/containerd/nri/blob/main/README.md)
3. [Kubernetes NUMA 感知调度](https://kubernetes.io/docs/tasks/administer-cluster/numa-resources/)
4. [Linux cpuset cgroup 文档](https://www.kernel.org/doc/html/latest/admin-guide/cgroup-v2.html)

