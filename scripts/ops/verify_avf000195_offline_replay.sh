#!/usr/bin/env bash
# Read-only prod verify: AVF000195 offline test orders + offline cursor.
set -Eeuo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
SQL_FILE="${ROOT}/scripts/ops/verify_avf000195_offline_replay.sql"
POSTGRES_TOOLS_IMAGE="${POSTGRES_TOOLS_IMAGE:-postgres:17-alpine}"
MIN_CURSOR="${MIN_CURSOR:-36}"
REQUIRE_ORDERS="${REQUIRE_ORDERS:-1}"
ORDER_A="ffc489f5-0bcd-45f3-971a-ce709f7c2056"
ORDER_B="0a6984ee-7139-4d18-b943-9a843da0e6d3"
ORDER_C="caea5197-7295-47f6-b9d8-7d1af2fc1207"
ORDER_D="54a327c2-3ecd-4014-b90c-121fed479cd6"
PRIMARY_ORDER="${PRIMARY_ORDER:-${ORDER_C}}"

fail() { echo "verify-avf000195-replay: error: $*" >&2; exit 1; }
note() { echo "verify-avf000195-replay: $*"; }

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
    psql "${psql_url}" -v ON_ERROR_STOP=1 -f "/ops/$(basename "${SQL_FILE}")"
}

assert_orders() {
  resolve_database_url || fail "DATABASE_URL unavailable"
  local psql_url count
  psql_url="$(psql_database_url)"
  local primary_count total_count
  primary_count="$(docker run --rm \
    -e "DATABASE_URL=${psql_url}" \
    "${POSTGRES_TOOLS_IMAGE}" \
    psql "${psql_url}" -t -A -v ON_ERROR_STOP=1 \
      -c "SELECT count(*) FROM orders WHERE id = '${PRIMARY_ORDER}'::uuid;")"
  primary_count="$(echo "${primary_count}" | tr -d '\r\n ')"
  [[ "${primary_count}" == "1" ]] || fail "expected primary order ${PRIMARY_ORDER} in DB, got count=${primary_count:-0} (kiosk may still be draining outbox)"
  total_count="$(docker run --rm \
    -e "DATABASE_URL=${psql_url}" \
    "${POSTGRES_TOOLS_IMAGE}" \
    psql "${psql_url}" -t -A -v ON_ERROR_STOP=1 \
      -c "SELECT count(*) FROM orders WHERE id IN ('${ORDER_A}'::uuid, '${ORDER_B}'::uuid, '${ORDER_C}'::uuid, '${ORDER_D}'::uuid);")"
  total_count="$(echo "${total_count}" | tr -d '\r\n ')"
  note "orders in DB (test set)=${total_count}"
}

assert_cursor() {
  resolve_database_url || fail "DATABASE_URL unavailable"
  local psql_url last_seq
  psql_url="$(psql_database_url)"
  last_seq="$(docker run --rm \
    -e "DATABASE_URL=${psql_url}" \
    "${POSTGRES_TOOLS_IMAGE}" \
    psql "${psql_url}" -t -A -v ON_ERROR_STOP=1 \
      -c "SELECT COALESCE((SELECT last_sequence FROM machine_sync_cursors WHERE machine_id = '01a0a7e5-3c68-7895-b526-bcb6504bccfb'::uuid AND stream_name = 'offline'), 0);")"
  last_seq="$(echo "${last_seq}" | tr -d '\r\n ')"
  if [[ -z "${last_seq}" ]] || [[ "${last_seq}" -lt "${MIN_CURSOR}" ]]; then
    fail "offline.last_sequence=${last_seq:-missing} expected>=${MIN_CURSOR}"
  fi
  note "offline.last_sequence=${last_seq}"
}

note "min_cursor=${MIN_CURSOR} require_orders=${REQUIRE_ORDERS} primary=${PRIMARY_ORDER} orders=${ORDER_A} ${ORDER_B} ${ORDER_C} ${ORDER_D}"
run_sql
assert_cursor
if [[ "${REQUIRE_ORDERS}" == "1" ]]; then
  assert_orders
  note "verified orders + cursor OK"
else
  note "cursor OK (order assert skipped)"
fi
