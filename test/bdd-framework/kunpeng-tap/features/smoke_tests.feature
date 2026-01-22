Feature: kunpeng-tap 冒烟测试
  作为系统管理员
  我希望快速验证 kunpeng-tap 的核心功能是否正常工作
  以确保基本的拓扑感知调度功能可用

  # 冒烟测试 - 快速验证核心功能
  # 执行时间: ~5分钟
  # 覆盖范围: Guaranteed/Burstable/BestEffort QoS, NUMA/Socket 亲和, 批量部署, 混合部署, 边界条件

  @smoke @guaranteed_qos @2numa @numa_affinity @critical
  Scenario: Guaranteed容器NUMA亲和基本验证 - SMOKE-01
    Given 集群有 2numa_48 配置的节点
    When 我创建 1 个 guaranteed 类型的 Deployment
    And Deployment 具有 4/4 CPU 资源配置
    And Deployment 具有标签 "test-SMOKE-01"
    Then 容器应该被成功调度
    And 容器的 CPU 应该分配在 NUMA 级别

  @smoke @burstable_qos @2numa @numa_affinity @critical
  Scenario: Burstable容器NUMA亲和基本验证 - SMOKE-02
    Given 集群有 2numa_48 配置的节点
    When 我创建 1 个 burstable 类型的 Deployment
    And Deployment 具有 2/4 CPU 资源配置
    And Deployment 具有标签 "test-SMOKE-02"
    Then 容器应该被成功调度
    And 容器的 CPU 应该分配在 NUMA 级别

  @smoke @besteffort_qos @2numa @no_affinity @critical
  Scenario: BestEffort容器无亲和验证 - SMOKE-03
    Given 集群有 2numa_48 配置的节点
    When 我创建 1 个 besteffort 类型的 Deployment
    And Deployment 具有标签 "test-SMOKE-03"
    Then 容器应该被成功调度
    And 容器不应该应用拓扑亲和性

  @smoke @guaranteed_qos @2numa @qos_classification @critical
  Scenario: QoS类别分类验证 - SMOKE-10
    Given 集群有 2numa_48 配置的节点
    When 我创建 1 个 guaranteed 类型的 Deployment
    And Deployment 具有 4/4 CPU 资源配置
    And Deployment 具有标签 "test-SMOKE-10"
    Then 容器应该被成功调度
    And 容器的 QoS 类别应该是 Guaranteed

  @smoke @guaranteed_qos @2numa @cross_numa @critical
  Scenario: 大容器跨NUMA调度验证 - SMOKE-04
    Given 集群有 2numa_48 配置的节点
    When 我创建 1 个 guaranteed 类型的 Deployment
    And Deployment 具有 60/60 CPU 资源配置
    And Deployment 具有标签 "test-SMOKE-04"
    Then 容器应该被成功调度
    And 容器的 CPU 应该分配在 NUMA 级别

  @smoke @guaranteed_qos @4numa @numa_affinity @critical
  Scenario: 4NUMA容器基本亲和验证 - SMOKE-05
    Given 集群有 4numa_24 配置的节点
    When 我创建 1 个 guaranteed 类型的 Deployment
    And Deployment 具有 8/8 CPU 资源配置
    And Deployment 具有标签 "test-SMOKE-05"
    Then 容器应该被成功调度
    And 容器的 CPU 应该分配在 NUMA 级别

  @smoke @guaranteed_qos @4numa @socket_affinity @critical
  Scenario: Socket级别亲和验证 - SMOKE-06
    Given 集群有 4numa_24 配置的节点
    When 我创建 1 个 guaranteed 类型的 Deployment
    And Deployment 具有 40/40 CPU 资源配置
    And Deployment 具有标签 "test-SMOKE-06"
    Then 容器应该被成功调度
    And 容器的 CPU 应该分配在 NUMA 级别

  @smoke @bulk_deployment @guaranteed_qos @2numa @numa_balance @critical
  Scenario: 小批量部署NUMA均衡验证 - SMOKE-07
    Given 集群有 2numa_48 配置的节点
    When 我创建 10 个 guaranteed 类型的 Deployment
    And Deployment 具有 2/2 CPU 资源配置
    And Deployment 具有标签 "test-SMOKE-07"
    Then 容器应该被成功调度
    And 容器应该在 NUMA 节点间均衡分布

  @smoke @mixed_deployment @guaranteed_qos @2numa @load_balancing @critical
  Scenario: 资源竞争下的调度验证 - SMOKE-08
    Given 集群有 2numa_48 配置的节点
    And 已有容器1占用 20/20 CPU 资源，QoS为 guaranteed
    When 我创建 1 个 guaranteed 类型的 Deployment
    And Deployment 具有 10/10 CPU 资源配置
    And Deployment 具有标签 "test-SMOKE-08"
    Then 容器应该被成功调度
    And 容器的 CPU 应该分配在 NUMA 级别

  @smoke @boundary_condition @guaranteed_qos @2numa @max_numa_capacity @critical
  Scenario: 单NUMA最大容量验证 - SMOKE-09
    Given 集群有 2numa_48 配置的节点
    When 我创建 1 个 guaranteed 类型的 Deployment
    And Deployment 具有 48/48 CPU 资源配置
    And Deployment 具有标签 "test-SMOKE-09"
    Then 容器应该被成功调度
    And 容器的 CPU 应该分配在 NUMA 级别
