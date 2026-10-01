#!/usr/bin/env bash
# Read-only verification for AVF000195 commissioning (2026-10-01 logcat session).
#
# Prefers admin API when ADMIN_TOKEN or ADMIN_EMAIL+ADMIN_PASSWORD are set.
# Falls back to direct Postgres when DATABASE_URL is set or FORCE_DB=1 on app-node.
#
# Usage:
#   bash scripts/ops/verify_avf000195_commissioning.sh
#   FORCE_DB=1 bash scripts/ops/verify_avf000195_commissioning.sh
set -Eeuo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
# shellcheck source=../e2e/lib/common.sh
source "${ROOT}/scripts/e2e/lib/common.sh"

BASE_URL="${BASE_URL:-https://api.ldtv.dev}"
BASE_URL="${BASE_URL%/}"
export BASE_URL

MACHINE_CODE="${MACHINE_CODE:-AVF000195}"
MACHINE_ID="${MACHINE_ID:-01a0a7e5-3c68-7895-b526-bcb6504bccfb}"
ORDER_15K="${ORDER_15K:-01a0f5fa-5f54-7d77-8a04-0a0352a51a43}"
ORDER_20K="${ORDER_20K:-01a0f5fb-53cc-7eff-835a-b59b01bb81d3}"
SQL_FILE="${ROOT}/scripts/ops/verify_avf000195_commissioning.sql"

: "${ADMIN_EMAIL:=${E2E_PROD_ADMIN_EMAIL:-}}"
: "${ADMIN_PASSWORD:=${E2E_PROD_ADMIN_PASSWORD:-}}"
: "${ADMIN_TOKEN:=${E2E_PROD_ADMIN_TOKEN:-}}"

e2e_require_cmd curl jq
e2e_init_run_dir "verify-avf000195-commissioning"

fail() { echo "verify-commissioning: error: $*" >&2; exit 1; }
note() { echo "verify-commissioning: $*"; }

fetch_admin_json() {
  local name="$1"
  local url="$2"
  local token="$3"
  local out="${E2E_RUN_DIR}/raw/${name}.body"
  local code
  code="$(curl -sS -o "$out" -w '%{http_code}' \
    -H "Authorization: Bearer ${token}" \
    -H "Accept: application/json" \
    --connect-timeout 8 --max-time 25 \
    "$url")"
  jq -nc --arg name "$name" --arg url "$url" --argjson code "${code:-0}" \
    '{name:$name,url:$url,http_code:$code}' >>"${E2E_RUN_DIR}/admin-index.ndjson"
  if [[ "$code" != "200" ]]; then
    note "WARN ${name} http=${code} url=${url}"
    return 1
  fi
  jq . "$out" >"${E2E_RUN_DIR}/raw/${name}.json"
  return 0
}

summarize_order() {
  local file="$1"
  local label="$2"
  jq -nc --arg label "$label" \
    --arg machineCode "$(jq -r '.machineCode // .machine_code // ""' "$file")" \
    --arg status "$(jq -r '.status // ""' "$file")" \
    --arg paymentState "$(jq -r '.paymentState // .payment_state // ""' "$file")" \
    --arg paymentMethod "$(jq -r '.paymentMethod // .payment_method // ""' "$file")" \
    --argjson totalMinor "$(jq -r '.totalMinor // .total_minor // 0' "$file")" \
    --argjson itemCount "$(jq -r '(.items // .orderItems // []) | length' "$file")" \
    --arg vendStates "$(jq -r '[(.items // .orderItems // [])[] | (.vendState // .vend_state // "?")] | join(",")' "$file")" \
    '{
      label: $label,
      machineCode: $machineCode,
      status: $status,
      paymentState: $paymentState,
      paymentMethod: $paymentMethod,
      totalMinor: $totalMinor,
      itemCount: $itemCount,
      vendStates: $vendStates
    }'
}

verify_via_admin_api() {
  local token
  token="$(e2e_admin_token)" || fail "admin auth required (ADMIN_TOKEN or ADMIN_EMAIL+ADMIN_PASSWORD)"
  note "using admin API ${BASE_URL}"

  fetch_admin_json "machine-detail" "${BASE_URL}/v1/admin/machines/${MACHINE_ID}" "$token" || true
  fetch_admin_json "machine-inventory" "${BASE_URL}/v1/admin/machines/${MACHINE_ID}/inventory" "$token" || true
  fetch_admin_json "machine-planogram-current" "${BASE_URL}/v1/admin/machines/${MACHINE_ID}/planogram/current" "$token" || true
  fetch_admin_json "order-15k" "${BASE_URL}/v1/admin/orders/${ORDER_15K}" "$token" || true
  fetch_admin_json "order-20k" "${BASE_URL}/v1/admin/orders/${ORDER_20K}" "$token" || true

  local summary="${E2E_RUN_DIR}/verification-summary.json"
  jq -nc \
    --arg verifiedAt "$(e2e_now_utc)" \
    --arg machineCode "$MACHINE_CODE" \
    --arg machineId "$MACHINE_ID" \
    --arg order15k "$ORDER_15K" \
    --arg order20k "$ORDER_20K" \
    --argjson machine "$(jq -c '{code:.code,status:.status,machineType:.machineType,publishedPlanogramVersionId:(.publishedPlanogramVersionId // null)}' "${E2E_RUN_DIR}/raw/machine-detail.json" 2>/dev/null || echo '{}')" \
    --argjson order15 "$(summarize_order "${E2E_RUN_DIR}/raw/order-15k.json" "order-15k" 2>/dev/null || echo '{}')" \
    --argjson order20 "$(summarize_order "${E2E_RUN_DIR}/raw/order-20k.json" "order-20k" 2>/dev/null || echo '{}')" \
    '{
      verifiedAt: $verifiedAt,
      machineCode: $machineCode,
      machineId: $machineId,
      orderIds: { order15k: $order15k, order20k: $order20k },
      machine: $machine,
      orders: { order15k: $order15, order20k: $order20 }
    }' >"$summary"
  note "wrote ${summary}"
}

verify_via_db() {
  [[ -f "$SQL_FILE" ]] || fail "missing SQL file ${SQL_FILE}"
  if [[ -z "${DATABASE_URL:-}" ]]; then
    fail "DATABASE_URL required for DB verification"
  fi
  note "using DATABASE_URL psql"
  psql "$DATABASE_URL" -f "$SQL_FILE" | tee "${E2E_RUN_DIR}/db-verify.log"
}

if [[ -n "${FORCE_DB:-}" && "${FORCE_DB}" == "1" ]]; then
  verify_via_db
  exit 0
fi

if e2e_admin_token >/dev/null 2>&1; then
  verify_via_admin_api
  exit 0
fi

if [[ -n "${DATABASE_URL:-}" ]]; then
  verify_via_db
  exit 0
fi

fail "set ADMIN_TOKEN or ADMIN_EMAIL+ADMIN_PASSWORD for API, or DATABASE_URL / FORCE_DB=1 for SQL"
