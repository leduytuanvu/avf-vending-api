#!/usr/bin/env bash
# Enable offline_cash_sale_allowed for one machine via feature_flag_targets.
# Resolves DATABASE_URL from api container on production app-node when unset.
#
# Usage:
#   LOOKUP_MACHINE_ID=01a089ec-c7bb-7e0d-83a9-6f599f061f12 bash scripts/ops/enable_offline_cash_sale_machine.sh
set -Eeuo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
SQL_FILE="${ROOT}/scripts/ops/enable_offline_cash_sale_machine.sql"
DIAG_SQL_FILE="${ROOT}/scripts/ops/diagnose_machine_session.sql"
POSTGRES_TOOLS_IMAGE="${POSTGRES_TOOLS_IMAGE:-postgres:17-alpine}"

LOOKUP_MACHINE_ID="${LOOKUP_MACHINE_ID:-01a089ec-c7bb-7e0d-83a9-6f599f061f12}"
MACHINE_CODE="${MACHINE_CODE:-}"
DEVICE_SERIAL="${DEVICE_SERIAL:-0OVP8AYEFQ}"

fail() { echo "enable-offline-cash: error: $*" >&2; exit 1; }
note() { echo "enable-offline-cash: $*"; }

find_api_container() {
  docker ps --format '{{.Names}}' | grep -E 'api' | head -n1
}

container_env() {
  local container="$1"
  local key="$2"
  docker inspect "${container}" --format '{{range .Config.Env}}{{println .}}{{end}}' \
    | grep -E "^${key}=" | tail -n1 | cut -d= -f2- | tr -d '\r'
}

resolve_database_url() {
  if [[ -n "${DATABASE_URL:-}" ]]; then
    note "using DATABASE_URL from environment"
    return 0
  fi
  local api_container
  api_container="$(find_api_container)"
  [[ -n "${api_container}" ]] || return 1
  DATABASE_URL="$(container_env "${api_container}" DATABASE_URL)"
  [[ -n "${DATABASE_URL}" ]] || return 1
  note "api container=${api_container}"
}

psql_database_url() {
  python3 - "${DATABASE_URL}" <<'PY'
import sys
from urllib.parse import parse_qsl, urlencode, urlparse, urlunparse
u = urlparse(sys.argv[1])
drop = {"default_query_exec_mode", "pgbouncer"}
q = [(k, v) for k, v in parse_qsl(u.query, keep_blank_values=True) if k not in drop]
print(urlunparse((u.scheme, u.netloc, u.path, u.params, urlencode(q), u.fragment)))
PY
}

run_sql() {
  local sql_file="$1"
  [[ -f "${sql_file}" ]] || fail "missing ${sql_file}"
  command -v docker >/dev/null 2>&1 || fail "docker required for DB apply"
  resolve_database_url || fail "DATABASE_URL unavailable"

  local psql_url
  psql_url="$(psql_database_url)"
  docker run --rm \
    -e "DATABASE_URL=${psql_url}" \
    -v "${ROOT}/scripts/ops:/ops:ro" \
    "${POSTGRES_TOOLS_IMAGE}" \
    psql "${psql_url}" \
      -v ON_ERROR_STOP=1 \
      -v "lookup_machine_id=${LOOKUP_MACHINE_ID}" \
      -v "machine_code=${MACHINE_CODE}" \
      -v "device_serial=${DEVICE_SERIAL}" \
      -f "/ops/$(basename "${sql_file}")"
}

note "machine lookup id=${LOOKUP_MACHINE_ID} code=${MACHINE_CODE:-<empty>} serial=${DEVICE_SERIAL}"
if [[ -f "${DIAG_SQL_FILE}" ]]; then
  note "running diagnose_machine_session.sql (best-effort)"
  if ! run_sql "${DIAG_SQL_FILE}"; then
    note "diagnose failed; continuing with enable script"
  fi
fi
note "running enable_offline_cash_sale_machine.sql"
run_sql "${SQL_FILE}"
note "done — restart kiosk app or wait for bootstrap sync"
