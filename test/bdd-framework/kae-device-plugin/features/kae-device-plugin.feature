Feature: KAE Device Plugin直通KAE设备与设置QoS测试
  测试KAE Device Plugin直通KAE设备与设置QoS功能

  Background:
    Given 集群有部署 kunpeng-kae-plugin daemonset配置的节点

  Scenario: KAE设备直通测试
    Given 使用容器镜像 "kae-test:latest"
    When 我创建 1 个 guaranteed 类型的 Deployment
    And Deployment 具有 4/4 CPU 资源配置
    And Deployment 具有 16Gi/16Gi Memory 资源配置
    And Deployment 具有自定义资源 kae.kunpeng.com/hisi_hpre 配置为 1/1
    And Deployment 具有自定义资源 kae.kunpeng.com/hisi_sec2 配置为 1/1
    And Deployment 具有自定义资源 kae.kunpeng.com/hisi_zip 配置为 1/1
    And Deployment 具有标签 "kae-test-0"
    Then 容器应该被成功调度

  Scenario: 设置KAE设备QoS测试
    Given 使用容器镜像 "kae-test:latest"
    When 我创建 1 个 guaranteed 类型的 Deployment
    And Deployment 具有 4/4 CPU 资源配置
    And Deployment 具有 16Gi/16Gi Memory 资源配置
    And Deployment 具有自定义资源 kae.kunpeng.com/hisi_hpre 配置为 1/1
    And Deployment 具有自定义资源 kae.kunpeng.com/hisi_sec2 配置为 1/1
    And Deployment 具有自定义资源 kae.kunpeng.com/hisi_zip 配置为 1/1
    And Deployment 具有 annotation qos.kae.kunpeng.com/hisi_hpre=500
    And Deployment 具有 annotation qos.kae.kunpeng.com/hisi_sec2=600
    And Deployment 具有 annotation qos.kae.kunpeng.com/hisi_zip=700
    And Deployment 具有标签 "kae-test-1"
    Then 容器应该被成功调度
    And 容器的KAE QoS应该被成功设置 hpre 为 500
    And 容器的KAE QoS应该被成功设置 sec2 为 600
    And 容器的KAE QoS应该被成功设置 zip 为 700
