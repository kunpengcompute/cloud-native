Feature: kunpeng-tap 故障恢复场景测试
  作为系统管理员
  我希望验证 kunpeng-tap 在各种故障场景下的恢复能力
  以确保系统的稳定性和容器亲和性的持久性

  # 故障恢复测试 - 对应 TESTCASE.md 第 5 节

  @failure_recovery @tap_restart
  Scenario: tap插件重启后亲和关系重建 - FAIL-01
    Given 集群有 4numa_24 配置的节点
    When 我创建 3 个 guaranteed 类型的 Deployment
    And Deployment 具有 4/4 CPU 资源配置
    And Deployment 具有标签 "test-FAIL-01"
    Then 容器应该被成功调度
    And 容器的 CPU 应该分配在 NUMA 级别
    When kunpeng-tap 插件重启
    And 等待 30 秒让系统恢复
    Then 所有容器应该保持运行状态
    And 容器的 CPU 应该分配在 NUMA 级别
    And 容器的 NUMA 亲和性应该与重启前一致

  @failure_recovery @container_restart
  Scenario: 容器反复重启不影响其他容器 - FAIL-02
    Given 集群有 4numa_24 配置的节点
    When 我创建 2 个 guaranteed 类型的 Deployment
    And Deployment 具有 8/8 CPU 资源配置
    And Deployment 具有标签 "test-FAIL-02-stable"
    Then 容器应该被成功调度
    And 容器的 CPU 应该分配在 NUMA 级别
    When 我创建 1 个 guaranteed 类型的 Deployment
    And Deployment 具有 100/100 CPU 资源配置
    And Deployment 具有标签 "test-FAIL-02-failing"
    Then 容器调度应该失败
    And 稳定容器应该保持运行状态
    And 稳定容器的 NUMA 亲和性应该保持不变
    And 稳定容器在不同 NUMA 上分配应该均匀

  @failure_recovery @containerd_restart
  Scenario: containerd重启后亲和关系重建 - FAIL-03
    Given 集群有 4numa_24 配置的节点
    When 我创建 4 个 guaranteed 类型的 Deployment
    And Deployment 具有 8/8 CPU 资源配置
    And Deployment 具有标签 "test-FAIL-03"
    Then 容器应该被成功调度
    And 容器的 CPU 应该分配在 NUMA 级别
    And 容器在不同 NUMA 上分配应该均匀
    When containerd 服务重启
    And 等待 60 秒让系统恢复
    Then 所有容器应该保持运行状态
    And 容器的 CPU 应该分配在 NUMA 级别
    And 容器在不同 NUMA 上分配应该均匀
    And 容器的 NUMA 亲和性应该与重启前一致
