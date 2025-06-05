# 构建系统说明

## 概述

本项目采用三层Makefile结构，实现了统一管理和独立构建的目标：

```
kunpeng-cloud-computing/
├── Makefile                           # 第一层：顶层统一管理
└── Boostkit_CloudNative/K8S/
    ├── Makefile                       # 第二层：K8S项目协调
    ├── Makefile.mpam                  # 第三层：MPAM专用
    └── Makefile.kunpeng-tap           # 第三层：Kunpeng TAP专用
```

## 构建系统特点

### 1. 层次化管理
- **顶层Makefile**: 提供项目级别的统一入口
- **协调Makefile**: 管理相关子项目的组合操作
- **专用Makefile**: 每个子项目的独立构建逻辑

### 2. 命令一致性
所有层级都提供一致的命令接口：
- `build`: 构建项目
- `clean`: 清理构建产物
- `test`: 运行测试
- `fmt`: 代码格式化
- `vet`: 静态检查
- `help`: 显示帮助

### 3. 项目隔离
每个子项目有独立的：
- 构建配置
- 依赖管理
- 输出目录
- 版本控制

## 使用方法

### 从顶层构建所有项目
```bash
cd kunpeng-cloud-computing
make build                    # 构建所有项目
make clean                    # 清理所有项目
make test                     # 测试所有项目
```

### 从K8S层构建相关项目
```bash
cd Boostkit_CloudNative/K8S
make build                    # 构建K8S相关项目
make mpam-build              # 仅构建MPAM
make kunpeng-tap-build       # 仅构建Kunpeng TAP
make kunpeng-tap-build-manager  # 仅构建TAP管理器
make kunpeng-tap-build-proxy    # 仅构建TAP代理
```

### 直接构建特定项目
```bash
cd Boostkit_CloudNative/K8S

# 构建MPAM Controller
make -f Makefile.mpam build

# 构建Kunpeng TAP
make -f Makefile.kunpeng-tap build
make -f Makefile.kunpeng-tap build-manager  # 仅构建管理器
make -f Makefile.kunpeng-tap build-proxy    # 仅构建代理
```

## 项目构建详情

### K8S MPAM Controller
- **源码位置**: `cmd/k8s-mpam-controller/`, `pkg/k8s-mpam-controller/`
- **构建产物**: 
  - `$GOPATH/bin/k8s-mpam-controller`
  - `bin/k8s-mpam-controller` (本地构建)
- **Docker镜像**: `k8s-mpam-controller:0.1.0`

### Kunpeng TAP
- **源码位置**: `cmd/kunpeng-tap/`, `pkg/kunpeng-tap/`
- **构建产物**: 
  - `bin/kunpeng-tap-manager`
  - `bin/kunpeng-tap-proxy`
- **Docker镜像**: `kunpeng-tap:latest`

## 命令映射表

| 顶层命令 | K8S层命令 | 专用命令 | 说明 |
|---------|-----------|----------|------|
| `make mpam-build` | `make mpam-build` | `make -f Makefile.mpam build` | 构建MPAM |
| `make kunpeng-tap-build` | `make kunpeng-tap-build` | `make -f Makefile.kunpeng-tap build` | 构建TAP |
| `make kunpeng-tap-build-manager` | `make kunpeng-tap-build-manager` | `make -f Makefile.kunpeng-tap build-manager` | 构建TAP管理器 |
| `make kunpeng-tap-build-proxy` | `make kunpeng-tap-build-proxy` | `make -f Makefile.kunpeng-tap build-proxy` | 构建TAP代理 |
| `make build` | `make build` | - | 构建所有项目 |
| `make clean` | `make clean` | - | 清理所有项目 |

## 扩展指南

### 添加新项目
1. 在适当目录创建项目代码
2. 创建项目专用Makefile（如`Makefile.newproject`）
3. 在上层Makefile中添加对应命令：
   ```makefile
   .PHONY: newproject-build
   newproject-build: ## Build new project.
   	$(MAKE) -f Makefile.newproject build
   ```
4. 更新组合命令（如`build`目标）

### 自定义构建选项
每个专用Makefile都支持环境变量配置：
```bash
# 自定义版本
VERSION=1.0.0 make -f Makefile.kunpeng-tap build

# 自定义镜像名
IMG=my-registry/kunpeng-tap:latest make -f Makefile.kunpeng-tap docker-build
```

## 故障排除

### 常见问题

1. **make命令被其他程序覆盖**
   ```bash
   # 使用完整路径
   /usr/bin/make build
   ```

2. **构建失败**
   ```bash
   # 清理后重新构建
   make clean && make build
   ```

3. **依赖问题**
   ```bash
   # 更新依赖
   make tidy
   ```

### 调试技巧

1. **查看详细构建过程**
   ```bash
   make -f Makefile.kunpeng-tap build VERBOSE=1
   ```

2. **检查生成的文件**
   ```bash
   ls -la bin/
   ls -la $GOPATH/bin/
   ```

3. **验证构建产物**
   ```bash
   ./bin/kunpeng-tap-proxy --version
   $GOPATH/bin/k8s-mpam-controller --help
   ```

## 最佳实践

1. **开发时使用项目专用Makefile**，获得最快的构建速度
2. **CI/CD使用顶层Makefile**，确保完整的项目构建
3. **定期运行`make ci`**，保证代码质量
4. **使用`make help`**查看可用命令
5. **构建前运行`make tidy`**，确保依赖最新