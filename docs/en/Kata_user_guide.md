# Kata Deployment Guide

## Overview

Kata Containers is an open-source container runtime project that aims to combine the lightweight container features with the security isolation advantages of virtual machines (VMs). Kata Containers starts a lightweight VM for each pod or container, and uses hardware virtualization technologies (such as KVM) to provide strong isolation. It is compatible with the Open Container Initiative (OCI) and Kubernetes container orchestration ecosystem. This document describes how to install, configure, and verify Kata Containers in the Kunpeng Arm architecture, and how to use the Nydus image acceleration solution to efficiently distribute container images.

## Constraints

- This document applies only to the **AArch64** architecture and has been verified based on the new Kunpeng 920 processor models and Kunpeng 950 processors. It is not applicable to the x86_64 architecture.
- Kata Containers uses Cloud-Hypervisor (CLH) as the VM manager, which requires the host machine to support hardware virtualization (KVM). Ensure that the KVM module has been enabled in the host machine kernel (`ls /dev/kvm`).
- When a large number of Kata sandboxes (for example, more than 1,000) are concurrently running on a single node, the system resources of the host machine, such as memory, file descriptors, and PIDs, are consumed significantly. You need to adjust the system resources by following instructions in [Optimization for Concurrent Creation of Thousands of Sandboxes](#optimization-for-concurrent-creation-of-thousands-of-sandboxes) in this document.
- Nydus image acceleration depends on the network connectivity of the remote registry service. Ensure that the cluster nodes can access the configured image repository address.
- By default, pods in the Kata runtime use independent network namespaces. The host machine namespace sharing features, such as HostPID, may be restricted or not supported. In this document, `hostNetwork` is used, for only verification of concurrent startup of thousands of Kata containers, to reduce the impact of network plugin restrictions.
- Some Kubernetes features (such as the privileged mode and some device plugins) may be restricted or not supported in the Kata runtime.

## Application Scenarios

- **Multi-tenant isolation**: On cloud platforms or in PaaS services, workloads from different tenants require strong security isolation. Kata Containers uses VM-level isolation to prevent side-channel attacks between tenants and resource escape.
- **Untrusted workload running**: For container images with untrusted sources or high security requirements (for example, the code execution environment submitted by a third party), Kata Containers can be used to limit the attack surface to the VMs.
- **Security compliance**: Industries such as finance and government have strict compliance requirements on container isolation. Kata Containers provides hardware-level isolation to meet higher security standards.
- **AI/Code sandbox**: Combined with Nydus image acceleration, this feature is applicable to temporary running environments that require fast startup and strong isolation, such as the AI agent code execution sandbox.

## Software Installation

### Environment Requirements

This document provides guidance based on specific environments. Before performing operations, ensure that your hardware and software meet the requirements.

**Table 1** Hardware requirement

<table><thead align="left"><tr id="row237mcpsimp"><th class="cellrowborder" valign="top" width="27%" id="mcps1.2.3.1.1"><p id="p239mcpsimp">Item</p>
</th>
<th class="cellrowborder" valign="top" width="73%" id="mcps1.2.3.1.2"><p id="p241mcpsimp">Description</p>
</th>
</tr>
</thead>
<tbody><tr id="row243mcpsimp"><td class="cellrowborder" valign="top" width="27%" headers="mcps1.2.3.1.1 "><p id="p245mcpsimp">Processor</p>
</td>
<td class="cellrowborder" valign="top" width="73%" headers="mcps1.2.3.1.2 "><p id="p173486314562">New Kunpeng 920 processor model or Kunpeng 950 processor</p>
</td>
</tr>
</tbody>
</table>

**Table 2** OS and software requirements

<table><thead align="left"><tr id="row254mcpsimp"><th class="cellrowborder" valign="top" width="26.26262626262626%" id="mcps1.2.4.1.1"><p id="p256mcpsimp">Item</p>
</th>
<th class="cellrowborder" valign="top" width="38.38383838383838%" id="mcps1.2.4.1.2"><p id="p258mcpsimp">Version</p>
</th>
<th class="cellrowborder" valign="top" width="35.35353535353536%" id="mcps1.2.4.1.3"><p id="p260mcpsimp">How to Obtain</p>
</th>
</tr>
</thead>
<tbody><tr id="row262mcpsimp"><td class="cellrowborder" valign="top" width="26.26262626262626%" headers="mcps1.2.4.1.1 "><p id="p264mcpsimp">OS</p>
</td>
<td class="cellrowborder" valign="top" width="38.38383838383838%" headers="mcps1.2.4.1.2 "><p id="p266mcpsimp">openEuler 24.03 LTS SP3</p>
</td>
<td class="cellrowborder" valign="top" width="35.35353535353536%" headers="mcps1.2.4.1.3 "><p id="p268mcpsimp"><a href="https://mirrors.huaweicloud.com/openeuler/openEuler-24.03-LTS-SP3/ISO/aarch64/openEuler-24.03-LTS-SP3-everything-aarch64-dvd.iso" target="_blank" rel="noopener noreferrer">Link</a></p>
</td>
</tr>
<tr id="row270mcpsimp"><td class="cellrowborder" valign="top" width="26.26262626262626%" headers="mcps1.2.4.1.1 "><p id="p272mcpsimp">Kubernetes</p>
</td>
<td class="cellrowborder" valign="top" width="38.38383838383838%" headers="mcps1.2.4.1.2 "><p id="p274mcpsimp">1.28.14</p>
</td>
<td class="cellrowborder" valign="top" width="35.35353535353536%" headers="mcps1.2.4.1.3 "><p id="p276mcpsimp"><a href="https://www.hikunpeng.com/document/detail/en/kunpengcpfs/ecosystemEnable/Kubernetes/kunpengk8s_04_0001.html" target="_blank" rel="noopener noreferrer">Link</a></p>
</td>
</tr>
<tr id="row278mcpsimp"><td class="cellrowborder" valign="top" width="26.26262626262626%" headers="mcps1.2.4.1.1 "><p id="p280mcpsimp">containerd</p>
</td>
<td class="cellrowborder" valign="top" width="38.38383838383838%" headers="mcps1.2.4.1.2 "><p id="p282mcpsimp">1.7.27</p>
</td>
<td class="cellrowborder" valign="top" width="35.35353535353536%" headers="mcps1.2.4.1.3 "><p id="p284mcpsimp"><a href="https://www.hikunpeng.com/document/detail/en/kunpengcpfs/ecosystemEnable/Containerd/kunpengcontainerd_03_0001.html" target="_blank" rel="noopener noreferrer">Link</a></p>
</td>
</tr>
<tr id="row286mcpsimp"><td class="cellrowborder" valign="top" width="26.26262626262626%" headers="mcps1.2.4.1.1 "><p id="p288mcpsimp">Kata Containers</p>
</td>
<td class="cellrowborder" valign="top" width="38.38383838383838%" headers="mcps1.2.4.1.2 "><p id="p290mcpsimp">3.27.0</p>
</td>
<td class="cellrowborder" valign="top" width="35.35353535353536%" headers="mcps1.2.4.1.3 "><p id="p292mcpsimp"><a href="https://github.com/kata-containers/kata-containers/releases/tag/3.27.0" target="_blank" rel="noopener noreferrer">Link</a></p>
</td>
</tr>
<tr id="row294mcpsimp"><td class="cellrowborder" valign="top" width="26.26262626262626%" headers="mcps1.2.4.1.1 "><p id="p296mcpsimp">Nydus</p>
</td>
<td class="cellrowborder" valign="top" width="38.38383838383838%" headers="mcps1.2.4.1.2 "><p id="p298mcpsimp">2.4.0</p>
</td>
<td class="cellrowborder" valign="top" width="35.35353535353536%" headers="mcps1.2.4.1.3 "><p id="p300mcpsimp"><a href="https://github.com/dragonflyoss/nydus/releases/tag/v2.4.0" target="_blank" rel="noopener noreferrer">Link</a></p>
</td>
</tr>
<tr id="row302mcpsimp"><td class="cellrowborder" valign="top" width="26.26262626262626%" headers="mcps1.2.4.1.1 "><p id="p304mcpsimp">Nydus snapshotter</p>
</td>
<td class="cellrowborder" valign="top" width="38.38383838383838%" headers="mcps1.2.4.1.2 "><p id="p306mcpsimp">0.15.12</p>
</td>
<td class="cellrowborder" valign="top" width="35.35353535353536%" headers="mcps1.2.4.1.3 "><p id="p308mcpsimp"><a href="https://github.com/containerd/nydus-snapshotter/releases/tag/v0.15.12" target="_blank" rel="noopener noreferrer">Link</a></p>
</td>
</tr>
<tr id="row6598192163914"><td class="cellrowborder" valign="top" width="26.26262626262626%" headers="mcps1.2.4.1.1 "><p id="p05981928393">nerdctl</p>
</td>
<td class="cellrowborder" valign="top" width="38.38383838383838%" headers="mcps1.2.4.1.2 "><p id="p8599112153918">1.7.5</p>
</td>
<td class="cellrowborder" valign="top" width="35.35353535353536%" headers="mcps1.2.4.1.3 "><p id="p1859919203919"><a href="https://github.com/containerd/nerdctl/releases/tag/v1.7.5" target="_blank" rel="noopener noreferrer">Link</a></p>
</td>
</tr>
</tbody>
</table>

### Installing and Configuring Kata

This section describes how to install and enable Kata. Before the installation and deployment, ensure that a Kubernetes cluster and containerd have been deployed and configured, and the Kata installation package described in [Environment Requirements](#environment-requirements) has been downloaded.

1. Create a temporary directory.

    ```shell
    mkdir ~/kata-temp
    ```

2. Decompress the Kata installation package to the temporary directory.

    ```shell
    tar -xvf kata-static-3.27.0-arm64.tar.zst -C ~/kata-temp
    ```

3. View the decompressed content and copy it to the target path.

    ```shell
    ls -R ~/kata-temp | head -n 20 
    cp -ra ~/kata-temp/opt/kata /opt/
    ```

4. Set the permission to ensure that the `root` user has the execute permission.

    ```shell
    chmod -R +x /opt/kata/bin/
    ```

5. Create symbolic links to the system paths.

    ```shell
    ln -sf /opt/kata/bin/kata-runtime /usr/local/bin/kata-runtime 
    ln -sf /opt/kata/bin/containerd-shim-kata-v2 /usr/local/bin/containerd-shim-kata-v2
    ```

6. Check whether the installation is successful and delete the temporary directory.

    ```shell
    kata-runtime --version
    rm -rf ~/kata-temp
    ```

7. To configure containerd to access Kata, run the following command to modify the `/etc/containerd/config.toml` file and register the Kata runtime with the CRI plugin:

    ```toml
    sed -i '/\[plugins.\"io.containerd.grpc.v1.cri\".containerd.runtimes\]/a\        [plugins.\"io.containerd.grpc.v1.cri\".containerd.runtimes.kata]\n          runtime_type = \"io.containerd.kata.v2\"\n           [plugins.\"io.containerd.grpc.v1.cri\".containerd.runtimes.kata.options]\n             ConfigPath = \"/opt/kata/share/defaults/kata-containers/configuration-clh.toml\"' /etc/containerd/config.toml
    ```

8. Restart the containerd service for the configuration to take effect.

    ```shell
    systemctl restart containerd
    ```

    >**NOTE:**
    >You are advised to back up the `/etc/containerd/config.toml` file before the modification to ensure that the file can be restored if any problem occurs.

## (Optional) Nydus Image Acceleration

Nydus is a container image acceleration solution that significantly improves the image pull and startup speed of Kata containers. Nydus is an optional component. If Nydus is not installed, Kata can still be used properly. If you need to perform the following operations, download the Nydus installation package of the corresponding version as described in [Environment Requirements](#environment-requirements).

### Installing and Configuring Nydus

This section describes how to install the Nydus component and configure related services.

1. Install the core component RPM package.

    ```shell
    rpm -ivh nydus-static-2.4.0-linux-arm64.rpm
    ```

2. Decompress Snapshotter and copy it to the related directory.

    ```shell
    tar -zxvf nydus-snapshotter-v0.15.12-linux-arm64.tar.gz 
    cp bin/* /usr/local/bin/
    ```

3. Check whether Nydus is successfully installed.

    ```shell
    nydusctl -V
    ```

4. Create the configuration file `/etc/nydus/config.toml` and configure the file as follows:

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

5. Create the configuration file `/etc/nydus/nydusd-config.json` and configure the file as follows:

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

### Creating the Nydus Systemd Service

1. Create a service configuration file.

    ```shell
    vim /etc/systemd/system/containerd-nydus-grpc.service
    ```

2. Add the following content and save the file:

    ```txt
    [Unit] 
    Description=Nydus containerd snapshotter 
    After=network.target 
     
    [Service] 
    # Ensure that the socket storage directory exists.
    ExecStartPre=/bin/mkdir -p /run/containerd-nydus 
    # Start the gRPC service.
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

3. Start the service and enable it to automatically start upon system startup.

    ```shell
    systemctl daemon-reload 
    systemctl enable --now containerd-nydus-grpc.service
    ```

4. Check whether the .sock file is successfully generated.

    ```shell
    ls -l /run/containerd-nydus/containerd-nydus-grpc.sock
    ```

### Configuring Nydus to Access containerd

1. Modify and add the following content to the `/etc/containerd/config.toml` file.

    ```toml
        [plugins."io.containerd.grpc.v1.cri".containerd] 
          snapshotter = "nydus" 
          disable_snapshot_annotations = false 
          discard_unpacked_layers = false  # Ensure that the unpacked layers are not discarded abnormally.
          default_runtime_name = "runc" 
     
    [proxy_plugins] 
      [proxy_plugins.nydus] 
        type = "snapshot"  # This is the key modification. According to the official document, the value must be snapshot.
        address = "/run/containerd-nydus/containerd-nydus-grpc.sock"
    ```

2. Restart containerd for the modification to take effect.

    ```shell
    systemctl restart containerd
    ```

### Configuring Nydus to Kata

Modify the following parameters in the Kata configuration file `/opt/kata/share/defaults/kata-containers/configuration-clh.toml`:

    ```toml
    shared_fs = "virtio-fs-nydus" 
    virtio_fs_daemon = "/usr/bin/nydusd" 
    valid_virtio_fs_daemon_paths = ["/usr/bin/nydusd"] 
    virtio_fs_extra_args = []
    ```

## Kata Usage Verification

This section describes how to verify the use of Kata in a Kubernetes cluster with containerd.

### Preparing an Image

1. Download the `sandbox-templates` image and import it to containerd.

    ```shell
    docker pull docker.io/docker/sandbox-templates:claude-code 
    docker save -o sandbox-template-claude-code.tar docker.io/docker/sandbox-templates:claude-code 
    ctr -n k8s.io images import sandbox-template-claude-code.tar
    ```

2. Log in to the remote repository using `nerdctl`.

    ```shell
    nerdctl login -u admin -p passw0rd sealos.hub:5000 --insecure-registry
    ```

3. Convert the `sandbox-templates` image.

    ```shell
    nerdctl -n k8s.io image convert --nydus --oci docker/sandbox-templates:claude-code sealos.hub:5000/sandbox-templates:claude-code-nydus
    ```

4. Push the image to the sealos repository.

    ```shell
    ctr -n k8s.io images push --plain-http --user admin:passw0rd sealos.hub:5000/sandbox-templates:claude-code-nydus
    nerdctl -n k8s.io image push sealos.hub:5000/sandbox-templates:claude-code-nydus
    ```

5. After the image is pushed, delete the local image.

    ```shell
    crictl rmi sealos.hub:5000/sandbox-templates:claude-code-nydus
    ```

6. Check whether the repository contains the corresponding image.

    ```shell
    curl -u admin:passw0rd http://sealos.hub:5000/v2/_catalog
    ```

    >**NOTE:**
    >1. The image can be any image. The image in this document is used only to verify whether Kata and Nydus can be used properly. It is for reference only.
    >2. If the Kubernetes cluster is installed using sealos, you can push the image to the local sealos repository by referring to this document. Modify the image repository based on the actual environment requirements.

### Enabling Kata in Kubernetes

1. Create a RuntimeClass resource in Kubernetes.

    ```shell
    cat <<EOF | kubectl apply -f -  
    apiVersion: node.k8s.io/v1  
    kind: RuntimeClass  
    metadata:  
      name: kata-runtime 
    handler: kata 
    EOF
    ```

    >**NOTE:**
    >The `handler` name must be the suffix of `[plugins...runtimes.kata]` in the `/etc/containerd/config.toml` configuration file. In this document, the example is `kata`.

2. Check whether the creation is successful.

    ```shell
    # kubectl get runtimeclass 

     NAME           HANDLER   AGE
     kata-runtime   kata      16m
    ```

3. Add `runtimeClassName: kata-runtime` to the YAML file of the pod to connect the pod to Kata. The following is an example of the deployment file:

    ```yaml
    apiVersion: v1 
    kind: Pod 
    metadata: 
      name: claude-code-sandbox 
    spec: 
      # 1. Specify the Kata runtime.
      runtimeClassName: kata-runtime 
     
      # 2. Forcibly specify that the pod is deployed on the master node.
      # Note: Run the kubectl get nodes command to check that the node name is master.
      nodeName: master 
     
      # 3. Key: Tolerate the taint of the master node.
      # By default, sealos adds a NoSchedule taint to the master node. This configuration must be added so that the pod can be started.
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

4. Start the pod and check whether Kata is successfully connected.

    ```shell
    ps aux | grep cloud-hypervisor
    ```

### (Optional) Verifying the Enablement and Startup of Kata + Nydus

The following verification depends on the Nydus image acceleration component. Complete all configurations in [(Optional) Nydus Image Acceleration](#optional-nydus-image-acceleration) first.

1. Configure `nydus-sandbox.yaml`.

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

2. Configure `nydus-container.yaml`.

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

3. Test the deployment.

    ```shell
    crictl run -r kata  nydus-container.yaml nydus-sandbox.yaml
    ```

## Optimization for Concurrent Creation of Thousands of Sandboxes

To concurrently create 1,000 Kata sandboxes on a single node, you need to optimize the configuration.

### Configuring the Maximum Number of Pods

1. Modify the `/var/lib/kubelet/config.yaml` file by increasing the value of `maxPods` to a value greater than 1,000.

    ```yaml
    maxOpenFiles: 1000000
    maxPods: 2000
    memoryManagerPolicy: None
    ```

2. Restart the kubelet service for the configuration to take effect.

    ```shell
    systemctl daemon-reload
    systemctl restart kubelet
    ```

### Modifying Kernel Parameter Restrictions

Each pod of Kata Containers is a cloud-hypervisor process, which consumes a large number of system handles, ARP cache tables, and PIDs of the host machine. Therefore, the Linux kernel restrictions need to be loosened.

1. Add the following configurations to the `/etc/sysctl.conf` file:

    ```conf
    # 1. Increase the inotify restrictions. (This is very important because each VM needs to monitor many file descriptors.)
    fs.inotify.max_user_watches = 1048576
    fs.inotify.max_user_instances = 81920
    
    # 2. Expand the ARP cache table. (This prevents network disconnection and packet loss when there are a large number of containers.)
    net.ipv4.neigh.default.gc_thresh1 = 4096
    net.ipv4.neigh.default.gc_thresh2 = 8192
    net.ipv4.neigh.default.gc_thresh3 = 16384
    
    # 3. Increase the upper limit on the number of system file descriptors and PIDs.
    fs.file-max = 2097152
    kernel.pid_max = 4194304
    
    # 4. Optimize Conntrack to prevent the number of connections from being used up in high-concurrency scenarios.
    net.netfilter.nf_conntrack_max = 2097152
    ```

2. Run the following command to make the configuration take effect:

    ```shell
    sysctl -p
    ```

### Deployment Example for Starting Thousands of Kata Containers

After the preceding Kata runtime configurations, RuntimeClass creation, and system parameter tuning are complete, you can use `Deployment` to start Kata containers in batches on a single node. The following example creates 1,000 Kata pods on the `master` node to verify the capability of starting thousands of Kata sandboxes concurrently.

>**NOTE:**
>
>1. The `nodeName: master`, image path, CPU, and memory requests in the example are for reference only. Adjust them based on the actual node name, image repository, and host machine resource specifications.
>2. This example uses `hostNetwork: true` to enable pods to use the host network, reducing the impact of CNI plugins, pod IP address allocation, and ARP table scale on the verification for starting thousands of pods. When the host network is used, pods do not have independent networks, and the listening ports are shared with the host machine ports. Therefore, do not start services with the same fixed listening port in the container.
>3. If the node has a master or control plane taint, the `tolerations` configuration must be retained. Otherwise, the pods cannot be scheduled due to the taint.
>4. When thousands of Kata pods are created, a large number of `cloud-hypervisor` processes are started. You are advised to gradually increase the number of pods to 100, 300, 600, and 1,000, and observe the CPU, memory, PID, file descriptor, and system network status.

1. Create a namespace for deployment.

    ```shell
    kubectl create namespace kata-scale
    ```

2. Create the configuration file `kata-sandbox-1000.yaml` for thousands of Kata containers.

    ```yaml
    apiVersion: apps/v1
    kind: Deployment
    metadata:
      name: kata-sandbox-1000
      namespace: kata-scale
    spec:
      # You are advised to set the number of sandboxes to 100 for the first deployment and gradually increase the number to 1,000 after confirming that the resources are stable.
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

    To verify the Kata+Nydus image acceleration, replace `image` with the converted Nydus image in the previous section. For example:

    ```shell
    image: sealos.hub:5000/sandbox-templates:claude-code-nydus
    ```

3. Deploy sandboxes with its quantity increased in batches.

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

4. Check the pod running status.

    ```shell
    kubectl -n kata-scale get pods -o wide
    kubectl -n kata-scale get pods --field-selector=status.phase=Running --no-headers | wc -l
    ```

    When the number of pods in the `Running` state reaches 1,000, thousands of Kata containers have been started at the Kubernetes layer.

5. Check the number of Kata VM processes.

    ```shell
    ps -ef | grep cloud-hypervisor | grep -v grep | wc -l
    ```

    The number should be close to the number of pods in the `Running` state. If the number is obviously small, run the following commands to check the failure cause:

    ```shell
    kubectl -n kata-scale get events --sort-by=.lastTimestamp
    kubectl -n kata-scale describe pod <pod-name>
    journalctl -u kubelet -f
    journalctl -u containerd -f
    ```

6. After the verification is complete, clear the test resources.

    ```shell
    kubectl delete namespace kata-scale
    ```
