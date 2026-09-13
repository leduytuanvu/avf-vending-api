#!/usr/bin/env bash
# Enable MoMo/ZaloPay/VietQR on a machine and periodic 5m layout snapshot capture via admin API.
# Requires ADMIN_TOKEN or ADMIN_EMAIL+ADMIN_PASSWORD (or E2E_PROD_* aliases).
#
# Usage:
#   MACHINE_CODE=AVF000195 bash scripts/ops/enable_production_qr_and_snapshot.sh
set -Eeuo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
# shellcheck source=../e2e/lib/common.sh
source "${ROOT}/scripts/e2e/lib/common.sh"

BASE_URL="${BASE_URL:-https://api.ldtv.dev}"
BASE_URL="${BASE_URL%/}"
export BASE_URL

MACHINE_CODE="${MACHINE_CODE:-AVF000195}"
PERIODIC_FLAG_KEY="${PERIODIC_FLAG_KEY:-periodic_snapshot_capture_enabled}"

: "${ADMIN_EMAIL:=${E2E_PROD_ADMIN_EMAIL:-}}"
: "${ADMIN_PASSWORD:=${E2E_PROD_ADMIN_PASSWORD:-}}"
: "${ADMIN_TOKEN:=${E2E_PROD_ADMIN_TOKEN:-}}"

e2e_require_cmd curl jq
e2e_init_run_dir "production-enable-qr-snapshot"

fail() { echo "enable-qr-snapshot: error: $*" >&2; exit 1; }
note() { echo "enable-qr-snapshot: $*"; }

ADMIN_TOK=""
ADMIN_TOK="$(e2e_admin_token)" || fail "admin auth failed (set ADMIN_TOKEN or ADMIN_EMAIL+ADMIN_PASSWORD)"

auth_hdr=(-H "Authorization: Bearer ${ADMIN_TOK}" -H "Accept: application/json")

note "resolving machine code=${MACHINE_CODE}"
machines_body="${E2E_RUN_DIR}/raw/machines.json"
code="$(curl -sS -o "${machines_body}" -w '%{http_code}' \
  "${auth_hdr[@]}" \
  "${BASE_URL}/v1/admin/machines?limit=20&q=$(python3 -c "import urllib.parse; print(urllib.parse.quote('${MACHINE_CODE}'))")")"
[[ "${code}" == "200" ]] || fail "list machines HTTP ${code}"
machine_id="$(jq -r --arg c "${MACHINE_CODE}" '
  (.items // .machines // .data // [])
  | map(select((.machineCode // .machine_code // .code // "") == $c))
  | .[0].id // empty
' "${machines_body}")"
[[ -n "${machine_id}" ]] || fail "machine not found for code ${MACHINE_CODE}"

note "machine_id=${machine_id}"

note "loading feature flags"
flags_body="${E2E_RUN_DIR}/raw/feature-flags.json"
code="$(curl -sS -o "${flags_body}" -w '%{http_code}' \
  "${auth_hdr[@]}" \
  "${BASE_URL}/v1/admin/feature-flags?limit=100")"
[[ "${code}" == "200" ]] || fail "list feature flags HTTP ${code}"
flag_id="$(jq -r --arg k "${PERIODIC_FLAG_KEY}" '
  (.items // .flags // [])
  | map(select(.flagKey == $k))
  | .[0].id // empty
' "${flags_body}")"
[[ -n "${flag_id}" ]] || fail "feature flag not found: ${PERIODIC_FLAG_KEY}"

note "setting machine target for ${PERIODIC_FLAG_KEY} flag_id=${flag_id}"
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

deployment_supported="$(jq -r '.deploymentSupported // false' "${E2E_RUN_DIR}/raw/payment-methods.json")"
note "payment-methods deploymentSupported=${deployment_supported}"

version_body="${E2E_RUN_DIR}/raw/version.json"
curl -sS -o "${version_body}" "${BASE_URL}/version" || true
payment_mode="$(jq -r '.payment_runtime.payment_mode // "unknown"' "${version_body}")"
qr_available="$(jq -r '.payment_runtime.card_qr_sessions_available // false' "${version_body}")"
note "public /version payment_mode=${payment_mode} card_qr_sessions_available=${qr_available}"

note "done machine=${MACHINE_CODE} id=${machine_id} periodic_flag=${PERIODIC_FLAG_KEY}"
echo "NEXT: device bootstrap refresh (reboot app or wait for sync) to apply flags + payment methods on kiosk."
