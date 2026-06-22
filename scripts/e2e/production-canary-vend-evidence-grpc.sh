#!/usr/bin/env bash
# Live gRPC smoke: per-machine vend hardware evidence enforcement on canary machine.
set -Eeuo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
# shellcheck source=lib/common.sh
source "${ROOT}/scripts/e2e/lib/common.sh"

e2e_require_cmd curl jq grpcurl
e2e_init_run_dir "${E2E_CANARY_LABEL:-production-canary-vend-evidence-grpc}"

_env_file="${E2E_ENV_FILE:-${ROOT}/tests/e2e/.env.production.destructive.local}"
if [[ -f "${_env_file}" ]]; then
  _env_tmp="$(mktemp)"
  tr -d '\r' <"${_env_file}" >"${_env_tmp}"
  set -a
  # shellcheck disable=SC1090
  source "${_env_tmp}" || true
  set +a
  rm -f "${_env_tmp}"
fi
CANARY_ENV="${ROOT}/tests/e2e/production/.env.production.e2e.local"
if [[ -f "$CANARY_ENV" ]]; then
  _canary_tmp="$(mktemp)"
  tr -d '\r' <"$CANARY_ENV" >"$_canary_tmp"
  set -a
  # shellcheck disable=SC1090
  source "$_canary_tmp"
  set +a
  rm -f "$_canary_tmp"
  : "${BASE_URL:=${E2E_PROD_BASE_URL:-}}"
fi
export BASE_URL="${BASE_URL:-https://api.ldtv.dev}"
TEST_MACHINE_ID="${TEST_MACHINE_ID:-${E2E_TEST_MACHINE_ID:-019e702c-11c6-7ab0-89c7-5eb32f0b12cb}}"
TEST_PRODUCT_ID="${TEST_PRODUCT_ID:-${E2E_TEST_PRODUCT_ID:-}}"
TEST_SLOT_CODE="${TEST_SLOT_CODE:-${E2E_TEST_SLOT_CODE:-A1}}"
SLOT_INDEX="${SLOT_INDEX:-1}"
TEST_PRICE_MINOR="${TEST_PRICE_MINOR:-150}"
TEST_CURRENCY="${TEST_CURRENCY:-VND}"
: >"${E2E_RUN_DIR}/probes.tsv"
FAILURES=0

pass_probe() { e2e_record_probe "$1" "PASS" "${2:-}"; echo "PASS $1"; }
fail_probe() { e2e_record_probe "$1" "FAIL" "${2:-}"; echo "FAIL $1"; FAILURES=$((FAILURES + 1)); }

DIGEST="4d7a1c1f2b3e4a5d6c7b8a9f0e1d2c3b4a5f6e7d8c9b0a1f2e3d4c5b6a7f8e9d"
_run_ts="$(date -u +%s)"

ctx() {
  local prefix="$1"
  local suffix="$2"
  jq -nc --arg k "${prefix}-${suffix}" --arg e "evt-${prefix}-${suffix}" --arg ts "$(e2e_now_utc)" \
    '{idempotencyKey:$k, clientEventId:$e, clientCreatedAt:$ts}'
}

evidence_fixture() {
  jq -nc \
    --arg digest "$DIGEST" \
    --arg va "$(uuidgen 2>/dev/null || echo 11111111-1111-1111-1111-111111111111)" \
    --arg corr "$(uuidgen 2>/dev/null || echo 22222222-2222-2222-2222-222222222222)" \
    --argjson amt "$TEST_PRICE_MINOR" \
    --arg cur "$TEST_CURRENCY" \
    --arg slot "$TEST_SLOT_CODE" \
    '{
      vendAttemptId:$va, correlationId:$corr,
      command:{commandId:"cmd-1", txRxDigest:$digest},
      billFinal:{eventId:"bill-1", amountMinor:$amt, currency:$cur},
      tcnDispense:{slot:$slot, result:"ok", dropped:true, digest:$digest}
    }'
}

run_order_through_vend_start() {
  local prefix="$1"
  local tag="$2"
  local pid="$3"
  local create_ctx order_body oid
  create_ctx="$(ctx "${prefix}" "${tag}-create")"
  order_body="$(jq -nc \
    --argjson ctx "$create_ctx" \
    --arg pid "$pid" \
    --arg cur "$TEST_CURRENCY" \
    --argjson slot "$SLOT_INDEX" \
    '{context:$ctx, productId:$pid, currency:$cur, slot:{slotIndex:$slot}}')"
  e2e_grpc_call "avf.machine.v1.MachineCommerceService/CreateOrder" "$order_body" "ev-${tag}-create-order" machine "$(echo "$create_ctx" | jq -r '.idempotencyKey')" || return 1
  oid="$(jq -r '.orderId // empty' "${E2E_RUN_DIR}/raw/ev-${tag}-create-order.response.json")"
  [[ -n "$oid" ]] || return 1
  local cash_ctx vstart_ctx
  cash_ctx="$(ctx "${prefix}" "${tag}-cash")"
  vstart_ctx="$(ctx "${prefix}" "${tag}-vstart")"
  e2e_grpc_call "avf.machine.v1.MachineCommerceService/ConfirmCashPayment" \
    "$(jq -nc --argjson ctx "$cash_ctx" --arg oid "$oid" '{context:$ctx, orderId:$oid}')" \
    "ev-${tag}-cash" machine "$(echo "$cash_ctx" | jq -r '.idempotencyKey')" || return 1
  e2e_grpc_call "avf.machine.v1.MachineCommerceService/StartVend" \
    "$(jq -nc --argjson ctx "$vstart_ctx" --arg oid "$oid" --argjson si "$SLOT_INDEX" '{context:$ctx, orderId:$oid, slotIndex:$si}')" \
    "ev-${tag}-vstart" machine "$(echo "$vstart_ctx" | jq -r '.idempotencyKey')" || return 1
  printf '%s' "$oid"
}

export GRPC_ADDR="${GRPC_ADDR:-machine-api.ldtv.dev:443}"
export GRPC_MAX_TIME="${GRPC_MAX_TIME:-120}"
export MACHINE_ACCESS_TOKEN="${MACHINE_ACCESS_TOKEN:-${MACHINE_TOKEN:-}}"
[[ -n "$MACHINE_ACCESS_TOKEN" ]] || { echo "FATAL: MACHINE_ACCESS_TOKEN required (mint via run-production-canary-vend-evidence-with-mint.sh)" >&2; exit 2; }

# Resolve catalog from production bootstrap/planogram when product unset
if [[ -z "$TEST_PRODUCT_ID" ]]; then
  if ! e2e_grpc_call "avf.machine.v1.MachineBootstrapService/GetBootstrap" '{}' "ev-bootstrap" machine ""; then
    fail_probe "evidence.bootstrap" "GetBootstrap failed"
    exit 1
  fi
  e2e_redact_tokens_in_json_file "${E2E_RUN_DIR}/raw/ev-bootstrap.response.json"
  TEST_PRODUCT_ID="$(jq -r '.products[0].productId // empty' "${E2E_RUN_DIR}/raw/ev-bootstrap.response.json" 2>/dev/null)"
  TEST_CURRENCY="$(jq -r '.products[0].currency // .currency // "VND"' "${E2E_RUN_DIR}/raw/ev-bootstrap.response.json" 2>/dev/null)"
  TEST_PRICE_MINOR="$(jq -r '.products[0].priceMinor // .products[0].price_minor // empty' "${E2E_RUN_DIR}/raw/ev-bootstrap.response.json" 2>/dev/null)"
  SLOT_INDEX="$(jq -r --arg sc "$TEST_SLOT_CODE" '.planogram.slots[]? | select(.slotCode==$sc) | .slotIndex' "${E2E_RUN_DIR}/raw/ev-bootstrap.response.json" 2>/dev/null | head -n1)"
  [[ -n "$SLOT_INDEX" && "$SLOT_INDEX" != "null" ]] || SLOT_INDEX=1
fi
[[ -n "$TEST_PRODUCT_ID" ]] || { fail_probe "evidence.product_id" "missing from bootstrap"; exit 1; }
TEST_PRICE_MINOR="${TEST_PRICE_MINOR:-150}"

write_verdict() {
  jq -nc \
    --arg machine "$TEST_MACHINE_ID" \
    --arg reject_order "${REJECT_ORDER_ID:-}" \
    --arg success_order "${SUCCESS_ORDER_ID:-}" \
    --arg fail_order "${FAIL_ORDER_ID:-}" \
    --argjson failures "$FAILURES" \
    '{machineId:$machine, rejectOrderId:$reject_order, successOrderId:$success_order, failOrderId:$fail_order, failures:$failures, verdict:(if $failures==0 then "VERIFIED_LIVE" else "FAILED" end)}' \
    >"${E2E_RUN_DIR}/EVIDENCE_CANARY_VERDICT.json"
}

fail_gate4_hard_stop() {
  local reason="$1"
  fail_probe "evidence.reject_without" "$reason"
  write_verdict
  exit 1
}

# Order A — reject no-evidence (fresh order)
REJECT_ORDER_ID="$(run_order_through_vend_start "gate4-reject-${_run_ts}" "reject" "$TEST_PRODUCT_ID")" || REJECT_ORDER_ID=""
if [[ -z "$REJECT_ORDER_ID" ]]; then
  fail_probe "evidence.create_order" "CreateOrder/cash/start failed for reject subcase"
  write_verdict
  exit 1
fi
pass_probe "evidence.create_order" "CreateOrder lat=${E2E_GRPC_LAST_LAT:-0}ms order=${REJECT_ORDER_ID}"
pass_probe "evidence.cash" "ConfirmCashPayment ok"
pass_probe "evidence.start_vend" "StartVend ok"

vs_no_ctx="$(ctx "gate4-reject-${_run_ts}" "vsucc-no-ev")"
vs_no_ev="$(jq -nc --argjson ctx "$vs_no_ctx" --arg oid "$REJECT_ORDER_ID" --argjson si "$SLOT_INDEX" '{context:$ctx, orderId:$oid, slotIndex:$si}')"
if e2e_grpc_call "avf.machine.v1.MachineCommerceService/ConfirmVendSuccess" "$vs_no_ev" "ev-vsuccess-no-evidence" machine "$(echo "$vs_no_ctx" | jq -r '.idempotencyKey')"; then
  fail_gate4_hard_stop "allowlist not live — ConfirmVendSuccess succeeded without evidence"
fi
pass_probe "evidence.reject_without" "ConfirmVendSuccess rejected without evidence"

# Order B — fixture success + idempotent replay (fresh order)
SUCCESS_ORDER_ID="$(run_order_through_vend_start "gate4-success-${_run_ts}" "success" "$TEST_PRODUCT_ID")" || SUCCESS_ORDER_ID=""
if [[ -z "$SUCCESS_ORDER_ID" ]]; then
  fail_probe "evidence.accept_with_fixture" "could not create order for success subcase"
  write_verdict
  exit 1
fi
evidence="$(evidence_fixture)"
vs_ev_ctx="$(ctx "gate4-success-${_run_ts}" "vsucc-ev")"
vs_ev="$(jq -nc --argjson ctx "$vs_ev_ctx" --arg oid "$SUCCESS_ORDER_ID" --argjson si "$SLOT_INDEX" --argjson ev "$evidence" '{context:$ctx, orderId:$oid, slotIndex:$si, evidence:$ev}')"
if e2e_grpc_call "avf.machine.v1.MachineCommerceService/ConfirmVendSuccess" "$vs_ev" "ev-vsuccess-with-evidence" machine "$(echo "$vs_ev_ctx" | jq -r '.idempotencyKey')"; then
  pass_probe "evidence.accept_with_fixture" "ConfirmVendSuccess with evidence"
else
  fail_probe "evidence.accept_with_fixture" "expected accept with valid evidence"
fi
if e2e_grpc_call "avf.machine.v1.MachineCommerceService/ConfirmVendSuccess" "$vs_ev" "ev-vsuccess-with-evidence-replay" machine "$(echo "$vs_ev_ctx" | jq -r '.idempotencyKey')"; then
  pass_probe "evidence.idempotent_replay" "ConfirmVendSuccess replay same key"
else
  fail_probe "evidence.idempotent_replay" "expected idempotent replay"
fi

# Order C — ReportVendFailure + fixture evidence + replay (fresh order)
FAIL_ORDER_ID="$(run_order_through_vend_start "gate4-fail-${_run_ts}" "vfail" "$TEST_PRODUCT_ID")" || FAIL_ORDER_ID=""
if [[ -n "$FAIL_ORDER_ID" ]]; then
  fail_evidence="$(evidence_fixture)"
  vfail_ctx="$(ctx "gate4-fail-${_run_ts}" "vfail")"
  vfail_body="$(jq -nc \
    --argjson ctx "$vfail_ctx" \
    --arg oid "$FAIL_ORDER_ID" \
    --argjson si "$SLOT_INDEX" \
    --argjson ev "$fail_evidence" \
    '{context:$ctx, orderId:$oid, slotIndex:$si, failureReason:"canary-fixture-failure", evidence:$ev}')"
  if e2e_grpc_call "avf.machine.v1.MachineCommerceService/ReportVendFailure" "$vfail_body" "ev-vfail-with-evidence" machine "$(echo "$vfail_ctx" | jq -r '.idempotencyKey')"; then
    pass_probe "evidence.report_failure_with_evidence" "ReportVendFailure with fixture evidence"
    if e2e_grpc_call "avf.machine.v1.MachineCommerceService/ReportVendFailure" "$vfail_body" "ev-vfail-with-evidence-replay" machine "$(echo "$vfail_ctx" | jq -r '.idempotencyKey')"; then
      pass_probe "evidence.report_failure_idempotent" "ReportVendFailure idempotent replay"
    else
      fail_probe "evidence.report_failure_idempotent" "ReportVendFailure replay failed"
    fi
  else
    fail_probe "evidence.report_failure_with_evidence" "ReportVendFailure with evidence failed"
  fi
else
  fail_probe "evidence.report_failure_with_evidence" "could not create separate order for ReportVendFailure"
fi

write_verdict

[[ "$FAILURES" -eq 0 ]] || exit 1
echo "VERIFIED_LIVE evidence canary: ${E2E_RUN_DIR}"
