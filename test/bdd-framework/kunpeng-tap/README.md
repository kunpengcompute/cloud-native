# kunpeng-tap BDD 测试套件

kunpeng-tap 拓扑感知调度插件的完整 BDD 测试套件。

## 📋 目录

- [快速开始](#快速开始)
- [测试概览](#测试概览)
- [运行测试](#运行测试)
- [测试报告](#测试报告)
- [编写新测试](#编写新测试)
- [故障排查](#故障排查)

## 🚀 快速开始

### 前置条件

- Python 3.7+
- Kubernetes 集群（已安装 kunpeng-tap）
- kubectl 已配置
- 测试节点配置：
  - 2NUMA 配置：每个 NUMA 48 核（总 96 核）
  - 4NUMA 配置：每个 NUMA 24 核（总 96 核）

### 安装依赖

运行完整测试前的检查清单：

- [ ] 集群可访问
- [ ] kunpeng-tap 已安装并运行
- [ ] 测试节点有正确的 NUMA 配置
- [ ] Python 依赖已安装
- [ ] kubectl 已配置
- [ ] 有足够的集群资源

```bash
# 安装 Python 依赖
pip install pytest pytest-bdd pytest-html pytest-timeout pytest-xdist kubernetes
```

### 运行测试

```bash
# 运行所有测试
pytest -v

# 运行冒烟测试（快速验证）
pytest -m smoke -v

# 生成 HTML 报告
pytest --html=report.html --self-contained-html
```

## 📊 测试概览

### 测试统计

- **总测试数**: 113 个测试场景
- **测试类型**: 6 个测试套件
- **覆盖范围**: 拓扑亲和、NUMA 亲和、QoS、混合部署、边界条件、故障恢复

### 测试套件

| 测试套件 | 文件 | 测试数 | 描述 |
|---------|------|--------|------|
| 冒烟测试 | test_smoke.py | 10 | 快速验证核心功能 |
| 拓扑感知测试 | test_topology_aware.py | 51 | NUMA/Socket 亲和性测试 (topology-aware) |
| NUMA 感知测试 | test_numa_aware.py | 13 | CPU 亲和性测试 (numa-aware) |
| 混合部署测试 | test_mixed_deployment.py | 21 | 资源竞争场景 |
| 边界条件测试 | test_boundary_condition.py | 15 | 极端资源请求 (2NUMA: 7, 4NUMA: 8) |
| 故障恢复测试 | test_failure_recovery.py | 3 | 故障恢复能力 |

### 测试覆盖

#### 1. QoS 类型
- ✅ Guaranteed QoS
- ✅ Burstable QoS
- ✅ BestEffort QoS

#### 2. 拓扑级别
- ✅ NUMA 级别亲和
- ✅ Socket 级别亲和
- ✅ 跨 NUMA 调度
- ✅ 跨 Socket 调度

#### 3. 机器配置
- ✅ 2NUMA 配置（每个 NUMA 48 核）
- ✅ 4NUMA 配置（每个 NUMA 24 核）

#### 4. 部署场景
- ✅ 单容器部署
- ✅ 混合部署（资源竞争）
- ✅ 边界条件（最小/最大资源）

#### 5. 故障场景
- ✅ kunpeng-tap 插件重启
- ✅ 容器反复重启
- ✅ containerd 服务重启

## 🧪 运行测试

### 基本命令

```bash
# 运行所有测试
pytest -v

# 运行特定测试套件
pytest test_smoke.py -v                    # 冒烟测试
pytest test_topology_aware.py -v          # 拓扑感知测试
pytest test_numa_aware.py -v              # NUMA 感知测试
pytest test_mixed_deployment.py -v        # 混合部署测试
pytest test_boundary_condition.py -v      # 边界条件测试
pytest test_failure_recovery.py -v        # 故障恢复测试

# 运行特定测试
pytest test_smoke.py::test_guaranteed容器numa亲和基本验证__smoke01 -v
```

### 使用标记过滤

```bash
# 按测试类型
pytest -m smoke -v                         # 冒烟测试
pytest -m critical -v                      # 关键测试
pytest -m regression -v                    # 回归测试

# 按 QoS 类型
pytest -m guaranteed_qos -v                # Guaranteed QoS
pytest -m burstable_qos -v                 # Burstable QoS
pytest -m besteffort_qos -v                # BestEffort QoS

# 按拓扑类型
pytest -m numa_affinity -v                 # NUMA 亲和
pytest -m socket_affinity -v               # Socket 亲和
pytest -m cross_numa -v                    # 跨 NUMA

# 按机器配置
pytest -m 2numa -v                         # 2NUMA 配置
pytest -m 4numa -v                         # 4NUMA 配置

# 组合使用
pytest -m "smoke and critical" -v
pytest -m "guaranteed_qos and 2numa" -v
pytest -m "smoke or critical" -v
```

### 使用关键字过滤

```bash
# 按场景名称
pytest -k "guaranteed" -v                  # 所有 Guaranteed 测试
pytest -k "numa" -v                        # 所有 NUMA 相关测试
pytest -k "smoke01 or smoke02" -v          # 特定场景

# 排除特定测试
pytest -k "not slow" -v
```

### 并行运行

```bash
# 安装 pytest-xdist
pip install pytest-xdist

# 并行运行（4 个进程）
pytest -n 4 -v

# 并行运行冒烟测试
pytest -m smoke -n 4 -v
```

### 调试选项

```bash
# 显示 print 输出
pytest -v -s

# 失败时进入调试器
pytest --pdb

# 第一个失败后停止
pytest -x

# 显示最慢的 10 个测试
pytest --durations=10

# 详细的错误回溯
pytest --tb=long
```

## 📊 测试报告

### HTML 报告

```bash
# 生成 HTML 报告
pytest --html=report.html --self-contained-html

# 生成带元数据的报告
pytest --html=report.html --self-contained-html \
       --metadata "Environment" "Test" \
       --metadata "Cluster" "kunpeng-test" \
       --metadata "Tester" "$(whoami)"

# 在浏览器中打开
firefox report.html  # 或 chrome report.html
```

## ✍️ 编写新测试

### 步骤 1: 创建 Feature 文件

在 `features/` 目录下创建 `.feature` 文件：

```gherkin
# features/my_new_tests.feature
Feature: 我的新功能测试
  作为开发者
  我希望验证新功能
  以确保功能正常工作

  @my-test @smoke
  Scenario: 新功能基本验证 - MY-01
    Given 集群有 2numa_48 配置的节点
    When 我创建 1 个 guaranteed 类型的 Deployment
    And Deployment 具有 4/4 CPU 资源配置
    And Deployment 具有标签 "test-MY-01"
    Then 容器应该被成功调度
    And 容器的 CPU 应该分配在 NUMA 级别
```

### 步骤 2: 注册 Marker

在 `pytest.ini` 中添加新的 marker：

```ini
[pytest]
markers =
    my-test: 我的新测试
```

### 步骤 3: 创建测试运行器

创建 `test_my_new_tests.py`:

```python
"""我的新功能测试"""
import pytest
from pytest_bdd import scenarios

# 导入步骤定义
from step_definitions.topology_aware_steps import *

# 加载所有场景
scenarios('features/my_new_tests.feature')

# 测试前后处理
@pytest.fixture(scope="function", autouse=True)
def setup_and_cleanup():
    print("\n🚀 开始测试...")
    yield
    print("\n🧹 清理资源...")
    cleanup_test_resources()
```

### 步骤 4: 添加自定义步骤定义（可选）

如果需要新的步骤定义，在 `step_definitions/` 中添加：

```python
# step_definitions/my_steps.py
from pytest_bdd import then, parsers
from step_definitions.common_steps import _get_test_pods

@then(parsers.parse('容器应该分配到 {expected_node}'))
def verify_node_assignment(expected_node):
    """验证容器分配到特定节点"""
    pods = _get_test_pods()

    for pod in pods:
        actual_node = pod.spec.node_name
        if actual_node != expected_node:
            raise Exception(
                f"节点不匹配: 期望 {expected_node}, 实际 {actual_node}"
            )

    print(f"✅ 所有容器都分配到节点 {expected_node}")
```

然后在测试运行器中导入：

```python
from step_definitions.topology_aware_steps import *
from step_definitions.my_steps import *
```

### 步骤 5: 运行测试

```bash
pytest test_my_new_tests.py -v
```

## 🔧 故障排查

### 问题: 测试失败 - 资源不足

**原因**: 集群资源不足

**解决方案**:
```bash
# 检查节点资源
kubectl describe nodes

# 清理旧的测试资源
kubectl delete deployment -l test-label

# 减少并发测试数量
pytest -n 2 -v  # 使用 2 个进程而不是 4 个
```

### 问题: 测试超时

**原因**: Pod 调度或启动时间过长

**解决方案**:
```bash
# 增加超时时间
pytest --timeout=600 -v  # 10 分钟超时

# 检查 Pod 状态
kubectl get pods -A | grep test-
```

### 问题: 步骤定义找不到

**原因**: 步骤定义未导入

**解决方案**:
```python
# 确保导入了所有步骤定义
from step_definitions.topology_aware_steps import *
```

### 问题: NUMA 拓扑信息获取失败

**原因**: 节点没有 NUMA 拓扑信息

**解决方案**:
```bash
# 检查节点标签
kubectl get nodes --show-labels

# 检查节点是否有 NUMA 信息
kubectl describe node <node-name> | grep -i numa
```

## 📚 参考文档

- [TESTCASE.md](TESTCASE.md) - 完整的测试用例文档
- [pytest.ini](pytest.ini) - pytest 配置
- [../demo-project/](../demo-project/) - BDD 框架演示项目

## 🎉 开始测试

现在你已经准备好运行 kunpeng-tap 的完整测试套件了！

```bash
# 快速验证
pytest -m smoke -v

# 完整测试
pytest -v --html=report.html --self-contained-html

# 查看报告
firefox report.html
```

祝你测试愉快！
