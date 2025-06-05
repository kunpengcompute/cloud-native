# K8S MPAM Controller 编译和部署指南

## 概述

K8S MPAM Controller 是一个Kubernetes内存分区和监控控制器，提供内存资源的精细化管理，支持MPAM（Memory Partitioning and Monitoring）技术。

## 项目结构

```
├── cmd/k8s-mpam-controller/
│   └── main.go               # 主程序入口
├── pkg/k8s-mpam-controller/
│   ├── agent/                # 代理组件
│   ├── collector/            # 数据收集器
│   ├── infomer/              # 信息通知器
│   ├── typedef/              # 类型定义
│   ├── dynamic/              # 动态配置
│   └── util/                 # 工具函数
└── config/                   # 配置文件
```

## 编译

### 前置条件

- Go 1.23.6 或更高版本
- Linux 环境（推荐 Ubuntu 18.04+）
- 支持MPAM的硬件平台（推荐鲲鹏处理器）

### 使用 Makefile 编译

```bash
# 查看所有可用命令
make -f Makefile.mpam help

# 编译项目（安装到GOPATH/bin）
make -f Makefile.mpam build

# 编译到本地bin目录
make -f Makefile.mpam build-local

# 清理编译产物
make -f Makefile.mpam clean
```

### 手动编译

```bash
# 编译到GOPATH/bin
go install ./cmd/k8s-mpam-controller

# 编译到本地目录
go build -o bin/k8s-mpam-controller ./cmd/k8s-mpam-controller
```

## 开发

### 代码格式化和检查

```bash
# 格式化代码
make -f Makefile.mpam fmt

# 代码静态检查
make -f Makefile.mpam vet

# 整理依赖
make -f Makefile.mpam tidy
```

### 运行测试

```bash
# 运行单元测试
make -f Makefile.mpam test
```

### 本地运行

```bash
# 运行应用程序
make -f Makefile.mpam run

# 或直接运行二进制文件
./bin/k8s-mpam-controller [options]
```

## 部署

### 系统安装

```bash
# 安装到系统
sudo make -f Makefile.mpam install

# 卸载
sudo make -f Makefile.mpam uninstall
```

### 容器化部署

```bash
# 构建Docker镜像
make -f Makefile.mpam docker-build

# 推送镜像
make -f Makefile.mpam docker-push

# 运行容器
make -f Makefile.mpam docker-run
```

## 配置

### 命令行参数

- `-ca-file string`: 根证书文件的绝对路径
- `-cert-file string`: 证书文件的绝对路径
- `-key-file string`: 私钥文件的绝对路径
- `-cn string`: 证书的通用名称(CN)
- `-direct`: 直接模式，默认false。如果为true，不依赖服务器中继pod信息
- `-kubeconfig string`: kubeconfig文件的绝对路径（可选，如果在集群中运行）
- `-server string`: 中继服务器地址

### 配置文件

项目支持通过配置文件进行配置，配置文件位于`config/`目录下。

## 功能特性

### MPAM支持
- 内存分区管理
- 内存带宽监控
- 内存访问控制
- 性能隔离

### Kubernetes集成
- 自定义资源定义(CRD)
- 控制器模式
- 事件监听
- 状态同步

## 故障排除

### 常见问题

1. **编译失败**
   - 确保Go版本 >= 1.23.6
   - 运行 `make -f Makefile.mpam tidy` 更新依赖

2. **运行时错误**
   - 检查是否有足够的权限
   - 确认硬件支持MPAM功能
   - 验证Kubernetes集群连接

3. **权限问题**
   - 确保以适当权限运行
   - 检查证书和密钥文件权限

### 日志查看

```bash
# 查看应用程序日志
./bin/k8s-mpam-controller -v=2

# 如果作为系统服务运行
sudo journalctl -u k8s-mpam-controller -f
```

## 版本信息

```bash
# 查看版本信息
./bin/k8s-mpam-controller --version

# 查看帮助信息
./bin/k8s-mpam-controller --help
```

## 开发指南

### 代码结构

- `cmd/k8s-mpam-controller/`: 主程序入口
- `pkg/k8s-mpam-controller/agent/`: 核心代理逻辑
- `pkg/k8s-mpam-controller/collector/`: 数据收集组件
- `pkg/k8s-mpam-controller/util/`: 通用工具函数

### 添加新功能

1. 在相应的pkg子目录下添加代码
2. 更新相关的测试
3. 运行 `make -f Makefile.mpam check` 验证
4. 更新文档

## 贡献

欢迎提交Issue和Pull Request来改进K8S MPAM Controller。

## 许可证

本项目采用Apache License 2.0许可证。 