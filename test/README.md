# Kubernetes 项目测试框架

这个目录包含了 Kubernetes 相关项目的测试框架和具体项目的测试实现。

## 📁 目录结构

```
test/
├── bdd-framework/                      # 通用 BDD 测试框架
│   ├── core/                           # 核心组件
│   ├── kunpeng-tap/
│   │   └── bdd-tests/                  # kunpeng-tap 项目的 BDD 测试实现
│   ├── requirements.txt                # 框架依赖
│   ├── conftest.py                     # 通用配置
│   ├── pytest.ini                      # pytest 配置
│   └── README.md                       # 框架文档
├── kunpeng-tap/                        # kunpeng-tap 其他测试（Go 等）
│   ├── fake/                           # 模拟测试
│   ├── numa_aware/                     # NUMA 感知测试
│   ├── topology-aware/                 # 拓扑感知测试
│   └── utils/                          # 测试工具
├── kunpeng-perf-monitor/               # kunpeng-perf-monitor 项目测试
│   └── fake/                           # 模拟测试
└── README.md                           # 本文档
```

## 🎯 设计理念

### 通用框架 + 项目特定实现

- **通用 BDD 框架**: 提供可复用的测试基础设施
- **项目特定测试**: 基于通用框架实现具体项目的测试逻辑
- **标准化**: 统一的测试结构和最佳实践
- **可扩展性**: 新项目可以快速集成测试框架

## 🧩 框架组件

### 通用 BDD 框架 (`bdd-framework/`)

提供以下核心组件：

- **BaseManager**: 所有管理器的基类
- **ClusterManager**: Kubernetes 集群操作
- **ContainerManager**: 容器生命周期管理
- **ResourceValidator**: 资源验证功能
- **MetricsCollector**: 监控指标收集

### 项目特定测试

每个项目在自己的目录下实现：

- **BDD 测试**: 基于通用框架的行为驱动测试
- **集成测试**: 组件间集成测试
- **性能测试**: 性能基准测试

## 🚀 快速开始

### 🎯 新手入门（推荐）

如果您是 BDD 测试的新手：

```bash
# 1. 学习 BDD 基础概念
cd bdd-framework
cat QUICK_START.md

# 2. 阅读快速入门指南
cat QUICK_START.md

# 可选：直接体验 kunpeng-tap 集成示例
cd ../bdd-framework/kunpeng-tap
pip install -r ../requirements.txt
./run_generated_tests.sh -m "standard and 2numa" --dry-run
```

### ⚡ 快速体验现有项目

```bash
# 运行 kunpeng-tap BDD 测试
cd bdd-framework/kunpeng-tap
make setup && make test-smoke
```

### 🛠️ 为新项目添加测试

```bash
# 在 bdd-framework 下创建你的项目测试目录
mkdir -p bdd-framework/your-project
cd bdd-framework/your-project

# 参考 kunpeng-tap 示例的结构与脚本
ls ../kunpeng-tap
```

## 📋 支持的项目

### kunpeng-tap
- **拓扑感知调度**: 验证 NUMA 节点亲和性
- **containerd 集成**: 验证 NRI 插件功能
- **资源管理**: 验证 CPU/内存分配
- **性能测试**: 调度性能基准

### kunpeng-perf-monitor
- **性能监控**: 验证性能指标收集
- **数据分析**: 验证性能数据分析
- **告警功能**: 验证性能告警机制

## 🔧 配置管理

### 环境变量

通用配置：
```bash
export KUBECONFIG=/path/to/kubeconfig
export TEST_NAMESPACE=test-namespace
export LOG_LEVEL=INFO
```

项目特定配置：
```bash
# kunpeng-tap
export CONTAINER_RUNTIME=containerd
export KUNPENG_TAP_BINARY=/usr/local/bin/kunpeng-tap

# kunpeng-perf-monitor  
export PERF_MONITOR_BINARY=/usr/local/bin/kunpeng-perf-monitor
```

### 配置文件

每个项目可以有自己的配置文件：
- `conftest.py`: pytest 配置和 fixtures
- `pytest.ini`: pytest 运行配置
- `Makefile`: 构建和测试命令

## 🧪 测试类型和标记

### 通用测试标记

- `@pytest.mark.e2e`: 端到端测试
- `@pytest.mark.unit`: 单元测试
- `@pytest.mark.integration`: 集成测试
- `@pytest.mark.smoke`: 冒烟测试
- `@pytest.mark.performance`: 性能测试
- `@pytest.mark.regression`: 回归测试

### 项目特定标记

每个项目可以定义自己的测试标记：
- `@pytest.mark.topology_aware`: 拓扑感知测试
- `@pytest.mark.numa`: NUMA 相关测试
- `@pytest.mark.containerd`: containerd 集成测试

## 📊 测试报告

### HTML 报告

```bash
# 生成项目测试报告
cd project-tests/
make report
```

报告包含：
- 测试执行结果
- 代码覆盖率
- 性能指标
- 错误详情

### 持续集成

每个项目可以配置自己的 CI/CD 流水线：
- 自动化测试执行
- 测试报告生成
- 质量门禁检查

## 🤝 贡献指南

### 添加新项目测试

1. **创建项目目录**: `mkdir your-project-tests`
2. **复制模板**: 使用 `bdd-framework/examples/project-template`
3. **自定义配置**: 修改 `conftest.py` 和相关配置
4. **编写测试**: 添加 `.feature` 文件和步骤定义
5. **更新文档**: 更新项目 README 和本文档

### 扩展通用框架

1. **添加新组件**: 在 `bdd-framework/core/` 中添加新的管理器
2. **更新基类**: 扩展 `BaseManager` 功能
3. **添加工具**: 在 `bdd-framework/utils/` 中添加通用工具
4. **更新文档**: 更新框架文档和示例

### 最佳实践

1. **配置外部化**: 使用环境变量进行配置
2. **测试隔离**: 确保测试之间的独立性
3. **资源清理**: 测试后清理创建的资源
4. **错误处理**: 提供清晰的错误信息和日志
5. **文档同步**: 保持文档与代码同步

## 📝 许可证

本测试框架遵循项目许可证。
