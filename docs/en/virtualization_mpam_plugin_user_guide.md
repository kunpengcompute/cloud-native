# MPAM Plugin User Guide

## Introduction<a name="EN-US_TOPIC_0000002518691938"></a>

This document describes how to install and use the Memory System Resource Partitioning and Monitoring (MPAM) plugin on a Kunpeng server.

**MPAM Plugin Overview<a name="section1577519106461"></a>**

The MPAM plugin is a tool used to enable the MPAM feature for Kubernetes. You can create resource groups and use them to isolate and monitor resources when creating Pods.

To do so, you can run the MPAM plugin in the form of a DaemonSet on each node that supports the MPAM feature. The MPAM plugin provides the following functions:

- Indicates whether a node supports the MPAM feature. The label (for example `mpam:enabled`) is used by the scheduler in cluster scheduling.
- Monitors ConfigMaps in the Kubernetes cluster using the list-watch mechanism and automatically applies the new configuration to corresponding nodes after a ConfigMap is added or updated.
- Monitors Pods and automatically adds them to resource groups when necessary.

**Concept and Mechanism of MPAM<a name="section116731497501"></a>**

MPAM is a technology used in the AArch64 architecture to solve performance problems caused by resource sharing (memory bandwidth and L3 cache) on a server.

MPAM copes with the problem that the performance of some key applications or the overall system performance deteriorates due to resource contention (such as the cache, DMC, or interconnect) when different types of services are deployed on a server. It adds different labels (such as PARTID and PMG) to all requests of different service flows in the CPU or system memory management units (SMMUs) so that the hardware can detect the service flows. Based on the label information, system resources (such as the cache and DMC bandwidth) can be dynamically allocated to isolate service flows from each other and reduce mutual interference.

After MPAM is enabled for Kubernetes, you can specify a resource group when creating a Pod. As a result, resources accessed by an application are isolated from those accessed by other applications. In this way, resource usage is monitored and resource utilization is improved. In addition, you can analyze the monitoring data to locate the causes of performance deterioration of key applications and take countermeasures in a timely manner.

**Application Scenarios<a name="section692815178512"></a>**

When deploying online and offline services together, you can use the MPAM feature to limit resources for offline services while ensuring the performance of online services.

- MPAM addresses the requirement for prioritizing response to online services.
    - Container cloud customers generally deploy online services for real-time loads and offline services for non-real-time loads (for example offline settlement), and they want that response to online services must be ensured at the same time offline services are running.
    - In most scenarios, offline service containers are created periodically and destroyed after tasks are executed. They do not have fixed locations and may be deployed together with any service.

- Offline services compete with online services for L3 cache and memory bandwidth. As a result, the response to online services may be delayed.

    Customer's container scheduling platforms try to ensure that both online and offline services are allocated sufficient CPU and memory resources. However, offline and online services deployed on the same node contend for L3 cache and memory bandwidth, and the performance of online services may deteriorate.

    >![](public_sys-resources/icon-note.gif) **NOTE:**
    >In hybrid deployment environments, online service performance may be impacted by multiple factors. Since MPAM isolation specifically addresses the memory subsystem, it proves particularly effective for online services that are sensitive to memory bandwidth. However, it is crucial to maintain offline service CPU utilization at moderate levels, ideally between 10% to 20%, as excessive utilization could significantly disrupt online service performance.

## Installation and Usage<a name="EN-US_TOPIC_0000002550131799"></a>

### Environment Requirements<a name="EN-US_TOPIC_0000002518532050"></a>

Before installing the MPAM plugin, ensure that the environment meets the hardware and software requirements. The hardware environment includes CPUs, memory, and drives. The software environment includes the OS and applications.

**Hardware Requirements<a name="section1172912110256"></a>**

[**Table 1**](#hardware-requirements) lists the hardware requirements.

**Table 1** Hardware requirements<a id="hardware-requirements"></a>

|Item|Description|
|--|--|
|Server|Kunpeng server|
|CPU|Kunpeng 920 series processor or Kunpeng 950 processor|
|System drive|No special requirements|

**OS and Software Requirements<a name="section2794612"></a>**

[**Table 2**](#os-and-software-requirements) lists the OS and software requirements.

**Table 2** OS and software requirements<a id="os-and-software-requirements"></a>

|Item|Version|How to Obtain|
|--|--|--|
|OS|openEuler 22.03 LTS SP2 or later|[Link](https://repo.openeuler.org/openEuler-22.03-LTS-SP4/ISO/aarch64/)|
|Docker|18.09.0 or later|Install it using a Yum repository.|
|containerd|1.7.14 or later|Install it using a Yum repository.|
|Kubernetes|1.23.1 or later|Install it using a Yum repository.|
|k8s-mpam-controller source code|-|[Link](https://gitee.com/kunpeng_compute/kunpeng-cloud-computing)|

### Installing the MPAM Plugin<a name="EN-US_TOPIC_0000002550011791"></a>

To install the MPAM plugin, obtain its source code and create an image file, mount the MPAM feature to the physical machine, and run the plugin. Unless otherwise specified, the following operations are performed on the master node.

1. Obtain the source code.

    ```shell
    git clone https://gitcode.com/boostkit/cloud-native.git
    ```

2. Go to the MPAM plugin directory and create the image file `k8s-mpam-controller:0.1.0`.

    ```shell
    cd cloud-native/Boostkit_CloudNative/K8S
    make mpam-docker
    ```

3. View the created image file.

    ```shell
    docker images | grep k8s-mpam-controller
    ```

    The following information is displayed. The values of `CREATED` and `SIZE` may vary depending on the actual environment.

    ```txt
    REPOSITORY                          TAG               IMAGE ID       CREATED         SIZE
    k8s-mpam-controller                 0.1.0               9f363522bbc9   42 hours ago    259MB
    ```

    If the cluster uses containerd as the runtime, run the following commands to import the image to the containerd image repository:

    ```shell
    docker save k8s-mpam-controller:0.1.0 -o k8s-mpam-controller.tar
    ctr -n k8s.io images import k8s-mpam-controller.tar
    ```

4. On the physical machine of the worker node, mount the MPAM feature.

    ```shell
    mount -t resctrl resctrl /sys/fs/resctrl
    ```

5. Go to the `samples` directory and edit the `k8s-mpam-controller.yaml` file.

    >![](public_sys-resources/icon-note.gif) **NOTE:**
    >The `k8s-mpam-controller.yaml` file is used to start the MPAM plugin. In this file, a service account named `mpam-controller-agent` is created and is assigned the resource access permission, which allows the service account to access kube-apiserver. This file deploys the MPAM plugin as a DaemonSet on each node in the cluster to enable the MPAM feature. The dynamic MPAM isolation function requires the SYS_ADMIN permission. The plugin user is usually a cluster administrator who has the cluster management permission.

    1. Open the MPAM plugin configuration file.

        ```shell
        cd k8s-mpam-controller-config/samples
        vi k8s-mpam-controller.yaml
        ```

    2. Press `i` to enter the insert mode and change the content following `image:` in the file to the name and version of the compiled image file (`k8s-mpam-controller:0.1.0`). The content of `k8s-mpam-controller.yaml` is as follows:

        ```yaml
        apiVersion: v1
        kind: ServiceAccount
        metadata:
          name: mpam-controller-agent
        ---
        apiVersion: rbac.authorization.k8s.io/v1
        kind: ClusterRole
        metadata:
          name: mpam-controller-agent
        rules:
          - apiGroups:
              - ""
            resources:
              - configmaps
              - pods
            verbs:
              - get
              - list
              - watch
          - apiGroups:
              - ""
            resources:
              - nodes
            verbs:
              - get
              - list
              - patch
              - update
              - watch
        ---
        apiVersion: rbac.authorization.k8s.io/v1
        kind: ClusterRoleBinding
        metadata:
          name: mpam-controller-agent
        roleRef:
          apiGroup: rbac.authorization.k8s.io
          kind: ClusterRole
          name: mpam-controller-agent
        subjects:
          - kind: ServiceAccount
            name: mpam-controller-agent
            namespace: default
        ---
        apiVersion: apps/v1
        kind: DaemonSet
        metadata:
          name: mpam-controller-daemonset-agent
        spec:
          selector:
            matchLabels:
              app: k8s-mpam-controller-agent
          template:
            metadata:
              labels:
                app: k8s-mpam-controller-agent
            spec:
              serviceAccountName: mpam-controller-agent
              hostPID: true
          
              containers:
                - name: k8s-mpam-controller-agent
                  image: k8s-mpam-controller:0.1.0
                  imagePullPolicy: IfNotPresent
                  securityContext:
                    capabilities:
                      add:
                        - SYS_ADMIN
                  command: ["/usr/bin/agent"]
                  args: ["-direct"]
                  env:
                    - name: NODE_NAME
                      valueFrom:
                        fieldRef:
                          apiVersion: v1
                          fieldPath: spec.nodeName
                  volumeMounts:
                    - name: resctrl
                      mountPath: /sys/fs/resctrl/
                    - name: hostname
                      mountPath: /etc/hostname
                    - name: sysfs
                      mountPath: /sys/fs/cgroup/
              volumes:
                - name: resctrl
                  hostPath:
                    path: /sys/fs/resctrl/
                - name: hostname
                  hostPath:
                    path: /etc/hostname
                - name: sysfs
                  hostPath:
                    path: /sys/fs/cgroup/
        ```

    3. Press `Esc` to exit the insert mode. Type `:wq!` and press `Enter` to save the file and exit.

6. Apply the `k8s-mpam-controller.yaml` file to run the MPAM plugin.

    ```shell
    kubectl apply -f k8s-mpam-controller.yaml
    ```

    >![](public_sys-resources/icon-note.gif) **NOTE:**
    >After the `k8s-mpam-controller.yaml` file is applied, Kubernetes creates a Pod on each node to run the MPAM plugin. The number of Pods that can be created on each node decreases by one.

7. Check whether the Pod corresponding to the MPAM plugin is running properly.

    ```shell
    kubectl get pods
    ```

    The following information is displayed if the Pod is running properly:

    ```txt
    NAME                                    READY   STATUS    RESTARTS   AGE
    mpam-controller-daemonset-agent-bj2gv   1/1     Running   0          143m
    ```

8. View the run logs of the MPAM plugin. In this example, *xxx* indicates the name of the Pod corresponding to the MPAM plugin.

    ```shell
    kubectl logs -f xxx
    ```

### Creating an MPAM Resource Group<a name="EN-US_TOPIC_0000002518691942"></a>

To limit resources for a Pod, you need to create an MPAM resource group.

**Procedure<a name="section20415535311"></a>**

1. Go to the `samples` directory and modify the configuration file (in .yaml format) of the MPAM resource group. The following uses `example-config.yaml` as an example.

    In the `example-config.yaml` file, a node resource group may have any of the three configurations, as described in [**Table 1**](#configuration-types). You can use ConfigMaps to create a configuration for a node or a group of nodes. After the configuration is created, the MPAM plugin manages the ConfigMaps in the Kubernetes cluster and automatically applies the configuration to the corresponding nodes after a ConfigMap is added or updated.

    **Table 1** Configuration types<a id="configuration-types"></a>

    |Configuration Type|Configuration Name|Description|
    |--|--|--|
    |Node configuration|rc-config.node.{NODE_NAME}|Provides the configuration of the node named *Node_NAME*.|
    |Node group configuration|rc-config.group.{GROUP_NAME}|You can use the <code>ngroup</code> label to add a node to a node group. For example, if a node contains the <code>ngroup=grp1</code> label, the node belongs to the node group <code>grp1</code>. If the <code>ConfigMap rc-config.node.{NODE_NAME}</code> for a node does not exist but the node belongs to the *GROUP_NAME* node group, the ConfigMap named <code>rc-config.group.{GROUP_NAME}</code> is applied to this node.|
    |Default configuration|rc-config.default|If a node does not belong to any node group and the corresponding ConfigMap does not exist, the ConfigMap named <code>rc-config.default</code> is applied to this node.|
 
2. Open the file.

        ```shell
        cd samples
        vi example-config.yaml
        ```

3. Press `i` to enter the insert mode. Set the `name` field to the actual configuration name in [**Table 1**](#configuration-types) and add the resource group information to the `mpam` field.

        ```yaml
        apiVersion: v1
        kind: ConfigMap
        metadata:
          name: ${CONFIG_NAME}
          namespace: rc-config
        data:
          rc.conf: |
            mpam:
              group1:
                llc: <schemata>
                mb: <schemata>
              group2:
                llc: <schemata>
                mb: <schemata>
              group3:
                llc: <schemata>
                mb: <schemata>
        ```

    >![](public_sys-resources/icon-note.gif) **NOTE:**
        >-   Replace the schemata following `llc` with the restriction on the L3 cache and the schemata following `mb` with the bandwidth restriction. Set the values based on your requirements. For details about the complete configuration of the `example-config.yaml` file, see the following example for reference.
        >-   A maximum of 32 resource groups can be configured. (The root group occupies one resource group by default, and a maximum of 31 new resource groups can be created under the root group.) Each schemata must comply with the syntax rules.
        >-   If an item is not configured in a resource group or a configuration item does not meet the syntax rules, the resource group uses the default configuration of the configuration item. The default L3 cache configuration is `"L3:0=fffffff;1=fffffff;2=fffffff;3=fffffff"` and the default bandwidth configuration is `"MB:0=100;1=100;2=100;3=100"`. If `mbHdl` is selected for mounting, the default `Hard Limit` configuration is `"MBHDL:0=1;1=1;2=1;3=1"`.

4. Press `Esc` to exit the insert mode. Type `:wq!` and press `Enter` to save the file and exit.

5. In the `samples` directory, use the `example-config.yaml` file to create a ConfigMap.

    ```shell
    kubectl apply -f example-config.yaml
    ```

6. On the node, go to the `/sys/fs/resctrl` directory and check whether a resource group has been created and whether the resource group configuration matches the `example-config.yaml` file.

    ```shell
    cd /sys/fs/resctrl
    ls
    ```

    >![](public_sys-resources/icon-note.gif) **NOTE:**
    >For example, you can run the following command to view the configuration of the resource group `group1`:
    >
    >```shell
    >cat group1/schemata
    >```

**Example<a name="section13967105919315"></a>**

The following example of the `example-config.yaml` file shows the configuration items of the MPAM resource group.

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: rc-config
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: rc-config.default
  namespace: rc-config
data:
  rc.conf: |
    mpam:
      group1:
        llc: "L3:0=1ff;1=1ff;2=1ff;3=1ff"
        mb: "MB:0=10;1=10;2=10;3=10"
      group2:
        llc: "L3:0=3ff;1=3ff;2=3ff;3=3ff"
        mb: "MB:0=20;1=20;2=20;3=20"
      group3:
        llc: "L3:0=7ff;1=7ff;2=7ff;3=7ff"
        mb: "MB:0=30;1=30;2=30;3=30"
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: rc-config.group.clx
  namespace: rc-config
data:
  rc.conf: |
    mpam:
      group1:
        llc: "L3:0=1ff;1=1ff;2=1ff;3=1ff"
        mb: "MB:0=40;1=40;2=40;3=40"
      group2:
        llc: "L3:0=3ff;1=3ff;2=3ff;3=3ff"
        mb: "MB:0=50;1=50;2=50;3=50"
      group3:
        llc: "L3:0=7ff;1=7ff;2=7ff;3=7ff"
        mb: "MB:0=60;1=60;2=60;3=60"
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: rc-config.group.icx
  namespace: rc-config
data:
  rc.conf: |
    mpam:
      group1:
        llc: "L3:0=1f;1=1f;2=1f;3=1f"
      group2:
        llc: "L3:0=3f;1=3f;2=3f;3=3f"
      group3:
        llc: "L3:0=ff;1=ff;2=ff;3=ff"
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: rc-config.node.master
  namespace: rc-config
data:
  rc.conf: |
    mpam:
      group1:
        llc: "L3:0=1ff;1=1ff;2=1ff;3=1ff"
        mb: "MB:0=70;1=70;2=70;3=70"
      group2:
        llc: "L3:0=3ff;1=3ff;2=3ff;3=3ff"
        mb: "MB:0=80;1=80;2=80;3=80"
      group3:
        llc: "L3:0=7ff;1=7ff;2=7ff;3=7ff"
        mb: "MB:0=90;1=90;2=90;3=90"
```

### Creating a Pod and Adding It to a Resource Group<a name="EN-US_TOPIC_0000002518532046"></a>

To add a Pod to a resource group, specify the resource group when creating the Pod.

1. Modify the Pod configuration file (in .yaml format). The following uses `example-pod.yaml` as an example.
    1. Go to the `samples` directory and open the `example-pod.yaml` file.

        ```shell
        cd samples
        vi example-pod.yaml
        ```

    2. Press `i` to enter the insert mode and add the following content to the file:

        ```yaml
        labels:
            rcgroup: group2
        ```

        ```yaml
        nodeSelector:
            MPAM: enabled
        ```

        >![](public_sys-resources/icon-note.gif) **NOTE:**
        >- In the `labels` field, set the `rcgroup` field to specify the associated resource group. For example, add the Pod to `group2`.
        >- Add `MPAM: enabled` to the `nodeSelector` field so that the scheduler can schedule the Pod to a node that supports the MPAM feature.

        The updated `example-pod.yaml` file has the following content:

        ```yaml
        apiVersion: v1
        kind: Pod
        metadata:
          name: nginx
          labels:
            rcgroup: group2
        spec:
          containers:
          - name: nginx
            image: nginx:1.16.1
            ports:
            - containerPort: 80
              hostPort: 8088
          nodeSelector:
            MPAM: enabled
        ```

    3. Press `Esc` to exit the insert mode. Type `:wq!` and press `Enter` to save the file and exit.

2. Create a Pod.

    ```shell
    kubectl apply -f example-pod.yaml
    ```

3. On the node, go to the `/sys/fs/resctrl` directory and then the owning resource group (for example `group1`) of the Pod. You can view the configuration and monitoring data in the resource group and the PIDs of the restricted applications in the current resource group.

    ```shell
    cd /sys/fs/resctrl/group1
    ```

    - Run the following command to view the configuration of the resource group:

        ```shell
        cat schemata
        ```

    - Run the following command to view the PIDs of the resource group:

        ```shell
        cat tasks
        ```

    - Run the following command to view the monitoring data of the resource group:

        ```shell
        grep . mon_data/*
        ```

### Using Dynamic MPAM Isolation<a name="EN-US_TOPIC_0000002550131803"></a>

You can use the dynamic MPAM isolation function to adjust the resource usage of some offline services. **Note that dynamic isolation cannot be used together with static MPAM resource group creation.**

**(Optional) Configuring Dynamic MPAM Isolation Parameters<a name="section4141521171610"></a>**

The plugin provides default configurations. Dynamic MPAM isolation can be used without configuring ConfigMaps. Dynamic MPAM isolation parameters are configured in a JSON file. For details about the parameters, see [**Table 1**](#dynamic-mpam-isolation-parameters). To manually change the configurations, refer to the following content.

```yaml
 {
      "mpamConfig":{
        "adjustInterval": 5000,
        "perfDuration": 3000,
        "l3Percent": {
          "low": 20,
          "high": 50
        },
        "memBandPercent": {
          "low": 10,
          "high": 50
        },
        "cacheMiss": {
          "minMiss": 10,
          "maxMiss": 50
        }
      }
    }
```

**Table 1** Dynamic MPAM isolation parameters<a id="dynamic-mpam-isolation-parameters"></a>

|Parameter|Description|
|--|--|
|adjustInterval|Interval between dynamic adjustments. For example, if this parameter is set to <code>1000</code>, dynamic adjustment is performed every second.|
|perfDuration|Perf collection duration. For example, if this parameter is set to <code>1000</code>, perf will collect data within one second each time.|
|l3Percent|Maximum and minimum L3 cache percentages that can be used by offline services during dynamic adjustment. For example, if <code>low</code> is set to <code>20</code> and <code>high</code> is set to <code>50</code>, offline services can use at least 20% and at most 50% of L3 CacheWay during dynamic adjustment.|
|memBandPercent|Maximum and minimum memory bandwidth percentages that can be used by offline services during dynamic adjustment. For example, if <code>low</code> is set to <code>10</code> and <code>high</code> is set to <code>50</code>, offline services can use at least 10% and at most 50% of the memory bandwidth during dynamic adjustment.|
|cacheMiss|Basis for determining whether to perform dynamic adjustment. For example, if <code>minMiss</code> is set to <code>10</code> and <code>maxMiss</code> is set to <code>50</code>, the available resources of offline services are reduced when the cache miss rate of online services is greater than 50%, and are increased when the cache miss rate of online services is less than 10%.|

A JSON file is configured in the form of ConfigMap in the `k8s-mpam-controller.yaml` file. The complete YAML file is as follows:

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: mpam-controller-agent
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: mpam-controller-agent
rules:
- apiGroups:
  - ""
  resources:
  - configmaps
  - pods
  verbs:
  - get
  - list
  - watch
- apiGroups:
  - ""
  resources:
  - nodes
  verbs:
  - get
  - list
  - patch
  - update
  - watch
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: mpam-controller-agent
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: mpam-controller-agent
subjects:
- kind: ServiceAccount
  name: mpam-controller-agent
  namespace: default
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: mpam-config
data:
  config.json: |
    {
      "mpamConfig":{
        "adjustInterval": 10000,
        "perfDuration": 3000,
        "l3Percent": {
          "low": 20,
          "high": 50
        },
        "memBandPercent": {
          "low": 10,
          "high": 50
        },
        "cacheMiss": {
          "minMiss": 10,
          "maxMiss": 30
        }
      }
    }
---
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: mpam-controller-daemonset-agent
spec:
  selector:
    matchLabels:
      app: k8s-mpam-controller-agent
  template:
    metadata:
      labels:
        app: k8s-mpam-controller-agent
    spec:
      serviceAccountName: mpam-controller-agent
      hostPID: true
      containers:
      - name: k8s-mpam-controller-agent
        image: k8s-mpam-controller:0.1
        securityContext:
          capabilities:
            add:
              - SYS_ADMIN
        command: ["/usr/bin/agent"]
        args: ["-direct"]
        resources:
          limits:
            memory: 200Mi
          requests:
            cpu: 100m
            memory: 200Mi
        env:
        - name: NODE_NAME
          valueFrom:
            fieldRef:
              apiVersion: v1
              fieldPath: spec.nodeName
        volumeMounts:
        - name: resctrl
          mountPath: /sys/fs/resctrl/
        - name: hostname
          mountPath: /etc/hostname
        - name: sysfs
          mountPath: /sys/fs/cgroup/
        - name: config-volume
          mountPath: /var/lib/mpam-config
      volumes:
      - name: resctrl
        hostPath:
          path: /sys/fs/resctrl/
      - name: hostname
        hostPath:
          path: /etc/hostname
      - name: sysfs
        hostPath:
          path: /sys/fs/cgroup/
      - name: config-volume
        configMap:
          name: mpam-config
          items:
            - key: config.json
              path: config.json
```

After the dynamic MPAM isolation function is enabled, the plugin creates the `mpam-controller_dynamic` directory in the `/sys/fs/resctrl` directory, as shown in the following figure.

![](figures/en-us_image_0000002518532054.png)

**Deploying Offline Services<a name="section114717555501"></a>**

1. Add the annotation `kunpeng.com/offline: "true"` to the YAML file of the Pod to label the Pod as an offline service so that the plugin can restrict the Pod. The following is a `bw-mem.yaml` file example.

    ```yaml
    apiVersion: v1
    kind: Pod
    metadata:
      name: bw-mem
      annotations:
        kunpeng.com/offline: "true"
    spec:
      containers:
      - name: bw-mem
        image: bw-mem:latest
        imagePullPolicy: IfNotPresent
        command: [ "/bin/sh", "-c", "--" ]
        args: [ "while true; do sleep 300000; done;" ]
        securityContext:
          capabilities:
            add: ["ALL"]
        resources:
          requests:
            cpu: "9.6"
          limits:
            cpu: "9.6"
    ```

2. Deploy the offline service to be restricted.

    ```shell
    kubectl apply -f bw-mem.yaml
    ```

    After the deployment is successful, the PID of the offline service is added to tasks in the `mpam-controller_dynamic` control group.

3. Run the following commands to check PIDs of restricted offline services:

    ```shell
    cd /sys/fs/resctrl/mpam-controller_dynamic
    cat tasks
    ```

### (Optional) Uninstalling the Plugin<a name="EN-US_TOPIC_0000002550011793"></a>

You can uninstall the plugin if it is no longer required.

```shell
kubectl delete -f k8s-mpam-controller.yaml
```

## Acronyms and Abbreviations<a name="EN-US_TOPIC_0000002518691944"></a>

|**Acronym/Abbreviation**|**Full Spelling**|
|--|--|
|MPAM|Memory System Resource Partitioning and Monitoring|
