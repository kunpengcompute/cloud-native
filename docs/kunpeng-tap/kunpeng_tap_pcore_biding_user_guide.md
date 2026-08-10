# Kunpeng-TAP Pcore Biding插件 用户指南

Kunpeng-TAP Pcore Biding插件是Kunpeng-TAP面向Kata机密容器场景提供的物理核绑定插件。Kunpeng-TAP提供通用的容器CPU、内存等资源拓扑亲和能力，本插件在此基础上针对Kata Pod提供更细粒度的物理核绑定能力，通过NRI接入containerd，将符合白名单条件的Pod的2个逻辑CPU收敛到同一个物理核心的SMT sibling pair。插件可独立部署，不依赖Kunpeng-TAP主程序。

本文说明如何以DaemonSet方式部署和使用`kunpeng-tap-pcore-biding`。文中的containerd、Kata、cloud-hypervisor和arm64节点信息是当前已验证的测试条件。用户可在相同或等价条件下按本文步骤完成部署、参数配置和绑定结果检查。

## 1. 功能范围

- 只处理Pod层级，不处理container层级。
- 只处理聚合CPU limit恰好为`2`的目标Pod，CPU request不参与过滤。
- 将Pod的2个逻辑CPU绑定到同一个物理核心的sibling pair。
- 不保存绑定状态；插件每轮从当前Pod cgroup的`cpuset.cpus`重新计算占用。
- 不允许多个符合条件的Pod收敛到同一对sibling。
- 通过namespace和runtimeClass白名单限定处理范围。

## 2. 已验证测试条件

当前部署流程已在以下arm64测试环境中验证通过：

| 项目 | 测试值 |
| --- | --- |
| 测试节点 | `root@192.168.25.61` |
| CPU架构 | `aarch64` |
| Kubernetes | `v1.34.7` |
| containerd | `v2.1.7` |
| cgroup | cgroup v1，cpuset挂载点为`/sys/fs/cgroup/cpuset` |
| NRI socket | `/var/run/nri/nri.sock` |
| Kata runtime handler | `kata-clh` |
| Kata hypervisor | cloud-hypervisor |
| 验证规模 | 100个`runtimeClassName: kata-clh`的Pod |

部署前需要确认：

- containerd已启用NRI，并生成`/var/run/nri/nri.sock`。
- Kata Containers已安装，并配置可用的runtime handler，例如`kata-clh`。
- 节点CPU开启SMT，且可读取`/sys/devices/system/cpu/cpu*/topology/thread_siblings_list`。
- 目标namespace和runtimeClass已加入插件白名单。
- 目标Pod的聚合CPU limit为2核；CPU request可按业务需要设置，但不能大于limit。

## 3. 启用containerd NRI

containerd v2.1可通过`/etc/containerd/config.toml`启用NRI。建议先备份配置文件：

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

重启containerd：

```bash
systemctl restart containerd
```

检查NRI socket：

```bash
test -S /var/run/nri/nri.sock && echo "NRI socket is ready"
```

也可以通过containerd配置导出结果确认：

```bash
containerd config dump | grep -n -A8 "io.containerd.nri.v1.nri"
```

## 4. 准备Kata RuntimeClass

如果测试环境的QEMU后端存在CPU hotplug限制，可以使用cloud-hypervisor后端。仓库提供`kata-clh` RuntimeClass清单：

```bash
kubectl apply -f config/kunpeng-tap-pcore-biding/runtimeclass-cloud-hypervisor.yaml
kubectl get runtimeclass kata-clh
```

containerd中需要存在与RuntimeClass handler同名的runtime配置。containerd 2.x的CRI插件拆分为`io.containerd.cri.v1.runtime`，当前已验证的containerd 2.1环境使用如下配置：

```toml
[plugins.'io.containerd.cri.v1.runtime'.containerd.runtimes.kata-clh]
  runtime_type = 'io.containerd.kata.v2'
  sandboxer = 'podsandbox'
  privileged_without_host_devices = false
  [plugins.'io.containerd.cri.v1.runtime'.containerd.runtimes.kata-clh.options]
    ConfigPath = '/opt/kata/share/defaults/kata-containers/configuration-clh.toml'
```

containerd 1.7仍使用`io.containerd.grpc.v1.cri`作为CRI插件名称，不能直接使用上述containerd 2.x配置。containerd 1.7使用如下配置：

```toml
[plugins.'io.containerd.grpc.v1.cri'.containerd.runtimes.kata-clh]
  runtime_type = 'io.containerd.kata.v2'
  privileged_without_host_devices = false
  [plugins.'io.containerd.grpc.v1.cri'.containerd.runtimes.kata-clh.options]
    ConfigPath = '/opt/kata/share/defaults/kata-containers/configuration-clh.toml'
```

修改containerd配置后需要重启containerd：

```bash
systemctl restart containerd
crictl info | grep -A20 kata-clh
```

## 5. 编译镜像

在仓库根目录执行：

```bash
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o kunpeng-tap-pcore-biding ./cmd/kunpeng-tap-pcore-biding
docker build --platform=linux/arm64 -f Dockerfile.kunpeng-tap-pcore-biding -t kunpeng-tap-pcore-biding:latest .
rm -f kunpeng-tap-pcore-biding
```

如果测试节点无法从镜像仓库拉取镜像，可以直接导入到目标节点containerd的`k8s.io` namespace。以下命令以当前已验证节点为例：

```bash
docker save kunpeng-tap-pcore-biding:latest | ssh root@192.168.25.61 'ctr -n k8s.io images import -'
```

如果使用镜像仓库，将`config/kunpeng-tap-pcore-biding/daemonset.yaml`中的镜像地址改为仓库地址，并确认kubelet可以拉取该镜像。

## 6. DaemonSet部署

DaemonSet是默认部署形式。每个节点运行一个插件Pod，只对本节点的Kata Pod cgroup做本地收敛。

部署插件：

```bash
kubectl apply -f config/kunpeng-tap-pcore-biding/daemonset.yaml
kubectl rollout status daemonset/kunpeng-tap-pcore-biding -n kunpeng-tap-pcore-biding --timeout=180s
```

查看插件状态：

```bash
kubectl get pod -n kunpeng-tap-pcore-biding -l app=kunpeng-tap-pcore-biding
kubectl logs -n kunpeng-tap-pcore-biding -l app=kunpeng-tap-pcore-biding --since=10m
```

默认清单使用较小权限面：

- 不启用`privileged`。
- 禁止权限提升。
- drop所有Linux capabilities。
- 使用只读根文件系统。
- 不自动挂载service account token。
- 只读挂载`/var/run/nri`和`/sys/devices/system/cpu`。
- 仅将`/sys/fs/cgroup/cpuset`作为可写hostPath挂载。

## 7. DaemonSet参数设置

主要参数位于`config/kunpeng-tap-pcore-biding/daemonset.yaml`的container `args`：

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
| `--nri-socket-path` | `/var/run/nri/nri.sock` | containerd NRI socket路径。 |
| `--scan-interval` | `10s` | 后台扫描收敛周期。NRI事件也会触发一次异步收敛。 |
| `--cgroup-root` | 空 | cpuset cgroup根路径。为空时插件从`/proc/self/mountinfo`自动发现。 |
| `--namespace-whitelist` | `default` | 只处理这些namespace下的Pod，多个值用逗号分隔。 |
| `--runtimeclass-whitelist` | `kata` | 只处理这些runtimeClass/runtime handler的Pod，多个值用逗号分隔。 |
| `--dry-run` | `false` | 为`true`时只打印计划，不写`cpuset.cpus`。仓库DaemonSet清单默认设为`true`，用于首次部署验证。 |

首次部署建议保持`--dry-run=true`，确认插件能正常注册NRI后再切换为实际写入：

```bash
kubectl patch ds kunpeng-tap-pcore-biding -n kunpeng-tap-pcore-biding --type=json \
  -p='[{"op":"replace","path":"/spec/template/spec/containers/0/args/4","value":"--dry-run=false"}]'
kubectl rollout status daemonset/kunpeng-tap-pcore-biding -n kunpeng-tap-pcore-biding --timeout=180s
```

如果只处理`kata-clh`，可以修改runtimeClass白名单：

```bash
kubectl patch ds kunpeng-tap-pcore-biding -n kunpeng-tap-pcore-biding --type=json \
  -p='[{"op":"replace","path":"/spec/template/spec/containers/0/args/3","value":"--runtimeclass-whitelist=kata-clh"}]'
```

## 8. 创建测试Pod

创建CPU limit分别为1、2、4核的回归测试Pod：

```bash
kubectl apply -f config/kunpeng-tap-pcore-biding/test-pods-cpu-limit.yaml
kubectl wait --for=condition=Ready pod -l app=kata-clh-cpuset-limit-test -n default --timeout=300s
```

只有`kata-clh-cpuset-limit-2`应被收敛到一个SMT sibling pair。该Pod的CPU request为1核，用于确认request不参与过滤；limit为1核和4核的Pod应保持原有`cpuset.cpus`。

创建2个cloud-hypervisor测试Pod：

```bash
kubectl apply -f config/kunpeng-tap-pcore-biding/test-pods-cloud-hypervisor.yaml
kubectl wait --for=condition=Ready pod -l app=kata-clh-cpuset-test -n default --timeout=300s
```

创建100副本规模测试：

```bash
kubectl apply -f config/kunpeng-tap-pcore-biding/test-deployment-cloud-hypervisor-scale.yaml
kubectl rollout status deployment/kata-clh-cpuset-scale -n default --timeout=900s
```

确认插件日志没有写入失败：

```bash
kubectl logs -n kunpeng-tap-pcore-biding -l app=kunpeng-tap-pcore-biding --since=10m | \
  grep -E 'Write pod cpuset failed|Resolve pod cgroup path failed|No free sibling|broken pipe|failed sending|panic|Error' || true
```

## 9. 绑定结果检查

### 9.1 直接查看cpuset范围

以下命令在节点上执行，可以直接展示每个测试Pod当前观测到的`cpuset.cpus`范围值：

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

查看单个Pod的parent cgroup和sandbox cgroup文件：

```bash
pod=kata-clh-cpuset-scale-56b8466877-254rp
uid=$(kubectl get pod "$pod" -n default -o jsonpath='{.metadata.uid}' | tr - _)
find /sys/fs/cgroup/cpuset -path "*pod${uid}.slice*/cpuset.cpus" -print | sort | while read -r file; do
  printf '%s = ' "$file"
  cat "$file"
done
```

输出中如果parent cgroup与sandbox cgroup均为同一个范围，例如`0-1`，说明该Pod已被收敛到对应sibling pair。

### 9.2 检查100个Pod是否重复绑定

以下脚本读取每个测试Pod的Pod parent cgroup与Kata sandbox cgroup，检查二者是否一致，并统计sibling pair是否重复：

```bash
ns=default
selector=app=kata-clh-cpuset-scale
root=/sys/fs/cgroup/cpuset
out=/tmp/kata-cpuset-scale-results.txt
: > "$out"

kubectl get pods -n "$ns" -l "$selector" -o jsonpath='{range .items[*]}{.metadata.name}{"\n"}{end}' | sort | while read -r pod; do
  uid=$(kubectl get pod "$pod" -n "$ns" -o jsonpath='{.metadata.uid}' | tr - _)
  sid=$(crictl pods -q --name "^${pod}$" --namespace "$ns" --state Ready | head -n 1)
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

100个Pod测试通过时应看到：

```text
pods 100
unique_pairs 100
duplicate_pairs 0
bad_records 0
```

## 10. 清理

清理测试workload：

```bash
kubectl delete -f config/kunpeng-tap-pcore-biding/test-deployment-cloud-hypervisor-scale.yaml --ignore-not-found
kubectl delete -f config/kunpeng-tap-pcore-biding/test-pods-cloud-hypervisor.yaml --ignore-not-found
kubectl delete -f config/kunpeng-tap-pcore-biding/test-pods-cpu-limit.yaml --ignore-not-found
```

卸载插件：

```bash
kubectl delete -f config/kunpeng-tap-pcore-biding/daemonset.yaml
```
