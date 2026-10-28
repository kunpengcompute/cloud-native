# E2B infra source provenance

TemplateCreator contains a source snapshot derived from E2B infra. The snapshot is used to build the module itself; callers do not need the maintainer's local infra checkout.

| Field | Value |
| --- | --- |
| Upstream repository | <https://github.com/e2b-dev/infra> |
| Maintainer input path | `/home/fsq/Desktop/fsq/infra` (not a caller dependency) |
| Upstream commit | `4853232b93a8871b4ded73079184e801d84e0e26` |
| Upstream description | tag `2026.09` |
| Upstream license | Apache License 2.0 |
| Extraction target | `internal/upstream/` |
| Extraction method | Deterministic package-list-driven source sync via `cmd/sync-infra-source` |

## Extracted scope

`scripts/infra-packages.txt` is derived from the locked `cmd/create-build` local dependency closure, excluding the command package itself. It contains 80 orchestrator/shared/clickhouse package directories.

The generated `internal/upstream/SOURCE_MANIFEST.json` records 387 payload files and their SHA-256 values. The snapshot contains 388 files including the manifest itself and occupies approximately 4.2 MiB.

## Local modifications

The sync tool rewrites the three upstream module import prefixes to TemplateCreator's `internal/upstream` paths. It also applies two deterministic namespace mappings required by the older source layout:

- `orchestrator/internal/*` becomes `internal/upstream/orchestrator/pkg/*`;
- `shared/pkg/feature-flags` becomes `internal/upstream/shared/pkg/featureflags`.

The current module prefix is `gitcode.com/boostkit/cloud-native.git/TemplateCreator`; `.git` identifies the parent Git repository for Go module discovery. The source sync tool and snapshot imports use this same prefix, and manifest checksums are refreshed for files changed by the module-path migration.

The manifest records 261 files as mechanically modified, including files whose destination changed only because of a namespace mapping. Modified Go files carry a prominent TemplateCreator modification notice while preserving build tags and embed directives. The namespace mapping is intentionally mechanical: TemplateCreator-specific Builder/runtime adaptation remains outside the snapshot under `internal/runtime`.

There are no behavioral or other non-mechanical upstream source patches in the current snapshot. TemplateCreator-specific runtime assembly is implemented outside the snapshot under `internal/runtime`.

The module pins direct dependencies to the versions resolved by the locked infra Go workspace and preserves its `github.com/willscott/go-nfs` replacement. Production code, `go.mod`, and generated files do not depend on the maintainer input path.

## License copy

The upstream license is stored at `LICENSES/e2b-infra-APACHE-2.0.txt`.
The snapshot also embeds the BusyBox 1.36.1 binary (`orchestrator/pkg/template/build/core/systeminit/busybox_1.36.1-2`, GPL-2.0); its license copy is stored at `LICENSES/busybox-GPL-2.0.txt`.

```text
SHA-256: b4ef1bf811cb4095229fb86b574199e467c78f0ac4078cf8be62189e1fbd0818
```
