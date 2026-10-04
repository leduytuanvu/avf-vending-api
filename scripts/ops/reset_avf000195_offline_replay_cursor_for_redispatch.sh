#!/usr/bin/env bash
# Reset AVF000195 offline cursor + rejected rows so kiosk can redispatch after API client_created_at fix.
set -Eeuo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
SQL_FILE="${ROOT}/scripts/ops/reset_avf000195_offline_replay_cursor_for_redispatch.sql"
POSTGRES_TOOLS_IMAGE="${POSTGRES_TOOLS_IMAGE:-postgres:17-alpine}"

fail() { echo "reset_avf000195_offline_replay: error: $*" >&2; exit 1; }
note() { echo "reset_avf000195_offline_replay: $*"; }

find_api_container() {
  docker ps --format '{{.Names}}' | grep -E 'api' | head -n1
}

resolve_database_url() {
  if [[ -n "${DATABASE_URL:-}" ]]; then
    return 0
  fi
  local api_container
  api_container="$(find_api_container)"
  [[ -n "${api_container}" ]] || return 1
  DATABASE_URL="$(docker inspect "${api_container}" --format '{{range .Config.Env}}{{println .}}{{end}}' \
    | grep -E '^DATABASE_URL=' | tail -n1 | cut -d= -f2- | tr -d '\r')"
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

[[ -f "${SQL_FILE}" ]] || fail "missing ${SQL_FILE}"
command -v docker >/dev/null 2>&1 || fail "docker required"
resolve_database_url || fail "DATABASE_URL unavailable"
psql_url="$(psql_database_url)"
note "applying ${SQL_FILE}"
docker run --rm \
  -e "DATABASE_URL=${psql_url}" \
  -v "${ROOT}/scripts/ops:/ops:ro" \
  "${POSTGRES_TOOLS_IMAGE}" \
  psql "${psql_url}" -v ON_ERROR_STOP=1 -f "/ops/$(basename "${SQL_FILE}")"
note "done"
