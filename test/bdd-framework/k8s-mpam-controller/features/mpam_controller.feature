Feature: MPAM Operator 核心闭环
  首批 BDD E2E 只覆盖核心闭环：QoSPolicy 生命周期 + Pod 绑定 + resctrl 断言。

  Background:
    Given MPAM controller 已部署并且节点具备 resctrl

  @e2e @smoke @lifecycle @resctrl
  Scenario: 创建路径 - 创建策略并绑定 Pod 到控制组
    When 创建默认全局 QoSPolicy "offline-small"
    And 创建带控制组标签 "offline-small" 的测试 Pod "offline-small-pod"
    Then Pod "offline-small-pod" 最终应为 Running
    And resctrl 控制组 "offline-small" 应该存在
    And 控制组 "offline-small" 的 schemata 应与默认 QoSPolicy 配置一致
    And 控制组 "offline-small" 的 tasks 应为非空

  @e2e @smoke @lifecycle @resctrl
  Scenario: 更新路径 - 更新策略后 schemata 生效
    Given 已创建默认全局 QoSPolicy "offline-small"
    And 已创建带控制组标签 "offline-small" 的测试 Pod "offline-small-pod"
    When 更新 QoSPolicy "offline-small" 的 MB.MAX 为 80 且 L3.WAYS 为 4
    Then 控制组 "offline-small" 的 schemata 应匹配更新后的 MB.MAX=80 和 L3.WAYS=4

  @e2e @smoke @lifecycle @resctrl
  Scenario: 删除路径 - 删除策略后控制组被清理
    Given 已创建默认全局 QoSPolicy "offline-small"
    When 删除 QoSPolicy "offline-small"
    Then resctrl 控制组 "offline-small" 最终应被清理

  @e2e @smoke @negative
  Scenario: 负向校验 - 非法 CR 在 apply 时被拒绝
    When 创建非法 QoSPolicy "invalid-policy"
    Then 创建应被 API 直接拒绝
    And 不应创建名为 "invalid-policy" 的 resctrl 控制组

  @e2e @smoke @lifecycle
  Scenario: 离线标签路径 - Pod 的 cpu.qos_level 被设置为 -1
    Given 已创建默认全局 QoSPolicy "offline-small"
    When 创建带离线标签和控制组标签 "offline-small" 的测试 Pod "offline-qos-pod"
    Then Pod "offline-qos-pod" 最终应为 Running
    And Pod "offline-qos-pod" 的所有容器 cpu.qos_level 应为 "-1"

  @e2e @smoke @lifecycle @resctrl
  Scenario: NodeSelector 匹配当前节点时策略应生效
    When 创建仅在当前节点生效的 QoSPolicy "node-selector-match"
    Then resctrl 控制组 "node-selector-match" 应该存在
    And 控制组 "node-selector-match" 的 schemata 应与默认 QoSPolicy 配置一致

  @e2e @smoke @lifecycle @resctrl
  Scenario: NodeSelector 不匹配当前节点时策略不应生效
    When 创建不匹配当前节点的 QoSPolicy "node-selector-miss"
    Then resctrl 控制组 "node-selector-miss" 不应存在

  @e2e @smoke @lifecycle @resctrl
  Scenario: NodeSelector 从匹配改为不匹配后应清理本地控制组
    When 创建仅在当前节点生效的 QoSPolicy "node-selector-switch"
    Then resctrl 控制组 "node-selector-switch" 应该存在
    When 更新 QoSPolicy "node-selector-switch" 的 NodeSelector 为不匹配当前节点
    Then resctrl 控制组 "node-selector-switch" 最终应被清理
