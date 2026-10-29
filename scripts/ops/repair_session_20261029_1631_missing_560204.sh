#!/usr/bin/env bash
# Repair AVF000195 2026-10-29 16:31 missing recycler 10k 560204 (boot-scoped insert).
# Default dry-run.
#
# Usage:
#   DRY_RUN=1 bash scripts/ops/repair_session_20261029_1631_missing_560204.sh
#   DRY_RUN=0 bash scripts/ops/repair_session_20261029_1631_missing_560204.sh
set -Eeuo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
SQL_FILE="${ROOT}/scripts/ops/repair_session_20261029_1631_missing_560204.sql"
POSTGRES_TOOLS_IMAGE="${POSTGRES_TOOLS_IMAGE:-postgres:17-alpine}"
DRY_RUN="${DRY_RUN:-1}"

fail() { echo "repair-20261029-1631: error: $*" >&2; exit 1; }
note() { echo "repair-20261029-1631: $*"; }

API_CONTAINER=""

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
  API_CONTAINER="$(find_api_container || true)"
  if [[ -n "${DATABASE_URL:-}" ]]; then
    note "using DATABASE_URL from environment"
    return 0
  fi
  [[ -n "${API_CONTAINER}" ]] || return 1
  DATABASE_URL="$(container_env "${API_CONTAINER}" DATABASE_URL)"
  [[ -n "${DATABASE_URL}" ]] || return 1
  note "api container=${API_CONTAINER}"
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
  local docker_network_args=()
  if [[ -n "${API_CONTAINER}" ]]; then
    docker_network_args=(--network "container:${API_CONTAINER}")
    note "psql network=container:${API_CONTAINER}"
  fi
  docker run --rm \
    "${docker_network_args[@]}" \
    -e "DATABASE_URL=${psql_url}" \
    -v "${ROOT}/scripts/ops:/ops:ro" \
    "${POSTGRES_TOOLS_IMAGE}" \
    psql "${psql_url}" \
      -v ON_ERROR_STOP=1 \
      -v "dry_run=${DRY_RUN}" \
      -f "/ops/$(basename "${SQL_FILE}")"
}

note "dry_run=${DRY_RUN}"
run_sql
note "done"
