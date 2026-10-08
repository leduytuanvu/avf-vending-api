#!/usr/bin/env bash
# Clear poisoned machine_idempotency_keys for bill_credit cash_movement keys without acceptance rows.
#
# Usage:
#   DRY_RUN=1 bash scripts/ops/repair_cash_movement_idempotency_poison.sh
#   DRY_RUN=0 bash scripts/ops/repair_cash_movement_idempotency_poison.sh
set -Eeuo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
SQL_FILE="${ROOT}/scripts/ops/repair_cash_movement_idempotency_poison.sql"
POSTGRES_TOOLS_IMAGE="${POSTGRES_TOOLS_IMAGE:-postgres:17-alpine}"
DRY_RUN="${DRY_RUN:-0}"
MACHINE_ID="${MACHINE_ID:-01a0a7e5-3c68-7895-b526-bcb6504bccfb}"

fail() { echo "repair-cash-idempotency: error: $*" >&2; exit 1; }
note() { echo "repair-cash-idempotency: $*"; }

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

  local psql_url
  psql_url="$(psql_database_url)"
  docker run --rm \
    -e "DATABASE_URL=${psql_url}" \
    -v "${ROOT}/scripts/ops:/ops:ro" \
    "${POSTGRES_TOOLS_IMAGE}" \
    psql "${psql_url}" \
      -v ON_ERROR_STOP=1 \
      -v "dry_run=${DRY_RUN}" \
      -v "machine_id=${MACHINE_ID}" \
      -f "/ops/$(basename "${SQL_FILE}")"
}

note "dry_run=${DRY_RUN} machine_id=${MACHINE_ID}"
run_sql
note "done"
