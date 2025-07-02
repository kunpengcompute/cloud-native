# Kunpeng-TAP 编译和部署指南

## 概述

Kunpeng-TAP 是一个为鲲鹏处理器优化的容器拓扑感知调度组件，支持 NUMA 感知和拓扑感知的容器资源分配策略。

## 项目结构

```
├── cmd/kunpeng-tap/
│   ├── manager/          # TAP 管理器
│   └── proxy/            # TAP 代理
├── pkg/kunpeng-tap/
│   ├── cache/            # 缓存管理
│   ├── policy/           # 调度策略
│   ├── server/           # 服务器实现
│   ├── monitoring/       # 监控指标
│   └── version/          # 版本信息
├── api/kunpeng-tap/      # API 定义
├── test/kunpeng-tap/     # 测试代码
└── hack/kunpeng-tap/     # 部署脚本
```

## 编译

### 前置条件

- Go 1.23.6 或更高版本
- Linux 环境（推荐 Ubuntu 18.04+）

### 使用 Makefile 编译

```bash
# 查看所有可用命令
make -f Makefile.kunpeng-tap help

# 编译所有组件
make -f Makefile.kunpeng-tap build

# 仅编译管理器
make -f Makefile.kunpeng-tap build-manager

# 仅编译代理
make -f Makefile.kunpeng-tap build-proxy

# 清理编译产物
make -f Makefile.kunpeng-tap clean
```

### 手动编译

```bash
# 编译管理器
go build -o bin/kunpeng-tap-manager ./cmd/kunpeng-tap/manager

# 编译代理
go build -o bin/kunpeng-tap-proxy ./cmd/kunpeng-tap/proxy
```

## 开发

### 代码格式化和检查

```bash
# 格式化代码
make -f Makefile.kunpeng-tap fmt

# 代码静态检查
make -f Makefile.kunpeng-tap vet

# 整理依赖
make -f Makefile.kunpeng-tap tidy
```

### 运行测试

```bash
# 运行单元测试
make -f Makefile.kunpeng-tap test

# 运行端到端测试
make -f Makefile.kunpeng-tap test-e2e
```

### 本地运行

```bash
# 运行管理器
make -f Makefile.kunpeng-tap run-manager

# 运行代理
make -f Makefile.kunpeng-tap run-proxy
```

## 部署

### 系统服务部署

#### Docker 运行时环境

```bash
# 安装服务（Docker 运行时）
sudo make -f Makefile.kunpeng-tap install-service-docker

# 启动服务
sudo make -f Makefile.kunpeng-tap start-service

# 查看服务状态
sudo make -f Makefile.kunpeng-tap status-service
```

#### Containerd 运行时环境

```bash
# 安装服务（Containerd 运行时）
sudo make -f Makefile.kunpeng-tap install-service-containerd

# 启动服务
sudo make -f Makefile.kunpeng-tap start-service
```

### 服务管理

```bash
# 启动服务
sudo make -f Makefile.kunpeng-tap start-service

# 停止服务
sudo make -f Makefile.kunpeng-tap stop-service

# 重启服务
sudo make -f Makefile.kunpeng-tap restart-service

# 查看服务状态
sudo make -f Makefile.kunpeng-tap status-service

# 卸载服务
sudo make -f Makefile.kunpeng-tap uninstall-service
```

## 容器化部署

### 构建 Docker 镜像

```bash
# 构建镜像
make -f Makefile.kunpeng-tap docker-build

# 推送镜像
make -f Makefile.kunpeng-tap docker-push

# 多架构构建
make -f Makefile.kunpeng-tap docker-buildx
```

## 配置

### 代理配置选项

- `--runtime-proxy-endpoint`: 运行时代理端点（默认：/var/run/kunpeng-tap/runtimeproxy.sock）
- `--container-runtime-service-endpoint`: 容器运行时服务端点
- `--container-runtime-mode`: 容器运行时模式（Docker|Containerd）
- `--resource-policy`: 资源策略（numa-aware|topology-aware）
- `--enable-memory-topology`: 启用内存拓扑感知

### 管理器配置选项

- `--policy-manager-endpoint`: 策略管理器端点
- `--container-runtime-service-endpoint`: 容器运行时服务端点
- `--resource-policy`: 资源策略

## 监控

Kunpeng-TAP 提供 Prometheus 指标，默认在 `:9091/metrics` 端点暴露。

## 故障排除

### 常见问题

1. **编译失败**
   - 确保 Go 版本 >= 1.23.6
   - 运行 `make -f Makefile.kunpeng-tap tidy` 更新依赖

2. **服务启动失败**
   - 检查容器运行时是否正常运行
   - 确保有足够的权限访问容器运行时套接字

3. **权限问题**
   - 确保以 root 权限运行服务安装命令
   - 检查 systemd 服务文件权限

### 日志查看

```bash
# 查看服务日志
sudo journalctl -u kunpeng-tap.service -f

# 查看服务状态
sudo systemctl status kunpeng-tap.service
```

## 版本信息

```bash
# 查看版本信息
./bin/kunpeng-tap --version
./bin/kunpeng-tap-manager --version
```

## 贡献

欢迎提交 Issue 和 Pull Request 来改进 Kunpeng-TAP。 