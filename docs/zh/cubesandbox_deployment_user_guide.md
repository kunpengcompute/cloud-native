# CubeSandbox部署用户指南

## 简介

[CubeSandbox](https://github.com/TencentCloud/CubeSandbox)是基于RustVMM和KVM的沙箱系统。在鲲鹏服务器上部署时，CubeSandbox使用鲲鹏处理器提供的ARM64虚拟化能力运行MicroVM。

CubeSandbox从v0.5.0开始提供ARM64全栈支持。本文以已在ARM64环境完成验证的CubeSandbox v0.5.1为例，其对应的Python SDK包版本为0.5.0。除非特别说明，本文后续安装和测试命令均使用该版本组合。

本指南面向鲲鹏950处理器，介绍CubeSandbox的部署、模板制作和基础验证过程。

1. 确认ARM64、KVM、XFS和基础软件版本满足要求。
2. 安装CubeSandbox并检查核心服务。
3. 制作ARM64模板，验证Sandbox和快照功能。
4. 了解KVM irqbypass优化补丁和vNMI兼容限制。

## 环境要求

**表 1** 环境要求

| 项目 | 要求 |
| --- | --- |
| 处理器 | 鲲鹏950处理器 |
| CPU架构 | aarch64 |
| 虚拟化 | 宿主机已启用ARM64 KVM，并存在/dev/kvm |
| 操作系统 | openEuler 24.03 LTS SP3或满足本文要求的兼容ARM64操作系统 |
| 文件系统 | /data/cubelet使用支持reflink的XFS文件系统 |
| 磁盘空间 | /data/cubelet至少预留50 GB；制作多个模板时建议预留200 GB |
| ARM64支持下限 | CubeSandbox v0.5.0 |
| 已验证CubeSandbox版本 | v0.5.1 ARM64 release安装包 |
| 已验证Python SDK版本 | cubesandbox==0.5.0 |
| 已验证Docker版本 | 25.0.3 |
| 已验证Docker Compose版本 | v2.30.3 |
| 功能验证工具 | Python 3 |
| 可选性能评估工具 | cube-bench；构建时需要Go 1.25或以上版本 |

执行以下步骤检查处理器架构、KVM、glibc和文件系统。

1. 检查处理器架构和型号。

   ```bash
   lscpu | grep -E 'Architecture|Vendor ID|Model name'
   ```

   期望`Architecture`为`aarch64`。处理器型号应与服务器资产信息一致，并确认使用鲲鹏950处理器。

2. 检查KVM设备。

   ```bash
   test -c /dev/kvm && echo '/dev/kvm is ready'
   ```

   期望输出`/dev/kvm is ready`。无输出表示`/dev/kvm`不存在或不是字符设备，不能继续安装CubeSandbox。

3. 检查KVM模块。

   ```bash
   lsmod | grep kvm
   ```

   KVM以模块方式提供时，期望看到`kvm`相关模块。KVM编译进内核时可能无输出，此时以`/dev/kvm`存在且可访问为最终判断依据。

4. 检查glibc版本。

   ```bash
   getconf GNU_LIBC_VERSION
   ```

   期望输出类似`glibc 2.38`，版本不得低于2.31。

5. 检查`/data/cubelet`文件系统类型。

   ```bash
   findmnt -no FSTYPE /data/cubelet
   ```

   期望输出`xfs`。无输出表示该路径尚未挂载；输出其他类型表示不满足CubeSandbox存储要求。

6. 检查XFS reflink能力。

   ```bash
   xfs_info /data/cubelet | grep 'reflink=1'
   ```

   期望输出包含`reflink=1`。无输出表示XFS未启用reflink，模板和快照功能不能正常使用。

## 注意事项

- 只有目标内核已经合入ARM64 vNMI系列时，才需要检查是否合入PR #27059对应修复；未合入vNMI系列的内核不受该问题影响。
- 当前CubeSandbox不支持ARM64 vNMI状态的64位保存和恢复，不得为Guest主动启用vNMI。
- 生产环境启用前，应在测试节点完成安装、模板、Sandbox和快照验证。

## 安装CubeSandbox

鲲鹏950服务器使用原生ARM64 KVM，不安装仅面向x86_64的PVM宿主机内核。本文使用已完成ARM64验证的CubeSandbox v0.5.1 ARM64 release安装包，并以root用户执行安装。

### 安装前检查

先完成[环境要求](#环境要求)中的架构、KVM、glibc和XFS检查，再执行以下补充检查。

#### 检查物理内存

```bash
awk '/MemTotal/ {printf "memory: %.1f GiB\n", $2/1024/1024}' /proc/meminfo
```

期望内存不少于8 GiB。低于该容量时安装器会终止，且高并发测试也无法得到有效结果。

#### 检查数据盘空间

```bash
df -h /data/cubelet
```

期望可用空间不少于50 GB；制作多个模板或执行高并发测试时建议不少于200 GB。

#### 检查内核是否支持bpffs

```bash
grep -w bpf /proc/filesystems
```

期望输出包含`bpf`。无输出表示内核未启用bpf文件系统，CubeVS无法运行。

#### 检查bpffs挂载状态

```bash
findmnt -no FSTYPE /sys/fs/bpf
```

期望输出`bpf`。无输出或输出其他类型时，执行`mount -t bpf bpf /sys/fs/bpf`完成挂载。

#### 检查cgroup类型和CPU控制器

```bash
stat -fc %T /sys/fs/cgroup
```

使用cgroup v2时，期望输出`cgroup2fs`。使用cgroup v1时会输出其他文件系统类型，由安装器通过v1接口继续检查。

使用cgroup v2时继续检查控制器。

```bash
cat /sys/fs/cgroup/cgroup.controllers
```

期望输出包含`cpu`。使用cgroup v1时不执行该命令。

#### 检查Docker

```bash
docker version --format '{{.Server.Version}}'
```

期望输出`25.0.3`。

```bash
docker info
```

期望命令正常输出Server信息且不包含连接错误。Docker异常会导致控制面依赖和CubeEgress镜像无法启动。

#### 检查Docker Compose

```bash
docker compose version --short
```

期望输出`2.30.3`。

### 规划沙箱网段

CubeSandbox默认使用`192.168.0.0/18`分配沙箱IP。该网段不能与宿主机接口、路由、容器网络或业务网络重叠。安装前检查现有网络。

```bash
ip -4 address show
```

确认宿主机接口地址不在计划使用的沙箱网段内。

```bash
ip -4 route show
```

确认现有路由不覆盖计划使用的沙箱网段。

```bash
docker network ls
```

确认Docker网络未使用相同或重叠的网段。

```bash
ss -lntp | grep -E ':(80|443|3000|3010|3306|6379|9000|9001|12088)\b' || true
```

无输出表示这些默认端口未被占用；有输出时应确认占用服务是否会与CubeSandbox冲突。

如果默认网段冲突，选择未被使用的私有网段，并在安装时设置`CUBE_SANDBOX_NETWORK_CIDR`。掩码范围应为`/16`至`/24`，以下地址仅为示例，使用前仍需检查冲突。

```bash
export CUBE_SANDBOX_NETWORK_CIDR=10.100.0.0/18
```

不要通过`CUBE_SANDBOX_NETWORK_CIDR_SKIP_CONFLICT_CHECK=1`绕过已知冲突。网段冲突通常表现为模板探针超时、Sandbox网络不通或安装预检失败。

### 获取并安装ARM64版本

从CubeSandbox官方release下载ARM64安装包。执行脚本前应审核安装包来源和`install.sh`内容。

```bash
export CUBE_VERSION=v0.5.1
curl -fLO \
  "https://github.com/TencentCloud/CubeSandbox/releases/download/${CUBE_VERSION}/cube-sandbox-one-click-${CUBE_VERSION}-arm64.tar.gz"
```

检查压缩包是否完整。

```bash
tar -tzf "cube-sandbox-one-click-${CUBE_VERSION}-arm64.tar.gz" >/dev/null
```

解压安装包。

```bash
tar -xzf "cube-sandbox-one-click-${CUBE_VERSION}-arm64.tar.gz"
```

进入安装目录并审核安装脚本。

```bash
cd "cube-sandbox-one-click-${CUBE_VERSION}-arm64"
less install.sh
```

执行安装并保存日志。

```bash
set -o pipefail
MIRROR=cn ./install.sh 2>&1 | tee cubesandbox-install.log
```

国内网络使用`MIRROR=cn`选择CubeSandbox组件镜像。如果仍然拉取失败，应先验证Docker DNS、代理、镜像仓库连通性和剩余磁盘空间，再重新执行安装。

### 检查安装结果

安装完成后检查核心服务、监听端口、容器和一键部署健康检查。

```bash
systemctl list-units 'cube-*' --all --no-pager
```

期望CubeSandbox核心单元的`LOAD`为`loaded`、`ACTIVE`为`active`、`SUB`为`running`。出现`failed`、`inactive`或缺少核心服务时，应查看对应单元日志。

```bash
ss -lntp | grep ':3000 '
```

期望CubeAPI监听3000端口。

```bash
docker ps --format 'table {{.Names}}\t{{.Status}}\t{{.Image}}'
```

确认CubeSandbox依赖容器处于运行状态。

```bash
/usr/local/services/cubetoolbox/scripts/one-click/quickcheck.sh
```

核心服务应为`active`，CubeAPI应监听3000端口，健康检查应通过。检查失败时先查看安装日志和对应服务日志。

```bash
tail -n 200 cubesandbox-install.log
```

```bash
journalctl \
  -u cube-sandbox-cube-api.service \
  -u cube-sandbox-cubemaster.service \
  -u cube-sandbox-cubelet.service \
  -n 100 --no-pager
```

```bash
find /data/log/Cubelet /data/log/CubeVmm -maxdepth 2 -type f -print
```

### 常见安装问题

**表 2** CubeSandbox常见安装问题

| 问题 | 典型现象 | 处理方法 |
| --- | --- | --- |
| 安装包架构错误 | 启动组件时报Exec format error | 确认uname -m为aarch64，并下载文件名包含arm64的安装包 |
| KVM不可用 | 预检提示/dev/kvm not found | 使用开启虚拟化能力的物理机或裸金属服务器，检查KVM模块和设备权限 |
| XFS或reflink不满足 | 预检提示not XFS，模板制作失败 | 将独立XFS数据盘挂载到/data/cubelet，并确认xfs_info显示reflink=1 |
| bpffs未挂载 | eBPF或CubeVS初始化失败 | 确认内核支持bpf文件系统并挂载/sys/fs/bpf |
| cgroup CPU控制器不可用 | Cubelet CPU配额初始化失败 | 检查cgroup版本及cgroup.controllers，按发行版要求启用CPU控制器 |
| 沙箱网段冲突 | 安装预检失败、模板探针超时或Sandbox网络不通 | 设置不冲突的CUBE_SANDBOX_NETWORK_CIDR后重新安装 |
| 镜像拉取失败 | Docker返回超时、DNS或TLS错误 | 使用MIRROR=cn，检查Docker DNS、代理、仓库连通性和磁盘空间 |
| 端口占用 | 服务启动失败或监听检查不通过 | 使用ss -lntp定位占用进程，释放端口或按安装配置修改端口 |
| 核心服务未启动 | systemctl is-active不是active | 查看安装日志、journalctl及/data/log/Cubelet、/data/log/CubeVmm |

### 配置SDK环境

安装与CubeSandbox v0.5.1配套的Python SDK 0.5.0，并设置API地址和密钥。

```bash
python3 -m pip install 'cubesandbox==0.5.0'
```

检查SDK版本。

```bash
python3 -c 'from importlib.metadata import version; print(version("cubesandbox"))'
```

期望输出`0.5.0`。

设置SDK连接参数。

```bash
export CUBE_API_URL=http://127.0.0.1:3000
export E2B_API_URL=http://127.0.0.1:3000
export E2B_API_KEY=e2b_000000
```

生产环境应将示例密钥替换为实际密钥，且不得将密钥提交到代码仓库。

## 制作并验证模板

### 创建ARM64模板

`--image`应由用户指定，并且镜像必须支持ARM64。以下命令中的镜像地址只是示例；使用其他镜像时，还应根据镜像实际服务端口调整`--expose-port`和`--probe`。

```bash
export CUBE_TEMPLATE_IMAGE='<your-arm64-oci-image>'
```

创建模板。

```bash
cubemastercli tpl create-from-image \
  --image "$CUBE_TEMPLATE_IMAGE" \
  --writable-layer-size 1G \
  --expose-port 49999 \
  --expose-port 49983 \
  --probe 49999
```

例如，测试代码解释器模板时可以将`CUBE_TEMPLATE_IMAGE`设置为`cube-sandbox-cn.tencentcloudcr.com/cube-sandbox/sandbox-code:latest`。该地址仅用于演示，不是客户环境必须使用的镜像。

记录命令返回的`job_id`，等待冷启动、快照和模板发布完成。

```bash
cubemastercli tpl watch --job-id '<job-id>'
```

任务完成后检查模板状态。

```bash
cubemastercli tpl info --template-id '<template-id>'
```

模板状态应为`READY`，并至少存在一个状态为`READY`的副本。设置后续验证使用的模板ID。

```bash
export CUBE_TEMPLATE_ID='<template-id>'
```

### 验证模板启动

以下脚本使用模板启动Sandbox并执行Python代码。

```bash
python3 - <<'PY'
import os
from cubesandbox import Sandbox

template_id = os.environ["CUBE_TEMPLATE_ID"]
with Sandbox.create(template=template_id, timeout=60) as sandbox:
    result = sandbox.run_code("print('kunpeng-template-start-ok')")
    output = "".join(result.logs.stdout).strip()
    print("sandbox_id:", sandbox.sandbox_id)
    print("output:", output)
    assert output == "kunpeng-template-start-ok"
PY
```

脚本无异常退出且输出`kunpeng-template-start-ok`，表示模板启动、Guest执行和结果返回正常。

### 验证快照启动

快照验证会覆盖ARM64 vGIC状态保存和恢复，是检查vNMI兼容性的必要步骤。

```bash
python3 - <<'PY'
import os
from cubesandbox import Sandbox

template_id = os.environ["CUBE_TEMPLATE_ID"]
restored = None
snapshot_id = None

try:
    with Sandbox.create(template=template_id, timeout=120) as source:
        source.run_code("open('/tmp/kunpeng-marker','w').write('snapshot-ok')")
        snapshot = source.create_snapshot()
        snapshot_id = snapshot.snapshot_id

    restored = Sandbox.create(template=snapshot_id, timeout=60)
    result = restored.run_code("print(open('/tmp/kunpeng-marker').read())")
    output = "".join(result.logs.stdout).strip()
    print("snapshot_id:", snapshot_id)
    print("restored_sandbox_id:", restored.sandbox_id)
    print("output:", output)
    assert output == "snapshot-ok"
finally:
    if restored is not None:
        restored.kill()
    if snapshot_id is not None:
        Sandbox.delete_snapshot(snapshot_id)
PY
```

脚本无异常退出且输出`snapshot-ok`，表示快照创建、恢复和状态继承正常。如果该步骤在vGIC寄存器保存或恢复阶段失败，应检查后文所述的vNMI兼容问题。

## 内核优化与兼容性补丁

### KVM irqbypass优化补丁

#### 优化效果

> **鲲鹏服务器测试结果：** 在鲲鹏服务器单节点、并发数为50、累计启动500个Sandbox的测试中，使用KVM irqbypass XArray优化后，测试记录的Sandbox启动时延从约1100 ms降低至约200 ms。

KVM irqbypass XArray优化可能减少大量Sandbox并发创建或恢复时的宿主机锁等待，从而改善创建或恢复延迟及吞吐量。该优化不改变CubeSandbox API、模板格式或日常操作方式。

实际收益取决于并发规模以及CPU、存储、网络和调度等瓶颈。低并发场景或irqbypass不是主要热点时，性能可能没有明显变化，因此不能将该优化视为固定收益或CubeSandbox部署的前置条件。

#### 基本原理

KVM irqbypass原实现使用全局链表保存producer和consumer。注册或注销对象时，需要在同一个mutex保护下遍历链表，并根据共享token匹配对象；对象数量增多时，遍历和持锁开销会相应增加。

优化补丁使用XArray分别保存producer和consumer，并以共享token作为索引。注册时可直接查找匹配对象，注销时可直接删除对应项，从而减少线性遍历和锁占用时间。补丁不改变现有回调关系和对外接口。

#### 上游和下游补丁

该优化的上游提交为[`8394b32faecd`](https://git.kernel.org/pub/scm/linux/kernel/git/torvalds/linux.git/commit/?id=8394b32faecd9c63b3c436e78e62519e9548e530)，首次包含在Linux v6.17中。

对于尚未包含该优化的Linux 6.6内核，可以直接考虑合入BoostKit提供的[irqbypass XArray补丁](https://gitcode.com/boostkit/cloud-virtual/blob/br_bk26_630/kernel/kernel-6.6.0/irqbypass/%5Birqbypass%5Dkvm-irqbypass-xarray-v2.patch)。

补丁合入方法可参考openEuler [PR #26321](https://gitcode.com/openeuler/kernel/pull/26321)及其提交[`c51917e52d7f`](https://gitcode.com/openeuler/kernel/commit/c51917e52d7f267f75adeeeb747ce967adf6989b)。合入前应由操作系统或内核维护人员确认目标内核基线与补丁适配。

### vNMI兼容问题（可选）

CubeSandbox v0.5.1在ARM64 vGIC快照中以32位数据保存和恢复`ICC_AP1Rn_EL1`。目标内核未合入vNMI系列时只使用低32位，不存在该兼容问题。

如果目标内核合入了[vNMI patchset](https://mailweb.openeuler.org/archives/list/kernel@openeuler.org/thread/ZRW2NMY5DXWIF25JSZRHM7W7XFC3RTQH/)，并在支持FEAT_NMI的宿主机上向Guest启用vNMI，`ICH_AP1Rn_EL2`高32位中的NMI优先级状态可能无法被CubeSandbox完整保存和恢复。

只有确认目标内核包含vNMI系列时，才需要处理该兼容问题。可以直接考虑合入BoostKit提供的[vNMI兼容补丁](https://gitcode.com/boostkit/cloud-virtual/blob/br_bk26_630/kernel/kernel-6.6.0/%5Bkvm%5Darm64-vgic-v3-Restrict-ICH_AP1Rn_EL2-accesses-to-64-bit-only-with-vNMI.patch)。

补丁合入方法可参考openEuler kernel [PR #27059](https://gitcode.com/openeuler/kernel/pull/27059)。使用CubeSandbox v0.5.1时不得为Guest启用vNMI；合入前应由操作系统或内核维护人员确认目标内核已经包含vNMI系列，并审核补丁适配性。

## 故障排除

### 模板或快照启动失败的解决方法

**问题现象：** 模板长时间不是`READY`，或者从模板、快照创建Sandbox失败。

**原因分析：** 常见原因包括ARM64镜像不可用、`/data/cubelet`未启用XFS reflink、KVM不可用、模板探针失败、Cubelet或VMM异常。快照恢复失败还可能是宿主机默认启用vNMI，但CubeSandbox仍按32位保存和恢复`ICC_AP1Rn_EL1`。

**解决方法：** 依次检查`/dev/kvm`、`xfs_info /data/cubelet`、模板任务状态，以及`/data/log/Cubelet/`、`/data/log/CubeVmm/`下的业务日志。修复环境或镜像问题后，模板应进入`READY`状态。如果目标内核包含vNMI系列，vGIC状态错误还需确认内核包含PR #27059对应修复，并确认CubeSandbox Guest没有启用vNMI。

## 参考资料

- [CubeSandbox源码仓库](https://github.com/TencentCloud/CubeSandbox)
- [CubeSandbox 快速开始](https://cubesandbox.com/zh/guide/quickstart.html)
- [CubeSandbox ARM64裸金属部署](https://cubesandbox.com/zh/guide/bare-metal-deploy.html)
- [CubeSandbox v0.5.0 Release](https://github.com/TencentCloud/CubeSandbox/releases/tag/v0.5.0)
- [CubeSandbox v0.5.1 Release](https://github.com/TencentCloud/CubeSandbox/releases/tag/v0.5.1)
- [CubeSandbox v0.5.1 Python SDK版本信息](https://github.com/TencentCloud/CubeSandbox/blob/v0.5.1/sdk/python/pyproject.toml)
- [CubeSandbox 模板概览](https://cubesandbox.com/zh/guide/templates.html)
- [CubeSandbox 快照、回滚与克隆](https://cubesandbox.com/zh/guide/snapshot-rollback-clone.html)
- [CubeSandbox 服务管理与日志](https://cubesandbox.com/zh/guide/service-management.html)
- [CubeSandbox部署排障](https://github.com/TencentCloud/CubeSandbox/blob/v0.5.1/docs/zh/guide/troubleshooting/deployment.md)
- [CubeSandbox沙箱网段冲突排障](https://github.com/TencentCloud/CubeSandbox/blob/v0.5.1/docs/zh/guide/troubleshooting/local-network-cidr-conflict.md)
- [cube-bench 使用说明](https://github.com/TencentCloud/CubeSandbox/tree/v0.5.1/examples/cube-bench)
- [CubeSandbox v0.5.1 ARM64 vGIC状态实现](https://github.com/TencentCloud/CubeSandbox/blob/v0.5.1/hypervisor/hypervisor/src/kvm/aarch64/gic/icc_regs.rs)
- [Linux v6.17 irqbypass XArray提交](https://git.kernel.org/pub/scm/linux/kernel/git/torvalds/linux.git/commit/?id=8394b32faecd9c63b3c436e78e62519e9548e530)
- [BoostKit Linux 6.6 irqbypass XArray补丁](https://gitcode.com/boostkit/cloud-virtual/blob/br_bk26_630/kernel/kernel-6.6.0/irqbypass/%5Birqbypass%5Dkvm-irqbypass-xarray-v2.patch)
- [openEuler irqbypass XArray合入参考PR](https://gitcode.com/openeuler/kernel/pull/26321)
- [openEuler OLK-6.6 irqbypass XArray适配参考提交](https://gitcode.com/openeuler/kernel/commit/c51917e52d7f267f75adeeeb747ce967adf6989b)
- [ARM64 KVM vNMI patchset](https://mailweb.openeuler.org/archives/list/kernel@openeuler.org/thread/ZRW2NMY5DXWIF25JSZRHM7W7XFC3RTQH/)
- [BoostKit ARM64 vNMI兼容补丁](https://gitcode.com/boostkit/cloud-virtual/blob/br_bk26_630/kernel/kernel-6.6.0/%5Bkvm%5Darm64-vgic-v3-Restrict-ICH_AP1Rn_EL2-accesses-to-64-bit-only-with-vNMI.patch)
- [openEuler OLK-6.6 vNMI兼容补丁PR](https://gitcode.com/openeuler/kernel/pull/27059)

## 修订记录

| 文档版本 | 发布日期   | 修改说明         |
| -------- | ---------- | ---------------- |
| 01       | 2026-09-30 | 第一次正式发布。 |
