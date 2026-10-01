#!/usr/bin/env bash
# Verify repaired stuck-vend orders via admin API.
#
# Usage:
#   bash scripts/ops/verify_stuck_vend_repair.sh
set -Eeuo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
# shellcheck source=../e2e/lib/common.sh
source "${ROOT}/scripts/e2e/lib/common.sh"

BASE_URL="${BASE_URL:-https://api.ldtv.dev}"
BASE_URL="${BASE_URL%/}"
export BASE_URL

ORDER_15K="${ORDER_15K:-01a0f5fa-5f54-7d77-8a04-0a0352a51a43}"
ORDER_25K="${ORDER_25K:-01a0f634-9563-7e11-b3a7-a5a3225c017a}"

: "${ADMIN_EMAIL:=${E2E_PROD_ADMIN_EMAIL:-}}"
: "${ADMIN_PASSWORD:=${E2E_PROD_ADMIN_PASSWORD:-}}"
: "${ADMIN_TOKEN:=${E2E_PROD_ADMIN_TOKEN:-}}"

e2e_require_cmd curl jq
e2e_init_run_dir "verify-stuck-vend-repair"

fail() { echo "verify-stuck-vend-repair: error: $*" >&2; exit 1; }
note() { echo "verify-stuck-vend-repair: $*"; }

fetch_order() {
  local label="$1"
  local order_id="$2"
  local token="$3"
  local out="${E2E_RUN_DIR}/raw/${label}.json"
  local code
  code="$(curl -sS -o "$out" -w '%{http_code}' \
    -H "Authorization: Bearer ${token}" \
    -H "Accept: application/json" \
    --connect-timeout 8 --max-time 25 \
    "${BASE_URL}/v1/admin/orders/${order_id}")"
  [[ "$code" == "200" ]] || fail "${label} http=${code}"
  jq . "$out" >"${E2E_RUN_DIR}/raw/${label}.pretty.json"
}

summarize_order() {
  local file="$1"
  local label="$2"
  jq -nc --arg label "$label" \
    --arg status "$(jq -r '.status // ""' "$file")" \
    --argjson totalMinor "$(jq -r '.totalMinor // .total_minor // 0' "$file")" \
    --argjson itemCount "$(jq -r '(.items // .orderItems // []) | length' "$file")" \
    --arg vendStates "$(jq -r '[(.items // .orderItems // [])[] | (.vendState // .vend_state // "?")] | join(",")' "$file")" \
    --arg vendLabels "$(jq -r '[(.items // .orderItems // [])[] | (.vendState // .vend_state // "?")] | join(",")' "$file")" \
    '{
      label: $label,
      status: $status,
      totalMinor: $totalMinor,
      itemCount: $itemCount,
      vendStates: $vendStates
    }'
}

token="$(e2e_admin_token)" || fail "admin auth required"
note "using admin API ${BASE_URL}"

fetch_order "order-15k" "${ORDER_15K}" "$token"
fetch_order "order-25k" "${ORDER_25K}" "$token"

summary="${E2E_RUN_DIR}/verification-summary.json"
jq -nc \
  --arg verifiedAt "$(e2e_now_utc)" \
  --argjson order15 "$(summarize_order "${E2E_RUN_DIR}/raw/order-15k.json" "order-15k")" \
  --argjson order25 "$(summarize_order "${E2E_RUN_DIR}/raw/order-25k.json" "order-25k")" \
  '{
    verifiedAt: $verifiedAt,
    orders: { order15k: $order15, order25k: $order25 },
    expectations: {
      order15k: { status: "completed", vendStates: "success,success" },
      order25k: { status: "partially_completed", vendStates: "success,success,failed" }
    }
  }' >"$summary"

note "wrote ${summary}"
jq . "$summary"

status15="$(jq -r '.orders.order15k.status' "$summary")"
states15="$(jq -r '.orders.order15k.vendStates' "$summary")"
status25="$(jq -r '.orders.order25k.status' "$summary")"
states25="$(jq -r '.orders.order25k.vendStates' "$summary")"

ok=true
if [[ "$status15" != "completed" || "$states15" != "success,success" ]]; then
  note "WARN order15k status=${status15} vendStates=${states15}"
  ok=false
fi
if [[ "$status25" != "partially_completed" || "$states25" != "success,success,failed" ]]; then
  note "WARN order25k status=${status25} vendStates=${states25}"
  ok=false
fi

if [[ "$ok" == "true" ]]; then
  note "PASS repaired orders match expected terminal states"
else
  fail "verification failed — see ${summary}"
fi
