# kunpeng-tap 拓扑感知测试用例

## 测试目标
测试容器在 topology-aware 策略下是否达成了预期的 NUMA/SOCKET 亲和性。

## 测试对象 - QoS 类型
根据 Kubernetes QoS 分类，测试不同类型容器的亲和行为：

### 1. Guaranteed QoS (会产生亲和)
- **定义**: requests = limits (CPU 和 Memory)
- **预期行为**: 应该实现拓扑亲和调度
- **测试重点**: 验证亲和性是否正确实现

### 2. Burstable QoS (会产生亲和)
- **定义**: requests < limits 或只设置部分资源的 requests/limits
- **预期行为**: 应该实现拓扑亲和调度
- **测试重点**: 验证亲和性是否正确实现

### 3. BestEffort QoS (不会产生亲和)
- **定义**: 不设置 requests 和 limits
- **预期行为**: 不应该实现拓扑亲和调度，使用默认调度策略
- **测试重点**: 验证不会错误地应用亲和性

## 机器配置类型

### 2NUMA 配置
* **配置1**: NUMA#0: 0-47 (48核), NUMA#1: 48-95 (48核)
* **配置2**: NUMA#0: 0-159 (160核), NUMA#1: 160-319 (160核)

### 4NUMA 配置
* **配置1**: NUMA#0: 0-23, NUMA#1: 24-47, NUMA#2: 48-71, NUMA#3: 72-95 (SOCKET#0: 0-47, SOCKET#1: 48-95)
* **配置2**: NUMA#0: 0-79, NUMA#1: 80-159, NUMA#2: 160-239, NUMA#3: 240-319 (SOCKET#0: 0-159, SOCKET#1: 160-319)

## 测试场景分类

### 1. CPU 亲和性功能测试
按照 requests.cpu 和 limits.cpu 值，测试与 NUMA、SOCKET 亲和的关系：

#### 2NUMA 场景测试案例

**机器配置1**: NUMA#0: 0-47 (48核), NUMA#1: 48-95 (48核)

| 编号 | request | limit | 期望亲和 | 测试目标 | 备注 |
|------|---------|-------|----------|----------|------|
| 2N-01 | 4 | 4 | NUMA#0 或 NUMA#1 | 多核容器NUMA内亲和 | 中等资源请求 |
| 2N-02 | 24 | 24 | NUMA#0 或 NUMA#1 | 半NUMA容器亲和 | NUMA内一半资源 |
| 2N-03 | 47 | 47 | NUMA#0 或 NUMA#1 | 接近满NUMA亲和 | 接近NUMA边界 |
| 2N-04 | 48 | 48 | NUMA#0 或 NUMA#1 | 满NUMA亲和 | 完整NUMA资源 |
| 2N-05 | 49 | 49 | Socket#0 | 跨NUMA调度 | 超出单NUMA容量 |
| 2N-06 | 72 | 72 | Socket#0 | 大容器跨NUMA | 需要1.5个NUMA |
| 2N-07 | 90 | 90 | 调度/部署失败 | 满机器资源 | 使用全部CPU |
| 2N-08 | 0.5 | 1 | NUMA#0 或 NUMA#1 | 小数点资源请求 | request < limit |
| 2N-09 | 2 | 4 | NUMA#0 或 NUMA#1 | 弹性资源请求 | request < limit |

**机器配置2**: NUMA#0: 0-159 (160核), NUMA#1: 160-319 (160核)

| 编号 | request | limit | 期望亲和 | 测试目标 | 备注 |
|------|---------|-------|----------|----------|------|
| 2N-10 | 8 | 8 | NUMA#0 或 NUMA#1 | 多核容器NUMA内亲和 | 中等资源请求 |
| 2N-11 | 80 | 80 | NUMA#0 或 NUMA#1 | 半NUMA容器亲和 | NUMA内一半资源 |
| 2N-12 | 159 | 159 | NUMA#0 或 NUMA#1 | 接近满NUMA亲和 | 接近NUMA边界 |
| 2N-13 | 160 | 160 | NUMA#0 或 NUMA#1 | 满NUMA亲和 | 完整NUMA资源 |
| 2N-14 | 161 | 161 | Socket#0 | 跨NUMA调度 | 超出单NUMA容量 |
| 2N-15 | 240 | 240 | Socket#0 | 大容器跨NUMA | 需要1.5个NUMA |
| 2N-16 | 310 | 310 | 调度失败 | 满机器资源 | 使用全部CPU |
| 2N-17 | 0.5 | 1 | NUMA#0 或 NUMA#1 | 小数点资源请求 | request < limit |
| 2N-18 | 16 | 32 | NUMA#0 或 NUMA#1 | 弹性资源请求 | request < limit |

#### 4NUMA 场景测试案例

**机器配置1**: NUMA#0: 0-23 (24核), NUMA#1: 24-47 (24核), NUMA#2: 48-71 (24核), NUMA#3: 72-95 (24核)
SOCKET#0: 0-47 (48核), SOCKET#1: 48-95 (48核)

| 编号 | request | limit | 期望亲和 | 测试目标 | 备注 |
|------|---------|-------|----------|----------|------|
| 4N-01 | 4 | 4 | 单NUMA | 多核容器NUMA内亲和 | 中等资源请求 |
| 4N-02 | 12 | 12 | 单NUMA | 半NUMA容器亲和 | NUMA内一半资源 |
| 4N-03 | 23 | 23 | 单NUMA | 接近满NUMA亲和 | 接近NUMA边界 |
| 4N-04 | 24 | 24 | 单NUMA | 满NUMA亲和 | 完整NUMA资源 |
| 4N-05 | 25 | 25 | 跨NUMA同SOCKET | 跨NUMA调度 | 超出单NUMA，同SOCKET |
| 4N-06 | 36 | 36 | 跨NUMA同SOCKET | 1.5NUMA容器 | 需要1.5个NUMA |
| 4N-07 | 48 | 48 | 单SOCKET | 满SOCKET亲和 | 完整SOCKET资源 |
| 4N-08 | 49 | 49 | 跨SOCKET | 跨SOCKET调度 | 超出单SOCKET容量 |
| 4N-09 | 72 | 72 | 跨SOCKET | 大容器跨SOCKET | 需要1.5个SOCKET |
| 4N-10 | 90 | 90 | 跨SOCKET | 满机器资源 | 使用全部CPU |
| 4N-11 | 0.5 | 1 | 单NUMA | 小数点资源请求 | request < limit |
| 4N-12 | 2 | 4 | 单NUMA | 弹性资源请求 | request < limit |

**机器配置2**: NUMA#0: 0-79 (80核), NUMA#1: 80-159 (80核), NUMA#2: 160-239 (80核), NUMA#3: 240-319 (80核)
SOCKET#0: 0-159 (160核), SOCKET#1: 160-319 (160核)

| 编号 | request | limit | 期望亲和 | 测试目标 | 备注 |
|------|---------|-------|----------|----------|------|
| 4N-13 | 8 | 8 | 单NUMA | 多核容器NUMA内亲和 | 中等资源请求 |
| 4N-14 | 40 | 40 | 单NUMA | 半NUMA容器亲和 | NUMA内一半资源 |
| 4N-15 | 79 | 79 | 单NUMA | 接近满NUMA亲和 | 接近NUMA边界 |
| 4N-16 | 80 | 80 | 单NUMA | 满NUMA亲和 | 完整NUMA资源 |
| 4N-17 | 81 | 81 | 跨NUMA同SOCKET | 跨NUMA调度 | 超出单NUMA，同SOCKET |
| 4N-18 | 120 | 120 | 跨NUMA同SOCKET | 1.5NUMA容器 | 需要1.5个NUMA |
| 4N-19 | 160 | 160 | 单SOCKET | 满SOCKET亲和 | 完整SOCKET资源 |
| 4N-20 | 161 | 161 | 跨SOCKET | 跨SOCKET调度 | 超出单SOCKET容量 |
| 4N-21 | 240 | 240 | 跨SOCKET | 大容器跨SOCKET | 需要1.5个SOCKET |
| 4N-22 | 310 | 310 | 跨SOCKET | 满机器资源 | 使用全部CPU |
| 4N-23 | 0.5 | 1 | 单NUMA | 小数点资源请求 | request < limit |
| 4N-24 | 16 | 32 | 单NUMA | 弹性资源请求 | request < limit |

### 3. 混合部署与资源竞争测试用例

测试在已有容器占用资源的情况下，新容器的拓扑亲和行为。重点验证 topology-aware 策略在资源竞争场景下的调度决策。

#### 3.1 NUMA 亲和后的资源竞争测试

**场景**: 多容器都亲和到 NUMA 后，测试新容器的亲和行为

**2NUMA 配置1**: NUMA#0: 0-47 (48核), NUMA#1: 48-95 (48核)

| 编号 | 已有容器配置（Guaranteed） | 新容器配置 | 期望亲和 | 测试目标 | 备注 |
|------|-------------|------------|----------|----------|------|
| MIX-2N-01 | NUMA#0: 20核, NUMA#1: 20核 | 4核/4核 | NUMA#0或NUMA#1| 小资源在已占用NUMA内亲和 | NUMA内剩余资源充足 |
| MIX-2N-02 | NUMA#0: 40核, NUMA#1: 30核 | 20核/20核 | NUMA#1 | 大资源选择剩余较多NUMA | 负载均衡策略 |
| MIX-2N-04 | NUMA#0: 45核, NUMA#1: 45核 | 8核/8核 | 跨NUMA | 资源不足时跨NUMA调度 | 单NUMA剩余不足 |
| MIX-2N-05 | NUMA#0: 10核, NUMA#1: 10核 | 49核/49核 | 跨NUMA | 大容器必须跨NUMA | 超出单NUMA剩余容量 |
| MIX-2N-06 | NUMA#0: 49核, NUMA#1: 0核 | 10核/10核 | NUMA#1 | 单NUMA剩余不足 | NUMA#0剩余不足，切换NUMA |


| 编号 | 已有容器配置（Burstable - request/limit） | 新容器配置 | 期望亲和 | 测试目标 | 备注 |
|------|-------------|------------|----------|----------|------|
| MIX-2N-07 | NUMA#0: 10核/20核, NUMA#1: 10核/20核 | 5核/10核 | NUMA#0或NUMA#1 | 中等资源切换到空闲NUMA | NUMA内剩余资源充足 |
| MIX-2N-08 | NUMA#0: 10核/40核, NUMA#1: 10核/30核 | 10核/20核 | NUMA#1 | 大资源选择剩余较多NUMA | 负载均衡策略 |
| MIX-2N-09 | NUMA#0: 10核/45核, NUMA#1: 10核/45核 | 4核/8核 | NUMA#0或NUMA#1 | requests资源充足，limits资源超出 | 单NUMA资源request容量充足，limits不足 |
| MIX-2N-10 | NUMA#0: 5核/10核, NUMA#1: 5核/10核 | 25核/49核 | 跨NUMA | 大容器必须跨NUMA | 超出单NUMA剩余容量 |

//**TODO：计算NUMA资源的依据 - limit/request?**

**4NUMA 配置1**: NUMA#0-3: 各24核, SOCKET#0: 0-47, SOCKET#1: 48-95

| 编号 | 已有容器配置 | 新容器配置 | 期望亲和 | 测试目标 | 备注 |
|------|-------------|------------|----------|----------|------|
| MIX-4N-01 | NUMA#0: 20核, NUMA#1: 15核 | 4核/4核 | NUMA#2或NUMA#3 | 小资源在剩余最多NUMA亲和 | NUMA内剩余资源充足 |
| MIX-4N-02 | NUMA#0: 20核, NUMA#1: 20核 | 8核/8核 | NUMA#2或NUMA#3 | 中等资源选择空闲NUMA | 避开已占用NUMA |
| MIX-4N-03 | 各NUMA: 10核 | 20核/20核 | NUMA#2+NUMA#3 | 大资源在同SOCKET内跨NUMA | 单NUMA不足，同SOCKET优先 |
| MIX-4N-04 | SOCKET#0各NUMA: 20核 | 30核/30核 | SOCKET#1内跨NUMA | 大资源切换到空闲SOCKET | SOCKET#0剩余不足 |
| MIX-4N-05 | 各NUMA: 10核 | 50核/50核 | 跨SOCKET | 超大资源必须跨SOCKET | 单SOCKET剩余不足 |

| 编号 | 已有容器配置（Burstable） | 新容器配置 | 期望亲和 | 测试目标 | 备注 |
|------|-------------|------------|----------|----------|------|
| MIX-4N-06 | NUMA#0: 10核/20核, NUMA#1: 10核/20核 | 5核/10核 | NUMA#0或NUMA#1 | 中等资源切换到空闲NUMA | NUMA内剩余资源充足 |
| MIX-4N-07 | NUMA#0: 10核/40核, NUMA#1: 10核/30核 | 10核/20核 | NUMA#1 | 大资源选择剩余较多NUMA | 负载均衡策略 |
| MIX-4N-08 | NUMA#0: 10核/45核, NUMA#1: 10核/45核 | 4核/8核 | NUMA#0或NUMA#1 | requests资源充足，limits资源超出 | 单NUMA资源request容量充足，limits不足 |
| MIX-4N-09 | NUMA#0: 5核/10核, NUMA#1: 5核/10核 | 25核/49核 | 跨NUMA | 大容器必须跨NUMA | 超出单NUMA剩余容量 |

#### 3.2 Socket 亲和后的资源竞争测试

**场景**: 多容器都亲和到 Socket 后，测试新容器的亲和行为

**4NUMA 配置1**: NUMA#0-3: 各24核, SOCKET#0: 0-47, SOCKET#1: 48-95

| 编号 | 已有容器配置（Guaranteed） | 新容器配置 | 期望亲和 | 测试目标 | 备注 |
|------|-------------|------------|----------|----------|------|
| MIX-SK-GA-01 | SOCKET#0: 40核 | 4核/4核 | SOCKET#1内单NUMA | NUMA亲和负载均衡 | 其他Socket/NUMA内剩余充足 |

| 编号 | 已有容器配置（Burstable） | 新容器配置 | 期望亲和 | 测试目标 | 备注 |
|------|-------------|------------|----------|----------|------|
| MIX-SK-BU-01 | SOCKET#0: 40核/40核, SOCKET#1: 40核/40核 | 4核/8核 | SOCKET#0或SOCKET#1内单NUMA | NUMA亲和负载均衡 | Socket/NUMA内剩余充足 |

### 4. 边界条件测试用例

| 编号 | request | limit | 机器配置 | 期望行为 | 测试目标 | 备注 |
|------|---------|-------|----------|----------|----------|------|
| BC-01 | 0.1 | 0.1 | 任意 | 调度成功 | 最小资源请求 | 极小资源 |
| BC-02 | 97 | 97 | 2NUMA-48核 | 调度失败 | 超出机器容量 | 资源不足 |
| BC-03 | 321 | 321 | 4NUMA-320核 | 调度失败 | 超出机器容量 | 资源不足 |
| BC-04 | 1000m | 2000m | 任意 | 调度成功 | 毫核单位 | 单位转换 |
| BC-05 | 1.5 | 1.5 | 任意 | 调度成功 | 小数点CPU | 非整数CPU |

### 5. 故障恢复场景测试

// **TODO: 增加容器反复重启，runtime断开链接，tap插件重启等用例**

**场景**: 测试节点故障恢复时的亲和性重建

| 编号 | 故障场景 | 恢复操作 | 期望行为 | 测试目标 | 备注 |
|------|----------|----------|----------|----------|------|
| FAIL-01 | tap插件重启 | 重建亲和关系 | 保持原有亲和 | 亲和关系重建 | 故障恢复后优化 |
| FAIL-02 | 容器反复重启 | 无法成功部署 | 保证其他正常容器亲和且均衡 | 失败容器部署影响 | 容器故障后恢复 |
| FAIL-03 | 容器运行时 containerd 重启 | 重建亲和关系 | 保持原有亲和 | 亲和关系重建 | 故障恢复后优化 |

### 6 . 压力测试用例
测试在大批量、高并发的容器部署场景下，tap 的亲和功能是否能够正常运行。

#### 批量部署测试 (使用 kube-burner 或类似工具)

| 编号 | 容器数量 | QoS类型 | CPU配置 | 机器配置 | 验证方法 | 测试目标 |
|------|----------|---------|---------|----------|----------|----------|
| BULK-01 | 50 | Guaranteed | 1核/1核 | 2NUMA-48核 | NUMA均衡检查 | 小容器批量亲和 |
| BULK-02 | 20 | Guaranteed | 2核/2核 | 2NUMA-48核 | NUMA均衡检查 | 中等容器批量亲和 |
| BULK-03 | 10 | Burstable | 1核/2核 | 2NUMA-48核 | NUMA均衡检查 | 弹性容器批量亲和 |
| BULK-04 | 100 | BestEffort | 无限制 | 2NUMA-48核 | 无亲和验证 | BestEffort批量调度 |
| BULK-05 | 80 | Guaranteed | 1核/1核 | 4NUMA-80核 | NUMA均衡检查 | 4NUMA小容器批量 |
| BULK-06 | 40 | Guaranteed | 2核/2核 | 4NUMA-80核 | NUMA均衡检查 | 4NUMA中等容器批量 |

#### 混合 QoS 批量测试

| 编号 | Guaranteed数量 | Burstable数量 | BestEffort数量 | 机器配置 | 验证方法 | 测试目标 |
|------|----------------|---------------|----------------|----------|----------|----------|
| MIX-BULK-01 | 20 | 20 | 20 | 2NUMA-48核 | 分类亲和检查 | 混合QoS批量调度 |
| MIX-BULK-02 | 30 | 30 | 30 | 4NUMA-80核 | 分类亲和检查 | 4NUMA混合QoS批量 |

## BDD 测试框架设计

### 通用步骤抽象

基于您的想法，我们将测试步骤抽象为以下通用模式：

#### Create 步骤
```gherkin
Given 我创建 <replica_count> 个 <qos_type> 类型的 Deployment
And Deployment 具有 <cpu_request>/<cpu_limit> CPU 资源配置
And Deployment 具有标签 <test_label>
```

#### Check 步骤
```gherkin
Then 容器的 CPU 应该分配在 <topology_level> 级别
And 容器在不同 <topology_level> 上分配应该均匀
```

#### AfterEach 步骤
```gherkin
After 删除带有标签 <test_label> 的 Deployment
```

### 参数化配置

#### QoS 类型定义
```yaml
qos_types:
  guaranteed:
    cpu_request: "2"
    cpu_limit: "2"
    memory_request: "4Gi"
    memory_limit: "4Gi"
    expected_affinity: true

  burstable:
    cpu_request: "1"
    cpu_limit: "2"
    memory_request: "2Gi"
    memory_limit: "4Gi"
    expected_affinity: true

  besteffort:
    # 不设置 requests 和 limits
    expected_affinity: false
```

#### 拓扑级别定义
```yaml
topology_levels:
  - CLUSTER    # 集群级别
  - NUMA       # NUMA 节点级别
  - SOCKET     # SOCKET 级别
  - DIE        # DIE 级别 (如果支持)
  - ROOT       # 根级别
```

#### 验证方法定义
```yaml
validation_methods:
  numa_balance_check:
    description: "检查 /sys/fs/cgroup/cpuset 下各 NUMA 的数量是否均衡"
    script: "check_numa_balance.sh"

  affinity_verification:
    description: "验证容器是否实现了预期的拓扑亲和"
    script: "verify_affinity.sh"

  qos_classification:
    description: "验证容器的 QoS 分类是否正确"
    script: "check_qos_class.sh"
```

### BDD 场景模板

#### 功能测试场景
```gherkin
Scenario Outline: <qos_type> QoS 容器的 <topology_level> 亲和测试
  Given 集群有 <machine_config> 配置的节点
  When 我创建 1 个 <qos_type> 类型的 Deployment
  And Deployment 具有 <cpu_request>/<cpu_limit> CPU 资源配置
  And Deployment 具有标签 "test-<test_id>"
  Then 容器应该被成功调度
  And 容器的 CPU 应该分配在 <expected_topology> 级别
  And 拓扑亲和性应该符合 <qos_type> 的预期行为
```

#### 混合部署测试场景
```gherkin
Scenario Outline: 资源竞争下的拓扑亲和测试
  Given 集群有 <machine_config> 配置的节点
  And 已有容器占用 <existing_allocation>
  When 我创建 <new_replica_count> 个 <qos_type> 类型的 Deployment
  And 每个 Deployment 具有 <cpu_request>/<cpu_limit> CPU 资源配置
  And Deployment 具有标签 "mix-test-<test_id>"
  Then 新容器应该被成功调度
  And 新容器的 CPU 应该分配在 <expected_topology> 级别
  And 整体 NUMA 均衡度应该在可接受范围内
  And 不应该影响已有容器的亲和性
```

#### 批量测试场景
```gherkin
Scenario Outline: 批量 <qos_type> 容器的 NUMA 均衡测试
  Given 集群有 <machine_config> 配置的节点
  When 我创建 <replica_count> 个 <qos_type> 类型的 Deployment
  And 每个 Deployment 具有 <cpu_request>/<cpu_limit> CPU 资源配置
  And Deployment 具有标签 "bulk-test-<test_id>"
  Then 所有容器应该被成功调度
  And 容器在不同 NUMA 上分配应该均匀
  And NUMA 均衡度应该在可接受范围内
```

#### 动态资源变化测试场景
```gherkin
Scenario Outline: 动态资源变化下的亲和性调整
  Given 集群有 <machine_config> 配置的节点
  And 初始部署 <initial_containers>
  When 执行 <operation> 操作
  And 创建新容器 <new_container_spec>
  Then 新容器应该根据当前资源状态调度
  And 拓扑亲和性应该符合 <expected_behavior>
  And 系统整体负载应该保持均衡
```

### 验证脚本设计

#### NUMA 均衡检查脚本
```bash
#!/bin/bash
# check_numa_balance.sh
# 检查 /sys/fs/cgroup/cpuset 下各 NUMA 的容器分布是否均衡

check_numa_balance() {
    local test_label=$1
    local tolerance=${2:-0.2}  # 允许的不均衡度

    # 获取各 NUMA 节点上的容器数量
    # 计算均衡度
    # 返回检查结果
}
```

#### 亲和性验证脚本
```bash
#!/bin/bash
# verify_affinity.sh
# 验证容器的 CPU 亲和性是否符合预期

verify_affinity() {
    local pod_name=$1
    local expected_topology=$2

    # 检查容器的 cpuset 配置
    # 验证是否符合拓扑感知策略
    # 返回验证结果
}
```