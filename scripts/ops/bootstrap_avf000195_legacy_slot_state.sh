#!/usr/bin/env bash
# Bootstrap legacy machine_slot_state for AVF000195 from current machine_slot_configs.
#
# Usage:
#   bash scripts/ops/bootstrap_avf000195_legacy_slot_state.sh            # apply
#   DRY_RUN=1 bash scripts/ops/bootstrap_avf000195_legacy_slot_state.sh  # preview only
set -Eeuo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
SQL_FILE="${ROOT}/scripts/ops/bootstrap_avf000195_legacy_slot_state.sql"
POSTGRES_TOOLS_IMAGE="${POSTGRES_TOOLS_IMAGE:-postgres:17-alpine}"
DRY_RUN="${DRY_RUN:-0}"

fail() { echo "bootstrap-avf000195-legacy: error: $*" >&2; exit 1; }
note() { echo "bootstrap-avf000195-legacy: $*"; }

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
  [[ -f "${SQL_FILE}" ]] || fail "missing ${SQL_FILE}"
  command -v docker >/dev/null 2>&1 || fail "docker required for DB apply"
  resolve_database_url || fail "DATABASE_URL unavailable"

  local psql_url dry_run_flag
  psql_url="$(psql_database_url)"
  dry_run_flag="${DRY_RUN}"
  docker run --rm \
    -e "DATABASE_URL=${psql_url}" \
    -v "${ROOT}/scripts/ops:/ops:ro" \
    "${POSTGRES_TOOLS_IMAGE}" \
    psql "${psql_url}" \
      -v ON_ERROR_STOP=1 \
      -v "dry_run=${dry_run_flag}" \
      -f "/ops/$(basename "${SQL_FILE}")"
}

note "dry_run=${DRY_RUN}"
run_sql
note "done"
