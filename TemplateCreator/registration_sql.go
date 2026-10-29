package templatecreator

import (
	"encoding/json"
	"fmt"
	"net/url"
	"path"
	"path/filepath"
	"strings"
)

const (
	registrationSQLFileName        = "registration.sql"
	defaultRegistrationTeamID      = "REPLACE_WITH_TEAM_ID"
	defaultRegistrationNamespace   = "local-dev-team"
	defaultRegistrationClusterID   = ""
	defaultRegistrationClusterNode = "local"
	defaultRegistrationAlias       = "REPLACE_WITH_TEMPLATE_ALIAS"
)

// registrationSQLRequest 只包含渲染 SQL 所需的构建事实。
// team/namespace 等目标侧参数不进入 CreateRequest，避免把同一 Template 绑定到单个 E2B。
type registrationSQLRequest struct {
	StoreURL       string
	TemplateID     string
	BuildID        string
	ImageRef       string
	IdempotencyKey string
	Metadata       BuildMetadata
}

// renderedRegistrationSQL 将公开结果元数据与仅供内部落盘的 SQL 正文分开。
// SQL 正文不进入 CreateResult，避免 stdout 和 info.json 重复携带完整脚本。
type renderedRegistrationSQL struct {
	Result   RegistrationSQLResult
	Contents string
}

// renderRegistrationSQL 生成针对锁定 infra schema 的 psql 脚本。
// 脚本提供本地部署默认值，同时允许调用者通过 psql -v 覆盖目标侧参数。
func renderRegistrationSQL(request registrationSQLRequest) (renderedRegistrationSQL, error) {
	if err := validateFileStoreURL(request.StoreURL); err != nil {
		return renderedRegistrationSQL{}, fmt.Errorf("registration SQL StoreURL: %w", err)
	}
	parsed, err := url.Parse(request.StoreURL)
	if err != nil {
		return renderedRegistrationSQL{}, fmt.Errorf("parse registration SQL StoreURL: %w", err)
	}

	alias := request.IdempotencyKey
	warnings := []string{
		"team_id 当前为 REPLACE_WITH_TEAM_ID，执行前必须替换为目标 E2B 中真实存在的 team UUID。",
		"cluster_id 本地默认留空并以 NULL 写入；目标 E2B 使用远程 cluster 时必须覆盖为 clusters 表中真实存在的 UUID。",
		"namespace=local-dev-team、cluster_node_id=local 仅是本地部署默认值，跨环境时必须确认并修改。",
		"执行 PostgreSQL SQL 后仍需清理目标 E2B 的 Redis Template/Build/alias cache，并创建真实 Sandbox 验证。",
	}
	if alias == "" {
		alias = defaultRegistrationAlias
		warnings = append(warnings, "任务编号为空，template_alias 使用 REPLACE_WITH_TEMPLATE_ALIAS，执行前必须修改。")
	} else {
		warnings = append(warnings, "template_alias 默认使用任务编号；若不符合目标命名规则或已存在冲突，执行前必须修改。")
	}

	cpuFlags := request.Metadata.MachineInfo.CPUFlags
	if cpuFlags == nil {
		// PostgreSQL 的 jsonb_array_elements_text 只接受数组；nil slice 如果编码为
		// JSON null 会在执行注册 SQL 时失败，因此稳定归一化为空数组。
		cpuFlags = []string{}
	}
	cpuFlagsJSON, err := json.Marshal(cpuFlags)
	if err != nil {
		return renderedRegistrationSQL{}, fmt.Errorf("encode registration SQL CPU flags: %w", err)
	}

	objectKey := path.Join(request.BuildID, registrationSQLFileName)
	localPath := filepath.Join(parsed.Path, filepath.FromSlash(objectKey))
	var sql strings.Builder
	sql.WriteString("-- TemplateCreator 为当前 Template 生成的 PostgreSQL 注册 SQL。\n")
	sql.WriteString("-- 重要：以下目标 E2B 参数只是便于本地联调的预填值，执行前必须逐项确认。\n")
	for _, warning := range warnings {
		fmt.Fprintf(&sql, "-- 提示：%s\n", warning)
	}
	sql.WriteString("-- 可使用 psql -v team_id=... -v cluster_id=... -v namespace=... -v template_alias=... -v cluster_node_id=... -f registration.sql 覆盖默认值。\n\n")
	sql.WriteString("\\set ON_ERROR_STOP on\n")
	writePSQLDefault(&sql, "team_id", defaultRegistrationTeamID)
	writePSQLDefault(&sql, "cluster_id", defaultRegistrationClusterID)
	writePSQLDefault(&sql, "namespace", defaultRegistrationNamespace)
	writePSQLDefault(&sql, "cluster_node_id", defaultRegistrationClusterNode)
	writePSQLDefault(&sql, "template_alias", alias)
	sql.WriteString("\n\\echo '提示：执行前请确认 team_id、cluster_id、namespace、cluster_node_id 和 template_alias；必要时使用 -v 覆盖。'\n")
	sql.WriteString("\\echo '提示：SQL 成功后仍需清理 Redis cache，并创建 Sandbox 验证。'\n\n")
	sql.WriteString("BEGIN;\n\n")
	fmt.Fprintf(&sql, "INSERT INTO envs (id, created_at, updated_at, public, build_count, spawn_count, team_id, cluster_id, source)\nVALUES (%s, now(), now(), false, 1, 0, :'team_id'::uuid, NULLIF(:'cluster_id', '')::uuid, 'template');\n\n", sqlString(request.TemplateID))
	sql.WriteString("INSERT INTO env_builds (\n")
	sql.WriteString("  id, created_at, updated_at, finished_at, status, dockerfile, start_cmd,\n")
	sql.WriteString("  vcpu, ram_mb, free_disk_size_mb, total_disk_size_mb,\n")
	sql.WriteString("  kernel_version, firecracker_version, env_id, envd_version, ready_cmd,\n")
	sql.WriteString("  cluster_node_id, reason, version,\n")
	sql.WriteString("  cpu_architecture, cpu_family, cpu_model, cpu_model_name, cpu_flags, team_id\n")
	sql.WriteString(") VALUES (\n")
	fmt.Fprintf(&sql, "  %s::uuid, now(), now(), now(), 'uploaded',\n", sqlString(request.BuildID))
	fmt.Fprintf(&sql, "  jsonb_build_object('from_image', %s, 'from_template', NULL, 'steps', '[]'::jsonb)::text,\n", sqlString(request.ImageRef))
	fmt.Fprintf(&sql, "  %s, %d, %d, %d, %d,\n",
		sqlString(request.Metadata.Resources.StartCommand), request.Metadata.Resources.VCPU,
		request.Metadata.Resources.MemoryMB, request.Metadata.Resources.DiskSizeMB, request.Metadata.RootfsSizeMB)
	fmt.Fprintf(&sql, "  %s, %s, %s, %s, %s,\n",
		sqlString(request.Metadata.KernelVersion), sqlString(request.Metadata.FirecrackerVersion),
		sqlString(request.TemplateID), sqlString(request.Metadata.EnvdVersion), sqlString(request.Metadata.Resources.ReadyCommand))
	fmt.Fprintf(&sql, "  :'cluster_node_id', jsonb_build_object('message', ''), %s,\n", sqlString(request.Metadata.TemplateVersion))
	fmt.Fprintf(&sql, "  %s, %s, %s, %s,\n",
		sqlString(request.Metadata.MachineInfo.CPUArchitecture), sqlString(request.Metadata.MachineInfo.CPUFamily),
		sqlString(request.Metadata.MachineInfo.CPUModel), sqlString(request.Metadata.MachineInfo.CPUModelName))
	fmt.Fprintf(&sql, "  ARRAY(SELECT jsonb_array_elements_text(%s::jsonb)), :'team_id'::uuid\n", sqlString(string(cpuFlagsJSON)))
	sql.WriteString(");\n\n")
	fmt.Fprintf(&sql, "INSERT INTO env_build_assignments (env_id, build_id, tag, source, created_at)\nVALUES (%s, %s::uuid, 'default', 'app', now());\n\n", sqlString(request.TemplateID), sqlString(request.BuildID))
	fmt.Fprintf(&sql, "INSERT INTO env_aliases (alias, env_id, namespace, is_renamable)\nVALUES (:'template_alias', %s, :'namespace', true);\n\n", sqlString(request.TemplateID))
	sql.WriteString("COMMIT;\n\n")
	fmt.Fprintf(&sql, "SELECT ea.alias, ea.namespace, ea.env_id, eb.id AS build_id, eb.status, eb.status_group\nFROM env_aliases AS ea\nJOIN env_build_assignments AS eba ON eba.env_id = ea.env_id AND eba.tag = 'default'\nJOIN env_builds AS eb ON eb.id = eba.build_id\nWHERE ea.alias = :'template_alias' AND ea.namespace = :'namespace'\n  AND ea.env_id = %s AND eb.id = %s::uuid;\n\n", sqlString(request.TemplateID), sqlString(request.BuildID))
	sql.WriteString("\\echo '下一步：精确清理 Redis cache，再通过 E2B API 创建 Sandbox 验证。'\n")

	return renderedRegistrationSQL{
		Result: RegistrationSQLResult{
			ObjectKey: objectKey,
			LocalPath: localPath,
			Warnings:  warnings,
		},
		Contents: sql.String(),
	}, nil
}

// writePSQLDefault 只在调用者没有通过 psql -v 提供变量时设置默认值。
func writePSQLDefault(builder *strings.Builder, name, value string) {
	fmt.Fprintf(builder, "\\if :{?%s}\n\\else\n\\set %s '%s'\n\\endif\n", name, name, escapePSQLMetaValue(value))
}

// escapePSQLMetaValue 转义 psql 反斜杠命令中的单引号、反斜杠和换行。
func escapePSQLMetaValue(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `'`, `\'`)
	value = strings.ReplaceAll(value, "\r", `\r`)
	return strings.ReplaceAll(value, "\n", `\n`)
}

// sqlString 生成 PostgreSQL 单引号字符串字面量；所有构建事实都通过这里写入 SQL。
func sqlString(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}
