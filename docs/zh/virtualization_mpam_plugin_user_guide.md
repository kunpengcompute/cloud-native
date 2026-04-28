# MPAM插件 用户指南

## 介绍<a name="ZH-CN_TOPIC_0000002518691938"></a>

本文主要介绍如何在鲲鹏服务器上安装和使用MPAM插件。

**MPAM插件简介<a name="section1577519106461"></a>**

MPAM（Memory System Resource Partitioning and Monitoring，内存系统资源分区与监控）插件是用于Kubernetes使能MPAM特性的工具，可以配置资源组并在创建Pod时指定资源隔离和监控。

如果要在创建Pod时指定资源组对其进行资源隔离和监控，可以以DaemonSet的形式在每个支持MPAM特性的Node节点上运行MPAM插件。MPAM插件实现的主要功能如下：

- 标记Node节点是否支持MPAM特性，被打上的标签（如**mpam:enabled**）用于调度器调度时使用。
- 通过list-watch机制监控kubernetes集群中的ConfigMap，并在添加或更新ConfigMap后将配置自动应用到相应的节点。
- 监控Pods并在需要时自动将其加入到对应的资源组中。

**MPAM概念和机制介绍<a name="section116731497501"></a>**

MPAM是一种在ARM64架构下用于解决服务器系统中不同类型业务共享资源（内存带宽、L3Cache）时带来性能问题的技术。

为了解决服务器系统中混合部署不同类型业务时，由于共享资源（如Cache、DMC或Interconnect）的竞争导致某些关键应用性能下降或者系统整体性能下降的问题，MPAM通过在CPU或SMMU源头给不同业务流的所有请求打上不同的标签（如PARTID或PMG），使得硬件能够感知到业务流，基于标签信息，实现在系统资源的各个组件（如Cache容量或DMC带宽）动态分配资源，实现不同业务流的隔离，降低干扰。

Kubernetes使能MPAM特性后，可以在创建Pod时指定对应的资源组，从而实现不同应用进程访问资源的隔离和监控，提高资源的利用率。此外，还可以通过分析监控数据得到影响关键应用的性能下降的原因，及时做出调整。

**应用场景<a name="section692815178512"></a>**

在线业务和离线业务混合部署场景下，可以使用MPAM特性对离线业务进行资源限制，保障在线业务的性能。

- 容器云客户期望离线业务与在线业务混合部署时，在线业务的业务响应及时性不受影响。
    - 容器云客户包括在线业务和离线业务混合部署的场景。在线业务处理实时负载，离线业务处理非实时负载（如离线结算等），要求离线业务运行时不得影响在线业务的业务响应及时性。
    - 离线业务容器一般定时创建，任务执行完后销毁，所以容器部署的具体位置是不确定的，可能会与任何业务混合部署。

- 离线业务与在线业务竞争L3 Cache和内存带宽可能导致在线业务的响应及时性下降。

    客户的容器调度平台会保障在线业务和离线业务均有足够的CPU/内存资源，但是对于部署到同一个节点的离线业务和在线业务仍会竞争L3 Cache的容量和内存带宽，离线类业务可能会大量占用L3 Cache和内存带宽从而影响在线业务的实时性和性能。

    >![](public_sys-resources/icon-note.gif) **说明：** 
    >混合部署场景中在线业务的性能会受到各种因素的干扰，MPAM的隔离只针对内存子系统，因此对于内存带宽敏感型的在线业务效果会比较好，同时也要确保离线业务的CPU利用率不要过高，否则对在线业务的干扰会比较严重，建议在10%\~20%左右。

## 安装和使用插件<a name="ZH-CN_TOPIC_0000002550131799"></a>

### 环境要求<a name="ZH-CN_TOPIC_0000002518532050"></a>

在安装MPAM插件之前，需要确保使用环境均满足要求，包括硬件和软件配置。硬件配置包括CPU、内存和磁盘等。软件配置包括操作系统和应用程序。

**硬件要求<a name="section1172912110256"></a>**

环境的硬件构成如[**表 1** 硬件配置要求](#硬件配置要求)所示。

**表 1** 硬件配置要求<a id="硬件配置要求"></a>

|项目|物理机配置|
|--|--|
|服务器|鲲鹏服务器|
|CPU|鲲鹏920系列处理器、鲲鹏950处理器|
|系统盘|无特殊要求|


**系统及软件版本<a name="section2794612"></a>**

操作系统及软件要求如[**表 2** 操作系统及软件配置要求](#操作系统及软件配置要求)所示。

**表 2** 操作系统及软件配置要求<a id="操作系统及软件配置要求"></a>

|类目|版本|获取方式|
|--|--|--|
|操作系统|openEuler 22.03 （LTS-SP2）及以上|[获取链接](https://repo.openeuler.org/openEuler-22.03-LTS-SP4/ISO/aarch64/)|
|Docker|18.09.0及以上|通过配置Yum源安装|
|Containerd|1.7.14及以上|通过配置Yum源安装|
|Kubernetes|1.23.1及以上|通过配置Yum源安装|
|k8s-mpam-controller源码|-|[获取链接](https://gitcode.com/boostkit/cloud-native)|


### 安装MPAM插件<a name="ZH-CN_TOPIC_0000002550011791"></a>

安装MPAM插件，首先需要获取MPAM插件的源码并生成镜像文件，然后挂载MPAM特性到物理机上，最后运行MPAM插件。若无特殊说明，则以下操作均在master节点完成。

1. 获取源码。

    ```
    git clone https://gitcode.com/boostkit/cloud-native.git
    ```

2. 进入MPAM插件目录，生成镜像文件“k8s-mpam-controller:0.1.0”。

    ```
    cd cloud-native/Boostkit_CloudNative/K8S
    make mpam-docker
    ```

3. 查看生成的镜像文件。

    ```
    docker images | grep k8s-mpam-controller
    ```

    回显结果如下所示，其中CREATED、SIZE数值可能根据实际环境会有不同。

    ```
    REPOSITORY                          TAG               IMAGE ID       CREATED         SIZE
    k8s-mpam-controller                 0.1.0               9f363522bbc9   42 hours ago    259MB
    ```

    如果集群使用的runtime是containerd，则执行如下命令把镜像导入containerd的镜像仓库。

    ```
    docker save k8s-mpam-controller:0.1.0 -o k8s-mpam-controller.tar
    ctr -n k8s.io images import k8s-mpam-controller.tar
    ```

4. 在worker节点的物理机上挂载MPAM特性。

    ```
    mount -t resctrl resctrl /sys/fs/resctrl
    ```

5. 进入samples目录，配置k8s-mpam-controller.yaml文件。

    >![](public_sys-resources/icon-note.gif) **说明：** 
    >k8s-mpam-controller.yaml文件是用于启动MPAM插件的配置文件，文件中创建了一个名为mpam-controller-agent的ServiceAccount，并赋予其访问资源的权限，用于插件运行过程中访问kube-apiserver。此外，该文件会以DaemonSet的形式将MPAM插件部署在集群中的每一个Node节点上，从而使能MPAM特性。动态MPAM隔离功能需要插件开启SYS\_ADMIN权限，本插件的使用者一般为拥有集群管理权限的集群管理者。

    1. 打开MPAM插件配置文件。

        ```
        cd k8s-mpam-controller-config/samples
        vi k8s-mpam-controller.yaml
        ```

    2. 按“i”进入编辑模式，将文件中的`image: `修改为编译得到的镜像文件名称和版本（k8s-mpam-controller:0.1.0）。k8s-mpam-controller.yaml的内容如下：

        ```
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

    3. 按“Esc”键退出编辑模式，输入 **:wq!**，按“Enter”键保存并退出文件。

6. 应用k8s-mpam-controller.yaml文件以运行MPAM插件。

    ```
    kubectl apply -f k8s-mpam-controller.yaml
    ```

    >![](public_sys-resources/icon-note.gif) **说明：** 
    >应用k8s-mpam-controller.yaml文件后，K8s会在每个Node节点上创建一个Pod运行MPAM插件，节点上可创建Pod的数量也会相应地减少一个。

7. 查看MPAM插件对应的Pod是否正常运行。

    ```
    kubectl get pods
    ```

    正常运行的回显如下。

    ```
    NAME                                    READY   STATUS    RESTARTS   AGE
    mpam-controller-daemonset-agent-bj2gv   1/1     Running   0          143m
    ```

8. 查看MPAM插件运行的日志。在本例中，xxx指MPAM插件对应的Pod名。

    ```
    kubectl logs -f xxx
    ```


### 创建MPAM资源组<a name="ZH-CN_TOPIC_0000002518691942"></a>

当需要对Pod进行资源限制时，需要创建MPAM资源组。

**操作步骤<a name="section20415535311"></a>**

1. 进入“samples”目录，修改MPAM资源组的配置文件（.yaml格式），以example-config.yaml为例。

    example-config.yaml文件中，Node资源组包括如[**表 1** 配置类型及其说明](#配置类型及其说明)所示3种不同级别的配置，用户可以通过ConfigMap为Node节点或一组节点创建配置。创建配置后，MPAM插件将管理Kubernetes集群中的ConfigMap，并在添加或更新ConfigMap后将配置自动应用到相应的节点。

    **表 1** 配置类型及其说明<a id="配置类型及其说明"></a>

   |配置类型|配置名称|配置说明|
   |--|--|--|
   |node配置|rc-config.node.{NODE_NAME}|该配置提供了名为Node_NAME的节点的配置。|
   |node group配置|rc-config.group.{GROUP_NAME}|可以通过“ngroup”标签将Node节点加到对应的组中。例如，如果某个Node节点含有“ngroup=grp1”的标签，那么该节点就属于Node组grp1。如果Node节点特定的ConfigMap rc-config.node.{NODE_NAME}不存在，但节点属于名为{GROUP_NAME}的节点组，则将应用名称为rc-config.group.{GROUP_NAME}的ConfigMap。|
   |默认配置|rc-config.default|如果节点不属于任何节点组，并且节点特定的ConfigMap不存在，则将应用名称为rc-config.default的ConfigMap。|
 
 1. 打开文件。

       ```
        cd samples
        vi example-config.yaml
       ```

 2. 按“i”进入编辑模式，在文件中修改name字段指定为[**表 1** 配置类型及其说明](#配置类型及其说明)中的实际配置名称，将mpam字段下添加对应的资源组信息：

       ```
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

    >![](public_sys-resources/icon-note.gif) **说明：** 
        >-   将llc中的schemata替换为对L3 Cache的限制，将mb中的schemata替换为对带宽的限制，请根据实际情况进行修改。example-config.yaml文件的完整配置样例请参见下方参考示例。
        >-   最多可以设置32个资源组（根分组默认占一个资源组，根分组下最多实际只能创建出31个资源组），每条<schemata\>必须要满足语法规则。
        >-   如果某个资源组中没有对某一项进行配置或者已配置的配置项不满足语法规则，该资源组将使用该配置项的默认配置。L3 cache的默认配置为 **"L3:0=fffffff;1=fffffff;2=fffffff;3=fffffff"** ；带宽的默认配置为 **"MB:0=100;1=100;2=100;3=100"** ；如果挂载的时候选择了mbHdl参数，Hard Limit的默认配置为 **"MBHDL:0=1;1=1;2=1;3=1"** 。

    3. 按“Esc”键退出编辑模式，输入 **:wq!**，按“Enter”键保存并退出文件。

2. 在samples目录下，应用example-config.yaml文件以创建ConfigMap。

    ```
    kubectl apply -f example-config.yaml
    ```

3. 在Node节点上，进入“/sys/fs/resctrl”目录，查看资源组是否已创建，以及对应的资源组配置是否和example-config.yaml中的一致。

    ```
    cd /sys/fs/resctrl
    ls
    ```

    >![](public_sys-resources/icon-note.gif) **说明：** 
    >例如，可以通过以下命令查看资源组group1的配置。
    >```
    >cat group1/schemata
    >```

**参考示例<a name="section13967105919315"></a>**

以下为example-config.yaml配置样例，样例中展示了MPAM资源组的配置项。

```
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

### 创建Pod并指定资源组<a name="ZH-CN_TOPIC_0000002518532046"></a>

当需要将某个Pod加入到某个资源组中时，需要在创建Pod时指定资源组。

1. 修改Pod的配置文件（.yaml格式），以example-pod.yaml为例。
    1. 进入samples目录，打开example-pod.yaml文件。

        ```
        cd samples
        vi example-pod.yaml
        ```

    2. 按“i”进入编辑模式，在配置文件中分别添加如下信息：

        ```
        labels:
            rcgroup: group2
        ```

        ```
        nodeSelector:
            MPAM: enabled
        ```

        >![](public_sys-resources/icon-note.gif) **说明：** 
        >-   在**labels**字段中通过rcgroup字段指定对应的资源组，例如将Pod加到**group2**中。
        >-   在**nodeSelector**字段中增加**MPAM：enabled**，用于调度器将该Pod调度到支持MPAM特性的节点上去。

        修改后的example-pod.yaml文件如下所示。

        ```
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

    3. 按“Esc”键退出编辑模式，输入 **:wq!**，按“Enter”键保存并退出文件。

2. 创建Pod。

    ```
    kubectl apply -f example-pod.yaml
    ```

3. 在Node节点上，进入“/sys/fs/resctrl”目录，再进入Pod所属的资源组中（例如Pod属于资源组group1），可以在资源组中查看对应的配置以及监控数据，还可以查看当前资源组下被限制应用的pid。

    ```
    cd /sys/fs/resctrl/group1
    ```

    - 通过以下命令查看资源组的配置。

        ```
        cat schemata
        ```

    - 通过以下命令查看该资源组下的pid。

        ```
        cat tasks
        ```

    - 通过以下命令查看资源组下的监控数据。

        ```
        grep . mon_data/*
        ```


### 使用动态MPAM隔离功能<a name="ZH-CN_TOPIC_0000002550131803"></a>

当需要动态调整某些离线业务的资源使用量时可以使用动态MPAM隔离功能。**注意动态隔离功能不能与静态创建MPAM资源组同时使用。**

**（可选）配置动态MPAM隔离参数<a name="section4141521171610"></a>**

插件提供了默认配置，如果不配置ConfigMap同样可以使用动态MPAM隔离功能。MPAM动态隔离参数通过json文件配置，参数含义请参见[**表 1** MPAM动态隔离参数说明](#MPAM动态隔离参数说明)。如果需要手动更改配置参考下面内容进行配置。

```
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

**表 1** MPAM动态隔离参数说明<a id="MPAM动态隔离参数说明"></a>

|参数名|参数含义|
|--|--|
|adjustInterval|每次动态调整之间的间隔，比如设置值为1000就是每隔1s执行一次动态调整。|
|perfDuration|perf采集指标的时间，比如设置1000就是每次perf采集就采集1s的数据。|
|l3Percent|动态调整过程中离线业务可以使用的L3Cache的最大值和最小值。比如设置low=20，high=50，那么在动态调整过程中离线业务最少可以使用20%的L3 CacheWay，最多可以使用50%的L3 CacheWay。|
|memBandPercent|动态调整过程中离线业务可以使用的内存带宽的最大值和最小值。比如设置low=10，high=50，那么在动态调整过程中离线业务最少可以使用10%的内存带宽，最多可以使用50%的内存带宽。|
|cacheMiss|是否进行动态调整的判别依据。比如设置minMiss=10，maxMiss=50，那么当在线业务的Cache Miss率大于50%时就降低离线业务的可用资源量，当在线业务的Cache Miss率小于10%时就增加离线业务的可用资源量。|


json文件通过ConfigMap的形式进行配置，在k8s-mpam-controller.yaml文件中配置，完整的yaml文件如下。

```
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

使用动态MPAM隔离功能后插件会在“/sys/fs/resctrl”目录下创建mpam-controller\_dynamic目录，如下图所示。

![](figures/zh-cn_image_0000002518532054.png)

**部署离线业务<a name="section114717555501"></a>**

1. 在Pod的yaml文件中添加注解kunpeng.com/offline: "true"，标记该Pod为离线业务，方便插件对其进行限制，示例bw-mem.yaml文件如下。

    ```
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

2. 部署要进行限制的离线业务。

    ```
    kubectl apply -f bw-mem.yaml
    ```

    部署成功后离线业务的pid会被加入到mpam-controller\_dynamic控制组中的tasks。

3. 通过以下命令查看被限制的离线业务的pid。

    ```
    cd /sys/fs/resctrl/mpam-controller_dynamic
    cat tasks
    ```


### （可选）插件卸载<a name="ZH-CN_TOPIC_0000002550011793"></a>

当不需要使用此插件时，可以使用如下命令卸载插件。

```
kubectl delete -f k8s-mpam-controller.yaml
```


## 缩略语<a name="ZH-CN_TOPIC_0000002518691944"></a>

|**缩略语**|**英文全称**|**中文全称**|
|--|--|--|
|MPAM|Memory System Resource Partitioning and Monitoring|内存系统资源分区与监控|



