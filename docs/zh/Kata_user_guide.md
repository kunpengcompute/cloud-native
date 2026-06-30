# Kata部署指南

## 简介

Kata Containers是一个开源的容器运行时项目，旨在将容器的轻量级特性与虚拟机的安全性隔离优势相结合。Kata Containers通过为每个Pod或容器启动一个轻量级虚拟机（VM），利用硬件虚拟化技术（如KVM）提供强隔离，同时兼容OCI（Open Container Initiative）和Kubernetes容器编排生态。本文档基于鲲鹏ARM架构环境，指导用户完成Kata Containers的安装、配置及验证，并结合Nydus镜像加速方案实现高效的容器镜像分发。

## 约束与限制

- 本文档仅适用于**AArch64（ARM64）**架构，基于鲲鹏920新型号处理器和鲲鹏950处理器验证，不适用于x86_64架构。
- Kata Containers使用Cloud-Hypervisor（CLH）作为虚拟机管理器，需要宿主机支持硬件虚拟化（KVM）。请确认宿主机内核已启用KVM模块（`ls /dev/kvm`）。
- 单节点并发运行大量Kata沙箱（如1000+）时，对宿主机的内存、文件描述符、PID数量等系统资源消耗极大，需按本文档[千级沙箱并发调优](#千级沙箱并发调优)章节进行调整。
- Nydus镜像加速依赖远程Registry服务的网络连通性，请确保集群节点可访问配置的镜像仓库地址。
- Kata运行时中的Pod默认使用独立网络命名空间，HostPID等共享宿主机命名空间特性可能受限或不支持；本文千级Kata容器启动部署实例为减少网络插件限制影响，使用hostNetwork仅用于并发启动验证。
- 部分Kubernetes特性（如Privileged特权模式、某些Device Plugin）在Kata运行时中可能受限或不支持。

## 应用场景

- **多租户隔离**：在云平台或PaaS服务中，不同租户的工作负载需要强安全隔离。Kata Containers通过虚拟机级隔离防止租户间的侧信道攻击和资源逃逸。
- **不可信工作负载运行**：对于来源不可信或安全性要求较高的容器镜像（如第三方提交的代码执行环境），使用Kata Containers可将攻击面限制在虚拟机内部。
- **安全合规场景**：金融、政务等行业对容器隔离有严格合规要求，Kata Containers提供的硬件级隔离能够满足更高的安全标准。
- **AI/代码沙箱**：结合Nydus镜像加速，适用于AI Agent代码执行沙箱等需要快速启动、强隔离的临时运行环境。

## 软件安装

### 环境要求

本文基于特定环境提供指导，在正式操作前请确保软硬件均满足要求。

**表1**  硬件要求

<table><thead align="left"><tr id="row237mcpsimp"><th class="cellrowborder" valign="top" width="27%" id="mcps1.2.3.1.1"><p id="p239mcpsimp">项目</p>
</th>
<th class="cellrowborder" valign="top" width="73%" id="mcps1.2.3.1.2"><p id="p241mcpsimp">说明</p>
</th>
</tr>
</thead>
<tbody><tr id="row243mcpsimp"><td class="cellrowborder" valign="top" width="27%" headers="mcps1.2.3.1.1 "><p id="p245mcpsimp">处理器</p>
</td>
<td class="cellrowborder" valign="top" width="73%" headers="mcps1.2.3.1.2 "><p id="p173486314562">鲲鹏920新型号处理器、鲲鹏950处理器</p>
</td>
</tr>
</tbody>
</table>

**表2**  操作系统和软件要求

<table><thead align="left"><tr id="row254mcpsimp"><th class="cellrowborder" valign="top" width="26.26262626262626%" id="mcps1.2.4.1.1"><p id="p256mcpsimp">项目</p>
</th>
<th class="cellrowborder" valign="top" width="38.38383838383838%" id="mcps1.2.4.1.2"><p id="p258mcpsimp">版本</p>
</th>
<th class="cellrowborder" valign="top" width="35.35353535353536%" id="mcps1.2.4.1.3"><p id="p260mcpsimp">获取方法</p>
</th>
</tr>
</thead>
<tbody><tr id="row262mcpsimp"><td class="cellrowborder" valign="top" width="26.26262626262626%" headers="mcps1.2.4.1.1 "><p id="p264mcpsimp">OS</p>
</td>
<td class="cellrowborder" valign="top" width="38.38383838383838%" headers="mcps1.2.4.1.2 "><p id="p266mcpsimp">openEuler 24.03 LTS SP3</p>
</td>
<td class="cellrowborder" valign="top" width="35.35353535353536%" headers="mcps1.2.4.1.3 "><p id="p268mcpsimp"><a href="https://mirrors.huaweicloud.com/openeuler/openEuler-24.03-LTS-SP3/ISO/aarch64/openEuler-24.03-LTS-SP3-everything-aarch64-dvd.iso" target="_blank" rel="noopener noreferrer">获取链接</a></p>
</td>
</tr>
<tr id="row270mcpsimp"><td class="cellrowborder" valign="top" width="26.26262626262626%" headers="mcps1.2.4.1.1 "><p id="p272mcpsimp">Kubernetes</p>
</td>
<td class="cellrowborder" valign="top" width="38.38383838383838%" headers="mcps1.2.4.1.2 "><p id="p274mcpsimp">1.28.14</p>
</td>
<td class="cellrowborder" valign="top" width="35.35353535353536%" headers="mcps1.2.4.1.3 "><p id="p276mcpsimp"><a href="https://www.hikunpeng.com/document/detail/zh/kunpengcpfs/ecosystemEnable/Kubernetes/kunpengk8s_04_0001.html" target="_blank" rel="noopener noreferrer">获取链接</a></p>
</td>
</tr>
<tr id="row278mcpsimp"><td class="cellrowborder" valign="top" width="26.26262626262626%" headers="mcps1.2.4.1.1 "><p id="p280mcpsimp">Containerd</p>
</td>
<td class="cellrowborder" valign="top" width="38.38383838383838%" headers="mcps1.2.4.1.2 "><p id="p282mcpsimp">1.7.27</p>
</td>
<td class="cellrowborder" valign="top" width="35.35353535353536%" headers="mcps1.2.4.1.3 "><p id="p284mcpsimp"><a href="https://www.hikunpeng.com/document/detail/zh/kunpengcpfs/ecosystemEnable/Containerd/kunpengcontainerd_03_0001.html" target="_blank" rel="noopener noreferrer">获取链接</a></p>
</td>
</tr>
<tr id="row286mcpsimp"><td class="cellrowborder" valign="top" width="26.26262626262626%" headers="mcps1.2.4.1.1 "><p id="p288mcpsimp">Kata Containers</p>
</td>
<td class="cellrowborder" valign="top" width="38.38383838383838%" headers="mcps1.2.4.1.2 "><p id="p290mcpsimp">3.27.0</p>
</td>
<td class="cellrowborder" valign="top" width="35.35353535353536%" headers="mcps1.2.4.1.3 "><p id="p292mcpsimp"><a href="https://github.com/kata-containers/kata-containers/releases/tag/3.27.0" target="_blank" rel="noopener noreferrer">获取链接</a></p>
</td>
</tr>
<tr id="row294mcpsimp"><td class="cellrowborder" valign="top" width="26.26262626262626%" headers="mcps1.2.4.1.1 "><p id="p296mcpsimp">Nydus</p>
</td>
<td class="cellrowborder" valign="top" width="38.38383838383838%" headers="mcps1.2.4.1.2 "><p id="p298mcpsimp">2.4.0</p>
</td>
<td class="cellrowborder" valign="top" width="35.35353535353536%" headers="mcps1.2.4.1.3 "><p id="p300mcpsimp"><a href="https://github.com/dragonflyoss/nydus/releases/tag/v2.4.0" target="_blank" rel="noopener noreferrer">获取链接</a></p>
</td>
</tr>
<tr id="row302mcpsimp"><td class="cellrowborder" valign="top" width="26.26262626262626%" headers="mcps1.2.4.1.1 "><p id="p304mcpsimp">Nydus Snapshotter</p>
</td>
<td class="cellrowborder" valign="top" width="38.38383838383838%" headers="mcps1.2.4.1.2 "><p id="p306mcpsimp">0.15.12</p>
</td>
<td class="cellrowborder" valign="top" width="35.35353535353536%" headers="mcps1.2.4.1.3 "><p id="p308mcpsimp"><a href="https://github.com/containerd/nydus-snapshotter/releases/tag/v0.15.12" target="_blank" rel="noopener noreferrer">获取链接</a></p>
</td>
</tr>
<tr id="row6598192163914"><td class="cellrowborder" valign="top" width="26.26262626262626%" headers="mcps1.2.4.1.1 "><p id="p05981928393">Nerdctl</p>
</td>
<td class="cellrowborder" valign="top" width="38.38383838383838%" headers="mcps1.2.4.1.2 "><p id="p8599112153918">1.7.5</p>
</td>
<td class="cellrowborder" valign="top" width="35.35353535353536%" headers="mcps1.2.4.1.3 "><p id="p1859919203919"><a href="https://github.com/containerd/nerdctl/releases/tag/v1.7.5" target="_blank" rel="noopener noreferrer">获取连接</a></p>
</td>
</tr>
</tbody>
</table>

### 安装Kata及配置

本章节主要提供Kata安装以及使能相关配置。在开始安装部署前，已完成底层环境Kubernetes集群+Containerd的部署配置，并且根据[环境要求](#环境要求)下载Kata安装包。

1. 创建临时目录。

    ```shell
    mkdir ~/kata-temp
    ```

2. 解压Kata安装包到临时目录。

    ```shell
    tar -xvf kata-static-3.27.0-arm64.tar.zst -C ~/kata-temp
    ```

3. 查看解压出来的内容，并拷贝到目标路径。

    ```shell
    ls -R ~/kata-temp | head -n 20 
    cp -ra ~/kata-temp/opt/kata /opt/
    ```

4. 设置权限，确保有root执行权。

    ```shell
    chmod -R +x /opt/kata/bin/
    ```

5. 创建软链接到系统路径。

    ```shell
    ln -sf /opt/kata/bin/kata-runtime /usr/local/bin/kata-runtime 
    ln -sf /opt/kata/bin/containerd-shim-kata-v2 /usr/local/bin/containerd-shim-kata-v2
    ```

6. 验证是否安装成功，并删除临时目录。

    ```shell
    kata-runtime --version
    rm -rf ~/kata-temp
    ```

7. 配置Containerd接入Kata，修改/etc/containerd/config.toml文件，在CRI插件中注册Kata运行时，使用如下命令：

    ```toml
    sed -i '/\[plugins.\"io.containerd.grpc.v1.cri\".containerd.runtimes\]/a\        [plugins.\"io.containerd.grpc.v1.cri\".containerd.runtimes.kata]\n          runtime_type = \"io.containerd.kata.v2\"\n           [plugins.\"io.containerd.grpc.v1.cri\".containerd.runtimes.kata.options]\n             ConfigPath = \"/opt/kata/share/defaults/kata-containers/configuration-clh.toml\"' /etc/containerd/config.toml
    ```

8. 重启containerd服务，使配置生效。

    ```shell
    systemctl restart containerd
    ```

    >**说明：** 
    >建议修改之前备份/etc/containerd/config.toml文件，防止出现问题之后无法恢复。

## （可选）Nydus镜像加速

Nydus是一种容器镜像加速方案，可显著提升Kata容器的镜像拉取和启动速度。Nydus为可选组件，不安装Nydus不影响Kata的正常使用。如需按以下流程操作，请先根据[环境要求](#环境要求)章节下载对应版本的Nydus安装包。

### 安装Nydus及配置

本章节主要提供Nydus组件的安装，以及各项服务配置的操作指导。

1. 安装核心组件RPM包。

    ```shell
    rpm -ivh nydus-static-2.4.0-linux-arm64.rpm
    ```

2. 解压Snapshotter，并拷贝到相关目录。

    ```shell
    tar -zxvf nydus-snapshotter-v0.15.12-linux-arm64.tar.gz 
    cp bin/* /usr/local/bin/
    ```

3. 检查Nydus是否安装成功。

    ```shell
    nydusctl -V
    ```

4. 创建配置文件/etc/nydus/config.toml，配置内容如下：

    ```toml
    version = 1 
     
    root = "/var/lib/containerd/io.containerd.snapshotter.v1.nydus" 
    address = "/run/containerd-nydus/containerd-nydus-grpc.sock" 
    daemon_mode = "dedicated" 
    cleanup_on_close = false 
     
    [daemon] 
    # Specify a configuration file for nydusd 
    nydusd_config = "/etc/nydus/nydusd-config.json" 
    nydusd_path = "/usr/bin/nydusd" 
    nydusimage_path = "/usr/bin/nydus-image" 
    # fusedev or fscache 
    fs_driver = "fusedev" 
    # How to process when daemon dies: "none", "restart" or "failover" 
    recover_policy = "failover" 
    # Nydusd worker thread number to handle FUSE or fscache requests, [0-1024]. 
    # Setting to 0 will use the default configuration of nydusd. 
    threads_number = 4 
     
    [log] 
    log_to_stdout = true 
    level = "debug" 
     
    [snapshot] 
    enable_nydus_overlayfs = true 
    nydus_overlayfs_path = "nydus-overlayfs" 
     
    [system] 
    # Snapshotter's debug and trace HTTP server interface 
    enable = true 
    # Unix domain socket path where system controller is listening on 
    address = "/run/containerd-nydus/system.sock"
    ```

5. 创建配置文件/etc/nydus/nydusd-config.json，配置内容如下：

    ```json
    { 
      "device": { 
        "backend": { 
          "type": "registry", 
          "config": { 
            "timeout": 5, 
            "host": "sealos.hub:5000", 
            "skip_verify": true, 
            "connect_timeout": 5, 
            "retry_limit": 2 
          } 
        }, 
        "cache": { 
          "type": "blobcache", 
          "config": { 
            "work_dir": "/var/lib/containerd/io.containerd.snapshotter.v1.nydus/cache" 
          } 
        } 
      }, 
      "mode": "direct", 
      "digest_validate": false, 
      "iostats_files": false, 
      "enable_xattr": true, 
      "amplify_io": 1048576, 
      "fs_prefetch": { 
        "enable": true, 
        "threads_count": 8, 
        "merging_size": 1048576, 
        "prefetch_all": true 
      } 
    }
    ```

### 创建Nydus Systemd服务

1. 创建服务配置文件。

    ```shell
    vim /etc/systemd/system/containerd-nydus-grpc.service
    ```

2. 增加以下内容并保存。

    ```shell
    [Unit] 
    Description=Nydus containerd snapshotter 
    After=network.target 
     
    [Service] 
    # 确保socket存放目录存在。
    ExecStartPre=/bin/mkdir -p /run/containerd-nydus 
    # 启动grpc服务。
    ExecStart=/usr/local/bin/containerd-nydus-grpc --config /etc/nydus/config.toml  
    Restart=always 
    RestartSec=1 
    KillMode=process 
    OOMScoreAdjust=-999 
    StandardOutput=journal 
    StandardError=journal 
     
    [Install] 
    WantedBy=multi-user.target
    ```

3. 启动服务并设置开机自启动。

    ```shell
    systemctl daemon-reload 
    systemctl enable --now containerd-nydus-grpc.service
    ```

4. 检查sock文件是否成功生成。

    ```shell
    ls -l /run/containerd-nydus/containerd-nydus-grpc.sock
    ```

### 配置Nydus接入Containerd

1. 编辑/etc/containerd/config.toml文件，修改增加如下内容：

    ```toml
        [plugins."io.containerd.grpc.v1.cri".containerd] 
          snapshotter = "nydus" 
          disable_snapshot_annotations = false 
          discard_unpacked_layers = false  # 确保未解压的层不会被异常丢弃。
          default_runtime_name = "runc" 
     
    [proxy_plugins] 
      [proxy_plugins.nydus] 
        type = "snapshot"  # 重点修正：官方文档要求这里是snapshot。
        address = "/run/containerd-nydus/containerd-nydus-grpc.sock"
    ```

2. 重启Containerd生效。

    ```shell
    systemctl restart containerd
    ```

### 配置Nydus到Kata中

修改Kata配置文件/opt/kata/share/defaults/kata-containers/configuration-clh.toml，修改参数如下：

    ```toml
    shared_fs = "virtio-fs-nydus" 
    virtio_fs_daemon = "/usr/bin/nydusd" 
    valid_virtio_fs_daemon_paths = ["/usr/bin/nydusd"] 
    virtio_fs_extra_args = []
    ```

## 验证使用Kata

本章节主要提供在Kubernetes集群+Containerd中Kata的使用验证方式。

### 准备镜像

1. 下载sandbox-templates镜像并导入Containerd中。

    ```shell
    docker pull docker.io/docker/sandbox-templates:claude-code 
    docker save -o sandbox-template-claude-code.tar docker.io/docker/sandbox-templates:claude-code 
    ctr -n k8s.io images import sandbox-template-claude-code.tar
    ```

2. nerdctl登录远程仓库：

    ```shell
    nerdctl login -u admin -p passw0rd sealos.hub:5000 --insecure-registry
    ```

3. 转换sandbox-templates镜像。

    ```shell
    nerdctl -n k8s.io image convert --nydus --oci docker/sandbox-templates:claude-code sealos.hub:5000/sandbox-templates:claude-code-nydus
    ```

4. 把镜像推送至sealos仓。

    ```shell
    ctr -n k8s.io images push --plain-http --user admin:passw0rd sealos.hub:5000/sandbox-templates:claude-code-nydus
    nerdctl -n k8s.io image push sealos.hub:5000/sandbox-templates:claude-code-nydus
    ```

5. 推送完成后，删除本地镜像。

    ```shell
    crictl rmi sealos.hub:5000/sandbox-templates:claude-code-nydus
    ```

6. 查看仓库是否包含对应的镜像。

    ```shell
    curl -u admin:passw0rd http://sealos.hub:5000/v2/_catalog
    ```

    >**说明：** 
    >1. 可以是任意镜像，本文档的镜像仅用于验证Kata、Nydus是否能正常使用，仅供参考。
    >2. 如果使用sealos安装的Kubernetes集群，可以参考本文档将镜像推送至本地sealos仓库，请根据实际环境需求更改。

### 在Kubernetes中使能Kata

1. 在k8s中创建RuntimeClass资源。

    ```shell
    cat <<EOF | kubectl apply -f -  
    apiVersion: node.k8s.io/v1  
    kind: RuntimeClass  
    metadata:  
      name: kata-runtime 
    handler: kata 
    EOF
    ```

    >**说明：** 
    >handler的名称必须对应/etc/containerd/config.toml配置文件中\[plugins...runtimes.**kata**\]的后缀，本文档示例为:kata。

2. 检查是否成功创建。

    ```txt
    # kubectl get runtimeclass 

     NAME           HANDLER   AGE
     kata-runtime   kata      16m
    ```

3. 在Pod的YAML中加入runtimeClassName: kata-runtime，将Pod接入Kata。部署文件示例如下所示：

    ```yaml
    apiVersion: v1 
    kind: Pod 
    metadata: 
      name: claude-code-sandbox 
    spec: 
      # 1. 指定使用Kata运行时。
      runtimeClassName: kata-runtime 
     
      # 2. 强制指定部署到名为master的节点。
      # 注意：请通过kubectl get nodes确认你的节点名称确实叫master。
      nodeName: master 
     
      # 3. 关键：容忍Master节点的污点。
      # Sealos默认会给Master加上不可调度的污点，必须加上这个配置Pod才能启动。
      tolerations: 
      - key: "node-role.kubernetes.io/control-plane" 
        operator: "Exists" 
        effect: "NoSchedule" 
      - key: "node-role.kubernetes.io/master" 
        operator: "Exists" 
        effect: "NoSchedule" 
     
      containers: 
      - name: sandbox-container 
        image: docker/sandbox-templates:claude-code
        imagePullPolicy: IfNotPresent 
        stdin: true 
        tty: true 
        command: ["/bin/bash"] 
        args: ["-c", "sleep infinity"] 
        resources: 
          requests: 
            cpu: "500m" 
            memory: "1Gi"
    ```

4. 启动Pod，检查是否接入成功Kata。

    ```shell
    ps aux | grep cloud-hypervisor
    ```

### （可选）验证使能启动Kata+Nydus

以下验证依赖Nydus镜像加速组件，请先完成[（可选）Nydus镜像加速](#可选nydus镜像加速)章节的全部配置。

1. 配置nydus-sandbox.yaml。

    ```yaml
    metadata: 
      attempt: 1 
      name: nydus-sandbox-2 
      uid: nydus-uid-2 
      namespace: default 
    log_directory: /tmp 
    linux: 
      security_context: 
        namespace_options: 
          network: 2 
    annotations: 
      "io.containerd.osfeature": "nydus.remoteimage.v1"
    ```

2. 配置nydus-container.yaml。

    ```yaml
    metadata: 
      name: nydus-container 
    image: 
      image: sealos.hub:5000/sandbox-templates:claude-code-nydus 
    command: 
      - /bin/sleep 
    args: 
      - 600 
    log_path: container.1.log
    ```

3. 部署测试。

    ```shell
    crictl run -r kata  nydus-container.yaml nydus-sandbox.yaml
    ```

## 千级沙箱并发调优

在单节点中并发创建1000个kata沙箱，需调整调优配置。

### 最大Pod数量限制配置

1. 修改/var/lib/kubelet/config.yaml文件，把maxPods参数放宽到1000以上。

    ```yaml
    maxOpenFiles: 1000000
    maxPods: 2000
    memoryManagerPolicy: None
    ```

2. 重启kubelet服务使之生效。

    ```shell
    systemctl daemon-reload
    systemctl restart kubelet
    ```

### 内核参数限制修改

Kata Containers每个Pod都是一个cloud-hypervisor进程，这对宿主机的系统句柄、ARP缓存表、PID数量消耗极大，需要放大Linux内核的限制。

1. 在/etc/sysctl.conf中增加如下配置：

    ```conf
    # 1. 增加inotify限制（非常重要，每个虚拟机需要监控很多文件句柄）。
    fs.inotify.max_user_watches = 1048576
    fs.inotify.max_user_instances = 81920
    
    # 2. 扩大ARP缓存表（防止容器多了之后网络不通、丢包）。
    net.ipv4.neigh.default.gc_thresh1 = 4096
    net.ipv4.neigh.default.gc_thresh2 = 8192
    net.ipv4.neigh.default.gc_thresh3 = 16384
    
    # 3. 扩大系统文件描述符和PID限制。
    fs.file-max = 2097152
    kernel.pid_max = 4194304
    
    # 4. 优化网络连接跟踪（Conntrack），防止高并发连接打满。
    net.netfilter.nf_conntrack_max = 2097152
    ```

2. 执行命令生效。

    ```shell
    sysctl -p
    ```

### 千级Kata容器启动部署实例

完成上述Kata运行时配置、RuntimeClass创建和系统参数调优后，可通过Deployment在单节点上批量启动Kata容器。以下示例在`master`节点上创建1000个Kata Pod，用于验证千级Kata沙箱并发启动能力。

>**说明：** 
>
>1. 示例中的`nodeName: master`、镜像地址、CPU和内存请求仅供参考，请根据实际节点名称、镜像仓库和宿主机资源规格调整。
>2. 示例通过`hostNetwork: true`让Pod使用宿主机网络，减少CNI插件、Pod IP分配和ARP表规模对千级启动验证的影响。使用宿主机网络时Pod没有独立Pod网络，且监听端口会与宿主机端口共享，请勿在容器中启动固定监听端口相同的服务。
>3. 如果节点存在Master或Control Plane污点，需要保留`tolerations`配置，否则Pod会因污点无法调度。
>4. 千级Kata Pod会同时拉起大量`cloud-hypervisor`进程，建议先按100、300、600、1000逐步扩容，观察CPU、内存、PID、文件句柄和系统网络状态。

1. 创建部署命名空间。

    ```shell
    kubectl create namespace kata-scale
    ```

2. 创建千级Kata容器Deployment配置文件kata-sandbox-1000.yaml。

    ```yaml
    apiVersion: apps/v1
    kind: Deployment
    metadata:
      name: kata-sandbox-1000
      namespace: kata-scale
    spec:
      # 建议首次部署时先设置为100，确认资源稳定后再逐步扩容到1000。
      replicas: 100
      selector:
        matchLabels:
          app: kata-sandbox-scale
      template:
        metadata:
          labels:
            app: kata-sandbox-scale
        spec:
          runtimeClassName: kata-runtime
          nodeName: master
          hostNetwork: true
          dnsPolicy: ClusterFirstWithHostNet
          terminationGracePeriodSeconds: 0
          tolerations:
          - key: "node-role.kubernetes.io/control-plane"
            operator: "Exists"
            effect: "NoSchedule"
          - key: "node-role.kubernetes.io/master"
            operator: "Exists"
            effect: "NoSchedule"
          containers:
          - name: sandbox-container
            image: docker/sandbox-templates:claude-code
            imagePullPolicy: IfNotPresent
            command: ["/bin/bash"]
            args: ["-c", "sleep infinity"]
            resources:
              requests:
                cpu: "100m"
                memory: "256Mi"
              limits:
                cpu: "500m"
                memory: "1Gi"
    ```

    如果需要验证Kata+Nydus镜像加速，可将`image`替换为前文转换后的Nydus镜像，例如：

    ```yaml
    image: sealos.hub:5000/sandbox-templates:claude-code-nydus
    ```

3. 部署并分批扩容。

    ```shell
    kubectl apply -f kata-sandbox-1000.yaml
    kubectl -n kata-scale rollout status deployment/kata-sandbox-1000

    kubectl -n kata-scale scale deployment kata-sandbox-1000 --replicas=300
    kubectl -n kata-scale rollout status deployment/kata-sandbox-1000

    kubectl -n kata-scale scale deployment kata-sandbox-1000 --replicas=600
    kubectl -n kata-scale rollout status deployment/kata-sandbox-1000

    kubectl -n kata-scale scale deployment kata-sandbox-1000 --replicas=1000
    kubectl -n kata-scale rollout status deployment/kata-sandbox-1000
    ```

4. 检查Pod运行状态。

    ```shell
    kubectl -n kata-scale get pods -o wide
    kubectl -n kata-scale get pods --field-selector=status.phase=Running --no-headers | wc -l
    ```

    当Running状态Pod数量达到1000时，说明Kubernetes层面已完成千级Kata容器启动。

5. 检查Kata虚拟机进程数量。

    ```shell
    ps -ef | grep cloud-hypervisor | grep -v grep | wc -l
    ```

    该数量应接近Running状态Pod数量。若数量明显偏少，请通过如下命令查看失败原因。

    ```shell
    kubectl -n kata-scale get events --sort-by=.lastTimestamp
    kubectl -n kata-scale describe pod <pod-name>
    journalctl -u kubelet -f
    journalctl -u containerd -f
    ```

6. 验证完成后清理测试资源。

    ```shell
    kubectl delete namespace kata-scale
    ```
