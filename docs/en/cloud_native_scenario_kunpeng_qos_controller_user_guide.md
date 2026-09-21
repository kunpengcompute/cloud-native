# Kunpeng QoS Controller User Guide

<!-- md-trans-meta sourceCommit=9fd7c94e23a6aa60d2544882f494e60f3ae46551 translatedAt=2026-09-17T10:36:49.624Z pushedAt=2026-09-18T02:45:10.508Z -->

## Introduction

`kunpeng-qos-controller` is a local node operator running in DaemonSet mode. After a user declares a resource control policy using `QoSPolicy`, the controller creates and configures a `resctrl` control group on the matching node, binds the pod process to the control group, and sets the container `cgroup` parameters as required. In this way, node-level QoS resource management and control are implemented.

Its main capabilities are as follows:

- Listens to `QoSPolicy` custom resources (CRs) and creates, updates, or deletes the corresponding `resctrl` control groups on the node.
- Binds a pod to the target control group (using the pod label `qos.kunpeng.huawei.com/group`).
- Adjusts the resource configuration of offline workloads (`qos.kunpeng.huawei.com/workload-class=offline`) based on the interference detection result.
- Controls `cpu.qos_level` of the group pod via the `cpu.qosLevel` field of `QoSPolicy`.

## Application Scenarios

- The local `resctrl` of nodes in a Kubernetes cluster needs to be managed and controlled in a unified manner.
- Different QoS policies need to be allocated to pods based on service types (for example, offline and online tasks are isolated from each other).
- Policies need to be declared using custom resource definitions (CRDs), and the operator automatically creates control groups and delivers parameters.

## Environment Requirements<a name="EN-US_TOPIC_0000002518412470"></a>

This document provides guidance based on specific environments. Before performing operations, ensure that your hardware and software meet the requirements.

**Hardware Requirements<a name="section8140133619490"></a>**

[**Table 1**](#hardware-requirement) lists the hardware requirement.

**Table 1** Hardware requirement<a id="hardware-requirement"></a>

|Item|Specification|
|--|--|
|CPU|New Kunpeng 920 processor model or Kunpeng 950 processor|

**OS and Software Requirements<a name="section21631338357"></a>**

The OS and software versions listed here have been verified. Other versions can also use this plugin, but you need to adapt them yourself. If Kubernetes or containerd is not installed in your environment, install it as described in [**Table 2**](#verified-os-and-software-versions).

**Table 2** Verified OS and software versions<a id="verified-os-and-software-versions"></a>

| Software | Version | How to Obtain |
|--|--|--|
| OS | openEuler 24.03 LTS SP2 | [Link](https://www.openeuler.org/en/download/#openEuler%2024.03%20LTS%20SP2) |
| Kubernetes | 1.25.16 | See [Kubernetes Deployment Guide (CentOS & openEuler)](https://www.hikunpeng.com/document/detail/en/kunpengcpfs/ecosystemEnable/Kubernetes/kunpengk8s_04_0001.html). |
| containerd | 1.7.10 | See [Containerd Installation Guide (CentOS 8.1 and openEuler 20.03)](https://www.hikunpeng.com/document/detail/en/kunpengcpfs/ecosystemEnable/Containerd/kunpengcontainerd_03_0001.html). |
| Docker | 18.0.9 | Install it using Yum. |
| kunpeng-qos-controller | 1.0.0 | [Link](https://gitcode.com/boostkit/cloud-native) |

## Principles

The system consists of two main Reconcilers:

- `QoSPolicyReconciler`:
  - Listens to `QoSPolicy`.
  - Determines whether a policy needs to take effect on the current node based on `nodeSelector`.
  - Creates or updates `resctrl` control groups for the matching node and writes the policy to `schemata`.
  - Performs local control group clearance when deleting the policy.
- `PodBindingReconciler`:
  - Listens to pods.
  - Adds a pod process to the specified control group based on the pod label `qos.kunpeng.huawei.com/group`.
  - Writes `cpu.qos_level` to the pod container based on `cpu.qosLevel` of the target `QoSPolicy`.

In the current implementation, `QoSPolicy.metadata.name` and the `resctrl` control group name are the same.

# Software Compilation

Run the following commands in the root directory of the repository.

## Local Binary Compilation

```bash
make kunpeng-qos-controller-build
```

Output path:

 `bin/kunpeng-qos-controller`

## Docker Image Compilation

```bash
make kunpeng-qos-controller-docker
```

NOTE: The image is built using `Dockerfile.kunpeng-qos-controller`.

If `containerd` is used for cluster runtime, you can export the image and then import it to the target node:

```bash
docker save kunpeng-qos-controller:0.1.0 -o kunpeng-qos-controller.tar
```

After copying `kunpeng-qos-controller.tar` to the target node, run the following command:

```bash
ctr -n k8s.io images import /path/to/kunpeng-qos-controller.tar
```

# Software Deployment

Prerequisites: The target node has the `resctrl` capability and `/sys/fs/resctrl` is mounted to the node. The container can access `/sys/fs/cgroup`.
>NOTE: If `/sys/fs/resctrl` is not mounted to the node, mount it by running the `mount -t resctrl resctrl /sys/fs/resctrl` command. If the `resctrl` directory does not exist in the `/sys/fs` directory, the MPAM function is not enabled in the kernel. You can enable the function by adding `arm64.mpam` to the kernel startup parameters.
>
## Deploying CRDs

```bash
kubectl apply -f config/kunpeng-qos-controller-config/crd/bases/qos.kunpeng.huawei.com_qospolicies.yaml
```

## Deploying the Operator (DaemonSet + RBAC)

```bash
kubectl apply -f config/kunpeng-qos-controller-config/samples/qos-controller-daemonset-v1alpha1.yaml
```

## Checking the Running Status

```bash
kubectl -n qos-system get pod -l app=qos-controller -o wide
kubectl -n qos-system logs -l app=qos-controller --tail=200
```

The possible command output is as follows. The status of `qos-controller` is `Running`, and no error is reported in the log.
![Figure: Checking the running status](figures/docs_images_kunpeng_qos_controller_deploy.png)

## (Optional) Local Debugging and Running

```bash
export NODE_NAME=<Your_node_name>
./bin/kunpeng-qos-controller
  --kubeconfig ~/.kube/config
```

# Feature Usage

## Creating a Policy and Delivering It to Nodes

> NOTE: `cpu.qos_level` depends on the kernel capability and requires the kernel version to be `6.6.0-154.0.0` or later.
> In addition, you need to add `xint` to the kernel startup parameters and run the following command to enable the scheduling feature after the system is started:
> `echo SMT_TAG_PULL > /sys/kernel/debug/sched/features`

### QoSPolicy Field Description

| Field| Meaning| Range/Default Value| Description|
|---------|---------|---------|---------|
| `spec.nodeSelector` | Nodes where the policy takes effect.| Map (optional)| The policy is applied only to the nodes with matched node labels.|
| `spec.mb.hdl` | MBHDL switch.| 0–1. The default value is `1`.| Generally, `1` indicates that the function is enabled.|
| `spec.mb.pri` | Memory bandwidth (MB) priority.| 0–7. The default value is `3`.| A larger value indicates a higher priority.|
| `spec.mb.min` | Minimum MB guarantee ratio.| 0–100. The default value is `0`.| Presented in percentages.|
| `spec.mb.max` | Maximum MB limit ratio.| 0–100. The default value is `100`.| Presented in percentages.|
| `spec.l3.pri` | L3 priority.| 0–3. The default value is `0`.| Controls the L3 priority.|
| `spec.l3.min` | Minimum L3 guarantee ratio.| 0–100. The default value is `0`.| Presented in percentages.|
| `spec.l3.max` | Maximum L3 limit ratio.| 0–100. The default value is `100`.| Presented in percentages.|
| `spec.l3.ways` | Number of allocated cache ways.| ≥ 1| The upper limit is determined by node hardware.|
| `spec.cpu.qosLevel` | Pod CPU QoS level.| `-1`/`0`/`1`. The default value is `0`.| `cpu.qos_level` written to the pod in the group. The value <code>-1</code> indicates a low priority, <code>0</code> indicates the default priority, and <code>1</code> indicates a high priority.|

### Example: Creating QoSPolicy

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

Application:

```bash
kubectl apply -f qospolicy-offline-small.yaml
```

### Updating the Control Group Configuration (by Updating QoSPolicy)

The control group parameters are updated by modifying the `QoSPolicy` CR with the same name. The operator automatically synchronizes the updated configuration to the local `resctrl` control group.

```bash
kubectl edit qospolicy offline-small
```

You can modify the corresponding fields (such as `mb.max`, `l3.ways`, and `cpu.qosLevel`) under `spec` on the editing page, save the modification, and exit to trigger the update.

An example command output is as follows:
![Figure: Example of modifying QoSPolicy using kubectl edit](figures/docs_images_kunpeng_qos_controller_update_resctrl.png)
In this example, `mb.max` is changed from `60` to `50`, and `l3.ways` is changed from `4` to `1`.
> NOTE: `cpu.qos_level` can be modified only once. The default value is `0`. The value can be changed from `0` to `-1` or from `0` to `1`. However, the value cannot be changed after being set.
>
#### Verifying the Update

```bash
kubectl get qospolicy offline-small -o yaml
POD=$(kubectl -n qos-system get pod -l app=qos-controller -o name | head -n1)
kubectl -n qos-system exec -it "${POD#pod/}" -- cat /sys/fs/resctrl/offline-small/schemata
```

The possible command output is as follows. You can see that `mb.max` and `l3.ways` in the CR have been updated to `50` and `1`, respectively, and the values in `schemata` of the `resctrl` control group have been updated accordingly.
![Figure: Verifying the update](figures/docs_images_kunpeng_qos_controller_update_ensure.png)

## Adding a Pod to a Specified Control Group

### Description

- `QoSPolicy.metadata.name = offline-small`
- The control group directory is `/sys/fs/resctrl/offline-small`.
- The pod label `qos.kunpeng.huawei.com/group` must be the same as the policy name.

### Example: Creating a Pod with the group Label

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

Application:

```bash
kubectl apply -f pod-offline-small.yaml
```

### Verification

#### Checking Whether a Pod Is Added to a Specified Control Group

```bash
POD=$(kubectl -n qos-system get pod -l app=qos-controller -o name | head -n1)
kubectl -n qos-system exec -it "${POD#pod/}" -- cat /sys/fs/resctrl/offline-small/tasks
```

The possible command output is as follows. You can see that there is a corresponding `pid` in `tasks` of the `offline-small` control group, indicating that `offline-small-nginx` has been added to the `offline-small` control group.  
![Figure: Verification](figures/docs_images_kunpeng_qos_controller_pod_join_resctrl.png)

#### Checking Whether qos_level Is Correctly Set for a Pod

```bash
kubectl exec -it offline-small-nginx -- cat /sys/fs/cgroup/cpu/cpu.qos_level
```

The possible command output is as follows. `cpu.qos_level` of `offline-small-nginx` is `-1`, indicating that `offline-small-nginx` has been added to the `offline-small` control group.  
![Figure: Verification](figures/docs_images_kunpeng_qos_controller_pod_qos_level.png)

## Interference Detection

The interference detection function is used in hybrid deployment scenarios with both online and offline services. The interference-detection agent monitors the running status of online workloads and identifies the cause of interference. Based on the detection result, the controller adjusts the offline workloads on the same node to reduce the impact of offline workloads on online workloads.

### Deploying the Interference Detection Component

Before using this feature, deploy the DaemonSet manifest that contains the controller and the interference-detection agent.

```text
config/kunpeng-qos-controller-config/samples/qos-controller-with-interference-agent-daemonset-v1alpha1.yaml
```

#### Building the WAAS Agent Image

The `Dockerfile.waasagent` in the repository root directory is used to build the interference-detection agent image. This `Dockerfile` is based on openEuler 24.03 LTS SP3 and downloads and installs the following components during the build:

- `libkperf v2.1.0` and its Python bindings.
- WAAS `waasagent-dev` branch and its Python dependencies.

Before building, install Docker and ensure that the build environment can access the openEuler software repository, GitCode, and the Python software repository. Run the following commands in the repository root directory:

```bash
docker build \
  -f Dockerfile.waasagent \
  -t waas-agent:latest \
  .
```

After the build is complete, run the following command to confirm that the image has been generated:

```bash
docker image inspect waas-agent:latest
```

If the cluster nodes cannot directly use the local image, push the image to an image repository accessible to the cluster.

```bash
docker tag waas-agent:latest <registry>/waas-agent:<tag>
docker push <registry>/waas-agent:<tag>
```

Then, change the `image` of the `interference-agent` container in the DaemonSet manifest to the actual image address.

```yaml
- name: interference-agent
  image: <registry>/waas-agent:<tag>
```

Before deployment, modify the following images in the manifest according to the actual environment:

- `kunpeng-qos-controller:0.1.0`: Replace it with the controller image actually used.
- `waas-agent:latest`: Replace it with the interference-detection agent image actually used. The agent image must contain `libkperf` and the default local model files.

Then, execute the following commands:

```bash
kubectl apply -f \
  config/kunpeng-qos-controller-config/samples/qos-controller-with-interference-agent-daemonset-v1alpha1.yaml

kubectl -n qos-system rollout status daemonset/qos-controller
```

This manifest runs the following two containers in the `qos-controller` pod on each node:

| Container | Function |
|---------|---------|
| `qos-controller` | Collects online pod information, obtains the interference cause, and adjusts offline services based on the detection result. |
| `interference-agent` | Monitors online pods, analyzes node interference, and returns the interference cause through a local HTTP interface. |

The manifest enables the interference detection and processing feature and configures the controller to interact with the interference-detection agent in the same pod through `http://127.0.0.1:18080`. Therefore, you do not need to manually add startup parameters.

After the deployment is complete, confirm that both containers are in the ready state.

```bash
kubectl -n qos-system get pods -l app=qos-controller
kubectl -n qos-system get pods -l app=qos-controller \
  -o jsonpath='{range .items[*]}{.metadata.name}{"  "}{.status.containerStatuses[*].name}{"\n"}{end}'
```

### Identifying Online and Offline Pods

Use the pod label `qos.kunpeng.huawei.com/workload-class` to identify the workload type.

| Label Value | Function |
|---------|------|
| `online` | The controller collects the cgroup path of this pod and sends it to the interference-detection agent as the interference detection target. |
| `offline` | After interference is detected, the controller adjusts the offline QoS policy of the node to which this pod belongs. |

Online pod example:

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

Offline pod example:

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

You can also add labels to pods that have already been created.

```bash
kubectl label pod online-nginx qos.kunpeng.huawei.com/workload-class=online
kubectl label pod offline-nginx qos.kunpeng.huawei.com/workload-class=offline
```

### Interference Detection and Processing Flow

> Description: Interference detection is an independent function. Deploying the interference-detection agent, collecting online pod information, and obtaining interference causes do not depend on `cpu.qos_level`, nor do they require configuring `xint` or enabling the `SMT_TAG_PULL` scheduling feature. It is necessary to meet the kernel version and scheduling feature requirements described earlier only when `cpu.qosLevel` is used to adjust the CPU priority of offline pods.

The interference detection and processing flow is as follows:

1. The controller collects the pods on the local node that are in the `Running` state and carry the `online` label, and periodically sends their cgroup paths to the interference-detection agent.
2. The controller periodically obtains the interference causes of the local node from the agent.
3. The controller maps the agent's raw interference causes to CPU, memory bandwidth, or L3 interference.
4. After the plugin is deployed, it creates a QoS policy named `qos-dynamic-offline-<node-name>` to control the resource allocation of offline pods.
5. After interference is detected, the controller updates the `qos-dynamic-offline-<node-name>` QoS policy of the local node and applies the adjustment result to the pods carrying the `offline` label.

> If you need to clean up the `qos-dynamic-offline-<node-name>` control group, you can manually delete the control group in the `/sys/fs/resctrl` directory after uninstalling the plugin.

Online pods are the targets of interference detection, and their resource configuration is not adjusted by this function. What is actually adjusted is the resource configuration of the pods carrying the `offline` label on the same node. The handling of each type of interference is as follows:

| Raw Cause from the Agent | Classification from the Controller | Handling for Offline Pods |
|---------|---------|---------|
| `base` | `none` | Indicates no interference and thus no adjustment. |
| `compute`, `l2`, `tlb`, or `frontend` | `cpu` | Sets `cpu.qosLevel` in the offline policy to `-1`. |
| `l3` | `l3` | Reduces `l3.ways` and `l3.max` in the offline policy by `1` and `10`, respectively, each time. Their minimum values are `1`. |
| `membw` | `mb` | Reduces `mb.max` in the offline policy by `10` each time. Its minimum value is `1`. |

### Viewing Interference Causes Through Logs

Run the following commands to view the interference causes received by the controller:

```bash
kubectl -n qos-system logs \
  -l app=qos-controller \
  -c qos-controller \
  --prefix \
  --tail=200 | grep "dynamic-control received interference"
```

For example, when the numbers returned by the agent correspond to `compute`, `l2`, `l3`, and `membw`, you can see the following logs:

```text
dynamic-control received interference reasons: node=k8s-master reasons=[compute l2 l3 membw]
dynamic-control received interference result: node=k8s-master reasons=[cpu mb l3]
```

The first line is the original cause name corresponding to the number returned by the agent, which helps confirm the agent's detection result. The second line is the interference classification after the controller deduplicates and maps the causes, which is also the actual cause used to adjust offline services.

## Example of Deleting a Control Group

To delete a control group, you need to delete the corresponding `QoSPolicy`. After `QoSPolicy` is deleted, the operator will clear the `resctrl` control group with the same name on the local node.

```bash
kubectl delete qospolicy offline-small
```

### Verifying the Deletion

```bash
kubectl get qospolicy offline-small
POD=$(kubectl -n qos-system get pod -l app=qos-controller -o name | head -n1)
kubectl -n qos-system exec -it "${POD#pod/}" -- ls /sys/fs/resctrl
```

The possible command output is as follows. If no object is found in `qospolicy` and `offline-small` does not exist in the `resctrl` directory, the control group has been deleted.

![Figure: Clearance result after QoSPolicy deletion](figures/docs_images_kunpeng_qos_controller_delete.png)

## Common Path List

- CRD:
   `config/kunpeng-qos-controller-config/crd/bases/qos.kunpeng.huawei.com_qospolicies.yaml`
- Deployment example:
   `config/kunpeng-qos-controller-config/samples/qos-controller-daemonset-v1alpha1.yaml`
- Interference detection deployment example:
   `config/kunpeng-qos-controller-config/samples/qos-controller-with-interference-agent-daemonset-v1alpha1.yaml`
- WAAS agent image build file:
   `Dockerfile.waasagent`
- `QoSPolicy` example:
   `config/kunpeng-qos-controller-config/samples/qospolicy-examples-v1alpha1.yaml`
- Pod example:
   `config/kunpeng-qos-controller-config/samples/pod-examples-for-qospolicy-v1alpha1.yaml`

# Feature Maintenance

## Uninstalling Software

You are advised to uninstall service resources and then control-plane resources to avoid residual objects.

1. Clear the `QoSPolicy` CRs.

    ```bash
    kubectl delete qospolicy --all
    ```

    If you use policy files with specific names, you can delete CRs by file:

    ```bash
    kubectl delete -f config/kunpeng-qos-controller-config/samples/qospolicy-examples-v1alpha1.yaml
    ```

2. (Optional) Clear service pods that use QoS.

    ```bash
    kubectl delete -f config/kunpeng-qos-controller-config/samples/pod-examples-for-qospolicy-v1alpha1.yaml
    ```

3. Uninstall the operator (DaemonSet + RBAC + ServiceAccount + namespace).

    ```bash
    kubectl delete -f config/kunpeng-qos-controller-config/samples/qos-controller-daemonset-v1alpha1.yaml
    ```

4. Delete CRDs.

    ```bash
    kubectl delete -f config/kunpeng-qos-controller-config/crd/bases/qos.kunpeng.huawei.com_qospolicies.yaml
    ```

5. Verify the clearing result.

    ```bash
    kubectl get crd | grep qospolicies.qos.kunpeng.huawei.com
    kubectl get qospolicy
    kubectl -n qos-system get all
    ```

    The possible command output is as follows. If the corresponding resources cannot be found, the resources have been successfully cleared.  
    ![Figure: Verification result](figures/docs_images_kunpeng_qos_controller_clean.png)

    NOTE: If CRDs have been deleted, `kubectl get qospolicy` may display a message indicating that the resource type does not exist, which is expected.

# Change History

|Version| Date | Description |
|--------| -------- | -------- |
| 01 | 2026-09-30 | This is the first official release. |
