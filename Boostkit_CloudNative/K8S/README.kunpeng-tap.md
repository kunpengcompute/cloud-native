# Kunpeng-TAP 部署指南

## 📖 概述

Kunpeng-TAP 是一个为鲲鹏处理器优化的容器拓扑感知调度组件，支持 NUMA 感知和拓扑感知的容器资源分配策略。

### 🚀 快速开始

**选择适合您的部署方式：**

- **🏢 生产环境** → RPM 包部署（传统系统服务）
- **☁️ 容器化环境** → NRI 容器化部署（推荐）

---

## 🎯 Quick Start

### 方式一：NRI 容器化部署（推荐）

- Linux 操作系统（推荐 OpenEuler 22.03及更高版本）
- 仅适用于containerd >= v1.7.0
- Go 1.25.0 或更高版本（用于编译）
- containerd 开启 NRI 支持

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

### 方式二：系统服务部署

**适用于传统 Linux 环境**

#### 环境要求
- Linux 操作系统（推荐 OpenEuler 22.03及更高版本）
- 已安装的容器运行时（Docker 或 Containerd）
- Go 1.25.0 或更高版本（用于编译）
- systemd 权限（用于安装系统服务）

```bash
# 1. 环境检查
docker --version  # 或 containerd --version
go version        # 确认编译环境

# 2. 编译项目
make -f Makefile.kunpeng-tap build

# 3. 安装服务（根据容器运行时选择）
sudo make -f Makefile.kunpeng-tap install-service-docker
# 或
sudo make -f Makefile.kunpeng-tap install-service-containerd

# 4. 启动服务
sudo make -f Makefile.kunpeng-tap start-service

# 5. 验证服务
sudo make -f Makefile.kunpeng-tap status-service
```

---

## 📋 环境要求

### 基础要求
- **操作系统**: Linux（推荐 Ubuntu 18.04+）
- **Go 版本**: 1.23.6 或更高版本

### NRI 部署额外要求
- **Kubernetes**: 版本 >= 1.24
- **containerd**: 版本 >= v1.7.0
- **NRI 支持**: 已启用 NRI 的容器运行时环境

---

## 🏗️ 详细部署指南

### 方式一：NRI 容器化部署（Kubernetes）

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

#### 3. 集群部署

```bash
# 部署到 Kubernetes 集群
make -f Makefile.kunpeng-tap nri-deploy

# 验证部署状态
make -f Makefile.kunpeng-tap nri-status
```

#### 4. 运维管理

```bash
# 查看插件状态
make -f Makefile.kunpeng-tap nri-status

# 查看运行日志
make -f Makefile.kunpeng-tap nri-logs

# 重启插件（重新部署）
make -f Makefile.kunpeng-tap nri-restart

# 卸载插件
make -f Makefile.kunpeng-tap nri-undeploy
```

### 方式二：系统服务部署（传统环境）

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

### 方式三：RPM 包部署（生产环境）

**推荐用于生产环境的标准化部署**

详细安装指南请参考：**[📖 RPM 部署指南](./hack/kunpeng-tap/README.md)**

---

## ⚙️ 配置选项

### 核心参数

| 参数 | 描述 | 默认值 | 适用场景 |
|------|------|--------|----------|
| `--container-runtime-mode` | 运行时模式 | - | `Docker/Containerd/NRI` |
| `--resource-policy` | 资源策略 | `topology-aware` | `numa-aware/topology-aware` |

## 🔧 运维管理

### 查看运行状态

```bash
# NRI 模式状态检查
make -f Makefile.kunpeng-tap nri-status

# 系统服务状态检查
sudo systemctl status kunpeng-tap.service
```

### 日志管理

```bash
# NRI 插件日志（推荐）
make -f Makefile.kunpeng-tap nri-logs

# 系统服务日志
sudo journalctl -u kunpeng-tap.service -f
```

### 更新与维护

```bash
# 更新 NRI 插件
make -f Makefile.kunpeng-tap nri-restart

# 重启系统服务
sudo systemctl restart kunpeng-tap.service
```

---

## 🚨 故障排除

### 常见问题诊断

#### 1. NRI 插件启动失败
```bash
# 检查版本要求
containerd --version    # 应该 >= 1.7.0

# 检查 NRI 环境
ls -la /var/run/nri/    # socket 目录应该存在

# 检查节点类型
kubectl get nodes --show-labels
```

#### 2. 服务权限问题
```bash
# 检查系统权限
sudo systemctl status containerd

# 查看详细日志
sudo journalctl -u containerd | grep -i nri
```

#### 3. 编译问题
```bash
# 确认环境
go version    # 应该 >= 1.23.6

# 清理重建
make -f Makefile.kunpeng-tap clean
make -f Makefile.kunpeng-tap tidy
make -f Makefile.kunpeng-tap build
```

### 获取帮助

- **环境检查**: `make -f Makefile.kunpeng-tap nri-status`
- **日志诊断**: `make -f Makefile.kunpeng-tap nri-logs`
- **服务重启**: `make -f Makefile.kunpeng-tap nri-restart`

---

## 📚 扩展信息

- **详细 RPM 部署**: [📖 RPM 部署指南](./hack/kunpeng-tap/README.md)
- **问题反馈**: 提交 Issue 或 Pull Request 