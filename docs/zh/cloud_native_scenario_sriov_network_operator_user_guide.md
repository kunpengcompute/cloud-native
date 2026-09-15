# SR-IOV Network Operator虚拟机内容器低时延 用户指南

## 简介

SR-IOV Network Operator（后文简称Operator）是用于在Kubernetes（简称K8s）集群中配置和管理SR-IOV网络设备的组件，可集中管理SR-IOV网卡设备，并部署和协调SR-IOV CNI、Device Plugin等组件。本仓库的Operator基于[上游社区Operator](https://github.com/k8snetworkplumbingwg/sriov-network-operator)进行了相关拓展，可在虚拟机内的K8s集群中部署、发现直通入虚拟机的VF，并根据策略将VF作为可调度网络资源提供给Pod。

本文介绍虚拟机内容器低时延部署与测试方案，主要内容包括：在鲲鹏服务器的虚拟机内部署Kubernetes集群，将ConnectX-5网卡的VF通入虚拟机，并通过本仓库的SR-IOV Network Operator组件将VF提供给qperf容器。最后通过`qperf`测试两个容器与两台物理机之间的TCP和UDP时延，评估虚拟机内容器场景中的网络时延开销。

本方案操作步骤概览：

1. 两台物理机分别开启GICv4.1,配置内存大页。
2. 两台物理机分别为一个物理网卡PF创建多个VF（至少3个）。
3. 两台物理机机上创建三台虚拟机，分别作为虚拟机内K8s集群的master节点和两台计算节点（分别部署在两台物理机）。
4. master节点虚拟机通入一个VF，用于Kubernetes集群网络；计算节点虚拟机通入两个VF：一张用于Kubernetes集群网络，另一张用于容器数据网络。
5. 三台虚拟机基于上述用于集群网络的VF建立K8s集群。
6. 部署Operator及其相关组件，Operator自动发现并纳管其他可用的VF，且可将其提供给指定容器。
7. 部署测试容器，Operator为测试容器通入VF作为网络接口并赋予网络IP。
8. 基于相关IP在测试容器中测试TCP和UDP时延，并与物理机测试结果对比，计算损耗。

为降低测试过程中的时延波动，测试环境还进行了以下操作：

- 计算节点虚拟机的CPU、内存亲和：将VCPU与内存绑定在宿主机CX5网卡亲和的NUMA节点对应的cpu和内存节点。
- 将虚拟机内的网卡中断绑定到CPU `4-7`。
- 通过Linux cgroup的`cpuset.cpus`文件，将qperf容器限制在CPU `0-7`上运行。

本文的配置分别从以下三个方向优化时延：

- SR-IOV直通让容器绕过虚拟机和主机的软件网络栈，数据直接进出物理网卡，去掉虚拟网桥和多次内存拷贝带来的开销。
- NUMA对齐保证CPU访问内存和网卡时走本地通路，避免跨节点访问的额外延迟。
- 中断绑核与容器绑核将处理网卡中断和运行qperf进程的CPU固定下来，减少调度迁移和缓存失效造成的抖动。

后文各步骤会在对应位置再次说明具体操作。

### 必要概念

本文主要使用[**表1** 必要概念](#必要概念)中的几个必要概念。后文第一次执行相关操作时还会再次说明。

<!-- markdownlint-disable-next-line MD033 -->
**表1** 必要概念<a id="必要概念"></a>

| 名称 | 含义 |
| --- | --- |
| 宿主机 | 运行QEMU/KVM虚拟机的物理服务器（需要两台，并通过CX5网卡直连） |
| master节点 | 管理Kubernetes集群的虚拟机，主要执行kubectl和部署命令 |
| compute节点 | 运行测试容器的计算虚拟机 |
| Pod | Kubernetes运行容器的基本单元；本文的每个qperf Pod中有一个qperf容器 |
| PF | 物理网卡端口，例如宿主机上的ConnectX-5端口 |
| VF | 从PF划分出来的虚拟网卡，可独立分配给虚拟机或容器 |
| NUMA节点 | 一组距离相近的CPU和内存；网卡、虚拟机CPU和虚拟机内存位于同一节点时，访问路径更短 |

## 环境要求

本文基于[**表2** 硬件要求](#硬件要求)、[**表3** 资源规格](#资源规格)和[**表4** 软件要求](#软件要求)所示环境完成实操验证。使用其他规格或版本时，需要自行确认兼容性。

<!-- markdownlint-disable-next-line MD033 -->
**表2** 硬件要求<a id="硬件要求"></a>

| 项目 | 已验证规格 |
| --- | --- |
| CPU | 鲲鹏920新型号处理器、鲲鹏950处理器 |
| 网卡 | Mellanox ConnectX-5 |

<!-- markdownlint-disable-next-line MD033 -->
**表3** 资源规格<a id="资源规格"></a>

| 对象 | 数量 | CPU | 内存 | 用途 |
| --- | --- | --- | --- | --- |
| master虚拟机 | 1台 | 64 vCPU | 128 GiB | Kubernetes管理节点 |
| compute虚拟机 | 2台 | 64 vCPU | 128 GiB | 运行低时延测试容器 |
| qperf容器 | 2个 | 8 CPU | 16 GiB | 执行网络时延测试 |

<!-- markdownlint-disable-next-line MD033 -->
**表4** 软件要求<a id="软件要求"></a>

| 软件 | 已验证版本 | 获取方式 |
| --- | --- | --- |
| 虚拟机OS | openEuler 24.03 LTS SP3 | [下载链接](https://www.openeuler.org/zh/download/?version=openEuler%2024.03%20LTS%20SP3) |
| QEMU | 8.2.0 | yum安装 |
| libvirt | 9.10 | yum安装 |
| Kubernetes | 1.28.14 | 参考[拉取镜像](#拉取sealos集群镜像) |
| Flannel | 0.25.6 | 参考[拉取镜像](#拉取sealos集群镜像) |
| CNI plugins | 1.5.1 | [下载链接](https://github.com/containernetworking/plugins/releases/download/v1.5.1/cni-plugins-linux-arm64-v1.5.1.tgz) |
| Helm | 3.9.4 | 参考[拉取镜像](#拉取sealos集群镜像) |
| sealos | 5.1.1 | [获取链接](https://sealos.run/docs/advanced/k8s/getting-started/install-cli) |
| Multus | 4.x（multi-daemonset.yaml） | [获取链接](https://github.com/k8snetworkplumbingwg/multus-cni/tree/master/deployments/multus-daemonset.yml) |
| Whereabouts | latest | [获取链接](https://github.com/k8snetworkplumbingwg/whereabouts) |
| Go | 1.23.4 | [下载链接](https://golang.google.cn/dl/go1.23.4.linux-arm64.tar.gz) |
| operator-sdk | 1.42.3 | [获取链接](https://sdk.operatorframework.io/docs/installation/) |
| SR-IOV Network Operator | 本仓库sriov-network-operator分支 | [获取源码](https://gitcode.com/boostkit/cloud-native/tree/sriov-network-operator) |

### 选择部署方式

本文同时介绍在线部署和离线部署。两种方式部署的组件和最终功能相同，区别在于目标Kubernetes集群是否能够直接获取安装包、源码和容器镜像。

- **在线部署**：master节点和compute节点能够访问本文使用的代码仓库、软件源和镜像仓库，或者能够访问已经同步所需内容的内部镜像仓库。此时可以在目标环境中直接下载安装包和拉取镜像，不需要执行镜像导出、复制、导入等离线操作。
- **离线部署**：目标Kubernetes集群因安全策略、网络隔离或生产环境访问限制，无法访问公网代码仓库、软件源或镜像仓库。此时需要使用一台可访问互联网且架构兼容的联网服务器，提前下载源码、安装包和镜像，将其导出为文件后传入目标环境，再在master节点和compute节点中导入。

离线部署的意义是让目标集群在不访问公网的条件下完成部署，满足隔离网络和安全合规要求。离线部署不会改变SR-IOV网络功能，也不会带来额外的性能收益。

根据目标环境选择部署方式：

- 目标节点无法访问本文使用的任一公共仓库，且没有包含全部依赖的内部仓库时，使用离线部署。
- 目标节点能够访问公共仓库，或能够从内部仓库直接获取全部依赖时，使用在线部署。使用内部仓库时，按实际地址替换本文的公共仓库地址即可。
- 只有master节点可以访问仓库、compute节点不能访问时，仍应按离线方式为compute节点准备并导入镜像。

> **说明：**
> 后文标题或步骤中标有“仅离线部署”的操作，只用于下载、导出、复制或导入所需源码或镜像。在线部署可以跳过这些操作，直接执行后续通用部署步骤。离线部署中的“联网服务器”不属于目标Kubernetes集群，仅用于准备可传入隔离环境的文件。

准备环境时可参考以下资料。

- [openEuler 24.03 LTS SP3镜像下载](https://www.openeuler.org/zh/download/?version=openEuler%2024.03%20LTS%20SP3)
- [openEuler 24.03 LTS SP3虚拟化环境准备](https://docs.openeuler.org/zh/docs/24.03_LTS_SP3/virtualization/virtulization_platform/virtulization/environment_preparation.html)
- [openEuler 24.03 LTS SP3虚拟化组件安装](https://docs.openeuler.org/zh/docs/24.03_LTS_SP3/virtualization/virtulization_platform/virtulization/virtualization_installation.html)
- [openEuler 24.03 LTS SP3虚拟机管理](https://docs.openeuler.org/zh/docs/24.03_LTS_SP3/virtualization/virtulization_platform/virtulization/managing_vms.html)
- [openEuler 24.03 LTS SP3虚拟机配置](https://docs.openeuler.org/zh/docs/24.03_LTS_SP3/virtualization/virtulization_platform/virtulization/vm_configuration.html)
- [sealos CLI安装指南](https://sealos.run/docs/advanced/k8s/getting-started/install-cli)
- [Multus项目](https://github.com/k8snetworkplumbingwg/multus-cni)
- [Whereabouts项目](https://github.com/k8snetworkplumbingwg/whereabouts)

## 操作前准备

1. 获取本仓库代码。

    - 在线部署时，在**master节点和用于编译镜像的AArch64服务器**上直接克隆代码。
    - 离线部署时，在联网服务器上克隆代码，再将完整的`cloud-native`目录复制到master节点。

    ```bash
    git clone https://gitcode.com/boostkit/cloud-native.git --branch sriov-network-operator
    cd cloud-native/sriov-network-operator
    ```

2. 准备三台openEuler 24.03 LTS SP3虚拟机。

    - 一台master节点，通入一个VF，用于Kubernetes集群网络（称为“集群VF”）。
    - 两台compute节点，分别运行qperf server和qperf client，各通入两个VF：一个用于Kubernetes集群网络，一个用于容器数据网络（称为“容器VF”）。

    三台虚拟机的集群VF处于同一个网络（例如`192.168.100.0/24`），Kubernetes节点通过集群VF互相通信。两台compute节点的容器VF处于另一个网络（例如`10.56.217.0/24`），供qperf容器直接通信。

    两个compute节点必须部署在不同物理机上，使两个qperf容器之间的测试流量经过物理网络。master节点不要求独占物理机，可以与其中一个compute节点部署在同一台物理机上（为了避免资源争抢，需要绑定不同的vCPU和内存节点）。例如，将`sno-master`和`sno-compute01`部署在物理机A，将`sno-compute02`部署在物理机B。

    **组网示例**

    ```text
                                Kubernetes cluster network (192.168.100.0/24)
               |                                |                                           |
               v                                v                                           v
    +------------------------ Physical host A -----------------------+   +---------- Physical host B -----------+
    | +------------------+     +----------------------------------+  |   | +----------------------------------+ |
    | | sno-master       |     | sno-compute01                    |  |   | | sno-compute02                    | |
    | | cluster VF       |     | cluster VF                       |  |   | | cluster VF                       | |
    | +------------------+     | container VF                     |  |   | | container VF                     | |
    |                          +----------------+-----------------+  |   | +----------------+-----------------+ |
    +-------------------------------------------|--------------------+   +------------------|-------------------+
                                                |                                           |
                                                +---------- Container data network ---------+
                                                              (10.56.217.0/24)
    ```

3. 了解[**表5** 测试环境示例数据](#测试环境示例数据)中的信息。表中的IP、PCI地址、MAC地址等只是本文测试环境的示例数据。实际操作中，部分信息（如虚拟机内VF的PCI与MAC地址）要等虚拟机创建、VF接入后才能查到。这里先集中列出，方便后续步骤对照替换为实际值。

    <!-- markdownlint-disable-next-line MD033 -->
    **表5** 测试环境示例数据<a id="测试环境示例数据"></a>

    | 项目 | 示例值 |
    | --- | --- |
    | master节点及其集群VF IP | sno-master / 192.168.100.5 |
    | server计算节点及其集群VF IP | sno-compute01 / 192.168.100.6 |
    | client计算节点及其集群VF IP | sno-compute02 / 192.168.100.11 |
    | 宿主机PF名称 | enp24s0f1np1 |
    | 宿主机集群VF PCI地址 | 0000:18:06.1 |
    | 虚拟机内集群VF PCI地址 | 0000:06:00.0 |
    | 虚拟机内集群VF MAC地址 | 46:77:d7:59:73:6f |
    | 宿主机容器VF PCI地址 | 0000:18:06.3 |
    | 虚拟机内容器VF PCI地址 | 0000:07:00.0 |
    | 虚拟机管理网卡PCI地址 | 0000:01:00.0 |
    | 虚拟机管理网卡MAC地址 | 52:54:00:a4:4f:97 |
    | 集群VF使用的网络 | 192.168.100.0/24 |
    | 容器VF使用的网络 | 10.56.217.0/24 |

    表中每台虚拟机各占用一张集群VF，master节点只需集群VF，两台compute节点还各需一张容器VF。表中的VF PCI地址以`sno-compute01`为例，`sno-compute02`与`sno-master`各自分配到的VF地址不同，操作时以实际查到的值为准。

    虚拟机管理网卡是基础XML中`<interface type='network'>`对应的libvirt默认网络接口，用于虚拟机的日常管理访问，在虚拟机内的PCI地址通常为`0000:01:00.0`。它和集群VF都需要留在虚拟机中，后续会一并加入黑名单，因此这里要一起记录它的PCI地址和MAC地址。

    > **说明：**
    > 本文中的PCI地址、网卡名、MAC地址、CPU编号、IP地址和密码均为测试环境示例。执行命令前必须替换为实际值。

4. 确保可以执行以下操作。

    - 修改宿主机BIOS和内核启动参数。
    - 使用`virsh`管理虚拟机。
    - 在Kubernetes master节点使用`kubectl`。
    - 在所有集群节点（即虚拟机内）使用root权限导入镜像和修改cgroup文件。

## 配置宿主机

### 配置内核启动参数

修改内核启动参数的方法可参考[《鲲鹏虚拟化损耗调优指南》中的开启内存大页](https://www.hikunpeng.com/document/detail/zh/kunpengcpfs/systuningguide/systemtg/kunpeng_shty_zn_64_005.html)。本文根据测试环境补充了SR-IOV虚拟化所需参数。

1. 备份并打开宿主机的`/etc/grub2-efi.cfg`文件。

    ```bash
    sudo cp /etc/grub2-efi.cfg /etc/grub2-efi.cfg.bak
    sudo vi /etc/grub2-efi.cfg
    ```

2. 在**对应内核**的启动参数中加入以下参数：

    ```text
    iommu.passthrough=1 pci=realloc default_hugepagesz=1024M hugepagesz=1024M hugepages=560 arm64.nopauth kvm-arm.vgic_v4_enable=1
    ```

3. 删除相关内核参数（若有）：

    ```text
    rhgb quiet
    ```

4. 修改完成后，在`vi`中执行`:wq`，保存文件并退出，配合下文的配置BIOS使参数生效。

> **说明：**
>
> 1. `hugepages=560`表示预留560个1 GiB大页，是测试服务器使用的数值。该值按**保证单个NUMA节点能提供约140 GiB大页**估算，以为128GiB虚拟机提供足够的内存大页。
> 2. 内核参数需要**重启宿主机**才能生效，此外`kvm-arm.vgic_v4_enable=1`也需要宿主机BIOS**开启GICv4.1**才能生效。

### 配置BIOS

进入BIOS，按照[**表6** BIOS配置](#BIOS配置)设置，不同服务器固件中的选项名称可能略有差异。

<!-- markdownlint-disable-next-line MD033 -->
**表6** BIOS配置<a id="BIOS配置"></a>

| 适用平台 | 选项 | 配置值 |
| --- | --- | --- |
| 鲲鹏920新型号处理器、鲲鹏950处理器 | Power Policy | Performance |
| 鲲鹏920新型号处理器、鲲鹏950处理器 | Custom Refresh Rate | 1x mode |
| 鲲鹏920新型号处理器、鲲鹏950处理器 | Support SMMU | Enabled |
| 鲲鹏920新型号处理器、鲲鹏950处理器 | SR-IOV | Enabled |
| 鲲鹏920新型号处理器、鲲鹏950处理器 | GIC Version | 4.1 |
| 鲲鹏950处理器 | HiBoost | Enabled |

内核参数和BIOS配置完成后，重启宿主机。

### 验证配置

1. 确认内核参数已经生效。

    ```bash
    cat /proc/cmdline
    ```

2. 确认GICv4.1已启用。

    ```bash
    dmesg | grep -i gicv4.1
    ```

    预期包含：

    ```text
    GICv4.1 support enabled
    ```

3. 确认大页已经创建。

    ```bash
    grep -E "HugePages_Total|HugePages_Free|Hugepagesize" /proc/meminfo
    ```

### 创建VF

在两台物理机上创建VF，并为PF赋予网络IP。操作示例如下

1. 确认所使用的CX5网卡PF后，查询该PF所在的NUMA节点。

    ```bash
    PF=enp24s0f1np1
    cat /sys/class/net/${PF}/device/numa_node
    ```

    记录输出的NUMA节点编号。后续计算节点虚拟机的CPU和内存需要使用同一个NUMA节点。

2. 从PF创建8个VF。

    ```bash
    echo 8 | sudo tee /sys/class/net/${PF}/device/sriov_numvfs
    cat /sys/class/net/${PF}/device/sriov_numvfs
    ```

    预期输出为`8`。

3. 查看每个VF的宿主机PCI地址。

    ```bash
    for vf in /sys/class/net/${PF}/device/virtfn*; do
      printf "%s -> %s\n" \
        "$(basename "${vf}")" \
        "$(basename "$(readlink -f "${vf}")")"
    done
    ```

4. 为PF配置测试网络地址。

    ```bash
    sudo ip addr add 192.168.100.105/24 dev ${PF}
    sudo ip link set ${PF} up
    ```

## 创建openEuler虚拟机并接入VF

### 准备虚拟机镜像

1. 从[openEuler 24.03 LTS SP3下载页](https://www.openeuler.org/zh/download/?version=openEuler%2024.03%20LTS%20SP3)获取AArch64虚拟机镜像（qcow2文件），并按下载页提供的校验值验证文件。

    - 在线部署时，可以直接在宿主机下载镜像。
    - 离线部署时，在联网服务器上下载并校验镜像，再将镜像文件复制到两台宿主机。

2. 将基础镜像复制为每台虚拟机各自使用的磁盘文件。三台虚拟机各用一份，例如：

    ```bash
    sudo cp openEuler-24.03-LTS-SP3-aarch64.qcow2 \
      /home/images/fsq/sno-master.img
    sudo cp openEuler-24.03-LTS-SP3-aarch64.qcow2 \
      /home/images/fsq/sno-compute01.img
    sudo cp openEuler-24.03-LTS-SP3-aarch64.qcow2 \
      /home/images/fsq/sno-compute02.img
    ```

    实际镜像文件名以下载结果为准。

### 创建虚拟机

1. 准备虚拟机安装环境。
   运行如下命令安装相关依赖,并启动libvirtd服务。

    ```bash
    sudo yum install -y qemu libvirt edk2-aarch64.noarch
    systemctl restart libvirtd
    ```

    非root用户可参考[非root用户配置](https://docs.openeuler.org/zh/docs/24.03_LTS_SP3/virtualization/virtulization_platform/virtulization/environment_preparation.html#%E9%9D%9Eroot%E7%94%A8%E6%88%B7%E9%85%8D%E7%BD%AE)进行相关相关配置

2. 复制仓库中的基础XML。仓库只提供`sno-compute01.xml`一份样例，`sno-compute02`和`sno-master`可以它为基础复制修改后使用。

    ```bash
    cp examples/cx5/sno-compute01.xml /tmp/sno-compute01.xml
    ```

    示例XML已经包含以下配置，不需要从头添加，只需按照实际情况修改。

    - 64个vCPU，并通过`vcpupin`将虚拟机vCPU `0-63`与宿主机CPU `80-143`一一绑定。
    - 虚拟机CPU和内存绑定到宿主机NUMA节点`1`。
    - `socket/die/cluster/core/thread`拓扑为`1/1/8/4/2`，总数为`1 × 1 × 8 × 4 × 2 = 64`个vCPU。
    - 两张VF直通：宿主机`0000:18:06.1`映射为虚拟机内`0000:06:00.0`，用于集群网络；宿主机`0000:18:06.3`映射为虚拟机内`0000:07:00.0`，用于容器数据网络。

    其中CPU拓扑透传、NUMA亲和与vCPU 1:1绑定是降低测试时延的关键配置，对应XML片段如下。

    CPU采用`host-passthrough`把宿主机CPU特性直接透传给虚拟机，并声明与物理拓扑一致的`topology`，同时用单个NUMA cell把64个vCPU与内存归拢到同一节点。

    ```xml
    <cpu mode='host-passthrough' check='none'>
      <topology sockets='1' dies='1' clusters='8' cores='4' threads='2'/>
      <numa>
        <cell id='0' cpus='0-63' memory='134217728' unit='KiB' memAccess='shared'/>
      </numa>
    </cpu>
    ```

    `numatune`把虚拟机的内存严格分配到宿主机NUMA节点`1`，保证内存与网卡、vCPU位于同一节点，避免跨NUMA访问。

    ```xml
    <numatune>
      <memory mode='strict' nodeset='1'/>
      <memnode cellid='0' mode='strict' nodeset='1'/>
    </numatune>
    ```

    `cputune`把64个vCPU与宿主机CPU `80-143`一一绑定（1:1 pinning），并将QEMU模拟线程（`emulatorpin`）单独固定到`80-119`，减少vCPU被调度迁移带来的抖动。

    ```xml
    <cputune>
      <vcpupin vcpu='0' cpuset='80'/>
      <vcpupin vcpu='1' cpuset='81'/>
      <!-- vcpu 2-62 依次与宿主机 CPU 82-142 一一绑定 -->
      <vcpupin vcpu='63' cpuset='143'/>
      <emulatorpin cpuset='80-119'/>
    </cputune>
    ```

    宿主机CPU `80-143`属于NUMA节点`1`，与网卡、内存节点一致。换到其他服务器时，这些CPU编号必须按网卡所在NUMA节点重新选择，`nodeset`也要相应调整。

3. 根据当前服务器修改`/tmp/sno-compute01.xml`中的示例值。

    <!-- markdownlint-disable-next-line MD033 -->
    **表7** 基础XML必查项<a id="基础XML必查项"></a>

    | 配置 | 修改要求 |
    | --- | --- |
    | 虚拟机名称 | 各虚拟机不可重名 |
    | 磁盘路径 | 各虚拟机使用单独的磁盘文件 |
    | CPU和内存 | 64 vCPU、128 GiB内存 |
    | vCPU绑定 | vcpu与宿主机CPU一一绑定，且绑定的宿主机CPU （示例中为80-143） 所在NUMA必须与当前网卡所在的NUMA节点一致 |
    | vCPU拓扑 | 尽量贴合物理机的拓扑情况，且修改时应确保乘积等于vCPU总数。示例为1个socket、1个die、8个cluster、每个cluster 4个core、每个core 2个thread，对应鲲鹏920新型号处理器开启超线程时的拓扑 |
    | NUMA绑定 | numatune中的内存节点以及虚拟机CPU和内存使用的NUMA节点，都要与PF所亲和的宿主机NUMA节点一致 |
    | QEMU和UEFI文件路径 | 与当前宿主机的安装路径一致 |
    | nvram路径 | &lt;nvram template='...vars-template-pflash.raw'&gt;...&lt;/nvram&gt;中的模板路径需要与宿主机edk2安装路径一致，nvram变量文件名需要随虚拟机名改成对应虚拟机，例如/var/lib/libvirt/qemu/nvram/sno-compute01_VARS.fd；不同虚拟机不能共用同一个nvram变量文件 |
    | VF PCI地址 | 改为分配给该虚拟机的实际的VF PCI地址 |

4. 定义并启动虚拟机。

    ```bash
    sudo virsh define /tmp/sno-compute01.xml
    sudo virsh start sno-compute01
    sudo virsh dominfo sno-compute01
    ```

5. 检查虚拟机的CPU绑定结果。

    ```bash
    sudo virsh vcpupin sno-compute01
    ```

6. 按相同方法准备`sno-compute02`，并给它分配另外两张宿主机VF。

7. 准备master虚拟机`sno-master`。master只作为Kubernetes管理节点，不运行测试容器，因此只需接入一张集群VF，不需要容器VF。

    以`sno-compute01.xml`为基础复制一份master用的XML，除按[**表7** 基础XML必查项](#基础XML必查项)修改虚拟机名称、磁盘路径、CPU/NUMA绑定外，还需删除容器VF对应的`<interface type='hostdev'>`段（即映射到虚拟机内`0000:07:00.0`的那一段），只保留集群VF一段。

    ```bash
    cp examples/cx5/sno-compute01.xml /tmp/sno-master.xml
    # 编辑 /tmp/sno-master.xml：改名为 sno-master、改磁盘路径为 /home/images/fsq/sno-master.img、
    # 按 master 所在物理机的 NUMA 节点调整 vcpupin 与 numatune、删除容器 VF 的 hostdev 段，
    # 并把集群 VF 的宿主机 PCI 地址改为分配给 master 的 VF 地址。
    sudo virsh define /tmp/sno-master.xml
    sudo virsh start sno-master
    ```

    master与其中一个compute节点部署在同一物理机时，二者的`vcpupin`与`numatune`必须使用互不重叠的CPU与内存节点，避免资源争抢影响时延测试。

### （可选）热插VF

示例XML已经接入两张VF，使用该XML创建虚拟机时无需重复执行本节操作。如果已有虚拟机缺少VF，可使用以下命令分别接入集群VF和容器VF。PCI地址必须替换为分配给该虚拟机的实际地址。

```bash
sudo virsh attach-interface sno-compute01 hostdev <cluster-vf-pci> \
  --managed --live --config
sudo virsh attach-interface sno-compute01 hostdev <container-vf-pci> \
  --managed --live --config
```

对`sno-compute02`重复上述操作，并使用另外两张VF。不同虚拟机不能使用同一个实际VF。

### 在虚拟机内确认网卡

进入每台虚拟机（`sno-master`和两台compute节点），执行：

```bash
lspci -nnD | grep -i 15b3

for dev in /sys/class/net/*; do
  netdev=$(basename "${dev}")
  pci=$(basename "$(readlink -f "${dev}/device")" 2>/dev/null)
  mac=$(cat "${dev}/address" 2>/dev/null)
  printf "%s  pci=%s  mac=%s\n" "${netdev}" "${pci}" "${mac}"
done
```

记录以下信息：

- 集群VF的PCI地址和MAC地址。该网卡留在虚拟机中用于集群网络，后续需要加入黑名单。
- 要提供给容器的VF PCI地址和MAC地址（master节点没有容器VF，可跳过此项）。
- 虚拟机管理网卡的PCI地址和MAC地址。后续也需要把管理网卡加入黑名单。
- `lspci`显示的厂商编号和设备编号。ConnectX-5 VF通常为`15b3:1018`，实际值以命令输出为准。

### 为集群VF配置IP

每台虚拟机的集群VF需要配置[**表5** 测试环境示例数据](#测试环境示例数据)规划的集群网络IP，作为Kubernetes节点之间通信的地址。下面以`sno-compute01`为例，其他虚拟机替换为各自的网卡名和IP。

1. 在虚拟机内找到集群VF对应的网卡名。上一步`for`循环的输出中，PCI地址等于虚拟机内集群VF地址（示例`0000:06:00.0`）的那一行就是集群VF的网卡名。

2. 为该网卡配置IP并拉起。

    ```bash
    CLUSTER_NIC=<集群 VF 网卡名>
    sudo ip addr add 192.168.100.6/24 dev ${CLUSTER_NIC}
    sudo ip link set ${CLUSTER_NIC} up
    ```

    以下为示例IP规划：
    - `sno-master`使用`192.168.100.5/24`。
    - `sno-compute01`使用`192.168.100.6/24`。
    - `sno-compute02`使用`192.168.100.11/24`。

3. 三台虚拟机都配置完成后，互相`ping`验证集群网络连通。集群网络是后续部署Kubernetes的前提，必须先连通再继续。

    ```bash
    ping -c 3 192.168.100.5
    ping -c 3 192.168.100.6
    ping -c 3 192.168.100.11
    ```

> **说明：**
> 使用`ip addr add`配置的地址在虚拟机重启后会丢失。如需持久化，可参考[openEuler 24.03 LTS SP3：配置网络](https://docs.openeuler.org/zh/docs/24.03_LTS_SP3/server/network/network_config/network_configuration.html#user-content-配置静态ip连接)写入NetworkManager或`ifcfg`配置文件。

## 部署Kubernetes集群

在鲲鹏服务器上部署Kubernetes集群，请参见《[鲲鹏Kubernetes部署指南（CentOS和openEuler）](https://www.hikunpeng.com/document/detail/zh/kunpengcpfs/ecosystemEnable/Kubernetes/kunpengk8s_04_0001.html)》。也可使用sealos快速部署集群，下面以sealos为例分别说明在线和离线环境中的集群搭建操作。

### 使用sealos快速部署Kubernetes集群

以下命令基于实操验证使用的sealos 5.1.1编写，不强制要求使用该版本。使用其他版本时，请根据对应版本的sealos文档确认命令参数。在线部署时，所有sealos命令都在master节点执行；离线部署时，镜像拉取和导出命令在联网服务器执行，其余命令在master节点执行。

#### 安装sealos

1. 请参见《[sealos CLI安装指南](https://sealos.run/docs/advanced/k8s/getting-started/install-cli)》安装sealos。

    - 在线部署时，直接在master节点下载安装。
    - 离线部署时，在联网服务器下载安装包，再将安装包复制到master节点安装。

2. 检查版本。

    ```bash
    sealos version
    ```

    后续命令参考使用的版本是`5.1.1`。

#### （仅离线部署）准备集群镜像

本节仅用于master节点或compute节点无法访问sealos镜像仓库的场景。在线部署时，sealos会直接从镜像仓库获取集群镜像，可以跳过本节，直接执行“准备CNI plugins”。

<!-- markdownlint-disable-next-line MD033 -->
<a id="拉取sealos集群镜像"></a>

1. 在可访问镜像仓库的服务器上拉取并导出Kubernetes、Flannel和Helm镜像。

    ```bash
    sealos pull registry.cn-shanghai.aliyuncs.com/labring/kubernetes:v1.28.14
    sealos pull registry.cn-shanghai.aliyuncs.com/labring/flannel:v0.25.6
    sealos pull registry.cn-shanghai.aliyuncs.com/labring/helm:v3.9.4

    sealos save -o kubernetes-v1.28.14.tar \
      registry.cn-shanghai.aliyuncs.com/labring/kubernetes:v1.28.14
    sealos save -o flannel-v0.25.6.tar \
      registry.cn-shanghai.aliyuncs.com/labring/flannel:v0.25.6
    sealos save -o helm-v3.9.4.tar \
      registry.cn-shanghai.aliyuncs.com/labring/helm:v3.9.4
    ```

2. 将三个镜像包复制到master节点，并在master节点导入。

    ```bash
    sealos load -i flannel-v0.25.6.tar
    sealos load -i helm-v3.9.4.tar
    sealos load -i kubernetes-v1.28.14.tar
    sealos images
    ```

#### 准备CNI plugins

在线和离线部署均需要手动准备CNI plugins。本节使用CNI plugins 1.5.1，其ARM64软件包从[containernetworking/plugins](https://github.com/containernetworking/plugins/releases/download/v1.5.1/cni-plugins-linux-arm64-v1.5.1.tgz)下载。

1. 获取CNI plugins软件包。

    - 在线部署时，在master节点下载软件包。

        ```bash
        curl -fLo /tmp/cni-plugins-linux-arm64-v1.5.1.tgz \
          https://github.com/containernetworking/plugins/releases/download/v1.5.1/cni-plugins-linux-arm64-v1.5.1.tgz
        ```

    - 离线部署时，在联网服务器下载软件包，再将其复制到master节点的`/tmp`目录。

2. 在master节点解压CNI plugins软件包。

    ```bash
    CNI_ARCHIVE=/tmp/cni-plugins-linux-arm64-v1.5.1.tgz
    sudo test -f "${CNI_ARCHIVE}"
    sudo mkdir -p /opt/cni/bin
    sudo tar -xzvf "${CNI_ARCHIVE}" -C /opt/cni/bin
    ```

3. 仍在master节点，将同一个软件包复制到两个compute节点，并在各节点解压。

    ```bash
    sudo chmod 0644 "${CNI_ARCHIVE}"
    scp "${CNI_ARCHIVE}" root@<sno-compute01-ip>:/tmp/
    scp "${CNI_ARCHIVE}" root@<sno-compute02-ip>:/tmp/

    ssh root@<sno-compute01-ip> \
      'mkdir -p /opt/cni/bin && tar -xzvf /tmp/cni-plugins-linux-arm64-v1.5.1.tgz -C /opt/cni/bin'
    ssh root@<sno-compute02-ip> \
      'mkdir -p /opt/cni/bin && tar -xzvf /tmp/cni-plugins-linux-arm64-v1.5.1.tgz -C /opt/cni/bin'
    ```

#### 节点前置准备

1. 在所有虚拟机的`/etc/hosts`中加入：

    ```text
    192.168.100.5  sno-master
    192.168.100.6  sno-compute01
    192.168.100.11 sno-compute02
    ```

2. 在三个节点上执行`date`，确认时间一致。节点时间差异较大时，先校准时间，具体操作可参考鲲鹏Kubernetes部署指南中的[配置NTP](https://www.hikunpeng.com/document/detail/zh/kunpengcpfs/ecosystemEnable/Kubernetes/kunpengk8s_04_0006.html)。

#### 创建集群

1. 先在master节点创建单节点Kubernetes集群。此时先不加入compute节点，也不安装Flannel。

    ```bash
    sealos run registry.cn-shanghai.aliyuncs.com/labring/kubernetes:v1.28.14 --masters 192.168.100.5
    ```

2. 加入两个compute节点。将密码替换为实际root密码。

    ```bash
    sealos add --nodes 192.168.100.6 -p '<root-password>'
    sealos add --nodes 192.168.100.11 -p '<root-password>'
    ```

3. 安装Flannel和Helm。

    ```bash
    sealos run registry.cn-shanghai.aliyuncs.com/labring/helm:v3.9.4
    sealos run registry.cn-shanghai.aliyuncs.com/labring/flannel:v0.25.6
    ```

4. 检查集群。

    ```bash
    kubectl get nodes -o wide
    kubectl get pods -A
    ```

三个节点都应显示为`Ready`。

#### （可选）重置集群

集群创建失败或需要重新部署时，可先用sealos重置，再清理各节点上的残留配置。以下命令具有破坏性，会删除集群状态和网络配置，仅在确认要重建集群时执行。

1. 在master节点用sealos重置集群。

    ```bash
    sealos reset
    ```

2. 如果sealos重置后仍有残留，在所有节点执行彻底清理，然后重启虚拟机避免残留进程占用端口。

    ```bash
    kubeadm reset -f
    rm -rf /etc/cni/net.d /etc/kubernetes /var/lib/etcd "${HOME}/.kube/config"

    systemctl stop kubelet containerd
    systemctl disable kubelet containerd

    ip link delete cni0 2>/dev/null
    ip link delete flannel.1 2>/dev/null
    iptables -F
    iptables -t nat -F
    iptables -t mangle -F
    iptables -X
    ```

3. 清理完成后重启虚拟机，再从“创建集群”重新开始。

## 安装容器附加网络

Kubernetes节点通过集群VF通信，Flannel为普通容器提供默认网络。本方案还需要把容器VF作为第二张网卡加入qperf容器，因此需要安装：

- Multus：给一个容器添加第二张网卡。
- Whereabouts：给不同compute节点上的容器分配不重复的IP地址。

### （仅离线部署）准备Multus镜像

按照[Multus部署文件multi-daemonset.yaml](https://github.com/k8snetworkplumbingwg/multus-cni/tree/master/deployments/multus-daemonset.yml)准备部署镜像。本节仅用于Kubernetes节点无法访问`multi-daemonset.yaml`中Multus镜像地址的场景。在线部署时，节点可以在部署Multus时直接拉取镜像，可以跳过本节，直接执行“部署Multus”。

1. 在AArch64联网服务器上，根据`multi-daemonset.yaml`中的实际镜像地址拉取并导出Multus镜像。

    ```bash
    MULTUS_IMAGE=<multi-daemonset.yaml中的Multus镜像地址>
    docker pull "${MULTUS_IMAGE}"
    docker save "${MULTUS_IMAGE}" -o multus.tar
    ```

2. 将`multus.tar`复制到master节点和两个compute节点，并在每个节点导入。

    ```bash
    sudo ctr -n k8s.io images import multus.tar
    ```

3. 在`multi-daemonset.yaml`中将Multus容器的`imagePullPolicy`设置为`IfNotPresent`，避免Kubernetes在镜像已经导入后仍尝试访问公网。在线部署可以忽略此步骤并保留原有镜像拉取策略。

### 部署Multus

获取[Multus部署文件multi-daemonset.yaml](https://github.com/k8snetworkplumbingwg/multus-cni/tree/master/deployments/multus-daemonset.yml)，进行相关修改后部署。
在线部署时，在master节点直接下载`multi-daemonset.yaml`；离线部署时，在联网服务器下载该文件后复制到master节点。
在`multi-daemonset.yaml`中完成以下修改：

1. 将自动生成配置的参数改为：

    ```yaml
    - "--multus-conf-file=/tmp/multus-conf/00-multus.conf"
    ```

2. 文件末尾引用的配置文件名也改为`00-multus.conf`。

3. 部署并检查Multus。

    ```bash
    kubectl apply -f multi-daemonset.yaml
    kubectl get pods -A | grep -i multus
    ```

### 部署Whereabouts

在线部署执行步骤1、5和6，步骤2～4仅用于离线部署，在线部署可以忽略。离线部署需要执行全部步骤。

1. 获取Whereabouts源码。

    - 在线部署时，在master节点执行`git clone`。
    - 离线部署时，在联网服务器执行`git clone`，再将整个`whereabouts`目录复制到master节点。

    ```bash
    git clone https://github.com/k8snetworkplumbingwg/whereabouts
    cd whereabouts
    ```

2. **（仅离线部署）** 在AArch64联网服务器上拉取并导出部署文件引用的镜像。在线部署可以忽略此步骤。

    ```bash
    docker pull ghcr.io/k8snetworkplumbingwg/whereabouts:latest
    docker save ghcr.io/k8snetworkplumbingwg/whereabouts:latest \
      -o whereabouts.tar
    ```

3. **（仅离线部署）** 将`whereabouts.tar`复制到master节点和两个compute节点，并在每个节点导入。在线部署可以忽略此步骤。

    ```bash
    sudo ctr -n k8s.io images import whereabouts.tar
    ```

4. **（仅离线部署）** 在`doc/crds/daemonset-install.yaml`的Whereabouts容器配置中增加`imagePullPolicy: IfNotPresent`，使Kubernetes优先使用节点中已经导入的镜像。在线部署可以忽略此步骤并保留原有拉取策略。

    ```yaml
    containers:
      - name: whereabouts
        image: ghcr.io/k8snetworkplumbingwg/whereabouts:latest
        imagePullPolicy: IfNotPresent
    ```

5. 部署`whereabouts`。

    ```bash
    kubectl apply \
      -f doc/crds/daemonset-install.yaml \
      -f doc/crds/whereabouts.cni.cncf.io_ippools.yaml \
      -f doc/crds/whereabouts.cni.cncf.io_overlappingrangeipreservations.yaml
    ```

6. 检查Whereabouts是否运行。

    ```bash
    kubectl get pods -A | grep -i whereabouts
    ```

## 编译并部署SR-IOV Network Operator

### 编译镜像

编译过程需要访问Go模块、容器基础镜像和其他构建依赖。在线部署可以在能够访问这些依赖的master节点或AArch64构建服务器上编译；离线部署应在AArch64联网服务器上完成编译和镜像导出，再将镜像包传入目标环境。

1. 进入项目目录。

    ```bash
    cd /path/to/cloud-native/sriov-network-operator
    ```

    **可选操作：Dockerfile中配置网络代理与GO代理**：此操作仅适用于需要通过代理访问外部依赖的受限联网环境。编译环境无法直接拉取依赖但可以使用代理时，需要在3个Dockerfile中临时配置网络代理与GO代理。相关代理配置位置已在Dockerfile中注释，根据实际情况取消注释并增加Go代理，参考配置：

    ```Dockerfile
    # 按需取消如下注释以保证容器内编译可正常拉取依赖
    ENV https_proxy=http://ip:port
    ENV http_proxy=http://ip:port

    # go代理配置需要手动补充
    RUN go env -w GO111MODULE=on
    RUN go env -w GOPROXY=https://mirrors.huaweicloud.com/repository/goproxy,direct
    RUN go env -w GONOSUMDB=*
    ```

2. 一次性编译三个镜像。

    ```bash
    make image
    ```

    如果需要分别编译三个镜像，可依次执行：

    ```bash
    docker build -f Dockerfile \
      -t ghcr.io/k8snetworkplumbingwg/sriov-network-operator:latest .
    docker build -f Dockerfile.sriov-network-config-daemon \
      -t ghcr.io/k8snetworkplumbingwg/sriov-network-operator-config-daemon:latest .
    docker build -f Dockerfile.webhook \
      -t ghcr.io/k8snetworkplumbingwg/sriov-network-operator-webhook:latest .
    ```

3. 检查镜像。

    ```bash
    docker images | grep sriov-network-operator
    ```

    **在线部署时**，将编译出的三个镜像推送到Kubernetes节点可以访问的镜像仓库，并将`hack/env.sh`中的`SRIOV_NETWORK_OPERATOR_IMAGE`、`SRIOV_NETWORK_CONFIG_DAEMON_IMAGE`和`SRIOV_NETWORK_WEBHOOK_IMAGE`设置为实际镜像地址。节点能够从该仓库拉取镜像后，可以忽略步骤4、步骤5和后续“导入镜像”章节。

4. **（仅离线部署）** 保存编译出的三个镜像。在线部署可以忽略此步骤。

    ```bash
    docker save ghcr.io/k8snetworkplumbingwg/sriov-network-operator:latest \
      -o sriov-network-operator.tar
    docker save ghcr.io/k8snetworkplumbingwg/sriov-network-operator-config-daemon:latest \
      -o sriov-network-operator-config-daemon.tar
    docker save ghcr.io/k8snetworkplumbingwg/sriov-network-operator-webhook:latest \
      -o sriov-network-operator-webhook.tar
    ```

5. **（仅离线部署）** 准备另外五个CNI镜像。在线部署时，Kubernetes会从`hack/env.sh`配置的镜像地址拉取这些镜像，可以忽略此步骤。

    `make image`只会构建operator、config-daemon、webhook三个镜像；Operator运行时还依赖sriov-cni、ib-sriov-cni、sriov-network-device-plugin、ovs-cni-plugin、rdma-cni五个镜像。这五个镜像不由本仓库构建，而是从`ghcr.io/k8snetworkplumbingwg`拉取（默认地址定义在`hack/env.sh`中）。离线部署时，在AArch64联网服务器上拉取并导出这五个镜像。

    ```bash
    docker pull ghcr.io/k8snetworkplumbingwg/sriov-cni:latest
    docker pull ghcr.io/k8snetworkplumbingwg/ib-sriov-cni:latest
    docker pull ghcr.io/k8snetworkplumbingwg/sriov-network-device-plugin:latest
    docker pull ghcr.io/k8snetworkplumbingwg/ovs-cni-plugin:latest
    docker pull ghcr.io/k8snetworkplumbingwg/rdma-cni:latest

    docker save ghcr.io/k8snetworkplumbingwg/sriov-cni:latest -o sriov-cni.tar
    docker save ghcr.io/k8snetworkplumbingwg/ib-sriov-cni:latest -o ib-sriov-cni.tar
    docker save ghcr.io/k8snetworkplumbingwg/sriov-network-device-plugin:latest \
      -o sriov-network-device-plugin.tar
    docker save ghcr.io/k8snetworkplumbingwg/ovs-cni-plugin:latest -o ovs-cni-plugin.tar
    docker save ghcr.io/k8snetworkplumbingwg/rdma-cni:latest -o rdma-cni.tar
    ```

    实际使用的镜像标签应与`hack/env.sh`中的默认值或部署文件保持一致。

### （仅离线部署）导入镜像

本节仅用于Kubernetes节点无法访问Operator及其依赖镜像仓库的场景。
将下面八个镜像包复制到三个节点并全部导入。

```bash
sudo ctr -n k8s.io images import sriov-network-operator.tar
sudo ctr -n k8s.io images import sriov-network-operator-config-daemon.tar
sudo ctr -n k8s.io images import sriov-network-operator-webhook.tar
sudo ctr -n k8s.io images import sriov-cni.tar
sudo ctr -n k8s.io images import ib-sriov-cni.tar
sudo ctr -n k8s.io images import sriov-network-device-plugin.tar
sudo ctr -n k8s.io images import ovs-cni-plugin.tar
sudo ctr -n k8s.io images import rdma-cni.tar
```

导入后执行：

```bash
sudo crictl images
```

显示相关镜像信息即说明上述镜像导入成功。

### 为compute节点添加标签

在master节点执行：

```bash
kubectl label node sno-compute01 node-role.kubernetes.io/worker=""
kubectl label node sno-compute01 feature.node.kubernetes.io/network-sriov.capable=true
kubectl label node sno-compute01 node-virtualization.kubernetes.io/type=qemu-kvm

kubectl label node sno-compute02 node-role.kubernetes.io/worker=""
kubectl label node sno-compute02 feature.node.kubernetes.io/network-sriov.capable=true
kubectl label node sno-compute02 node-virtualization.kubernetes.io/type=qemu-kvm
```

最后一个标签告诉本仓库的SR-IOV Network Operator：当前节点是QEMU/KVM虚拟机，需要发现已经接入虚拟机的VF，而不是在虚拟机内创建新的VF。

### 部署SR-IOV Network Operator

部署操作在master节点执行。在线部署可以直接从软件源和代码仓库安装依赖；离线部署需要提前在联网服务器下载Go安装包、operator-sdk以及操作系统依赖包，再复制到master节点安装。执行部署命令前，离线环境应确保`go`、`operator-sdk`、`skopeo`、`envsubst`和`kubectl`均已可用。

1. 根据网络环境准备依赖访问方式。

    受限联网环境无法直接访问外部依赖但允许使用代理时，可以临时配置网络代理。完全在线环境不需要配置代理；完全离线环境应使用提前准备的软件包，也不执行代理配置。网络代理配置示例如下，请按需修改。

    ```bash
    export http_proxy=http://10.10.10.10:8080
    export https_proxy=http://10.10.10.10:8080
    ```

2. 安装go1.23.4。

   部署SR-IOV Network Operator需要Go 1.23.4。在线部署可以直接下载；离线部署使用联网服务器下载并传入master节点的安装包。参考链接：

   - [go 1.23.4 linux-arm64官方下载链接](https://golang.google.cn/dl/go1.23.4.linux-arm64.tar.gz)
   - [Go官方安装指南](https://golang.google.cn/doc/install)

3. 安装operator-sdk。在线部署可以按照[operator-sdk官方安装文档](https://sdk.operatorframework.io/docs/installation/)直接下载；离线部署使用提前传入master节点的安装文件。核心操作：

    ```bash
    chmod +x operator-sdk_linux_arm64
    sudo cp operator-sdk_linux_arm64 /usr/local/bin/operator-sdk
    operator-sdk version
    ```

4. （可选）受限联网环境配置Go代理。完全在线环境通常不需要修改，离线部署可参考如下配置。

    ```bash
    go env -w GO111MODULE=on
    go env -w GOPROXY=http://mirrors.huaweicloud.com/repository/goproxy/,direct
    go env -w GONOSUMDB='*'
    ```

5. 部署。

    参考如下命令拉取相关Go依赖并部署Operator，若master节点可以直接拉取官方Go依赖则不需要配置GOPROXY：

    ```bash
    cd /path/to/cloud-native/sriov-network-operator
    make deploy-setup-k8s \
      GOPROXY=http://mirrors.huaweicloud.com/repository/goproxy/,direct
    ```

    > **说明：**
    > 网络代理（非Go代理）可能影响对Kubernetes API的访问。受限联网环境如果在准备依赖时配置了网络代理，大概率只能完成依赖拉取，但无法成功部署Operator。此时需要在**相关依赖拉取完成后取消代理并重新运行部署命令**：
    unset http_proxy https_proxy
    make deploy-setup-k8s

6. 检查部署结果。

    ```bash
    kubectl get pods -n sriov-network-operator -o wide
    ```

    预期能看到sriov-network-operator和sriov-network-config-daemon相关Pod正常运行。

## 配置容器使用VF

### 排除不交给容器的网卡

SR-IOV Network Operator会扫描虚拟机中的PCI网卡。虚拟机管理网卡和集群VF都必须留在虚拟机中，不能交给测试容器，因此需要把这两张网卡加入黑名单。

1. 编辑`examples/cx5/blacklist.yaml`。填写虚拟机管理网卡的PCI地址和MAC地址，集群VF的PCI地址和MAC地址。示例PCI地址分别使用`0000:01:00.0`和`0000:06:00.0`。

    > **说明：**
    >
    > - 黑名单中的PCI地址**对所有节点生效**，部署黑名单后所有节点对应的pci设备 都不会被Operator接管。
    > - 若多个节点的网卡设备PCI地址相同，MAC地址**只需填其中一个**节点对应的MAC地址即可。

2. 部署黑名单。

    ```bash
    kubectl apply -f examples/cx5/blacklist.yaml
    ```

3. 查看Operator发现的网卡。

    ```bash
    kubectl get sriovnetworknodestate -n sriov-network-operator
    kubectl get sriovnetworknodestate -n sriov-network-operator \
      sno-compute01 -o yaml
    kubectl get sriovnetworknodestate -n sriov-network-operator \
      sno-compute02 -o yaml
    ```

结果中应包含要提供给容器的VF，不应包含黑名单中的管理网卡和集群VF。

### 选择要提供给容器的VF

`examples/cx5/node-policy-cx5.yaml`用于选择VF，并把这类网卡命名为`mlxcx5`。示例文件的主要字段含义如[**表8** node-policy-cx5.yaml关键字段](#node-policy-cx5-yaml关键字段)所示。

<!-- markdownlint-disable-next-line MD033 -->
**表8** node-policy-cx5.yaml关键字段<a id="node-policy-cx5-yaml关键字段"></a>

| 字段 | 示例值 | 说明 |
| --- | --- | --- |
| resourceName | mlxcx5 | Operator把命中的VF汇总成的资源名。容器通过openshift.io/mlxcx5申请这类VF |
| deviceType | netdevice | 以普通内核网络设备方式提供VF，容器内可直接看到网卡；不使用vfio-pci用户态直通 |
| externallyManaged | true | 关键开关。声明VF由外部（宿主机 + libvirt直通）管理，Operator只纳管已经存在的VF，不在虚拟机内再创建或删除VF |
| numVfs | 0 | 与externallyManaged: true配合。因为不由Operator创建VF，所以设为0 |
| mtu | 1500 | VF的MTU，按实际网络调整 |
| nicSelector.deviceID | 1018 | ConnectX-5 VF的设备编号，以虚拟机内lspci -nnD为准 |
| nicSelector.vendor | 15b3 | Mellanox/NVIDIA厂商编号 |
| nicSelector.pciAddresses | ["0000:07:00.0"] | 命中的VF在虚拟机内的PCI地址，详见下方说明 |
| nodeSelector | feature.node.kubernetes.io/network-sriov.capable: "true" | 策略生效的节点范围，与前面为compute节点添加的标签对应 |

1. 编辑该文件中的`deviceID`、`vendor`、`pciAddresses`，其余字段一般保持示例值即可。

    > **说明：**
    > `pciAddresses`的意义是：可被分配给容器的VF的PCI地址，只有在此范围内的VF才会被Operator管理并分配给容器。两台compute虚拟机各有独立的PCI空间，但按照示例xml部署的虚拟机，所使用的容器VF在虚拟机内的地址都是`0000:07:00.0`，单个地址即可同时命中两台节点，无需为`sno-compute02`再追加地址。若某台虚拟机内的容器VF地址不同，需要在数组中补充对应地址。

2. 部署文件。

    ```bash
    kubectl apply -f examples/cx5/node-policy-cx5.yaml
    ```

3. 检查每个节点可用的VF数量。

    ```bash
    kubectl get node sno-compute01 \
      -o jsonpath='{.status.allocatable.openshift\.io/mlxcx5}{"\n"}'
    kubectl get node sno-compute02 \
      -o jsonpath='{.status.allocatable.openshift\.io/mlxcx5}{"\n"}'
    ```

虽然每台compute虚拟机接入了两张VF，但只有容器VF被选为目标设备，因此预期输出为`1`。

### 创建VF网络

`examples/cx5/host-device.yaml`完成两件事：

- 创建基于`mlxcx5`网卡资源的网络。
- 使用Whereabouts从`10.56.217.0/24`中分配不重复的IP地址。

示例的`ipam`段使用`"type": "whereabouts"`，并通过`"exclude": ["10.56.217.1"]`把网段首地址排除在自动分配之外，避免与网关或其他固定地址冲突。

部署并检查：

```bash
kubectl apply -f examples/cx5/host-device.yaml
kubectl get net-attach-def -n sriov-network-operator
```

预期可以看到`host-network-cx5`。

## 部署qperf测试容器

### 部署Pod

1. 检查两个样例中的节点名和镜像：

    - `examples/cx5/qperf-server.yaml`运行在`sno-compute01`。
    - `examples/cx5/qperf-client.yaml`运行在`sno-compute02`。
    - 两个容器都申请8 CPU、16 GiB内存和一个`mlxcx5` VF。

2. 根据部署方式准备`iecedge/qperf:latest`镜像。

    - 在线部署时，确认两个compute节点能够访问该镜像地址即可，部署Pod时由containerd自动拉取镜像，不需要手动导出和导入。
    - 离线部署时，在AArch64联网服务器上拉取并导出镜像：

        ```bash
        docker pull iecedge/qperf:latest
        docker save iecedge/qperf:latest -o qperf.tar
        ```

      将`qperf.tar`复制到两个compute节点，然后分别在两个节点导入。在线部署可以忽略镜像导出、复制和导入操作。

        ```bash
        sudo ctr -n k8s.io images import qperf.tar
        ```

      离线部署还需要确认两个Pod部署文件使用`imagePullPolicy: IfNotPresent`，避免Kubernetes尝试从公网重新拉取镜像。在线部署可以保留原有镜像拉取策略。

3. 部署两个Pod。

    ```bash
    kubectl apply -f examples/cx5/qperf-server.yaml
    kubectl apply -f examples/cx5/qperf-client.yaml
    kubectl get pods -n sriov-network-operator -o wide
    ```

    两个Pod都应显示为`Running`。
4. 检查网络配置。使用kubectl describe pod -n sriov-network-operator qperf-server可查看qperf server的网络配置，预期会有两个IP，其中一个为10.56.217.0/24子网（即host-network-cx5配置的子网）IP。qperf-client类似。

### 将容器绑定到CPU 0-7

Linux使用cgroup文件控制容器可以使用哪些CPU。测试环境将qperf容器对应的`cpuset.cpus`写成`0-7`，因此之后通过`kubectl exec`启动的qperf进程也只能在CPU `0-7`上运行。

容器重建后，容器ID和cgroup路径会改变，需要重新执行本节操作。

1. 登录`sno-compute01`，设置qperf server容器名称。

    ```bash
    CONTAINER_NAME=qperf-server
    ```

2. 获取容器ID和容器主进程号。

    ```bash
    CONTAINER_ID=$(sudo crictl ps -q --name "${CONTAINER_NAME}" | head -n 1)
    CONTAINER_PID=$(sudo crictl inspect "${CONTAINER_ID}" |
      sed -n 's/.*"pid": \([0-9][0-9]*\).*/\1/p' | head -n 1)

    test -n "${CONTAINER_ID}"
    test -n "${CONTAINER_PID}"
    printf "container=%s pid=%s\n" "${CONTAINER_ID}" "${CONTAINER_PID}"
    ```

    两个值都不能为空。

3. 根据当前系统使用的cgroup版本找到`cpuset.cpus`。该命令会自动处理cgroup v1和v2。

    ```bash
    if grep -q '^0::' "/proc/${CONTAINER_PID}/cgroup"; then
      CGROUP_PATH=$(awk -F: '$1 == "0" {print $3}' "/proc/${CONTAINER_PID}/cgroup")
      CPUSET_FILE="/sys/fs/cgroup${CGROUP_PATH}/cpuset.cpus"
    else
      CGROUP_PATH=$(awk -F: '$2 ~ /(^|,)cpuset(,|$)/ {print $3}' \
        "/proc/${CONTAINER_PID}/cgroup")
      CPUSET_FILE="/sys/fs/cgroup/cpuset${CGROUP_PATH}/cpuset.cpus"
    fi

    printf "cpuset file: %s\n" "${CPUSET_FILE}"
    sudo test -f "${CPUSET_FILE}"
    ```

4. 写入CPU `0-7`并读取确认。

    ```bash
    echo 0-7 | sudo tee "${CPUSET_FILE}"
    sudo cat "${CPUSET_FILE}"
    ```

    预期输出为：

    ```text
    0-7
    ```

5. 从容器内再次确认。

    ```bash
    kubectl exec qperf-server -n sriov-network-operator -- cat /sys/fs/cgroup/cpuset/cpuset.cpus
    ```

    预期包含：

    ```text
    0-7
    ```

6. 登录`sno-compute02`，将`CONTAINER_NAME`改为`qperf-client`，重复上述操作。

### 绑定VF中断

容器绑核控制qperf进程使用哪些CPU；本节的中断绑核控制VF收到数据后由哪些CPU处理。这是两个不同操作。

1. 将脚本复制到两个compute节点。

    ```bash
    scp examples/cx5/irq.sh root@sno-compute01:/root/irq.sh
    scp examples/cx5/irq.sh root@sno-compute02:/root/irq.sh
    ```

2. 在两个compute节点停止防火墙和自动中断调度服务。该操作仅用于隔离的性能测试环境。

    ```bash
    sudo systemctl stop firewalld
    sudo systemctl stop irqbalance
    ```

3. 将VF中断绑定到CPU `4-7`。PCI地址以虚拟机内实际VF地址为准。

    ```bash
    sudo bash /root/irq.sh bind 0000:07:00.0 4-7
    sudo bash /root/irq.sh check 0000:07:00.0
    ```

检查命令的输出应只包含`4`、`5`、`6`、`7`。

## 执行qperf测试

### 准备测试脚本

1. 在`sno-compute02`创建测试结果目录，并复制脚本。

    ```bash
    sudo mkdir -p /root/qperf-result
    sudo cp /path/to/cloud-native/sriov-network-operator/examples/cx5/qperf.sh \
      /root/qperf-result/qperf.sh
    sudo chmod +x /root/qperf-result/qperf.sh
    ```

2. 编辑`/root/qperf-result/qperf.sh`，将`SERVER_IP`改为qperf server容器的实际host-network-cx5网络IP。除该地址外，脚本其余内容通常无需改动。

    ```bash
    SERVER_IP="10.56.217.171"
    ```

    脚本内部行为如下，便于理解运行结果：

    - 以`qperf`直接发起测试，不对qperf进程额外绑核。client侧的CPU约束由前面写入的容器`cpuset.cpus`（`0-7`）统一控制，脚本变量`CORE_BIND_CMD`保持为`qperf`即可。
    - 每类测试重复多次，结果按`测试模式-类型-消息长度`命名，保存在`/root/qperf-result/`下的子目录中，方便多轮取平均。
    - 运行前会清理残留的qperf进程，避免上一轮测试干扰。

    `irq.sh`的作用是把指定PCI设备的所有中断轮流绑定到给定的CPU列表：`bind`子命令执行绑定，`check`子命令读取当前绑定结果，脚本本身不需要修改，只需传入正确的VF PCI地址和CPU范围。

### 启动测试

1. 在终端1启动qperf server。该命令会持续运行，测试完成前不要关闭终端。

    ```bash
    kubectl exec -it qperf-server -n sriov-network-operator -- qperf
    ```

2. 在终端2进入qperf client并运行测试脚本。

    ```bash
    kubectl exec -it qperf-client -n sriov-network-operator -- bash
    cd /root/qperf-result
    bash qperf.sh client
    ```

脚本执行以下测试，并把每次结果保存到`/root/qperf-result/`：

- TCP 1448字节消息时延。
- TCP 1字节到1 KiB消息时延。
- UDP 18字节消息时延。
- UDP 1字节到1 KiB消息时延。

### （可选）物理机基线测试

虚拟机内容器的时延应与同一张网卡在物理机上直接测得的时延对比，才能判断虚拟化带来的额外开销是否在可接受范围内。可在两台宿主机上分别用同一张PF运行一组qperf，作为基线参照。物理机测试不经过虚拟机和容器，绑核范围也直接使用宿主机CPU编号。

1. 在两台宿主机停止防火墙和自动中断调度服务，并将PF中断绑定到与虚拟机内容器测试时网卡中断所绑定的相同的物理CPU（如84-87）。以下PCI地址和CPU编号为测试环境示例，需按当前网卡所在NUMA节点替换。

    ```bash
    sudo systemctl stop firewalld
    sudo systemctl stop irqbalance
    sudo bash /root/irq.sh bind 0000:18:00.1 84-87
    sudo bash /root/irq.sh check 0000:18:00.1
    ```

2. 在作为server的宿主机启动qperf，并将进程绑定到与虚拟机内容器测试时`qperf-server`所绑定的相同的物理CPU（如80-87）和NUMA内存结点。

    ```bash
    numactl -C 80-87 -m 1 qperf
    ```

3. 在作为client的宿主机创建并修改测试脚本。
    
    创建`qperf.sh`并复制示例脚本中的内容，将其中的`SERVER_IP`改为server宿主机PF的实际IP（如192.168.100.105），将`TEST_MODE`修改为其他值（如host-cx5-pf），设定qperf进程绑定（与虚拟机内容器测试时qperf-client相同）的cpu与内存，如将`CORE_BIND_CMD="qperf"`修改为`CORE_BIND_CMD="numactl -C 80-87 -m 1 qperf"`。脚本修改后相关内容示例如下。
    
    ```bash
    #!/bin/bash
    # usege: ./qperf.sh client
    # set trace level
    set -ex

    # set variables for test
    SERVER_IP="192.168.100.105"
    # Client_IP="192.168.100.111"  # 此部分可不修改

    CORE_BIND_CMD="numactl -C 80-87 -m 1 qperf"
    TEST_MODE="host-cx5-pf"
    ```

4. 在作为client的宿主机运行测试脚本。

    ```bash
    bash qperf.sh client
    ```

将物理机基线结果与虚拟机内容器结果放在一起对比，即可评估虚拟化和容器化引入的时延增量。

## 验收检查

按[**表9** 验收检查](#验收检查)逐项检查。

<!-- markdownlint-disable-next-line MD033 -->
**表9** 验收检查<a id="验收检查"></a>

| 检查项 | 通过条件 |
| --- | --- |
| 宿主机配置 | 内核参数、大页、SR-IOV、SMMU和GICv4.1已生效 |
| 虚拟机配置 | 虚拟机CPU、内存和两张VF使用同一个宿主机NUMA节点，vCPU已按示例要求绑定 |
| VF接入 | 两台compute虚拟机内都能看到各自的集群VF和容器VF |
| Operator | 网卡列表包含容器VF，不包含黑名单中的管理网卡和集群VF |
| 容器网络 | 两个qperf容器都有net1，IP地址不同且能够互通 |
| 容器绑核 | 两个容器内的Cpus_allowed_list都是0-7 |
| 中断绑核 | VF中断只分配到CPU 4-7 |
| qperf | TCP和UDP测试均执行完成，结果文件中没有连接错误 |
| 时延基线对比 | 虚拟机内容器时延与物理机基线的差距在项目规定的可接受范围内 |

不同硬件和网络距离的绝对时延不同。验收时应使用项目规定的时延目标判断，并参照“物理机基线测试”得到的基线数据评估虚拟化开销，不应只以命令执行成功作为低时延达标依据。

## 常见问题

| 现象 | 检查方法 |
| --- | --- |
| Operator看不到VF | 检查compute节点的qemu-kvm标签、虚拟机内lspci -nnD和Operator Pod状态 |
| Operator把管理网卡或集群VF也列出来 | 修正examples/cx5/blacklist.yaml中两张网卡的PCI地址和MAC地址 |
| 节点可用VF数量为0 | 核对examples/cx5/node-policy-cx5.yaml中的厂商编号、设备编号和虚拟机内PCI地址 |
| qperf Pod一直不是Running | 执行kubectl describe pod，检查镜像、VF数量和容器附加网络 |
| 两个容器得到相同IP | 确认使用examples/cx5/host-device.yaml的Whereabouts配置，而不是host-local |
| 找不到cpuset.cpus | 确认容器仍在运行、crictl找到的容器ID和PID不为空；Pod重建后重新定位路径 |
| cpuset.cpus无法写入0-7 | 先确认父cgroup允许CPU 0-7，并确认以root权限执行 |
| qperf无法连接 | 核对server的net1 IP、两个VF所在网络、防火墙状态和qperf server进程 |
| 时延波动明显 | 检查宿主机性能模式、NUMA对齐、大页、容器绑核和VF中断绑核 |

## （可选）卸载

1. 删除测试Pod和VF网络配置。

    ```bash
    kubectl delete -f examples/cx5/qperf-client.yaml --ignore-not-found
    kubectl delete -f examples/cx5/qperf-server.yaml --ignore-not-found
    kubectl delete -f examples/cx5/host-device.yaml --ignore-not-found
    kubectl delete -f examples/cx5/node-policy-cx5.yaml --ignore-not-found
    kubectl delete -f examples/cx5/blacklist.yaml --ignore-not-found
    ```

2. 卸载SR-IOV Network Operator。

    ```bash
    make undeploy-k8s
    ```

3. 恢复测试期间停止的服务。

    ```bash
    sudo systemctl start irqbalance
    sudo systemctl start firewalld
    ```

4. 确认没有虚拟机继续使用VF后，在宿主机删除VF。

    ```bash
    echo 0 | sudo tee /sys/class/net/${PF}/device/sriov_numvfs
    ```

## 修订记录

| 文档版本 | 发布日期 | 修改说明 |
| ------- | -------- | -------- |
|    01   | 2026-09-30 | 第一次正式发布。 |
