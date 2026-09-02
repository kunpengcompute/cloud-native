# CubeSandbox鲲鹏适配和优化 用户指南

## 简介

[CubeSandbox](https://github.com/TencentCloud/CubeSandbox)是基于RustVMM和KVM的沙箱系统。在鲲鹏服务器上部署时，CubeSandbox使用鲲鹏处理器提供的ARM64虚拟化能力运行MicroVM。

CubeSandbox从v0.5.0开始提供ARM64全栈支持。本文原始验证使用v0.5.1；面向客户部署时推荐使用当前最新稳定版v0.7.0。除非特别说明，本文后续安装和测试命令均以v0.7.0为例。复现历史验证结果时，应使用CubeSandbox平台v0.5.1及其对应的Python SDK包版本v0.5.0。

本指南面向鲲鹏950处理器，介绍以下适配和优化过程。

1. 确认ARM64、KVM、XFS和内核版本满足要求。
2. 安装CubeSandbox，制作ARM64模板并记录优化前基线。
3. 准备匹配的内核源码，检查vNMI系列并按需处理兼容问题。
4. 确认KVM irqbypass XArray版本，按需回合明确的上游XArray提交并安装目标内核。
5. 执行同参数性能复测，对比优化前后结果。

KVM irqbypass原实现使用全局链表保存producer和consumer，注册时需要在同一个mutex保护下遍历对象。XArray优化将线性遍历改为按eventfd直接查找，主要降低大量Sandbox并发启动或恢复时的宿主机锁等待，不改变CubeSandbox API和模板格式。

## 环境要求

**表 1** 环境要求

| 项目 | 要求 |
| --- | --- |
| 处理器 | 鲲鹏950处理器 |
| CPU架构 | aarch64 |
| 虚拟化 | 宿主机已启用ARM64 KVM，并存在/dev/kvm |
| 操作系统 | openEuler 24.03 LTS SP3或兼容OLK-6.6内核构建流程的操作系统 |
| 文件系统 | /data/cubelet使用支持reflink的XFS文件系统 |
| 磁盘空间 | /data/cubelet至少预留50 GB；制作多个模板时建议预留200 GB |
| 内核源码 | 与目标宿主机版本匹配的openEuler kernel源码 |
| ARM64支持下限 | CubeSandbox v0.5.0 |
| 已验证版本 | CubeSandbox v0.5.1，Python SDK v0.5.0 |
| 推荐版本 | CubeSandbox v0.7.0 ARM64 release安装包，Python SDK v0.7.0 |
| 验证工具 | Python 3、cubesandbox==0.7.0、Go 1.21或以上版本、cube-bench、jq，以及可选的perf |

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

7. （可选）检查锁竞争追踪点。

   ```bash
   perf list 'lock:*' | grep -E 'contention_(begin|end)'
   ```

   期望输出`lock:contention_begin`和`lock:contention_end`。该检查仅用于辅助分析性能变化原因；无输出不影响使用`cube-bench`验证性能提升。

## 注意事项

- 只有目标内核已经合入ARM64 vNMI系列时，才需要检查是否合入PR #27059对应修复；未合入vNMI系列的内核不受该问题影响。
- vNMI兼容修复和irqbypass性能优化解决的问题不同，不能相互替代。
- 修改内核前，应检查目标源码是否已包含对应提交，避免重复合入。
- irqbypass XArray提交首次合入Linux v6.17，低于该版本的内核应检查并按需回合上游提交`8394b32faecd`。
- 当前CubeSandbox不支持ARM64 vNMI状态的64位保存和恢复，不得为Guest主动启用vNMI。
- 补丁主要改善高并发场景，低并发或单实例固定开销不一定有明显变化。
- 应在同一台服务器上使用相同模板、并发度和请求数进行优化前后对比。
- 安装新内核前，应保留至少一个已知可启动的旧内核并记录回退启动项。
- 生产环境启用前，应在测试节点完成功能、性能和回退验证。
- 启用 Secure Boot 或内核模块签名时，应遵循组织的内核签名流程。

## 安装CubeSandbox

鲲鹏950服务器使用原生ARM64 KVM，不安装仅面向x86_64的PVM宿主机内核。推荐使用CubeSandbox v0.7.0 ARM64 release安装包，并以root用户执行安装。

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
docker info
```

期望命令正常输出Server信息且不包含连接错误。Docker异常会导致控制面依赖和CubeEgress镜像无法启动。

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
export CUBE_VERSION=v0.7.0
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

安装与平台版本一致的CubeSandbox Python SDK，并设置API地址和密钥。

```bash
python3 -m pip install 'cubesandbox==0.7.0'
```

检查SDK版本。

```bash
python3 -c 'from importlib.metadata import version; print(version("cubesandbox"))'
```

设置SDK连接参数。

```bash
export CUBE_API_URL=http://127.0.0.1:3000
export E2B_API_URL=http://127.0.0.1:3000
export E2B_API_KEY=e2b_000000
```

生产环境应将示例密钥替换为实际密钥，且不得将密钥提交到代码仓库。

## 制作模板并验证基线

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

脚本无异常退出且输出`snapshot-ok`，表示快照创建、恢复和状态继承正常。如果该步骤在vGIC寄存器保存或恢复阶段失败，应先处理vNMI兼容问题，不要继续性能测试。

### 构建压测工具

使用与CubeSandbox部署版本相同的tag构建`cube-bench`。v0.7.0 Makefile的默认目标是`build`，执行`make`和`make build`效果相同；本文使用显式目标`make build`。

```bash
git clone --depth 1 --branch v0.7.0 \
  https://github.com/TencentCloud/CubeSandbox.git ~/CubeSandbox
```

进入工具目录并构建。

```bash
cd ~/CubeSandbox/examples/cube-bench
```

```bash
make build
```

检查构建产物。

```bash
test -x ./bin/cube-bench
```

```bash
./bin/cube-bench --help 2>&1 | head
```

### 采集优化前基线

用于基线的内核不能包含irqbypass XArray实现。如果内核已合入vNMI系列但缺少兼容修复，应先阅读[vNMI兼容问题（可选）](#vnmi兼容问题可选)，构建仅含vNMI修复的内核，然后返回本节采集基线。未合入vNMI系列时无需该修复。

```bash
uname -r | tee irqbypass-before-kernel.txt
```

```bash
rpm -qa 'kernel*' | sort | tee irqbypass-before-rpms.txt
```

采集优化前性能数据。

```bash
./bin/cube-bench \
  --api-url "$E2B_API_URL" \
  --api-key "$E2B_API_KEY" \
  --template "$CUBE_TEMPLATE_ID" \
  --concurrency 50 \
  --total 500 \
  --warmup 3 \
  --mode create-delete \
  --no-tui \
  --output irqbypass-before.json
```

`create-delete`会在每次测试后删除Sandbox，避免遗留500个实例。保存成功率、平均创建延迟、P95、P99和吞吐量；后续复测必须使用同一模板和相同参数。

#### （可选）采集优化前锁竞争数据

如果内核提供锁竞争追踪点，可以使用`perf lock`采集辅助分析数据。该步骤不是性能验证的必要条件。

```bash
sudo perf lock record -o perf-before.data -a -- ./bin/cube-bench \
  --api-url "$E2B_API_URL" \
  --api-key "$E2B_API_KEY" \
  --template "$CUBE_TEMPLATE_ID" \
  --concurrency 50 \
  --total 500 \
  --warmup 3 \
  --mode create-delete \
  --no-tui
```

查看可选的锁竞争结果。

```bash
sudo perf lock contention -i perf-before.data | \
  grep -E 'irq_bypass|kvm' || true
```

## 准备目标内核源码

内核源码必须与宿主机当前RPM对应。优先使用发行版tag或固定提交，不要直接以持续变化的开发分支作为构建基线。

```bash
uname -r
```

指定与当前RPM对应的内核tag或提交。

```bash
export KERNEL_REF='<kernel-tag-or-commit>'
```

克隆内核源码。

```bash
git clone https://gitcode.com/openeuler/kernel.git ~/kernel
```

切换到指定基线。

```bash
cd ~/kernel
git checkout "$KERNEL_REF"
```

创建本次适配分支并记录提交。

```bash
git checkout -b cubesandbox-kvm-optimize
git rev-parse HEAD
```

修改前记录源码提交、当前内核版本和启动项。后续vNMI与irqbypass补丁均在此工作分支中按需应用。

## vNMI兼容问题（可选）

### 适用条件和不兼容原因

CubeSandbox当前在ARM64 vGIC快照中以32位数据保存和恢复`ICC_AP1Rn_EL1`。未合入vNMI系列的内核只使用低32位，与CubeSandbox兼容，不需要PR #27059。

目标内核合入vNMI系列后，宿主机支持FEAT_NMI时可能为Guest默认启用vNMI。此时对应的`ICH_AP1Rn_EL2`为64位，NMI优先级位于高32位，而CubeSandbox不能保存和恢复该状态，因此才存在兼容问题。

vNMI上游补丁位置见[vNMI patchset邮件线程](https://mailweb.openeuler.org/archives/list/kernel@openeuler.org/thread/ZRW2NMY5DXWIF25JSZRHM7W7XFC3RTQH/)。

该线程列出15个ARM64 KVM和vGIC补丁，并给出[上游arm64/nmi分支](https://git.kernel.org/pub/scm/linux/kernel/git/maz/arm-platforms.git/log/?h=arm64/nmi)。用户可据此检查目标内核是否已使能vNMI。

[openEuler kernel PR #27059](https://gitcode.com/openeuler/kernel/pull/27059)使vNMI改为由VMM显式选择。未启用vNMI时，KVM仅保存和恢复低32位；启用vNMI时才访问完整64位。该行为与CubeSandbox当前的32位vGIC状态格式兼容。

### 判断目标内核状态

先根据vNMI patchset中的关键补丁主题检查系列是否合入，再检查默认启用逻辑和PR #27059修复特征。

```bash
git log --oneline --all \
  --grep='KVM: arm64: vgic-v3: Upgrade AP1Rn to 64bit'
```

```bash
git log --oneline --all \
  --grep='KVM: arm64: Allow userspace to control ID_AA64PFR1_EL1.NMI'
```

```bash
git log --oneline --all \
  --grep='KVM: arm64: Allow GICv3.3 NMI if the host supports it'
```

```bash
git grep -n 'pfr1_nmi' -- \
  arch/arm64/include/asm/kvm_host.h arch/arm64/kvm
```

```bash
git grep -n \
  'kvm->arch.pfr1_nmi = ID_AA64PFR1_EL1_NMI_IMP' -- \
  arch/arm64/kvm/arm.c
```

```bash
git grep -n 'vgic_v3_vcpu_has_vnmi' -- arch/arm64
```

**表 3** vNMI兼容状态判断

| 检查结果 | 结论 | 处理方法 |
| --- | --- | --- |
| 未找到pfr1_nmi等vNMI实现 | 目标内核未合入vNMI系列 | 不需要PR #27059 |
| 找到vgic_v3_vcpu_has_vnmi | 目标内核已包含PR #27059对应修复 | 不重复合入 |
| 找到pfr1_nmi和默认赋值，但未找到vgic_v3_vcpu_has_vnmi | 已合入vNMI系列但缺少兼容修复 | 检查并回合PR #27059 |
| 仅找到部分特征 | 可能是部分回合或定制实现 | 由内核维护人员评审后决定 |

### 合入修复补丁

仅在目标内核已合入vNMI系列、尚未包含修复且与OLK-6.6接口兼容时，下载并审核PR补丁后合入测试分支。

```bash
mkdir -p ~/kernel-patches
```

```bash
curl -fL \
  https://gitcode.com/openeuler/kernel/merge_requests/27059.patch \
  -o ~/kernel-patches/olk-vnmi-cubesandbox.patch
```

```bash
git am --3way ~/kernel-patches/olk-vnmi-cubesandbox.patch
```

如果补丁与目标发行分支存在冲突，应由内核维护人员完成回合和评审，不要跳过冲突。补丁合入后仍不得为CubeSandbox Guest启用vNMI；只有CubeSandbox VMM实现64位vNMI状态保存、恢复和显式特性协商后，才能启用该功能。

如果目标内核未合入vNMI系列，可以跳过本章。如果需要PR #27059，应先构建并启动仅含vNMI修复的内核，再返回[采集优化前基线](#采集优化前基线)。完成基线后继续合入上游XArray提交，确保A/B测试中唯一性能变量是irqbypass实现。

## 合入KVM irqbypass XArray优化

### 确认内核版本

irqbypass XArray最终上游实现首次合入Linux v6.17，核心XArray提交为`8394b32faecd`。确认的版本边界如下。

**表 4** KVM irqbypass XArray版本边界

| 内核来源 | 首个包含版本 | XArray提交 | 使用建议 |
| --- | --- | --- | --- |
| Linux主线 | v6.17 | 8394b32faecd | 已包含，不重复合入 |
| openEuler OLK-6.6 | 6.6.0-167.0.0 | c51917e52d7f | 上游提交8394b32faecd的OLK-6.6适配参考 |
| openEuler 24.03 LTS SP3/SP4发行分支 | 截至2026-09-01尚未包含 | 无 | 由内核维护人员回合上游提交8394b32faecd |

实际回合只以Linux上游提交`8394b32faecd`为补丁来源。openEuler [PR #26321](https://gitcode.com/openeuler/kernel/pull/26321)中的提交`c51917e52d7f`用于参考OLK-6.6的代码上下文适配，不作为补丁来源。

在目标内核源码中执行以下命令。

```bash
git log --oneline --all \
  --grep='irqbypass: Use xarray to track producers and consumers'
```

```bash
git grep -n 'DEFINE_XARRAY(producers)' -- virt/lib/irqbypass.c
```

任一命令确认XArray实现存在时，不要重复合入。Linux v6.17及以上主线内核、包含提交`8394b32faecd`的内核，以及包含openEuler提交`c51917e52d7f`的内核均已具备该优化。除非另行构建同基线的链表版本内核，否则不能生成严格的优化前后A/B数据。

### 回合Linux上游XArray提交

对于不包含XArray实现的目标内核，下载上游提交`8394b32faecd`的标准patch。

```bash
mkdir -p ~/kernel-patches/irqbypass-v6.17
```

```bash
curl -fL \
  'https://git.kernel.org/pub/scm/linux/kernel/git/torvalds/linux.git/patch/?id=8394b32faecd9c63b3c436e78e62519e9548e530' \
  -o ~/kernel-patches/irqbypass-v6.17/8394b32faecd.patch
```

合入上游XArray补丁。

```bash
git am --3way ~/kernel-patches/irqbypass-v6.17/8394b32faecd.patch
```

上游提交基于Linux v6.17的irqbypass代码上下文，直接应用到旧内核时可能冲突。发生冲突后执行`git am --abort`，由内核维护人员仅围绕XArray替换语义适配`include/linux/irqbypass.h`和`virt/lib/irqbypass.c`，并参考openEuler提交`c51917e52d7f`处理OLK-6.6差异。

确认最终源码使用XArray，并查看本次内核分支上的补丁记录。

```bash
git grep -n 'DEFINE_XARRAY' -- virt/lib/irqbypass.c
```

```bash
git log --oneline -1 --grep='irqbypass: Use xarray to track producers and consumers'
```

### 构建和启动目标内核

内核构建方式随openEuler分支、RPM规范、Secure Boot和组织签名流程而变化，本文不展开通用编译步骤。应使用目标发行版或组织已经验证的内核构建、签名、安装和GRUB配置流程。

构建时至少确认以下事项。

- `.config`来源于目标宿主机或对应发行版配置。
- 仅含vNMI修复的基线内核使用`-cubesandbox-vnmi`后缀。
- 合入irqbypass XArray提交的最终内核使用`-cubesandbox-kvm-opt`后缀。
- 启用Secure Boot或模块签名时，不清空证书配置，应完成组织要求的签名。
- 安装前保留可启动的旧内核，并记录当前默认GRUB启动项。

可使用以下命令设置和确认版本后缀，随后按既有流程构建并安装内核包。

```bash
export LOCALVERSION_SUFFIX='-cubesandbox-kvm-opt'
```

复用当前配置并设置版本后缀。

```bash
cp /boot/config-$(uname -r) .config
./scripts/config --set-str LOCALVERSION "$LOCALVERSION_SUFFIX"
make olddefconfig
```

确认构建版本。

```bash
make kernelrelease
```

重启后确认实际运行内核、默认启动项和KVM模块。

```bash
uname -r
```

```bash
grubby --default-kernel
```

```bash
lsmod | grep kvm
```

`uname -r`应包含对应版本后缀。确认CubeSandbox核心服务恢复为`active`后，再执行功能和性能回归。

## 优化后回归与性能对比

重启进入新内核后，确认CubeSandbox服务和原模板状态正常，并重新设置当前shell使用的环境变量。

```bash
systemctl list-units 'cube-*' --all --no-pager
```

```bash
cubemastercli tpl info --template-id '<template-id>'
```

重新设置性能测试环境变量。

```bash
export CUBE_TEMPLATE_ID='<template-id>'
export E2B_API_URL=http://127.0.0.1:3000
export E2B_API_KEY=e2b_000000
```

### 采集优化后数据

进入之前构建的`cube-bench`目录，以完全相同的参数采集优化后数据。

```bash
cd ~/CubeSandbox/examples/cube-bench
```

```bash
uname -r | tee irqbypass-after-kernel.txt
```

```bash
git -C ~/kernel rev-parse HEAD | tee irqbypass-after-source.txt
```

```bash
./bin/cube-bench \
  --api-url "$E2B_API_URL" \
  --api-key "$E2B_API_KEY" \
  --template "$CUBE_TEMPLATE_ID" \
  --concurrency 50 \
  --total 500 \
  --warmup 3 \
  --mode create-delete \
  --no-tui \
  --output irqbypass-after.json
```

#### （可选）采集优化后锁竞争数据

只有在优化前已经采集`perf-before.data`时，才需要以相同参数采集优化后锁竞争数据。

```bash
sudo perf lock record -o perf-after.data -a -- ./bin/cube-bench \
  --api-url "$E2B_API_URL" \
  --api-key "$E2B_API_KEY" \
  --template "$CUBE_TEMPLATE_ID" \
  --concurrency 50 \
  --total 500 \
  --warmup 3 \
  --mode create-delete \
  --no-tui
```

查看可选的锁竞争结果。

```bash
sudo perf lock contention -i perf-after.data | \
  grep -E 'irq_bypass|kvm' || true
```

### 对比结果

执行以下命令对比成功率、吞吐量和创建延迟。

```bash
jq -s -r '
  ["指标", "优化前", "优化后"],
  ["成功率", .[0].summary.success_rate, .[1].summary.success_rate],
  ["吞吐量(QPS)", .[0].summary.throughput_qps, .[1].summary.throughput_qps],
  ["平均创建延迟(ms)", .[0].create.avg, .[1].create.avg],
  ["P95创建延迟(ms)", .[0].create.p95, .[1].create.p95],
  ["P99创建延迟(ms)", .[0].create.p99, .[1].create.p99]
  | @tsv
' irqbypass-before.json irqbypass-after.json | column -t -s $'\t'
```

优化后成功率不得下降。性能结论以相同模板、参数和负载下的平均延迟、P95、P99及吞吐量提升为主。`perf lock`仅用于辅助判断锁竞争是否变化，不作为性能提升成立的必要条件；如果irqbypass不是实际热点，提升可能不明显。

## 回退方法

查看已安装内核和GRUB启动项。

```bash
uname -r
```

```bash
sudo grubby --info=ALL | grep -E '^(index|kernel|title)='
```

将`<old-kernel-release>`替换为保留的旧内核版本，设置默认启动项并重启。

```bash
sudo grubby --set-default '/boot/vmlinuz-<old-kernel-release>'
```

```bash
sudo reboot
```

重启后执行`uname -r`确认已切回旧内核。业务验证稳定后，方可删除补丁内核包。

## 故障排除

### 补丁无法合入的解决方法

**问题现象：** 执行`git am --3way`时出现补丁失败或合并冲突。

**原因分析：** 上游提交`8394b32faecd`基于Linux v6.17的irqbypass代码结构，目标openEuler内核可能仍使用旧字段和接口，因此patch上下文无法直接匹配。

**解决方法：** 执行`git am --abort`恢复到回合前状态。确认目标源码尚未包含XArray实现，再以提交`8394b32faecd`的变更语义完成适配。openEuler提交`c51917e52d7f`仅用于参考OLK-6.6上下文差异。处理后`git status --short`应无冲突或未提交文件。

### 新内核版本无法识别的解决方法

**问题现象：** 安装RPM并重启后，`uname -r`没有`cubesandbox-kvm-opt`后缀，或者仍运行旧内核。

**原因分析：** 可能未设置`CONFIG_LOCALVERSION`、安装了旧RPM，或GRUB默认项未指向新内核。

**解决方法：** 执行`make kernelrelease`确认构建版本，使用`rpm -qp`核对待安装RPM，再通过`grubby --info=ALL`和`grubby --default-kernel`确认启动项。重新安装并选择正确启动项后，`uname -r`应显示带后缀的新内核版本。

### 模板或快照启动失败的解决方法

**问题现象：** 模板长时间不是`READY`，或者从模板、快照创建Sandbox失败。

**原因分析：** 常见原因包括ARM64镜像不可用、`/data/cubelet`未启用XFS reflink、KVM不可用、模板探针失败、Cubelet或VMM异常。快照恢复失败还可能是宿主机默认启用vNMI，但CubeSandbox仍按32位保存和恢复`ICC_AP1Rn_EL1`。

**解决方法：** 依次检查`/dev/kvm`、`xfs_info /data/cubelet`、模板任务状态，以及`/data/log/Cubelet/`、`/data/log/CubeVmm/`下的业务日志。修复环境或镜像问题后，模板应进入`READY`状态。如果目标内核包含vNMI系列，vGIC状态错误还需确认内核包含PR #27059对应修复，并确认CubeSandbox Guest没有启用vNMI。

### 高并发性能没有改善的解决方法

**问题现象：** 优化前后平均延迟、P95、P99或吞吐量没有明显变化。

**原因分析：** 可能是两次测试参数不一致，或者实际瓶颈不在irqbypass，而在VMM restore、VGIC、Virtio MSI-X、网络TAP、存储或调度资源过滤等路径。

**解决方法：** 先统一模板、并发度、请求数和预热次数，优先比较`cube-bench`的延迟、吞吐量和成功率。如果已采集可选的`perf lock`数据，可以进一步判断irqbypass是否为热点；没有该数据时，不影响性能指标对比。

## 参考资料

- [CubeSandbox源码仓库](https://github.com/TencentCloud/CubeSandbox)
- [CubeSandbox 快速开始](https://cubesandbox.com/zh/guide/quickstart.html)
- [CubeSandbox ARM64裸金属部署](https://cubesandbox.com/zh/guide/bare-metal-deploy.html)
- [CubeSandbox v0.5.0 Release](https://github.com/TencentCloud/CubeSandbox/releases/tag/v0.5.0)
- [CubeSandbox v0.5.1 Release](https://github.com/TencentCloud/CubeSandbox/releases/tag/v0.5.1)
- [CubeSandbox v0.5.1 Python SDK版本信息](https://github.com/TencentCloud/CubeSandbox/blob/v0.5.1/sdk/python/pyproject.toml)
- [CubeSandbox v0.7.0 Release](https://github.com/TencentCloud/CubeSandbox/releases/tag/v0.7.0)
- [CubeSandbox 模板概览](https://cubesandbox.com/zh/guide/templates.html)
- [CubeSandbox 快照、回滚与克隆](https://cubesandbox.com/zh/guide/snapshot-rollback-clone.html)
- [CubeSandbox 服务管理与日志](https://cubesandbox.com/zh/guide/service-management.html)
- [CubeSandbox部署排障](https://github.com/TencentCloud/CubeSandbox/blob/v0.7.0/docs/zh/guide/troubleshooting/deployment.md)
- [CubeSandbox沙箱网段冲突排障](https://github.com/TencentCloud/CubeSandbox/blob/v0.7.0/docs/zh/guide/troubleshooting/local-network-cidr-conflict.md)
- [cube-bench 使用说明](https://github.com/TencentCloud/CubeSandbox/tree/master/examples/cube-bench)
- [CubeSandbox v0.7.0 ARM64 vGIC状态实现](https://github.com/TencentCloud/CubeSandbox/blob/v0.7.0/hypervisor/hypervisor/src/kvm/aarch64/gic/icc_regs.rs)
- [Linux v6.17 irqbypass XArray提交](https://git.kernel.org/pub/scm/linux/kernel/git/torvalds/linux.git/commit/?id=8394b32faecd9c63b3c436e78e62519e9548e530)
- [openEuler OLK-6.6 irqbypass XArray回合参考提交](https://gitcode.com/openeuler/kernel/commit/c51917e52d7f267f75adeeeb747ce967adf6989b)
- [openEuler OLK-6.6 irqbypass回合参考PR](https://gitcode.com/openeuler/kernel/pull/26321)
- [ARM64 KVM vNMI patchset](https://mailweb.openeuler.org/archives/list/kernel@openeuler.org/thread/ZRW2NMY5DXWIF25JSZRHM7W7XFC3RTQH/)
- [ARM64 KVM vNMI上游分支](https://git.kernel.org/pub/scm/linux/kernel/git/maz/arm-platforms.git/log/?h=arm64/nmi)
- [openEuler OLK-6.6 vNMI兼容补丁PR](https://gitcode.com/openeuler/kernel/pull/27059)

## 修订记录

| 文档版本 | 发布日期   | 修改说明         |
| -------- | ---------- | ---------------- |
| 01       | 2026-09-30 | 第一次正式发布。 |
