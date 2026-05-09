# Kata Cpuset NRI 插件用户指南

本文说明如何以 DaemonSet 方式部署和使用 `kata-cpuset-nri`。插件通过 NRI 接入 containerd，对符合白名单条件的 Kata Pod 执行 Pod 级 cpuset 收敛，将 Pod 的 2 个逻辑 CPU 绑定到同一个物理核心的 SMT sibling pair。

本文中的 containerd、Kata、cloud-hypervisor 和 arm64 节点信息是当前已验证的测试条件。用户可在相同或等价条件下按本文步骤完成部署、参数配置和绑定结果检查。

## 1. 功能范围

- 只处理 Pod 层级，不处理 container 层级。
- 目标 Pod 的 CPU request/limit 固定为 `2`。
- 将 Pod 的 2 个逻辑 CPU 绑定到同一个物理核心的 sibling pair。
- 不保存绑定状态；插件每轮从当前 Pod cgroup 的 `cpuset.cpus` 重新计算占用。
- 不允许多个符合条件的 Pod 收敛到同一对 sibling。
- 通过 namespace 和 runtimeClass 白名单限定处理范围。

## 2. 已验证测试条件

当前部署流程已在以下 arm64 测试环境中验证通过：

| 项目 | 测试值 |
| --- | --- |
| 测试节点 | `root@192.168.25.61` |
| CPU 架构 | `aarch64` |
| Kubernetes | `v1.34.7` |
| containerd | `v2.1.7` |
| cgroup | cgroup v1，cpuset 挂载点为 `/sys/fs/cgroup/cpuset` |
| NRI socket | `/var/run/nri/nri.sock` |
| Kata runtime handler | `kata-clh` |
| Kata hypervisor | cloud-hypervisor |
| 验证规模 | 100 个 `runtimeClassName: kata-clh` Pod |

部署前需要确认：

- containerd 已启用 NRI，并生成 `/var/run/nri/nri.sock`。
- Kata Containers 已安装，并配置可用的 runtime handler，例如 `kata-clh`。
- 节点 CPU 开启 SMT，且可读取 `/sys/devices/system/cpu/cpu*/topology/thread_siblings_list`。
- 目标 namespace 和 runtimeClass 已加入插件白名单。
- 目标 Pod 使用 2 CPU request/limit。

## 3. 启用 containerd NRI

containerd v2.1 可通过 `/etc/containerd/config.toml` 启用 NRI。建议先备份配置文件：

```bash
cp /etc/containerd/config.toml /etc/containerd/config.toml.bak
```

确认或添加如下配置：

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

重启 containerd：

```bash
systemctl restart containerd
```

检查 NRI socket：

```bash
test -S /var/run/nri/nri.sock && echo "NRI socket is ready"
```

也可以通过 containerd 配置导出结果确认：

```bash
containerd config dump | grep -n -A8 "io.containerd.nri.v1.nri"
```

## 4. 准备 Kata RuntimeClass

如果测试环境的 QEMU 后端存在 CPU hotplug 限制，可以使用 cloud-hypervisor 后端。仓库提供 `kata-clh` RuntimeClass 清单：

```bash
kubectl apply -f config/kata-cpuset-nri/runtimeclass-cloud-hypervisor.yaml
kubectl get runtimeclass kata-clh
```

containerd 中需要存在与 RuntimeClass handler 同名的 runtime 配置。当前已验证环境使用如下配置形态：

```toml
[plugins.'io.containerd.cri.v1.runtime'.containerd.runtimes.kata-clh]
  runtime_type = 'io.containerd.kata.v2'
  sandboxer = 'podsandbox'
  privileged_without_host_devices = false
  [plugins.'io.containerd.cri.v1.runtime'.containerd.runtimes.kata-clh.options]
    ConfigPath = '/opt/kata/share/defaults/kata-containers/configuration-clh.toml'
```

修改 containerd 配置后需要重启 containerd：

```bash
systemctl restart containerd
crictl info | grep -A20 kata-clh
```

## 5. 编译镜像

在仓库根目录执行：

```bash
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o kata-cpuset-nri ./cmd/kata-cpuset-nri
docker build --platform=linux/arm64 -f Dockerfile.kata-cpuset-nri -t kata-cpuset-nri:latest .
rm -f kata-cpuset-nri
```

如果测试节点无法从镜像仓库拉取镜像，可以直接导入到目标节点 containerd 的 `k8s.io` namespace。以下命令以当前已验证节点为例：

```bash
docker save kata-cpuset-nri:latest | ssh root@192.168.25.61 'ctr -n k8s.io images import -'
```

如果使用镜像仓库，将 `config/kata-cpuset-nri/daemonset.yaml` 中的镜像地址改为仓库地址，并确认 kubelet 可以拉取该镜像。

## 6. DaemonSet 部署

DaemonSet 是默认部署形式。每个节点运行一个插件 Pod，只对本节点的 Kata Pod cgroup 做本地收敛。

部署插件：

```bash
kubectl apply -f config/kata-cpuset-nri/daemonset.yaml
kubectl rollout status daemonset/kata-cpuset-nri -n kata-cpuset-nri --timeout=180s
```

查看插件状态：

```bash
kubectl get pod -n kata-cpuset-nri -l app=kata-cpuset-nri
kubectl logs -n kata-cpuset-nri -l app=kata-cpuset-nri --since=10m
```

默认清单使用较小权限面：

- 不启用 `privileged`。
- 禁止权限提升。
- drop 所有 Linux capabilities。
- 使用只读根文件系统。
- 不自动挂载 service account token。
- 只读挂载 `/var/run/nri` 和 `/sys/devices/system/cpu`。
- 仅将 `/sys/fs/cgroup/cpuset` 作为可写 hostPath 挂载。

## 7. DaemonSet 参数设置

主要参数位于 `config/kata-cpuset-nri/daemonset.yaml` 的 container `args`：

```yaml
args:
- --nri-socket-path=/var/run/nri/nri.sock
- --scan-interval=10s
- --namespace-whitelist=default
- --runtimeclass-whitelist=kata,kata-clh
- --dry-run=true
```

参数说明：

| 参数 | 默认值 | 说明 |
| --- | --- | --- |
| `--nri-socket-path` | `/var/run/nri/nri.sock` | containerd NRI socket 路径。 |
| `--scan-interval` | `10s` | 后台扫描收敛周期。NRI 事件也会触发一次异步收敛。 |
| `--cgroup-root` | 空 | cpuset cgroup 根路径。为空时插件从 `/proc/self/mountinfo` 自动发现。 |
| `--namespace-whitelist` | `default` | 只处理这些 namespace 下的 Pod，多个值用逗号分隔。 |
| `--runtimeclass-whitelist` | `kata` | 只处理这些 runtimeClass/runtime handler 的 Pod，多个值用逗号分隔。 |
| `--dry-run` | `false` | 为 `true` 时只打印计划，不写 `cpuset.cpus`。仓库 DaemonSet 清单默认设为 `true`，用于首次部署验证。 |

首次部署建议保持 `--dry-run=true`，确认插件能正常注册 NRI 后再切换为实际写入：

```bash
kubectl patch ds kata-cpuset-nri -n kata-cpuset-nri --type=json \
  -p='[{"op":"replace","path":"/spec/template/spec/containers/0/args/4","value":"--dry-run=false"}]'
kubectl rollout status daemonset/kata-cpuset-nri -n kata-cpuset-nri --timeout=180s
```

如果只处理 `kata-clh`，可以修改 runtimeClass 白名单：

```bash
kubectl patch ds kata-cpuset-nri -n kata-cpuset-nri --type=json \
  -p='[{"op":"replace","path":"/spec/template/spec/containers/0/args/3","value":"--runtimeclass-whitelist=kata-clh"}]'
```

## 8. 创建测试 Pod

创建 2 个 cloud-hypervisor 测试 Pod：

```bash
kubectl apply -f config/kata-cpuset-nri/test-pods-cloud-hypervisor.yaml
kubectl wait --for=condition=Ready pod -l app=kata-clh-cpuset-test -n default --timeout=300s
```

创建 100 副本规模测试：

```bash
kubectl apply -f config/kata-cpuset-nri/test-deployment-cloud-hypervisor-scale.yaml
kubectl rollout status deployment/kata-clh-cpuset-scale -n default --timeout=900s
```

确认插件日志没有写入失败：

```bash
kubectl logs -n kata-cpuset-nri -l app=kata-cpuset-nri --since=10m | \
  grep -E 'Write pod cpuset failed|Resolve pod cgroup path failed|No free sibling|broken pipe|failed sending|panic|Error' || true
```

## 9. 绑定结果检查

### 9.1 直接查看 cpuset 范围

以下命令在节点上执行，可以直接展示每个测试 Pod 当前观测到的 `cpuset.cpus` 范围值：

```bash
ns=default
selector=app=kata-clh-cpuset-scale
root=/sys/fs/cgroup/cpuset

kubectl get pods -n "$ns" -l "$selector" -o jsonpath='{range .items[*]}{.metadata.name}{"\n"}{end}' | sort | while read -r pod; do
  uid=$(kubectl get pod "$pod" -n "$ns" -o jsonpath='{.metadata.uid}' | tr - _)
  values=$(find "$root" -path "*pod${uid}.slice*/cpuset.cpus" -exec cat {} \; | sort -u | paste -sd, -)
  printf '%s %s\n' "$pod" "${values:-NONE}"
done
```

输出示例：

```text
kata-clh-cpuset-scale-56b8466877-254rp 0-1
kata-clh-cpuset-scale-56b8466877-262nc 2-3
kata-clh-cpuset-scale-56b8466877-26kwr 4-5
```

查看单个 Pod 的 parent cgroup 和 sandbox cgroup 文件：

```bash
pod=kata-clh-cpuset-scale-56b8466877-254rp
uid=$(kubectl get pod "$pod" -n default -o jsonpath='{.metadata.uid}' | tr - _)
find /sys/fs/cgroup/cpuset -path "*pod${uid}.slice*/cpuset.cpus" -print | sort | while read -r file; do
  printf '%s = ' "$file"
  cat "$file"
done
```

输出中如果 parent cgroup 与 sandbox cgroup 均为同一个范围，例如 `0-1`，说明该 Pod 已被收敛到对应 sibling pair。

### 9.2 检查 100 Pod 是否重复绑定

以下脚本读取每个测试 Pod 的 Pod parent cgroup 与 Kata sandbox cgroup，检查二者是否一致，并统计 sibling pair 是否重复：

```bash
ns=default
selector=app=kata-clh-cpuset-scale
root=/sys/fs/cgroup/cpuset
out=/tmp/kata-cpuset-scale-results.txt
: > "$out"

kubectl get pods -n "$ns" -l "$selector" -o jsonpath='{range .items[*]}{.metadata.name}{"\n"}{end}' | sort | while read -r pod; do
  uid=$(kubectl get pod "$pod" -n "$ns" -o jsonpath='{.metadata.uid}' | tr - _)
  sid=$(crictl pods -q --name "$pod" | head -n 1)
  parent_file=$(find "$root" -path "*pod${uid}.slice/cpuset.cpus" -print | sort | head -n 1)
  sandbox_file=$(find "$root" -path "*pod${uid}.slice*${sid}*/cpuset.cpus" -print | sort | head -n 1)
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

100 Pod 测试通过时应看到：

```text
pods 100
unique_pairs 100
duplicate_pairs 0
bad_records 0
```

## 10. 清理

清理测试 workload：

```bash
kubectl delete -f config/kata-cpuset-nri/test-deployment-cloud-hypervisor-scale.yaml --ignore-not-found
kubectl delete -f config/kata-cpuset-nri/test-pods-cloud-hypervisor.yaml --ignore-not-found
```

卸载插件：

```bash
kubectl delete -f config/kata-cpuset-nri/daemonset.yaml
```
