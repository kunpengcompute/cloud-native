# Kunpeng Cloud Computing

鲲鹏云计算项目集合，包含多个针对鲲鹏处理器优化的云原生组件。

## 项目结构

```
kunpeng-cloud-computing/
├── Makefile                           # 顶层Makefile，统一管理所有项目
├── README.md                          # 项目总体说明
├── LICENSE                            # 许可证
└── Boostkit_CloudNative/K8S/          # K8S相关项目
    ├── Makefile                       # K8S项目协调Makefile
    ├── Makefile.mpam                  # K8S MPAM Controller专用Makefile
    ├── Makefile.kunpeng-tap           # Kunpeng TAP专用Makefile
    ├── cmd/                           # 命令行工具
    │   ├── k8s-mpam-controller/       # MPAM Controller
    │   └── kunpeng-tap/               # Kunpeng TAP组件
    ├── pkg/                           # 包代码
    │   ├── k8s-mpam-controller/       # MPAM Controller包
    │   └── kunpeng-tap/               # Kunpeng TAP包
    ├── api/                           # API定义
    ├── config/                        # 配置文件
    ├── hack/                          # 部署脚本
    └── test/                          # 测试代码
```

## 子项目

### K8S MPAM Controller
鲲鹏MPAM Qos插件，支持把离线业务加入到一个MPAM的动态控制组中，动态调整离线业务可用资源，降低对在线业务的干扰率。

**详细文档**: [README.mpam.md](Boostkit_CloudNative/K8S/README.mpam.md)

### Kunpeng TAP (Topology Aware Plugin)
鲲鹏拓扑感知调度组件，支持NUMA感知和拓扑感知的容器资源分配。

**详细文档**: [README.kunpeng-tap.md](Boostkit_CloudNative/K8S/README.kunpeng-tap.md)

## 环境要求与构建

### 环境要求
- Go 1.23.6 或更高版本
- Docker（推荐20.10.14，用于容器镜像构建）
- Linux 环境（推荐OpenEuler 20.03、22.03和24.03）
- 硬件：Kunpeng 920系列 ARM64 服务器
- 系统管理员权限（用于服务安装）

### 快速构建

#### 查看帮助
```bash
# 顶层帮助
make help

# K8S项目帮助
make -C Boostkit_CloudNative/K8S help

# 特定项目帮助
make -C Boostkit_CloudNative/K8S -f Makefile.mpam help
make -C Boostkit_CloudNative/K8S -f Makefile.kunpeng-tap help
```

#### 构建所有项目
```bash
# 从顶层构建所有项目
make build

# 或者从K8S目录构建
make -C Boostkit_CloudNative/K8S build
```

#### 构建特定项目
```bash
# 构建MPAM Controller
make mpam-build

# 构建Kunpeng TAP
make kunpeng-tap-build
```

#### 构建产物
- **MPAM Controller**: 
  - `$GOPATH/bin/k8s-mpam-controller` (build)
  - `Boostkit_CloudNative/K8S/bin/k8s-mpam-controller` (build-local)
- **Kunpeng TAP**: 
  - `Boostkit_CloudNative/K8S/bin/kunpeng-tap-manager`
  - `Boostkit_CloudNative/K8S/bin/kunpeng-tap-proxy`

## 命令说明

### 构建和清理命令
```bash
make build                  # 构建所有项目
make clean                  # 清理所有构建产物
make docker-build           # 构建所有Docker镜像
```

### MPAM Controller命令
```bash
make mpam-build             # 构建MPAM Controller
make mpam-build-local       # 本地构建MPAM Controller
make mpam-clean             # 清理MPAM构建产物
make mpam-docker            # 构建MPAM Docker镜像
make mpam-install           # 安装到系统
make mpam-run               # 运行MPAM Controller
make mpam-fmt               # 格式化MPAM代码
make mpam-vet               # 运行MPAM go vet检查
make mpam-test              # 测试MPAM项目
```

### Kunpeng TAP命令
```bash
# 构建命令
make kunpeng-tap-build                      # 构建TAP组件
make kunpeng-tap-clean                      # 清理TAP构建产物

# 管理命令
make kunpeng-tap-install-service-docker     # 安装Docker版本服务
make kunpeng-tap-install-service-containerd # 安装Containerd版本服务
make kunpeng-tap-start-service              # 启动服务
make kunpeng-tap-stop-service               # 停止服务
make kunpeng-tap-restart-service            # 重启服务
make kunpeng-tap-status-service             # 查看服务状态
make kunpeng-tap-uninstall-service          # 卸载TAP服务
```

## 构建系统架构

本项目采用三层Makefile结构：

- **顶层Makefile** (`./Makefile`): 统一管理所有子项目
- **K8S协调Makefile** (`./Boostkit_CloudNative/K8S/Makefile`): 协调K8S相关的两个子项目
- **项目专用Makefile**: `Makefile.mpam`和`Makefile.kunpeng-tap`

**详细说明**: [README.build-system.md](Boostkit_CloudNative/K8S/README.build-system.md)

## 开发指南

### 添加新项目
1. 在适当的目录下创建项目代码
2. 创建项目专用的Makefile（如`Makefile.newproject`）
3. 在上层Makefile中添加对应的命令
4. 更新文档

### 代码规范
- 使用`make fmt`格式化代码
- 使用`make vet`进行静态检查
- 编写单元测试并使用`make test`验证

## 许可证

本项目采用Apache License 2.0许可证。详见[LICENSE](LICENSE)文件。

## 贡献

欢迎提交Issue和Pull Request来改进项目。请确保：

1. 代码符合项目规范
2. 包含适当的测试
3. 更新相关文档

## 联系方式

如有问题或建议，请通过Gitee Issues和[Kunpeng 社区](https://www.hikunpeng.com/zh/developer/boostkit/)联系我们。 
