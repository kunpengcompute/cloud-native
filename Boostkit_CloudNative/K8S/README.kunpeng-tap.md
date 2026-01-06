# Kunpeng-TAP 部署指南

## 📖 概述

Kunpeng-TAP 是一个为鲲鹏处理器优化的容器拓扑感知调度组件，支持 NUMA 感知和拓扑感知的容器资源分配策略。

**核心特性：**
- 🎯 NUMA 感知的容器资源分配
- 🔄 拓扑感知的调度策略
- ⚡ GPU 优先/CPU 优先资源分配策略
- 🔌 支持多种部署模式（NRI/Docker/Containerd）

**选择适合您的部署方式：**
- **☁️ Kubernetes 集群** → NRI 容器化部署（推荐）
- **🖥️ 传统 Linux 环境** → 系统服务部署
- **📦 标准化部署** → RPM 包部署

## 📋 环境要求

### 基础要求
- **操作系统**: Linux（推荐 OpenEuler 22.03+）
- **Go 版本**: 1.25.0 或更高版本（用于编译）

### 部署模式要求

| 部署模式 | 容器运行时 | Kubernetes | 其他要求 |
|---------|-----------|-----------|---------|
| NRI 容器化部署 | containerd >= 1.7.0 | >= 1.23 | 启用 NRI 支持 |
| 系统服务部署 | Docker 或 Containerd | 不需要 | systemd 权限 |
| RPM 包部署 | Docker 或 Containerd | 不需要 | RPM 包管理器 |

## 🚀 快速开始

### 方式一：NRI 容器化部署（Kubernetes 集群）

**适用场景：** Kubernetes 集群环境，推荐用于云原生应用

```bash
# 1. 环境检查
containerd --version  # 确认版本 >= 1.7.0

# 2. 构建镜像
make -f Makefile.kunpeng-tap nri-build-image

# 3. 部署到集群
make -f Makefile.kunpeng-tap nri-deploy

# 4. 验证部署
make -f Makefile.kunpeng-tap nri-status
```

### 方式二：系统服务部署（传统 Linux 环境）

**适用场景：** 传统 Linux 环境，直接在宿主机上运行

```bash
# 1. 编译项目
make -f Makefile.kunpeng-tap build

# 2. 安装服务（根据容器运行时选择）
sudo make -f Makefile.kunpeng-tap install-service-docker      # Docker 环境
# 或
sudo make -f Makefile.kunpeng-tap install-service-containerd  # Containerd 环境

# 3. 启动服务
sudo make -f Makefile.kunpeng-tap start-service

# 4. 验证服务
sudo make -f Makefile.kunpeng-tap status-service
```

### 方式三：RPM 包部署

**适用场景：** 需要标准化部署和版本管理的环境

详细安装指南请参考：**[📖 RPM 部署指南](./hack/kunpeng-tap/README.md)**

## 🏗️ 详细部署指南

### NRI 容器化部署

#### 1. 环境准备

```bash
# 检查容器运行时版本
containerd --version  # 确保 >= 1.7.0

# 检查 NRI 支持
ls -la /var/run/nri/    # 确认 NRI socket 目录存在
```

#### 2. 镜像构建

```bash
# 构建 NRI 专用镜像
make -f Makefile.kunpeng-tap nri-build-image

# 自定义镜像标签（可选）
make -f Makefile.kunpeng-tap nri-build-image NRI_IMG=my-registry/kunpeng-tap-nri:v1.0.0
```

#### 3. 配置和部署

NRI 插件通过 DaemonSet 配置文件进行部署和配置。

**配置文件位置：** `config/kunpeng-tap/nri-plugin/daemonset.yaml`

**关键配置项：**

```yaml
# 容器启动参数配置（默认配置）
args:
  - "--container-runtime-mode=NRI"              # 运行时模式：NRI
  - "--nri-socket-path=/var/run/nri/nri.sock"  # NRI socket 路径
  - "--resource-policy=topology-aware"          # 资源策略
  - "-v=2"                                      # 日志级别

# 资源限制配置
resources:
  requests:
    cpu: 100m
    memory: 128Mi
  limits:
    cpu: 500m
    memory: 512Mi
```

**部署到集群：**

```bash
# 部署 NRI 插件
make -f Makefile.kunpeng-tap nri-deploy

# 验证部署状态
make -f Makefile.kunpeng-tap nri-status
```

**自定义配置：**

如需修改配置（如启用 GPU 优先策略或内存拓扑感知），编辑 `config/kunpeng-tap/nri-plugin/daemonset.yaml` 文件中的 `args` 部分：

```yaml
# 示例：启用 GPU 优先策略和内存拓扑感知
containers:
- name: kunpeng-tap-nri
  image: kunpeng-tap-nri:latest
  args:
    - "--container-runtime-mode=NRI"
    - "--nri-socket-path=/var/run/nri/nri.sock"
    - "--resource-policy=topology-aware"
    - "--resource-priority=gpu-first"           # 添加：GPU 优先策略
    - "--enable-memory-topology=true"           # 添加：启用内存拓扑感知
    - "-v=2"
```

修改后重新部署使配置生效：

```bash
make -f Makefile.kunpeng-tap nri-restart
```

#### 4. 运维管理

```bash
# 查看插件状态
make -f Makefile.kunpeng-tap nri-status

# 查看运行日志
make -f Makefile.kunpeng-tap nri-logs

# 实时跟踪日志
make -f Makefile.kunpeng-tap nri-logs-follow

# 重启插件（应用配置更改后）
make -f Makefile.kunpeng-tap nri-restart

# 卸载插件
make -f Makefile.kunpeng-tap nri-undeploy
```

### 系统服务部署

#### 1. 编译项目

```bash
# 编译所有组件
make -f Makefile.kunpeng-tap build

# 清理旧版本
make -f Makefile.kunpeng-tap clean
```

#### 2. 服务安装

```bash
# Docker 环境
sudo make -f Makefile.kunpeng-tap install-service-docker

# Containerd 环境
sudo make -f Makefile.kunpeng-tap install-service-containerd
```

#### 3. 服务管理

```bash
# 启动服务
sudo make -f Makefile.kunpeng-tap start-service

# 查看状态
sudo make -f Makefile.kunpeng-tap status-service

# 停止服务
sudo make -f Makefile.kunpeng-tap stop-service

# 卸载服务
sudo make -f Makefile.kunpeng-tap uninstall-service
```

## ⚙️ 配置选项

### 核心参数

| 参数 | 描述 | 默认值 | 可选值 |
|------|------|--------|--------|
| `--runtime-proxy-endpoint` | 运行时代理端点 | `/var/run/kunpeng-tap/runtimeproxy.sock` | 自定义路径 |
| `--container-runtime-service-endpoint` | 容器运行时服务端点 | - | Docker/Containerd socket 路径 |
| `--container-runtime-mode` | 容器运行时模式 | - | `Docker`/`Containerd`/`NRI` |
| `--nri-socket-path` | NRI socket 路径 | `/var/run/nri/nri.sock` | 自定义路径（仅 NRI 模式） |
| `--resource-policy` | 资源分配策略 | `topology-aware` | `numa-aware`/`topology-aware` |
| `--resource-priority` | 资源优先级策略 | `cpu-first` | `cpu-first`/`gpu-first` |
| `--enable-memory-topology` | 启用内存拓扑感知 | `false` | `true`/`false` |
| `--metrics-bind-address` | Prometheus 指标端点 | `:9091` | 自定义地址 |

### 配置示例

#### NRI 模式配置

NRI 模式通过 DaemonSet 配置文件进行配置，编辑 `config/kunpeng-tap/nri-plugin/daemonset.yaml`：

```yaml
# 基础配置（默认）
spec:
  template:
    spec:
      containers:
      - name: kunpeng-tap-nri
        args:
          - "--container-runtime-mode=NRI"
          - "--nri-socket-path=/var/run/nri/nri.sock"
          - "--resource-policy=topology-aware"
          - "-v=2"

# GPU 优先配置
spec:
  template:
    spec:
      containers:
      - name: kunpeng-tap-nri
        args:
          - "--container-runtime-mode=NRI"
          - "--nri-socket-path=/var/run/nri/nri.sock"
          - "--resource-policy=topology-aware"
          - "--resource-priority=gpu-first"
          - "--enable-memory-topology=true"
          - "-v=2"
```

修改配置后，使用 `make -f Makefile.kunpeng-tap nri-restart` 重启插件使配置生效。

#### Docker 模式配置
```bash
./kunpeng-tap \
  --container-runtime-mode=Docker \
  --container-runtime-service-endpoint=/var/run/docker.sock \
  --resource-policy=topology-aware \
  --resource-priority=cpu-first
```

#### Containerd 模式配置
```bash
./kunpeng-tap \
  --container-runtime-mode=Containerd \
  --container-runtime-service-endpoint=/run/containerd/containerd.sock \
  --resource-policy=topology-aware \
  --enable-memory-topology=true
```

### 资源优先级策略说明

Kunpeng-TAP 的 topology-aware 策略支持配置资源分配优先级（`--resource-priority`），允许用户根据工作负载特性优化容器放置。

#### CPU 优先策略（默认）

**适用场景：** CPU 密集型工作负载、通用应用程序、需要向后兼容的环境

**策略行为：** 优先考虑 CPU 资源的可用性和容量，对于 GPU 工作负载，先选择 CPU 容量最佳的 NUMA 节点，再考虑 GPU 亲和性

```bash
./kunpeng-tap --resource-policy=topology-aware --resource-priority=cpu-first
```

#### GPU 优先策略

**适用场景：** GPU 密集型应用、机器学习训练任务、对 GPU-CPU 数据传输延迟敏感的应用

**策略行为：** 优先考虑 GPU 设备的 NUMA 亲和性，优先选择 GPU 设备所在的 NUMA 节点，确保最佳的 GPU-CPU 数据传输性能

```bash
./kunpeng-tap --resource-policy=topology-aware --resource-priority=gpu-first
```

**GPU 亲和性计算：**
1. 检测容器环境变量中的 GPU 设备请求（`VA_VISIBLE_DEVICES` 等）
2. 查找请求的 GPU 设备在系统中的 NUMA 节点位置
3. 计算每个候选 NUMA 节点与请求 GPU 设备的亲和性权重
4. 亲和性权重越高，表示 GPU-CPU 数据传输性能越好

## 🔧 运维管理

### 日志查看

#### NRI 模式
```bash
# 查看 NRI 插件日志
make -f Makefile.kunpeng-tap nri-logs

# 查看 Pod 状态
kubectl get pods -n kunpeng-tap -l app=kunpeng-tap-nri
```

#### 系统服务模式
```bash
# 查看服务日志
sudo journalctl -u kunpeng-tap.service -f

# 查看服务状态
sudo systemctl status kunpeng-tap.service
```

#### 通用命令
```bash
# 查看版本信息
./bin/kunpeng-tap --version
```

### 服务更新

#### NRI 模式
```bash
# 更新 NRI 插件
make -f Makefile.kunpeng-tap nri-restart
```

#### 系统服务模式
```bash
# 重启系统服务
sudo systemctl restart kunpeng-tap.service
```

### 监控指标

Kunpeng-TAP 提供 Prometheus 指标，默认在 `:9091/metrics` 端点暴露。

```bash
# 访问指标端点
curl http://localhost:9091/metrics
```

## 🚨 故障排除

### 常见问题

#### 1. 编译失败

**症状：** 编译过程中出现错误

**解决方案：**
```bash
# 检查 Go 版本
go version    # 应该 >= 1.25.0

# 清理并重新编译
make -f Makefile.kunpeng-tap clean
make -f Makefile.kunpeng-tap tidy
make -f Makefile.kunpeng-tap build
```

#### 2. NRI 插件启动失败

**症状：** NRI 插件无法启动或频繁重启

**解决方案：**
```bash
# 检查 containerd 版本
containerd --version    # 应该 >= 1.7.0

# 检查 NRI 环境
ls -la /var/run/nri/    # socket 目录应该存在

# 查看详细日志
make -f Makefile.kunpeng-tap nri-logs

# 检查节点标签
kubectl get nodes --show-labels
```

#### 3. 系统服务启动失败

**症状：** systemd 服务无法启动

**解决方案：**
```bash
# 检查容器运行时状态
sudo systemctl status docker       # 或 containerd
sudo systemctl status containerd

# 检查权限
ls -la /var/run/docker.sock        # 或 /run/containerd/containerd.sock

# 查看详细日志
sudo journalctl -u kunpeng-tap.service -xe
```

#### 4. 权限问题

**症状：** 无法访问容器运行时套接字

**解决方案：**
```bash
# 确保以 root 权限运行
sudo make -f Makefile.kunpeng-tap install-service-docker

# 检查 systemd 服务文件权限
ls -la /etc/systemd/system/kunpeng-tap.service
```

### 诊断工具

```bash
# 环境检查
make -f Makefile.kunpeng-tap nri-status

# 日志诊断
make -f Makefile.kunpeng-tap nri-logs

# 服务重启
make -f Makefile.kunpeng-tap nri-restart
```

## 📚 扩展资源

- **RPM 部署指南**: [📖 详细 RPM 部署文档](./hack/kunpeng-tap/README.md)
- **问题反馈**: 提交 Issue 或 Pull Request
- **技术支持**: 查看项目文档或联系维护团队
