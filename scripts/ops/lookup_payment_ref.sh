#!/usr/bin/env bash
# Read-only prod lookup: MoMo orderId / transId -> platform order.
set -Eeuo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
SQL_FILE="${ROOT}/scripts/ops/lookup_payment_ref.sql"
POSTGRES_TOOLS_IMAGE="${POSTGRES_TOOLS_IMAGE:-postgres:17-alpine}"
MOMO_ORDER_ID="${MOMO_ORDER_ID:-}"
MOMO_TRANS_ID="${MOMO_TRANS_ID:-}"
MACHINE_CODE="${MACHINE_CODE:-}"

fail() { echo "lookup-payment-ref: error: $*" >&2; exit 1; }
note() { echo "lookup-payment-ref: $*"; }

[[ -n "${MOMO_ORDER_ID}" ]] || fail "MOMO_ORDER_ID is required"

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
  command -v docker >/dev/null 2>&1 || fail "docker required"
  resolve_database_url || fail "DATABASE_URL unavailable"

  local psql_url
  psql_url="$(psql_database_url)"
  docker run --rm \
    -e "DATABASE_URL=${psql_url}" \
    -v "${ROOT}/scripts/ops:/ops:ro" \
    "${POSTGRES_TOOLS_IMAGE}" \
    psql "${psql_url}" \
      -v ON_ERROR_STOP=1 \
      -v "momo_order_id=${MOMO_ORDER_ID}" \
      -v "momo_trans_id=${MOMO_TRANS_ID}" \
      -v "machine_code=${MACHINE_CODE}" \
      -f "/ops/$(basename "${SQL_FILE}")"
}

note "momo_order_id=${MOMO_ORDER_ID} trans_id=${MOMO_TRANS_ID:-<empty>} machine=${MACHINE_CODE:-<any>}"
run_sql
note "done"
