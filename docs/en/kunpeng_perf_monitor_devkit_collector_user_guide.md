# kunpeng-perf-monitor DevKit Collector User Guide

<!-- md-trans-meta sourceCommit=8007e1c5d2faa7fe3dbcd3ab3ef0d5304b736a7b translatedAt=2026-09-17T10:24:43.031Z pushedAt=2026-09-18T11:08:56.197Z -->

## Introduction<a name="devkit-collector-introduction"></a>

kunpeng-perf-monitor is a node performance monitoring component for Kubernetes clusters on Kunpeng servers. It is deployed on cluster nodes in DaemonSet mode and provides performance metrics through the Prometheus Exporter interface.

DevKit Collector is a collection component in kunpeng-perf-monitor implemented based on the [Kunpeng DevKit Tuner CLI](https://www.hikunpeng.com/document/detail/en/kunpengdevps/profiler/profiler/KunpengDevKitCli_0032.html). This component periodically collects the CPU pipeline, cache, and memory access data, and converts the collection results into Prometheus metrics for analyzing performance issues such as CPU pipeline bottlenecks, memory access efficiency, and bandwidth.

By deploying the DevKit Collector, users can continuously observe node performance changes in Prometheus or Grafana. This shortens the time required to locate performance bottlenecks. This document describes the environment preparation, deployment, configuration, metric query, and troubleshooting methods of the DevKit Collector. The DevKit Collector includes the following two collectors.

**Table 1** Collector description in DevKit Collector<a id="devkit-collector-collector-description"></a>

|Collector|Purpose|Supported Collection Scope|
|--|--|--|
|devkit-topdown|Analyzes CPU pipeline bottlenecks.|System, CPU cores, and PIDs|
|devkit-memory|Collects cache, memory, and other related metrics.|System and CPU cores|

This document provides the following two deployment modes:

- NodePort standalone mode: used for first-time deployment and quick verification. You can view the current metrics through the node IP and port `30010`. This mode does not depend on kube-prometheus, does not save historical data, and does not allow you to view metric trends in Grafana.
- Prometheus mode: used for continuous collection, storage, and visualization of metrics. This mode requires that kube-prometheus be deployed in the cluster and the DevKit Collector be connected to Prometheus through ServiceMonitor. This document uses Grafana for metric visualization. Therefore, Grafana must also be available (a complete kube-prometheus deployment includes Grafana, and thus no separate operation is required).

For first-time deployment, you are advised to use the NodePort standalone mode. Confirm that the DevKit Collector can collect metrics normally, and then switch to the Prometheus mode as needed.

> **NOTE:**
> The NodePort standalone mode and Prometheus mode use Kubernetes resources with the same names. The two modes cannot be used for deployment at the same time. Uninstall the current-mode deployment before switching to the other mode.

## Environment Requirements<a name="devkit-collector-environment-requirements"></a>

This document provides guidance based on specific environments. Before performing operations, ensure that your hardware, software, and operation permissions meet the requirements.

> NOTE:
> Kubernetes is the basic prerequisite environment for this document, and kube-prometheus is the external prerequisite environment for Prometheus mode. The installation and deployment of prerequisite environments are not covered in this document, but this document provides related links as deployment references. After completing the deployment, check the environment status as required in this section.

**Hardware Requirements<a name="devkit-collector-hardware-requirements"></a>**

[**Table 2**](#devkit-collector-hardware-requirement) lists the hardware requirement.

**Table 2** Hardware requirement<a id="devkit-collector-hardware-requirement"></a>

|Item|Requirement|
|--|--|
|Processor|Kunpeng 950|

**OS and Software Requirements<a name="devkit-collector-software-requirements"></a>**

The versions in [**Table 3**](#devkit-collector-verified-software-versions) have been verified. When using other compatible versions, you need to verify them yourself.

**Table 3** Verified OS and software versions<a id="devkit-collector-verified-software-versions"></a>

|Software|Version or Requirement|How to Obtain|Remarks|
|--|--|--|--|
|OS|openEuler 24.03 LTS SP3|[Link](https://www.openeuler.org/en/download/archive/detail/?version=openEuler%2024.03%20LTS%20SP3)||
|Kubernetes|1.28.14|See the [Sealos-based Kubernetes cluster deployment](https://sealos.run/en/docs/advanced/k8s/getting-started) for quick deployment.|DevKit Collector runtime environment|
|containerd|1.7.13|Installed together with Sealos-based Kubernetes v1.28.14 installation.|Kubernetes container runtime|
|Go|1.25.0 recommended|[Link](https://go.dev/dl/)|If the official link download speed is too slow, you can switch to another trusted download source.|
|Docker|18.09.0|Install it using Yum.|Used to build the kunpeng-perf-monitor image|
|kube-prometheus|release-0.16|See the [official community](https://github.com/prometheus-operator/kube-prometheus/tree/release-0.16) for installation and deployment.|Prerequisite environment for Prometheus mode|

Docker is only used to build the `kunpeng-perf-monitor` image. Kubernetes nodes use containerd as the underlying container runtime.

Before deployment, run the following command to check the Kubernetes nodes.

```bash
kubectl get nodes -L kubernetes.io/arch -o wide
```

The target node should be in the `Ready` state, and `kubernetes.io/arch` should be `arm64`. The current user should also have the permission to create resources such as DaemonSet, Service, ConfigMap, ServiceAccount, Role, and RoleBinding.

## Port Exposure Description

DevKit Collector exposes Prometheus metrics over HTTP. Authentication and TLS are not enabled by default. In production environments, restrict the source addresses allowed for access through Kubernetes NetworkPolicy, firewalls, or security groups.

In NodePort standalone mode, to facilitate obtaining metrics on a physical machine, the corresponding deployment file configures `nodePort` as `30010`. In actual usage, if this requirement is not needed, you can delete the related NodePort configuration so that the related ports are not exposed on the physical machine.

In Prometheus mode, Prometheus discovers targets based on ServiceMonitor and directly scrapes port `9100` of the DevKit Collector Pod. The DevKit Collector program itself does not expose the related port outside the cluster.

|Source Device|Source IP Address|Source Port|Destination Device|Destination IP Address|Destination Port<br>(Listening)|Protocol|Port Description|Whether the Listening Port Can Be Changed|Authentication Method|Encryption Method|Plane|Version|Special Scenario|Remarks|
|--|--|--|--|--|--|--|--|--|--|--|--|--|--|--|
|Kubernetes node|DevKit Collector Pod IP address|9100|Kubernetes node (DevKit Collector NodePort)|Kubernetes node IP address|30010|TCP|Accesses the `/metrics` interface of the DevKit Collector to verify the current collection result.|Yes. Modify the `nodePort` of `Service` in `deployment-devkit.yaml` and adjust the access address.|None|None, HTTP plaintext|O&M management plane|kunpeng-perf-monitor 1.0|NodePort standalone mode only|`Service` enables port forwarding from `30010` to `9100` (Collector Pod). It is recommended for opening the `30010` port only to the O&M network segment.|

> **NOTE:**
>
> `30010` is an example NodePort. When modifying it, avoid ports already occupied within the cluster NodePort range, and adjust the firewall or security group rules accordingly.

## Image Compilation<a name="devkit-collector-build"></a>

Before compilation, ensure that the server used to compile images can access the Go module and DevKit Tuner CLI download addresses, and can pull the container base images.

**Preparation Before Compilation<a name="devkit-collector-build-preparation"></a>**

Check Go and Docker to ensure that they are installed and their versions meet the requirements.

```bash
go version
docker version
```

Go 1.25 is recommended, and the Docker daemon should be in an available state.

**Procedure<a name="devkit-collector-build-steps"></a>**

1. Obtain the source code.

    ```bash
    git clone https://gitcode.com/boostkit/cloud-native.git
    cd /path/to/cloud-native
    ```

    `/path/to/cloud-native` indicates the actual source code directory. All subsequent commands are executed in this directory.

2. Build the `kunpeng-perf-monitor:1.0` image.

    ```bash
    make kunpeng-perf-monitor-docker
    ```

    The build process uses `Dockerfile.kunpeng-perf-monitor` to compile the container image and installs the Kunpeng DevKit Tuner CLI to the `/opt/devkit` directory of the image.

3. Check whether the image is built successfully.

    ```bash
    docker images | grep kunpeng-perf-monitor
    ```

    If `kunpeng-perf-monitor` can be queried and its tag is `1.0`, the image has been built successfully.

4. Check the exporter and DevKit Tuner CLI in the image.

    ```bash
    docker run --rm --entrypoint sh kunpeng-perf-monitor:1.0 -c \
      'test -x /bin/kunpeng-perf-monitor && \
       test -x /opt/devkit/devkit && \
       test -f /opt/devkit/execute.ini && \
       test -f /opt/devkit/tuner/lib/libkperf.so'
    ```

    If the command exit code is `0` and there is no error output, the image contains the files required for running the DevKit Collector.

5. Export the image as a TAR package.

    ```bash
    docker save kunpeng-perf-monitor:1.0 -o kunpeng-perf-monitor-1.0.tar
    ```

6. Copy `kunpeng-perf-monitor-1.0.tar` to each AArch64 target node, and then import it into Kubernetes.

    ```bash
    ctr -n k8s.io images import kunpeng-perf-monitor-1.0.tar
    ctr -n k8s.io images ls | grep kunpeng-perf-monitor
    ```

    If `kunpeng-perf-monitor:1.0` can be queried, the image has been imported successfully.

> **NOTICE:**
> The deployment manifest uses `imagePullPolicy: IfNotPresent` to avoid repeated image pulling. If the `kunpeng-perf-monitor:1.0` image already exists in the environment before import, delete the old image on the node or confirm that the image content has been updated after the import.

## DevKit Collector Deployment<a name="devkit-collector-deployment"></a>

The commands in this section are executed on the control node where the source code has been obtained and the Kubernetes cluster is accessible. For first-time use, you are advised to use the NodePort standalone mode for deployment.

The deployment manifest `config/kunpeng-perf-monitor/k8s/deployment-devkit.yaml` creates the following resources in the `default` namespace.

**Table 4** Default deployment resources<a id="devkit-collector-default-deployment-resources"></a>

|Resource|Name or Value|
|--|--|
|ServiceAccount|kunpeng-perf-monitor|
|Role/RoleBinding|kunpeng-perf-monitor-config-reader|
|ConfigMap|kunpeng-perf-monitor-devkit-config|
|DaemonSet|kunpeng-perf-monitor-devkit|
|Service|kunpeng-perf-monitor-devkit|
|Container port|9100|
|DevKit Tuner CLI|/opt/devkit/devkit|

> **NOTICE:**
> Kunpeng DevKit Tuner CLI uses PMU to capture related data. Therefore, the deployment file runs the DaemonSet as the `root` user, adds the `SYS_ADMIN` capability, and mounts the CPU- and PMU-related sysfs in read-only mode. Before deployment, confirm that these permissions comply with the security policy of the target cluster.
> In addition, the DaemonSet sets `hostPID: true` to obtain process information on the node.

**Deployment in NodePort Standalone Mode<a name="devkit-collector-nodeport-deployment"></a>**

1. Go to the source code directory.

    ```bash
    cd /path/to/cloud-native
    ```

2. Confirm that the DevKit Collector is not deployed in Prometheus mode. If it has been deployed in Prometheus mode, run the following command to delete it first.

    ```bash
    kubectl delete -f config/kunpeng-perf-monitor/k8s/devkit-prometheus/deployment.yaml --ignore-not-found
    ```

3. Deploy the DevKit Collector in NodePort standalone mode.

    ```bash
    kubectl apply -f config/kunpeng-perf-monitor/k8s/deployment-devkit.yaml
    ```

4. Wait for the DaemonSet to become ready.

    ```bash
    kubectl rollout status \
      daemonset/kunpeng-perf-monitor-devkit --timeout=5m
    ```

    If `daemon set "kunpeng-perf-monitor-devkit" successfully rolled out` is displayed, the DaemonSet is ready.

5. View the Pods and services.

    ```bash
    kubectl get daemonset,pod,service -o wide
    ```

    Each eligible node should run one Pod that is `Running` and `Ready`. The service type should be `NodePort`, and the port should include `9100:30010/TCP`. The output is similar to the following:

    ```bash
    NAME                                         DESIRED   CURRENT   READY   UP-TO-DATE   AVAILABLE   NODE SELECTOR              AGE   CONTAINERS             IMAGES                     SELECTOR
    daemonset.apps/kunpeng-perf-monitor-devkit   1         1         1       1            1           kubernetes.io/arch=arm64   43h   kunpeng-perf-monitor   kunpeng-perf-monitor:1.0   app.kubernetes.io/component=devkit-collector,app.kubernetes.io/name=kunpeng-perf-monitor

    NAME                                    READY   STATUS    RESTARTS   AGE   IP             NODE     NOMINATED NODE   READINESS GATES
    pod/kunpeng-perf-monitor-devkit-kflkq   1/1     Running   0          43h   100.64.0.149   master   <none>           <none>

    NAME                                  TYPE        CLUSTER-IP    EXTERNAL-IP   PORT(S)          AGE   SELECTOR
    service/kubernetes                    ClusterIP   10.96.0.1     <none>        443/TCP          21d   <none>
    service/kunpeng-perf-monitor-devkit   NodePort    10.96.1.186   <none>        9100:30010/TCP   43h   app.kubernetes.io/component=devkit-collector,app.kubernetes.io/name=kunpeng-perf-monitor
    ```

**(Optional) Deployment in Prometheus Mode<a name="devkit-collector-prometheus-deployment"></a>**

If **the cluster has completed Prometheus deployment according to the [kube-prometheus release-0.16 official documentation](https://github.com/prometheus-operator/kube-prometheus/tree/release-0.16), and you want Prometheus to automatically discover the DevKit Collector, continuously collect metrics, and visualize them through Grafana**, perform the following steps:

1. Check the `ServiceMonitor` CRD, Prometheus service, and Grafana service.

    ```bash
    kubectl api-resources | grep -w servicemonitors
    kubectl -n monitoring get service prometheus-k8s grafana
    ```

    The `ServiceMonitor` resource and the `prometheus-k8s` and `grafana` services should be queryable. Only then can the DevKit Collector be deployed.

2. Delete the deployment based on NodePort standalone mode and wait for the old Pods to be deleted.

    ```bash
    kubectl delete -f config/kunpeng-perf-monitor/k8s/deployment-devkit.yaml \
      --ignore-not-found
    kubectl -n default wait --for=delete pod \
      -l app.kubernetes.io/component=devkit-collector --timeout=5m
    ```

    If the NodePort standalone mode was not used for deployment, the second command may report that no matching resources were found. You can proceed to the next step.

3. Deploy the DevKit Collector in Prometheus mode.

    ```bash
    kubectl apply -f config/kunpeng-perf-monitor/k8s/devkit-prometheus/deployment.yaml
    kubectl -n default rollout status \
      daemonset/kunpeng-perf-monitor-devkit --timeout=5m
    ```

4. View the services and ServiceMonitor.

    ```bash
    kubectl -n default get service kunpeng-perf-monitor-devkit
    kubectl -n default get servicemonitor kunpeng-perf-monitor-devkit -o yaml | grep endpoints
    ```

    The service type should be `ClusterIP`, the ServiceMonitor collection path should be `/metrics`, and each `Ready` Pod should correspond to one endpoint.

## DevKit Collector Usage<a name="devkit-collector-usage"></a>

### Metric Collection Verification<a name="devkit-collector-first-verification"></a>

DevKit Collector starts the first collection immediately after startup. The TopDown and Memory collectors run serially in the same Pod. It is recommended that you wait about 15 seconds after the Pod is ready before checking the metrics.

**NodePort Standalone Mode<a name="devkit-collector-nodeport-verification"></a>**

1. View the `INTERNAL-IP` of the node where the Pod is located.

    Assuming the node name is `master`, use the following command to view the `INTERNAL-IP` of the node.

    ```bash
    kubectl get node master -o wide
    ```

    Reference output:

    ```bash
    NAME     STATUS   ROLES                  AGE   VERSION    INTERNAL-IP     EXTERNAL-IP   OS-IMAGE                    KERNEL-VERSION                             CONTAINER-RUNTIME
    master   Ready    control-plane,worker   22d   v1.28.14   192.168.122.2   <none>        openEuler 24.03 (LTS-SP3)   6.6.0   containerd://1.6.22.28
    ```

    Record the `INTERNAL-IP` of the node (`192.168.122.2` in the example). This address is represented by `<node-ip>` in the following text.

2. Access the metric interface to view the related metrics.

    ```bash
    curl http://<node-ip>:30010/metrics | \
      grep -E kunpeng_node_devkit
    ```

    Under the default system collection scope, the command output should contain the following metrics:

    ```text
    kunpeng_node_devkit_topdown_collection_success{target="system",target_type="system"} 1
    kunpeng_node_devkit_memory_collection_success{period_milliseconds="1000",target="system",target_type="system"} 1
    ```

    `collection_success=1` indicates that the last DevKit Tuner CLI execution and result parsing are successful. If the collection fails, the value is `0` and no other related metrics are output.

**Prometheus Mode<a name="devkit-collector-prometheus-verification"></a>**

1. Confirm the address and port of the Prometheus service `<prometheus-address>:<port>`.

  > **NOTE:**
  >
  > 1. In the default kube-prometheus deployment, the Prometheus service is deployed as `ClusterIP` and **is accessible only within the cluster**. To access it from outside the cluster (for example, from the physical machine of the corresponding node), configure it as `NodePort`. For configuration reference, see the **NodePort Configuration for Prometheus and Grafana** section below.
  > 2. Grafana is the graphical operation interface built into kube-prometheus, which enables convenient metric query. It will also be used for metric query later in this document. Therefore, configure it as `NodePort` as well.

  **(Optional) NodePort Configuration for Prometheus and Grafana**  
  In the default kube-prometheus deployment, the service type of Prometheus and Grafana is `ClusterIP`, meaning that they are accessible only within the cluster. To access these two services from outside, change their type to `NodePort`. The modification reference is as follows:

  In the `manifests/` directory of kube-prometheus, modify the two service files as follows. The `nodePort` value can be adjusted based on the actual situation.
  `grafana-service.yaml`: Change the value of `spec.type` to `NodePort`, and specify `nodePort: 30000` for the `http` port.

  ```yaml
  spec:
    type: NodePort
    ports:
    - name: http
      port: 3000
      targetPort: http
      nodePort: 30000
  ```

  `prometheus-service.yaml` (service name: `prometheus-k8s`): Change the value of `spec.type` to `NodePort`, and specify `nodePort: 30090` for the `web` port (the `reloader-web` port is not exposed externally and is automatically assigned by the system).

  ```yaml
  spec:
    type: NodePort
    ports:
    - name: web
      port: 9090
      targetPort: web
      nodePort: 30090
    - name: reloader-web
      port: 8080
      targetPort: reloader-web
  ```

  In the root directory of the kube-prometheus source code, run the following commands to make the modified service configuration take effect.

  ```bash
  kubectl apply -f manifests/grafana-service.yaml \
    -f manifests/prometheus-service.yaml
  ```

  After the application is complete, run the following command to confirm that the two services have been updated to `NodePort`, and record the actual assigned ports.

  ```bash
  kubectl -n monitoring get service grafana prometheus-k8s
  ```

  **Querying `<prometheus-address>:<port>`**  
  Run the following command:

  ```bash
  kubectl get svc -A | grep -i prometheus
  ```

  In the output, find the namespace where the Prometheus service resides (default: `monitoring`), the service name (default: `prometheus-k8s`), and `PORT(S)` (default: `9090`). If the Prometheus NodePort is configured as described above, `PORT(S)` is `9090:30090/TCP`, where `9090` is the service port and `30090` is the NodePort.

- When accessing Prometheus within the cluster, `<prometheus-address>` is the `CLUSTER-IP` of the service, and `<port>` is the service port.
- When accessing Prometheus from outside the cluster via NodePort, `<prometheus-address>` is the `INTERNAL-IP` of the node, and `<port>` is the NodePort after the colon in `PORT(S)`. The node address can be confirmed by running `kubectl get node <node-name> -o wide`.

1. Replace `<prometheus-address>:<port>` with the actual address and port confirmed in the previous step, and query the active target.

    ```bash
    curl -fsS 'http://<prometheus-address>:<port>/api/v1/targets?state=active' | \
      jq '.data.activeTargets[] |
        select(.labels.job == "kunpeng-perf-monitor") |
        {scrapeUrl,health,lastError,labels}'
    ```

    The output should be similar to the following:

    ```bash
    {
      "scrapeUrl": "http://<pod-ip>:9100/metrics",
      "health": "up",
      "lastError": "",
      "labels": {
        "container": "kunpeng-perf-monitor",
        "endpoint": "metrics",
        "instance": "<pod-ip>:9100",
        "job": "kunpeng-perf-monitor",
        "namespace": "default",
        "pod": "kunpeng-perf-monitor-devkit-bcclv",
        "service": "kunpeng-perf-monitor-devkit"
      }
    }
    ```

    `health` should be `up`, and `lastError` should be empty. You can then query the related metrics in Prometheus or Grafana.

### Collection Scope Configuration<a name="devkit-collector-configure-scope"></a>

The collection configuration is stored in `devkit-tuner.yaml` of the ConfigMap `default/kunpeng-perf-monitor-devkit-config`. After the ConfigMap update passes validation, it takes effect automatically without a Pod restart.

The default configuration is as follows.

```yaml
topdown:
  cpu: ""
  pid: ""
  duration: 3
memory:
  cpu: ""
  duration: 3
  period: 1000
```

**Table 5** ConfigMap field description<a id="devkit-collector-configmap-field-description"></a>

|Field|Function|Valid Value|
|--|--|--|
|topdown.cpu|Specifies the collection scope as CPU cores for the TopDown collector; an empty value indicates the system scope.|CPU core number, range, or comma-separated set, for example `0`, `0-3`, and `0,2-3`|
|topdown.pid|Specifies the collection scope as PIDs for the TopDown collector; an empty value indicates the system scope.|Digits or comma-separated PIDs, for example `12345` and `12345,12346`|
|topdown.duration|Specifies the collection duration for the TopDown collector, in seconds.|An integer ranging from 1 to 5; default value: `3`|
|memory.cpu|Specifies the collection scope as CPU cores for the Memory collector; an empty value indicates the system scope.|CPU core number, range, or comma-separated set|
|memory.duration|Specifies the collection duration for the Memory collector, in seconds.|An integer ranging from 1 to 5; default value: `3`|
|memory.period|Specifies the sampling period for the Memory collector, in milliseconds.|`100` or `1000`; default value: `1000`|

Comply with the following rules during configuration:

- `topdown.cpu` and `topdown.pid` cannot be specified at the same time.
- `topdown.pid` does not accept `ALL`.
- The Memory collector does not support the PID or cgroup collection scope.
- When `memory.duration` is `1`, `memory.period` must be left empty or set to `100`.
- The TopDown profile level is fixed at `-L 0`, and the Memory metric is fixed at `-m 1`. Users cannot modify them through the ConfigMap.
- When the configuration contains unknown fields, incorrect types, or invalid values, the DevKit Collector rejects the entire configuration and continues to use the previous valid configuration.

Perform the following steps to modify the collection scope:

1. View the current ConfigMap.

    ```bash
    kubectl -n default get configmap kunpeng-perf-monitor-devkit-config \
      -o jsonpath='{.data.devkit-tuner\.yaml}'
    ```

2. Create a complete new configuration. The following example sets the collection scopes to CPU cores `0-3` for both TopDown and Memory collectors.

    ```bash
    cat > /tmp/devkit-tuner.yaml <<'EOF'
    topdown:
      cpu: "0-3"
      pid: ""
      duration: 3
    memory:
      cpu: "0-3"
      duration: 3
      period: 1000
    EOF
    ```

3. Update the ConfigMap.

    ```bash
    kubectl -n default create configmap kunpeng-perf-monitor-devkit-config \
      --from-file=devkit-tuner.yaml=/tmp/devkit-tuner.yaml \
      --dry-run=client -o yaml | kubectl apply -f -
    ```

4. Check whether the DevKit Collector accepts the configuration.

    ```bash
    kubectl -n default logs \
      -l app.kubernetes.io/component=devkit-collector --since=2m | \
      grep -E 'devkit_config_changed|devkit_config_rejected'
    ```

    If `devkit_config_changed` is displayed, the configuration has taken effect. If `devkit_config_rejected` is displayed, the configuration was rejected, and the DevKit Collector continues to use the previous valid configuration.

5. After waiting for the next collection round (15–30s), query the related metrics again. For the example in which the collection scope is set to CPU cores `0-3`, the labels should include `target_type="cpu"` and `target="cpu0-3"`.

To restore the system collection scope, set `topdown.cpu`, `topdown.pid`, and `memory.cpu` to empty strings, and then update the entire ConfigMap. **Deleting the ConfigMap does not restore the default values**, and the DevKit Collector continues to use the previous valid configuration.

To delete the ConfigMap, run the following command:

```bash
kubectl -n default delete configmap kunpeng-perf-monitor-devkit-config
```

**PID Collection Scope Example of the TopDown Collector<a name="devkit-collector-pid-scope-example"></a>**

```yaml
topdown:
  cpu: ""
  pid: "12345"
  duration: 3
```

This example corresponds to `target_type="pid"` and `target="pid12345"`. The Memory collector configuration remains unchanged and PIDs cannot be set.

**(Optional) Background Collection Interval Adjustment<a name="devkit-collector-interval"></a>**

The default background collection interval is `15` seconds. The environment variable `DEVKIT_COLLECT_INTERVAL` accepts only positive integers, for example `30`; values such as `30s`, `5m`, decimals, zero, or negative numbers are not allowed. For invalid values, `devkit_collect_interval_invalid` will be logged and the 15-second interval is used.

1. Temporarily modify the background collection interval of the current DaemonSet.

    ```bash
    kubectl -n default set env \
      daemonset/kunpeng-perf-monitor-devkit \
      DEVKIT_COLLECT_INTERVAL=30
    kubectl -n default rollout status \
      daemonset/kunpeng-perf-monitor-devkit --timeout=5m
    ```

2. To persist the configuration, add the following environment variable to the container in the deployment manifest actually in use (such as `config/kunpeng-perf-monitor/k8s/deployment-devkit.yaml` and `config/kunpeng-perf-monitor/k8s/devkit-prometheus/deployment.yaml`), and then reapply the manifest.

    ```yaml
    env:
    - name: DEVKIT_COLLECT_INTERVAL
      value: "30"
    ```

    Reapply the corresponding manifest in the root directory of the source code according to the current deployment mode.

    ```bash
    # NodePort standalone mode
    kubectl apply -f config/kunpeng-perf-monitor/k8s/deployment-devkit.yaml

    # Prometheus mode
    kubectl apply -f config/kunpeng-perf-monitor/k8s/devkit-prometheus/deployment.yaml
    ```

    This variable is read only when the process starts. Modifying it triggers a rolling update of the Pod. A configuration value less than `11` seconds does not prevent startup, but the log will display `devkit_capacity_warning`. In Prometheus mode, it is recommended that this value be consistent with `interval` of ServiceMonitor.
3. Verify that the configuration takes effect.
   Run the following command to query the current collection interval:

   ```bash
   kubectl get daemonset/kunpeng-perf-monitor-devkit -o yaml | grep DEVKIT_COLLECT_INTERVAL -A 1
   ```

   The expected output is as follows:

   ```bash
    - name: DEVKIT_COLLECT_INTERVAL
      value: "30"
   ```

### Metric Check<a name="devkit-collector-metrics"></a>

**Table 6** Main metrics of the TopDown collector<a id="devkit-collector-topdown-metrics"></a>

|Metric|Description|
|--|--|
|kunpeng_node_devkit_topdown_cycles|CPU cycles in the last collection window|
|kunpeng_node_devkit_topdown_instructions|Instructions in the last collection window|
|kunpeng_node_devkit_topdown_ipc_ratio|IPC ratio|
|kunpeng_node_devkit_topdown_bound_percent|Tree node percentage|
|kunpeng_node_devkit_topdown_pmu_event_count_value|PMU event count|
|kunpeng_node_devkit_topdown_collection_success|Whether the last collection is successful|
|kunpeng_node_devkit_topdown_last_success_unixtime_seconds|Time of the last successful collection|

**Table 7** Main metrics of the Memory collector<a id="devkit-collector-memory-metrics"></a>

|Metric|Description|
|--|--|
|kunpeng_node_devkit_memory_cache_miss_percent|Cache miss percentage|
|kunpeng_node_devkit_memory_ddr_system_bandwidth_megabytes_per_second|System DDR bandwidth|
|kunpeng_node_devkit_memory_access_bandwidth_megabytes_per_second|L1, L2, and TLB access bandwidth|
|kunpeng_node_devkit_memory_access_hit_percent|L1, L2, and TLB hit rate|
|kunpeng_node_devkit_memory_l3_read_bandwidth_megabytes_per_second|L3 read bandwidth|
|kunpeng_node_devkit_memory_l3_read_hit_bandwidth_megabytes_per_second|L3 read hit bandwidth|
|kunpeng_node_devkit_memory_l3_read_hit_percent|L3 read hit rate|
|kunpeng_node_devkit_memory_ddrc_bandwidth_megabytes_per_second|DDRC read and write bandwidth|
|kunpeng_node_devkit_memory_collection_success|Whether the last collection is successful|
|kunpeng_node_devkit_memory_last_success_unixtime_seconds|Time of the last successful collection|

The unit of bandwidth metrics is MB/s as reported by the DevKit Tuner CLI. Percentage and hit rate metrics are dimensionless values. All service metrics are gauge snapshots of the last collection window and should not be calculated as counters using `rate()` or `increase()` of PromQL.

#### (Optional) Prometheus Query Example using PromQL

Query the health status of the TopDown collector with the system collection scope.

```promql
kunpeng_node_devkit_topdown_collection_success{
  job="kunpeng-perf-monitor",
  target_type="system",
  target="system"
}
```

Query the number of seconds elapsed since the last successful metric collection of the Memory collector.

```promql
time() - kunpeng_node_devkit_memory_last_success_unixtime_seconds{
  job="kunpeng-perf-monitor"
}
```

Query the `Memory Bound` child nodes of the TopDown collector.

```promql
kunpeng_node_devkit_topdown_bound_percent{
  path=~"backend_bound\\.memory_bound\\..*"
}
```

> **NOTE:**
> `up=1` of Prometheus only indicates a successful HTTP scrape. To determine whether the DevKit Collector background collection is successful, query both `collection_success` and `last_success_unixtime_seconds` of the corresponding collector.

#### (Recommended) Graphical Metric Check in Grafana<a name="devkit-collector-grafana"></a>

This section applies to scenarios where **kube-prometheus has been deployed** and **the DevKit Collector is deployed in Prometheus mode**. The NodePort standalone mode only provides the `/metrics` interface and does not automatically write historical data to Prometheus. Therefore, trend charts cannot be viewed directly in Grafana.

**Graphical Metric Check**

1. Access Grafana using a browser.

    ```text
    http://<node-ip>:<grafana-node-port>/
    ```

2. Log in to Grafana. The default username and password are `admin` and `admin`, respectively. After logging in, you will be required to change the password. It is recommended that you change it to a complex password.
3. Query related metrics.
   Select `Explore` in the left navigation bar, click `Metric`, and enter `devkit`. Then, all related metrics are automatically listed. Select one of them, and finally click `Run query` to view the metric trend chart. See the following figure.

  ![Grafana operation diagram](figures/grafana-guide.png)

   If the `kunpeng_node_devkit_*` metrics can be queried and curves that change over time are displayed, the metric visualization process described in this document has been completed.

> **NOTE:**
> You can set the query time range in the upper right corner on the `Explore` page.

### Troubleshooting<a name="devkit-collector-troubleshooting"></a>

1. View the DevKit Collector logs for the last 10 minutes.

    ```bash
    kubectl -n default logs \
      -l app.kubernetes.io/component=devkit-collector \
      --timestamps --since=10m | \
      grep -E 'collection_(start|finish)|devkit_config_(rejected|deleted)|devkit_collect_interval_invalid|devkit_capacity_warning'
    ```

    A normal collection produces paired `collection_start` and `collection_finish`, and the log for a successful collection contains `status=success`.

2. Troubleshoot based on the symptoms.

    **Table 8** Handling methods for common issues<a id="devkit-collector-common-issues"></a>

    |Symptom|Possible Cause|Handling Method|
    |--|--|--|
    |The Pod fails to start and the image cannot be found.|The image is not imported into the `k8s.io` namespace of containerd, or the node still uses the old image.|Run `ctr -n k8s.io images ls`, re-import the image, and then delete the failed Pod.|
    |The NodePort cannot be accessed.|The Pod or service is not ready, or the node port is blocked by the firewall.|Check the Pod, service, EndpointSlice, and node network.|
    |`/metrics` is accessible but `collection_success` is `0`.|The DevKit Tuner CLI fails to execute commands or times out, or the report parsing fails.|Check the status and error in the `collection_finish` log.|
    |ConfigMap changes do not take effect.|The YAML file contains unknown fields, incorrect field types or values, or simultaneous configuration of CPU cores and PIDs for the TopDown collector.|Check `devkit_config_rejected`, correct the errors, and resubmit the complete configuration.|
    |The system collection scope is not restored after the ConfigMap is deleted.|Deleting the ConfigMap retains the last valid configuration.|Resubmit a complete configuration with both CPU cores and PIDs being empty.|
    |There is no target in Prometheus.|The ServiceMonitor is not selected by Prometheus.|Check the serviceMonitorSelector, namespace selector, and service labels of Prometheus.|
    |`up` is `1` in Prometheus but service collection fails.|The HTTP scrape is normal, but the background collection fails.|Query `collection_success` and the last successful collection time of the two collectors.|

When the DevKit Tuner CLI execution or result parsing fails, the DevKit Collector sets the current round's `collection_success` to `0` and stops publishing the current round's service metrics to avoid mistaking old data for current data. The last successful collection time is retained; after the issue is fixed, the next successful collection republishes the service metrics.

## (Optional) DevKit Collector Uninstallation<a name="devkit-collector-uninstall"></a>

Select the corresponding command based on the current deployment mode.

1. In NodePort standalone mode, run the following commands:

    ```bash
    kubectl delete -f config/kunpeng-perf-monitor/k8s/deployment-devkit.yaml \
      --ignore-not-found
    ```

2. In Prometheus mode, run the following commands:

    ```bash
    kubectl delete -f config/kunpeng-perf-monitor/k8s/devkit-prometheus/deployment.yaml \
      --ignore-not-found
    ```

3. Check whether the resources have been deleted.

    ```bash
    kubectl -n default get \
      daemonset,service,configmap,serviceaccount,role,rolebinding,servicemonitor \
      | grep kunpeng-perf-monitor || true
    ```

    If no output is returned, the DevKit Collector resources created in this document have been deleted.

## Change History

   | Version | Date   | Description         |
   | -------- | ---------- | ---------------- |
   | 01       | 2026-09-30 | This is the first official release. |
