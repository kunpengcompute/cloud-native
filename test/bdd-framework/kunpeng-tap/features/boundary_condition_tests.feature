Feature: kunpeng-tap 边界条件测试
  作为系统管理员
  我希望验证极端资源请求和边界情况下的调度行为
  以确保系统在边界条件下的稳定性和正确性

  # 边界条件测试
  @boundary_condition @min_resource @guaranteed_qos
  Scenario: 最小资源请求 - BC-01
    Given 集群有 2numa_48 配置的节点
    When 我创建 1 个 guaranteed 类型的 Deployment
    And Deployment 具有 0.1/0.1 CPU 资源配置
    And Deployment 具有标签 "test-BC-01"
    Then 容器应该被成功调度
    And 容器不应该应用拓扑亲和性

  @boundary_condition @resource_overflow @guaranteed_qos @2numa
  Scenario: 超出2NUMA机器容量 - BC-02
    Given 集群有 2numa_48 配置的节点
    When 我创建 1 个 guaranteed 类型的 Deployment
    And Deployment 具有 97/97 CPU 资源配置
    And Deployment 具有标签 "test-BC-02"
    Then 容器调度应该失败
    And 容器状态应该是 Pending

  @boundary_condition @resource_overflow @guaranteed_qos @4numa
  Scenario: 超出4NUMA机器容量 - BC-03
    Given 集群有 4numa_80 配置的节点
    When 我创建 1 个 guaranteed 类型的 Deployment
    And Deployment 具有 321/321 CPU 资源配置
    And Deployment 具有标签 "test-BC-03"
    Then 容器调度应该失败
    And 容器状态应该是 Pending

  @boundary_condition @millicores @burstable_qos @unit_conversion
  Scenario: 毫核单位资源请求 - BC-04
    Given 集群有 2numa_48 配置的节点
    When 我创建 1 个 burstable 类型的 Deployment
    And Deployment 具有 1000m/2000m CPU 资源配置
    And Deployment 具有标签 "test-BC-04"
    Then 容器应该被成功调度
    And 容器不应该应用拓扑亲和性

  @boundary_condition @decimal_cpu @guaranteed_qos
  Scenario: 小数点CPU资源请求 - BC-05
    Given 集群有 2numa_48 配置的节点
    When 我创建 1 个 guaranteed 类型的 Deployment
    And Deployment 具有 1.5/1.5 CPU 资源配置
    And Deployment 具有标签 "test-BC-05"
    Then 容器应该被成功调度
    And 容器不应该应用拓扑亲和性

  @boundary_condition @max_numa_capacity @guaranteed_qos @2numa @numa_affinity
  Scenario: 单NUMA最大容量 - BC-06
    Given 集群有 2numa_48 配置的节点
    When 我创建 1 个 guaranteed 类型的 Deployment
    And Deployment 具有 48/48 CPU 资源配置
    And Deployment 具有标签 "test-BC-06"
    Then 容器应该被成功调度
    And 容器的 CPU 应该分配在 NUMA 级别

  @boundary_condition @exceed_numa_by_one @guaranteed_qos @2numa @cross_numa
  Scenario: 单NUMA超出1核 - BC-07
    Given 集群有 2numa_48 配置的节点
    When 我创建 1 个 guaranteed 类型的 Deployment
    And Deployment 具有 49/49 CPU 资源配置
    And Deployment 具有标签 "test-BC-07"
    Then 容器应该被成功调度
    And 容器的 CPU 应该跨 NUMA 分配

  @boundary_condition @max_numa_capacity @guaranteed_qos @4numa @numa_affinity
  Scenario: 4NUMA单NUMA最大容量 - BC-08
    Given 集群有 4numa_24 配置的节点
    When 我创建 1 个 guaranteed 类型的 Deployment
    And Deployment 具有 24/24 CPU 资源配置
    And Deployment 具有标签 "test-BC-08"
    Then 容器应该被成功调度
    And 容器的 CPU 应该分配在 NUMA 级别

  @boundary_condition @max_socket_capacity @guaranteed_qos @4numa @socket_affinity
  Scenario: 4NUMA单Socket最大容量 - BC-09
    Given 集群有 4numa_24 配置的节点
    When 我创建 1 个 guaranteed 类型的 Deployment
    And Deployment 具有 48/48 CPU 资源配置
    And Deployment 具有标签 "test-BC-09"
    Then 容器应该被成功调度
    And 容器的 CPU 应该分配在 SOCKET 级别

  @boundary_condition @besteffort_qos @no_affinity
  Scenario: BestEffort无资源限制 - BC-10
    Given 集群有 2numa_48 配置的节点
    When 我创建 1 个 besteffort 类型的 Deployment
    And Deployment 具有标签 "test-BC-10"
    Then 容器应该被成功调度
    And 容器不应该应用拓扑亲和性

  # 4NUMA 边界条件测试（对齐 2NUMA 测试覆盖）
  @boundary_condition @exceed_numa_by_one @guaranteed_qos @4numa @cross_numa
  Scenario: 4NUMA单NUMA超出1核 - BC-11
    Given 集群有 4numa_24 配置的节点
    When 我创建 1 个 guaranteed 类型的 Deployment
    And Deployment 具有 25/25 CPU 资源配置
    And Deployment 具有标签 "test-BC-11"
    Then 容器应该被成功调度
    And 容器的 CPU 应该跨 NUMA 分配

  @boundary_condition @exceed_socket_by_one @guaranteed_qos @4numa @cross_socket
  Scenario: 4NUMA单Socket超出1核 - BC-12
    Given 集群有 4numa_24 配置的节点
    When 我创建 1 个 guaranteed 类型的 Deployment
    And Deployment 具有 49/49 CPU 资源配置
    And Deployment 具有标签 "test-BC-12"
    Then 容器应该被成功调度
    And 容器的 CPU 应该跨 SOCKET 分配

  @boundary_condition @min_resource @guaranteed_qos @4numa
  Scenario: 4NUMA最小资源请求 - BC-13
    Given 集群有 4numa_24 配置的节点
    When 我创建 1 个 guaranteed 类型的 Deployment
    And Deployment 具有 0.1/0.1 CPU 资源配置
    And Deployment 具有标签 "test-BC-13"
    Then 容器应该被成功调度
    And 容器应该分布在 1 个 NUMA 节点

  @boundary_condition @millicores @burstable_qos @unit_conversion @4numa
  Scenario: 4NUMA毫核单位资源请求 - BC-14
    Given 集群有 4numa_24 配置的节点
    When 我创建 1 个 burstable 类型的 Deployment
    And Deployment 具有 1000m/2000m CPU 资源配置
    And Deployment 具有标签 "test-BC-14"
    Then 容器应该被成功调度
    And 容器应该分布在 1 个 NUMA 节点

  @boundary_condition @decimal_cpu @guaranteed_qos @4numa
  Scenario: 4NUMA小数点CPU资源请求 - BC-15
    Given 集群有 4numa_24 配置的节点
    When 我创建 1 个 guaranteed 类型的 Deployment
    And Deployment 具有 1.5/1.5 CPU 资源配置
    And Deployment 具有标签 "test-BC-15"
    Then 容器应该被成功调度
    And 容器应该分布在 1 个 NUMA 节点
