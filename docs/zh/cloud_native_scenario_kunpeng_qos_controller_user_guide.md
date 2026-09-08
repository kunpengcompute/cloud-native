# 在离线混部插件 用户指南

## 简介

`kunpeng-qos-controller`是一个以DaemonSet方式运行的节点本地Operator，用于把Kubernetes配置映射到节点的`resctrl`/`cgroup`控制能力，实现节点级QoS资源管控。

当前主要能力：

- 监听`QoSPolicy` CR，并在本节点创建、更新、删除对应`resctrl`控制组。
- 将Pod绑定到目标控制组（通过Pod标签`qos.kunpeng.huawei.com/group`）。
- 根据干扰检测结果调整离线负载（`qos.kunpeng.huawei.com/workload-class=offline`）的资源配置。
- 可通过`QoSPolicy`的`cpu.qosLevel`字段控制组内Pod的`cpu.qos_level`。

## 应用场景

- 需要在Kubernetes集群中对节点本地`resctrl`进行统一管控。
- 需要按业务类型给Pod分配不同QoS策略（例如离线与在线任务隔离）。
- 需要通过CRD声明策略，并由Operator自动完成控制组创建和参数下发。

## 环境要求<a name="ZH-CN_TOPIC_0000002518412470"></a>

本文基于特定环境提供指导，在正式操作前请确保软硬件均满足要求。

**硬件要求<a name="section8140133619490"></a>**

如[**表 1** 硬件要求](#硬件要求)所示。

**表 1** 硬件要求<a id="硬件要求"></a>

|项目|规格|
|--|--|
|CPU|鲲鹏920新型号处理器、鲲鹏950处理器|

**操作系统和软件要求<a name="section21631338357"></a>**

此处列的操作系统版本和软件版本为已验证的版本，其他版本也可使用本插件，需要自行调整适配。如果环境未安装K8s或Containerd，可参见[**表 2** 已验证的操作系统和软件版本](#已验证的操作系统和软件版本)进行安装。

**表 2** 已验证的操作系统和软件版本<a id="已验证的操作系统和软件版本"></a>

|软件|版本|获取地址|
|--|--|--|
|OS|openEuler 24.03 LTS SP2|[获取链接](https://www.openeuler.org/en/download/#openEuler%2024.03%20LTS%20SP2)|
|Kubernetes|1.25.16|请参见《[Kubernetes 部署指南（CentOS&openEuler）](https://www.hikunpeng.com/document/detail/zh/kunpengcpfs/ecosystemEnable/Kubernetes/kunpengk8s_04_0001.html)》进行下载部署。|
|Containerd|1.7.10|请参见《[Containerd 安装指南（CentOS 8.1&openEuler 20.03）](https://www.hikunpeng.com/document/detail/zh/kunpengcpfs/ecosystemEnable/Containerd/kunpengcontainerd_03_0001.html)》进行下载部署。|
|Docker|18.0.9|通过配置Yum源方式安装|
|kunpeng-qos-controller|1.0.0|[获取链接](https://gitcode.com/boostkit/cloud-native)|

## 原理描述

系统由两个主要Reconciler组成：

- `QoSPolicyReconciler`：
  - 监听`QoSPolicy`。
  - 依据`nodeSelector`判断当前节点是否需要生效。
  - 对匹配节点创建/更新`resctrl`控制组并写入`schemata`。
  - 删除策略时执行本地控制组清理。
- `PodBindingReconciler`：
  - 监听Pod。
  - 根据Pod标签`qos.kunpeng.huawei.com/group`把Pod进程加入指定控制组。
  - 根据目标`QoSPolicy`的`cpu.qosLevel`对Pod容器写入`cpu.qos_level`。

当前实现中，`QoSPolicy.metadata.name`与`resctrl`控制组名为1:1映射。

# 软件编译

以下命令在仓库根目录执行。

## 本地二进制编译

```bash
make kunpeng-qos-controller-build
```

输出路径：

 `bin/kunpeng-qos-controller`

## Docker镜像编译

```bash
make kunpeng-qos-controller-docker
```

说明：镜像通过`Dockerfile.kunpeng-qos-controller`构建。

如果集群运行时使用的是`containerd`，可先导出镜像，再在目标节点导入：

```bash
docker save kunpeng-qos-controller:0.1.0 -o kunpeng-qos-controller.tar
```

将`kunpeng-qos-controller.tar`复制到目标节点后，执行：

```bash
ctr -n k8s.io images import /path/to/kunpeng-qos-controller.tar
```

# 软件部署

前提：目标节点已具备`resctrl`能力并挂载了`/sys/fs/resctrl`，容器可访问`/sys/fs/cgroup`。
>说明：如果节点未挂载`/sys/fs/resctrl`，请先挂载。执行命令`mount -t resctrl resctrl /sys/fs/resctrl`。如果`/sys/fs`下面没有`resctrl`目录，说明内核没有开启MPAM功能。在内核启动参数中添加`arm64.mpam`即可开启。
>
## 部署CRD

```bash
kubectl apply -f config/kunpeng-qos-controller-config/crd/bases/qos.kunpeng.huawei.com_qospolicies.yaml
```

## 部署Operator（DaemonSet + RBAC）

```bash
kubectl apply -f config/kunpeng-qos-controller-config/samples/qos-controller-daemonset-v1alpha1.yaml
```

## 检查运行状态

```bash
kubectl -n qos-system get pod -l app=qos-controller -o wide
kubectl -n qos-system logs -l app=qos-controller --tail=200
```

可能的回显结果如下, qos-controller运行状态为`Running`，并且日志中没有报错。
![图：检查运行状态](../images/kunpeng-qos-controller-deploy.png)

## 本地调试运行（可选）

```bash
export NODE_NAME=<你的节点名>
./bin/kunpeng-qos-controller
  --kubeconfig ~/.kube/config
```

# 使用特性

## 创建策略并下发到节点

> 说明：`cpu.qos_level`依赖内核能力，要求内核版本为`6.6.0-154.0.0`之后版本。
> 同时需要在内核启动参数中添加`xint`，并在系统启动后执行以下命令启用调度特性：
> `echo SMT_TAG_PULL > /sys/kernel/debug/sched/features`

### QoSPolicy字段说明

| 字段 | 含义 | 取值范围/默认值 | 说明 |
|---------|---------|---------|---------|
| **spec.nodeSelector** | 指定策略在哪些节点生效 | map，可选 | 与节点标签匹配的节点才应用该策略。 |
| **spec.mb.hdl** | MB开关 | **0~1**，默认**1** | 一般**1**表示开启。 |
| **spec.mb.pri** | MB优先级 | **0~7**，默认**3** | 数值越高表示优先级越高。 |
| **spec.mb.min** | MB最小保障比例 | **0~100**，默认**0** | 百分比语义。 |
| **spec.mb.max** | MB最大上限比例 | **0~100**，默认**100** | 百分比语义。 |
| **spec.l3.pri** | L3优先级 | **0~3**，默认**0** | L3的优先级控制。 |
| **spec.l3.min** | L3最小保障比例 | **0~100**，默认**0** | 百分比语义。 |
| **spec.l3.max** | L3最大上限比例 | **0~100**，默认**100** | 百分比语义。 |
| **spec.l3.ways** | 分配的Cache way数量 | **>=1** | 上限由节点硬件决定。 |
| **spec.cpu.qosLevel** | Pod CPU QoS级别 | **-1/0/1**，默认**0** | 写入组内Pod的**cpu.qos_level**。 -1表示低优先级业务，0表示默认优先级，1表示高优先级业务。 |

### 示例：创建QoSPolicy

```yaml
apiVersion: qos.kunpeng.huawei.com/v1alpha1
kind: QoSPolicy
metadata:
  name: offline-small
spec:
  nodeSelector:
    kubernetes.io/hostname: <your-node-name>
  mb:
    hdl: 1
    pri: 2
    min: 10
    max: 60
  l3:
    pri: 1
    min: 10
    max: 60
    ways: 4
  cpu:
    qosLevel: -1
```

应用：

```bash
kubectl apply -f qospolicy-offline-small.yaml
```

### 更新控制组配置（通过更新QoSPolicy）

控制组参数更新通过修改同名`QoSPolicy` CR实现，Operator会自动将更新后的配置同步到本地`resctrl`控制组。

```bash
kubectl edit qospolicy offline-small
```

在编辑界面中修改`spec`下对应字段（例如`mb.max`、`l3.ways`、`cpu.qosLevel`）后保存退出即可触发更新。

可能的回显结果如下
![图：kubectl edit修改QoSPolicy示例](../images/kunpeng-qos-controller-update-resctrl.png)
这里把`mb.max`从`60`改为`50`,`l3.ways`从`4`改为`1`
> 注意：cpu.qos_level只能设置一次，默认值是`0`。可以从0设置到-1，也可以从0设置到1，但是设置之后就不能再更改。
>
#### 更新后验证

```bash
kubectl get qospolicy offline-small -o yaml
POD=$(kubectl -n qos-system get pod -l app=qos-controller -o name | head -n1)
kubectl -n qos-system exec -it "${POD#pod/}" -- cat /sys/fs/resctrl/offline-small/schemata
```

可能的回显结果如下，可以看到CR中的`mb.max`和`l3.ways`已更新为`50`和`1`，同时resctrl的控制组中的schemata的值也进行了相应的更新。
![图：更新后验证](../images/kunpeng-qos-controller-update-ensure.png)

## 将Pod加入指定控制组

### 关系说明

- `QoSPolicy.metadata.name = offline-small`
- 控制组目录为`/sys/fs/resctrl/offline-small`
- Pod标签`qos.kunpeng.huawei.com/group`必须与策略名一致。

### 示例：创建带group标签的Pod

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: offline-small-nginx
  labels:
    app: nginx
    qos.kunpeng.huawei.com/group: offline-small
spec:
  nodeSelector:
    kubernetes.io/hostname: <your-node-name>
  containers:
  - name: nginx
    image: nginx:1.25
```

应用：

```bash
kubectl apply -f pod-offline-small.yaml
```

### 验证

#### 验证Pod是否加入到指定控制组中

```bash
POD=$(kubectl -n qos-system get pod -l app=qos-controller -o name | head -n1)
kubectl -n qos-system exec -it "${POD#pod/}" -- cat /sys/fs/resctrl/offline-small/tasks
```

可能的回显结果如下，可以看到在`offline-small`控制组中的`tasks`中有相应的`pid`,说明`offline-small-nginx`已加入到`offline-small`控制组中。
![图：验证](../images/kunpeng-qos-controller-pod-join-resctrl.png)

#### 验证Pod是否正确设置了qos_level

```bash
kubectl exec -it offline-small-nginx -- cat /sys/fs/cgroup/cpu/cpu.qos_level
```

可能的回显结果如下，可以看到`offline-small-nginx`的`cpu.qos_level`为`-1`,说明`offline-small-nginx`已加入到`offline-small`控制组中，且`cpu.qos_level`为`-1`
![图：验证](../images/kunpeng-qos-controller-pod-qos-level.png)

## 使用干扰检测功能

干扰检测功能用于在线、离线业务混部场景。干扰Agent监测在线业务的运行情况并识别干扰原因，Controller根据检测结果调整同一节点上的离线业务，降低离线业务对在线业务的影响。

### 部署干扰检测组件

使用该功能前，需要先部署包含Controller和干扰Agent的DaemonSet清单：

```text
config/kunpeng-qos-controller-config/samples/qos-controller-with-interference-agent-daemonset-v1alpha1.yaml
```

#### 构建WAAS Agent镜像

仓库根目录中的`Dockerfile.waasagent`用于构建干扰Agent镜像。该Dockerfile基于openEuler 24.03 LTS SP3，构建时会下载并安装以下组件：

- `libkperf v2.1.0`及其Python绑定。
- WAAS `waasagent-dev`分支及其Python依赖。

构建前需要安装Docker，并确保构建环境能够访问openEuler软件源、GitCode和Python软件源。在仓库根目录执行：

```bash
docker build \
  -f Dockerfile.waasagent \
  -t waas-agent:latest \
  .
```

构建完成后，执行以下命令确认镜像已生成：

```bash
docker image inspect waas-agent:latest
```

如果集群节点无法直接使用本地镜像，需要将镜像推送到集群可访问的镜像仓库：

```bash
docker tag waas-agent:latest <registry>/waas-agent:<tag>
docker push <registry>/waas-agent:<tag>
```

然后将DaemonSet清单中`interference-agent`容器的`image`修改为实际镜像地址：

```yaml
- name: interference-agent
  image: <registry>/waas-agent:<tag>
```

部署前，按照实际环境修改清单中的以下镜像：

- `kunpeng-qos-controller:0.1.0`：替换为实际使用的Controller镜像。
- `waas-agent:latest`：替换为实际使用的干扰Agent镜像。Agent镜像需要包含`libkperf`和默认的本地模型文件。

然后执行：

```bash
kubectl apply -f \
  config/kunpeng-qos-controller-config/samples/qos-controller-with-interference-agent-daemonset-v1alpha1.yaml

kubectl -n qos-system rollout status daemonset/qos-controller
```

该清单会在每个节点的`qos-controller` Pod中运行以下两个容器：

| 容器 | 作用 |
|---------|---------|
| **qos-controller** | 收集在线Pod信息、获取干扰原因，并根据检测结果调整离线业务。 |
| **interference-agent** | 监测在线Pod，分析节点干扰，并通过本地HTTP接口返回干扰原因。 |

清单已经启用干扰检测和处理功能，并将Controller配置为通过`http://127.0.0.1:18080`与同一Pod内的干扰Agent交互，不需要再手动添加启动参数。

部署完成后，确认两个容器都处于就绪状态：

```bash
kubectl -n qos-system get pods -l app=qos-controller
kubectl -n qos-system get pods -l app=qos-controller \
  -o jsonpath='{range .items[*]}{.metadata.name}{"  "}{.status.containerStatuses[*].name}{"\n"}{end}'
```

### 标识在线Pod和离线Pod

通过Pod标签`qos.kunpeng.huawei.com/workload-class`标识工作负载类型：

| 标签值 | 作用 |
|---------|------|
| **online** | Controller收集该Pod的cgroup路径并发送给干扰Agent，作为干扰检测对象。 |
| **offline** | 检测到干扰后，Controller调整该Pod所属节点的离线QoS策略。 |

在线Pod示例：

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: online-nginx
  labels:
    qos.kunpeng.huawei.com/workload-class: online
spec:
  containers:
  - name: nginx
    image: nginx:1.25
```

离线Pod示例：

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: offline-nginx
  labels:
    qos.kunpeng.huawei.com/workload-class: offline
spec:
  containers:
  - name: nginx
    image: nginx:1.25
```

也可以为已经创建的Pod添加标签：

```bash
kubectl label pod online-nginx qos.kunpeng.huawei.com/workload-class=online
kubectl label pod offline-nginx qos.kunpeng.huawei.com/workload-class=offline
```

### 干扰检测和处理流程

> 说明：干扰检测是一个独立功能，部署干扰Agent、采集在线Pod信息及获取干扰原因不依赖`cpu.qos_level`，也不要求配置`xint`或启用`SMT_TAG_PULL`调度特性。只有使用`cpu.qosLevel`调节离线Pod的CPU优先级时，才需要满足前文所述的内核版本和调度特性要求。

干扰检测和处理过程如下：

1. Controller收集本节点处于`Running`状态并带有`online`标签的Pod，将其cgroup路径定期发送给干扰Agent。
2. Controller定期从Agent获取本节点的干扰原因。
3. Controller将Agent的原始干扰原因映射为CPU、内存带宽或L3干扰。
4. 检测到干扰后，Controller创建或更新本节点的`qos-dynamic-offline-<node-name>` QoS策略，并将调节结果应用到带有`offline`标签的Pod。

在线Pod是干扰检测对象，其资源配置不会被该功能调整；实际被调整的是同一节点上带有`offline`标签的Pod。各类干扰的处理方式如下：

| Agent原始原因 | Controller分类 | 对离线Pod的处理 |
|---------|---------|---------|
| **base** | **none** | 表示没有干扰，不执行调节。 |
| **compute**、**l2**、**tlb**、**frontend** | **cpu** | 将离线策略的**cpu.qosLevel**设置为**-1**。 |
| **l3** | **l3** | 每次将离线策略的**l3.ways**减少**1**，并将**l3.max**减少**10**，最低为**1**。 |
| **membw** | **mb** | 每次将离线策略的**mb.max**减少**10**，最低为**1**。 |

### 通过日志查看干扰原因

使用以下命令查看Controller接收到的干扰原因：

```bash
kubectl -n qos-system logs \
  -l app=qos-controller \
  -c qos-controller \
  --prefix \
  --tail=200 | grep "dynamic-control received interference"
```

例如Agent返回的编号对应`compute`、`l2`、`l3`和`membw`时，可以看到以下日志：

```text
dynamic-control received interference reasons: node=k8s-master reasons=[compute l2 l3 membw]
dynamic-control received interference result: node=k8s-master reasons=[cpu mb l3]
```

第一行是Agent返回编号对应的原始原因名称，便于确认Agent的检测结果；第二行是Controller去重并映射后的干扰分类，也是实际用于调整离线业务的原因。

## 删除控制组示例

控制组的删除通过删除对应`QoSPolicy`完成，`QoSPolicy`删除后，Operator会在本节点清理同名`resctrl`控制组。

```bash
kubectl delete qospolicy offline-small
```

### 删除后验证

```bash
kubectl get qospolicy offline-small
POD=$(kubectl -n qos-system get pod -l app=qos-controller -o name | head -n1)
kubectl -n qos-system exec -it "${POD#pod/}" -- ls /sys/fs/resctrl
```

可能的回显结果如下，`qospolicy`查询不到对象，且`resctrl`目录中不存在`offline-small`，说明控制组已删除。

![图：删除QoSPolicy后控制组清理结果](../images/kunpeng-qos-controller-delete.png)

## 常用清单路径

- CRD：
   `config/kunpeng-qos-controller-config/crd/bases/qos.kunpeng.huawei.com_qospolicies.yaml`
- 部署示例：
   `config/kunpeng-qos-controller-config/samples/qos-controller-daemonset-v1alpha1.yaml`
- 干扰检测部署示例：
   `config/kunpeng-qos-controller-config/samples/qos-controller-with-interference-agent-daemonset-v1alpha1.yaml`
- WAAS Agent镜像构建文件：
   `Dockerfile.waasagent`
- QoSPolicy示例：
   `config/kunpeng-qos-controller-config/samples/qospolicy-examples-v1alpha1.yaml`
- Pod示例：
   `config/kunpeng-qos-controller-config/samples/pod-examples-for-qospolicy-v1alpha1.yaml`

# 维护特性

## 卸载软件

卸载建议按“先业务资源、再控制面资源”的顺序执行，避免残留对象。

1. 清理QoSPolicy CR。

    ```bash
    kubectl delete qospolicy --all
    ```

    如果你使用了特定命名的策略文件，也可以按文件删除：

    ```bash
    kubectl delete -f config/kunpeng-qos-controller-config/samples/qospolicy-examples-v1alpha1.yaml
    ```

2. 清理使用QoS的业务Pod（可选）。

    ```bash
    kubectl delete -f config/kunpeng-qos-controller-config/samples/pod-examples-for-qospolicy-v1alpha1.yaml
    ```

3. 卸载Operator（DaemonSet + RBAC + ServiceAccount + Namespace）。

    ```bash
    kubectl delete -f config/kunpeng-qos-controller-config/samples/qos-controller-daemonset-v1alpha1.yaml
    ```

4. 删除CRD。

    ```bash
    kubectl delete -f config/kunpeng-qos-controller-config/crd/bases/qos.kunpeng.huawei.com_qospolicies.yaml
    ```

5. 验证清理结果。

    ```bash
    kubectl get crd | grep qospolicies.qos.kunpeng.huawei.com
    kubectl get qospolicy
    kubectl -n qos-system get all
    ```

    可能的回显结果如下，可以看到查询不到相应的资源，说明已成功清理。
    ![图：验证结果](../images/kunpeng-qos-controller-clean.png)

    说明：如果已删除CRD，`kubectl get qospolicy`可能提示资源类型不存在，这是预期行为。

# 修订记录

|文档版本| 发布日期 | 修改说明 |
|--------| -------- | -------- |
| 01 | 2026-09-30 | 第一次正式发布。 |
