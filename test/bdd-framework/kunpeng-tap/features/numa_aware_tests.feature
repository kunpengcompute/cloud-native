Feature: NUMA-Aware 策略 CPU 亲和性测试
  测试容器在 numa-aware 策略下的 CPU 亲和性行为
  主要测试单 NUMA 和跨 NUMA 跨 SOCKET 场景

  Background:
    Given 集群有 4numa_24 配置的节点

  # ============================================================================
  # 单 NUMA 场景测试
  # ============================================================================

  @numa_aware @single_numa @4n_01
  Scenario: 多核容器NUMA内亲和 - 4N-01
    When 我创建 1 个 guaranteed 类型的 Deployment
    And Deployment 具有 4/4 CPU 资源配置
    And Deployment 具有标签 "test-4n-01"
    Then 容器应该被成功调度
    And 容器的 CPU 应该分配在 NUMA 级别
    And 容器应该分布在 1 个 NUMA 节点
    And 容器的 QoS 类别应该是 Guaranteed

  @numa_aware @single_numa @4n_02
  Scenario: 半NUMA容器亲和 - 4N-02
    When 我创建 1 个 guaranteed 类型的 Deployment
    And Deployment 具有 12/12 CPU 资源配置
    And Deployment 具有标签 "test-4n-02"
    Then 容器应该被成功调度
    And 容器的 CPU 应该分配在 NUMA 级别
    And 容器应该分布在 1 个 NUMA 节点
    And 容器的 QoS 类别应该是 Guaranteed

  @numa_aware @single_numa @4n_03
  Scenario: 接近满NUMA亲和 - 4N-03
    When 我创建 1 个 guaranteed 类型的 Deployment
    And Deployment 具有 23/23 CPU 资源配置
    And Deployment 具有标签 "test-4n-03"
    Then 容器应该被成功调度
    And 容器的 CPU 应该分配在 NUMA 级别
    And 容器应该分布在 1 个 NUMA 节点
    And 容器的 QoS 类别应该是 Guaranteed

  @numa_aware @single_numa @4n_04
  Scenario: 满NUMA亲和 - 4N-04
    When 我创建 1 个 guaranteed 类型的 Deployment
    And Deployment 具有 24/24 CPU 资源配置
    And Deployment 具有标签 "test-4n-04"
    Then 容器应该被成功调度
    And 容器的 CPU 应该分配在 NUMA 级别
    And 容器应该分布在 1 个 NUMA 节点
    And 容器的 QoS 类别应该是 Guaranteed

  @numa_aware @single_numa @burstable @4n_11
  Scenario: 小数点资源请求 - 4N-11
    When 我创建 1 个 burstable 类型的 Deployment
    And Deployment 具有 0.5/1 CPU 资源配置
    And Deployment 具有标签 "test-4n-11"
    Then 容器应该被成功调度
    And 容器的 CPU 应该分配在 NUMA 级别
    And 容器应该分布在 1 个 NUMA 节点
    And 容器的 QoS 类别应该是 Burstable

  @numa_aware @single_numa @burstable @4n_12
  Scenario: 弹性资源请求 - 4N-12
    When 我创建 1 个 burstable 类型的 Deployment
    And Deployment 具有 2/4 CPU 资源配置
    And Deployment 具有标签 "test-4n-12"
    Then 容器应该被成功调度
    And 容器的 CPU 应该分配在 NUMA 级别
    And 容器应该分布在 1 个 NUMA 节点
    And 容器的 QoS 类别应该是 Burstable

  # ============================================================================
  # 跨 NUMA 跨 SOCKET 场景测试
  # numa-aware 策略：NUMA 亲和失败后直接升级到全局（不应用亲和性）
  # ============================================================================

  @numa_aware @cross_socket @no_affinity @4n_08
  Scenario: 跨SOCKET调度 - 4N-08
    When 我创建 1 个 guaranteed 类型的 Deployment
    And Deployment 具有 49/49 CPU 资源配置
    And Deployment 具有标签 "test-4n-08"
    Then 容器应该被成功调度
    And 容器不应该应用拓扑亲和性
    And 容器的 QoS 类别应该是 Guaranteed

  @numa_aware @cross_socket @no_affinity @4n_09
  Scenario: 大容器跨SOCKET - 4N-09
    When 我创建 1 个 guaranteed 类型的 Deployment
    And Deployment 具有 72/72 CPU 资源配置
    And Deployment 具有标签 "test-4n-09"
    Then 容器应该被成功调度
    And 容器不应该应用拓扑亲和性
    And 容器的 QoS 类别应该是 Guaranteed

  @numa_aware @cross_socket @no_affinity @4n_10
  Scenario: 满机器资源 - 4N-10
    When 我创建 1 个 guaranteed 类型的 Deployment
    And Deployment 具有 90/90 CPU 资源配置
    And Deployment 具有标签 "test-4n-10"
    Then 容器应该被成功调度
    And 容器不应该应用拓扑亲和性
    And 容器的 QoS 类别应该是 Guaranteed

  # ============================================================================
  # BestEffort QoS 测试（不应该有亲和性）
  # ============================================================================

  @numa_aware @besteffort @no_affinity
  Scenario: BestEffort容器不应用亲和性
    When 我创建 1 个 besteffort 类型的 Deployment
    And Deployment 具有标签 "test-besteffort"
    Then 容器应该被成功调度
    And 容器不应该应用拓扑亲和性
    And 容器的 QoS 类别应该是 BestEffort

  # ============================================================================
  # 多副本部署测试
  # ============================================================================

  @numa_aware @multi_replica @load_balance
  Scenario: 多副本容器NUMA负载均衡
    When 我创建 4 个 guaranteed 类型的 Deployment
    And Deployment 具有 8/8 CPU 资源配置
    And Deployment 具有标签 "test-multi-replica"
    Then 容器应该被成功调度
    And 容器的 CPU 应该分配在 NUMA 级别
    And 容器在不同 NUMA 上分配应该均匀

  @numa_aware @multi_replica @single_numa
  Scenario: 多副本小容器单NUMA亲和
    When 我创建 8 个 guaranteed 类型的 Deployment
    And Deployment 具有 2/2 CPU 资源配置
    And Deployment 具有标签 "test-multi-small"
    Then 容器应该被成功调度
    And 容器的 CPU 应该分配在 NUMA 级别

  @numa_aware @multi_replica @cross_socket @no_affinity
  Scenario: 多副本大容器跨SOCKET
    When 我创建 2 个 guaranteed 类型的 Deployment
    And Deployment 具有 40/40 CPU 资源配置
    And Deployment 具有标签 "test-multi-large"
    Then 容器应该被成功调度
    And 容器不应该应用拓扑亲和性

