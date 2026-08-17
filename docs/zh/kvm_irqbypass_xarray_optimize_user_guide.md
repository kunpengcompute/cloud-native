# CubeSandbox鲲鹏适配和优化 用户指南

## 介绍

CubeSandbox 是基于 RustVMM 和 KVM 的沙箱系统。在鲲鹏服务器上部署时，CubeSandbox 使用鲲鹏处理器提供的 ARM64 虚拟化能力运行 MicroVM。

本指南面向鲲鹏 950 处理器，介绍以下适配和优化过程：

- 在鲲鹏 950 服务器上确认 ARM64、KVM 和存储环境。
- 部署 CubeSandbox，并制作包含 ARM64 运行环境的模板。
- 合入 KVM irqbypass XArray 补丁，降低高并发 irqfd 注册时的锁竞争。
- 分别验证模板启动和快照启动，并对比优化前后的高并发启动性能。

KVM irqbypass 原实现使用全局链表保存 producer 和 consumer，注册时需要在
同一个 mutex 保护下按 token 遍历对象。XArray 补丁将线性遍历改为按 token
直接查找，主要优化大量 Sandbox 并发启动或恢复时的宿主机锁等待，不改变
CubeSandbox API、模板格式和 Guest 可见行为。

## 环境要求

| 项目 | 要求 |
| --- | --- |
| 处理器 | 鲲鹏 950 处理器 |
| CPU 架构 | `aarch64` |
| 虚拟化 | 宿主机已启用 ARM64 KVM，并存在`/dev/kvm` |
| 操作系统 | openEuler 或兼容 openEuler kernel RPM 构建流程的操作系统 |
| 文件系统 | `/data/cubelet`使用支持 reflink 的 XFS 文件系统 |
| 磁盘空间 | `/data/cubelet`至少预留 50 GB；制作多个模板时建议预留 200 GB |
| 内核源码 | 与目标宿主机版本匹配的 openEuler kernel 源码 |
| CubeSandbox | 支持 ARM64 的CubeSandbox版本 |
| 验证工具 | Python 3、`cubesandbox>=0.5.0`、`cube-bench`和`perf` |

执行以下命令检查处理器架构、KVM 和文件系统：

```bash
lscpu | grep -E 'Architecture|Vendor ID|Model name'
test -c /dev/kvm && echo '/dev/kvm is ready'
lsmod | grep kvm
findmnt -no FSTYPE /data/cubelet
```

`Architecture`应为`aarch64`，`/dev/kvm`应存在，`/data/cubelet`的文件系统
类型应为`xfs`。处理器具体型号以服务器资产信息和`dmidecode`输出为准。

## 注意事项

- 本文的内核补丁只优化 KVM irqbypass 注册路径，不替代 CubeSandbox 的 ARM64 适配。
- 修改内核前，应确认目标内核没有等价的 XArray 实现，避免重复合入。
- v1 是单补丁版本，依赖 producer unregister 前置修复；v2 将前置修复和
  XArray 优化组成补丁系列。新移植建议使用 v2。
- 补丁主要改善高并发场景，低并发或单实例固定开销不一定有明显变化。
- 应在同一台服务器上使用相同模板、并发度和请求数进行优化前后对比。
- 安装新内核前，应保留至少一个已知可启动的旧内核并记录回退启动项。
- 生产环境启用前，应在测试节点完成功能、性能和回退验证。
- 启用 Secure Boot 或内核模块签名时，应遵循组织的内核签名流程。

## 部署 CubeSandbox

### 安装服务

鲲鹏 950 服务器提供原生 ARM64 KVM，不需要安装仅面向 x86_64 的 PVM 宿主机内核。确认环境要求满足后，使用 CubeSandbox 官方安装脚本部署服务：

```bash
curl -sL \
  https://cnb.cool/CubeSandbox/CubeSandbox/-/git/raw/master/deploy/one-click/online-install.sh \
  | MIRROR=cn bash
```

生产环境执行远程脚本前，应先下载并审核脚本内容。安装完成后检查服务和 API：

```bash
systemctl is-active cube-sandbox-cube-api.service
systemctl is-active cube-sandbox-cubemaster.service
systemctl is-active cube-sandbox-cubelet.service
ss -lnt | grep ':3000 '
```

以上服务应为`active`，并且 CubeAPI 应监听`3000`端口。

### 配置 SDK 环境

安装 CubeSandbox Python SDK，并设置 API 地址和密钥：

```bash
python3 -m pip install 'cubesandbox>=0.2.0'
export CUBE_API_URL=http://127.0.0.1:3000
export E2B_API_URL=http://127.0.0.1:3000
export E2B_API_KEY=e2b_000000
```

生产环境应将示例密钥替换为实际密钥，且不得将密钥提交到代码仓库。

## 合入 KVM irqbypass XArray 优化

### 选择补丁版本

| 版本 | 内容 | 使用建议 |
| --- | --- | --- |
| v1 | 单个 XArray 转换补丁，依赖单独的 producer unregister 修复 | 仅用于分析已有 v1 移植，不建议新环境单独合入 |
| v2 | producer unregister 前置修复和 XArray 优化补丁系列 | 新移植优先使用 |

v1 和 v2 均可从公开补丁归档下载。下文使用 v2：

- [v1 补丁归档](https://patchew.org/linux/20230801115646.33990-1-likexu%40tencent.com/)
- [v2 补丁系列](https://lore.kernel.org/all/20230802051700.52321-1-likexu@tencent.com/)

### 下载补丁和内核源码

安装构建依赖：

```bash
sudo dnf install -y \
  git gcc gcc-c++ make bc bison flex \
  openssl-devel elfutils-libelf-devel ncurses-devel \
  dwarves rpm-build rsync perl tar xz
```

从可直接下载的 Patchew 归档获取 v2 mbox：

```bash
mkdir -p ~/kernel-patches
curl -fL \
  'https://patchew.org/linux/20230802051700.52321-1-likexu%40tencent.com/mbox' \
  -o ~/kernel-patches/irqbypass-xarray-v2.mbox
grep -E '^Subject:' ~/kernel-patches/irqbypass-xarray-v2.mbox
```

克隆与当前宿主机内核版本对应的 openEuler kernel 分支。将`<kernel-branch>`
替换为实际分支名，不要使用与运行内核不匹配的分支：

```bash
uname -r
git clone --depth 1 --branch <kernel-branch> \
  https://gitee.com/openeuler/kernel.git ~/kernel
cd ~/kernel
```

### 合入补丁

在内核源码根目录执行：

```bash
git checkout -b cubesandbox-irqbypass-xarray
git am --3way ~/kernel-patches/irqbypass-xarray-v2.mbox
git log --oneline -2
```

确认补丁修改`include/linux/irqbypass.h`和`virt/lib/irqbypass.c`：

```bash
git diff HEAD~2..HEAD --stat
```

如果`git am`报告冲突，执行`git am --abort`恢复到合入前状态，然后按照
“补丁无法合入的解决方法”处理，不要在未确认语义的情况下跳过冲突。

### 编译并安装内核

复用当前宿主机内核配置并设置可识别的版本后缀：

```bash
cp /boot/config-$(uname -r) .config
./scripts/config --set-str SYSTEM_TRUSTED_KEYS ''
./scripts/config --set-str SYSTEM_REVOCATION_KEYS ''
./scripts/config --set-str LOCALVERSION '-irqbypass-xarray'
make olddefconfig
make kernelrelease
```

清空证书配置只适用于未使用发行版签名密钥的测试构建环境。启用 Secure Boot
或模块签名时，应保留证书配置并执行组织的签名流程。

编译 RPM：

```bash
make binrpm-pkg -j"$(nproc)"
find ~/rpmbuild/RPMS -name 'kernel-*.rpm' -type f -print
```

安装新内核并设置默认启动项：

```bash
sudo dnf install -y ~/rpmbuild/RPMS/$(uname -m)/kernel-*.rpm
sudo grubby --info=ALL | grep -E '^(index|kernel|title)='
sudo grubby --set-default /boot/vmlinuz-<patched-kernel-release>
sudo grubby --default-kernel
sudo reboot
```

重启后确认内核和 KVM 状态：

```bash
uname -r
lsmod | grep kvm
```

`uname -r`输出应包含`irqbypass-xarray`。

## 冷启动并制作模板

CubeSandbox 制作模板时会基于 OCI 镜像准备 rootfs，冷启动 MicroVM，等待探针
就绪后创建快照并发布模板。执行以下命令创建 ARM64 代码解释器模板：

```bash
cubemastercli tpl create-from-image \
  --image cube-sandbox-cn.tencentcloudcr.com/cube-sandbox/sandbox-code:latest \
  --writable-layer-size 1G \
  --expose-port 49999 \
  --expose-port 49983 \
  --probe 49999
```

记录命令返回的`job_id`，等待冷启动、快照和模板发布完成：

```bash
cubemastercli tpl watch --job-id <job-id>
cubemastercli tpl info --template-id <template-id>
```

模板状态应为`READY`，并至少存在一个状态为`READY`的副本。设置后续验证使用的
模板 ID：

```bash
export CUBE_TEMPLATE_ID=<template-id>
```

## 使用模板启动实例并验证

以下脚本使用模板启动 Sandbox，执行一段 Python 代码并输出 Sandbox ID 和结果：

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

脚本无异常退出且输出`kunpeng-template-start-ok`，表示模板启动、Guest 执行和
结果返回正常。

## 使用快照启动实例并验证

以下脚本先从模板创建源 Sandbox，在文件系统中写入标记，创建运行态快照，
再使用快照 ID 创建新 Sandbox 并检查标记是否保留：

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

脚本无异常退出且输出`snapshot-ok`，表示快照创建、快照恢复和状态继承正常。

## 验证高并发启动优化

### 构建压测工具

```bash
git clone --depth 1 https://github.com/TencentCloud/CubeSandbox.git
cd CubeSandbox/examples/cube-bench
make
```

### 采集性能数据

优化前后应使用相同的模板、并发度、请求数和预热次数：

```bash
./bin/cube-bench \
  --api-url "$E2B_API_URL" \
  --api-key "$E2B_API_KEY" \
  --template "$CUBE_TEMPLATE_ID" \
  --concurrency 50 \
  --total 500 \
  --warmup 3 \
  --mode create-only \
  --no-tui \
  --output result.json
```

记录成功率、平均延迟、P95、P99 和吞吐量。不要使用不同模板或不同并发参数的
两次结果判断优化效果。

使用`perf lock`采集同一压测过程的锁竞争：

```bash
sudo perf lock record -a -- ./bin/cube-bench \
  --api-url "$E2B_API_URL" \
  --api-key "$E2B_API_KEY" \
  --template "$CUBE_TEMPLATE_ID" \
  --concurrency 50 \
  --total 500 \
  --warmup 3 \
  --mode create-only \
  --no-tui
sudo perf lock contention -i perf.data | grep -E 'irq_bypass|kvm' || true
```

`perf lock contention`使用`-i perf.data`读取上一步生成的数据文件。补丁有效且
原瓶颈确实位于 irqbypass 时，`irq_bypass_register_consumer()`相关等待应下降。

## 回退方法

查看已安装内核和 GRUB 启动项：

```bash
uname -r
sudo grubby --info=ALL | grep -E '^(index|kernel|title)='
```

将`<old-kernel-release>`替换为保留的旧内核版本，设置默认启动项并重启：

```bash
sudo grubby --set-default /boot/vmlinuz-<old-kernel-release>
sudo reboot
```

重启后执行`uname -r`确认已切回旧内核。业务验证稳定后，方可删除补丁内核包。

## 故障排除

### 补丁无法合入的解决方法

问题现象描述：

执行`git am --3way`时出现`patch failed`或合并冲突。

关键过程、根本原因分析：

目标 openEuler 内核基线与补丁基线不同，或者目标内核已经包含部分前置修复。
如果直接合入 v1，还可能缺少 producer unregister 前置修复。

结论、解决方案及效果：

先执行`git am --abort`，确认内核分支与运行内核匹配，并检查
`include/linux/irqbypass.h`和`virt/lib/irqbypass.c`是否已有 XArray 实现。
没有等价实现时，基于 v2 补丁逐个解决冲突并完成代码评审。处理后应能完整合入
前置修复和 XArray 优化，且`git status --short`无未处理冲突。

### 新内核版本无法识别的解决方法

问题现象描述：

安装 RPM 并重启后，`uname -r`没有`irqbypass-xarray`后缀，或者仍运行旧内核。

关键过程、根本原因分析：

可能未设置`CONFIG_LOCALVERSION`、安装了旧 RPM，或 GRUB 默认项未指向新内核。

结论、解决方案及效果：

执行`make kernelrelease`确认构建版本，使用`rpm -qp`核对待安装 RPM，再通过
`grubby --info=ALL`和`grubby --default-kernel`确认启动项。重新安装并选择正确
启动项后，`uname -r`应显示带后缀的新内核版本。

### 模板或快照启动失败的解决方法

问题现象描述：

模板长时间不是`READY`，或者从模板、快照创建 Sandbox 失败。

关键过程、根本原因分析：

常见原因包括 ARM64 镜像不可用、`/data/cubelet`不是 XFS、KVM 不可用、模板探针
失败，以及 Cubelet 或 VMM 启动异常。

结论、解决方案及效果：

依次检查`/dev/kvm`、`findmnt /data/cubelet`、模板任务状态，以及
`/data/log/Cubelet/`、`/data/log/CubeVmm/`下的业务日志。修复环境或镜像问题后，
模板应进入`READY`状态，模板启动和快照启动验证脚本均应正常退出。

### 高并发性能没有改善的解决方法

问题现象描述：

优化前后平均延迟、P95、P99 或吞吐量没有明显变化。

关键过程、根本原因分析：

可能是两次测试参数不一致，或者实际瓶颈不在 irqbypass，而在 VMM restore、VGIC、
Virtio MSI-X、网络 TAP、存储或调度资源过滤等路径。

结论、解决方案及效果：

先统一模板、并发度、请求数和预热次数，再通过`perf lock contention -i perf.data`
确认 irqbypass 是否为热点。如果热点已经转移，应针对新的热点继续分析，而不是将
无变化直接归因于补丁失效。该方法可以区分补丁未生效与系统瓶颈转移。

## 参考资料

- [CubeSandbox 快速开始](https://cubesandbox.com/zh/guide/quickstart.html)
- [CubeSandbox 模板概览](https://cubesandbox.com/zh/guide/templates.html)
- [CubeSandbox 快照、回滚与克隆](https://cubesandbox.com/zh/guide/snapshot-rollback-clone.html)
- [CubeSandbox 服务管理与日志](https://cubesandbox.com/zh/guide/service-management.html)
- [cube-bench 使用说明](https://github.com/TencentCloud/CubeSandbox/tree/master/examples/cube-bench)
- [KVM irqbypass XArray v1 补丁归档](https://patchew.org/linux/20230801115646.33990-1-likexu%40tencent.com/)
- [KVM irqbypass XArray v2 补丁系列](https://lore.kernel.org/all/20230802051700.52321-1-likexu@tencent.com/)
