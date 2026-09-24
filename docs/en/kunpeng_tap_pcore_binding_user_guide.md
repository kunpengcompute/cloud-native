# Kunpeng-TAP Pcore Binding Plugin User Guide

<!-- md-trans-meta sourceCommit=0d1806cc482ddad2e8b1db19de4c9f208d321266 translatedAt=2026-09-17T10:18:50.023Z pushedAt=2026-09-18T02:45:20.738Z -->

## Introduction

The Kunpeng-TAP Pcore Binding plugin is a physical core binding plugin provided by Kunpeng-TAP for Kata confidential containers. Kunpeng-TAP provides general-purpose topology affinity capabilities for CPUs, memory, and other container resources. On top of this, the plugin provides finer-grained physical core binding for Kata Pods. It connects to containerd through the Node Resource Interface (NRI) and consolidates the two logical CPU cores of Pods that meet the trustlist conditions onto the simultaneous multi-threading (SMT) sibling pair of a single physical core. The plugin can be deployed independently and does not depend on the Kunpeng-TAP main program.

This document describes how to deploy and use `kunpeng-tap-pcore-binding` in DaemonSet mode. The information about the containerd, Kata, cloud-hypervisor, and AArch64 node in this document has been verified as test conditions. You can complete deployment, parameter configuration, and binding result checks by following the steps in this document under the same or equivalent conditions.

### Function Scope

- The plugin operates at the Pod level, not at the container level.
- The plugin processes only those Pods whose aggregated CPU limit is exactly `2`; CPU requests are not taken into account during filtering.
- The two logical CPU cores of such a Pod are bound to the SMT sibling pair of a single physical core.
- Binding state is not saved. On each iteration, the plugin recalculates occupancy from the current Pod cgroup's `cpuset.cpus`.
- Multiple eligible Pods are not allowed to converge onto the same sibling pair.
- The processing scope is restricted via namespace and RuntimeClass trustlists.

## Environment Requirements

This document provides guidance based on specific environments. Before performing operations, ensure that your hardware and software meet the requirements.

### Verified Test Conditions

The deployment process has been verified in the following AArch64 test environment.

**Table 1** Verified test conditions

| Item | Test Value |
| --- | --- |
| CPU architecture | AArch64 |
| Kubernetes | v1.34.7 |
| containerd | v2.1.7 |
| cgroup | cgroup v1, with the cpuset mount point at `/sys/fs/cgroup/cpuset` |
| NRI socket | `/var/run/nri/nri.sock` |
| Kata runtime handler | `kata-clh` |
| Kata hypervisor | cloud-hypervisor |
| Verification scale | 100 Pods with `runtimeClassName: kata-clh` |

### Theoretical Compatibility Scope

In addition to the tested versions above, the following theoretical compatibility scope can be derived based on the Container Resource Interface (CRI) and NRI that the current plugin implementation depends on.

**Table 2** Theoretical compatibility scope

| Component | Theoretical Compatibility Scope | Description |
| --- | --- | --- |
| Kubernetes | <code>v1.26.x to v1.36.x</code> | The plugin does not access the Kubernetes API Server at runtime and does not depend on Kubernetes API objects of a specific version. The lower version limit is <code>v1.26</code>, because starting from this version kubelet only supports CRI v1; the upper version limit is the version released at the time of document writing that has a recommended containerd combination. |
| containerd | <code>v1.7.x to v2.3.x</code> | containerd has integrated CRI NRI support since <code>v1.7</code>, and this capability has been stable and enabled by default starting from <code>v2.0</code>. The plugin requires containerd to provide Pod-level CPU quota and period through NRI PodSandbox events. |

This compatibility scope is a theoretical assessment based on the interfaces and code paths involved. It does not imply that all version combinations within the scope have been certified, nor does it represent a commitment to support every patch version within the scope. When selecting a Kubernetes and containerd combination, follow the [official Kubernetes support matrix for containerd](https://github.com/containerd/containerd/blob/main/RELEASES.md#kubernetes-support) and meet the following conditions:

- CRI v1 is used between Kubernetes and containerd. Starting from `v1.26`, Kubernetes requires the runtime to support CRI v1. For details, see the [Kubernetes Container Runtimes](https://kubernetes.io/docs/setup/production-environment/container-runtimes/#cri-version-support).
- containerd has CRI and NRI enabled, and the plugin can connect to the NRI socket and receive `Synchronize`, `RunPodSandbox`, `StopPodSandbox`, and `RemovePodSandbox` events.
- The NRI PodSandbox data contains the aggregated CPU quota and period; if these fields are missing, the plugin cannot confirm the CPU limit and conservatively skips the Pod.
- CRI NRI in containerd `v1.7` is an experimental capability. Therefore, you are advised to use the latest patch release of this branch. containerd `v2.0` or later is the preferred choice. For the NRI version status, see the [containerd release notes](https://github.com/containerd/containerd/blob/main/RELEASES.md#experimental-features).
- If the scope above is exceeded, a cross-major-version upgrade is performed, or an untested combination is used, you must re-execute the 1/2/4 CPU limit regression tests in this document, and verify that the `cpuset.cpus` results of the Pod parent cgroup and Kata sandbox cgroup are consistent.

### Pre-Deployment Check

Confirm the following items before deployment.

- containerd has NRI enabled and `/var/run/nri/nri.sock` is generated.
- Kata Containers is installed and a usable runtime handler, such as `kata-clh`, is configured.
- SMT is enabled on the node CPU, and `/sys/devices/system/cpu/cpu*/topology/thread_siblings_list` is readable.
- The target namespace and RuntimeClass have been added to the plugin trustlist.
- The aggregated CPU limit of the target Pod is two cores. CPU requests can be set according to service needs, but must not exceed the limit.

## Enabling containerd NRI

Both containerd `v1.7` and `v2.x` can configure NRI through `/etc/containerd/config.toml`, but the default state and the CRI plugin name differ.

**Table 3** containerd version differences

| containerd Version | Recommended Configuration Format | NRI Default State | CRI Plugin Name |
| --- | --- | --- | --- |
| v1.7.x | version = 2 | Disabled by default; you must explicitly set `disable = false` | io.containerd.grpc.v1.cri |
| v2.x | version = 3 | Enabled by default; you still need to confirm that external plugin connections are allowed | io.containerd.cri.v1.runtime |

Follow the steps below to enable NRI.

1. Back up the containerd configuration file.

    ```bash
    cp /etc/containerd/config.toml /etc/containerd/config.toml.bak
    ```

2. Confirm or add the following configuration in `/etc/containerd/config.toml`.

    ```toml
    [plugins.'io.containerd.nri.v1.nri']
      disable = false
      socket_path = '/var/run/nri/nri.sock'
      plugin_path = '/opt/nri/plugins'
      plugin_config_path = '/etc/nri/conf.d'
      plugin_registration_timeout = '10s'
      plugin_request_timeout = '5s'
      disable_connections = false
    ```

3. Restart containerd.

    ```bash
    systemctl restart containerd
    ```

4. Check the NRI socket.

    ```bash
    test -S /var/run/nri/nri.sock && echo "NRI socket is ready"
    ```

5. Check the containerd configuration output result to confirm that the NRI configuration has taken effect.

    ```bash
    containerd config dump | grep -n -A8 "io.containerd.nri.v1.nri"
    ```

## Preparing the Kata RuntimeClass

If the QEMU backend in the test environment has CPU hotplug limitations, you can use the cloud-hypervisor backend. The repository provides the `kata-clh` RuntimeClass manifest.

```bash
kubectl apply -f config/kunpeng-tap-pcore-binding/runtimeclass-cloud-hypervisor.yaml
kubectl get runtimeclass kata-clh
```

In containerd, a runtime configuration entry with the same name as the RuntimeClass handler must exist. In containerd `2.x`, the CRI plugin is identified as `io.containerd.cri.v1.runtime`. The following configuration is used in the currently verified containerd `2.1` environment:

```toml
[plugins.'io.containerd.cri.v1.runtime'.containerd.runtimes.kata-clh]
  runtime_type = 'io.containerd.kata.v2'
  sandboxer = 'podsandbox'
  privileged_without_host_devices = false
  [plugins.'io.containerd.cri.v1.runtime'.containerd.runtimes.kata-clh.options]
    ConfigPath = '/opt/kata/share/defaults/kata-containers/configuration-clh.toml'
```

containerd `1.7` still uses `io.containerd.grpc.v1.cri` as the CRI plugin name and cannot directly use the containerd `2.x` configuration above. containerd `1.7` uses the following configuration:

```toml
[plugins.'io.containerd.grpc.v1.cri'.containerd.runtimes.kata-clh]
  runtime_type = 'io.containerd.kata.v2'
  privileged_without_host_devices = false
  [plugins.'io.containerd.grpc.v1.cri'.containerd.runtimes.kata-clh.options]
    ConfigPath = '/opt/kata/share/defaults/kata-containers/configuration-clh.toml'
```

After modifying the containerd configuration, run the following commands to restart containerd and check the runtime configuration.

```bash
systemctl restart containerd
crictl info | grep -A20 kata-clh
```

## Compiling the Plugin

The following example uses `kunpeng-tap-pcore-binding:latest`, which is consistent with the default image name in the repository DaemonSet manifest and does not include a personal image repository prefix. Run the following command in the repository root directory:

```bash
make kunpeng-tap-pcore-binding-docker-build
```

If the test node cannot pull the image from the image repository, you can first save the image on the build node.

```bash
docker save -o kunpeng-tap-pcore-binding.tar kunpeng-tap-pcore-binding:latest
```

After copying the image file to the target node, import it into the `k8s.io` namespace of containerd on the target node.

```bash
ctr -n k8s.io images import kunpeng-tap-pcore-binding.tar
```

If you use a public or enterprise image repository, change both the build tag and the image address in `config/kunpeng-tap-pcore-binding/daemonset.yaml` to the actual repository address, and confirm that kubelet can pull the image. Before deployment, run the following command to check the image name used by the DaemonSet.

```bash
grep -n 'image:' config/kunpeng-tap-pcore-binding/daemonset.yaml
```

## Deploying the Plugin

DaemonSet is the default deployment form. One plugin Pod runs on each node, and it only performs local convergence on the Kata Pod cgroups of that node.

Run the following commands to deploy the plugin.

```bash
kubectl apply -f config/kunpeng-tap-pcore-binding/daemonset.yaml
kubectl rollout status daemonset/kunpeng-tap-pcore-binding -n kunpeng-tap-pcore-binding --timeout=180s
```

Run the following commands to check the plugin status.

```bash
kubectl get pod -n kunpeng-tap-pcore-binding -l app=kunpeng-tap-pcore-binding
kubectl logs -n kunpeng-tap-pcore-binding -l app=kunpeng-tap-pcore-binding --since=10m
```

The default manifest adopts a least-privilege configuration:

- `privileged` is disabled.
- Privilege escalation is disabled.
- All Linux capabilities are dropped.
- A read-only root file system is used.
- The service account token is not automatically mounted.
- `/var/run/nri` and `/sys/devices/system/cpu` are mounted as read-only.
- Only `/sys/fs/cgroup/cpuset` is mounted as a writable hostPath.

### Setting DaemonSet Parameters

The main parameters are located in container `args` of `config/kunpeng-tap-pcore-binding/daemonset.yaml`.

```yaml
args:
- --nri-socket-path=/var/run/nri/nri.sock
- --scan-interval=10s
- --namespace-whitelist=default
- --runtimeclass-whitelist=kata,kata-clh
- --dry-run=true
```

**Table 4** DaemonSet parameter description

| Parameter | Default Value | Description |
| --- | --- | --- |
| `--nri-socket-path` | `/var/run/nri/nri.sock` | containerd NRI socket path. |
| `--scan-interval` | `10s` | Interval for the background scan and convergence loop. An NRI event also triggers an asynchronous convergence. |
| `--cgroup-root` | Empty | cpuset cgroup root path. If it is left empty, the plugin automatically discovers it from `/proc/self/mountinfo`. |
| `--namespace-whitelist` | `default` | Only Pods under these namespaces are processed. Multiple values are separated by commas (,). |
| `--runtimeclass-whitelist` | `kata` | Only Pods with these RuntimeClass/runtime handlers are processed. Multiple values are separated by commas (,). |
| `--dry-run` | `false` | When this parameter is set to `true`, only the plan is printed and `cpuset.cpus` is not written. This parameter is set to `true` for the repository DaemonSet manifest by default to facilitate first-time deployment verification. |

For first-time deployment, you are advised to keep `--dry-run=true`. Switch to actual writing only after confirming that the plugin can register with NRI properly.

```bash
kubectl patch ds kunpeng-tap-pcore-binding -n kunpeng-tap-pcore-binding --type=json \
  -p='[{"op":"replace","path":"/spec/template/spec/containers/0/args/4","value":"--dry-run=false"}]'
kubectl rollout status daemonset/kunpeng-tap-pcore-binding -n kunpeng-tap-pcore-binding --timeout=180s
```

If only `kata-clh` needs to be processed, you can run the following commands to modify the RuntimeClass trustlist.

```bash
kubectl patch ds kunpeng-tap-pcore-binding -n kunpeng-tap-pcore-binding --type=json \
  -p='[{"op":"replace","path":"/spec/template/spec/containers/0/args/3","value":"--runtimeclass-whitelist=kata-clh"}]'
```

## Plugin Usage

### Creating Test Pods

Run the following commands to create regression test Pods with CPU limits of 1, 2, and 4 cores, respectively.

```bash
kubectl apply -f config/kunpeng-tap-pcore-binding/test-pods-cpu-limit.yaml
kubectl wait --for=condition=Ready pod -l app=kata-clh-cpuset-limit-test -n default --timeout=300s
```

Only the `kata-clh-cpuset-limit-2` Pod should be converged onto an SMT sibling pair. This Pod has a CPU request of 1 core, which is used to confirm that the request does not participate in filtering. The Pods with CPU limits of 1 core and 4 cores should retain their original `cpuset.cpus`.

Run the following commands to create two cloud-hypervisor test Pods.

```bash
kubectl apply -f config/kunpeng-tap-pcore-binding/test-pods-cloud-hypervisor.yaml
kubectl wait --for=condition=Ready pod -l app=kata-clh-cpuset-test -n default --timeout=300s
```

Run the following commands to create a scale test with 100 replicas.

```bash
kubectl apply -f config/kunpeng-tap-pcore-binding/test-deployment-cloud-hypervisor-scale.yaml
kubectl rollout status deployment/kata-clh-cpuset-scale -n default --timeout=900s
```

Run the following commands to confirm that no write failures appear in the plugin logs.

```bash
kubectl logs -n kunpeng-tap-pcore-binding -l app=kunpeng-tap-pcore-binding --since=10m | \
  grep -E 'Write pod cpuset failed|Resolve pod cgroup path failed|No free sibling|broken pipe|failed sending|panic|Error' || true
```

### Checking the Binding Result

#### Viewing the cpuset Range

Run the following commands on the node to directly view the current `cpuset.cpus` range for each test Pod:

```bash
ns=default
selector=app=kata-clh-cpuset-scale
root=/sys/fs/cgroup/cpuset

kubectl get pods -n "$ns" -l "$selector" -o jsonpath='{range .items[*]}{.metadata.name}{"\n"}{end}' | sort | while read -r pod; do
  uid=$(kubectl get pod "$pod" -n "$ns" -o jsonpath='{.metadata.uid}' | tr - _)
  values=$(find "$root" -path "*pod${uid}.slice/cpuset.cpus" -exec cat {} \; | sort -u | paste -sd, -)
  printf '%s %s\n' "$pod" "${values:-NONE}"
done
```

Example output:

```text
kata-clh-cpuset-scale-56b8466877-254rp 0-1
kata-clh-cpuset-scale-56b8466877-262nc 2-3
kata-clh-cpuset-scale-56b8466877-26kwr 4-5
```

Run the following commands to view the parent cgroup and sandbox cgroup files of a single Pod. The script uses the Pod UID to precisely match the corresponding sandbox.

```bash
ns=default
pod=kata-clh-cpuset-limit-2
root=/sys/fs/cgroup/cpuset

pod_uid=$(kubectl get pod "$pod" -n "$ns" -o jsonpath='{.metadata.uid}')
uid=$(printf '%s' "$pod_uid" | tr - _)
sid=$(crictl pods -q --label "io.kubernetes.pod.uid=${pod_uid}" --state Ready)
test -n "$sid" || { printf 'Ready sandbox not found for %s\n' "$pod" >&2; exit 1; }

find "$root" \( \
  -path "*pod${uid}.slice/cpuset.cpus" -o \
  -path "*pod${uid}.slice*${sid}*/cpuset.cpus" \
\) -print | sort | while read -r file; do
  printf '%s = ' "$file"
  cat "$file"
done
```

In the output, if both the parent cgroup and sandbox cgroup show the same CPU range (for example, `0-1`), it indicates that the Pod has been converged to the corresponding sibling pair.

#### Checking Duplicate Binding Among 100 Pods

The following script reads the Pod parent cgroup and the Kata sandbox cgroup of each test Pod, checks whether the two are consistent, and counts duplicate sibling pair bindings across Pods.

```bash
ns=default
selector=app=kata-clh-cpuset-scale
root=/sys/fs/cgroup/cpuset
out=/tmp/kata-cpuset-scale-results.txt
: > "$out"

kubectl get pods -n "$ns" -l "$selector" -o jsonpath='{range .items[*]}{.metadata.name}{"\n"}{end}' | sort | while read -r pod; do
  pod_uid=$(kubectl get pod "$pod" -n "$ns" -o jsonpath='{.metadata.uid}')
  uid=$(printf '%s' "$pod_uid" | tr - _)
  sid=$(crictl pods -q --label "io.kubernetes.pod.uid=${pod_uid}" --state Ready)
  sid=${sid:-NONE}
  parent_file=$(find "$root" -path "*pod${uid}.slice/cpuset.cpus" -print | sort | head -n 1)
  sandbox_file=
  if [ "$sid" != NONE ]; then
    sandbox_file=$(find "$root" -path "*pod${uid}.slice*${sid}*/cpuset.cpus" -print | sort | head -n 1)
  fi
  parent=$(cat "$parent_file" 2>/dev/null || printf NONE)
  sandbox=$(cat "$sandbox_file" 2>/dev/null || printf NONE)
  printf '%s %s %s %s\n' "$pod" "$sid" "$parent" "$sandbox" >> "$out"
done

awk '
BEGIN { bad=0 }
{
  pods++
  parent=$3
  sandbox=$4
  if (parent == "NONE" || sandbox == "NONE") { print "missing_cgroup", $1, parent, sandbox; bad++ }
  if (parent != sandbox) { print "mismatch", $1, parent, sandbox; bad++ }
  pair_count[parent]++
}
END {
  dup=0
  for (p in pair_count) {
    if (p != "NONE") unique++
    if (p != "NONE" && pair_count[p] > 1) {
      print "duplicate_pair", p, pair_count[p]
      dup++
    }
  }
  print "pods", pods
  print "unique_pairs", unique+0
  print "duplicate_pairs", dup
  print "bad_records", bad
}' "$out"
```

Expected output for a successful 100-Pod test:

```text
pods 100
unique_pairs 100
duplicate_pairs 0
bad_records 0
```

## (Optional) Plugin Uninstallation

Run the following commands to clear the test workloads:

```bash
kubectl delete -f config/kunpeng-tap-pcore-binding/test-deployment-cloud-hypervisor-scale.yaml --ignore-not-found
kubectl delete -f config/kunpeng-tap-pcore-binding/test-pods-cloud-hypervisor.yaml --ignore-not-found
kubectl delete -f config/kunpeng-tap-pcore-binding/test-pods-cpu-limit.yaml --ignore-not-found
```

Run the following command to uninstall the plugin:

```bash
kubectl delete -f config/kunpeng-tap-pcore-binding/daemonset.yaml
```

## Change History

| Version | Date | Description |
| --- | --- | --- |
| 01 | 2026-09-30 | This is the first official release. |
