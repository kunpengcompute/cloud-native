Feature: kunpeng-tap 混合部署测试
  作为系统管理员
  我希望验证在已有容器占用资源的情况下，新容器的拓扑亲和行为
  以确保资源竞争场景下的负载均衡和亲和性策略

  # 2NUMA Guaranteed QoS 混合部署测试
  @mixed_deployment @2numa @guaranteed_qos @numa_affinity @resource_competition
  Scenario: 小资源在已占用NUMA内亲和 - MIX-2N-01
    Given 集群有 2numa_48 配置的节点

    # 创建第一个已有容器
    When 我创建 1 个 guaranteed 类型的 Deployment
    And Deployment 具有 20/20 CPU 资源配置
    And Deployment 具有 40Gi/40Gi Memory 资源配置
    And Deployment 具有标签 "test-MIX-2N-01-existing-1"
    Then 容器应该被成功调度

    # 创建第二个已有容器
    When 我创建 1 个 guaranteed 类型的 Deployment
    And Deployment 具有 20/20 CPU 资源配置
    And Deployment 具有 40Gi/40Gi Memory 资源配置
    And Deployment 具有标签 "test-MIX-2N-01-existing-2"
    Then 容器应该被成功调度

    # 创建新容器（测试目标）
    When 我创建 1 个 guaranteed 类型的 Deployment
    And Deployment 具有 4/4 CPU 资源配置
    And Deployment 具有 8Gi/8Gi Memory 资源配置
    And Deployment 具有标签 "test-MIX-2N-01"
    Then 容器应该被成功调度
    And 容器的 CPU 应该分配在 NUMA 级别

  @mixed_deployment @2numa @guaranteed_qos @numa_affinity @load_balancing
  Scenario: 大资源选择剩余较多NUMA - MIX-2N-02
    Given 集群有 2numa_48 配置的节点

    # 创建第一个已有容器
    When 我创建 1 个 guaranteed 类型的 Deployment
    And Deployment 具有 40/40 CPU 资源配置
    And Deployment 具有 80Gi/80Gi Memory 资源配置
    And Deployment 具有标签 "test-MIX-2N-02-existing-1"
    Then 容器应该被成功调度

    # 创建第二个已有容器
    When 我创建 1 个 guaranteed 类型的 Deployment
    And Deployment 具有 30/30 CPU 资源配置
    And Deployment 具有 60Gi/60Gi Memory 资源配置
    And Deployment 具有标签 "test-MIX-2N-02-existing-2"
    Then 容器应该被成功调度

    # 创建新容器（测试目标）
    When 我创建 1 个 guaranteed 类型的 Deployment
    And Deployment 具有 20/20 CPU 资源配置
    And Deployment 具有 40Gi/40Gi Memory 资源配置
    And Deployment 具有标签 "test-MIX-2N-02"
    Then 容器应该被成功调度
    And 容器的 CPU 应该分配在 NUMA 级别
    And 容器应该调度到 numa1

  @mixed_deployment @2numa @guaranteed_qos @cross_numa @resource_shortage
  Scenario: 资源不足时跨NUMA调度 - MIX-2N-04
    Given 集群有 2numa_48 配置的节点

    # 创建第一个已有容器
    When 我创建 1 个 guaranteed 类型的 Deployment
    And Deployment 具有 45/45 CPU 资源配置
    And Deployment 具有 90Gi/90Gi Memory 资源配置
    And Deployment 具有标签 "test-MIX-2N-04-existing-1"
    Then 容器应该被成功调度

    # 创建第二个已有容器
    When 我创建 1 个 guaranteed 类型的 Deployment
    And Deployment 具有 45/45 CPU 资源配置
    And Deployment 具有 90Gi/90Gi Memory 资源配置
    And Deployment 具有标签 "test-MIX-2N-04-existing-2"
    Then 容器应该被成功调度

    # 创建新容器（测试目标）
    When 我创建 1 个 guaranteed 类型的 Deployment
    And Deployment 具有 8/8 CPU 资源配置
    And Deployment 具有 16Gi/16Gi Memory 资源配置
    And Deployment 具有标签 "test-MIX-2N-04"
    Then 容器应该被成功调度
    And 容器的 CPU 应该跨 NUMA 分配

  @mixed_deployment @2numa @guaranteed_qos @cross_numa @large_container
  Scenario: 大容器必须跨NUMA - MIX-2N-05
    Given 集群有 2numa_48 配置的节点

    # 创建第一个已有容器
    When 我创建 1 个 guaranteed 类型的 Deployment
    And Deployment 具有 10/10 CPU 资源配置
    And Deployment 具有 20Gi/20Gi Memory 资源配置
    And Deployment 具有标签 "test-MIX-2N-05-existing-1"
    Then 容器应该被成功调度

    # 创建第二个已有容器
    When 我创建 1 个 guaranteed 类型的 Deployment
    And Deployment 具有 10/10 CPU 资源配置
    And Deployment 具有 20Gi/20Gi Memory 资源配置
    And Deployment 具有标签 "test-MIX-2N-05-existing-2"
    Then 容器应该被成功调度

    # 创建新容器（测试目标）
    When 我创建 1 个 guaranteed 类型的 Deployment
    And Deployment 具有 49/49 CPU 资源配置
    And Deployment 具有 98Gi/98Gi Memory 资源配置
    And Deployment 具有标签 "test-MIX-2N-05"
    Then 容器应该被成功调度
    And 容器的 CPU 应该跨 NUMA 分配

  @mixed_deployment @2numa @guaranteed_qos @numa_switch @resource_shortage
  Scenario: 单NUMA剩余不足切换NUMA - MIX-2N-06
    Given 集群有 2numa_48 配置的节点

    # 创建第一个已有容器
    When 我创建 1 个 guaranteed 类型的 Deployment
    And Deployment 具有 49/49 CPU 资源配置
    And Deployment 具有 98Gi/98Gi Memory 资源配置
    And Deployment 具有标签 "test-MIX-2N-06-existing-1"
    Then 容器应该被成功调度

    # 创建新容器（测试目标）
    When 我创建 1 个 guaranteed 类型的 Deployment
    And Deployment 具有 10/10 CPU 资源配置
    And Deployment 具有 20Gi/20Gi Memory 资源配置
    And Deployment 具有标签 "test-MIX-2N-06"
    Then 容器应该被成功调度
    And 容器的 CPU 应该分配在 NUMA 级别
    And 容器应该调度到 numa1

  # 2NUMA Burstable QoS 混合部署测试
  @mixed_deployment @2numa @burstable_qos @numa_affinity @resource_competition
  Scenario: Burstable中等资源NUMA亲和 - MIX-2N-07
    Given 集群有 2numa_48 配置的节点

    # 创建第一个已有容器
    When 我创建 1 个 burstable 类型的 Deployment
    And Deployment 具有 10/20 CPU 资源配置
    And Deployment 具有 20Gi/40Gi Memory 资源配置
    And Deployment 具有标签 "test-MIX-2N-07-existing-1"
    Then 容器应该被成功调度

    # 创建第二个已有容器
    When 我创建 1 个 burstable 类型的 Deployment
    And Deployment 具有 10/20 CPU 资源配置
    And Deployment 具有 20Gi/40Gi Memory 资源配置
    And Deployment 具有标签 "test-MIX-2N-07-existing-2"
    Then 容器应该被成功调度

    # 创建新容器（测试目标）
    When 我创建 1 个 burstable 类型的 Deployment
    And Deployment 具有 5/10 CPU 资源配置
    And Deployment 具有 10Gi/20Gi Memory 资源配置
    And Deployment 具有标签 "test-MIX-2N-07"
    Then 容器应该被成功调度
    And 容器的 CPU 应该分配在 NUMA 级别

  @mixed_deployment @2numa @burstable_qos @numa_affinity @load_balancing
  Scenario: Burstable大资源选择剩余较多NUMA - MIX-2N-08
    Given 集群有 2numa_48 配置的节点

    # 创建第一个已有容器
    When 我创建 1 个 burstable 类型的 Deployment
    And Deployment 具有 10/40 CPU 资源配置
    And Deployment 具有 20Gi/80Gi Memory 资源配置
    And Deployment 具有标签 "test-MIX-2N-08-existing-1"
    Then 容器应该被成功调度

    # 创建第二个已有容器
    When 我创建 1 个 burstable 类型的 Deployment
    And Deployment 具有 10/30 CPU 资源配置
    And Deployment 具有 20Gi/60Gi Memory 资源配置
    And Deployment 具有标签 "test-MIX-2N-08-existing-2"
    Then 容器应该被成功调度

    # 创建新容器（测试目标）
    When 我创建 1 个 burstable 类型的 Deployment
    And Deployment 具有 10/20 CPU 资源配置
    And Deployment 具有 20Gi/40Gi Memory 资源配置
    And Deployment 具有标签 "test-MIX-2N-08"
    Then 容器应该被成功调度
    And 容器的 CPU 应该分配在 NUMA 级别
    And 容器应该调度到 numa1

  @mixed_deployment @2numa @burstable_qos @numa_affinity @request_limit_diff
  Scenario: Burstable requests充足limits超出 - MIX-2N-09
    Given 集群有 2numa_48 配置的节点

    # 创建第一个已有容器
    When 我创建 1 个 burstable 类型的 Deployment
    And Deployment 具有 10/45 CPU 资源配置
    And Deployment 具有 20Gi/90Gi Memory 资源配置
    And Deployment 具有标签 "test-MIX-2N-09-existing-1"
    Then 容器应该被成功调度

    # 创建第二个已有容器
    When 我创建 1 个 burstable 类型的 Deployment
    And Deployment 具有 10/45 CPU 资源配置
    And Deployment 具有 20Gi/90Gi Memory 资源配置
    And Deployment 具有标签 "test-MIX-2N-09-existing-2"
    Then 容器应该被成功调度

    # 创建新容器（测试目标）
    When 我创建 1 个 burstable 类型的 Deployment
    And Deployment 具有 4/8 CPU 资源配置
    And Deployment 具有 8Gi/16Gi Memory 资源配置
    And Deployment 具有标签 "test-MIX-2N-09"
    Then 容器应该被成功调度
    And 容器的 CPU 应该分配在 NUMA 级别

  @mixed_deployment @2numa @burstable_qos @cross_numa @large_container
  Scenario: Burstable大容器必须跨NUMA - MIX-2N-10
    Given 集群有 2numa_48 配置的节点

    # 创建第一个已有容器
    When 我创建 1 个 burstable 类型的 Deployment
    And Deployment 具有 5/10 CPU 资源配置
    And Deployment 具有 10Gi/20Gi Memory 资源配置
    And Deployment 具有标签 "test-MIX-2N-10-existing-1"
    Then 容器应该被成功调度

    # 创建第二个已有容器
    When 我创建 1 个 burstable 类型的 Deployment
    And Deployment 具有 5/10 CPU 资源配置
    And Deployment 具有 10Gi/20Gi Memory 资源配置
    And Deployment 具有标签 "test-MIX-2N-10-existing-2"
    Then 容器应该被成功调度

    # 创建新容器（测试目标）
    When 我创建 1 个 burstable 类型的 Deployment
    And Deployment 具有 25/49 CPU 资源配置
    And Deployment 具有 50Gi/98Gi Memory 资源配置
    And Deployment 具有标签 "test-MIX-2N-10"
    Then 容器应该被成功调度
    And 容器的 CPU 应该跨 NUMA 分配

  # 4NUMA Guaranteed QoS 混合部署测试
  @mixed_deployment @4numa @guaranteed_qos @numa_affinity @load_balancing
  Scenario: 小资源在剩余最多NUMA亲和 - MIX-4N-01
    Given 集群有 4numa_24 配置的节点

    # 创建第一个已有容器
    When 我创建 1 个 guaranteed 类型的 Deployment
    And Deployment 具有 20/20 CPU 资源配置
    And Deployment 具有 40Gi/40Gi Memory 资源配置
    And Deployment 具有标签 "test-MIX-4N-01-existing-1"
    Then 容器应该被成功调度

    # 创建第二个已有容器
    When 我创建 1 个 guaranteed 类型的 Deployment
    And Deployment 具有 15/15 CPU 资源配置
    And Deployment 具有 30Gi/30Gi Memory 资源配置
    And Deployment 具有标签 "test-MIX-4N-01-existing-2"
    Then 容器应该被成功调度

    # 创建新容器（测试目标）
    When 我创建 1 个 guaranteed 类型的 Deployment
    And Deployment 具有 4/4 CPU 资源配置
    And Deployment 具有 8Gi/8Gi Memory 资源配置
    And Deployment 具有标签 "test-MIX-4N-01"
    Then 容器应该被成功调度
    And 容器的 CPU 应该分配在 NUMA 级别
    And 容器应该调度到 numa2,numa3

  @mixed_deployment @4numa @guaranteed_qos @numa_affinity @avoid_occupied
  Scenario: 中等资源选择空闲NUMA - MIX-4N-02
    Given 集群有 4numa_24 配置的节点

    # 创建第一个已有容器
    When 我创建 1 个 guaranteed 类型的 Deployment
    And Deployment 具有 20/20 CPU 资源配置
    And Deployment 具有 40Gi/40Gi Memory 资源配置
    And Deployment 具有标签 "test-MIX-4N-02-existing-1"
    Then 容器应该被成功调度

    # 创建第二个已有容器
    When 我创建 1 个 guaranteed 类型的 Deployment
    And Deployment 具有 20/20 CPU 资源配置
    And Deployment 具有 40Gi/40Gi Memory 资源配置
    And Deployment 具有标签 "test-MIX-4N-02-existing-2"
    Then 容器应该被成功调度

    # 创建新容器（测试目标）
    When 我创建 1 个 guaranteed 类型的 Deployment
    And Deployment 具有 8/8 CPU 资源配置
    And Deployment 具有 16Gi/16Gi Memory 资源配置
    And Deployment 具有标签 "test-MIX-4N-02"
    Then 容器应该被成功调度
    And 容器的 CPU 应该分配在 NUMA 级别
    And 容器应该调度到 numa2,numa3

  @mixed_deployment @4numa @guaranteed_qos @socket_affinity @cross_numa
  Scenario: 大资源在同SOCKET内跨NUMA - MIX-4N-03
    Given 集群有 4numa_24 配置的节点

    # 创建第一个已有容器
    When 我创建 1 个 guaranteed 类型的 Deployment
    And Deployment 具有 10/10 CPU 资源配置
    And Deployment 具有 20Gi/20Gi Memory 资源配置
    And Deployment 具有标签 "test-MIX-4N-03-existing-1"
    Then 容器应该被成功调度

    # 创建第二个已有容器
    When 我创建 1 个 guaranteed 类型的 Deployment
    And Deployment 具有 10/10 CPU 资源配置
    And Deployment 具有 20Gi/20Gi Memory 资源配置
    And Deployment 具有标签 "test-MIX-4N-03-existing-2"
    Then 容器应该被成功调度

    # 创建第三个已有容器
    When 我创建 1 个 guaranteed 类型的 Deployment
    And Deployment 具有 10/10 CPU 资源配置
    And Deployment 具有 20Gi/20Gi Memory 资源配置
    And Deployment 具有标签 "test-MIX-4N-03-existing-3"
    Then 容器应该被成功调度

    # 创建第四个已有容器
    When 我创建 1 个 guaranteed 类型的 Deployment
    And Deployment 具有 10/10 CPU 资源配置
    And Deployment 具有 20Gi/20Gi Memory 资源配置
    And Deployment 具有标签 "test-MIX-4N-03-existing-4"
    Then 容器应该被成功调度

    # 创建新容器（测试目标）
    When 我创建 1 个 guaranteed 类型的 Deployment
    And Deployment 具有 20/20 CPU 资源配置
    And Deployment 具有 40Gi/40Gi Memory 资源配置
    And Deployment 具有标签 "test-MIX-4N-03"
    Then 容器应该被成功调度
    And 容器应该分布在 1 个 SOCKET

  @mixed_deployment @4numa @guaranteed_qos @socket_switch @cross_numa
  Scenario: 大资源切换到空闲SOCKET - MIX-4N-04
    Given 集群有 4numa_24 配置的节点

    # 创建第一个已有容器
    When 我创建 1 个 guaranteed 类型的 Deployment
    And Deployment 具有 20/20 CPU 资源配置
    And Deployment 具有 40Gi/40Gi Memory 资源配置
    And Deployment 具有标签 "test-MIX-4N-04-existing-1"
    Then 容器应该被成功调度

    # 创建第二个已有容器
    When 我创建 1 个 guaranteed 类型的 Deployment
    And Deployment 具有 20/20 CPU 资源配置
    And Deployment 具有 40Gi/40Gi Memory 资源配置
    And Deployment 具有标签 "test-MIX-4N-04-existing-2"
    Then 容器应该被成功调度

    # 创建新容器（测试目标）
    When 我创建 1 个 guaranteed 类型的 Deployment
    And Deployment 具有 30/30 CPU 资源配置
    And Deployment 具有 60Gi/60Gi Memory 资源配置
    And Deployment 具有标签 "test-MIX-4N-04"
    Then 容器应该被成功调度
    And 容器应该分布在 1 个 SOCKET

  @mixed_deployment @4numa @guaranteed_qos @cross_socket @large_container
  Scenario: 超大资源必须跨SOCKET - MIX-4N-05
    Given 集群有 4numa_24 配置的节点

    # 创建第一个已有容器
    When 我创建 1 个 guaranteed 类型的 Deployment
    And Deployment 具有 10/10 CPU 资源配置
    And Deployment 具有 40Gi/40Gi Memory 资源配置
    And Deployment 具有标签 "test-MIX-4N-05-existing-1"
    Then 容器应该被成功调度

    # 创建第二个已有容器
    When 我创建 1 个 guaranteed 类型的 Deployment
    And Deployment 具有 10/10 CPU 资源配置
    And Deployment 具有 40Gi/40Gi Memory 资源配置
    And Deployment 具有标签 "test-MIX-4N-05-existing-2"
    Then 容器应该被成功调度

    # 创建第三个已有容器
    When 我创建 1 个 guaranteed 类型的 Deployment
    And Deployment 具有 10/10 CPU 资源配置
    And Deployment 具有 40Gi/40Gi Memory 资源配置
    And Deployment 具有标签 "test-MIX-4N-05-existing-3"
    Then 容器应该被成功调度

    # 创建第四个已有容器
    When 我创建 1 个 guaranteed 类型的 Deployment
    And Deployment 具有 10/10 CPU 资源配置
    And Deployment 具有 40Gi/40Gi Memory 资源配置
    And Deployment 具有标签 "test-MIX-4N-05-existing-4"
    Then 容器应该被成功调度

    # 创建新容器（测试目标）
    When 我创建 1 个 guaranteed 类型的 Deployment
    And Deployment 具有 50/50 CPU 资源配置
    And Deployment 具有 100Gi/100Gi Memory 资源配置
    And Deployment 具有标签 "test-MIX-4N-05"
    Then 容器应该被成功调度
    And 容器应该分布在 2 个 SOCKET

  # 4NUMA Burstable QoS 混合部署测试
  @mixed_deployment @4numa @burstable_qos @numa_affinity @resource_competition
  Scenario: Burstable中等资源NUMA亲和 - MIX-4N-06
    Given 集群有 4numa_24 配置的节点

    # 创建第一个已有容器
    When 我创建 1 个 burstable 类型的 Deployment
    And Deployment 具有 10/20 CPU 资源配置
    And Deployment 具有 20Gi/40Gi Memory 资源配置
    And Deployment 具有标签 "test-MIX-4N-06-existing-1"
    Then 容器应该被成功调度

    # 创建第二个已有容器
    When 我创建 1 个 burstable 类型的 Deployment
    And Deployment 具有 10/20 CPU 资源配置
    And Deployment 具有 20Gi/40Gi Memory 资源配置
    And Deployment 具有标签 "test-MIX-4N-06-existing-2"
    Then 容器应该被成功调度

    # 创建新容器（测试目标）
    When 我创建 1 个 burstable 类型的 Deployment
    And Deployment 具有 5/10 CPU 资源配置
    And Deployment 具有 10Gi/20Gi Memory 资源配置
    And Deployment 具有标签 "test-MIX-4N-06"
    Then 容器应该被成功调度
    And 容器的 CPU 应该分配在 NUMA 级别

  @mixed_deployment @4numa @burstable_qos @numa_affinity @load_balancing
  Scenario: Burstable大资源选择剩余较多NUMA - MIX-4N-07
    Given 集群有 4numa_24 配置的节点

    # 创建第一个已有容器
    When 我创建 1 个 burstable 类型的 Deployment
    And Deployment 具有 10/40 CPU 资源配置
    And Deployment 具有 20Gi/80Gi Memory 资源配置
    And Deployment 具有标签 "test-MIX-4N-07-existing-1"
    Then 容器应该被成功调度

    # 创建第二个已有容器
    When 我创建 1 个 burstable 类型的 Deployment
    And Deployment 具有 10/30 CPU 资源配置
    And Deployment 具有 20Gi/60Gi Memory 资源配置
    And Deployment 具有标签 "test-MIX-4N-07-existing-2"
    Then 容器应该被成功调度

    # 创建新容器（测试目标）
    # 已有容器的分配顺序取决于策略实现（深度优先+容量优先），
    # 不能假设一定分配在特定NUMA上，只验证新容器分配在NUMA级别
    When 我创建 1 个 burstable 类型的 Deployment
    And Deployment 具有 10/20 CPU 资源配置
    And Deployment 具有 20Gi/40Gi Memory 资源配置
    And Deployment 具有标签 "test-MIX-4N-07"
    Then 容器应该被成功调度
    And 容器的 CPU 应该分配在 NUMA 级别

  @mixed_deployment @4numa @burstable_qos @numa_affinity @request_limit_diff
  Scenario: Burstable requests充足limits超出 - MIX-4N-08
    Given 集群有 4numa_24 配置的节点

    # 创建第一个已有容器
    When 我创建 1 个 burstable 类型的 Deployment
    And Deployment 具有 10/45 CPU 资源配置
    And Deployment 具有 20Gi/90Gi Memory 资源配置
    And Deployment 具有标签 "test-MIX-4N-08-existing-1"
    Then 容器应该被成功调度

    # 创建第二个已有容器
    When 我创建 1 个 burstable 类型的 Deployment
    And Deployment 具有 10/45 CPU 资源配置
    And Deployment 具有 20Gi/90Gi Memory 资源配置
    And Deployment 具有标签 "test-MIX-4N-08-existing-2"
    Then 容器应该被成功调度

    # 创建新容器（测试目标）
    When 我创建 1 个 burstable 类型的 Deployment
    And Deployment 具有 4/8 CPU 资源配置
    And Deployment 具有 8Gi/16Gi Memory 资源配置
    And Deployment 具有标签 "test-MIX-4N-08"
    Then 容器应该被成功调度
    And 容器的 CPU 应该分配在 NUMA 级别

  @mixed_deployment @4numa @burstable_qos @cross_numa @large_container
  Scenario: Burstable大容器必须跨NUMA - MIX-4N-09
    Given 集群有 4numa_24 配置的节点

    # 创建第一个已有容器
    When 我创建 1 个 burstable 类型的 Deployment
    And Deployment 具有 5/10 CPU 资源配置
    And Deployment 具有 10Gi/20Gi Memory 资源配置
    And Deployment 具有标签 "test-MIX-4N-09-existing-1"
    Then 容器应该被成功调度

    # 创建第二个已有容器
    When 我创建 1 个 burstable 类型的 Deployment
    And Deployment 具有 5/10 CPU 资源配置
    And Deployment 具有 10Gi/20Gi Memory 资源配置
    And Deployment 具有标签 "test-MIX-4N-09-existing-2"
    Then 容器应该被成功调度

    # 创建新容器（测试目标,request超出numa范围，limit在Socket范围内）
    When 我创建 1 个 burstable 类型的 Deployment
    And Deployment 具有 25/40 CPU 资源配置
    And Deployment 具有 50Gi/98Gi Memory 资源配置
    And Deployment 具有标签 "test-MIX-4N-09"
    Then 容器应该被成功调度
    And 容器的 CPU 应该分配在 SOCKET 级别

  @mixed_deployment @4numa @burstable_qos @cross_numa @large_container
  Scenario: Burstable大容器跨NUMA同SOCKET - MIX-4N-10
    Given 集群有 4numa_24 配置的节点

    # 创建第一个已有容器
    When 我创建 1 个 burstable 类型的 Deployment
    And Deployment 具有 5/10 CPU 资源配置
    And Deployment 具有 10Gi/20Gi Memory 资源配置
    And Deployment 具有标签 "test-MIX-4N-10-existing-1"
    Then 容器应该被成功调度

    # 创建第二个已有容器
    When 我创建 1 个 burstable 类型的 Deployment
    And Deployment 具有 5/10 CPU 资源配置
    And Deployment 具有 10Gi/20Gi Memory 资源配置
    And Deployment 具有标签 "test-MIX-4N-10-existing-2"
    Then 容器应该被成功调度

    # 创建新容器（测试目标,request=25超出单NUMA=24，但不超出Socket=48）
    # 按request过滤：25>24所以NUMA不通过，25<48所以Socket通过
    When 我创建 1 个 burstable 类型的 Deployment
    And Deployment 具有 25/49 CPU 资源配置
    And Deployment 具有 50Gi/98Gi Memory 资源配置
    And Deployment 具有标签 "test-MIX-4N-10"
    Then 容器应该被成功调度
    And 容器的 CPU 应该分配在 SOCKET 级别

  # Socket 亲和后的资源竞争测试
  @mixed_deployment @4numa @guaranteed_qos @socket_affinity @load_balancing
  Scenario: Socket亲和后NUMA负载均衡 - MIX-SK-GA-01
    Given 集群有 4numa_24 配置的节点

    # 创建第一个已有容器
    When 我创建 1 个 guaranteed 类型的 Deployment
    And Deployment 具有 40/40 CPU 资源配置
    And Deployment 具有 80Gi/80Gi Memory 资源配置
    And Deployment 具有标签 "test-MIX-SK-GA-01-existing-1"
    Then 容器应该被成功调度

    # 创建新容器（测试目标）
    When 我创建 1 个 guaranteed 类型的 Deployment
    And Deployment 具有 4/4 CPU 资源配置
    And Deployment 具有 8Gi/8Gi Memory 资源配置
    And Deployment 具有标签 "test-MIX-SK-GA-01"
    Then 容器应该被成功调度
    And 容器的 CPU 应该分配在 NUMA 级别

  @mixed_deployment @4numa @burstable_qos @socket_affinity @load_balancing
  Scenario: Burstable Socket亲和后NUMA负载均衡 - MIX-SK-BU-01
    Given 集群有 4numa_24 配置的节点

    # 创建第一个已有容器
    When 我创建 1 个 burstable 类型的 Deployment
    And Deployment 具有 40/40 CPU 资源配置
    And Deployment 具有 80Gi/80Gi Memory 资源配置
    And Deployment 具有标签 "test-MIX-SK-BU-01-existing-1"
    Then 容器应该被成功调度

    # 创建第二个已有容器
    When 我创建 1 个 burstable 类型的 Deployment
    And Deployment 具有 40/40 CPU 资源配置
    And Deployment 具有 80Gi/80Gi Memory 资源配置
    And Deployment 具有标签 "test-MIX-SK-BU-01-existing-2"
    Then 容器应该被成功调度

    # 创建新容器（测试目标）
    When 我创建 1 个 burstable 类型的 Deployment
    And Deployment 具有 4/8 CPU 资源配置
    And Deployment 具有 8Gi/16Gi Memory 资源配置
    And Deployment 具有标签 "test-MIX-SK-BU-01"
    Then 容器应该被成功调度
    And 容器的 CPU 应该分配在 NUMA 级别
