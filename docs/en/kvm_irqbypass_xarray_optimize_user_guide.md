# User Guide for CubeSandbox Adaptation and Optimization on Kunpeng

<!-- md-trans-meta sourceCommit=d302e3657d3cacf89c9190d6ac2afd87f0742c43 translatedAt=2026-08-18T02:48:25.880Z pushedAt=2026-08-26T07:19:25.264Z -->

## Introduction

CubeSandbox is a sandbox system based on RustVMM and KVM. When deployed on Kunpeng servers, CubeSandbox uses the AArch64 virtualization capabilities provided by the Kunpeng processor to run MicroVMs.

This guide is intended for the Kunpeng 950 processor and covers the following adaptation and optimization processes:

- Checking the AArch64, KVM, and storage environments on the Kunpeng 950 server

- Deploying CubeSandbox and creating a template that includes the AArch64 runtime environment

- Applying the KVM irqbypass XArray patch to reduce lock contention during high-concurrency irqfd registration

- Verifying template-based and snapshot-based startup separately, and comparing high-concurrency startup performance before and after optimization

The original KVM irqbypass implementation uses a global linked list to store producers and consumers, and traverses objects by token under the protection of a single mutex during registration. The XArray patch replaces the linear traversal with direct lookup by token. It primarily optimizes host-side lock contention when a large number of sandboxes are started or resumed concurrently. This change does not affect the CubeSandbox APIs, template format, or guest-visible behavior.

## Environment Requirements

| Item | Requirement |
| --- | --- |
| Processor | Kunpeng 950 |
| CPU architecture | AArch64 |
| Virtualization | AArch64 KVM enabled on the host, with <code>/dev/kvm</code> available |
| OS | openEuler or an OS compatible with the openEuler kernel RPM build process |
| File system | XFS file system supporting reflink for <code>/data/cubelet</code> |
| Drive space | At least 50 GB for <code>/data/cubelet</code>; 200 GB recommended if multiple templates are created |
| Kernel source code | openEuler kernel source matching the target host version |
| CubeSandbox | Compatible with AArch64 |
| Verification tools | Python 3, cubesandbox (>= 0.5.0), cube-bench, and perf |

Run the following commands to check the processor architecture, KVM, and file system:

```bash
lscpu | grep -E 'Architecture|Vendor ID|Model name'
test -c /dev/kvm && echo '/dev/kvm is ready'
lsmod | grep kvm
findmnt -no FSTYPE /data/cubelet
```

`Architecture` should be `aarch64`, `/dev/kvm` should exist, and the file system type in `/data/cubelet` should be `xfs`. The specific processor model is subject to the server asset information and the `dmidecode` output.

## Precautions

- The kernel patch in this document only optimizes the KVM irqbypass registration path and does not replace CubeSandbox's adaptation on AArch64.

- Before modifying the kernel, confirm that the target kernel does not have an equivalent XArray implementation to avoid duplicate application.

- v1 is a single patch and depends on the prerequisite fix for producer unregister. v2 combines this prerequisite fix with

  the XArray optimization into a single patch series. For new porting efforts, v2 is recommended.

- The patch primarily improves performance in high-concurrency scenarios. In low-concurrency or single-instance cases, the fixed overhead may not show a noticeable change.

- Use the same template, concurrency level, and number of requests on the same server when comparing performance before and after optimization.

- Before installing a new kernel, retain at least one bootable old kernel and record the rollback boot entry.

- Before enabling the feature in a production environment, complete functional, performance, and rollback verification on test nodes.

- When Secure Boot or kernel module signing is enabled, follow your organization's kernel signing process.

## Deploying CubeSandbox

### Installing Services

The Kunpeng 950 server provides native AArch64 KVM. You do not need to install the PVM host kernel that targets only x86_64. After confirming that the environment requirements are met, use the official CubeSandbox installation script to deploy services:

```bash
curl -sL \
  https://cnb.cool/CubeSandbox/CubeSandbox/-/git/raw/master/deploy/one-click/online-install.sh \
  | MIRROR=cn bash
```

Before executing a remote script in a production environment, download and review the script content first. After installation, check the services and APIs:

```bash
systemctl is-active cube-sandbox-cube-api.service
systemctl is-active cube-sandbox-cubemaster.service
systemctl is-active cube-sandbox-cubelet.service
ss -lnt | grep ':3000 '
```

The above services should be `active`, and CubeAPI should listen on port `3000`.

### Configure the SDK Environment

Install the CubeSandbox Python SDK and set the API addresses and key.

```bash
python3 -m pip install 'cubesandbox>=0.2.0'
export CUBE_API_URL=http://127.0.0.1:3000
export E2B_API_URL=http://127.0.0.1:3000
export E2B_API_KEY=e2b_000000
```

In a production environment, replace the example key with the actual one, and do not commit the key to the code repository.

## Applying the KVM irqbypass XArray Optimization

### Selecting a Patch Version

| Version | Content | Usage Recommendation |
| --- | --- | --- |
| v1 | Single XArray patch that depends on a separate producer unregister fix | For analyzing existing v1-based porting only; not recommended for use in new environments |
| v2 | Patch series containing the producer unregister prerequisite fix and the XArray optimization | Preferred for new porting |

Both v1 and v2 can be downloaded from public patch archives. The following uses v2.

- [v1 patch archive](https://patchew.org/linux/20230801115646.33990-1-likexu%40tencent.com/)

- [v2 patch series](https://lore.kernel.org/all/20230802051700.52321-1-likexu@tencent.com/)

### Downloading the Patch and Kernel Source Code

Install the build dependencies.

```bash
sudo dnf install -y \
  git gcc gcc-c++ make bc bison flex \
  openssl-devel elfutils-libelf-devel ncurses-devel \
  dwarves rpm-build rsync perl tar xz
```

Obtain the v2 mbox from the downloadable Patchew archive.

```bash
mkdir -p ~/kernel-patches
curl -fL \
  'https://patchew.org/linux/20230802051700.52321-1-likexu%40tencent.com/mbox' \
  -o ~/kernel-patches/irqbypass-xarray-v2.mbox
grep -E '^Subject:' ~/kernel-patches/irqbypass-xarray-v2.mbox
```

Clone the openEuler kernel branch that matches the current host kernel version. Replace `<kernel-branch>` with the actual branch name. Do not use a branch that does not match the running kernel.

```bash
uname -r
git clone --depth 1 --branch <kernel-branch> \
  https://gitee.com/openeuler/kernel.git ~/kernel
cd ~/kernel
```

### Applying the Patch

Run the following commands in the root directory of the kernel source code:

```bash
git checkout -b cubesandbox-irqbypass-xarray
git am --3way ~/kernel-patches/irqbypass-xarray-v2.mbox
git log --oneline -2
```

Confirm that `include/linux/irqbypass.h` and `virt/lib/irqbypass.c` are modified.

```bash
git diff HEAD~2..HEAD --stat
```

If `git am` reports a conflict, run `git am --abort` to restore the state before the patch was applied, and then follow instructions in section "Patch Application Failures" to resolve the issue. Do not skip the conflict without confirming the semantics.

### Compiling and Installing the Kernel

Reuse the current host kernel configuration and set a recognizable version suffix.

```bash
cp /boot/config-$(uname -r) .config
./scripts/config --set-str SYSTEM_TRUSTED_KEYS ''
./scripts/config --set-str SYSTEM_REVOCATION_KEYS ''
./scripts/config --set-str LOCALVERSION '-irqbypass-xarray'
make olddefconfig
make kernelrelease
```

Clearing the certificate configuration applies only to test build environments that do not use distribution signing keys. When Secure Boot or module signing is enabled, keep the certificate configuration and follow your organization's signing process.

Compile the RPM package.

```bash
make binrpm-pkg -j"$(nproc)"
find ~/rpmbuild/RPMS -name 'kernel-*.rpm' -type f -print
```

Install the new kernel and set it as the default boot entry.

```bash
sudo dnf install -y ~/rpmbuild/RPMS/$(uname -m)/kernel-*.rpm
sudo grubby --info=ALL | grep -E '^(index|kernel|title)='
sudo grubby --set-default /boot/vmlinuz-<patched-kernel-release>
sudo grubby --default-kernel
sudo reboot
```

After the reboot, confirm the kernel and KVM status.

```bash
uname -r
lsmod | grep kvm
```

The output of `uname -r` should contain `irqbypass-xarray`.

## Performing a Cold Start and Creating a Template

When creating a template, CubeSandbox prepares the rootfs based on the OCI image, cold-starts a MicroVM, waits for the probe to become ready, and then creates a snapshot and publishes the template. Run the following commands to create an AArch64 code interpreter template:

```bash
cubemastercli tpl create-from-image \
  --image cube-sandbox-cn.tencentcloudcr.com/cube-sandbox/sandbox-code:latest \
  --writable-layer-size 1G \
  --expose-port 49999 \
  --expose-port 49983 \
  --probe 49999
```

Record `job_id` in the command output, and wait for the cold start, snapshot creation, and template publishing to complete.

```bash
cubemastercli tpl watch --job-id <job-id>
cubemastercli tpl info --template-id <template-id>
```

At least one template in the `READY` state exists. Set the template ID for subsequent verification.

```bash
export CUBE_TEMPLATE_ID=<template-id>
```

## Starting an Instance from a Template and Verifying It

The following script starts a sandbox from a template and outputs the sandbox ID and the result.

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

If the script exits without errors and outputs `kunpeng-template-start-ok`, it indicates that template-based startup, guest execution, and result return are all successful.

## Starting an Instance from a Snapshot and Verifying It

The following script first creates a source sandbox from a template, writes a marker in the file system, creates a runtime snapshot, and then uses the snapshot ID to create a new sandbox and checks whether the marker is retained.

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

If the script exits without errors and outputs `snapshot-ok`, it indicates that snapshot creation, snapshot restoration, and state inheritance are all successful.

## Verifying High-Concurrency Startup Optimization

### Building the Stress Testing Tool

```bash
git clone --depth 1 https://github.com/TencentCloud/CubeSandbox.git
cd CubeSandbox/examples/cube-bench
make
```

### Collecting Performance Data

Use the same template, concurrency level, number of requests, and warm-up count before and after optimization.

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

Record the success rate, average latency, P95, P99, and throughput. Do not compare results from two runs that use different templates or different concurrency parameters when evaluating optimization effectiveness.

Use `perf lock` to collect lock contention during the same stress test.

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

`perf lock contention -i perf.data` reads the data file generated in the previous step. When the patch is effective and the original bottleneck is indeed in irqbypass, the wait associated with `irq_bypass_register_consumer()` should decrease.

## Rollback

View the installed kernels and GRUB boot entries:

```bash
uname -r
sudo grubby --info=ALL | grep -E '^(index|kernel|title)='
```

Replace `<old-kernel-release>` with the retained old kernel version, set the default boot entry, and reboot.

```bash
sudo grubby --set-default /boot/vmlinuz-<old-kernel-release>
sudo reboot
```

After reboot, run `uname -r` to confirm that the system has switched back to the old kernel. Delete the patched kernel package only after business verification confirms stability.

## Troubleshooting

### Patch Application Failures

Symptom:

When you apply the patch by running `git am --3way`, `patch failed` is displayed or a conflict occurs.

Key process and cause analysis:

The target openEuler kernel baseline differs from the patch baseline, or the target kernel already includes some prerequisite fixes.
If v1 is applied directly, it may still lack the producer unregister prerequisite fix.

Conclusion and solution:

First run `git am --abort`, confirm that the kernel branch matches the running kernel, and check whether `include/linux/irqbypass.h` and `virt/lib/irqbypass.c` already contain the XArray implementation. If no equivalent implementation exists, resolve conflicts one by one based on the v2 patch and complete the code review. After these steps are resolved, the prerequisite fixes and XArray optimization should be fully applied, and `git status --short` should show no unresolved conflicts.

### Unrecognized New Kernel Version

Symptom:

After the RPM package installation and reboot, the `uname -r` output lacks the `irqbypass-xarray` suffix, or the system is still running the old kernel.

Key process and cause analysis:

`CONFIG_LOCALVERSION` is not set, an old RPM package has been installed, or the GRUB default entry may not point to the new kernel.

Conclusion and solution:

Run `make kernelrelease` to confirm the build version, use `rpm -qp` to verify the RPM package to be installed, and then confirm the boot entry through `grubby --info=ALL` and `grubby --default-kernel`. After you reinstall the RPM package and select the correct boot entry, `uname -r` should display the new kernel version with the suffix.

### Template-based or Snapshot-based Startup Failures

Symptom:

The template is not in the `READY` state for a long time, or creating a sandbox from a template or snapshot fails.

Key process and cause analysis:

Common causes include unavailable AArch64 images, a non-XFS `/data/cubelet`, unavailable KVM, template probe failures, and abnormal Cubelet or VMM startup.

Conclusion and solution:

Check `/dev/kvm`, `findmnt /data/cubelet`, the template task status, and the service logs under `/data/log/Cubelet/` and `/data/log/CubeVmm/` in order. After the environment or image issues are fixed, the template should enter the `READY` state, and the scripts for verifying template-based and snapshot-based startup should exit normally.

### No Improvement in High-Concurrency Performance

Symptom:

The average latency, P95, P99, or throughput shows no significant change before and after optimization.

Key process and cause analysis:

This may be caused by inconsistent test parameters between the two runs, or the actual bottleneck is not in irqbypass but in paths such as VMM restore, VGIC, Virtio MSI-X, network TAP, storage, or scheduling resource filtering.

Conclusion and solution:

First unify the template, concurrency level, number of requests, and warm-up count, and then run `perf lock contention -i perf.data` to confirm whether irqbypass is the hotspot. If the hotspot has shifted, analyze the new hotspot instead of directly attributing the lack of performance improvement to patch failure. This method can distinguish between patch failure and system bottleneck shifting.

## References

- [CubeSandbox Quick Start](https://cubesandbox.com/guide/quickstart.html)

- [CubeSandbox Templates Overview](https://cubesandbox.com/guide/templates.html)

- [CubeSandbox Snapshot, Rollback & Clone](https://cubesandbox.com/guide/snapshot-rollback-clone.html)

- [CubeSandbox Service Management & Logs](https://cubesandbox.com/guide/service-management.html)

- [cube-bench Usage Description](https://github.com/TencentCloud/CubeSandbox/tree/master/examples/cube-bench)

- [KVM irqbypass XArray v1 patch archive](https://patchew.org/linux/20230801115646.33990-1-likexu%40tencent.com/)

- [KVM irqbypass XArray v2 patch series](https://lore.kernel.org/all/20230802051700.52321-1-likexu@tencent.com/)

## Change History

|Document Version|Date|Description|
|--|--|--|
|01|2026-09-30|This is the first official release.|
