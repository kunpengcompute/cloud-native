# TemplateCreator Go API 说明

本文说明 `gitcode.com/cloud-native/cloud-native/TemplateCreator` package 对外提供的 Go API、配置继承规则、错误处理和典型调用方式。运行条件、并发构建和产物导入见 [Go API 用户指南](TemplateCreator_Go_API_用户指南.md)；完整调用程序见 [concurrent-go-api/main.go](../examples/concurrent-go-api/main.go)。

## 1. 使用前提

TemplateCreator 的真实构建依赖 Linux、KVM、NBD、网络 namespace、Firecracker 以及相应系统权限。Go package 可以在其他平台参与编译或处理部分文件，但 `CreateTemplate` 在非 Linux 平台会返回不支持错误。

建议环境：

- Linux amd64 或 arm64，宿主架构与所选 runtime 架构一致；
- Go 版本满足本 module 的 `go.mod`；
- 构建进程拥有操作 KVM、NBD、网络和 firewall 的权限，当前联调方式通常使用 root；
- runtime manifest、scratch、Store 和导出路径均使用调用机上的绝对路径；
- 目标 E2B 具备与构建 metadata 一致的 architecture、kernel、Firecracker 和 envd ABI。

导入 package：

```go
import templatecreator "gitcode.com/cloud-native/cloud-native/TemplateCreator"
```

## 2. API 生命周期

一次完整调用通常包含四步：

1. `PrepareRuntime`：安装并校验 runtime 资产；
2. `NewCreator`：创建持有进程级共享资源的 Creator；
3. `Creator.CreateTemplate`：同步构建并发布 Template；多个 goroutine 可共用同一个 Creator 并发调用，由 `MaxParallel` 限制物理构建数；
4. 等待所有请求返回，再调用 `Creator.Close`：释放 proxy、firewall、NBD/network pool 等资源。

构建成功后可选调用：

- `ExportTemplateArtifacts`：导出原始对象目录树；
- `ExportTemplateBundle`：导出单个 `.tar.gz` 和 `.sha256` sidecar。

## 3. 调用示例

### 3.1 单个 Template 的完整最小示例

下面示例从 OCI 镜像构建 Template，并把已发布对象导出为目录树。所有路径都在调用前转成绝对路径；生产程序应根据自己的关停策略设置 timeout。

```go
package main

import (
    "context"
    "encoding/json"
    "fmt"
    "net/url"
    "os"
    "path/filepath"
    "runtime"
    "time"

    templatecreator "gitcode.com/cloud-native/cloud-native/TemplateCreator"
)

func main() {
    if err := run(); err != nil {
        panic(err)
    }
}

func run() error {
    workRoot := "/opt/templatecreator"
    runtimeRoot := filepath.Join(workRoot, "runtime")
    scratchRoot := filepath.Join(workRoot, "scratch")
    storeRoot := filepath.Join(workRoot, "store")
    exportRoot := filepath.Join(workRoot, "exports")
    manifestPath, err := filepath.Abs("runtime-assets/manifest.json")
    if err != nil {
        return err
    }

    for _, directory := range []string{runtimeRoot, scratchRoot, storeRoot, exportRoot} {
        if err := os.MkdirAll(directory, 0o755); err != nil {
            return fmt.Errorf("create %s: %w", directory, err)
        }
    }

    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
    defer cancel()

    prepared, err := templatecreator.PrepareRuntime(ctx, templatecreator.PrepareRuntimeRequest{
        ManifestPath: manifestPath,
        RuntimeRoot:  runtimeRoot,
        Architecture: runtime.GOARCH,
    })
    if err != nil {
        return fmt.Errorf("prepare runtime: %w", err)
    }

    creator, err := templatecreator.NewCreator(ctx, templatecreator.CreatorConfig{
        ScratchRoot: scratchRoot,
        Store: templatecreator.StoreConfig{
            URL: (&url.URL{Scheme: "file", Path: storeRoot}).String(),
        },
        Runtime:     prepared.Runtime,
        Defaults:    templatecreator.DefaultBuildSpec(),
        MaxParallel: 1,
        LogWriter:   os.Stderr,
        LogLevel:    templatecreator.LogLevelInfo,
    })
    if err != nil {
        return fmt.Errorf("create Creator: %w", err)
    }
    defer func() {
        closeCtx, closeCancel := context.WithTimeout(context.Background(), 30*time.Second)
        defer closeCancel()
        _ = creator.Close(closeCtx)
    }()

    result, err := creator.CreateTemplate(ctx, templatecreator.CreateRequest{
        ImageRef:       "docker.io/library/debian:bookworm-slim",
        IdempotencyKey: "debian-bookworm-v1",
        Labels:         map[string]string{"owner": "platform"},
    })
    if err != nil {
        return fmt.Errorf("create Template: %w", err)
    }

    exported, err := templatecreator.ExportTemplateArtifacts(ctx, templatecreator.ExportTemplateArtifactsRequest{
        Artifacts:  result.Artifacts,
        OutputRoot: exportRoot,
    })
    if err != nil {
        return fmt.Errorf("export artifacts: %w", err)
    }

    return json.NewEncoder(os.Stdout).Encode(struct {
        Result templatecreator.CreateResult                  `json:"result"`
        Export templatecreator.ExportTemplateArtifactsResult `json:"export"`
    }{Result: result, Export: exported})
}
```

`CreateTemplate` 是同步调用。context 取消会中断等待并发名额、下载、构建、发布或导出等支持取消的步骤。

### 3.2 并发创建示例

单任务示例将 `MaxParallel` 设为 1。需要并发时，在 `NewCreator` 中将它设为期望上限，例如 5，再从多个 goroutine 调用同一个 Creator。调用方负责提交请求和收集结果，Creator 负责限制物理构建数；仅提高 `MaxParallel` 并不会自动把顺序调用变为并发。

完成 runtime 准备并创建 `MaxParallel: 5` 的 Creator 后，可使用下列函数提交多份请求。函数需要 `context`、`sync` 和 TemplateCreator 的 package import；请求列表由调用方准备。

```go
type buildOutcome struct {
    Result templatecreator.CreateResult
    Err    error
}

func createConcurrently(
    ctx context.Context,
    creator *templatecreator.Creator,
    requests []templatecreator.CreateRequest,
) []buildOutcome {
    outcomes := make([]buildOutcome, len(requests))
    var group sync.WaitGroup
    for index := range requests {
        group.Add(1)
        go func(index int) {
            defer group.Done()
            // 每个 goroutine 只写自己的结果位置，等待全部返回后再统一读取。
            outcomes[index].Result, outcomes[index].Err =
                creator.CreateTemplate(ctx, requests[index])
        }(index)
    }
    group.Wait()
    return outcomes
}
```

返回切片与请求顺序一致，实际完成顺序可能不同。逐项检查 `Err`，保留成功项的 `CreateResult`，再按需导出或导入；某项失败不会自动取消其他请求。所有请求返回后，使用第 5.4 节的独立关停 context 调用 `Close`，并处理关闭错误。

[完整参考程序](../examples/concurrent-go-api/main.go) 包含 `PrepareRuntime → NewCreator → 并发 CreateTemplate → Close` 全流程，预置五份不同资源、格式和 steps 的请求，并汇总各项错误。在 module 根目录编译后运行：

```bash
go build -o /tmp/templatecreator-concurrent ./examples/concurrent-go-api
sudo /tmp/templatecreator-concurrent 2> concurrent-build.log
```

运行前调整程序顶部的 `manifestPath`、`imageTarPath`、`proxyPort` 和 `creatorConfig.MaxParallel`。示例从本地 docker save tar 读取镜像，其中一份请求启用 HugePages，需先准备宿主大页或改为关闭。结果摘要和运行日志均写到 stderr，成功项显示 TemplateID、BuildID、ManifestKey 与 RegistrationSQL 路径；失败项汇总后使程序非零退出。完整宿主准备步骤见 [用户指南](TemplateCreator_Go_API_用户指南.md#templatecreator-concurrent-build)。

并发调用还应遵守以下规则：

- `MaxParallel=0` 按 1 处理；例如提交五份请求但配置为 2，最多同时执行两个物理构建，其余等待。它不提供跨 Creator 或跨进程的统一限流。
- proxy、firewall、NBD/network pool 由 Creator 共享；每个物理构建的 Builder、可写 storage 和临时目录独立。按并发峰值准备内存、HugePages、NBD 和磁盘空间，增加并发数不保证耗时按比例缩短。
- 共享 context 被取消会影响所有使用它的请求，包括正在排队的请求。若需要单项超时或取消，为各项创建独立子 context；示例的 30 分钟 context 覆盖准备、初始化、排队及整批构建，并非每项自动获得 30 分钟。
- 不同 Template 使用不同的 `IdempotencyKey`，或留空。相同 key 的并发请求在同一个 Creator 内串行检查完成记录；同内容复用成功结果，不同内容冲突。此锁不提供跨 Creator 或跨进程协调。
- 调用期间不要修改请求共享的 `Labels`、`BuildOverride` 或 `Steps`；调用方提供的共享凭据 provider 与日志 writer 也应支持并发访问。
- `Close` 会拒绝新请求并等待已接纳请求，包含排队中的请求；它不会主动取消构建。需要提前停止时，先取消构建 context，再等待请求返回并关闭 Creator。

## 4. Runtime API

### 4.1 `PrepareRuntime`

```go
func PrepareRuntime(
    ctx context.Context,
    request PrepareRuntimeRequest,
) (PrepareRuntimeResult, error)
```

该函数严格解析 runtime manifest，选择一个 architecture 的 kernel、Firecracker、envd 和 BusyBox，校验 SHA-256 后安装到 `RuntimeRoot`。它不会修改进程级环境变量。

单个资产的来源优先级为：

```text
RuntimeRoot 中已有且 checksum 正确的 destination
  -> manifest bundled_path
  -> manifest reuse_paths（按数组顺序）
  -> manifest url 下载
```

来源存在但 checksum 错误不会被安装。最终写入通过目标目录内的临时文件和原子 rename 完成。

### 4.2 `PrepareRuntimeRequest`

| 字段 | 规则 |
| --- | --- |
| `ManifestPath` | 必填，绝对路径。相对 `bundled_path` 以此文件所在目录为基准。 |
| `RuntimeRoot` | 必填，绝对目录；安装后的布局必须符合 Builder contract。 |
| `Architecture` | 可选；空值使用当前进程 `runtime.GOARCH`。manifest 必须包含该架构完整四件套。 |
| `E2BInfraRoot` | 可选，绝对路径；把 manifest 中 `/fc-*` 等 `reuse_paths` 重映射到一个已有 infra 根文件系统。 |
| `Timeout` | 每个 HTTP 下载的超时；0 使用 5 分钟，负值无效。 |

### 4.3 `PrepareRuntimeResult`

| 字段 | 作用 |
| --- | --- |
| `Runtime` | 根据实际安装结果生成的 `RuntimeConfig`；应直接传给 `CreatorConfig.Runtime`。 |
| `Assets` | 每个已确认资产的类型、版本、架构、最终路径、SHA-256、来源类型和来源路径。 |

`PreparedRuntimeAsset.Source` 可能为：

- `existing`：目标位置已有合法文件；
- `bundled`：使用 manifest 附带文件；
- `local_reuse`：复用已有 infra 文件；
- `download`：从固定 URL 下载。

不要手工伪造 `RuntimeConfig` 或只修改 manifest checksum。snapshot 能否恢复取决于整套 runtime ABI。

## 5. Creator 生命周期 API

### 5.1 `NewCreator`

```go
func NewCreator(ctx context.Context, config CreatorConfig) (*Creator, error)
```

`NewCreator` 会：

- 应用进程级默认值并验证配置；
- 对 kernel、Firecracker、envd、BusyBox 逐个检查常规文件、执行权限和 SHA-256；
- 检测构建机 CPU，要求它与 `Runtime.Architecture` 一致；
- 初始化日志、feature flags、sandbox map、proxy、TCP firewall、NBD pool 和 network pool；
- 在 Store 中初始化幂等完成记录索引。

成功返回后，调用方必须执行 `Close`。Creator 可以处理多个请求，并用 `MaxParallel` 限制该 Creator 内真正运行的物理构建数。初始化 context 用于启动阶段；成功创建后的进程级 proxy、firewall 和资源池由 `Close` 停止，不会因为该初始化 context 后续取消而提前停掉在途构建。

### 5.2 `CreatorConfig`

| 字段 | 零值/约束 | 作用 |
| --- | --- | --- |
| `ScratchRoot` | 空值使用 `/opt/templatecreator/scratch`；必须是绝对路径 | 每个物理 build 的临时 job 根目录，结束时清理。 |
| `Store.URL` | 空值使用 `DefaultStoreURL()` | 最终对象、manifest、幂等记录、SQL 和 info；当前仅支持指向绝对目录的 `file://` URL。 |
| `Runtime` | 必填 | 固定 Creator 内所有构建使用的 runtime ABI；推荐直接使用 `PrepareRuntimeResult.Runtime`。 |
| `Cache` | 零值关闭 | 持久 base-layer cache 配置。infra 2026.09 adapter 当前不支持启用，`Enabled=true` 会返回明确错误。 |
| `Defaults` | 资源字段 0 使用集中默认值 | 所有请求的资源、命令和 steps 默认值。可直接使用 `DefaultBuildSpec()`。 |
| `Format` | 空值为 `fromImage` | Creator 级默认构建格式。 |
| `Publication` | 空值为 `all` | Creator 级默认发布策略。 |
| `MaxParallel` | 0 归一化为 1；负值无效 | 当前 Creator 同时执行的物理构建上限。幂等命中不占名额。 |
| `LogWriter` | nil 不输出 | 接收 TemplateCreator 和上游 Builder 日志；CLI/服务通常传 stderr 或独立日志 writer。 |
| `LogLevel` | `off`、`info`、`debug` | 显式日志级别。空值时：`DebugLogs=true` 为 debug，有 writer 为 info，否则 off。 |
| `DebugLogs` | 已弃用 | 仅在 `LogLevel` 为空时保留兼容；新代码使用 `LogLevel`。 |

`ScratchRoot`、Store 以及未来的 cache root 不能互相重叠，避免 job 清理误删持久产物。

`StoreConfig` 当前只有 `URL` 字段。`CacheConfig` 包含 `Enabled`、绝对目录 `Root` 和 `AllowMutableImageTags`；后者用于显式允许可变 OCI tag 进入共享 cache。由于 infra 2026.09 adapter 尚未完成共享 cache 适配，目前应保持 `CacheConfig{}`，不要启用。

### 5.3 `RuntimeConfig`

`RuntimeConfig` 固定 snapshot ABI 和 Builder 资产布局：

| 字段组 | 字段 |
| --- | --- |
| 版本来源 | `InfraCommit`、`TemplateVersion`、`Architecture` |
| kernel | `RuntimeDir`、`KernelVersion`、`KernelSHA256` |
| Firecracker | `RuntimeDir`、`FirecrackerVersion`、`FirecrackerSHA256` |
| envd | `EnvdVersion`、`EnvdPath`、`EnvdSHA256` |
| BusyBox | `BusyboxDir`、`BusyboxVersion`、`BusyboxSHA256` |
| 网络 | `ProxyPort`；0 使用默认端口 5007 |

Builder 返回后，TemplateCreator 会再次比较实际 envd/kernel/Firecracker 版本；不一致时不会发布产物。

### 5.4 `Close`

```go
func (creator *Creator) Close(ctx context.Context) error
```

`Close` 先拒绝新请求，再等待已经接纳的 `CreateTemplate` 返回，最后停止后台 producer、关闭并排空 network/NBD pool、firewall、proxy 和 feature flag client。新请求返回 `ErrCreatorClosed`。若本次 `Close` 的等待 context 到期，该调用返回 context 错误，但不会提前释放在途构建使用的资源；后台清理继续进行，后续 `Close` 可等待并取得最终关闭结果。资源清理使用独立的有界 context。

推荐用独立的关停 context，不要直接复用已经超时的 build context：

```go
closeCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()
if err := creator.Close(closeCtx); err != nil {
    log.Printf("close Creator: %v", err)
}
```

## 6. 构建 API

### 6.1 `CreateTemplate`

```go
func (creator *Creator) CreateTemplate(
    ctx context.Context,
    request CreateRequest,
) (CreateResult, error)
```

该方法同步完成验证、真实 Builder 构建、ABI 校验、闭包解析和发布。只有所有对象写入成功且 completion manifest 最后原子写入成功时，才返回非空 `CreateResult`。

成功发布后，程序还会尽力生成 `registration.sql` 和 `<TemplateID>/info.json`。这两个辅助文件写入失败只记录 warning，不推翻已经完成的 Template。

### 6.2 镜像输入

`CreateRequest` 支持两种互斥输入：

```go
// 从 registry 拉取。
templatecreator.CreateRequest{
    ImageRef: "docker.io/library/ubuntu:24.04",
}

// 使用 docker save 归档。
templatecreator.CreateRequest{
    ImageTarPath: "/opt/images/ubuntu.tar",
}
```

两者都为空时，API 使用 `DefaultImageRef`；两者同时非空时返回 `ErrInvalidRequest`。本地 tar 内容的 SHA-256 会进入幂等请求摘要，替换同一路径下的 tar 不会错误复用旧结果。

### 6.3 `CreateRequest`

| 字段 | 作用 |
| --- | --- |
| `ImageRef` | OCI 基础镜像引用，与 `ImageTarPath` 互斥。 |
| `ImageTarPath` | 本地 `docker save` tar，建议使用绝对路径，与 `ImageRef` 互斥。 |
| `BuildOverride` | 单次覆盖 `CreatorConfig.Defaults`；nil 表示完全继承。 |
| `Format` | 单次覆盖 Creator 的 `BuildFormat`；空值继承。 |
| `Publication` | 单次覆盖 Creator 的 `PublicationMode`；空值继承。 |
| `RegistryAuth` | 私有 registry 的按需凭据 provider；不持久化、不记录、不进入幂等摘要。仅 `ImageRef` 输入使用。 |
| `IdempotencyKey` | 可选稳定任务编号；同键同内容复用，同键不同内容返回 `ErrIdempotencyConflict`；该值还会作为 `registration.sql` 中的默认 `template_alias`。 |
| `Labels` | 复制到 completion manifest，并参与幂等请求摘要。不要放 secret。 |
| `BypassBaseCache` | 要求物理构建绕过旧 base cache；不绕过同 `IdempotencyKey` 的成功结果复用。当前 2026.09 adapter 不支持启用持久 cache。 |

> **注意**：
>
> 当前 E2B 启动沙箱时使用的 Template ID/alias 只允许：`a-z  0-9  -  _`，对应正则：`^[a-z0-9-_]+$`。建议 `IdempotencyKey` 使用符合此要求的字符串，使生成的 `registration.sql` 中的 `template_alias` 可以直接使用。

### 6.4 `BuildSpec` 的继承规则

`BuildSpec` 同时用于 Creator 默认值和单次 override：

| 字段 | 默认值 | 作为 `BuildOverride` 时 |
| --- | ---: | --- |
| `VCPU` | 2 | 0 继承，非 0 必须为正数。 |
| `MemoryMB` | 1024 MiB | 0 继承，非 0 必须为正数。 |
| `DiskSizeMB` | 4096 MiB | 0 继承，非 0 必须为正数。 |
| `HugePages` | false | 只要 override 非 nil，该 bool 就显式覆盖默认值。 |
| `StartCommand` | 空 | 空字符串继承，非空覆盖。 |
| `ReadyCommand` | 空 | 空字符串继承，非空覆盖。 |
| `Steps` | 由 format 决定 | nil 继承；非 nil 整体替换；显式空切片清空 steps。 |

如果 Creator 默认 `HugePages=true`，但单次只想覆盖内存，override 中也要再次填写 `HugePages: true`。同理，单次 override 无法用空字符串清除非空 Creator 默认命令；需要调整 Creator 默认配置。

示例：

```go
result, err := creator.CreateTemplate(ctx, templatecreator.CreateRequest{
    ImageRef: "docker.io/library/ubuntu:24.04",
    BuildOverride: &templatecreator.BuildSpec{
        MemoryMB:    2048,
        ReadyCommand: "test -f /tmp/ready",
    },
})
```

### 6.5 `BuildFormat`

| 常量 | 语义 |
| --- | --- |
| `BuildFormatFromImage` (`fromImage`) | 直接从镜像构建；默认不注入请求 steps。 |
| `BuildFormatFromDockerfile` (`fromDockerfile`) | 兼容 only-FROM Dockerfile 的 SDK 语义；`Steps=nil` 时注入 root `/` 到 user `/home/user` 的默认 USER/WORKDIR 上下文步骤。 |

`Format` 不解析任意 Dockerfile；实际 provisioning 通过 `TemplateStep` 提供。

### 6.6 `TemplateStep`

```go
type TemplateStep struct {
    Type string
    Args []string
}
```

公开调用常用类型和参数：

| Type | Args |
| --- | --- |
| `RUN` | `[命令, 可选用户]` |
| `USER` | `[用户名, 可选 sudo 标记]` |
| `WORKDIR` | `[目录]` |
| `ENV` / `ARG` | 成对的 `[key, value, ...]` |

示例：

```go
steps := []templatecreator.TemplateStep{
    {Type: "RUN", Args: []string{"apt-get update && apt-get install -y curl", "root"}},
    {Type: "WORKDIR", Args: []string{"/app"}},
    {Type: "ENV", Args: []string{"APP_ENV", "production"}},
}

result, err := creator.CreateTemplate(ctx, templatecreator.CreateRequest{
    ImageRef: "docker.io/library/debian:bookworm-slim",
    BuildOverride: &templatecreator.BuildSpec{Steps: steps},
})
```

上游 Builder 还包含 `COPY/ADD` 实现，但它们依赖额外的文件上传上下文；当前稳定公开 API 没有暴露该上下文，不应把它们当作普通 `TemplateStep` 使用。

### 6.7 `PublicationMode`

| 常量 | 语义 |
| --- | --- |
| `PublicationAll` (`all`) | 默认；发布 Builder staging 中的全部对象，便于调试和保留中间 layer。 |
| `PublicationSimple` (`simple`) | 只发布 final header 实际引用的 ancestor/object 最小运行闭包。 |

两种模式都会生成包含实际对象列表、size 和 SHA-256 的 completion manifest。

### 6.8 私有 registry

实现 `RegistryAuthProvider`：

```go
type envRegistryAuth struct{}

func (envRegistryAuth) Credentials(
    ctx context.Context,
    imageRef string,
) (templatecreator.RegistryCredentials, error) {
    return templatecreator.RegistryCredentials{
        Username: os.Getenv("REGISTRY_USERNAME"),
        Password: os.Getenv("REGISTRY_PASSWORD"),
    }, nil
}

result, err := creator.CreateTemplate(ctx, templatecreator.CreateRequest{
    ImageRef:       "registry.example.com/team/app:1.0",
    RegistryAuth:   envRegistryAuth{},
    IdempotencyKey: "team-app-1.0",
})
```

Provider 应返回短期凭据，不应自行记录 password/token。TemplateCreator 不会把凭据写入 metadata、任务摘要、Store manifest 或导出 bundle。

### 6.9 幂等语义

设置 `IdempotencyKey` 后：

- 相同 key、相同安全请求内容：直接复用已经发布的 `CreateResult`；
- 相同 key、不同请求内容：返回 `ErrIdempotencyConflict`；
- 并发相同 key：同一个 key lock 覆盖构建和发布全过程，其他调用不会看到半成品；
- 未设置 key：每次都分配新的 TemplateID/BuildID 并执行物理构建。

摘要包含镜像（本地 tar 还包含内容 hash）、解析后的 BuildSpec、format、publication、labels 和 runtime ABI；不包含 registry credential。`BypassBaseCache` 不改变产物内容摘要，同 key 的已完成结果仍优先复用。

## 7. 结果和 manifest API

### 7.1 `CreateResult`

| 字段 | 作用 |
| --- | --- |
| `TemplateID` | E2B 风格的公开 Template ID。 |
| `BuildID` | final 物理 build UUID，也是对象 key 的首段。 |
| `Artifacts` | `StoreURL` 和 `ManifestKey`，定位完成标记。 |
| `Metadata` | 实际 format、publication、runtime ABI、rootfs 大小、资源、调度摘要和构建机 CPU 事实。 |
| `Registration` | 生成的 `registration.sql` object key、本地路径和执行前警告。SQL 未被执行。 |

### 7.2 `BuildMetadata`

`BuildMetadata` 是构建结果的审计记录：

- 源版本：`InfraCommit`、`TemplateVersion`；
- runtime：`Architecture`、`EnvdVersion`、`KernelVersion`、`FirecrackerVersion`；
- 构建：`Format`、`Publication`、`RootfsSizeMB`、完整 `Resources`；
- 调度：`SchedulingMetadata`；
- 宿主 CPU：`MachineInfo`。

目标 E2B 必须根据真实 runtime 和 CPU 能力调度。`MachineInfo` 记录构建机事实，但单独存在不构成目标节点兼容证明。

### 7.3 `ArtifactManifest` 和 `ArtifactObject`

`ArtifactManifest` 是 completion manifest 的公开类型别名，schema version 可通过 `ManifestSchemaVersion` 检查。它包含：

- TemplateID、BuildID 和镜像输入；
- format、publication、infra commit 和构建时间；
- 完整 `Objects []ArtifactObject`；
- ancestor Build IDs；
- 原始 `BuildMetadata` JSON；
- labels 和可选幂等摘要。

`ArtifactObject` 包含对象 `Key`、`Size` 和 `SHA256`。导入目标 E2B 前必须逐项校验，并保持原始 slash-separated key。

读取 manifest：

```go
data, err := os.ReadFile(manifestPath)
if err != nil {
    return err
}
var manifest templatecreator.ArtifactManifest
if err := json.Unmarshal(data, &manifest); err != nil {
    return err
}
if manifest.SchemaVersion != templatecreator.ManifestSchemaVersion {
    return fmt.Errorf("unsupported manifest schema %d", manifest.SchemaVersion)
}
```

## 8. 导出 API

两种导出函数都只读取 `file://` Local Store，先按 manifest 验证对象路径、size 和 SHA-256，不接触 E2B 控制面。

### 8.1 `ExportTemplateArtifacts`

```go
func ExportTemplateArtifacts(
    ctx context.Context,
    request ExportTemplateArtifactsRequest,
) (ExportTemplateArtifactsResult, error)
```

请求字段：

- `Artifacts`：通常直接使用 `CreateResult.Artifacts`；
- `OutputRoot`：必填绝对目录。

输出布局：

```text
<OutputRoot>/<TemplateID>/manifest.json
<OutputRoot>/<TemplateID>/<BuildID>/<object>
<OutputRoot>/<TemplateID>/<AncestorBuildID>/<object>
```

结果包含 TemplateID、BuildID、对象数量、Template 输出目录和对象总大小。

### 8.2 `ExportTemplateBundle`

```go
func ExportTemplateBundle(
    ctx context.Context,
    request ExportBundleRequest,
) (ExportBundleResult, error)
```

`OutputPath` 必须是绝对文件路径。函数原子生成 gzip 压缩的 tar，并在 `<OutputPath>.sha256` 写入 bundle SHA-256。

```go
bundle, err := templatecreator.ExportTemplateBundle(ctx, templatecreator.ExportBundleRequest{
    Artifacts:  result.Artifacts,
    OutputPath: "/opt/templatecreator/exports/debian-bookworm.tar.gz",
})
```

结果包含 bundle 路径、sidecar 路径、SHA-256、大小和对象数量。

## 9. 错误处理

### 9.1 稳定 sentinel

| 错误 | 含义 |
| --- | --- |
| `ErrInvalidConfig` | `CreatorConfig`、runtime、Store、默认 BuildSpec 等进程配置无效。 |
| `ErrInvalidRequest` | 镜像输入、单次 BuildSpec、format 或 publication 无效。 |
| `ErrIdempotencyConflict` | 相同任务编号已经对应另一份安全请求内容。 |
| `ErrCreatorClosed` | Creator 已开始关闭，不再接纳新的构建请求。 |

用 `errors.Is` 判断：

```go
if errors.Is(err, templatecreator.ErrIdempotencyConflict) {
    // 更换 IdempotencyKey，或恢复与原任务完全相同的输入。
}
```

### 9.2 `BuildError`

构建分配 ID 后的阶段错误通常包装为 `*BuildError`：

```go
var buildErr *templatecreator.BuildError
if errors.As(err, &buildErr) {
    log.Printf("template=%s build=%s phase=%s diagnostics=%s cause=%v",
        buildErr.TemplateID,
        buildErr.BuildID,
        buildErr.Phase,
        buildErr.DiagnosticsLocation,
        buildErr.Unwrap(),
    )
}
```

公开阶段常量：

- `BuildPhaseValidate`
- `BuildPhasePrepare`
- `BuildPhasePullImage`
- `BuildPhaseBuild`
- `BuildPhasePublishObjects`
- `BuildPhasePublishManifest`
- `BuildPhaseCleanup`

`BuildError` 不保存完整请求或 registry credential，可以安全地作为结构化错误记录；底层错误仍可继续用 `errors.Is`/`errors.As` 检查。

## 10. 默认值 API

### 10.1 函数

```go
func DefaultBuildSpec() BuildSpec
func DefaultStoreURL() string
```

- `DefaultBuildSpec` 返回 2 vCPU、1024 MiB 内存和 4096 MiB disk；`Steps=nil`，由 format 决定默认 steps；
- `DefaultStoreURL` 返回 `DefaultStoreRoot` 对应的绝对 `file://` URL。

### 10.2 导出常量

| 常量 | 值/作用 |
| --- | --- |
| `DefaultImageRef` | `docker.io/library/debian:bookworm-slim` |
| `DefaultRuntimeInfoPath` | `/opt/templatecreator/runtime-info.json`，主要供 CLI 约定使用；Go API 不自动读取。 |
| `DefaultScratchRoot` | `/opt/templatecreator/scratch` |
| `DefaultStoreRoot` | `/opt/templatecreator/store` |
| `DefaultExportRoot` | 空字符串，由具体 CLI/调用方决定导出位置。 |
| `DefaultBuildTimeout` | 30 分钟；Go API 不自动创建 timeout，调用方应放入 context。 |
| `DefaultVCPU` | 2 |
| `DefaultMemoryMB` | 1024 |
| `DefaultDiskSizeMB` | 4096 |
| `DefaultMaxParallel` | 1 |
| `DefaultProxyPort` | 5007 |
| `DefaultLogLevel` | `info`，CLI 默认使用；纯 Go API 在 writer 为 nil 且 level 为空时实际关闭日志。 |
| `DefaultBuildFormat` | `BuildFormatFromImage` |
| `DefaultPublicationMode` | `PublicationAll` |

## 11. 公开符号索引

| 类别 | 公开符号 |
| --- | --- |
| Runtime | `PrepareRuntime`、`PrepareRuntimeRequest`、`PrepareRuntimeResult`、`PreparedRuntimeAsset`、`RuntimeConfig` |
| Creator | `NewCreator`、`Creator`、`CreatorConfig`、`StoreConfig`、`CacheConfig`、`Creator.Close` |
| Build | `Creator.CreateTemplate`、`CreateRequest`、`CreateResult`、`BuildSpec`、`TemplateStep` |
| 镜像认证 | `RegistryAuthProvider`、`RegistryCredentials` |
| 构建选择 | `BuildFormat`、`BuildFormatFromImage`、`BuildFormatFromDockerfile`、`PublicationMode`、`PublicationSimple`、`PublicationAll` |
| 日志 | `LogLevel`、`LogLevelOff`、`LogLevelInfo`、`LogLevelDebug` |
| Metadata | `BuildMetadata`、`MachineInfo`、`SchedulingMetadata`、`RegistrationSQLResult` |
| 产物 | `ArtifactLocation`、`ArtifactManifest`、`ArtifactObject`、`ManifestSchemaVersion` |
| 导出目录 | `ExportTemplateArtifacts`、`ExportTemplateArtifactsRequest`、`ExportTemplateArtifactsResult` |
| 导出 bundle | `ExportTemplateBundle`、`ExportBundleRequest`、`ExportBundleResult` |
| 错误 | `ErrInvalidConfig`、`ErrInvalidRequest`、`ErrIdempotencyConflict`、`ErrCreatorClosed`、`BuildError`、`BuildPhase` 及各阶段常量 |
| 默认值 | `DefaultBuildSpec`、`DefaultStoreURL` 及全部 `Default*` 常量 |

## 12. 集成建议

- 长生命周期服务通常创建一个 Creator 并复用，不要为每个请求重新初始化 NBD/network/proxy；
- 在一个 Creator 上并发调用多个 `CreateTemplate` 时，先等待请求返回，再调用 `Close`；调用方式见 [Go API 用户指南](TemplateCreator_Go_API_用户指南.md#templatecreator-concurrent-build) 和 [完整参考程序](../examples/concurrent-go-api/main.go)；
- 给每次构建设置 context deadline，给 `Close` 使用独立关停 context；
- `MaxParallel` 只控制当前 Creator，多个进程仍会竞争宿主 KVM/NBD/网络资源；
- 生产任务使用稳定、唯一且可追踪的 `IdempotencyKey`，请求内容变化时更换 key；
- 日志写专用 writer，不要与返回 JSON 共用 stdout；
- 不在 labels、错误、日志或 registry provider 中泄露 secret；
- 使用 `PrepareRuntimeResult.Runtime`，并把最终 `BuildMetadata` 与产物一起交付目标环境；
- 把 completion manifest 当作唯一完成标记，不根据单个对象存在与否判断构建成功；
- `registration.sql` 执行前必须替换/确认 team、cluster、namespace、alias 等目标环境参数，执行后仍需精确清理缓存并创建真实 Sandbox 验证。
