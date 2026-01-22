Feature: 通用 Kubernetes 部署与调度
  作为系统/平台工程师
  我希望快速验证容器部署与调度是否成功
  以便在不同项目中复用相同的步骤

  @smoke @generic
  Scenario: 创建 Guaranteed 容器并验证调度
    Given 集群有可用的 Kubernetes 节点
    When 我创建 1 个 guaranteed 类型的 Deployment
    And Deployment 具有 1/1 CPU 资源配置
    And Deployment 具有 2Gi/2Gi Memory 资源配置
    And Deployment 具有标签 "bdd-generic-guaranteed"
    Then 容器应该被成功调度
    And 容器的 QoS 类别应该是 Guaranteed

  @smoke @generic
  Scenario: 创建 BestEffort 容器并验证调度
    Given 集群有可用的 Kubernetes 节点
    When 我创建 1 个 besteffort 类型的 Deployment
    And Deployment 具有标签 "bdd-generic-besteffort"
    Then 容器应该被成功调度
    And 容器的 QoS 类别应该是 BestEffort

