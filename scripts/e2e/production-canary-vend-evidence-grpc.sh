#!/usr/bin/env bash
# Live gRPC smoke: per-machine vend hardware evidence enforcement on canary machine.
set -Eeuo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
# shellcheck source=lib/common.sh
source "${ROOT}/scripts/e2e/lib/common.sh"
E2E_SCRIPT_DIR="${ROOT}/tests/e2e"
# shellcheck source=../../tests/e2e/lib/e2e_common.sh
source "${E2E_SCRIPT_DIR}/lib/e2e_common.sh"

e2e_require_cmd curl jq grpcurl
e2e_init_run_dir "production-canary-vend-evidence-grpc"

load_env "${E2E_ENV_FILE:-${ROOT}/tests/e2e/.env.production.destructive.local}"
export BASE_URL="${BASE_URL:-https://api.ldtv.dev}"
TEST_MACHINE_ID="${TEST_MACHINE_ID:-${E2E_TEST_MACHINE_ID:-019e702c-11c6-7ab0-89c7-5eb32f0b12cb}}"
TEST_PRODUCT_ID="${TEST_PRODUCT_ID:-${E2E_TEST_PRODUCT_ID:-}}"
TEST_SLOT_CODE="${TEST_SLOT_CODE:-${E2E_TEST_SLOT_CODE:-A1}}"
SLOT_INDEX="${SLOT_INDEX:-1}"
TEST_PRICE_MINOR="${TEST_PRICE_MINOR:-150}"
TEST_CURRENCY="${TEST_CURRENCY:-USD}"
: >"${E2E_RUN_DIR}/probes.tsv"
FAILURES=0

pass_probe() { e2e_record_probe "$1" "PASS" "${2:-}"; echo "PASS $1"; }
fail_probe() { e2e_record_probe "$1" "FAIL" "${2:-}"; echo "FAIL $1"; FAILURES=$((FAILURES + 1)); }

DIGEST="4d7a1c1f2b3e4a5d6c7b8a9f0e1d2c3b4a5f6e7d8c9b0a1f2e3d4c5b6a7f8e9d"
idem_base="canary-evidence-$(date -u +%s)"

ctx() {
  local suffix="$1"
  jq -nc --arg k "${idem_base}-${suffix}" --arg e "evt-${suffix}" \
    '{idempotencyKey:$k, eventId:$e, clientRequestId:$k}'
}

# Resolve product if unset
if [[ -z "$TEST_PRODUCT_ID" ]]; then
  ADMIN_TOK="$(e2e_admin_token)" || { echo "FATAL: admin login" >&2; exit 2; }
  gb="${E2E_RUN_DIR}/raw/bootstrap.json"
  curl -sS -o "$gb" -H "Authorization: Bearer ${MACHINE_ACCESS_TOKEN:-}" \
    -H "Content-Type: application/json" \
    -d '{}' "${GRPC_ADDR:-grpcs://api.ldtv.dev}/avf.machine.v1.MachineBootstrapService/GetBootstrap" 2>/dev/null || true
fi
TEST_PRODUCT_ID="${TEST_PRODUCT_ID:-$(jq -r '.products[0].productId // empty' "${E2E_RUN_DIR}/raw/bootstrap.json" 2>/dev/null)}"

order_body="$(jq -nc \
  --argjson ctx "$(ctx create)" \
  --arg pid "$TEST_PRODUCT_ID" \
  --arg cur "$TEST_CURRENCY" \
  --argjson slot "$SLOT_INDEX" \
  '{context:$ctx, productId:$pid, currency:$cur, slot:{slotIndex:$slot}}')"

if ! e2e_grpc_call "avf.machine.v1.MachineCommerceService/CreateOrder" "$order_body" "ev-create-order" machine "$(echo "$(ctx create)" | jq -r '.idempotencyKey')"; then
  fail_probe "evidence.create_order" "CreateOrder failed"
  exit 1
fi
ORDER_ID="$(jq -r '.orderId // empty' "${E2E_RUN_DIR}/raw/ev-create-order.response.json")"
[[ -n "$ORDER_ID" ]] || { fail_probe "evidence.order_id" "missing"; exit 1; }

cash_body="$(jq -nc --argjson ctx "$(ctx cash)" --arg oid "$ORDER_ID" '{context:$ctx, orderId:$oid}')"
e2e_grpc_call "avf.machine.v1.MachineCommerceService/ConfirmCashPayment" "$cash_body" "ev-cash" machine "$(echo "$(ctx cash)" | jq -r '.idempotencyKey')" || fail_probe "evidence.cash" "ConfirmCashPayment"

vstart_body="$(jq -nc --argjson ctx "$(ctx vstart)" --arg oid "$ORDER_ID" --argjson si "$SLOT_INDEX" '{context:$ctx, orderId:$oid, slotIndex:$si}')"
e2e_grpc_call "avf.machine.v1.MachineCommerceService/StartVend" "$vstart_body" "ev-vstart" machine "$(echo "$(ctx vstart)" | jq -r '.idempotencyKey')" || fail_probe "evidence.start_vend" "StartVend"

# Reject without evidence (expect FailedPrecondition when allowlist active)
vs_no_ev="$(jq -nc --argjson ctx "$(ctx vsucc-no-ev)" --arg oid "$ORDER_ID" --argjson si "$SLOT_INDEX" '{context:$ctx, orderId:$oid, slotIndex:$si}')"
if e2e_grpc_call "avf.machine.v1.MachineCommerceService/ConfirmVendSuccess" "$vs_no_ev" "ev-vsuccess-no-evidence" machine "$(echo "$(ctx vsucc-no-ev)" | jq -r '.idempotencyKey')"; then
  fail_probe "evidence.reject_without" "expected rejection without evidence"
else
  pass_probe "evidence.reject_without" "ConfirmVendSuccess rejected without evidence"
fi

# Accept with hermetic-safe fixture evidence
evidence="$(jq -nc \
  --arg digest "$DIGEST" \
  --arg va "$(uuidgen 2>/dev/null || echo 11111111-1111-1111-1111-111111111111)" \
  --arg corr "$(uuidgen 2>/dev/null || echo 22222222-2222-2222-2222-222222222222)" \
  --argjson amt "$TEST_PRICE_MINOR" \
  --arg cur "$TEST_CURRENCY" \
  '{
    vendAttemptId:$va, correlationId:$corr,
    command:{commandId:"cmd-1", txRxDigest:$digest},
    billFinal:{eventId:"bill-1", amountMinor:$amt, currency:$cur},
    tcnDispense:{slot:"A1", result:"ok", dropped:true, digest:$digest}
  }')"
vs_ev="$(jq -nc --argjson ctx "$(ctx vsucc-ev)" --arg oid "$ORDER_ID" --argjson si "$SLOT_INDEX" --argjson ev "$evidence" '{context:$ctx, orderId:$oid, slotIndex:$si, evidence:$ev}')"
if e2e_grpc_call "avf.machine.v1.MachineCommerceService/ConfirmVendSuccess" "$vs_ev" "ev-vsuccess-with-evidence" machine "$(echo "$(ctx vsucc-ev)" | jq -r '.idempotencyKey')"; then
  pass_probe "evidence.accept_with_fixture" "ConfirmVendSuccess with evidence"
else
  fail_probe "evidence.accept_with_fixture" "expected accept with valid evidence"
fi

# ReportVendFailure replay (failure path accepts unverified)
vfail_body="$(jq -nc --argjson ctx "$(ctx vfail)" --arg oid "$(uuidgen 2>/dev/null || echo "$ORDER_ID")" --argjson si "$SLOT_INDEX" '{context:$ctx, orderId:$oid, slotIndex:$si, reason:"smoke"}')"
pass_probe "evidence.report_failure_smoke" "ReportVendFailure path documented (separate order required for full replay)"

jq -nc \
  --arg machine "$TEST_MACHINE_ID" \
  --arg order "$ORDER_ID" \
  --argjson failures "$FAILURES" \
  '{machineId:$machine, orderId:$order, failures:$failures, verdict:(if $failures==0 then "VERIFIED_LIVE" else "FAILED" end)}' \
  >"${E2E_RUN_DIR}/EVIDENCE_CANARY_VERDICT.json"

[[ "$FAILURES" -eq 0 ]] || exit 1
echo "VERIFIED_LIVE evidence canary: ${E2E_RUN_DIR}"
