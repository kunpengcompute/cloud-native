# QoS Controller E2E 编写与扩展指南

本文档用于指导在 `test/bdd-framework/k8s-mpam-controller` 下编写和扩展 BDD E2E 测试。

## 1. 测试代码结构与执行链路

当前套件的核心文件：

- `features/qos_controller.feature`：业务场景定义（Gherkin）
- `test_qos_controller.py`：将 feature 场景加载为 pytest 测试函数
- `step_definitions/common_steps.py`：Given/When/Then 的具体实现
- `conftest.py`：测试配置、环境变量映射、pytest hook

执行链路：

1. `pytest` 读取 `test_qos_controller.py`
2. `pytest-bdd` 将 `.feature` 场景转换为测试函数
3. 每个步骤文本匹配 `common_steps.py` 中对应装饰器函数
4. 场景结束后统一执行清理；失败时输出诊断信息

## 2. 如何新增一个 E2E 场景

建议遵循以下步骤：

1. 在 `features/qos_controller.feature` 新增 Scenario
2. 复用已有步骤文本；仅在必要时新增步骤实现
3. 在 `common_steps.py` 实现新步骤（优先组合已有 helper）
4. 执行 `pytest --collect-only -q` 验证步骤绑定
5. 在真实集群运行目标 mark 验证行为

建议优先复用的 helper：

- `_apply_policy`：创建/更新 `QoSPolicy`
- `_create_test_pod_doc`：构造测试 Pod
- `_wait_until`：轮询等待，适配异步 reconcile
- `_read_group_schemata`、`_group_exists`、`_group_tasks_non_empty`：`resctrl` 断言

## 3. 场景设计原则

### 3.1 一条场景只验证一个核心行为

避免一个 Scenario 同时覆盖“创建 + 更新 + 删除 + 负向”多个目标，出问题时很难定位。

### 3.2 断言要做“值一致性”，不要只做“存在性”

优先验证 `schemata` 实际值是否与 CR 对齐，而不是只判断包含 `MB:`/`L3:`。

### 3.3 对异步 reconcile 使用轮询

所有依赖 controller 写入结果的断言都应通过 `_wait_until`，避免瞬时状态导致误报。

### 3.4 场景可重复执行

确保使用唯一资源名或让清理逻辑可兜底，避免脏数据影响下次运行。

## 4. 扩展步骤实现的推荐模式

### 4.1 新增 Given/When/Then 时的优先级

1. 优先复用现有步骤
2. 复用现有 helper 组合新步骤
3. 最后才新增新的底层 helper

### 4.2 统一记录测试上下文

需要跨步骤共享的信息请写入 `test_context`，例如：

- 当前测试的策略名
- 上一次策略 spec（用于更新后断言）
- 目标节点信息

### 4.3 强制幂等

步骤内执行 `apply` 或标签设置逻辑时应考虑重复调用不会导致结果异常。

## 5. 扩展测试类型建议

可按以下方向分批增加：

- 幂等性：重复 apply 相同 CR，验证 `schemata` 不漂移
- NodeSelector：匹配/不匹配/切换清理
- Offline 行为：`workload-class=offline` 自动处理和 `cpu.qos_level` 校验
- 删除时序：Pod 与 CR 先后删除的行为一致性
- 异常恢复：手动破坏本地组后是否自愈

## 6. 常见问题与排查

### 6.1 `test_config not found`

确保当前 suite 的 `conftest.py` 内定义了 `test_config`（本套件已提供），不要依赖跨目录 `conftest` 导入行为。

### 6.2 场景卡超时

先看失败诊断输出：

- controller 日志
- `resctrl` 快照（`schemata`/`tasks`）
- Pod/CR describe

必要时临时调大：

- `QOS_E2E_TIMEOUT`
- `QOS_E2E_POLL_INTERVAL`

### 6.3 `tasks` 误判为空

`resctrl/tasks` 是特殊文件，不能用文件大小判断，必须读取内容判断是否有 PID 行。

## 7. 运行建议

快速检查步骤绑定：

```bash
cd test/bdd-framework/k8s-mpam-controller
pytest --collect-only -q
```

按标签运行：

```bash
pytest -m lifecycle
pytest -m "e2e and smoke"
```

单场景调试：

```bash
pytest -k "NodeSelector 匹配当前节点时策略应生效" -v -s
```

## 8. 变更提交建议

建议把 E2E 变更按以下粒度拆分提交：

1. `feature` 场景变更
2. `step_definitions` 实现变更
3. 文档更新

这样便于 review 和回滚。
