# TemplateCreator

TemplateCreator 是一个独立的 Go module：它基于锁定 commit 的 infra 2026.09 Builder 开发，可在未部署 E2B 的环境中，从 OCI 镜像或 docker save 保存的镜像 tar 构建 E2B Template，并把对象保存到本地 Store。

支持将产物导入上游 infra 2026.09，以及 [openeuler e2b-infra master](https://gitcode.com/src-openeuler/e2b-infra/commits/master) 的 `2edce03da2ba3af75d6e140a385fa2c2ef6dae06` commit。导入前必须使目标 E2B 的 Firecracker、kernel、envd、CPU 信息、对象布局和数据库注册参数与产物匹配；构建成功本身不代表目标环境已经完成导入或 Sandbox 启动验证。

> **说明**：
>
> 它不负责调用 E2B 控制面注册 Template，也不会自动执行 PostgreSQL SQL。

Go API 的运行条件、runtime 准备、并发创建和结果检查见 [Go API 用户指南](docs/TemplateCreator_Go_API_用户指南.md)；公开类型和字段详见 [Go API 说明](docs/go-api.md)。用户指南的导入与 Sandbox 验证主要基于上述 openeuler e2b-infra commit；两类目标环境的具体导入步骤见 [Template 离线导入与 Sandbox 启动测试操作说明](docs/TemplateImportDocs/Template离线注册并起沙箱操作说明.md)。

需要在同一进程中并发创建多个 Template 时，阅读[用户指南的并发创建章节](docs/TemplateCreator_Go_API_用户指南.md#templatecreator-concurrent-build)和[完整参考程序](examples/concurrent-go-api/main.go)。

## 能力概览

- `PrepareRuntime` 按版本锁定清单 `runtime-assets/manifest.json` 安装 kernel、Firecracker、envd 和 busybox 四个 runtime 资产到运行时目录，全部经过 SHA-256 校验；来源按优先级依次是运行时目录中已通过校验的现有文件、随 module 附带的 `bundled_path`（envd）、`reuse_paths` 本地复用（kernel、Firecracker、BusyBox），最后从 manifest 中指定的 `url` 下载；
- `CreateTemplate` 复用真实 infra Builder，在本地 Firecracker 虚拟机中完成Template构建，只有完整对象闭包和 completion manifest 都发布成功才返回成功；
- 同一个 Creator 支持多个 goroutine 并发调用 `CreateTemplate`，通过 `CreatorConfig.MaxParallel` 限制实际同时运行的构建数；每个物理构建独立分配 TemplateID、BuildID 和临时目录；
- `templatecreator-batch` 复用单个 Creator 有界并发处理多个请求，并按输入顺序汇总成功、失败和取消结果；
- 产物以 E2B 原生 `<BuildID>/<object>` 布局保存到本地 Store，并附带只生成、不执行的 `registration.sql`（用于在E2B中快速注册Template）；
- `templatecreator-demo` 构建成功后可自动导出原始产物目录；Go API 需显式调用 `ExportTemplateArtifacts`，也可用 `export-template` 从已有 manifest 重新导出。

## 快速开始

先准备 runtime：

```bash
sudo install -d -o "$(id -u)" -g "$(id -g)" \
  /opt/templatecreator/runtime /opt/templatecreator/scratch /opt/templatecreator/store

go run ./cmd/prepare-runtime \
  -manifest "$PWD/runtime-assets/manifest.json" \
  -runtime-root /opt/templatecreator/runtime \
  -arch "$(go env GOARCH)" \
  > /opt/templatecreator/runtime-info.json
```

然后使用集中默认值构建：

```bash
sudo go run ./cmd/templatecreator-demo \
  > build-result.json \
  2> build.log
```

stdout 只输出最终 JSON 结果，stderr 输出运行日志；更多用法见用户指南。

## CLI 命令

| 命令 | 作用 |
| --- | --- |
| `cmd/prepare-runtime` | 按 manifest 安装并校验 runtime 资产，输出 runtime-info.json |
| `cmd/templatecreator-demo` | preflight 检查 + 构建 Template + 发布到 Store + 自动导出 |
| `cmd/templatecreator-batch` | 单 Creator 有界并发创建多个 Template，汇总各项结果 |
| `cmd/export-template` | 从 Local Store 的 completion manifest 重新导出原始产物 |

各命令支持的参数可通过对应命令的 `-h` 查看；Go API 的配置、并发调用和结果处理见[Go API 用户指南](docs/TemplateCreator_Go_API_用户指南.md)。

## 默认值

所有默认值集中在 `defaults.go`，CLI 和 Go API 共用：

| 输入 | 默认值 |
| --- | --- |
| 基础镜像 | docker.io/library/debian:bookworm-slim |
| runtime-info / scratch / store | /opt/templatecreator/ 下的默认目录 |
| format | fromImage（fromDockerfile 对齐 only-FROM Dockerfile parser） |
| publication | all（simple 为最小运行闭包） |
| vCPU / memory / disk | 2 / 1024 MiB / 4096 MiB |
| 日志级别 | CLI 默认 info；Go API 不设置 `LogWriter` 时默认 off，提供 writer 后默认 info |

没有提供值时使用默认值；`BuildOverride` 的数值零值和空字符串通常继承 Creator 默认值，但非 nil 覆盖中的 `HugePages=false` 会明确关闭大页，非 nil `Steps` 会整体替换默认步骤。具体规则见 [Go API 用户指南](docs/TemplateCreator_Go_API_用户指南.md#templatecreator-request-options)。

## Go API

完整的公开类型、字段约束、继承规则、错误处理和导出 API 见：[TemplateCreator Go API 说明](docs/go-api.md)。

最小调用只需要 runtime 配置，构建输入可以省略：

```go
creator, err := templatecreator.NewCreator(ctx, templatecreator.CreatorConfig{
    Runtime:   prepared.Runtime,
    LogWriter: os.Stderr,
})
if err != nil {
    panic(err)
}
defer creator.Close(ctx)

result, err := creator.CreateTemplate(ctx, templatecreator.CreateRequest{})
```

空请求使用默认镜像、默认资源、fromImage 和 publication=all；只覆盖需要修改的字段：

```go
result, err := creator.CreateTemplate(ctx, templatecreator.CreateRequest{
    ImageRef:     "docker.io/library/ubuntu:24.04",
    Format:       templatecreator.BuildFormatFromDockerfile,
    Publication:  templatecreator.PublicationSimple,
    BuildOverride: &templatecreator.BuildSpec{MemoryMB: 2048},
})
```

本地 tar 输入与 `ImageRef` 互斥：

```go
result, err := creator.CreateTemplate(ctx, templatecreator.CreateRequest{
    ImageTarPath: "/opt/images/debian.tar",
})
```

### 并发创建多个 Template

一组请求共用一个 Creator，在初始化时设置并发上限：

```go
creator, err := templatecreator.NewCreator(ctx, templatecreator.CreatorConfig{
    Runtime:     prepared.Runtime,
    MaxParallel: 5, // 默认 1；允许最多五个物理构建同时执行。
    LogWriter:   os.Stderr,
})
```

随后用多个 goroutine 调用该 Creator 的 `CreateTemplate`，通过 `sync.WaitGroup` 等待全部返回并逐项检查结果，最后用独立的关停 context 调用 `Close`。`CreateTemplate` 对每个调用仍然同步返回；只设置 `MaxParallel` 而顺序调用不会产生并发。超过上限的构建等待空位，等待期间可以通过该请求的 context 取消。

[完整并发参考程序](examples/concurrent-go-api/main.go) 演示了五份请求、资源与 steps 覆盖、按请求顺序输出结果、错误汇总和 Creator 关闭。在 module 根目录运行：

```bash
go build -o /tmp/templatecreator-concurrent ./examples/concurrent-go-api
sudo /tmp/templatecreator-concurrent 2> concurrent-build.log
```

运行前修改程序顶部的 `manifestPath`、`imageTarPath` 和 `creatorConfig.MaxParallel`，并核对 `proxyPort`。该示例包含 `HugePages=true` 的请求，需预留足够大页或将其关闭；并发数还应与宿主内存、NBD 和磁盘容量匹配。程序将每项成功结果的 TemplateID、BuildID、ManifestKey 和 RegistrationSQL 路径写到 stderr，存在失败项时汇总错误并以非零状态退出。

不同 Template 应使用不同的 `IdempotencyKey`，或留空以执行新的构建。同一 Creator 内相同 key、相同内容的并发请求会等待并复用完成结果，不会创建多份 Template。详细调用片段与取消规则见 [Go API 并发说明](docs/go-api.md#32-并发创建示例)。

## 产物和导入

Store 中的 E2B object key 保持 `<BuildID>/<object>` 原样。默认还会把原始对象导出到 `<store-root>/exports`；简化 work-dir 模式默认导出到 work-dir。成功后会生成 completion manifest；`registration.sql` 位于 final BuildID 目录，但不属于 manifest，也不会自动执行。

> **注意**：
>
> 当前 E2B 启动沙箱时使用的 Template ID/alias 只允许：`a-z  0-9  -  _`，对应正则：`^[a-z0-9-_]+$`。建议构建程序的 `-id` 参数（主 CLI 中对应 `-idempotency-key`，Go API 中对应 `IdempotencyKey`）使用符合此要求的字符串，使生成的 `registration.sql` 中的 `template_alias` 可以直接使用。

导入目标 E2B 前必须：

1. 按 manifest 校验每个对象的 size 和 SHA-256；
2. 保持原始 object key；
3. 注册数据库并精确清理 Redis cache；
4. 创建真实 Sandbox 验证。

创建后的导入、注册、缓存处理和起沙箱测试见[离线导入操作说明](docs/TemplateImportDocs/Template离线注册并起沙箱操作说明.md)。
