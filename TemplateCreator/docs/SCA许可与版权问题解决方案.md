# SCA许可与版权问题解决方案

本文记录 core PR 流水线 SCA 扫描（license 与 copyright）报错的分析结论与解决方式，为临时工作文档；SCA 平台确认闭环后可归档或并入正式文档。

## 根因

SCA 将 `internal/upstream/` 快照文件与开源组件指纹库比对，命中三层匹配源：

| 匹配源 | 许可证 | 性质 |
| --- | --- | --- |
| `github.com/e2b-dev/infra`（多版本） | Apache-2.0 | 真实来源：本仓库快照锚定其 tag 2026.09 / commit `4853232b`（见 `third_party/INFRA_SOURCE.md`） |
| `gitcode/openeuler/kasandbox`、`github/jrimmer/spoond`、`github/aarani/hpcc` | 未知 / BSD-3-Clause / **AGPL-3.0** | 同源第三方副本：同样包含源自 e2b / firecracker 体系的代码，与快照文件指纹相似 80–91% |
| `firecracker-microvm/firecracker-go-sdk` 系 | Apache-2.0 | `shared/pkg/fc/client` 代码的更早源头（go-swagger 生成，Amazon 版权） |

报错分两类，修法不同：

- **copyright 报错**：快照文件无文件级版权声明（上游 e2b-infra 自身也只有仓库级 LICENSE），SCA 匹配停留在“未确认”。修法是文件级声明，可代码闭环。
- **license 报错**：主要因匹配到 `aarani/hpcc`（AGPL-3.0）触发高风险提示。本仓库真实来源链为 `firecracker-go-sdk (Apache-2.0) → e2b-infra (Apache-2.0) → 本仓库`，AGPL 匹配属指纹撞车而非真实来源。指纹相似度无法通过改代码消除，修法是 SCA 平台人工确认。

## 已实施的修复

### 1. 为 SCA 报错的 13 个 Go 文件恢复文件级版权头

- `internal/upstream/shared/pkg/fc/client/operations/` 下 11 个报错文件（`describe_instance_responses.go`、`patch_balloon_responses.go`、`patch_guest_drive_by_id_parameters.go`、`put_balloon_responses.go`、`put_cpu_configuration_parameters.go`、`put_cpu_configuration_responses.go`、`put_guest_drive_by_id_parameters.go`、`put_guest_network_interface_by_id_parameters.go`、`put_guest_network_interface_by_id_responses.go`、`put_guest_vsock_responses.go`、`put_metrics_parameters.go`）：添加双版权人头 `Copyright (c) 2023-2026 e2b Devs Inc.` + `Copyright (c) Amazon.com, Inc. or its affiliates.`，附 Apache-2.0 声明，覆盖 e2b 与 firecracker-go-sdk 两个匹配源；
- `internal/upstream/orchestrator/pkg/template/build/commands/copy.go` 与 `internal/upstream/shared/pkg/grpc/envd/process/processconnect/process.connect.go`：添加单版权人头 `Copyright (c) 2023-2026 e2b Devs Inc.`，附 Apache-2.0 声明；
- `SOURCE_MANIFEST.json` 对应 13 条 SHA-256 已重算，`.secrets.baseline` 同步刷新。

### 2. 补充 BusyBox 许可副本

`internal/upstream/orchestrator/pkg/template/build/core/systeminit/busybox_1.36.1-2` 为二进制资产（无法加文件头），SCA 100% 匹配 e2b-infra 同名 embed。其本体是 BusyBox 1.36.1 静态二进制（GPL-2.0），已补 `LICENSES/busybox-GPL-2.0.txt` 标准文本，并在 `third_party/INFRA_SOURCE.md` 的 License copy 小节登记。

### 3. 既有合规物（无需新增）

- `LICENSES/e2b-infra-APACHE-2.0.txt`：上游许可副本；
- `third_party/INFRA_SOURCE.md`：来源仓库、commit、提取方式与改写记录；
- 上游无 NOTICE 文件，Apache-2.0 的传播义务不涉及 NOTICE。

## 待平台侧闭环的操作（需要账号权限）

在 SCA / openlibing 平台对以下“未确认”条目执行人工确认，理由可参考：

- **e2b-dev/infra 各版本匹配条目**：确认为真实来源。代码自 `github.com/e2b-dev/infra` tag 2026.09（commit `4853232b93a8871b4ded73079184e801d84e0e26`）机械提取并改写 import 前缀，仓库保留其 Apache-2.0 许可副本（`LICENSES/e2b-infra-APACHE-2.0.txt`）与来源记录（`third_party/INFRA_SOURCE.md`）；相关文件已添加 e2b 版权头。
- **aarani/hpcc（AGPL-3.0）匹配条目**：确认为“同源第三方副本，非本仓库来源”。本仓库 fc/client 代码经 e2b-infra 溯源自 firecracker-go-sdk（Apache-2.0），与 hpcc 仓库内副本相似系同源代码所致；本仓库未引入 AGPL 代码，无传染路径。
- **jrimmer/spoond（BSD-3-Clause）、openeuler/KASandbox 匹配条目**：同上，均为同源第三方副本，非来源；真实来源链以 INFRA_SOURCE.md 为准。
- **busybox_1.36.1-2**：确认为 e2b-infra 快照携带的 BusyBox 1.36.1 二进制资产（GPL-2.0），许可副本已补 `LICENSES/busybox-GPL-2.0.txt`。

## 验证状态

- 加版权头后 `gofmt` / `go build ./...` 通过（纯注释变更）；
- `SOURCE_MANIFEST.json` 387 条与磁盘一致；
- `.secrets.baseline` 待审计清零，pre-commit detect-secrets 通过。

## 后续可选项

- 若 SCA 对版权头形式仍不确认（版权人写法与组件登记不完全一致），可在平台按实际登记形式微调；
- upstream 快照其余约 356 个未报错文件暂不加头；若后续 SCA 扩大报错范围，可按同样模板批量处理。
