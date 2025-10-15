# BDD 测试框架演示项目

## 🎯 项目简介

这是一个完整的演示项目，展示如何使用 BDD 测试框架进行 Kubernetes 测试。

### 演示的功能

- ✅ **基本部署** - 简单的 Deployment 创建和验证
- ✅ **环境变量配置** - 容器环境变量设置
- ✅ **Annotation 配置** - Pod annotations 设置
- ✅ **QoS 类别** - Guaranteed、Burstable 等 QoS 配置
- ✅ **组合配置** - 同时使用多种配置
- ✅ **多副本部署** - 创建多个副本

---

## 📁 项目结构

```
demo-project/
├── features/                           # Feature 文件
│   └── custom_resource_demo.feature   # 演示场景
│
├── step_definitions/                   # Step definitions
│   ├── __init__.py
│   └── common_steps.py                # 通用步骤（从框架复制）
│
├── test_custom_resource_demo.py        # 测试运行文件
├── pytest.ini                          # pytest 配置
└── README.md                           # 本文件
```

---

## 🚀 快速开始

### 前置条件

- Python 3.7+
- pytest
- pytest-bdd
- kubernetes Python 客户端
- 可访问的 Kubernetes 集群

### 安装依赖

```bash
pip install pytest pytest-bdd kubernetes
```

### 运行演示测试

```bash
# 运行所有演示测试
pytest -v

# 运行特定标记的测试
pytest -m basic -v        # 只运行基本部署测试
pytest -m demo -v         # 运行所有演示测试
pytest -m combined -v     # 运行组合配置测试
pytest -m qos -v          # 运行 QoS 相关测试

# 运行特定场景
pytest -k "基本部署" -v
pytest -k "环境变量" -v
```

---

## 📖 演示场景

### 1. 基本部署示例

演示如何创建一个简单的 Deployment：

```gherkin
Scenario: 基本部署示例
  Given 使用容器镜像 "nginx:latest"
  When 我创建 1 个 guaranteed 类型的 Deployment
  And Deployment 具有 2/2 CPU 资源配置
  And Deployment 具有 4Gi/4Gi Memory 资源配置
  And Deployment 具有标签 "test-basic-demo"
  Then 容器应该被成功调度
  And 容器的 QoS 类别应该是 Guaranteed
```

### 2. Annotation 配置示例

演示如何配置 Pod annotations：

```gherkin
Scenario: Annotation 配置示例
  Given 使用容器镜像 "nginx:latest"
  When 我创建 1 个 guaranteed 类型的 Deployment
  And Deployment 具有 annotation scheduler.alpha.kubernetes.io/critical-pod=""
  And Deployment 具有 annotation my-project/priority=high
  And Deployment 具有 annotation my-project/team=platform
  Then 容器应该被成功调度
```

### 3. 环境变量配置示例

演示如何配置环境变量：

```gherkin
Scenario: 环境变量配置示例
  Given 使用容器镜像 "busybox:latest"
  When 我创建 1 个 guaranteed 类型的 Deployment
  And Deployment 具有环境变量 APP_ENV=production
  And Deployment 具有环境变量 LOG_LEVEL=info
  And Deployment 具有环境变量 DEBUG=false
  Then 容器应该被成功调度
```

### 4. Burstable QoS 示例

演示如何配置 Burstable QoS 类别：

```gherkin
Scenario: Burstable QoS 示例
  Given 使用容器镜像 "nginx:latest"
  When 我创建 1 个 burstable 类型的 Deployment
  And Deployment 具有 1/2 CPU 资源配置
  And Deployment 具有 2Gi/4Gi Memory 资源配置
  Then 容器应该被成功调度
  And 容器的 QoS 类别应该是 Burstable
```

### 5. 组合配置示例

演示如何组合使用多种配置：

```gherkin
Scenario: 组合配置示例（环境变量 + Annotation + 命名空间）
  Given 使用容器镜像 "nginx:latest"
  And 使用命名空间 "demo-namespace"
  When 我创建 1 个 guaranteed 类型的 Deployment
  And Deployment 具有 4/4 CPU 资源配置
  And Deployment 具有 8Gi/8Gi Memory 资源配置
  And Deployment 具有环境变量 APP_NAME=demo-app
  And Deployment 具有环境变量 APP_VERSION=1.0.0
  And Deployment 具有 annotation app.kubernetes.io/name=demo-app
  And Deployment 具有 annotation app.kubernetes.io/version=1.0.0
  Then 容器应该被成功调度
```

### 6. 多副本部署示例

演示如何创建多个副本：

```gherkin
Scenario: 多副本部署示例
  Given 使用容器镜像 "nginx:latest"
  When 我创建 3 个 guaranteed 类型的 Deployment
  And Deployment 具有 1/1 CPU 资源配置
  And Deployment 具有 2Gi/2Gi Memory 资源配置
  Then 容器应该被成功调度
```

---

## 🎨 核心功能

### 可用的 Step Definitions

#### Given 步骤（环境准备）

- `Given 集群有 {config} 配置的节点`
- `Given 使用命名空间 "{namespace}"`
- `Given 使用容器镜像 "{image}"`

#### When 步骤（Deployment 创建）

- `When 我创建 {count} 个 {qos} 类型的 Deployment`
- `And Deployment 具有 {cpu_req}/{cpu_lim} CPU 资源配置`
- `And Deployment 具有 {mem_req}/{mem_lim} Memory 资源配置`
- `And Deployment 具有环境变量 {name}={value}` ⭐
- `And Deployment 具有 annotation {key}={value}` ⭐
- `And Deployment 具有标签 "{label}"`

#### Then 步骤（验证）

- `Then 容器应该被成功调度`
- `And 容器的 QoS 类别应该是 {qos}`

---

## 📊 预期输出

运行测试后，你应该看到类似的输出：

```
======================== test session starts =========================
collected 6 items

test_custom_resource_demo.py::test_基本部署示例 PASSED          [ 16%]
test_custom_resource_demo.py::test_annotation_配置示例 PASSED   [ 33%]
test_custom_resource_demo.py::test_环境变量配置示例 PASSED       [ 50%]
test_custom_resource_demo.py::test_burstable_qos_示例 PASSED    [ 66%]
test_custom_resource_demo.py::test_组合配置示例 PASSED           [ 83%]
test_custom_resource_demo.py::test_多副本部署示例 PASSED         [100%]

========================= 6 passed in 35.12s =========================
```

---

## 🔧 自定义和扩展

### 添加自定义验证步骤

如果需要添加项目特定的验证逻辑，创建 `step_definitions/custom_steps.py`：

```python
from pytest_bdd import then, parsers
from step_definitions.common_steps import _get_test_pods

@then(parsers.parse('容器应该运行在节点 "{node_name}"'))
def verify_pod_node(node_name):
    """验证 Pod 运行在指定节点"""
    pods = _get_test_pods()

    for pod in pods:
        if pod.spec.node_name != node_name:
            raise Exception(
                f"Pod {pod.metadata.name} 运行在节点 {pod.spec.node_name}, "
                f"期望节点 {node_name}"
            )

    print(f"✅ 所有容器都运行在节点 {node_name}")

@then(parsers.parse('容器应该有环境变量 {env_name}'))
def verify_env_var_exists(env_name):
    """验证容器有指定的环境变量"""
    pods = _get_test_pods()

    for pod in pods:
        container = pod.spec.containers[0]
        env_vars = {env.name: env.value for env in (container.env or [])}

        if env_name not in env_vars:
            raise Exception(f"容器缺少环境变量: {env_name}")

    print(f"✅ 所有容器都有环境变量 {env_name}")
```

然后在测试文件中导入：

```python
from step_definitions.common_steps import *
from step_definitions.custom_steps import *
```

---

## 📝 注意事项

1. **集群要求**: 确保有可访问的 Kubernetes 集群
2. **镜像拉取**: 确保集群可以拉取测试中使用的镜像（nginx、busybox 等）
3. **命名空间**: 测试会在指定的命名空间中创建资源
4. **资源清理**: 测试完成后会自动清理创建的资源
5. **资源配额**: 确保集群有足够的资源来运行测试

---

## 🎯 学习路径

1. **查看 Feature 文件** - 了解测试场景的定义
2. **查看 common_steps.py** - 了解步骤定义的实现
3. **运行测试** - 观察测试执行过程
4. **修改场景** - 尝试修改测试场景
5. **添加自定义步骤** - 实现项目特定的验证逻辑

---

## 🚀 下一步

- 阅读 [QUICKSTART.md](../QUICKSTART.md) 了解如何创建自己的项目
- 阅读 [ARCHITECTURE.md](../ARCHITECTURE.md) 了解框架架构
- 查看 [kunpeng-tap](../kunpeng-tap/) 项目了解完整的生产示例

---

**开始探索 BDD 测试框架吧！** 🎉

