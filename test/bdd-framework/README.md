# Kubernetes BDD 测试框架

## 🎯 框架简介

这是一个**通用的 Kubernetes BDD 测试框架脚手架**，为 K8s 相关项目提供快速端到端测试能力。

### 核心特性

- ✅ **零代码测试** - 使用 Gherkin 语法编写测试，无需写 Python 代码
- ✅ **自定义资源支持** - 支持 CPU/Memory 之外的任意自定义资源（GPU、FPGA 等）
- ✅ **灵活配置** - 支持环境变量、annotations、自定义镜像等
- ✅ **可复用框架** - 提供通用的 step definitions，开箱即用
- ✅ **快速上手** - 3 步即可运行你的第一个测试

---

## 📚 文档导航

### 快速开始

- **[QUICKSTART.md](QUICKSTART.md)** - 快速开始指南（3 步运行第一个测试）⭐
- **[DEMO_PROJECT.md](DEMO_PROJECT.md)** - 完整的 GPU 调度测试示例项目

### 深入了解

- **[ARCHITECTURE.md](ARCHITECTURE.md)** - 框架架构设计文档
- **[kunpeng-tap/FRAMEWORK_GUIDE.md](kunpeng-tap/FRAMEWORK_GUIDE.md)** - 框架使用指南
- **[kunpeng-tap/NEW_TEST_GUIDE.md](kunpeng-tap/NEW_TEST_GUIDE.md)** - 新用户测试用例定义流程

### 演示项目

- **[demo-project/](demo-project/)** - 框架演示项目（6 个演示测试）⭐

### 参考项目

- **[kunpeng-tap/](kunpeng-tap/)** - kunpeng-tap 拓扑感知调度测试（完整示例，108 个测试）

---

## 🚀 快速开始（3 步）

### 步骤 1: 创建项目目录

```bash
mkdir my-k8s-project
cd my-k8s-project
mkdir -p features step_definitions
```

### 步骤 2: 复制框架文件

```bash
# 从 demo-project 复制框架文件
cp ../demo-project/step_definitions/common_steps.py step_definitions/
cp ../demo-project/step_definitions/__init__.py step_definitions/
```

### 步骤 3: 编写并运行测试

**创建 Feature 文件** (`features/my_test.feature`):

```gherkin
Feature: 我的第一个测试
  Scenario: 创建一个 Deployment
    Given 集群有 default 配置的节点
    When 我创建 1 个 guaranteed 类型的 Deployment
    And Deployment 具有 1/1 CPU 资源配置
    And Deployment 具有 2Gi/2Gi Memory 资源配置
    And Deployment 具有标签 "test-first"
    Then 容器应该被成功调度
```

**创建测试文件** (`test_my_test.py`):

```python
from pytest_bdd import scenarios
from step_definitions.common_steps import *

scenarios('features/my_test.feature')
```

**运行测试**:

```bash
pytest test_my_test.py -v
```

**就这么简单！** 🎉

---

## 🎨 核心功能

### 1. 自定义资源支持

支持任意 Kubernetes 自定义资源：

```gherkin
# GPU 资源
And Deployment 具有自定义资源 nvidia.com/gpu 配置为 2/2

# FPGA 资源
And Deployment 具有自定义资源 intel.com/fpga 配置为 1/1

# 任意自定义设备
And Deployment 具有自定义资源 example.com/custom-device 配置为 4/4
```

### 2. 环境变量配置

```gherkin
And Deployment 具有环境变量 MY_VAR=my_value
And Deployment 具有环境变量 DEBUG=true
```

### 3. Annotation 配置

```gherkin
And Deployment 具有 annotation scheduler.alpha.kubernetes.io/critical-pod=""
And Deployment 具有 annotation my-project/priority=high
```

### 4. 自定义镜像和命名空间

```gherkin
Given 使用容器镜像 "my-registry.com/my-app:v1.0"
And 使用命名空间 "my-namespace"
```

### 5. 组合使用

```gherkin
Scenario: AI 训练任务
  Given 使用容器镜像 "pytorch/pytorch:latest"
  And 使用命名空间 "ai-training"
  When 我创建 1 个 guaranteed 类型的 Deployment
  And Deployment 具有 8/8 CPU 资源配置
  And Deployment 具有 32Gi/32Gi Memory 资源配置
  And Deployment 具有自定义资源 nvidia.com/gpu 配置为 2/2
  And Deployment 具有环境变量 TRAINING_MODE=distributed
  And Deployment 具有 annotation ai.scheduler/model=resnet50
  And Deployment 具有标签 "test-ai-training"
  Then 容器应该被成功调度
```

---

## 📖 可用的 Step Definitions

### Given 步骤（环境准备）

| Step | 说明 |
|------|------|
| `Given 集群有 {config} 配置的节点` | 设置集群配置 |
| `Given 使用命名空间 "{namespace}"` | 设置命名空间 |
| `Given 使用容器镜像 "{image}"` | 设置容器镜像 |

### When 步骤（Deployment 创建）

| Step | 说明 |
|------|------|
| `When 我创建 {count} 个 {qos} 类型的 Deployment` | 创建 Deployment |
| `And Deployment 具有 {cpu_req}/{cpu_lim} CPU 资源配置` | 设置 CPU |
| `And Deployment 具有 {mem_req}/{mem_lim} Memory 资源配置` | 设置 Memory |
| `And Deployment 具有自定义资源 {name} 配置为 {req}/{lim}` | **设置自定义资源** |
| `And Deployment 具有环境变量 {name}={value}` | **设置环境变量** |
| `And Deployment 具有 annotation {key}={value}` | **设置 annotation** |
| `And Deployment 具有标签 "{label}"` | 设置标签并创建 |

### Then 步骤（验证）

| Step | 说明 |
|------|------|
| `Then 容器应该被成功调度` | 验证调度成功 |
| `And 容器的 QoS 类别应该是 {qos}` | 验证 QoS |

---

## 🔧 扩展框架

### 添加自定义验证步骤

创建 `step_definitions/my_custom_steps.py`:

```python
from pytest_bdd import then, parsers
from step_definitions.common_steps import _get_test_pods

@then(parsers.parse('容器应该分配到 {gpu_count:d} 个 GPU'))
def verify_gpu_allocation(gpu_count):
    """验证 GPU 分配"""
    pods = _get_test_pods()

    for pod in pods:
        container = pod.spec.containers[0]
        gpu_limit = container.resources.limits.get('nvidia.com/gpu', '0')

        if int(gpu_limit) != gpu_count:
            raise Exception(f"GPU 数量不匹配: 期望 {gpu_count}, 实际 {gpu_limit}")

    print(f"✅ 所有容器都分配到 {gpu_count} 个 GPU")
```

在测试文件中导入：

```python
from step_definitions.common_steps import *
from step_definitions.my_custom_steps import *
```

---

## 📁 项目结构

### 推荐的目录结构

```
my-k8s-project/
├── features/                      # Feature 文件
│   ├── basic_tests.feature
│   ├── gpu_tests.feature
│   └── advanced_tests.feature
│
├── step_definitions/              # Step definitions
│   ├── __init__.py
│   ├── common_steps.py           # 通用步骤（从框架复制）
│   └── my_custom_steps.py        # 自定义步骤（可选）
│
├── test_basic.py                  # 测试运行文件
├── test_gpu.py
├── test_advanced.py
│
├── pytest.ini                     # pytest 配置
└── README.md
```

### pytest 配置示例

**`pytest.ini`**:

```ini
[pytest]
bdd_features_base_dir = features/

addopts =
    -v
    --tb=short
    --strict-markers

markers =
    smoke: 冒烟测试
    gpu: GPU 相关测试
    slow: 慢速测试
```

---

## 🎯 示例项目

### demo-project（演示项目）⭐

demo-project 是框架的演示项目，展示核心功能：

- ✅ GPU 资源配置示例
- ✅ FPGA 资源配置示例
- ✅ Annotation 配置示例
- ✅ 环境变量配置示例
- ✅ 组合配置示例
- ✅ 多种自定义资源配置示例

**总计**: 6 个演示测试

**查看示例**:

```bash
cd demo-project/
pytest -v  # 运行所有演示测试
pytest -m gpu -v  # 运行 GPU 测试
pytest -m demo -v  # 运行所有演示
```

### kunpeng-tap（完整生产示例）

kunpeng-tap 是一个完整的拓扑感知调度测试项目，包含：

- ✅ 47 个拓扑感知测试
- ✅ 20 个混合部署测试
- ✅ 15 个批量部署测试
- ✅ 10 个边界条件测试
- ✅ 10 个冒烟测试
- ✅ 6 个其他测试

**总计**: 108 个测试

**查看示例**:

```bash
cd kunpeng-tap/
pytest -v  # 运行所有测试
pytest -m smoke -v  # 运行冒烟测试
```

---

## 📊 框架架构

```
┌─────────────────────────────────────────┐
│  测试层 (Feature 文件 + test_*.py)      │
└─────────────────────────────────────────┘
                 ↓
┌─────────────────────────────────────────┐
│  业务层 (项目特定的 step definitions)    │
└─────────────────────────────────────────┘
                 ↓
┌─────────────────────────────────────────┐
│  框架层 (common_steps.py)               │
│  - 12 个 Given/When/Then 步骤           │
│  - 5 个辅助函数                          │
└─────────────────────────────────────────┘
                 ↓
┌─────────────────────────────────────────┐
│  Kubernetes API                         │
└─────────────────────────────────────────┘
```

详细架构设计请参考 [ARCHITECTURE.md](ARCHITECTURE.md)

---

## 🎉 开始使用

### 1. 阅读快速开始指南

```bash
cat QUICKSTART.md
```

### 2. 查看完整示例项目

```bash
cat DEMO_PROJECT.md
```

### 3. 运行 Demo 测试

```bash
cd demo-project/
pytest -v
```

### 4. 创建你的项目

```bash
mkdir my-k8s-project
cd my-k8s-project
# 按照 QUICKSTART.md 的步骤操作
```

---

**开始构建你的 BDD 测试吧！** 🚀
