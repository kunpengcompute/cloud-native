# 特性描述

### 简介

### 版本支持

### 约束与限制

### 应用场景

# 软件安装
### 环境要求

本文基于特定环境提供指导，在正式操作前请确保软硬件均满足要求。

**表 1**  硬件要求

<a name="table232mcpsimp"></a>
<table><thead align="left"><tr id="row237mcpsimp"><th class="cellrowborder" valign="top" width="27%" id="mcps1.2.3.1.1"><p id="p239mcpsimp"><a name="p239mcpsimp"></a><a name="p239mcpsimp"></a>项目</p>
</th>
<th class="cellrowborder" valign="top" width="73%" id="mcps1.2.3.1.2"><p id="p241mcpsimp"><a name="p241mcpsimp"></a><a name="p241mcpsimp"></a>说明</p>
</th>
</tr>
</thead>
<tbody><tr id="row243mcpsimp"><td class="cellrowborder" valign="top" width="27%" headers="mcps1.2.3.1.1 "><p id="p245mcpsimp"><a name="p245mcpsimp"></a><a name="p245mcpsimp"></a>处理器</p>
</td>
<td class="cellrowborder" valign="top" width="73%" headers="mcps1.2.3.1.2 "><p id="p173486314562"><a name="p173486314562"></a><a name="p173486314562"></a>鲲鹏920新型号处理器、鲲鹏950处理器</p>
</td>
</tr>
</tbody>
</table>

**表 2**  操作系统和软件要求

<a name="table248mcpsimp"></a>
<table><thead align="left"><tr id="row254mcpsimp"><th class="cellrowborder" valign="top" width="26.26262626262626%" id="mcps1.2.4.1.1"><p id="p256mcpsimp"><a name="p256mcpsimp"></a><a name="p256mcpsimp"></a>项目</p>
</th>
<th class="cellrowborder" valign="top" width="38.38383838383838%" id="mcps1.2.4.1.2"><p id="p258mcpsimp"><a name="p258mcpsimp"></a><a name="p258mcpsimp"></a>版本</p>
</th>
<th class="cellrowborder" valign="top" width="35.35353535353536%" id="mcps1.2.4.1.3"><p id="p260mcpsimp"><a name="p260mcpsimp"></a><a name="p260mcpsimp"></a>获取方法</p>
</th>
</tr>
</thead>
<tbody><tr id="row262mcpsimp"><td class="cellrowborder" valign="top" width="26.26262626262626%" headers="mcps1.2.4.1.1 "><p id="p264mcpsimp"><a name="p264mcpsimp"></a><a name="p264mcpsimp"></a>OS</p>
</td>
<td class="cellrowborder" valign="top" width="38.38383838383838%" headers="mcps1.2.4.1.2 "><p id="p266mcpsimp"><a name="p266mcpsimp"></a><a name="p266mcpsimp"></a>openEuler 24.03 LTS SP3</p>
</td>
<td class="cellrowborder" valign="top" width="35.35353535353536%" headers="mcps1.2.4.1.3 "><p id="p268mcpsimp"><a name="p268mcpsimp"></a><a name="p268mcpsimp"></a><a href="https://mirrors.huaweicloud.com/openeuler/openEuler-24.03-LTS-SP3/ISO/aarch64/openEuler-24.03-LTS-SP3-everything-aarch64-dvd.iso" target="_blank" rel="noopener noreferrer">获取链接</a></p>
</td>
</tr>
<tr id="row270mcpsimp"><td class="cellrowborder" valign="top" width="26.26262626262626%" headers="mcps1.2.4.1.1 "><p id="p272mcpsimp"><a name="p272mcpsimp"></a><a name="p272mcpsimp"></a>Kubernetes</p>
</td>
<td class="cellrowborder" valign="top" width="38.38383838383838%" headers="mcps1.2.4.1.2 "><p id="p274mcpsimp"><a name="p274mcpsimp"></a><a name="p274mcpsimp"></a>1.28.14</p>
</td>
<td class="cellrowborder" valign="top" width="35.35353535353536%" headers="mcps1.2.4.1.3 "><p id="p276mcpsimp"><a name="p276mcpsimp"></a><a name="p276mcpsimp"></a><a href="https://www.hikunpeng.com/document/detail/zh/kunpengcpfs/ecosystemEnable/Kubernetes/kunpengk8s_04_0001.html" target="_blank" rel="noopener noreferrer">获取链接</a></p>
</td>
</tr>
<tr id="row278mcpsimp"><td class="cellrowborder" valign="top" width="26.26262626262626%" headers="mcps1.2.4.1.1 "><p id="p280mcpsimp"><a name="p280mcpsimp"></a><a name="p280mcpsimp"></a>Containerd</p>
</td>
<td class="cellrowborder" valign="top" width="38.38383838383838%" headers="mcps1.2.4.1.2 "><p id="p282mcpsimp"><a name="p282mcpsimp"></a><a name="p282mcpsimp"></a>1.7.27</p>
</td>
<td class="cellrowborder" valign="top" width="35.35353535353536%" headers="mcps1.2.4.1.3 "><p id="p284mcpsimp"><a name="p284mcpsimp"></a><a name="p284mcpsimp"></a><a href="https://www.hikunpeng.com/document/detail/zh/kunpengcpfs/ecosystemEnable/Containerd/kunpengcontainerd_03_0001.html" target="_blank" rel="noopener noreferrer">获取链接</a></p>
</td>
</tr>
<tr id="row286mcpsimp"><td class="cellrowborder" valign="top" width="26.26262626262626%" headers="mcps1.2.4.1.1 "><p id="p288mcpsimp"><a name="p288mcpsimp"></a><a name="p288mcpsimp"></a>Kata Containers</p>
</td>
<td class="cellrowborder" valign="top" width="38.38383838383838%" headers="mcps1.2.4.1.2 "><p id="p290mcpsimp"><a name="p290mcpsimp"></a><a name="p290mcpsimp"></a>3.27.0</p>
</td>
<td class="cellrowborder" valign="top" width="35.35353535353536%" headers="mcps1.2.4.1.3 "><p id="p292mcpsimp"><a name="p292mcpsimp"></a><a name="p292mcpsimp"></a><a href="https://github.com/kata-containers/kata-containers/releases/tag/3.27.0" target="_blank" rel="noopener noreferrer">获取链接</a></p>
</td>
</tr>
<tr id="row294mcpsimp"><td class="cellrowborder" valign="top" width="26.26262626262626%" headers="mcps1.2.4.1.1 "><p id="p296mcpsimp"><a name="p296mcpsimp"></a><a name="p296mcpsimp"></a>Nydus (核心组件)</p>
</td>
<td class="cellrowborder" valign="top" width="38.38383838383838%" headers="mcps1.2.4.1.2 "><p id="p298mcpsimp"><a name="p298mcpsimp"></a><a name="p298mcpsimp"></a>2.4.0</p>
</td>
<td class="cellrowborder" valign="top" width="35.35353535353536%" headers="mcps1.2.4.1.3 "><p id="p300mcpsimp"><a name="p300mcpsimp"></a><a name="p300mcpsimp"></a><a href="https://github.com/dragonflyoss/nydus/releases/tag/v2.4.0" target="_blank" rel="noopener noreferrer">获取链接</a></p>
</td>
</tr>
<tr id="row302mcpsimp"><td class="cellrowborder" valign="top" width="26.26262626262626%" headers="mcps1.2.4.1.1 "><p id="p304mcpsimp"><a name="p304mcpsimp"></a><a name="p304mcpsimp"></a>Nydus Snapshotter</p>
</td>
<td class="cellrowborder" valign="top" width="38.38383838383838%" headers="mcps1.2.4.1.2 "><p id="p306mcpsimp"><a name="p306mcpsimp"></a><a name="p306mcpsimp"></a>0.15.12</p>
</td>
<td class="cellrowborder" valign="top" width="35.35353535353536%" headers="mcps1.2.4.1.3 "><p id="p308mcpsimp"><a name="p308mcpsimp"></a><a name="p308mcpsimp"></a><a href="https://github.com/containerd/nydus-snapshotter/releases/tag/v0.15.12" target="_blank" rel="noopener noreferrer">获取链接</a></p>
</td>
</tr>
<tr id="row6598192163914"><td class="cellrowborder" valign="top" width="26.26262626262626%" headers="mcps1.2.4.1.1 "><p id="p05981928393"><a name="p05981928393"></a><a name="p05981928393"></a>Nerdctl</p>
</td>
<td class="cellrowborder" valign="top" width="38.38383838383838%" headers="mcps1.2.4.1.2 "><p id="p8599112153918"><a name="p8599112153918"></a><a name="p8599112153918"></a>1.7.5</p>
</td>
<td class="cellrowborder" valign="top" width="35.35353535353536%" headers="mcps1.2.4.1.3 "><p id="p1859919203919"><a name="p1859919203919"></a><a name="p1859919203919"></a><a href="https://github.com/containerd/nerdctl/releases/tag/v1.7.5" target="_blank" rel="noopener noreferrer">获取连接</a></p>
</td>
</tr>
</tbody>
</table>

### 安装Kata及配置

本章节主要提供Kata安装以及使能相关配置。在开始安装部署前，已完成底层环境Kubernetes集群 + Containerd的部署配置。并且根据环境要求下载Kata安装包。

1.  创建临时目录。

    ```
    mkdir ~/kata-temp
    ```

2.  解压Kata安装包到临时目录。

    ```
    tar -xvf kata-static-3.27.0-arm64.tar.zst -C ~/kata-temp
    ```

3.  查看解压出来的内容，并拷贝到目标路径。

    ```
    ls -R ~/kata-temp | head -n 20 
    cp -ra ~/kata-temp/opt/kata /opt/
    ```

4.  设置权限，确保有root 执行权。

    ```
    chmod -R +x /opt/kata/bin/
    ```

5.  创建软链接到系统路径。

    ```
    ln -sf /opt/kata/bin/kata-runtime /usr/local/bin/kata-runtime 
    ln -sf /opt/kata/bin/containerd-shim-kata-v2 /usr/local/bin/containerd-shim-kata-v2
    ```

6.  验证是否安装成功，并删除临时文件。

    ```
    kata-runtime --version
    rm -rf ~/kata-temp
    ```

7.  配置 Containerd 接入 Kata，修改/etc/containerd/config.toml文件，在 CRI 插件中注册 Kata 运行时，使用如下命令：

    ```
    sed -i '/\[plugins.\"io.containerd.grpc.v1.cri\".containerd.runtimes\]/a\        [plugins.\"io.containerd.grpc.v1.cri\".containerd.runtimes.kata]\n          runtime_type = \"io.containerd.kata.v2\"\n           [plugins.\"io.containerd.grpc.v1.cri\".containerd.runtimes.kata.options]\n             ConfigPath = \"/opt/kata/share/defaults/kata-containers/configuration-clh.toml\"' /etc/containerd/config.toml
    ```

8.  重启containerd服务，使配置生效

    ```
    systemctl restart containerd
    ```

    >![](public_sys-resources/icon-note.gif) **说明：** 
    >建议修改之前备份/etc/containerd/config.toml文件，防止出现问题之后无法恢复

### 安装Nydus及配置

本章节主要提供Nydus组件的安装，以及各项服务配置的操作指导。    


1.  安装核心组件RPM包

    ```
    rpm -ivh nydus-static-2.4.0-linux-arm64.rpm
    ```

2.  解压Snapshotter，并拷贝到相关目录

    ```
    tar -zxvf nydus-snapshotter-v0.15.12-linux-arm64.tar.gz 
    cp bin/* /usr/local/bin/
    ```

3.  检查Nydus是否安装成功

    ```
    nydusctl -V
    ```

4.  创建配置文件/etc/nydus/config.toml，配置内容如下：

    ```
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

5.  创建配置文件 /etc/nydus/nydusd-config.json，配置内容如下：

    ```
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

1.  创建服务配置文件

    ```
    vim /etc/systemd/system/containerd-nydus-grpc.service
    ```

2.  增加以下内容并保存

    ```
    [Unit] 
    Description=Nydus containerd snapshotter 
    After=network.target 
     
    [Service] 
    # 确保 socket 存放目录存在 
    ExecStartPre=/bin/mkdir -p /run/containerd-nydus 
    # 启动 grpc 服务 
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

3.  启动服务并设置开机自启动

    ```
    systemctl daemon-reload 
    systemctl enable --now containerd-nydus-grpc.service
    ```

4.  检查sock文件是否成功生成

    ```
    ls -l /run/containerd-nydus/containerd-nydus-grpc.sock
    ```

### 配置Nydus接入Containerd

1.  编辑/etc/containerd/config.toml文件，修改增加如下内容：

    ```
        [plugins."io.containerd.grpc.v1.cri".containerd] 
          snapshotter = "nydus" 
          disable_snapshot_annotations = false 
          discard_unpacked_layers = false  # 确保未解压的层不会被异常丢弃 
          default_runtime_name = "runc" 
     
    [proxy_plugins] 
      [proxy_plugins.nydus] 
        type = "snapshot"  # 重点修正：官方文档要求这里是 snapshot 
        address = "/run/containerd-nydus/containerd-nydus-grpc.sock"
    ```

2.  重启Containerd生效

    ```
    systemctl restart containerd
    ```

### 配置Nydus到Kata中

1.  修改Kata配置文件/opt/kata/share/defaults/kata-containers/configuration-clh.toml，修改参数如下：

    ```
    shared_fs = "virtio-fs-nydus" 
    virtio_fs_daemon = "/usr/bin/nydusd" 
    valid_virtio_fs_daemon_paths = ["/usr/bin/nydusd"] 
    virtio_fs_extra_args = []
    ```

# 验证使用Kata和Nydus
本章节主要提供在Kubernetes集群 + Containerd中Kata和Nydus的使用方式。

### 准备镜像

1.  下载sandbox-templates镜像并导入Containerd中

    ```
    docker pull docker.io/docker/sandbox-templates:claude-code 
    docker save -o sandbox-template-claude-code.tar docker.io/docker/sandbox-templates:claude-code 
    ctr -n k8s.io images import sandbox-template-claude-code.tar
    ```

2.  nerdctl 登录远程仓库：

    ```
    nerdctl login -u admin -p passw0rd sealos.hub:5000 --insecure-registry
    ```

3.  转换sandbox-templates镜像

    ```
    nerdctl -n k8s.io image convert --nydus --oci docker/sandbox-templates:claude-code sealos.hub:5000/sandbox-templates:claude-code-nydus
    ```

4.  把镜像推送至sealos仓

    ```
    ctr -n k8s.io images push --plain-http --user admin:passw0rd sealos.hub:5000/sandbox-templates:claude-code-nydus
    nerdctl -n k8s.io image push sealos.hub:5000/sandbox-templates:claude-code-nydus
    ```

5.  推送完成后，删除本地镜像

    ```
    crictl rmi sealos.hub:5000/sandbox-templates:claude-code-nydus
    ```

6.  查看仓库是否包含对应的镜像

    ```
    curl -u admin:passw0rd http://sealos.hub:5000/v2/_catalog
    ```

    >![](public_sys-resources/icon-note.gif) **说明：** 
    >1. 可以是任意镜像，本文档的镜像仅用于验证Kata、Nydus是否能正常使用，仅供参考。
    >2. 如果使用sealos安装的Kubernetes集群，可以参考本文档将镜像推送至本地sealos仓库，请根据实际环境需求更改。

### 在Kubernetes中使能Kata

1.  在k8s中创建 RuntimeClass 资源

    ```
    cat <<EOF | kubectl apply -f -  
    apiVersion: node.k8s.io/v1  
    kind: RuntimeClass  
    metadata:  
      name: kata-runtime 
    handler: kata 
    EOF
    ```

    >![](public_sys-resources/icon-note.gif) **说明：** 
    >handler 的名称必须对应Kata配置/etc/containerd/config.toml配置文件中 \[plugins...runtimes.**kata**\] 的后缀，本文档示例为:kata

2.  检查是否成功创建

    ```
    # kubectl get runtimeclass 

     NAME           HANDLER   AGE
     kata-runtime   kata      16m
    ```

3.  在Pod的YAML中加入 runtimeClassName: kata-runtime，将Pod接入Kata。部署文件示例如下所示：

    ```
    apiVersion: v1 
    kind: Pod 
    metadata: 
      name: claude-code-sandbox 
    spec: 
      # 1. 指定使用 Kata 运行时 
      runtimeClassName: kata-runtime 
     
      # 2. 强制指定部署到名为 master 的节点 
      # 注意：请通过 kubectl get nodes 确认你的节点名称确实叫 master 
      nodeName: master 
     
      # 3. 关键：容忍 Master 节点的污点 
      # Sealos 默认会给 Master 加上不可调度的污点，必须加上这个配置 Pod 才能启动 
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

.  启动Pod，检查是否接入成功Kata

    ```
    ps aux | grep cloud-hypervisor
    ```


### 验证使能启动Kata+Nydus

1.  配置nydus-sandbox.yaml

    ```
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

2.  配置nydus-container.yaml

    ```
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

3.  部署测试

    ```
    crictl run -r kata  nydus-container.yaml nydus-sandbox.yaml
    ```

# 千级沙箱并发调优<a name="ZH-CN_TOPIC_0000002590641823"></a>

单节点可并发创建1000个kata沙箱的调优配置

### 最大Pod数量限制配置<a name="section16275145523814"></a>

1.  修改/var/lib/kubelet/config.yaml文件，把maxPods 参数放款到1000以上

    ```
    maxOpenFiles: 1000000
    maxPods: 2000
    memoryManagerPolicy: None
    ```

2.  重启kubelet服务使之生效

    ```
    systemctl daemon-reload
    systemctl restart kubelet
    ```

### 内核参数限制修改<a name="section428613567371"></a>

Kata Containers 每个Pod都是一个cloud-hypervisor进程，这对宿主机的系统句柄、ARP 缓存表、PID 数量消耗极大，需要放大 Linux 内核的限制。

1.  在/etc/sysctl.conf 中增加如下配置：

    ```
    # 1. 增加 inotify 限制（非常重要，每个虚拟机需要监控很多文件句柄） 
    fs.inotify.max_user_watches = 1048576
    fs.inotify.max_user_instances = 81920
    
    # 2. 扩大 ARP 缓存表（防止容器多了之后网络不通、丢包） 
    net.ipv4.neigh.default.gc_thresh1 = 4096
    net.ipv4.neigh.default.gc_thresh2 = 8192
    net.ipv4.neigh.default.gc_thresh3 = 16384
    
    # 3. 扩大系统文件描述符和 PID 限制 
    fs.file-max = 2097152 kernel.pid_max = 4194304
    
    # 4. 优化网络连接跟踪（Conntrack），防止高并发连接打满 
    net.netfilter.nf_conntrack_max = 2097152
    ```

2.  执行命令生效

    ```
    sysctl -p
    ```