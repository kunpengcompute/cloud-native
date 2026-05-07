# Kata Cpuset NRI 插件用户指南

本文说明如何以 DaemonSet 方式部署 `kata-cpuset-nri`，并在 `containerd v2.1 + kata` 环境中验证 Pod 级物理核心绑定能力。

## 1. 使用场景

`kata-cpuset-nri` 通过 NRI 接入 containerd，对符合白名单条件的 Kata Pod 执行 cpuset 收敛：

- 只处理 Pod 层级，不处理 container 层级。
- 目标 Pod 的 CPU request/limit 固定为 `2`。
- 将 Pod 的 2 个逻辑 CPU 绑定到同一个物理核心的 SMT sibling pair。
- 不保存绑定状态；插件每轮从当前 Pod cgroup 的 `cpuset.cpus` 重新计算占用。
- 不允许多个符合条件的 Pod 收敛到同一对 sibling。

## 2. 已验证测试条件

当前仓库中的部署和验证流程已在以下远端 arm64 环境中通过：

| 项目 | 测试值 |
| --- | --- |
| 远端节点 | `root@192.168.25.61` |
| CPU 架构 | `aarch64` |
| Kubernetes | `v1.34.7` |
| containerd | `v2.1.7` |
| cgroup | cgroup v1，cpuset 挂载点为 `/sys/fs/cgroup/cpuset` |
| NRI socket | `/var/run/nri/nri.sock` |
| Kata runtime handler | `kata-clh` |
| Kata hypervisor | cloud-hypervisor |
| 测试规模 | 100 个 `runtimeClassName: kata-clh` Pod |

测试环境要求：

- containerd 已启用 NRI，并存在 `/var/run/nri/nri.sock`。
- Kata Containers 已安装并配置可用的 runtime handler，例如 `kata-clh`。
- 节点 CPU 开启 SMT，且 `/sys/devices/system/cpu/cpu*/topology/thread_siblings_list` 可读取。
- 目标 namespace 和 runtimeClass 已加入插件白名单。
- 目标 Pod 使用 2 CPU request/limit。

## 3. 编译镜像

在仓库根目录执行：

```bash
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o kata-cpuset-nri ./pkg/kata-cpuset-nri/cmd
docker build --platform=linux/arm64 -f Dockerfile.kata-cpuset-nri -t kata-cpuset-nri:latest .
rm -f kata-cpuset-nri
```

如果远端测试节点没有连接镜像仓库，可以直接导入到远端 containerd 的 `k8s.io` namespace：

```bash
docker save kata-cpuset-nri:latest | ssh root@192.168.25.61 'ctr -n k8s.io images import -'
```

如果使用镜像仓库，将 `config/kata-cpuset-nri/daemonset.yaml` 中的镜像地址改为仓库地址，并在目标节点确认 kubelet 可以拉取该镜像。

## 4. DaemonSet 部署

DaemonSet 是默认部署形式。每个节点运行一个插件 Pod，只对本节点的 Kata Pod cgroup 做本地收敛。

部署清单：

```bash
kubectl apply -f config/kata-cpuset-nri/daemonset.yaml
kubectl rollout status daemonset/kata-cpuset-nri -n kata-cpuset-nri --timeout=180s
```

默认清单使用较小权限面：

- 不启用 `privileged`。
- 禁止权限提升。
- drop 所有 Linux capabilities。
- 使用只读根文件系统。
- 不自动挂载 service account token。
- 只读挂载 `/var/run/nri` 和 `/sys/devices/system/cpu`。
- 仅将 `/sys/fs/cgroup/cpuset` 作为可写 hostPath 挂载。

## 5. DaemonSet 参数设置

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

如果需要修改白名单，例如只处理 `kata-clh`：

```bash
kubectl patch ds kata-cpuset-nri -n kata-cpuset-nri --type=json \
  -p='[{"op":"replace","path":"/spec/template/spec/containers/0/args/3","value":"--runtimeclass-whitelist=kata-clh"}]'
```

## 6. cloud-hypervisor RuntimeClass

在测试环境中，如果 QEMU 后端存在 CPU hotplug 限制，可以使用 cloud-hypervisor 后端。仓库提供 `kata-clh` RuntimeClass 清单：

```bash
kubectl apply -f config/kata-cpuset-nri/runtimeclass-cloud-hypervisor.yaml
kubectl get runtimeclass kata-clh
```

containerd 中需要存在与 RuntimeClass handler 同名的 runtime 配置。测试环境使用的配置形态如下：

```toml
[plugins.'io.containerd.cri.v1.runtime'.containerd.runtimes.kata-clh]
  runtime_type = 'io.containerd.kata.v2'
  sandboxer = 'podsandbox'
  privileged_without_host_devices = false
  [plugins.'io.containerd.cri.v1.runtime'.containerd.runtimes.kata-clh.options]
    ConfigPath = '/opt/kata/share/defaults/kata-containers/configuration-clh.toml'
```

修改 containerd 配置后需重启 containerd，并确认 `crictl info` 中能看到 `kata-clh` runtime handler。

## 7. 功能验证

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

查看插件状态和日志：

```bash
kubectl get pod -n kata-cpuset-nri -l app=kata-cpuset-nri
kubectl logs -n kata-cpuset-nri -l app=kata-cpuset-nri --since=10m
```

确认无写入失败：

```bash
kubectl logs -n kata-cpuset-nri -l app=kata-cpuset-nri --since=10m | \
  grep -E 'Write pod cpuset failed|Resolve pod cgroup path failed|No free sibling|broken pipe|failed sending|panic|Error' || true
```

## 8. 绑定结果检查

以下脚本在节点上读取每个测试 Pod 的 Pod parent cgroup 与 Kata sandbox cgroup，检查二者是否一致，并统计 sibling pair 是否重复：

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

## 9. 清理

清理测试 workload：

```bash
kubectl delete -f config/kata-cpuset-nri/test-deployment-cloud-hypervisor-scale.yaml --ignore-not-found
kubectl delete -f config/kata-cpuset-nri/test-pods-cloud-hypervisor.yaml --ignore-not-found
```

卸载插件：

```bash
kubectl delete -f config/kata-cpuset-nri/daemonset.yaml
```
