
# Kata vNVDIMM特性指南

## 特性描述

### 简介

本文介绍在Kata Containers + QEMU环境下，利用QEMU NVDIMM将Kata Guest image映射为Guest PMEM设备。容器rootfs保持使用virtio-fs，额外通过PMEM文件系统路径访问数据。Guest内核识别PMEM设备后，将PMEM分区只读挂载到容器目录，用于优化目录遍历、大量小文件访问、stat、open/close和多文件读取等文件访问场景。

### 约束与限制

- 本文档仅适用于 **AArch64（ARM64）** 架构，基于鲲鹏920新型号处理器和鲲鹏950处理器验证，不适用于x86_64架构。
- 本特性通过容器挂载Guest PMEM文件系统提供额外数据访问路径，不改变容器默认rootfs使用方式。
- PMEM路径可降低部分文件访问场景下的访问开销，尤其适用于高频小文件访问和文件打开关闭等场景。
- 依赖Kata Containers、QEMU NVDIMM功能以及Guest kernel对PMEM设备的支持。
- 当前仅支持将PMEM分区挂载到容器指定目录，不支持直接作为容器rootfs。
- PMEM分区来源于Kata Guest image，当前建议以只读方式挂载，不支持作为可写业务数据盘使用。

### 应用场景

Kata PMEM文件访问加速特性适用于容器内高频读取文件的场景。

- 在AI沙箱、开发环境等场景中，容器需要频繁访问模型文件、依赖库、配置文件等只读数据时，可将高频访问数据放置于PMEM文件系统路径，降低文件访问开销。

- 在软件开发、构建等场景中，可用于优化大量小文件访问相关操作，例如目录遍历、文件属性查询、文件打开关闭等元数据操作。

- 在Kata容器存储性能优化场景中，可用于减少virtio-fs文件访问路径开销，提升特定文件访问场景下的性能表现。

### 原理描述

在默认Kata容器场景中，容器rootfs通常通过virtio-fs访问。文件操作需要经过Guest virtio-fs驱动、virtiofsd以及Host文件系统处理。

容器rootfs路径：

```text
Host rootfs/shared directory
→ virtiofsd
→ virtio-fs device
→ Guest virtio-fs driver
→ Container rootfs (/)
```

Guest PMEM路径：

```text
Host Kata Guest image
→ QEMU NVDIMM device
→ Guest PMEM device (/dev/pmem0)
→ PMEM partition (/dev/pmem0p1)
→ Guest filesystem
→ Container /pmem
```

本特性通过QEMU NVDIMM功能将Kata Guest image提供为Guest PMEM设备。Guest内核识别PMEM设备并挂载文件系统后，容器可通过挂载目录访问PMEM文件数据。

相比virtio-fs路径，PMEM文件访问主要由Guest内核和本地文件系统完成，减少了部分跨Guest/Host文件访问交互开销，适用于文件元数据操作和高频小文件访问场景。

## 软件安装

### 环境要求

本文基于特定环境提供指导，在正式操作前请确保软硬件均满足要求。

**表1**  硬件要求

| 项目 | 说明 |
| --- | --- |
| 处理器 | 鲲鹏920新型号处理器、鲲鹏950处理器 |

**表2**  操作系统和软件要求

| 项目 | 版本 | 获取方法 |
| --- | --- | --- |
| OS | openEuler 24.03 LTS SP3 | [获取链接](https://mirrors.huaweicloud.com/openeuler/openEuler-24.03-LTS-SP3/ISO/aarch64/openEuler-24.03-LTS-SP3-everything-aarch64-dvd.iso) |
| Kubernetes | 1.28.14 | [获取链接](https://www.hikunpeng.com/document/detail/zh/kunpengcpfs/ecosystemEnable/Kubernetes/kunpengk8s_04_0001.html) |
| Containerd | 1.7.27 | [获取链接](https://www.hikunpeng.com/document/detail/zh/kunpengcpfs/ecosystemEnable/Containerd/kunpengcontainerd_03_0001.html) |
| Kata Containers | 3.32.0 | [获取链接](https://github.com/kata-containers/kata-containers/releases#release-3.32.0) |

## 使用特性

### 容器镜像准备

#### Dockerfile

```dockerfile
FROM ubuntu:24.04 AS builder

RUN apt-get update && \
    apt-get install -y --no-install-recommends \
        gcc libc-dev && \
    rm -rf /var/lib/apt/lists/*

COPY treewalk.c /tmp/treewalk.c

RUN gcc /tmp/treewalk.c -o /tmp/treewalk


FROM ubuntu:24.04

RUN apt-get update && \
    apt-get install -y --no-install-recommends \
        bash coreutils findutils util-linux python3 ca-certificates && \
    rm -rf /var/lib/apt/lists/*

COPY --from=builder /tmp/treewalk /usr/local/bin/treewalk
COPY run_treewalk_test.sh /usr/local/bin/run_treewalk_test.sh
COPY export_treewalk_results.py /usr/local/bin/export_treewalk_results.py
COPY prepare_treewalk_data.sh /usr/local/bin/prepare_treewalk_data.sh

RUN chmod 0755 \
    /usr/local/bin/treewalk \
    /usr/local/bin/run_treewalk_test.sh \
    /usr/local/bin/export_treewalk_results.py \
    /usr/local/bin/prepare_treewalk_data.sh

WORKDIR /bench

CMD ["bash", "-lc", "sleep infinity"]
```

镜像构建。

```bash
docker build -f Dockerfile -t docker.io/library/ubuntu-treewalk:latest .
```

#### 目录结构

```text
kata-pmem-treewalk/
├── Dockerfile
├── treewalk.c
├── prepare_treewalk_data.sh
├── run_treewalk_test.sh
└── export_treewalk_results.py
```

说明：

- treewalk.c：文件访问性能测试程序源码。
- prepare_treewalk_data.sh：数据准备脚本，用于生成测试所需文件数据。
- run_treewalk_test.sh：测试执行脚本，用于执行测试流程。
- export_treewalk_results.py：测试结果导出脚本，用于整理和导出测试数据。

上述测试程序和辅助脚本已集成至测试容器镜像中，方便测试环境快速部署和复用。
也支持在容器启动后手动复制相关文件并执行测试。

相关测试程序和脚本请参见 `附录A：相关测试脚本`。

### Kata配置

#### 创建独立配置

为避免影响默认Kata配置，创建独立QEMU PMEM配置文件。

```bash
cp /opt/kata/share/defaults/kata-containers/configuration-qemu.toml \
   /opt/kata/share/defaults/kata-containers/configuration-qemu-pmem.toml
```

注：配置文件路径以实际为准。

#### 关键配置

主要配置如下：

```toml
[hypervisor.qemu]

# 容器 rootfs 默认通过 virtio-fs 访问。
shared_fs = "virtio-fs"

# virtio-fs daemon路径。
virtio_fs_daemon = "/opt/kata/libexec/virtiofsd"

# virtio-fs缓存模式。
virtio_fs_cache = "auto"

# false 表示允许 Kata Guest image 作为 NVDIMM设备提供给Guest。
disable_image_nvdimm = false
```

如使用runtime-rs， 需配置：

```toml
[hypervisor.qemu]

# runtime-rs 中将 VM rootfs driver 配置为 virtio-pmem。
# QEMU 底层仍通过 NVDIMM 设备向 Guest 提供 Kata Guest image。
vm_rootfs_driver = "virtio-pmem"
```

#### containerd runtime配置

修改 ```/etc/containerd/config.toml```，新增 kata-qemu-pmem runtime。

```toml
[plugins."io.containerd.grpc.v1.cri".containerd.runtimes.kata-qemu-pmem]
  runtime_type = "io.containerd.kata.v2"
  [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.kata-qemu-pmem.options]
    ConfigPath = "/opt/kata/share/defaults/kata-containers/configuration-qemu-pmem.toml"
```

重启containerd。

```bash
systemctl restart containerd
```

注：配置文件路径以实际为准。

#### 创建RuntimeClass

1. 创建 ```runtimeclass.yaml```。

    ```yaml
    apiVersion: node.k8s.io/v1
    kind: RuntimeClass
    metadata:
      name: kata-qemu-pmem
    handler: kata-qemu-pmem
    ```

2. 应用配置。

    ```bash
    kubectl apply -f runtimeclass.yaml
    ```

### Pod配置

#### 测试结果目录

在Host侧创建测试结果保存目录。

```bash
mkdir -p /opt/kata-pmem-test/results
# 测试目录需要允许容器写入结果文件。
# 生产环境建议根据容器运行用户配置 owner/group。
chmod 0770 /opt/kata-pmem-test/results
```

```text
该目录用于保存treewalk测试日志和结果文件。
```

#### Pod YAML

```yaml
apiVersion: v1
kind: Pod

metadata:
  name: kata-pmem-treewalk

spec:
  # 使用启用Guest image NVDIMM的Kata RuntimeClass。
  runtimeClassName: kata-qemu-pmem
  # Pod退出后不自动重启，适合一次性性能测试。
  restartPolicy: Never

  containers:
    - name: treewalk

      # 以实际镜像为准。
      image:  docker.io/library/ubuntu-treewalk:latest
      imagePullPolicy: IfNotPresent

      # 保持容器运行，进入容器后人工确认并挂载PMEM设备。
      command:
        - bash
        - -lc
        - sleep infinity

      securityContext:
        # 在容器内创建块设备节点和执行挂载操作。
        runAsUser: 0
        runAsGroup: 0

        # 不开启完整特权模式，只添加测试所需capability。
        privileged: false
        allowPrivilegeEscalation: false

        capabilities:
          add:
            # 当容器内没有/dev/pmemX设备节点时，
            # 用于执行mknod创建块设备节点。
            - MKNOD

            # 用于容器内执行mount/umount操作。
            - SYS_ADMIN

      volumeMounts:
        # 保存treewalk原始日志和汇总结果。
        - name: benchmark-results
          mountPath: /bench

  volumes:
    - name: benchmark-results
      hostPath:
        # 目标节点上的测试结果保存目录。
        path: /opt/kata-pmem-test/results
        type: Directory
```

### 特性验证

1. 部署测试Pod。

    ```bash
    kubectl apply -f kata-pmem-treewalk-pod.yaml
    kubectl get pod kata-pmem-treewalk -w
    ```

2. 进入容器。

    ```bash
    kubectl exec -it kata-pmem-treewalk -- bash
    ```

3. 验证容器rootfs。

    ```bash
    findmnt -T / -o TARGET,SOURCE,FSTYPE,OPTIONS
    ```

    预期`/`的文件系统类型为`virtiofs`。

    ```text
    TARGET SOURCE FSTYPE   OPTIONS
    /      none   virtiofs rw,relatime
    ```

4. 验证Guest是否识别PMEM设备。

    ```bash
    cat /proc/partitions | grep pmem
    ```

    示例输出：

    ```text
    259        0     260096 pmem0
    259        1     259072 pmem0p1
    ```

5. 创建设备节点。

    ```bash
    # 主设备号和次设备号以第4步中实际环境为准
    mknod /dev/pmem0p1 b 259 1
    ```

6. mount。

    ```bash
    #创建挂载目录。
    mkdir -p /pmem
    #以只读方式挂载PMEM分区。
    mount -t ext4 -o ro /dev/pmem0p1 /pmem
    ```

    说明：

    ```text
    根据实际文件系统类型调整 -t 参数。
    ```

7. 查看挂载结果。

    ```bash
    findmnt -T /pmem -o TARGET,SOURCE,FSTYPE,OPTIONS
    ```

    预期输出：

    ```text
    /pmem /dev/pmem0p1 ext4 ro,relatime,...
    ```

8. 验证只读。

    ```bash
    touch /pmem/write-test
    ```

    预期：

    ```text
    Read-only file system
    ```

### 测试执行

1. 确认测试程序已打包。

    ```bash
    ls -l \
      /usr/local/bin/treewalk \
      /usr/local/bin/prepare_treewalk_data.sh \
      /usr/local/bin/run_treewalk_test.sh \
      /usr/local/bin/export_treewalk_results.py
    ```

    确认 treewalk可以执行。

    ```bash
    /usr/local/bin/treewalk 2>&1 | head
    ```

2. 执行数据准备脚本。

    ```bash
    # 用于准备 PMEM 和 virtio-fs 两侧一致的测试数据。
    /usr/local/bin/prepare_treewalk_data.sh
    ```

3. 执行测试。

    ```bash
    /usr/local/bin/run_treewalk_test.sh
    ```

    如需指定测试轮数，添加ROUNDS参数。

    ```bash
    ROUNDS=10 /usr/local/bin/run_treewalk_test.sh
    ```

### 单项测试

如测试结果中变异系数（CV）较高，可执行单项测试进行定位。
测试项说明：

- `statwalk`：文件元数据遍历测试。
- `statopen`：文件打开关闭测试。
- `dataread`：顺序读测试。
- `randread`：随机读测试，最后参数表示测试持续时间（秒）。

PMEM路径测试如下：

```bash
treewalk statwalk /pmem/usr
treewalk statopen /pmem/usr
treewalk dataread /pmem/usr
treewalk randread /pmem/usr 30
```

virtio-fs路径测试如下：

```bash
treewalk statwalk /vfs/usr
treewalk statopen /vfs/usr
treewalk dataread /vfs/usr
treewalk randread /vfs/usr  30
```

### 数据导出

#### 容器内导出

执行数据导出脚本，将测试结果转换为CSV文件。

```bash
python3 /usr/local/bin/export_treewalk_results.py /bench/treewalk-results
```

生成：

```text
/bench/treewalk-results/treewalk-raw.csv
/bench/treewalk-results/treewalk-summary.csv
```

由于测试结果目录通过HostPath挂载，导出文件会同步保存到Host。

```bash
ls -l /opt/kata-pmem-test/results/treewalk-results
```

### 安全回退

测试完成后，删除测试Pod和测试结果。

```bash
kubectl delete pod kata-pmem-treewalk
kubectl delete runtimeclass kata-qemu-pmem
rm -rf /opt/kata-pmem-test/results/*
```

如需恢复测试前环境：

1. 删除PMEM独立Kata配置。

    ```bash
    # 删除 RuntimeClass 前，请确认无其他 Pod 使用该 RuntimeClass。
    rm -f /etc/kata-containers/configuration-qemu-pmem.toml
    ```

2. 删除containerd中 kata-qemu-pmem runtime配置。
3. 重启containerd。

    ```bash
    systemctl restart containerd
    ```

## 附录A：相关测试脚本

### A.1 treewalk.c

创建`treewalk.c`。

```c
    // treewalk: read-only file access benchmark
    // Supports metadata and data read tests.

    #define _GNU_SOURCE

    #include <stdio.h>
    #include <stdlib.h>
    #include <string.h>
    #include <dirent.h>
    #include <sys/stat.h>
    #include <fcntl.h>
    #include <unistd.h>
    #include <time.h>
    #include <errno.h>

    /*
    * 返回单调时钟秒数。
    */
    static double now(void)
    {
        struct timespec ts;

        clock_gettime(CLOCK_MONOTONIC, &ts);

        return ts.tv_sec + ts.tv_nsec / 1e9;
    }

    /* Test counters. */
    static unsigned long n_entries;
    static unsigned long n_files;
    static unsigned long o_files;

    /*
    * Walk directory and collect file metadata.
    */
    static void walk(const char *p)
    {
        DIR *d = opendir(p);

        if (!d)
            return;

        struct dirent *e;
        char b[4096];

        while ((e = readdir(d))) {

            if (!strcmp(e->d_name, ".") ||
                !strcmp(e->d_name, ".."))
                continue;

            n_entries++;

            snprintf(b, sizeof b, "%s/%s", p, e->d_name);

            struct stat st;

            /*
            * Use lstat to avoid following symbolic links.
            */
            if (lstat(b, &st))
                continue;

            if (S_ISDIR(st.st_mode)) {
                walk(b);
            } else if (S_ISREG(st.st_mode)) {
                n_files++;
            }
        }

        closedir(d);
    }

    /*
     * statopen：
     * 在statwalk的基础上，对每个普通文件执行：
     *   open(O_RDONLY)
     *   close
     */
    static void walkopen(const char *p)
    {
        DIR *d = opendir(p);

        if (!d)
            return;

        struct dirent *e;
        char b[4096];

        while ((e = readdir(d))) {
            if (!strcmp(e->d_name, ".") ||
                !strcmp(e->d_name, ".."))
                continue;

            snprintf(b, sizeof b, "%s/%s", p, e->d_name);

            struct stat st;

            if (lstat(b, &st))
                continue;

            if (S_ISDIR(st.st_mode)) {
                walkopen(b);
            } else if (S_ISREG(st.st_mode)) {
                int fd = open(b, O_RDONLY);

                if (fd >= 0) {
                    close(fd);
                    o_files++;
                }
            }
        }

        closedir(d);
    }

    /*
     * dataread成功读取的总字节数。
     */
    static unsigned long long db;

    /*
    * Read all regular files under directory.
    */
    static void readall(const char *p)
    {
        DIR *d = opendir(p);

        if (!d)
            return;

        struct dirent *e;
        char b[4096];

        while ((e = readdir(d))) {
            if (!strcmp(e->d_name, ".") ||
                !strcmp(e->d_name, ".."))
                continue;

            snprintf(b, sizeof b, "%s/%s", p, e->d_name);

            struct stat st;

            if (lstat(b, &st))
                continue;

            if (S_ISDIR(st.st_mode)) {
                readall(b);
            } else if (S_ISREG(st.st_mode)) {
                int fd = open(b, O_RDONLY);

                if (fd < 0)
                    continue;

                char t[1 << 16];
                ssize_t r;

                while ((r = read(fd, t, sizeof t)) > 0)
                    db += r;

                close(fd);
                n_files++;
            }
        }

        closedir(d);
    }

    /*
     * randread最多收集 4,000,000个文件路径。
     * 当前/usr只有几千个文件，远低于这个上限。
     */
    static char *paths[4000000];
    static int npaths;

    /*
     * 递归收集普通文件路径。
     */
    static void collect(const char *p)
    {
        DIR *d = opendir(p);

        if (!d)
            return;

        struct dirent *e;
        char b[4096];

        while ((e = readdir(d))) {
            if (!strcmp(e->d_name, ".") ||
                !strcmp(e->d_name, ".."))
                continue;

            snprintf(b, sizeof b, "%s/%s", p, e->d_name);

            struct stat st;

            if (lstat(b, &st))
                continue;

            if (S_ISDIR(st.st_mode)) {
                collect(b);
            } else if (S_ISREG(st.st_mode)) {
                paths[npaths] = strdup(b);
                if (paths[npaths] == NULL)
                    return;

                npaths++;
            }
        }

        closedir(d);
    }


    static int compare_paths(const void *a, const void *b)
    {
        const char *pa = *(const char * const *)a;
        const char *pb = *(const char * const *)b;

        return strcmp(pa, pb);
    }

    /*
     * 防止编译器完全忽略randread读取到的数据。
     */
    static unsigned long long rx;

    /*
    * Random read benchmark.
    * Files are sorted to keep PMEM and virtio-fs access order consistent.
    */
    static int run_randread(const char *dir, double secs)
    {
        if (secs <= 0) {
            fprintf(stderr, "randread secs must be greater than 0\n");
            return 1;
        }

        /*
         * 收集全部普通文件路径。
         */
        collect(dir);

        if (!npaths) {
            printf("RANDREAD files=0\n");
            return 0;
        }

        /* Sort paths for stable benchmark order. */
        qsort(
            paths,
            npaths,
            sizeof(paths[0]),
            compare_paths
        );

        int n = 0;
        int fds[64];
        off_t sz[64];

        /* Keep up to 64 files open for random reads. */
        for (int i = 0; i < 64 && i < npaths; i++) {
            int fd = open(paths[i], O_RDONLY);

            if (fd < 0)
                continue;

            off_t size = lseek(fd, 0, SEEK_END);

            if (size < 0) {
                close(fd);
                continue;
            }

            fds[n] = fd;
            sz[n] = size;
            n++;
        }

        if (!n) {
            printf("RANDREAD open_failed\n");
            return 0;
        }

        char buf[4096];
        unsigned long long bytes = 0;
        unsigned long ios = 0;

        /* 固定随机种子，保证每次执行使用相同随机序列。 */
        srand(1);

        double t0 = now();

        while (now() - t0 < secs) {
            int k = rand() % n;
            /* 文件偏移量 */
            off_t off =
                ((off_t)rand() * 4096) %
                (sz[k] ? sz[k] : 1);

            /* off 向下对齐到 4 KiB 边界 */
            off &= ~(off_t)4095;

            lseek(fds[k], off, SEEK_SET);

            ssize_t r = read(fds[k], buf, sizeof buf);

            if (r > 0) {
                bytes += r;
                ios++;
                rx ^= buf[0];
            }
        }

        double dt = now() - t0;

        printf(
            "RANDREAD files=%d ios=%lu "
            "bytes=%llu time=%.3f "
            "throughput=%.1f MB/s iops=%.0f\n",
            npaths,
            ios,
            bytes,
            dt,
            (bytes / 1e6) / dt,
            ios / dt
        );

        /* 关闭文件不计入测试时间。 */
        for (int i = 0; i < n; i++)
            close(fds[i]);

        return 0;
    }

    /*
     * createdel：
     *
     * 在指定目录创建n个空文件，再删除这些文件，
     * 分别统计创建和删除阶段的吞吐。
     *
     * 返回值：
     *   0：测试完成
     *   1：参数无效或目录创建失败
     */
    static int run_createdel(const char *dir, long n)
    {
        if (n <= 0) {
            fprintf(stderr, "createdel n must be greater than 0\n");
            return 1;
        }

        char p[4096];

        if (mkdir(dir, 0755) != 0 && errno != EEXIST) {
            perror("mkdir");
            return 1;
        }

        double t0 = now();

        for (long i = 0; i < n; i++) {
            snprintf(p, sizeof p, "%s/f%ld", dir, i);

            int fd = open(
                p,
                O_CREAT | O_WRONLY,
                0644
            );

            if (fd >= 0)
                close(fd);
        }

        double t1 = now();

        for (long i = 0; i < n; i++) {
            snprintf(p, sizeof p, "%s/f%ld", dir, i);
            unlink(p);
        }

        double t2 = now();

        printf(
            "CREATE n=%ld "
            "create_time=%.6f create_ops=%.0f "
            "remove_time=%.6f remove_ops=%.0f\n",
            n,
            t1 - t0,
            n / (t1 - t0),
            t2 - t1,
            n / (t2 - t1)
        );

        rmdir(dir);

        return 0;
    }

    int main(int argc, char **argv)
    {
        if (argc < 3) {
            fprintf(
                stderr,
                "usage: %s "
                "statwalk|statopen|dataread|randread|createdel "
                "<dir> [n|secs]\n",
                argv[0]
            );

            return 1;
        }

        const char *mode = argv[1];
        const char *dir = argv[2];

        if (!strcmp(mode, "statwalk")) {
            double t0 = now();

            walk(dir);

            double t1 = now();

            printf(
                "STAT entries=%lu files=%lu "
                "time=%.6f throughput=%.0f files/s\n",
                n_entries,
                n_files,
                t1 - t0,
                n_files / (t1 - t0)
            );

        } else if (!strcmp(mode, "statopen")) {
            double t0 = now();

            walkopen(dir);

            double t1 = now();

            printf(
                "STATOPEN files=%lu "
                "time=%.6f throughput=%.0f files/s\n",
                o_files,
                t1 - t0,
                o_files / (t1 - t0)
            );

        } else if (!strcmp(mode, "dataread")) {
            double t0 = now();

            readall(dir);

            double t1 = now();

            printf(
                "DATA files=%lu bytes=%llu "
                "time=%.6f throughput=%.1f MB/s\n",
                n_files,
                db,
                t1 - t0,
                (db / 1e6) / (t1 - t0)
            );

        } else if (!strcmp(mode, "randread")) {
            if (argc < 4) {
                fprintf(stderr, "randread needs secs\n");
                return 1;
            }

            return run_randread(dir, atof(argv[3]));

        } else if (!strcmp(mode, "createdel")) {
            if (argc < 4) {
                fprintf(stderr, "createdel needs n\n");
                return 1;
            }

            return run_createdel(dir, atol(argv[3]));

        } else {
            fprintf(stderr, "unknown mode %s\n", mode);
            return 1;
        }

        return 0;
    }
```

### A.2 prepare_treewalk_data.sh

创建`prepare_treewalk_data.sh`。

```bash
#!/usr/bin/env bash
set -euo pipefail

PMEM_DIR="${PMEM_DIR:-/pmem/usr}"
VFS_ROOT="${VFS_ROOT:-/vfs}"
VFS_DIR="${VFS_DIR:-${VFS_ROOT}/usr}"

if [ ! -d "${PMEM_DIR}" ]; then
    echo "missing PMEM source directory: ${PMEM_DIR}" >&2
    exit 1
fi

rm -rf "${VFS_DIR}"

mkdir -p "${VFS_ROOT}"

# 将/pmem/usr整体复制到/vfs下。
cp -a "${PMEM_DIR}" "${VFS_ROOT}/"

# 等待脏页写回，确保复制动作完成。
sync

echo "treewalk test data prepared:"
echo "  PMEM: ${PMEM_DIR}"
echo "  VFS:  ${VFS_DIR}"

# 简单检查文件数量。
echo "PMEM files: $(find "${PMEM_DIR}" -type f | wc -l)"
echo "VFS files:  $(find "${VFS_DIR}" -type f | wc -l)"
```

### A.3 run_treewalk_test.sh

创建 `run_treewalk_test.sh`。

```bash
#!/usr/bin/env bash

set -euo pipefail

# treewalk可执行文件路径。
TREEWALK="${TREEWALK:-/usr/local/bin/treewalk}"

# 测试目录。
PMEM_DIR="${PMEM_DIR:-/pmem/usr}"
VFS_DIR="${VFS_DIR:-/vfs/usr}"
OUT="${OUT:-/bench/treewalk-results}"

# 每种测试模式执行的轮数。
ROUNDS="${ROUNDS:-10}"

# randread单轮持续时间，单位为秒。
RANDREAD_SECS="${RANDREAD_SECS:-30}"

mkdir -p "${OUT}"

# 检查测试目录是否存在。
for path in "${PMEM_DIR}" "${VFS_DIR}"; do
    [ -d "${path}" ] || {
        echo "missing path: ${path}" >&2
        exit 21
    }
done

# 统计PMEM和virtio-fs目录中的普通文件数量和逻辑大小。
for backend in pmem vfs; do
    eval path='${'$(echo "${backend}" | tr '[:lower:]' '[:upper:]')'_DIR}'

    find "${path}" -type f |
        wc -l > "${OUT}/${backend}-file-count.txt"

    find "${path}" -type f -printf '%s\n' |
        awk '{sum += $1} END {print sum + 0}' \
        > "${OUT}/${backend}-logical-bytes.txt"
done

# 检查两边文件数量是否一致。
cmp -s \
    "${OUT}/pmem-file-count.txt" \
    "${OUT}/vfs-file-count.txt" || {
        echo "PMEM/VFS file counts differ" >&2
        exit 22
    }

# 检查两边文件逻辑大小是否一致。
cmp -s \
    "${OUT}/pmem-logical-bytes.txt" \
    "${OUT}/vfs-logical-bytes.txt" || {
        echo "PMEM/VFS logical sizes differ" >&2
        exit 23
    }

# 执行单次treewalk，并将输出同时保存到日志文件。
run_test() {
    local mode=$1
    local backend=$2
    local path=$3
    local round=$4

    echo "===== ${mode} round ${round} ${backend^^} ====="

    if [ "${mode}" = "randread" ]; then
        "${TREEWALK}" "${mode}" "${path}" "${RANDREAD_SECS}" |
            tee "${OUT}/${mode}-${backend}-r${round}.log"
    else
        "${TREEWALK}" "${mode}" "${path}" |
            tee "${OUT}/${mode}-${backend}-r${round}.log"
    fi
}


# 依次执行四种测试模式。
for mode in statwalk statopen dataread randread; do
    for round in $(seq 1 "${ROUNDS}"); do

        # 奇数轮先跑PMEM，偶数轮先跑VFS，
        # 用于减少固定执行顺序带来的缓存和系统状态偏差。
        if (( round % 2 == 1 )); then
            run_test "${mode}" pmem "${PMEM_DIR}" "${round}"
            run_test "${mode}" vfs  "${VFS_DIR}"  "${round}"
        else
            run_test "${mode}" vfs  "${VFS_DIR}"  "${round}"
            run_test "${mode}" pmem "${PMEM_DIR}" "${round}"
        fi
    done
done
```

### A.4 export_treewalk_results.py

创建 `export_treewalk_results.py`。

```python
#!/usr/bin/env python3

from __future__ import annotations

import csv
import re
import statistics
import sys
from pathlib import Path


# 匹配treewalk日志文件名。
# 格式：测试类型-后端-r次序.log
LOG_RE = re.compile(
    r"^(statwalk|statopen|dataread|randread)-(pmem|vfs)-r(\d+)\.log$"
)


# 提取日志中的性能指标。
PATTERNS = [
    (
        "files/s",
        re.compile(
            r"([0-9]+(?:\.[0-9]+)?)\s*files/s",
            re.I,
        ),
    ),
    (
        "MB/s",
        re.compile(
            r"([0-9]+(?:\.[0-9]+)?)\s*MB/s",
            re.I,
        ),
    ),
   (
        "IOPS",
        re.compile(
            r"\biops\s*=\s*([0-9]+(?:\.[0-9]+)?)",
            re.I,
        ),
    ),
]

# 计算变异系数(CV)，用于衡量测试结果波动。
def cv(values: list[float]) -> float:
    if len(values) < 2:
        return 0.0
    mean = statistics.mean(values)
    return (
        0.0
        if mean == 0
        else statistics.stdev(values) / mean * 100
    )

def main() -> int:

    root = Path(
        sys.argv[1]
        if len(sys.argv) > 1
        else "/bench/treewalk-results"
    )

    rows = []

    # 解析日志文件。
    for path in sorted(root.glob("*.log")):

  
        match = LOG_RE.match(path.name)
        if not match:
            continue

        text = path.read_text(
            encoding="utf-8",
            errors="replace",
        )

        mode, backend, round_no = match.groups()

        for metric, pattern in PATTERNS:

            values = pattern.findall(text)

            if values:
               # 取最后一次匹配结果，避免中间输出影响统计。
                rows.append(
                    {
                        "mode": mode,
                        "backend": backend,
                        "round": int(round_no),
                        "metric": metric,
                        "value": float(values[-1]),
                        "log_file": path.name,
                    }
                )

    if not rows:
        print(
            "no treewalk metrics found",
            file=sys.stderr,
        )
        return 2

    # 输出每轮原始测试结果。
    raw_csv = root / "treewalk-raw.csv"

    with raw_csv.open(
        "w",
        newline="",
        encoding="utf-8",
    ) as f:

        writer = csv.DictWriter(
            f,
            fieldnames=rows[0].keys(),
        )

        writer.writeheader()
        writer.writerows(rows)

    # 按测试类型、后端、指标进行分组统计。
    grouped = {}

    for row in rows:
        key = (
            row["mode"],
            row["backend"],
            row["metric"],
        )
        grouped.setdefault(
            key,
            [],
        ).append(
            row["value"]
        )

    summary = []

    for (
        mode,
        backend,
        metric,
    ), values in sorted(grouped.items()):

        summary.append(
            {
                "mode": mode,
                "backend": backend,
                "metric": metric,
                "rounds": len(values),
                "mean": statistics.mean(values),
                "median": statistics.median(values),
                "min": min(values),
                "max": max(values),
                "cv_percent": cv(values),
            }
        )

    # 输出汇总统计结果。
    summary_csv = root / "treewalk-summary.csv"

    with summary_csv.open(
        "w",
        newline="",
        encoding="utf-8",
    ) as f:

        writer = csv.DictWriter(
            f,
            fieldnames=summary[0].keys(),
        )

        writer.writeheader()
        writer.writerows(summary)

    return 0


if __name__ == "__main__":
    raise SystemExit(main())
```

脚本最终生成两份文件：

    treewalk-raw.csv
    → 每一轮的原始结果。

    treewalk-summary.csv
    → 按 mode、backend、metric 汇总后的均值、中位数、范围和变异系数。
