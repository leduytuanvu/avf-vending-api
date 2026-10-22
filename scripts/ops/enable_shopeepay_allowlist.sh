#!/usr/bin/env bash
# Enable ShopeePay QR on machines whose external code is in the SPP allowlist.
# Never enables the entire fleet. Requires DATABASE_URL (or api container on app-node).
#
# Usage:
#   DRY_RUN=1 bash scripts/ops/enable_shopeepay_allowlist.sh
#   bash scripts/ops/enable_shopeepay_allowlist.sh
#   ALLOWLIST_FILE=/path/to/store_ext_id_allowlist.txt bash scripts/ops/enable_shopeepay_allowlist.sh
set -Eeuo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
ALLOWLIST_FILE="${ALLOWLIST_FILE:-${ROOT}/deployments/prod/app-node/shopeepay/store_ext_id_allowlist.txt}"
SQL_FILE="${ROOT}/scripts/ops/enable_shopeepay_allowlist.sql"
POSTGRES_TOOLS_IMAGE="${POSTGRES_TOOLS_IMAGE:-postgres:17-alpine}"
DRY_RUN="${DRY_RUN:-0}"

fail() { echo "enable-shopeepay-allowlist: error: $*" >&2; exit 1; }
note() { echo "enable-shopeepay-allowlist: $*"; }

[[ -f "${ALLOWLIST_FILE}" ]] || fail "allowlist file not found: ${ALLOWLIST_FILE}"
[[ -f "${SQL_FILE}" ]] || fail "missing ${SQL_FILE}"

ALLOWLIST_CSV="$(
  awk '
    { sub(/\r$/, "") }
    /^[[:space:]]*#/ { next }
    /^[[:space:]]*$/ { next }
    { gsub(/[[:space:]]/, ""); if ($0 != "") print }
  ' "${ALLOWLIST_FILE}" | paste -sd, -
)"
[[ -n "${ALLOWLIST_CSV}" ]] || fail "allowlist is empty"

COUNT="$(awk -F, '{print NF}' <<<"${ALLOWLIST_CSV}")"
note "allowlist_file=${ALLOWLIST_FILE} codes=${COUNT}"

if [[ "${DRY_RUN}" == "1" ]]; then
  note "DRY_RUN=1; not writing machine_payment_methods"
  echo "${ALLOWLIST_CSV}" | tr ',' '\n' | head -n 20
  echo "..."
  exit 0
fi

command -v docker >/dev/null 2>&1 || fail "docker required"

find_api_container() {
  docker ps --format '{{.Names}}' | grep -E 'api' | head -n1
}

container_env() {
  local container="$1"
  local key="$2"
  docker inspect "${container}" --format '{{range .Config.Env}}{{println .}}{{end}}' \
    | grep -E "^${key}=" | tail -n1 | cut -d= -f2- | tr -d '\r'
}

if [[ -z "${DATABASE_URL:-}" ]]; then
  api_container="$(find_api_container)"
  [[ -n "${api_container}" ]] || fail "DATABASE_URL unset and no api container found"
  DATABASE_URL="$(container_env "${api_container}" DATABASE_URL)"
  [[ -n "${DATABASE_URL}" ]] || fail "api container has no DATABASE_URL"
  note "api container=${api_container}"
fi

psql_url="$(python3 - "${DATABASE_URL}" <<'PY'
import sys
from urllib.parse import parse_qsl, urlencode, urlparse, urlunparse
u = urlparse(sys.argv[1])
drop = {"default_query_exec_mode", "pgbouncer"}
q = [(k, v) for k, v in parse_qsl(u.query, keep_blank_values=True) if k not in drop]
print(urlunparse((u.scheme, u.netloc, u.path, u.params, urlencode(q), u.fragment)))
PY
)"

docker run --rm \
  -e "DATABASE_URL=${psql_url}" \
  -v "${ROOT}/scripts/ops:/ops:ro" \
  "${POSTGRES_TOOLS_IMAGE}" \
  psql "${psql_url}" \
    -v ON_ERROR_STOP=1 \
    -v "allowlist_csv=${ALLOWLIST_CSV}" \
    -f /ops/enable_shopeepay_allowlist.sql

note "done. Next: kiosk APK with ShopeePay un-hidden + bootstrap refresh."
