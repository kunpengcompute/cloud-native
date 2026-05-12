# k8s-mpam-controller (QoS) BDD E2E

本目录是 QoS Operator 的 BDD E2E 套件，覆盖核心闭环：

- `QoSPolicy` 创建/更新/删除
- 带 `qos.kunpeng.huawei.com/group=<policy-name>` 标签的 Pod 加组
- 目标节点 `resctrl` 文件级断言（`schemata` / `tasks`）
- 非法 CR 在 `apply` 阶段被 API 拒绝

扩展与编写规范请参考：

- `E2E-GUIDE.md`

## 目录结构

```text
k8s-mpam-controller/
├── conftest.py
├── features/
│   └── qos_controller.feature
├── pytest.ini
├── README.md
├── step_definitions/
│   ├── __init__.py
│   └── common_steps.py
└── test_qos_controller.py
```

## 前置条件

- 真实 Kubernetes 集群
- 节点已挂载并可用：`/sys/fs/resctrl`、`/sys/fs/cgroup`
- 可执行 `kubectl` 且有足够权限（创建 CRD/DaemonSet/Pod/CR）
- 仓库内存在以下清单：
  - `config/kunpeng-qos-controller-config/crd/bases/qos.kunpeng.huawei.com_qospolicies.yaml`
  - `config/kunpeng-qos-controller-config/samples/qos-controller-daemonset-v1alpha1.yaml`

## 环境变量

- `QOS_E2E_NAMESPACE`：测试业务 Pod 所在命名空间，默认 `qos-e2e`
- `QOS_OPERATOR_NAMESPACE`：operator 命名空间，默认 `qos-system`
- `QOS_OPERATOR_IMAGE`：operator 镜像，默认 `k8s-mpam-controller:0.1.0`
- `QOS_E2E_POD_IMAGE`：测试 Pod 镜像，默认 `busybox:1.36`
- `QOS_E2E_NODE_SELECTOR`：可选，格式 `key=value`，用于限制 DaemonSet 测试节点
- `QOS_E2E_TIMEOUT`：轮询超时秒数，默认 `180`
- `QOS_E2E_POLL_INTERVAL`：轮询间隔秒数，默认 `3`

## 运行方式

在仓库根目录执行：

```bash
cd test/bdd-framework/kunpeng-qos-controller
pytest -m "e2e and smoke"
```

只跑某一类场景示例：

```bash
pytest -m lifecycle
pytest -m negative
```

## 当前场景

- 创建路径：创建 `QoSPolicy` + 创建带 group 标签 Pod，验证组目录、`schemata`、`tasks`
- 更新路径：更新策略参数，验证 `schemata` 发生变化
- 删除路径：删除策略，验证组目录清理
- 负向校验：非法 CR 被 API 拒绝且不产生本地组

## 清理与失败诊断

- 每个场景结束后强制清理：
  - 测试创建的 Pod
  - 测试创建的 `QoSPolicy`
  - 测试部署的 operator 与 CRD
- 场景失败时会自动打印：
  - controller 日志（tail）
  - 对应 `resctrl` 组的 `schemata/tasks` 快照
  - 相关 Pod/CR 的 `describe` 输出
