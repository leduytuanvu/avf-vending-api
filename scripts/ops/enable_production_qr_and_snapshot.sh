#!/usr/bin/env bash
# Enable MoMo/ZaloPay/VietQR on a machine and periodic 5m layout snapshot capture.
# Prefers admin API when ADMIN_TOKEN or ADMIN_EMAIL+ADMIN_PASSWORD are set.
# Falls back to direct Postgres on production app-node (DATABASE_URL from api container).
#
# Usage:
#   MACHINE_CODE=AVF000195 bash scripts/ops/enable_production_qr_and_snapshot.sh
#   FORCE_DB=1 MACHINE_CODE=AVF000195 bash scripts/ops/enable_production_qr_and_snapshot.sh
set -Eeuo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
# shellcheck source=../e2e/lib/common.sh
source "${ROOT}/scripts/e2e/lib/common.sh"

BASE_URL="${BASE_URL:-https://api.ldtv.dev}"
BASE_URL="${BASE_URL%/}"
export BASE_URL

MACHINE_CODE="${MACHINE_CODE:-AVF000195}"
PERIODIC_FLAG_KEY="${PERIODIC_FLAG_KEY:-periodic_snapshot_capture_enabled}"
SQL_FILE="${ROOT}/scripts/ops/enable_production_qr_and_snapshot.sql"
POSTGRES_TOOLS_IMAGE="${POSTGRES_TOOLS_IMAGE:-postgres:17-alpine}"

: "${ADMIN_EMAIL:=${E2E_PROD_ADMIN_EMAIL:-}}"
: "${ADMIN_PASSWORD:=${E2E_PROD_ADMIN_PASSWORD:-}}"
: "${ADMIN_TOKEN:=${E2E_PROD_ADMIN_TOKEN:-}}"

e2e_require_cmd curl jq
e2e_init_run_dir "production-enable-qr-snapshot"

fail() { echo "enable-qr-snapshot: error: $*" >&2; exit 1; }
note() { echo "enable-qr-snapshot: $*"; }

print_version_probe() {
  local version_body="${E2E_RUN_DIR}/raw/version.json"
  curl -sS -o "${version_body}" "${BASE_URL}/version" || true
  local payment_mode qr_available
  payment_mode="$(jq -r '.payment_runtime.payment_mode // "unknown"' "${version_body}")"
  qr_available="$(jq -r '.payment_runtime.card_qr_sessions_available // false' "${version_body}")"
  note "public /version payment_mode=${payment_mode} card_qr_sessions_available=${qr_available}"
}

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

enable_via_db() {
  [[ -f "${SQL_FILE}" ]] || fail "missing ${SQL_FILE}"
  command -v docker >/dev/null 2>&1 || fail "docker required for DB fallback"
  resolve_database_url || fail "DATABASE_URL unavailable (set DATABASE_URL or run on app-node with api container)"

  local psql_url
  psql_url="$(psql_database_url)"
  note "applying machine config via Postgres machine_code=${MACHINE_CODE}"

  docker run --rm \
    -e "DATABASE_URL=${psql_url}" \
    -v "${ROOT}/scripts/ops:/ops:ro" \
    "${POSTGRES_TOOLS_IMAGE}" \
    psql "${psql_url}" \
      -v ON_ERROR_STOP=1 \
      -v "machine_code=${MACHINE_CODE}" \
      -v "periodic_flag_key=${PERIODIC_FLAG_KEY}" \
      -f /ops/enable_production_qr_and_snapshot.sql \
    | tee "${E2E_RUN_DIR}/logs/db-apply.log"

  print_version_probe
  note "done (db) machine=${MACHINE_CODE} periodic_flag=${PERIODIC_FLAG_KEY}"
  echo "NEXT: device bootstrap refresh (reboot app or wait for sync) to apply flags + payment methods on kiosk."
}

enable_via_admin_api() {
  local admin_tok
  admin_tok="$(e2e_admin_token)" || return 1

  local auth_hdr=(-H "Authorization: Bearer ${admin_tok}" -H "Accept: application/json")

  note "resolving machine code=${MACHINE_CODE}"
  local machines_body="${E2E_RUN_DIR}/raw/machines.json"
  local code
  code="$(curl -sS -o "${machines_body}" -w '%{http_code}' \
    "${auth_hdr[@]}" \
    "${BASE_URL}/v1/admin/machines?limit=20&q=$(python3 -c "import urllib.parse; print(urllib.parse.quote('${MACHINE_CODE}'))")")"
  [[ "${code}" == "200" ]] || fail "list machines HTTP ${code}"
  local machine_id
  machine_id="$(jq -r --arg c "${MACHINE_CODE}" '
    (.items // .machines // .data // [])
    | map(select((.machineCode // .machine_code // .code // "") == $c))
    | .[0].id // empty
  ' "${machines_body}")"
  [[ -n "${machine_id}" ]] || fail "machine not found for code ${MACHINE_CODE}"

  note "machine_id=${machine_id}"

  note "loading feature flags"
  local flags_body="${E2E_RUN_DIR}/raw/feature-flags.json"
  code="$(curl -sS -o "${flags_body}" -w '%{http_code}' \
    "${auth_hdr[@]}" \
    "${BASE_URL}/v1/admin/feature-flags?limit=100")"
  [[ "${code}" == "200" ]] || fail "list feature flags HTTP ${code}"
  local flag_id
  flag_id="$(jq -r --arg k "${PERIODIC_FLAG_KEY}" '
    (.items // .flags // [])
    | map(select(.flagKey == $k))
    | .[0].id // empty
  ' "${flags_body}")"
  [[ -n "${flag_id}" ]] || fail "feature flag not found: ${PERIODIC_FLAG_KEY}"

  note "setting machine target for ${PERIODIC_FLAG_KEY} flag_id=${flag_id}"
  local targets_payload
  targets_payload="$(jq -nc --arg mid "${machine_id}" '{
    targets: [{
      targetType: "machine",
      machineId: $mid,
      enabled: true,
      priority: 100
    }]
  }')"
  code="$(curl -sS -o "${E2E_RUN_DIR}/raw/flag-targets.json" -w '%{http_code}' \
    -X PUT \
    -H "Content-Type: application/json" \
    "${auth_hdr[@]}" \
    -d "${targets_payload}" \
    "${BASE_URL}/v1/admin/feature-flags/${flag_id}/targets")"
  [[ "${code}" == "200" ]] || fail "put feature flag targets HTTP ${code}"

  note "enabling payment methods cash+momo+zalopay+vietqr"
  local pay_payload
  pay_payload="$(jq -nc '{
    methods: [
      {methodKey: "cash", enabled: true, sortOrder: 0},
      {methodKey: "momo", enabled: true, sortOrder: 1},
      {methodKey: "zalopay", enabled: true, sortOrder: 2},
      {methodKey: "vietqr", enabled: true, sortOrder: 3}
    ]
  }')"
  code="$(curl -sS -o "${E2E_RUN_DIR}/raw/payment-methods.json" -w '%{http_code}' \
    -X PUT \
    -H "Content-Type: application/json" \
    "${auth_hdr[@]}" \
    -d "${pay_payload}" \
    "${BASE_URL}/v1/admin/machines/${machine_id}/payment-methods")"
  [[ "${code}" == "200" ]] || {
    jq . "${E2E_RUN_DIR}/raw/payment-methods.json" >&2 || true
    fail "put payment methods HTTP ${code}"
  }

  local deployment_supported
  deployment_supported="$(jq -r '.deploymentSupported // false' "${E2E_RUN_DIR}/raw/payment-methods.json")"
  note "payment-methods deploymentSupported=${deployment_supported}"

  print_version_probe
  note "done (admin api) machine=${MACHINE_CODE} id=${machine_id} periodic_flag=${PERIODIC_FLAG_KEY}"
  echo "NEXT: device bootstrap refresh (reboot app or wait for sync) to apply flags + payment methods on kiosk."
}

if [[ "${FORCE_DB:-0}" == "1" ]]; then
  enable_via_db
  exit 0
fi

if enable_via_admin_api; then
  exit 0
fi

note "admin auth unavailable; falling back to Postgres on app-node"
enable_via_db
