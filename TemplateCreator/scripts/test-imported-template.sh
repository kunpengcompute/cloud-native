#!/usr/bin/env bash
# 通过 E2B API 创建、查询并删除一个 Sandbox，验证离线导入的 Template 可启动。
set -Eeuo pipefail

# ============================ 用户配置区 ============================
# 可以直接修改等号右侧；真实 API key 建议通过 export E2B_API_KEY=... 临时注入。
E2B_API_URL="${E2B_API_URL:-REPLACE_WITH_E2B_API_URL}"
E2B_API_KEY="${E2B_API_KEY:-}"
TEMPLATE_ALIAS="${TEMPLATE_ALIAS:-REPLACE_WITH_TEMPLATE_ALIAS}"
SANDBOX_TIMEOUT="${SANDBOX_TIMEOUT:-300}"
KEEP_SANDBOX="${KEEP_SANDBOX:-false}"

# 查看本地 E2B 的模板、Build、Sandbox 状态，目前可使用 API：

#   curl http://localhost:3004/templates -H "X-API-Key: $E2B_API_KEY"

#   curl http://localhost:3004/sandboxes -H "X-API-Key: $E2B_API_KEY"

# ========================== 用户配置区结束 ==========================

usage() {
  cat <<'EOF'
TemplateCreator 导入后 Sandbox 启动测试脚本

用法：
  1. 直接修改脚本开头“用户配置区”的变量；
  2. 建议执行 export E2B_API_KEY=<部署环境的 API key>；
  3. 执行：bash scripts/test-imported-template.sh

脚本只验证核心启动链路：
  [1/3] POST /sandboxes 创建
  [2/3] GET /sandboxes/{id} 确认为 running
  [3/3] DELETE /sandboxes/{id} 清理

设置 KEEP_SANDBOX=true 可跳过删除，便于继续人工检查。
EOF
}

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  usage
  exit 0
fi
if (($# != 0)); then
  printf '不支持位置参数；请修改脚本顶部变量或使用同名环境变量。\n' >&2
  exit 2
fi

die() {
  printf '错误：%s\n' "$*" >&2
  exit 1
}

for command_name in curl python3 mktemp; do
  command -v "$command_name" >/dev/null 2>&1 || die "缺少命令：$command_name"
done
[[ -n "$E2B_API_URL" && "$E2B_API_URL" != REPLACE_WITH_* ]] || die '请先设置 E2B_API_URL'
[[ -n "$E2B_API_KEY" ]] || die '请先通过环境变量或用户配置区设置 E2B_API_KEY'
[[ -n "$TEMPLATE_ALIAS" && "$TEMPLATE_ALIAS" != REPLACE_WITH_* ]] || die '请先设置 TEMPLATE_ALIAS'
[[ "$SANDBOX_TIMEOUT" =~ ^[1-9][0-9]*$ ]] || die 'SANDBOX_TIMEOUT 必须是正整数秒数'
[[ "$KEEP_SANDBOX" == true || "$KEEP_SANDBOX" == false ]] || die 'KEEP_SANDBOX 只能是 true 或 false'
E2B_API_URL=${E2B_API_URL%/}

CREATE_RESPONSE=$(mktemp -t templatecreator-sandbox-create-XXXXXX.json)
STATUS_RESPONSE=$(mktemp -t templatecreator-sandbox-status-XXXXXX.json)
SANDBOX_ID=""

cleanup() {
  local status=$?
  if ((status != 0)) && [[ -n "$SANDBOX_ID" && "$KEEP_SANDBOX" == false ]]; then
    curl -sS -o /dev/null -X DELETE "$E2B_API_URL/sandboxes/$SANDBOX_ID" \
      -H "X-API-Key: $E2B_API_KEY" >/dev/null 2>&1 || true
  fi
  rm -f -- "$CREATE_RESPONSE" "$STATUS_RESPONSE"
}
trap cleanup EXIT

printf '[1/3] 创建 Sandbox\n'
create_code=$(curl -sS -o "$CREATE_RESPONSE" -w '%{http_code}' \
  -X POST "$E2B_API_URL/sandboxes" \
  -H "X-API-Key: $E2B_API_KEY" \
  -H 'Content-Type: application/json' \
  -d "{\"templateID\":\"$TEMPLATE_ALIAS\",\"timeout\":$SANDBOX_TIMEOUT}")
if [[ "$create_code" != 201 ]]; then
  cat "$CREATE_RESPONSE" >&2
  die "创建 Sandbox 返回 HTTP $create_code，预期 201"
fi
SANDBOX_ID=$(python3 - "$CREATE_RESPONSE" <<'PY'
import json
import pathlib
import sys

data = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
value = data.get("sandboxID")
if not isinstance(value, str) or not value:
    raise SystemExit("创建响应缺少 sandboxID")
print(value)
PY
)
printf 'sandbox_id=%s\n' "$SANDBOX_ID"

printf '[2/3] 等待并确认 Sandbox 状态为 running\n'
state=""
status_code=""
for ((attempt = 1; attempt <= 20; attempt++)); do
  status_code=$(curl -sS -o "$STATUS_RESPONSE" -w '%{http_code}' \
    "$E2B_API_URL/sandboxes/$SANDBOX_ID" \
    -H "X-API-Key: $E2B_API_KEY" || true)
  if [[ "$status_code" == 200 ]]; then
    state=$(python3 - "$STATUS_RESPONSE" <<'PY'
import json
import pathlib
import sys

data = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
print(data.get("state", ""))
PY
)
    [[ "$state" == running ]] && break
  fi
  sleep 0.5
done
if [[ "$status_code" != 200 || "$state" != running ]]; then
  cat "$STATUS_RESPONSE" >&2 || true
  die "Sandbox 未进入 running：HTTP $status_code state=$state"
fi
printf 'sandbox_state=%s\n' "$state"

printf '[3/3] 清理 Sandbox\n'
if [[ "$KEEP_SANDBOX" == true ]]; then
  printf 'KEEP_SANDBOX=true，已保留 Sandbox：%s\n' "$SANDBOX_ID"
else
  delete_code=$(curl -sS -o /dev/null -w '%{http_code}' \
    -X DELETE "$E2B_API_URL/sandboxes/$SANDBOX_ID" \
    -H "X-API-Key: $E2B_API_KEY")
  [[ "$delete_code" == 204 ]] || die "删除 Sandbox 返回 HTTP $delete_code，预期 204"
  printf 'sandbox_deleted=%s HTTP=204\n' "$SANDBOX_ID"
fi

printf 'Sandbox 启动测试成功：template=%s sandbox=%s\n' "$TEMPLATE_ALIAS" "$SANDBOX_ID"
