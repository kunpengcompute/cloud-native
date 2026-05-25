# Project Introduction<a name="EN-US_TOPIC_0000002440791696"></a>

This is a collection of Kunpeng cloud-native projects, including multiple cloud-native components optimized for Kunpeng processors.

# Version Description<a name="EN-US_TOPIC_0000002441456462"></a>

**Table 1** Version description

<a name="table1911819583242"></a>
<table><thead align="left"><tr id="row81181558182420"><th class="cellrowborder" valign="top" width="26.640000000000004%" id="mcps1.2.4.1.1"><p id="p613134819116"><a name="p613134819116"></a><a name="p613134819116"></a>Project Name</p>
</th>
<th class="cellrowborder" valign="top" width="19.61%" id="mcps1.2.4.1.2"><p id="p2011865842412"><a name="p2011865842412"></a><a name="p2011865842412"></a>Version</p>
</th>
<th class="cellrowborder" valign="top" width="53.75%" id="mcps1.2.4.1.3"><p id="p611895822414"><a name="p611895822414"></a><a name="p611895822414"></a>Description</p>
</th>
</tr>
</thead>
<tbody><tr id="row1011813587244"><td class="cellrowborder" valign="top" width="26.640000000000004%" headers="mcps1.2.4.1.1 "><p id="p2131348121110"><a name="p2131348121110"></a><a name="p2131348121110"></a>Kubernetes MPAM Controller</p>
</td>
<td class="cellrowborder" valign="top" width="19.61%" headers="mcps1.2.4.1.2 "><p id="p19118125872412"><a name="p19118125872412"></a><a name="p19118125872412"></a>0.1.0</p>
</td>
<td class="cellrowborder" valign="top" width="53.75%" headers="mcps1.2.4.1.3 "><a name="ul14988152791610"></a><a name="ul14988152791610"></a><ul id="ul14988152791610"><li>Pods can be managed by MPAM in Kubernetes. </li><li>In hybrid deployment of online and offline services, offline services can be dynamically restricted, ensuring the performance of online services.</li></ul>
</td>
</tr>
<tr id="row12424143501220"><td class="cellrowborder" valign="top" width="26.640000000000004%" headers="mcps1.2.4.1.1 "><p id="p16424173581216"><a name="p16424173581216"></a><a name="p16424173581216"></a>Kunpeng Topology Affinity Plugin (TAP)</p>
</td>
<td class="cellrowborder" valign="top" width="19.61%" headers="mcps1.2.4.1.2 "><p id="p12424133531215"><a name="p12424133531215"></a><a name="p12424133531215"></a>v0.3 (to be released)</p>
</td>
<td class="cellrowborder" valign="top" width="53.75%" headers="mcps1.2.4.1.3 "><p id="p142553511210"><a name="p142553511210"></a><a name="p142553511210"></a>-</p>
</td>
</tr>
</tbody>
</table>

# Environment Deployment<a name="EN-US_TOPIC_0000002441616318"></a>



## Kunpeng TAP<a name="EN-US_TOPIC_0000002441675240"></a>

**Hardware Requirements<a name="section8119658132416"></a>**

[**Table 1**](#table1911819583242) lists the hardware requirement.

**Table 1** Hardware requirement<a id="table1911819583242"></a>

<table><thead align="left"><tr id="row81181558182420"><th class="cellrowborder" valign="top" width="28.000000000000004%" id="mcps1.2.3.1.1"><p id="p2011865842412"><a name="p2011865842412"></a><a name="p2011865842412"></a>Item</p>
</th>
<th class="cellrowborder" valign="top" width="72%" id="mcps1.2.3.1.2"><p id="p611895822414"><a name="p611895822414"></a><a name="p611895822414"></a>Description</p>
</th>
</tr>
</thead>
<tbody><tr id="row1011813587244"><td class="cellrowborder" valign="top" width="28.000000000000004%" headers="mcps1.2.3.1.1 "><p id="p19118125872412"><a name="p19118125872412"></a><a name="p19118125872412"></a>Processor</p>
</td>
<td class="cellrowborder" valign="top" width="72%" headers="mcps1.2.3.1.2 "><p id="p211865813245"><a name="p211865813245"></a><a name="p211865813245"></a><span id="ph2083215281714"><a name="ph2083215281714"></a><a name="ph2083215281714"></a>Kunpeng 920 series</span></p>
</td>
</tr>
</tbody>
</table>

**Software Requirements<a name="section2021616722519"></a>**

[**Table 2**](#table18216177162517) lists the software requirements.

**Table 2** Software requirements<a id="table18216177162517"></a>

<table><thead align="left"><tr id="row102141372254"><th class="cellrowborder" valign="top" width="27.92%" id="mcps1.2.4.1.1"><p id="p152141075252"><a name="p152141075252"></a><a name="p152141075252"></a>Item</p>
</th>
<th class="cellrowborder" valign="top" width="33.52%" id="mcps1.2.4.1.2"><p id="p62140742515"><a name="p62140742515"></a><a name="p62140742515"></a>Version</p>
</th>
<th class="cellrowborder" valign="top" width="38.56%" id="mcps1.2.4.1.3"><p id="p1521411711250"><a name="p1521411711250"></a><a name="p1521411711250"></a>How to Obtain</p>
</th>
</tr>
</thead>
<tbody><tr id="row15611759195712"><td class="cellrowborder" rowspan="2" valign="top" width="27.92%" headers="mcps1.2.4.1.1 "><p id="p992185413314"><a name="p992185413314"></a><a name="p992185413314"></a>OS</p>
</td>
<td class="cellrowborder" valign="top" width="33.52%" headers="mcps1.2.4.1.2 "><p id="p49919297200"><a name="p49919297200"></a><a name="p49919297200"></a><span>openEuler 20.03 LTS SP3</span></p>
</td>
<td class="cellrowborder" valign="top" width="38.56%" headers="mcps1.2.4.1.3 "><p id="p3969416751"><a name="p3969416751"></a><a name="p3969416751"></a><a href="https://repo.openeuler.org/openEuler-20.03-LTS-SP3/ISO/aarch64/" target="_blank" rel="noopener noreferrer">Link</a></p>
</td>
</tr>
<tr id="row97069209545"><td class="cellrowborder" valign="top" headers="mcps1.2.4.1.1 "><p id="p17071120105415"><a name="p17071120105415"></a><a name="p17071120105415"></a>openEuler 22.03 LTS SP4</p>
</td>
<td class="cellrowborder" valign="top" headers="mcps1.2.4.1.2 "><p id="p3633721452"><a name="p3633721452"></a><a name="p3633721452"></a><a href="https://repo.openeuler.org/openEuler-22.03-LTS-SP4/ISO/aarch64/" target="_blank" rel="noopener noreferrer">Link</a></p>
</td>
</tr>
<tr id="row17307111824410"><td class="cellrowborder" valign="top" width="27.92%" headers="mcps1.2.4.1.1 "><p id="p12134123815817"><a name="p12134123815817"></a><a name="p12134123815817"></a>Kunpeng TAP source code</p>
</td>
<td class="cellrowborder" valign="top" width="33.52%" headers="mcps1.2.4.1.2 "><p id="p1634252516559"><a name="p1634252516559"></a><a name="p1634252516559"></a>release-0.2</p>
</td>
<td class="cellrowborder" valign="top" width="38.56%" headers="mcps1.2.4.1.3 "><p id="p1716882610513"><a name="p1716882610513"></a><a name="p1716882610513"></a><a href="https://gitee.com/kunpeng_compute/topo-affinity-plugin.git" target="_blank" rel="noopener noreferrer">Link</a></p>
</td>
</tr>
<tr id="row1821519714251"><td class="cellrowborder" valign="top" width="27.92%" headers="mcps1.2.4.1.1 "><p id="p421412715250"><a name="p421412715250"></a><a name="p421412715250"></a>Golang</p>
</td>
<td class="cellrowborder" valign="top" width="33.52%" headers="mcps1.2.4.1.2 "><p id="p821513752520"><a name="p821513752520"></a><a name="p821513752520"></a>1.23+</p>
</td>
<td class="cellrowborder" valign="top" width="38.56%" headers="mcps1.2.4.1.3 "><p id="p1021518792513"><a name="p1021518792513"></a><a name="p1021518792513"></a>Install it using a binary package.</p>
</td>
</tr>
<tr id="row121801256182211"><td class="cellrowborder" valign="top" width="27.92%" headers="mcps1.2.4.1.1 "><p id="p748691616198"><a name="p748691616198"></a><a name="p748691616198"></a>Make</p>
</td>
<td class="cellrowborder" valign="top" width="33.52%" headers="mcps1.2.4.1.2 "><p id="p9486111641912"><a name="p9486111641912"></a><a name="p9486111641912"></a>-</p>
</td>
<td class="cellrowborder" valign="top" width="38.56%" headers="mcps1.2.4.1.3 "><p id="p4927133881917"><a name="p4927133881917"></a><a name="p4927133881917"></a>Install it using a Yum repository.</p>
</td>
</tr>
</tbody>
</table>

**Plugin Compilation<a name="section14380431101311"></a>**

Compile the Kunpeng TAP source code and generate a plugin executable file.

1.  Obtain the Kunpeng TAP source code of the latest version `release-0.2` on the `Tags` tab.

    ```
    git clone --branch release-0.2 https://gitee.com/kunpeng_compute/topo-affinity-plugin.git
    ```

2.  Go to the `topo-affinity-plugin` directory and run the script for building the plugin.

    ```
    cd /path/to/topo-affinity-plugin
    go mod tidy
    make build
    ```

    In the preceding command, `/path/to/topo-affinity-plugin` indicates the path to the plugin source code. Replace it with the actual path.

    After the build is complete, ensure that the `kunpeng-tap` binary file is generated in the `bin` directory.

## Kubernetes MPAM Controller<a name="EN-US_TOPIC_0000002475075173"></a>

**Environment Dependencies<a name="section94010535258"></a>**

-   Go 1.23.6 or later
-   Linux environment (openEuler 22.03 is recommended)
-   Hardware platform supporting MPAM (Kunpeng processors are recommended)

**Docker Image Compilation<a name="section34166302913"></a>**

```
make mpam-docker
```

If the Kubernetes cluster uses containerd as the container runtime, you need to manually import the image to the containerd image repository.

```
docker save k8s-mpam-controller:0.1.0 -o k8s-mpam-controller.tar
ctr -n k8s.io images import k8s-mpam-controller.tar
```

# Quick Start<a name="EN-US_TOPIC_0000002474856269"></a>



## Kunpeng TAP<a name="EN-US_TOPIC_0000002475098685"></a>

**Plugin Deployment<a name="section03521240101415"></a>**

Deploy Kunpeng TAP on the target compute node and verify the running status of the plugin.

Kunpeng TAP depends on the Kubernetes cluster. Currently, Docker that adopts Dockershim for communication and containerd are supported. Before deployment, ensure that the network configuration of the Kubernetes cluster is correct and container instances can be deployed and run properly.

1.  Import the executable file of Kunpeng TAP to the target compute node.
2.  <a name="li1795401917439"></a>Start Kunpeng TAP.

    **Method 1**: Use systemd to start Kunpeng TAP. Go to the source code directory and run the `make` commands to install and start Kunpeng TAP.

    1.  Go to the source code directory.

        ```
        cd /path/to/topology-affinity-plugin
        ```

        In the preceding command, replace `/path/to/topology-affinity-plugin` with the actual path to the Kunpeng TAP source code.

    2.  Install the plugin. By default, the plugin is started in Docker.

        ```
        make install-service
        ```

    3.  To specify Docker as the runtime, use the following installation command:

        ```
        make install-service-docker
        ```

        If you need to modify startup parameters, modify the parameters under `ExecStart=` in the `hack/kunpeng-tap.service.docker` file in the source code directory.

        ```
        [Unit]
        Description=Kunpeng Topology-Affinity Plugin Service
        After=network.target
        
        [Service]
        ExecStart=/usr/local/bin/kunpeng-tap --runtime-proxy-endpoint="/var/run/kunpeng/tap-runtime-proxy.sock" \
            --container-runtime-service-endpoint="/var/run/docker.sock" --container-runtime-mode="Docker" \
            --resource-policy="numa-aware"
        Restart=always
        RestartSec=5
        
        [Install]
        WantedBy=multi-user.target
        ```

        >**NOTE:**
        >To specify containerd as the runtime, use the following installation command. You can modify the parameters in the `hack/kunpeng-tap.service.containerd` file in the source code directory.
        >```
        >make install-service-containerd
        >```

    4.  After the installation is complete, run the following command to start the plugin and automatically check the service status after the plugin is started:

        ```
        make start-service
        ```

    5.  Check log information.

        ```
        journalctl -u kunpeng-tap
        ```

    **Method 2**: Start the plugin directly.

    -   Example startup commands in Docker:

        ```
        kunpeng-tap --runtime-proxy-endpoint="/var/run/kunpeng/tap-runtime-proxy.sock" \
            --container-runtime-service-endpoint="/var/run/docker.sock" --container-runtime-mode="Docker" \
            --resource-policy="numa-aware"
        ```

    -   Example startup commands in containerd:

        ```
        kunpeng-tap --runtime-proxy-endpoint="/var/run/kunpeng/tap-runtime-proxy.sock" \
            --container-runtime-service-endpoint="/var/run/containerd/containerd.sock" --container-runtime-mode="Containerd" \
            --resource-policy="numa-aware"
        ```

    >**NOTE:**
    >You can modify the parameters in the startup commands as required. [**Table 1**](#table105725712163) describes the parameters.

    **Table 1** Parameter description<a id="table105725712163"></a>

    <table><thead align="left"><tr id="row10573770161"><th class="cellrowborder" valign="top" width="20.5%" id="mcps1.2.5.1.1"><p id="p13352104118163"><a name="p13352104118163"></a><a name="p13352104118163"></a>Parameter Name</p>
    </th>
    <th class="cellrowborder" valign="top" width="51.93000000000001%" id="mcps1.2.5.1.2"><p id="p535284121611"><a name="p535284121611"></a><a name="p535284121611"></a>Description</p>
    </th>
    <th class="cellrowborder" valign="top" width="10.3%" id="mcps1.2.5.1.3"><p id="p136571189517"><a name="p136571189517"></a><a name="p136571189517"></a>Default Value</p>
    </th>
    <th class="cellrowborder" valign="top" width="17.270000000000003%" id="mcps1.2.5.1.4"><p id="p1460410219118"><a name="p1460410219118"></a><a name="p1460410219118"></a>Configuration Principle</p>
    </th>
    </tr>
    </thead>
    <tbody><tr id="row1357377131611"><td class="cellrowborder" valign="top" width="20.5%" headers="mcps1.2.5.1.1 "><p id="p716171411176"><a name="p716171411176"></a><a name="p716171411176"></a>container-runtime-mode</p>
    </td>
    <td class="cellrowborder" valign="top" width="51.93000000000001%" headers="mcps1.2.5.1.2 "><p id="p316181411713"><a name="p316181411713"></a><a name="p316181411713"></a>Container runtime connected to the plugin, which can be Docker or containerd.</p>
    </td>
    <td class="cellrowborder" valign="top" width="10.3%" headers="mcps1.2.5.1.3 "><p id="p1816171431710"><a name="p1816171431710"></a><a name="p1816171431710"></a>Docker</p>
    </td>
    <td class="cellrowborder" valign="top" width="17.270000000000003%" headers="mcps1.2.5.1.4 "><p id="p31521417172"><a name="p31521417172"></a><a name="p31521417172"></a>Determine the container runtime according to that used in the Kubernetes cluster.</p>
    </td>
    </tr>
    <tr id="row1757316741612"><td class="cellrowborder" valign="top" width="20.5%" headers="mcps1.2.5.1.1 "><p id="p101591411170"><a name="p101591411170"></a><a name="p101591411170"></a>resource-policy</p>
    </td>
    <td class="cellrowborder" valign="top" width="51.93000000000001%" headers="mcps1.2.5.1.2 "><p id="p01551124165916"><a name="p01551124165916"></a><a name="p01551124165916"></a>Container resource optimization policy. Currently, numa-aware and topology-aware are supported.</p>
    <a name="ul1958981910598"></a><a name="ul1958981910598"></a><ul id="ul1958981910598"><li>numa-aware supports CPU NUMA affinity for containers of the Burstable type. </li><li>topology-aware provides CPU affinity at the socket, die, and NUMA levels, and supports memory and GPU resource optimization.</li></ul>
    </td>
    <td class="cellrowborder" valign="top" width="10.3%" headers="mcps1.2.5.1.3 "><p id="p171481414179"><a name="p171481414179"></a><a name="p171481414179"></a>numa-aware</p>
    </td>
    <td class="cellrowborder" valign="top" width="17.270000000000003%" headers="mcps1.2.5.1.4 "><p id="p18761721103016"><a name="p18761721103016"></a><a name="p18761721103016"></a>Select a policy as required.</p>
    </td>
    </tr>
    <tr id="row8873408361"><td class="cellrowborder" valign="top" width="20.5%" headers="mcps1.2.5.1.1 "><p id="p188144053617"><a name="p188144053617"></a><a name="p188144053617"></a>enable-memory-topology</p>
    </td>
    <td class="cellrowborder" valign="top" width="51.93000000000001%" headers="mcps1.2.5.1.2 "><p id="p11714757113619"><a name="p11714757113619"></a><a name="p11714757113619"></a>After the topology-aware policy is enabled (by setting <span class="parmvalue" id="parmvalue76100434148"><a name="parmvalue76100434148"></a><a name="parmvalue76100434148"></a><code>--resource-policy=topology-aware</code></span>), the memory NUMA optimization function is disabled by default. To enable NUMA affinity for container memory, set <span class="parmvalue" id="parmvalue17242135471413"><a name="parmvalue17242135471413"></a><a name="parmvalue17242135471413"></a><code>--enable-memory-topology=true</code></span>.</p>
    </td>
    <td class="cellrowborder" valign="top" width="10.3%" headers="mcps1.2.5.1.3 "><p id="p38811403360"><a name="p38811403360"></a><a name="p38811403360"></a>false</p>
    </td>
    <td class="cellrowborder" valign="top" width="17.270000000000003%" headers="mcps1.2.5.1.4 "><p id="p388104019369"><a name="p388104019369"></a><a name="p388104019369"></a>This configuration is in the alpha phase.</p>
    </td>
    </tr>
    <tr id="row7164174661720"><td class="cellrowborder" valign="top" width="20.5%" headers="mcps1.2.5.1.1 "><p id="p18165114601711"><a name="p18165114601711"></a><a name="p18165114601711"></a>v</p>
    </td>
    <td class="cellrowborder" valign="top" width="51.93000000000001%" headers="mcps1.2.5.1.2 "><p id="p10165146101717"><a name="p10165146101717"></a><a name="p10165146101717"></a>Log level. The value ranges from 2 to 5.</p>
    </td>
    <td class="cellrowborder" valign="top" width="10.3%" headers="mcps1.2.5.1.3 "><p id="p18165114621718"><a name="p18165114621718"></a><a name="p18165114621718"></a>2</p>
    </td>
    <td class="cellrowborder" valign="top" width="17.270000000000003%" headers="mcps1.2.5.1.4 "><p id="p4165846131715"><a name="p4165846131715"></a><a name="p4165846131715"></a>The higher the level, the more detailed the logs.</p>
    </td>
    </tr>
    </tbody>
    </table>

3.  Configure kubelet parameters on the compute nodes.

    To enable Kunpeng TAP to successfully process requests from kubelet, add a parameter to the kubelet command line configurations.

    -   In the Docker scenario, add or modify the following parameter in the kubelet startup parameters:

        ```
        --docker-endpoint=unix:///var/run/kunpeng/tap-runtime-proxy.sock
        ```

        For example, when using kubeadm to install a cluster, you can add the parameter to `/var/lib/kubelet/kubeadm-flags.env`.

        ```
        KUBELET_KUBEADM_ARGS="--network-plugin=cni --pod-infra-container-image=k8s.gcr.io/pause:3.6 --docker-endpoint=unix:///var/run/kunpeng/tap-runtime-proxy.sock"
        ```

        After the kubelet parameter is modified, run the following commands to restart the kubelet:

        ```
        systemctl daemon-reload
        systemctl restart kubelet
        ```

    -   In the containerd scenario, modify the following kubelet startup parameters.

        ```
        --container-runtime=remote --container-runtime-endpoint=unix:///var/run/kunpeng/tap-runtime-proxy.sock
        ```

        In this case, parameters in `/var/lib/kubelet/kubeadm-flags.env` can be:

        ```
        KUBELET_KUBEADM_ARGS="--network-plugin=cni --pod-infra-container-image=k8s.gcr.io/pause:3.6 --container-runtime=remote --container-runtime-endpoint=unix:///var/run/kunpeng/tap-runtime-proxy.sock"
        ```

        After the kubelet parameters are modified, run the following commands to restart the kubelet:

        ```
        systemctl daemon-reload
        systemctl restart kubelet
        ```

**TAP Usage<a name="section13759133761517"></a>**

Kunpeng TAP allows you to specify CPU resource requirements during Pod deployment. The system automatically allocates resources based on NUMA affinity. By writing a YAML file and specifying the node selector, you can deploy a Pod on a specific node. After the plugin is deployed, you only need to specify the values of `request` and `limit` for CPU resources when deploying other Pods. The system automatically allocates resources based on NUMA affinity.

The following is an example YAML file for deploying a single-container Pod. The CPU resources requested by the Pod are 4 cores at minimum, and 8 cores at maximum. The memory is fixed to 4 GiB. `busybox` is used as the container image.

1.  Create a YAML file `example.yaml`, and write the following configuration into the file:

    ```
    apiVersion: v1
    kind: Pod
    metadata:
      name: tap-test
      annotations:
    spec:
      containers:
      - name: tap-example
        image: busybox:latest
        imagePullPolicy: IfNotPresent
        command: ["/bin/sh"]
        args: ["-c", "while true; do echo `date`; sleep 5; done"]
        resources:
          requests:
            cpu: "4"
            memory: "4Gi"
          limits:
            cpu: "8"
            memory: "4Gi"
    ```

2.  For example, to make the Pod run on the `compute01` node, add the following content to the `spec` section in the YAML file.

    >**NOTE:**
    >In a Kubernetes cluster with multiple worker nodes, a Pod can be scheduled to different NUMA nodes. To make the Pod run on a specified node, add the `nodeSelector` field to the `spec` section in the YAML file and set `kubernetes.io/hostname` to the name of the target node.

    ```
      nodeSelector:
        kubernetes.io/hostname: compute01
    ```

3.  Apply the YAML file on the management node to deploy the Pod.

    ```
    kubectl apply -f example.yaml
    ```

4.  Check whether Kunpeng TAP takes effect.
    1.  Using Docker as an example, access the `compute01` node specified by `nodeSelector` in step 2, run the `docker` command to query the `CpusetCpus` parameter of the container, and determine whether NUMA affinity has been established.
    2.  Use `docker ps` to query container tasks running on the cluster nodes. In the `NAMES` column, find the `nri-1` container specified by `spec.containers.name` in step 1.

        ```
        # docker ps | grep nri-1
        CONTAINER ID   IMAGE                  COMMAND                  CREATED       STATUS       PORTS     NAMES
        ```

    3.  Query the deployment parameter `CpusetCpus` of the target container based on its container ID. This parameter indicates the schedulable CPU range of the container.

        ```
        # docker inspect bf32de0d09fe | grep "CpusetCpus"
                    "CpusetCpus": "0-23",
        ```

        If memory binding is enabled, you can run the following command to check the corresponding node.

        ```
        # docker inspect bf32de0d09fe | grep "CpusetMems"
                    "CpusetMems": "0",
        ```

        >**NOTE:**
        >The bound NUMA nodes are not fixed on different servers, and the values of `CpusetCpus` may be different.

        In the containerd scenario, you can run the following command to check the schedulable CPU range of a container:

        ```
        # crictl inspect bf32de0d09fe | grep "cpuset_cpus"
                    "cpuset_cpus": "0-23",
        ```

        If NUMA node affinity configuration fails, the `cpuset_cpus` output may fail to be queried.

    4.  Query the NUMA information of the system and compare it with the schedulable CPU core range of the container. The NUMA node matching the schedulable CPU core range is the affinity node of the container.

        ```
        # lscpu
        ...
        NUMA node0 CPU(s):               0-23
        NUMA node1 CPU(s):               24-47
        NUMA node2 CPU(s):               48-71
        NUMA node3 CPU(s):               72-95
        ...
        ```

        `node0` indicates the NUMA node whose index is `0`, and `0-23` indicates the CPU cores in this NUMA node.

5.  Run the following command to check the NUMA distribution of GPUs in the system. `0200` indicates the NIC device number.

    ```
    lspci -vvv -d :0200 | grep NUMA
    ```

    The command output is similar to the following:

    ```
    NUMA node: 0
    NUMA node: 0
    NUMA node: 0
    NUMA node: 0
    NUMA node: 0
    NUMA node: 2
    NUMA node: 2
    NUMA node: 2
    NUMA node: 2
    NUMA node: 2
    ```

## Kubernetes MPAM Controller<a name="EN-US_TOPIC_0000002441858770"></a>

**Plugin Deployment<a name="section24721531114518"></a>**

1.  Before deploying the plugin, ensure that the MPAM resctrl file system has been mounted. You can run the following command on the **worker node** to mount the file system:

    ```
    mount -t resctrl resctrl /sys/fs/resctrl
    ```

2.  Run the following commands on the **master node** to deploy the plugin:

    ```
    cd config/k8s-mpam-controller-config/samples
    kubectl apply -f k8s-mpam-controller.yaml
    ```

3.  Check whether the Pod corresponding to the MPAM plugin is running properly.

    ```
    kubectl get pods
    ```

    The following information may be displayed if the Pod is running properly:

    ```
    NAME                                    READY   STATUS    RESTARTS   AGE
    mpam-controller-daemonset-agent-bj2gv   1/1     Running   0          143m
    ```

**Creating an MPAM Resource Group<a name="section18443344816"></a>**

To limit resources for a Pod, you need to create an MPAM resource group.

1.  Go to the `samples` directory and modify the configuration file (in .yaml format) of the MPAM resource group. The following uses `example-config.yaml` as an example.

    In the `example-config.yaml` file, a node resource group may have any of the three configurations, as described in [**Table 1**](#table8171211407). You can use ConfigMaps to create a configuration for a node or a group of nodes. After the configuration is created, the MPAM plugin manages the ConfigMaps in the Kubernetes cluster and automatically applies the configuration to the corresponding nodes after a ConfigMap is added or updated.

    **Table 1** Configuration types<a id="table8171211407"></a>

    <table><thead align="left"><tr id="row41712012020"><th class="cellrowborder" valign="top" width="17.09%" id="mcps1.2.4.1.1"><p id="p41715112015"><a name="p41715112015"></a><a name="p41715112015"></a>Configuration Type</p>
    </th>
    <th class="cellrowborder" valign="top" width="32.910000000000004%" id="mcps1.2.4.1.2"><p id="p201712118017"><a name="p201712118017"></a><a name="p201712118017"></a>Configuration Name</p>
    </th>
    <th class="cellrowborder" valign="top" width="50%" id="mcps1.2.4.1.3"><p id="p4171611105"><a name="p4171611105"></a><a name="p4171611105"></a>Description</p>
    </th>
    </tr>
    </thead>
    <tbody><tr id="row191711311407"><td class="cellrowborder" valign="top" width="17.09%" headers="mcps1.2.4.1.1 "><p id="p1917117117013"><a name="p1917117117013"></a><a name="p1917117117013"></a>Node configuration</p>
    </td>
    <td class="cellrowborder" valign="top" width="32.910000000000004%" headers="mcps1.2.4.1.2 "><p id="p1417111205"><a name="p1417111205"></a><a name="p1417111205"></a>rc-config.node.{NODE_NAME}</p>
    </td>
    <td class="cellrowborder" valign="top" width="50%" headers="mcps1.2.4.1.3 "><p id="p3171101607"><a name="p3171101607"></a><a name="p3171101607"></a>Provides the configuration of the node named *Node_NAME*.</p>
    </td>
    </tr>
    <tr id="row151718113017"><td class="cellrowborder" valign="top" width="17.09%" headers="mcps1.2.4.1.1 "><p id="p9171411005"><a name="p9171411005"></a><a name="p9171411005"></a>Node group configuration</p>
    </td>
    <td class="cellrowborder" valign="top" width="32.910000000000004%" headers="mcps1.2.4.1.2 "><p id="p1217151900"><a name="p1217151900"></a><a name="p1217151900"></a>rc-config.group.{GROUP_NAME}</p>
    </td>
    <td class="cellrowborder" valign="top" width="50%" headers="mcps1.2.4.1.3 "><p id="p141711018020"><a name="p141711018020"></a><a name="p141711018020"></a>You can use the <code>ngroup</code> label to add a node to a node group. For example, if a node contains the <code>ngroup=grp1</code> label, the node belongs to the node group <code>grp1</code>. If the <code>ConfigMap rc-config.node.{NODE_NAME}</code> for a node does not exist but the node belongs to the <code>GROUP_NAME</code> node group, the ConfigMap named <span class="parmname" id="parmname51717119012"><a name="parmname51717119012"></a><a name="parmname51717119012"></a><code>rc-config.group.{GROUP_NAME}</code></span> is applied to this node.</p>
    </td>
    </tr>
    <tr id="row191719118017"><td class="cellrowborder" valign="top" width="17.09%" headers="mcps1.2.4.1.1 "><p id="p2171131001"><a name="p2171131001"></a><a name="p2171131001"></a>Default configuration</p>
    </td>
    <td class="cellrowborder" valign="top" width="32.910000000000004%" headers="mcps1.2.4.1.2 "><p id="p51721219014"><a name="p51721219014"></a><a name="p51721219014"></a>rc-config.default</p>
    </td>
    <td class="cellrowborder" valign="top" width="50%" headers="mcps1.2.4.1.3 "><p id="p017216115010"><a name="p017216115010"></a><a name="p017216115010"></a>If a node does not belong to any node group and the corresponding ConfigMap does not exist, the ConfigMap named <span class="parmname" id="parmname1217291703"><a name="parmname1217291703"></a><a name="parmname1217291703"></a><code>rc-config.default</code></span> is applied to this node.</p>
    </td>
    </tr>
    </tbody>
    </table>

    1.  Open the file.

        ```
        cd samples
        vi example-config.yaml
        ```

    2.  Press `i` to enter the insert mode. Set the `name` field to the actual configuration name in [**Table 1**](#table8171211407) and add the resource group information to the `mpam` field.

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

        >**NOTE:**
        >-   A maximum of 32 resource groups can be configured. (The root group occupies one resource group by default, and a maximum of 31 new resource groups can be created under the root group.) Each schemata must comply with the syntax rules.
        >-   If an item is not configured in a resource group or a configuration item does not meet the syntax rules, the resource group uses the default configuration of the configuration item. The default L3 cache configuration is `"L3:0=fffffff;1=fffffff;2=fffffff;3=fffffff"` and the default bandwidth configuration is `"MB:0=100;1=100;2=100;3=100"`.

    3.  Press `Esc` to exit the insert mode. Type `:wq!` and press `Enter` to save the file and exit.

2.  In the `samples` directory, use the `example-config.yaml` file to create a ConfigMap.

    ```
    kubectl apply -f example-config.yaml
    ```

3.  On the node, go to the `/sys/fs/resctrl` directory and check whether a resource group has been created and whether the resource group configuration matches the `example-config.yaml` file.

    ```
    cd /sys/fs/resctrl
    ls
    ```

    >**NOTE:**
    >For example, you can run the following command to view the configuration of the resource group `group1`:
    >```
    >cat group1/schemata
    >```

**Creating a Pod and Adding It to a Resource Group<a name="section582623085216"></a>**

To add a Pod to a resource group, specify the resource group when creating the Pod.

1.  Modify the Pod configuration file (in .yaml format). The following uses `example-pod.yaml` as an example.
    1.  Go to the `samples` directory and open the `example-pod.yaml` file.

        ```
        cd samples
        vi example-pod.yaml
        ```

    2.  Press `i` to enter the insert mode and add the following content to the file:

        ```
        labels:
            rcgroup: group2
        ```

        ```
        nodeSelector:
            MPAM: enabled
        ```

        >**NOTE:**
        >-   In the `labels` field, set the `rcgroup` field to specify the associated resource group. For example, add the Pod to `group2`.
        >-   Add `MPAM: enabled` to the `nodeSelector` field so that the scheduler can schedule the Pod to a node that supports the MPAM feature.

        The updated `example-pod.yaml` file has the following content:

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

    3.  Press `Esc` to exit the insert mode. Type `:wq!` and press `Enter` to save the file and exit.

2.  Create a Pod.

    ```
    kubectl apply -f example-pod.yaml
    ```

3.  On the node, go to the `/sys/fs/resctrl` directory and then the owning resource group (for example `group1`) of the Pod. You can view the configuration and monitoring data in the resource group and the PIDs of the restricted applications in the current resource group.

    ```
    cd /sys/fs/resctrl/group1
    ```

    -   Run the following command to view the configuration of the resource group:

        ```
        cat schemata
        ```

    -   Run the following command to view the PIDs of the resource group:

        ```
        cat tasks
        ```

    -   Run the following command to view the monitoring data of the resource group:

        ```
        grep . mon_data/*
        ```

**Using Dynamic MPAM Isolation<a name="section6968124245311"></a>**

You can use the dynamic MPAM isolation function to adjust the resource usage of some offline services.

**(Optional) Configuring Dynamic MPAM Isolation Parameters<a name="section182321957145418"></a>**

The plugin provides default configurations. Dynamic MPAM isolation can be used without configuring ConfigMaps. Dynamic MPAM isolation parameters are configured in a JSON file. For details about the parameters, see [**Table 2**](#table116484132237). To manually change the configurations, refer to the following content.

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

**Table 2** Dynamic MPAM isolation parameters<a id="table116484132237"></a>

<table><thead align="left"><tr id="row46491113142313"><th class="cellrowborder" valign="top" width="19.39%" id="mcps1.2.3.1.1"><p id="p864921318233"><a name="p864921318233"></a><a name="p864921318233"></a>Parameter</p>
</th>
<th class="cellrowborder" valign="top" width="80.61%" id="mcps1.2.3.1.2"><p id="p9649151310232"><a name="p9649151310232"></a><a name="p9649151310232"></a>Description</p>
</th>
</tr>
</thead>
<tbody><tr id="row96498138233"><td class="cellrowborder" valign="top" width="19.39%" headers="mcps1.2.3.1.1 "><p id="p16649161316231"><a name="p16649161316231"></a><a name="p16649161316231"></a>adjustInterval</p>
</td>
<td class="cellrowborder" valign="top" width="80.61%" headers="mcps1.2.3.1.2 "><p id="p1264941315236"><a name="p1264941315236"></a><a name="p1264941315236"></a>Interval between dynamic adjustments. For example, if this parameter is set to <code>1000</code>, dynamic adjustment is performed every second.</p>
</td>
</tr>
<tr id="row32311937162315"><td class="cellrowborder" valign="top" width="19.39%" headers="mcps1.2.3.1.1 "><p id="p12232137132319"><a name="p12232137132319"></a><a name="p12232137132319"></a>perfDuration</p>
</td>
<td class="cellrowborder" valign="top" width="80.61%" headers="mcps1.2.3.1.2 "><p id="p152321375231"><a name="p152321375231"></a><a name="p152321375231"></a>Perf collection duration. For example, if this parameter is set to <code>1000</code>, perf will collect data within one second each time.</p>
</td>
</tr>
<tr id="row184443193219"><td class="cellrowborder" valign="top" width="19.39%" headers="mcps1.2.3.1.1 "><p id="p084124333218"><a name="p084124333218"></a><a name="p084124333218"></a>l3Percent</p>
</td>
<td class="cellrowborder" valign="top" width="80.61%" headers="mcps1.2.3.1.2 "><p id="p08412439327"><a name="p08412439327"></a><a name="p08412439327"></a>Maximum and minimum L3 cache percentages that can be used by offline services during dynamic adjustment. For example, if <code>low</code> is set to <code>20</code> and <code>high</code> is set to <code>50</code>, offline services can use at least 20% and at most 50% of L3 CacheWay during dynamic adjustment.</p>
</td>
</tr>
<tr id="row178752010355"><td class="cellrowborder" valign="top" width="19.39%" headers="mcps1.2.3.1.1 "><p id="p48722003512"><a name="p48722003512"></a><a name="p48722003512"></a>memBandPercent</p>
</td>
<td class="cellrowborder" valign="top" width="80.61%" headers="mcps1.2.3.1.2 "><p id="p287202023513"><a name="p287202023513"></a><a name="p287202023513"></a>Maximum and minimum memory bandwidth percentages that can be used by offline services during dynamic adjustment. For example, if <code>low</code> is set to <code>10</code> and <code>high</code> is set to <code>50</code>, offline services can use at least 10% and at most 50% of the memory bandwidth during dynamic adjustment.</p>
</td>
</tr>
<tr id="row49915348386"><td class="cellrowborder" valign="top" width="19.39%" headers="mcps1.2.3.1.1 "><p id="p1399203483817"><a name="p1399203483817"></a><a name="p1399203483817"></a>cacheMiss</p>
</td>
<td class="cellrowborder" valign="top" width="80.61%" headers="mcps1.2.3.1.2 "><p id="p119927345381"><a name="p119927345381"></a><a name="p119927345381"></a>Basis for determining whether to perform dynamic adjustment. For example, if <code>minMiss</code> is set to <code>10</code> and <code>maxMiss</code> is set to <code>50</code>, the available resources of offline services are reduced when the cache miss rate of online services is greater than 50%, and are increased when the cache miss rate of online services is less than 10%.</p>
</td>
</tr>
</tbody>
</table>

A JSON file is configured in the form of ConfigMap in the `k8s-mpam-controller.yaml` file. The complete YAML file is as follows:

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
      hostIPC: true
      containers:
      - name: k8s-mpam-controller-agent
        image: k8s-mpam-controller:0.1
        securityContext:
          privileged: true
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

![](docs/images/mpam-controller_dynamic_dir.png)

**Deploying Offline Services<a name="section28948105567"></a>**

1.  Add the annotation `kunpeng.com/offline: "true"` to the YAML file of the Pod to label the Pod as an offline service so that the plugin can restrict the Pod. The following is a `bw-mem.yaml` file example.

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

2.  Deploy the offline service to be restricted.

    ```
    kubectl apply -f bw-mem.yaml
    ```

    After the deployment is successful, the PID of the offline service is added to tasks in the `mpam-controller_dynamic` control group.

3.  Run the following commands to check PIDs of restricted offline services:

    ```
    cd /sys/fs/resctrl/mpam-controller_dynamic
    cat tasks
    ```

# Contribution Guide<a name="EN-US_TOPIC_0000002474936453"></a>

You are welcome to submit issues and pull requests to improve the projects. Ensure that:

-   The code complies with the project specifications.
-   Appropriate tests are included.
-   The related documents are updated.

# Disclaimer<a name="EN-US_TOPIC_0000002441456466"></a>

This project does not include any express or implied warranties, including but not limited to warranties of merchantability, fitness for a particular purpose, and non-infringement. Under no circumstances shall the copyright owner or contributors be liable for any direct, indirect, special, incidental, or consequential damages arising from the use of this software. By using this software, you agree to the above terms.

# License<a name="EN-US_TOPIC_0000002441616326"></a>

This project is licensed under Apache License 2.0. For details, see [LICENSE](https://gitcode.com/boostkit/cloud-native/blob/master/LICENSE).
