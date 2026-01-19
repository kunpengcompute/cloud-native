Feature: kunpeng-tap 拓扑感知调度测试
  作为系统管理员
  我希望验证 kunpeng-tap 的拓扑感知调度功能
  以确保不同 QoS 类型的容器能够根据预期实现或不实现拓扑亲和

  # 2NUMA 拓扑测试
  @guaranteed_qos @2numa_guaranteed_medium @standard @2numa @affinity_expected
  Scenario: 2NUMA-48核-4核容器NUMA内亲和 - 2N-01
    Given 集群有 2numa_48 配置的节点
    When 我创建 1 个 guaranteed 类型的 Deployment
    And Deployment 具有 4/4 CPU 资源配置
    And Deployment 具有 8Gi/8Gi Memory 资源配置
    And Deployment 具有标签 "test-2N-01"
    Then 容器应该被成功调度
    And 容器的 CPU 应该分配在 NUMA 级别
    And 容器应该分布在 1 个 NUMA 节点

  @guaranteed_qos @2numa_guaranteed_large @standard @2numa @affinity_expected
  Scenario: 2NUMA-48核-24核容器半NUMA亲和 - 2N-02
    Given 集群有 2numa_48 配置的节点
    When 我创建 1 个 guaranteed 类型的 Deployment
    And Deployment 具有 24/24 CPU 资源配置
    And Deployment 具有 32Gi/32Gi Memory 资源配置
    And Deployment 具有标签 "test-2N-02"
    Then 容器应该被成功调度
    And 容器的 CPU 应该分配在 NUMA 级别
    And 容器应该分布在 1 个 NUMA 节点

  @guaranteed_qos @2numa_guaranteed_near_full @standard @2numa @affinity_expected
  Scenario: 2NUMA-48核-47核容器接近满NUMA - 2N-03
    Given 集群有 2numa_48 配置的节点
    When 我创建 1 个 guaranteed 类型的 Deployment
    And Deployment 具有 47/47 CPU 资源配置
    And Deployment 具有 32Gi/32Gi Memory 资源配置
    And Deployment 具有标签 "test-2N-03"
    Then 容器应该被成功调度
    And 容器的 CPU 应该分配在 NUMA 级别
    And 容器应该分布在 1 个 NUMA 节点

  @guaranteed_qos @2numa_guaranteed_full_numa @standard @2numa @affinity_expected
  Scenario: 2NUMA-48核-48核容器满NUMA - 2N-04
    Given 集群有 2numa_48 配置的节点
    When 我创建 1 个 guaranteed 类型的 Deployment
    And Deployment 具有 48/48 CPU 资源配置
    And Deployment 具有 32Gi/32Gi Memory 资源配置
    And Deployment 具有标签 "test-2N-04"
    Then 容器应该被成功调度
    And 容器的 CPU 应该分配在 NUMA 级别
    And 容器应该分布在 1 个 NUMA 节点

  @guaranteed_qos @2numa_guaranteed_cross_numa @standard @2numa @affinity_expected
  Scenario: 2NUMA-48核-49核容器跨NUMA - 2N-05
    Given 集群有 2numa_48 配置的节点
    When 我创建 1 个 guaranteed 类型的 Deployment
    And Deployment 具有 49/49 CPU 资源配置
    And Deployment 具有 40Gi/40Gi Memory 资源配置
    And Deployment 具有标签 "test-2N-05"
    Then 容器应该被成功调度
    And 容器的 CPU 应该分配在 CROSS_NUMA 级别
    And 容器应该分布在 2 个 NUMA 节点

  @guaranteed_qos @2numa_guaranteed_large_cross_numa @standard @2numa @affinity_expected
  Scenario: 2NUMA-48核-72核容器大跨NUMA - 2N-06
    Given 集群有 2numa_48 配置的节点
    When 我创建 1 个 guaranteed 类型的 Deployment
    And Deployment 具有 72/72 CPU 资源配置
    And Deployment 具有 48Gi/48Gi Memory 资源配置
    And Deployment 具有标签 "test-2N-06"
    Then 容器应该被成功调度
    And 容器的 CPU 应该分配在 CROSS_NUMA 级别
    And 容器应该分布在 2 个 NUMA 节点

  @guaranteed_qos @2numa_guaranteed_full_machine @standard @2numa @affinity_expected
  Scenario: 2NUMA-48核-90核容器满机器 - 2N-07
    Given 集群有 2numa_48 配置的节点
    When 我创建 1 个 guaranteed 类型的 Deployment
    And Deployment 具有 90/90 CPU 资源配置
    And Deployment 具有 64Gi/64Gi Memory 资源配置
    And Deployment 具有标签 "test-2N-07"
    Then 容器应该被成功调度
    And 容器的 CPU 应该分配在 CROSS_NUMA 级别
    And 容器应该分布在 2 个 NUMA 节点

  @burstable_qos @2numa_burstable_small @standard @2numa @affinity_expected
  Scenario: 2NUMA-48核-0.5核Burstable容器 - 2N-08
    Given 集群有 2numa_48 配置的节点
    When 我创建 1 个 burstable 类型的 Deployment
    And Deployment 具有 0.5/1 CPU 资源配置
    And Deployment 具有 1Gi/2Gi Memory 资源配置
    And Deployment 具有标签 "test-2N-08"
    Then 容器应该被成功调度
    And 容器的 CPU 应该分配在 NUMA 级别
    And 容器应该分布在 1 个 NUMA 节点

  @burstable_qos @2numa_burstable_medium @standard @2numa @affinity_expected
  Scenario: 2NUMA-48核-2核Burstable容器 - 2N-09
    Given 集群有 2numa_48 配置的节点
    When 我创建 1 个 burstable 类型的 Deployment
    And Deployment 具有 2/4 CPU 资源配置
    And Deployment 具有 4Gi/8Gi Memory 资源配置
    And Deployment 具有标签 "test-2N-09"
    Then 容器应该被成功调度
    And 容器的 CPU 应该分配在 NUMA 级别
    And 容器应该分布在 1 个 NUMA 节点

  @besteffort_qos @2numa_besteffort @standard @2numa @no_affinity_expected
  Scenario: 2NUMA-48核-BestEffort容器 - 2N-09-BE
    Given 集群有 2numa_48 配置的节点
    When 我创建 1 个 besteffort 类型的 Deployment
    And Deployment 具有标签 "test-2N-09-BE"
    Then 容器应该被成功调度
    And 容器不应该应用拓扑亲和性
    And 容器应该使用默认调度策略

  @guaranteed_qos @2numa_guaranteed_medium @high_perf @2numa @affinity_expected
  Scenario: 2NUMA-160核-8核容器NUMA内亲和 - 2N-10
    Given 集群有 2numa_160 配置的节点
    When 我创建 1 个 guaranteed 类型的 Deployment
    And Deployment 具有 8/8 CPU 资源配置
    And Deployment 具有 16Gi/16Gi Memory 资源配置
    And Deployment 具有标签 "test-2N-10"
    Then 容器应该被成功调度
    And 容器的 CPU 应该分配在 NUMA 级别
    And 容器应该分布在 1 个 NUMA 节点

  @guaranteed_qos @2numa_guaranteed_large @high_perf @2numa @affinity_expected
  Scenario: 2NUMA-160核-80核容器半NUMA亲和 - 2N-11
    Given 集群有 2numa_160 配置的节点
    When 我创建 1 个 guaranteed 类型的 Deployment
    And Deployment 具有 80/80 CPU 资源配置
    And Deployment 具有 128Gi/128Gi Memory 资源配置
    And Deployment 具有标签 "test-2N-11"
    Then 容器应该被成功调度
    And 容器的 CPU 应该分配在 NUMA 级别
    And 容器应该分布在 1 个 NUMA 节点

  @guaranteed_qos @2numa_guaranteed_near_full @high_perf @2numa @affinity_expected
  Scenario: 2NUMA-160核-159核容器接近满NUMA - 2N-12
    Given 集群有 2numa_160 配置的节点
    When 我创建 1 个 guaranteed 类型的 Deployment
    And Deployment 具有 159/159 CPU 资源配置
    And Deployment 具有 128Gi/128Gi Memory 资源配置
    And Deployment 具有标签 "test-2N-12"
    Then 容器应该被成功调度
    And 容器的 CPU 应该分配在 NUMA 级别
    And 容器应该分布在 1 个 NUMA 节点

  @guaranteed_qos @2numa_guaranteed_full_numa @high_perf @2numa @affinity_expected
  Scenario: 2NUMA-160核-160核容器满NUMA - 2N-13
    Given 集群有 2numa_160 配置的节点
    When 我创建 1 个 guaranteed 类型的 Deployment
    And Deployment 具有 160/160 CPU 资源配置
    And Deployment 具有 128Gi/128Gi Memory 资源配置
    And Deployment 具有标签 "test-2N-13"
    Then 容器应该被成功调度
    And 容器的 CPU 应该分配在 NUMA 级别
    And 容器应该分布在 1 个 NUMA 节点

  @guaranteed_qos @2numa_guaranteed_cross_numa @high_perf @2numa @affinity_expected
  Scenario: 2NUMA-160核-161核容器跨NUMA - 2N-14
    Given 集群有 2numa_160 配置的节点
    When 我创建 1 个 guaranteed 类型的 Deployment
    And Deployment 具有 161/161 CPU 资源配置
    And Deployment 具有 160Gi/160Gi Memory 资源配置
    And Deployment 具有标签 "test-2N-14"
    Then 容器应该被成功调度
    And 容器的 CPU 应该分配在 CROSS_NUMA 级别
    And 容器应该分布在 2 个 NUMA 节点

  @guaranteed_qos @2numa_guaranteed_large_cross_numa @high_perf @2numa @affinity_expected
  Scenario: 2NUMA-160核-240核容器大跨NUMA - 2N-15
    Given 集群有 2numa_160 配置的节点
    When 我创建 1 个 guaranteed 类型的 Deployment
    And Deployment 具有 240/240 CPU 资源配置
    And Deployment 具有 200Gi/200Gi Memory 资源配置
    And Deployment 具有标签 "test-2N-15"
    Then 容器应该被成功调度
    And 容器的 CPU 应该分配在 CROSS_NUMA 级别
    And 容器应该分布在 2 个 NUMA 节点

  @guaranteed_qos @2numa_guaranteed_full_machine @high_perf @2numa @affinity_expected
  Scenario: 2NUMA-160核-310核容器满机器 - 2N-16
    Given 集群有 2numa_160 配置的节点
    When 我创建 1 个 guaranteed 类型的 Deployment
    And Deployment 具有 310/310 CPU 资源配置
    And Deployment 具有 256Gi/256Gi Memory 资源配置
    And Deployment 具有标签 "test-2N-16"
    Then 容器应该被成功调度
    And 容器的 CPU 应该分配在 CROSS_NUMA 级别
    And 容器应该分布在 2 个 NUMA 节点

  @burstable_qos @2numa_burstable_small @high_perf @2numa @affinity_expected
  Scenario: 2NUMA-160核-0.5核Burstable容器 - 2N-17
    Given 集群有 2numa_160 配置的节点
    When 我创建 1 个 burstable 类型的 Deployment
    And Deployment 具有 0.5/1 CPU 资源配置
    And Deployment 具有 1Gi/2Gi Memory 资源配置
    And Deployment 具有标签 "test-2N-17"
    Then 容器应该被成功调度
    And 容器的 CPU 应该分配在 NUMA 级别
    And 容器应该分布在 1 个 NUMA 节点

  @burstable_qos @2numa_burstable_large @high_perf @2numa @affinity_expected
  Scenario: 2NUMA-160核-16核Burstable容器 - 2N-18
    Given 集群有 2numa_160 配置的节点
    When 我创建 1 个 burstable 类型的 Deployment
    And Deployment 具有 16/32 CPU 资源配置
    And Deployment 具有 32Gi/64Gi Memory 资源配置
    And Deployment 具有标签 "test-2N-18"
    Then 容器应该被成功调度
    And 容器的 CPU 应该分配在 NUMA 级别
    And 容器应该分布在 1 个 NUMA 节点

  @besteffort_qos @2numa_besteffort @high_perf @2numa @no_affinity_expected
  Scenario: 2NUMA-160核-BestEffort容器 - 2N-18-BE
    Given 集群有 2numa_160 配置的节点
    When 我创建 1 个 besteffort 类型的 Deployment
    And Deployment 具有标签 "test-2N-18-BE"
    Then 容器应该被成功调度
    And 容器不应该应用拓扑亲和性
    And 容器应该使用默认调度策略

  # 4NUMA 拓扑测试
  @guaranteed_qos @4numa_guaranteed_medium @standard @4numa @affinity_expected
  Scenario: 4NUMA-24核-4核容器单NUMA亲和 - 4N-01
    Given 集群有 4numa_24 配置的节点
    When 我创建 1 个 guaranteed 类型的 Deployment
    And Deployment 具有 4/4 CPU 资源配置
    And Deployment 具有 8Gi/8Gi Memory 资源配置
    And Deployment 具有标签 "test-4N-01"
    Then 容器应该被成功调度
    And 容器的 CPU 应该分配在 NUMA 级别
    And 容器应该分布在 1 个 NUMA 节点

  @guaranteed_qos @4numa_guaranteed_large @standard @4numa @affinity_expected
  Scenario: 4NUMA-24核-12核容器半NUMA亲和 - 4N-02
    Given 集群有 4numa_24 配置的节点
    When 我创建 1 个 guaranteed 类型的 Deployment
    And Deployment 具有 12/12 CPU 资源配置
    And Deployment 具有 16Gi/16Gi Memory 资源配置
    And Deployment 具有标签 "test-4N-02"
    Then 容器应该被成功调度
    And 容器的 CPU 应该分配在 NUMA 级别
    And 容器应该分布在 1 个 NUMA 节点

  @guaranteed_qos @4numa_guaranteed_near_full @standard @4numa @affinity_expected
  Scenario: 4NUMA-24核-23核容器接近满NUMA - 4N-03
    Given 集群有 4numa_24 配置的节点
    When 我创建 1 个 guaranteed 类型的 Deployment
    And Deployment 具有 23/23 CPU 资源配置
    And Deployment 具有 16Gi/16Gi Memory 资源配置
    And Deployment 具有标签 "test-4N-03"
    Then 容器应该被成功调度
    And 容器的 CPU 应该分配在 NUMA 级别
    And 容器应该分布在 1 个 NUMA 节点

  @guaranteed_qos @4numa_guaranteed_full_numa @standard @4numa @affinity_expected
  Scenario: 4NUMA-24核-24核容器满NUMA - 4N-04
    Given 集群有 4numa_24 配置的节点
    When 我创建 1 个 guaranteed 类型的 Deployment
    And Deployment 具有 24/24 CPU 资源配置
    And Deployment 具有 16Gi/16Gi Memory 资源配置
    And Deployment 具有标签 "test-4N-04"
    Then 容器应该被成功调度
    And 容器的 CPU 应该分配在 NUMA 级别
    And 容器应该分布在 1 个 NUMA 节点

  @guaranteed_qos @4numa_guaranteed_cross_numa_same_socket @standard @4numa @affinity_expected
  Scenario: 4NUMA-24核-25核容器跨NUMA同SOCKET - 4N-05
    Given 集群有 4numa_24 配置的节点
    When 我创建 1 个 guaranteed 类型的 Deployment
    And Deployment 具有 25/25 CPU 资源配置
    And Deployment 具有 20Gi/20Gi Memory 资源配置
    And Deployment 具有标签 "test-4N-05"
    Then 容器应该被成功调度
    And 容器的 CPU 应该分配在 SOCKET 级别
    And 容器应该分布在 2 个 NUMA 节点
    And 容器应该分布在 1 个 SOCKET

  @guaranteed_qos @4numa_guaranteed_cross_numa_same_socket @standard @4numa @affinity_expected
  Scenario: 4NUMA-24核-36核容器1.5NUMA - 4N-06
    Given 集群有 4numa_24 配置的节点
    When 我创建 1 个 guaranteed 类型的 Deployment
    And Deployment 具有 36/36 CPU 资源配置
    And Deployment 具有 24Gi/24Gi Memory 资源配置
    And Deployment 具有标签 "test-4N-06"
    Then 容器应该被成功调度
    And 容器的 CPU 应该分配在 SOCKET 级别
    And 容器应该分布在 2 个 NUMA 节点
    And 容器应该分布在 1 个 SOCKET

  @guaranteed_qos @4numa_guaranteed_full_socket @standard @4numa @affinity_expected
  Scenario: 4NUMA-24核-48核容器满SOCKET - 4N-07
    Given 集群有 4numa_24 配置的节点
    When 我创建 1 个 guaranteed 类型的 Deployment
    And Deployment 具有 48/48 CPU 资源配置
    And Deployment 具有 32Gi/32Gi Memory 资源配置
    And Deployment 具有标签 "test-4N-07"
    Then 容器应该被成功调度
    And 容器的 CPU 应该分配在 SOCKET 级别
    And 容器应该分布在 2 个 NUMA 节点
    And 容器应该分布在 1 个 SOCKET

  @guaranteed_qos @4numa_guaranteed_cross_socket @standard @4numa @affinity_expected
  Scenario: 4NUMA-24核-49核容器跨SOCKET - 4N-08
    Given 集群有 4numa_24 配置的节点
    When 我创建 1 个 guaranteed 类型的 Deployment
    And Deployment 具有 49/49 CPU 资源配置
    And Deployment 具有 40Gi/40Gi Memory 资源配置
    And Deployment 具有标签 "test-4N-08"
    Then 容器应该被成功调度
    And 容器不应该应用拓扑亲和性

  @guaranteed_qos @4numa_guaranteed_cross_socket @standard @4numa @affinity_expected
  Scenario: 4NUMA-24核-72核容器大跨SOCKET - 4N-09
    Given 集群有 4numa_24 配置的节点
    When 我创建 1 个 guaranteed 类型的 Deployment
    And Deployment 具有 72/72 CPU 资源配置
    And Deployment 具有 48Gi/48Gi Memory 资源配置
    And Deployment 具有标签 "test-4N-09"
    Then 容器应该被成功调度
    And 容器不应该应用拓扑亲和性

  @guaranteed_qos @4numa_guaranteed_full_machine @standard @4numa @affinity_expected
  Scenario: 4NUMA-24核-90核容器满机器 - 4N-10
    Given 集群有 4numa_24 配置的节点
    When 我创建 1 个 guaranteed 类型的 Deployment
    And Deployment 具有 90/90 CPU 资源配置
    And Deployment 具有 64Gi/64Gi Memory 资源配置
    And Deployment 具有标签 "test-4N-10"
    Then 容器应该被成功调度
    And 容器不应该应用拓扑亲和性

  @burstable_qos @4numa_burstable_small @standard @4numa @affinity_expected
  Scenario: 4NUMA-24核-0.5核Burstable容器 - 4N-11
    Given 集群有 4numa_24 配置的节点
    When 我创建 1 个 burstable 类型的 Deployment
    And Deployment 具有 0.5/1 CPU 资源配置
    And Deployment 具有 1Gi/2Gi Memory 资源配置
    And Deployment 具有标签 "test-4N-11"
    Then 容器应该被成功调度
    And 容器的 CPU 应该分配在 NUMA 级别
    And 容器应该分布在 1 个 NUMA 节点

  @burstable_qos @4numa_burstable_medium @standard @4numa @affinity_expected
  Scenario: 4NUMA-24核-2核Burstable容器 - 4N-12
    Given 集群有 4numa_24 配置的节点
    When 我创建 1 个 burstable 类型的 Deployment
    And Deployment 具有 2/4 CPU 资源配置
    And Deployment 具有 4Gi/8Gi Memory 资源配置
    And Deployment 具有标签 "test-4N-12"
    Then 容器应该被成功调度
    And 容器的 CPU 应该分配在 NUMA 级别
    And 容器应该分布在 1 个 NUMA 节点

  @besteffort_qos @4numa_besteffort @standard @4numa @no_affinity_expected
  Scenario: 4NUMA-24核-BestEffort容器 - 4N-12-BE
    Given 集群有 4numa_24 配置的节点
    When 我创建 1 个 besteffort 类型的 Deployment
    And Deployment 具有标签 "test-4N-12-BE"
    Then 容器应该被成功调度
    And 容器不应该应用拓扑亲和性
    And 容器应该使用默认调度策略

  @guaranteed_qos @4numa_guaranteed_medium @high_perf @4numa @affinity_expected
  Scenario: 4NUMA-80核-8核容器单NUMA亲和 - 4N-13
    Given 集群有 4numa_80 配置的节点
    When 我创建 1 个 guaranteed 类型的 Deployment
    And Deployment 具有 8/8 CPU 资源配置
    And Deployment 具有 16Gi/16Gi Memory 资源配置
    And Deployment 具有标签 "test-4N-13"
    Then 容器应该被成功调度
    And 容器的 CPU 应该分配在 NUMA 级别
    And 容器应该分布在 1 个 NUMA 节点

  @guaranteed_qos @4numa_guaranteed_large @high_perf @4numa @affinity_expected
  Scenario: 4NUMA-80核-40核容器半NUMA亲和 - 4N-14
    Given 集群有 4numa_80 配置的节点
    When 我创建 1 个 guaranteed 类型的 Deployment
    And Deployment 具有 40/40 CPU 资源配置
    And Deployment 具有 64Gi/64Gi Memory 资源配置
    And Deployment 具有标签 "test-4N-14"
    Then 容器应该被成功调度
    And 容器的 CPU 应该分配在 NUMA 级别
    And 容器应该分布在 1 个 NUMA 节点

  @guaranteed_qos @4numa_guaranteed_near_full @high_perf @4numa @affinity_expected
  Scenario: 4NUMA-80核-79核容器接近满NUMA - 4N-15
    Given 集群有 4numa_80 配置的节点
    When 我创建 1 个 guaranteed 类型的 Deployment
    And Deployment 具有 79/79 CPU 资源配置
    And Deployment 具有 64Gi/64Gi Memory 资源配置
    And Deployment 具有标签 "test-4N-15"
    Then 容器应该被成功调度
    And 容器的 CPU 应该分配在 NUMA 级别
    And 容器应该分布在 1 个 NUMA 节点

  @guaranteed_qos @4numa_guaranteed_full_numa @high_perf @4numa @affinity_expected
  Scenario: 4NUMA-80核-80核容器满NUMA - 4N-16
    Given 集群有 4numa_80 配置的节点
    When 我创建 1 个 guaranteed 类型的 Deployment
    And Deployment 具有 80/80 CPU 资源配置
    And Deployment 具有 64Gi/64Gi Memory 资源配置
    And Deployment 具有标签 "test-4N-16"
    Then 容器应该被成功调度
    And 容器的 CPU 应该分配在 NUMA 级别
    And 容器应该分布在 1 个 NUMA 节点

  @guaranteed_qos @4numa_guaranteed_cross_numa_same_socket @high_perf @4numa @affinity_expected
  Scenario: 4NUMA-80核-81核容器跨NUMA同SOCKET - 4N-17
    Given 集群有 4numa_80 配置的节点
    When 我创建 1 个 guaranteed 类型的 Deployment
    And Deployment 具有 81/81 CPU 资源配置
    And Deployment 具有 80Gi/80Gi Memory 资源配置
    And Deployment 具有标签 "test-4N-17"
    Then 容器应该被成功调度
    And 容器的 CPU 应该分配在 SOCKET 级别
    And 容器应该分布在 2 个 NUMA 节点
    And 容器应该分布在 1 个 SOCKET

  @guaranteed_qos @4numa_guaranteed_cross_numa_same_socket @high_perf @4numa @affinity_expected
  Scenario: 4NUMA-80核-120核容器1.5NUMA - 4N-18
    Given 集群有 4numa_80 配置的节点
    When 我创建 1 个 guaranteed 类型的 Deployment
    And Deployment 具有 120/120 CPU 资源配置
    And Deployment 具有 96Gi/96Gi Memory 资源配置
    And Deployment 具有标签 "test-4N-18"
    Then 容器应该被成功调度
    And 容器的 CPU 应该分配在 SOCKET 级别
    And 容器应该分布在 2 个 NUMA 节点
    And 容器应该分布在 1 个 SOCKET

  @guaranteed_qos @4numa_guaranteed_full_socket @high_perf @4numa @affinity_expected
  Scenario: 4NUMA-80核-160核容器满SOCKET - 4N-19
    Given 集群有 4numa_80 配置的节点
    When 我创建 1 个 guaranteed 类型的 Deployment
    And Deployment 具有 160/160 CPU 资源配置
    And Deployment 具有 128Gi/128Gi Memory 资源配置
    And Deployment 具有标签 "test-4N-19"
    Then 容器应该被成功调度
    And 容器的 CPU 应该分配在 SOCKET 级别
    And 容器应该分布在 2 个 NUMA 节点
    And 容器应该分布在 1 个 SOCKET

  @guaranteed_qos @4numa_guaranteed_cross_socket @high_perf @4numa @affinity_expected
  Scenario: 4NUMA-80核-161核容器跨SOCKET - 4N-20
    Given 集群有 4numa_80 配置的节点
    When 我创建 1 个 guaranteed 类型的 Deployment
    And Deployment 具有 161/161 CPU 资源配置
    And Deployment 具有 160Gi/160Gi Memory 资源配置
    And Deployment 具有标签 "test-4N-20"
    Then 容器应该被成功调度
    And 容器的 CPU 应该分配在 CROSS_SOCKET 级别
    And 容器应该分布在 4 个 NUMA 节点
    And 容器应该分布在 2 个 SOCKET

  @guaranteed_qos @4numa_guaranteed_cross_socket @high_perf @4numa @affinity_expected
  Scenario: 4NUMA-80核-240核容器大跨SOCKET - 4N-21
    Given 集群有 4numa_80 配置的节点
    When 我创建 1 个 guaranteed 类型的 Deployment
    And Deployment 具有 240/240 CPU 资源配置
    And Deployment 具有 192Gi/192Gi Memory 资源配置
    And Deployment 具有标签 "test-4N-21"
    Then 容器应该被成功调度
    And 容器的 CPU 应该分配在 CROSS_SOCKET 级别
    And 容器应该分布在 4 个 NUMA 节点
    And 容器应该分布在 2 个 SOCKET

  @guaranteed_qos @4numa_guaranteed_full_machine @high_perf @4numa @affinity_expected
  Scenario: 4NUMA-80核-310核容器满机器 - 4N-22
    Given 集群有 4numa_80 配置的节点
    When 我创建 1 个 guaranteed 类型的 Deployment
    And Deployment 具有 310/310 CPU 资源配置
    And Deployment 具有 256Gi/256Gi Memory 资源配置
    And Deployment 具有标签 "test-4N-22"
    Then 容器应该被成功调度
    And 容器的 CPU 应该分配在 CROSS_SOCKET 级别
    And 容器应该分布在 4 个 NUMA 节点
    And 容器应该分布在 2 个 SOCKET

  @burstable_qos @4numa_burstable_small @high_perf @4numa @affinity_expected
  Scenario: 4NUMA-80核-0.5核Burstable容器 - 4N-23
    Given 集群有 4numa_80 配置的节点
    When 我创建 1 个 burstable 类型的 Deployment
    And Deployment 具有 0.5/1 CPU 资源配置
    And Deployment 具有 1Gi/2Gi Memory 资源配置
    And Deployment 具有标签 "test-4N-23"
    Then 容器应该被成功调度
    And 容器的 CPU 应该分配在 NUMA 级别
    And 容器应该分布在 1 个 NUMA 节点

  @burstable_qos @4numa_burstable_large @high_perf @4numa @affinity_expected
  Scenario: 4NUMA-80核-16核Burstable容器 - 4N-24
    Given 集群有 4numa_80 配置的节点
    When 我创建 1 个 burstable 类型的 Deployment
    And Deployment 具有 16/32 CPU 资源配置
    And Deployment 具有 32Gi/64Gi Memory 资源配置
    And Deployment 具有标签 "test-4N-24"
    Then 容器应该被成功调度
    And 容器的 CPU 应该分配在 NUMA 级别
    And 容器应该分布在 1 个 NUMA 节点

  @besteffort_qos @4numa_besteffort @high_perf @4numa @no_affinity_expected
  Scenario: 4NUMA-80核-BestEffort容器 - 4N-24-BE
    Given 集群有 4numa_80 配置的节点
    When 我创建 1 个 besteffort 类型的 Deployment
    And Deployment 具有标签 "test-4N-24-BE"
    Then 容器应该被成功调度
    And 容器不应该应用拓扑亲和性
    And 容器应该使用默认调度策略

  # 边界条件测试
  @guaranteed_qos @boundary_min_guaranteed @standard @2numa @affinity_expected
  Scenario: 最小资源Guaranteed容器-0.1核 - BC-01
    Given 集群有 2numa_48 配置的节点
    When 我创建 1 个 guaranteed 类型的 Deployment
    And Deployment 具有 0.1/0.1 CPU 资源配置
    And Deployment 具有 128Mi/128Mi Memory 资源配置
    And Deployment 具有标签 "test-BC-01"
    Then 容器应该被成功调度
    And 容器的 CPU 应该分配在 NUMA 级别
    And 容器应该分布在 1 个 NUMA 节点

  @guaranteed_qos @boundary_full_numa @standard @2numa @affinity_expected
  Scenario: 满NUMA资源Guaranteed容器-48核 - BC-02
    Given 集群有 2numa_48 配置的节点
    When 我创建 1 个 guaranteed 类型的 Deployment
    And Deployment 具有 48/48 CPU 资源配置
    And Deployment 具有 32Gi/32Gi Memory 资源配置
    And Deployment 具有标签 "test-BC-02"
    Then 容器应该被成功调度
    And 容器的 CPU 应该分配在 NUMA 级别
    And 容器应该分布在 1 个 NUMA 节点

  @guaranteed_qos @boundary_exceed_numa @standard @2numa @affinity_expected
  Scenario: 超出单NUMA资源容器-49核 - BC-03
    Given 集群有 2numa_48 配置的节点
    When 我创建 1 个 guaranteed 类型的 Deployment
    And Deployment 具有 49/49 CPU 资源配置
    And Deployment 具有 33Gi/33Gi Memory 资源配置
    And Deployment 具有标签 "test-BC-03"
    Then 容器应该被成功调度
    And 容器的 CPU 应该分配在 CROSS_NUMA 级别
    And 容器应该分布在 2 个 NUMA 节点

  @besteffort_qos @boundary_besteffort_no_affinity @standard @2numa @no_affinity_expected
  Scenario: 零资源请求BestEffort容器 - BC-04
    Given 集群有 2numa_48 配置的节点
    When 我创建 1 个 besteffort 类型的 Deployment
    And Deployment 具有标签 "test-BC-04"
    Then 容器应该被成功调度
    And 容器不应该应用拓扑亲和性
    And 容器应该使用默认调度策略

  @guaranteed_qos @boundary_memory_intensive @standard @2numa @affinity_expected
  Scenario: 内存密集型容器-1核64Gi - BC-05
    Given 集群有 2numa_48 配置的节点
    When 我创建 1 个 guaranteed 类型的 Deployment
    And Deployment 具有 1/1 CPU 资源配置
    And Deployment 具有 64Gi/64Gi Memory 资源配置
    And Deployment 具有标签 "test-BC-05"
    Then 容器应该被成功调度
    And 容器的 CPU 应该分配在 SOCKET 级别
    And 容器应该分布在 2 个 NUMA 节点
