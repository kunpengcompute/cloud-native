Feature: kunpeng-tap Cluster 拓扑感知调度测试
  作为系统管理员
  我希望验证在 950 型号机器上（4NUMA × 96核，每NUMA 6个Cluster，每Cluster 16核）
  kunpeng-tap 的 Cluster 级别拓扑感知调度功能
  以确保不同大小的容器能够根据预期实现 Cluster/NUMA/Socket 级别的亲和

  # ============================================================================
  # 机器拓扑：
  #   Socket 0: NUMA 0 (CPU 0-95,   6 clusters: 0-15, 16-31, 32-47, 48-63, 64-79, 80-95)
  #             NUMA 1 (CPU 96-191,  6 clusters: 96-111, 112-127, ..., 176-191)
  #   Socket 1: NUMA 2 (CPU 192-287, 6 clusters: 192-207, 208-223, ..., 272-287)
  #             NUMA 3 (CPU 288-383, 6 clusters: 288-303, 304-319, ..., 368-383)
  #
  #   Single Cluster = 16 CPUs
  #   Single NUMA    = 96 CPUs (6 clusters)
  #   Single Socket  = 192 CPUs (2 NUMA, 12 clusters)
  #   Whole Machine  = 384 CPUs (4 NUMA, 24 clusters)
  # ============================================================================

  # Guaranteed QoS - Cluster 级别亲和测试
  @cluster_topology @guaranteed_qos @cluster_affinity
  Scenario: 小容器Cluster内亲和 - CL-01
    Given 集群有 4numa_384_cluster 配置的节点
    When 我创建 1 个 guaranteed 类型的 Deployment
    And Deployment 具有 4/4 CPU 资源配置
    And Deployment 具有 8Gi/8Gi Memory 资源配置
    And Deployment 具有标签 "test-CL-01"
    Then 容器应该被成功调度
    And 容器的 CPU 应该分配在 CLUSTER 级别
    And 容器应该分布在 1 个 CLUSTER

  @cluster_topology @guaranteed_qos @cluster_affinity
  Scenario: 半Cluster容器亲和 - CL-02
    Given 集群有 4numa_384_cluster 配置的节点
    When 我创建 1 个 guaranteed 类型的 Deployment
    And Deployment 具有 8/8 CPU 资源配置
    And Deployment 具有 16Gi/16Gi Memory 资源配置
    And Deployment 具有标签 "test-CL-02"
    Then 容器应该被成功调度
    And 容器的 CPU 应该分配在 CLUSTER 级别
    And 容器应该分布在 1 个 CLUSTER

  @cluster_topology @guaranteed_qos @cluster_affinity
  Scenario: 满Cluster容器亲和 - CL-03
    Given 集群有 4numa_384_cluster 配置的节点
    When 我创建 1 个 guaranteed 类型的 Deployment
    And Deployment 具有 16/16 CPU 资源配置
    And Deployment 具有 32Gi/32Gi Memory 资源配置
    And Deployment 具有标签 "test-CL-03"
    Then 容器应该被成功调度
    And 容器的 CPU 应该分配在 CLUSTER 级别
    And 容器应该分布在 1 个 CLUSTER

  # Guaranteed QoS - 跨 Cluster 同 NUMA 测试
  @cluster_topology @guaranteed_qos @cross_cluster @numa_affinity
  Scenario: 跨Cluster同NUMA亲和 - CL-04
    Given 集群有 4numa_384_cluster 配置的节点
    When 我创建 1 个 guaranteed 类型的 Deployment
    And Deployment 具有 20/20 CPU 资源配置
    And Deployment 具有 40Gi/40Gi Memory 资源配置
    And Deployment 具有标签 "test-CL-04"
    Then 容器应该被成功调度
    And 容器的 CPU 应该分配在 NUMA 级别
    And 容器应该分布在 1 个 NUMA 节点

  @cluster_topology @guaranteed_qos @cross_cluster @numa_affinity
  Scenario: 半NUMA容器跨多个Cluster - CL-05
    Given 集群有 4numa_384_cluster 配置的节点
    When 我创建 1 个 guaranteed 类型的 Deployment
    And Deployment 具有 48/48 CPU 资源配置
    And Deployment 具有 64Gi/64Gi Memory 资源配置
    And Deployment 具有标签 "test-CL-05"
    Then 容器应该被成功调度
    And 容器的 CPU 应该分配在 NUMA 级别
    And 容器应该分布在 1 个 NUMA 节点

  @cluster_topology @guaranteed_qos @cross_cluster @numa_affinity
  Scenario: 接近满NUMA容器 - CL-06
    Given 集群有 4numa_384_cluster 配置的节点
    When 我创建 1 个 guaranteed 类型的 Deployment
    And Deployment 具有 90/90 CPU 资源配置
    And Deployment 具有 120Gi/120Gi Memory 资源配置
    And Deployment 具有标签 "test-CL-06"
    Then 容器应该被成功调度
    And 容器的 CPU 应该分配在 NUMA 级别
    And 容器应该分布在 1 个 NUMA 节点

  @cluster_topology @guaranteed_qos @cross_cluster @numa_affinity
  Scenario: 满NUMA容器 - CL-07
    Given 集群有 4numa_384_cluster 配置的节点
    When 我创建 1 个 guaranteed 类型的 Deployment
    And Deployment 具有 96/96 CPU 资源配置
    And Deployment 具有 128Gi/128Gi Memory 资源配置
    And Deployment 具有标签 "test-CL-07"
    Then 容器应该被成功调度
    And 容器的 CPU 应该分配在 NUMA 级别
    And 容器应该分布在 1 个 NUMA 节点

  # Guaranteed QoS - 跨 NUMA 同 Socket 测试
  @cluster_topology @guaranteed_qos @cross_numa @socket_affinity
  Scenario: 跨NUMA同Socket容器 - CL-08
    Given 集群有 4numa_384_cluster 配置的节点
    When 我创建 1 个 guaranteed 类型的 Deployment
    And Deployment 具有 100/100 CPU 资源配置
    And Deployment 具有 140Gi/140Gi Memory 资源配置
    And Deployment 具有标签 "test-CL-08"
    Then 容器应该被成功调度
    And 容器的 CPU 应该分配在 SOCKET 级别
    And 容器应该分布在 1 个 SOCKET

  @cluster_topology @guaranteed_qos @cross_numa @socket_affinity
  Scenario: 满Socket容器 - CL-09
    Given 集群有 4numa_384_cluster 配置的节点
    When 我创建 1 个 guaranteed 类型的 Deployment
    And Deployment 具有 192/192 CPU 资源配置
    And Deployment 具有 256Gi/256Gi Memory 资源配置
    And Deployment 具有标签 "test-CL-09"
    Then 容器应该被成功调度
    And 容器的 CPU 应该分配在 SOCKET 级别
    And 容器应该分布在 1 个 SOCKET

  # Guaranteed QoS - 跨 Socket 测试
  @cluster_topology @guaranteed_qos @cross_socket
  Scenario: 跨Socket容器 - CL-10
    Given 集群有 4numa_384_cluster 配置的节点
    When 我创建 1 个 guaranteed 类型的 Deployment
    And Deployment 具有 200/200 CPU 资源配置
    And Deployment 具有 280Gi/280Gi Memory 资源配置
    And Deployment 具有标签 "test-CL-10"
    Then 容器应该被成功调度
    And 容器的 CPU 应该跨 SOCKET 分配

  @cluster_topology @guaranteed_qos @cross_socket @large_container
  Scenario: 大容器跨Socket - CL-11
    Given 集群有 4numa_384_cluster 配置的节点
    When 我创建 1 个 guaranteed 类型的 Deployment
    And Deployment 具有 300/300 CPU 资源配置
    And Deployment 具有 400Gi/400Gi Memory 资源配置
    And Deployment 具有标签 "test-CL-11"
    Then 容器应该被成功调度
    And 容器的 CPU 应该跨 SOCKET 分配

  @cluster_topology @guaranteed_qos @cross_socket @full_machine
  Scenario: 满机器资源 - CL-12
    Given 集群有 4numa_384_cluster 配置的节点
    When 我创建 1 个 guaranteed 类型的 Deployment
    And Deployment 具有 370/370 CPU 资源配置
    And Deployment 具有 480Gi/480Gi Memory 资源配置
    And Deployment 具有标签 "test-CL-12"
    Then 容器应该被成功调度
    And 容器的 CPU 应该跨 SOCKET 分配
    And 容器应该分布在 4 个 NUMA 节点

  # Burstable QoS - Cluster 级别亲和测试
  @cluster_topology @burstable_qos @cluster_affinity
  Scenario: Burstable小容器Cluster亲和 - CL-13
    Given 集群有 4numa_384_cluster 配置的节点
    When 我创建 1 个 burstable 类型的 Deployment
    And Deployment 具有 4/8 CPU 资源配置
    And Deployment 具有 8Gi/16Gi Memory 资源配置
    And Deployment 具有标签 "test-CL-13"
    Then 容器应该被成功调度
    And 容器的 CPU 应该分配在 CLUSTER 级别
    And 容器应该分布在 1 个 CLUSTER

  @cluster_topology @burstable_qos @cluster_affinity
  Scenario: Burstable满Cluster容器 - CL-14
    Given 集群有 4numa_384_cluster 配置的节点
    When 我创建 1 个 burstable 类型的 Deployment
    And Deployment 具有 8/16 CPU 资源配置
    And Deployment 具有 16Gi/32Gi Memory 资源配置
    And Deployment 具有标签 "test-CL-14"
    Then 容器应该被成功调度
    And 容器的 CPU 应该分配在 CLUSTER 级别
    And 容器应该分布在 1 个 CLUSTER

  # Burstable QoS - 跨 Cluster 同 NUMA 测试
  @cluster_topology @burstable_qos @cross_cluster @numa_affinity
  Scenario: Burstable跨Cluster同NUMA - CL-15
    Given 集群有 4numa_384_cluster 配置的节点
    When 我创建 1 个 burstable 类型的 Deployment
    And Deployment 具有 10/20 CPU 资源配置
    And Deployment 具有 20Gi/40Gi Memory 资源配置
    And Deployment 具有标签 "test-CL-15"
    Then 容器应该被成功调度
    And 容器的 CPU 应该分配在 NUMA 级别
    And 容器应该分布在 1 个 NUMA 节点

  @cluster_topology @burstable_qos @cross_cluster @numa_affinity
  Scenario: Burstable大容器NUMA内亲和 - CL-16
    Given 集群有 4numa_384_cluster 配置的节点
    When 我创建 1 个 burstable 类型的 Deployment
    And Deployment 具有 48/80 CPU 资源配置
    And Deployment 具有 64Gi/120Gi Memory 资源配置
    And Deployment 具有标签 "test-CL-16"
    Then 容器应该被成功调度
    And 容器的 CPU 应该分配在 NUMA 级别
    And 容器应该分布在 1 个 NUMA 节点

  # Burstable QoS - 跨 NUMA 同 Socket 测试
  @cluster_topology @burstable_qos @cross_numa @socket_affinity
  Scenario: Burstable跨NUMA同Socket - CL-17
    Given 集群有 4numa_384_cluster 配置的节点
    When 我创建 1 个 burstable 类型的 Deployment
    And Deployment 具有 50/100 CPU 资源配置
    And Deployment 具有 80Gi/140Gi Memory 资源配置
    And Deployment 具有标签 "test-CL-17"
    Then 容器应该被成功调度
    And 容器的 CPU 应该分配在 SOCKET 级别
    And 容器应该分布在 1 个 SOCKET

  # Burstable QoS - 大 request 仍在 Socket 内
  @cluster_topology @burstable_qos @cross_numa @socket_affinity
  Scenario: Burstable大request容器Socket内亲和 - CL-18
    Given 集群有 4numa_384_cluster 配置的节点
    When 我创建 1 个 burstable 类型的 Deployment
    And Deployment 具有 100/200 CPU 资源配置
    And Deployment 具有 140Gi/280Gi Memory 资源配置
    And Deployment 具有标签 "test-CL-18"
    Then 容器应该被成功调度
    And 容器的 CPU 应该分配在 SOCKET 级别
    And 容器应该分布在 1 个 SOCKET

  # BestEffort QoS 测试
  @cluster_topology @besteffort_qos @no_affinity
  Scenario: BestEffort无拓扑亲和 - CL-19
    Given 集群有 4numa_384_cluster 配置的节点
    When 我创建 1 个 besteffort 类型的 Deployment
    And Deployment 具有标签 "test-CL-19"
    Then 容器应该被成功调度
    And 容器不应该应用拓扑亲和性
