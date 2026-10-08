#!/usr/bin/env bash
# Read-only prod verify for cash ledger session (bill_credit + payout).
set -Eeuo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
SQL_FILE="${ROOT}/scripts/ops/verify_cash_ledger_session.sql"
POSTGRES_TOOLS_IMAGE="${POSTGRES_TOOLS_IMAGE:-postgres:17-alpine}"
MACHINE_ID="${MACHINE_ID:-01a0a7e5-3c68-7895-b526-bcb6504bccfb}"
WINDOW_FROM="${WINDOW_FROM:-2026-10-08 05:30:00+00}"
WINDOW_TO="${WINDOW_TO:-2026-10-08 06:15:00+00}"
WITHDRAWAL_ID="${WITHDRAWAL_ID:-a1803384-dfb8-4d2d-89ec-7cef515eeb85}"

fail() { echo "verify-cash-ledger: error: $*" >&2; exit 1; }
note() { echo "verify-cash-ledger: $*"; }

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
      -v "machine_id_input=${MACHINE_ID}" \
      -v "window_from_input=${WINDOW_FROM}" \
      -v "window_to_input=${WINDOW_TO}" \
      -v "withdrawal_id_input=${WITHDRAWAL_ID}" \
      -f "/ops/$(basename "${SQL_FILE}")"
}

note "machine_id=${MACHINE_ID} from=${WINDOW_FROM} to=${WINDOW_TO} withdrawal=${WITHDRAWAL_ID}"
run_sql
note "done"
