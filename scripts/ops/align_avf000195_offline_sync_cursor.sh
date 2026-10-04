#!/usr/bin/env bash
# Align offline sync cursor for AVF000195 (sequence hole: target = minPending - 1 from log).
#
# Usage:
#   TARGET_LAST_SEQUENCE=2 bash scripts/ops/align_avf000195_offline_sync_cursor.sh
#   DRY_RUN=1 TARGET_LAST_SEQUENCE=2 bash scripts/ops/align_avf000195_offline_sync_cursor.sh
set -Eeuo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
SQL_FILE="${ROOT}/scripts/ops/align_avf000195_offline_sync_cursor.sql"
POSTGRES_TOOLS_IMAGE="${POSTGRES_TOOLS_IMAGE:-postgres:17-alpine}"
DRY_RUN="${DRY_RUN:-0}"
TARGET_LAST_SEQUENCE="${TARGET_LAST_SEQUENCE:-2}"

fail() { echo "align-avf000195-cursor: error: $*" >&2; exit 1; }
note() { echo "align-avf000195-cursor: $*"; }

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
      -v "target_last_sequence=${TARGET_LAST_SEQUENCE}" \
      -f "/ops/$(basename "${SQL_FILE}")"
}

verify_cursor() {
  [[ "${DRY_RUN}" == "1" ]] && return 0
  resolve_database_url || fail "DATABASE_URL unavailable for verify"
  local psql_url last_seq
  psql_url="$(psql_database_url)"
  last_seq="$(docker run --rm \
    -e "DATABASE_URL=${psql_url}" \
    "${POSTGRES_TOOLS_IMAGE}" \
    psql "${psql_url}" -t -A -v ON_ERROR_STOP=1 \
      -c "SELECT COALESCE((SELECT last_sequence FROM machine_sync_cursors WHERE machine_id = '01a0a7e5-3c68-7895-b526-bcb6504bccfb'::uuid AND stream_name = 'offline'), 0);")"
  last_seq="$(echo "${last_seq}" | tr -d '\r\n ')"
  if [[ -z "${last_seq}" ]] || [[ "${last_seq}" -lt "${TARGET_LAST_SEQUENCE}" ]]; then
    fail "cursor verify failed: offline.last_sequence=${last_seq:-missing} expected>=${TARGET_LAST_SEQUENCE}"
  fi
  note "verified offline.last_sequence=${last_seq}"
}

note "dry_run=${DRY_RUN} target_last_sequence=${TARGET_LAST_SEQUENCE}"
run_sql
verify_cursor
note "done"
