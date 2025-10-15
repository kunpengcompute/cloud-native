Feature: Kubernetes 部署配置演示
  演示框架如何支持各种 Kubernetes 部署配置
  包括基本部署、环境变量、Annotation、QoS 等功能

  Background:
    Given 集群有 default 配置的节点

  @demo @basic
  Scenario: 基本部署示例
    Given 使用容器镜像 "nginx:latest"
    When 我创建 1 个 guaranteed 类型的 Deployment
    And Deployment 具有 2/2 CPU 资源配置
    And Deployment 具有 4Gi/4Gi Memory 资源配置
    And Deployment 具有标签 "test-basic-demo"
    Then 容器应该被成功调度
    And 容器的 QoS 类别应该是 Guaranteed

  @demo @annotation
  Scenario: Annotation 配置示例
    Given 使用容器镜像 "nginx:latest"
    When 我创建 1 个 guaranteed 类型的 Deployment
    And Deployment 具有 2/2 CPU 资源配置
    And Deployment 具有 4Gi/4Gi Memory 资源配置
    And Deployment 具有 annotation scheduler.alpha.kubernetes.io/critical-pod=""
    And Deployment 具有 annotation my-project/priority=high
    And Deployment 具有 annotation my-project/team=platform
    And Deployment 具有标签 "test-annotation-demo"
    Then 容器应该被成功调度

  @demo @env-vars
  Scenario: 环境变量配置示例
    Given 使用容器镜像 "busybox:latest"
    When 我创建 1 个 guaranteed 类型的 Deployment
    And Deployment 具有 1/1 CPU 资源配置
    And Deployment 具有 2Gi/2Gi Memory 资源配置
    And Deployment 具有环境变量 APP_ENV=production
    And Deployment 具有环境变量 LOG_LEVEL=info
    And Deployment 具有环境变量 DEBUG=false
    And Deployment 具有标签 "test-env-demo"
    Then 容器应该被成功调度

  @demo @qos @burstable
  Scenario: Burstable QoS 示例
    Given 使用容器镜像 "nginx:latest"
    When 我创建 1 个 burstable 类型的 Deployment
    And Deployment 具有 1/2 CPU 资源配置
    And Deployment 具有 2Gi/4Gi Memory 资源配置
    And Deployment 具有标签 "test-burstable-demo"
    Then 容器应该被成功调度
    And 容器的 QoS 类别应该是 Burstable

  @demo @combined
  Scenario: 组合配置示例（环境变量 + Annotation + 命名空间）
    Given 使用容器镜像 "nginx:latest"
    And 使用命名空间 "demo-namespace"
    When 我创建 1 个 guaranteed 类型的 Deployment
    And Deployment 具有 4/4 CPU 资源配置
    And Deployment 具有 8Gi/8Gi Memory 资源配置
    And Deployment 具有环境变量 APP_NAME=demo-app
    And Deployment 具有环境变量 APP_VERSION=1.0.0
    And Deployment 具有环境变量 ENVIRONMENT=production
    And Deployment 具有 annotation app.kubernetes.io/name=demo-app
    And Deployment 具有 annotation app.kubernetes.io/version=1.0.0
    And Deployment 具有 annotation app.kubernetes.io/component=web-server
    And Deployment 具有标签 "test-combined-demo"
    Then 容器应该被成功调度
    And 容器的 QoS 类别应该是 Guaranteed

  @demo @multi-replica
  Scenario: 多副本部署示例
    Given 使用容器镜像 "nginx:latest"
    When 我创建 3 个 guaranteed 类型的 Deployment
    And Deployment 具有 1/1 CPU 资源配置
    And Deployment 具有 2Gi/2Gi Memory 资源配置
    And Deployment 具有标签 "test-multi-replica-demo"
    Then 容器应该被成功调度

