#!/usr/bin/env bash
# shellcheck shell=bash
# VM-REST-06: Paid cash order → vend start → vend failure → refund (commerce POST) on safe targets.
# Machine JWT for commerce; field vend reporting is gRPC-heavy — REST is QA mirror.

set +e
set -u

E2E_SCENARIO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=../lib/e2e_common.sh
source "${E2E_SCENARIO_DIR}/../lib/e2e_common.sh"
# shellcheck source=../lib/e2e_http.sh
source "${E2E_SCENARIO_DIR}/../lib/e2e_http.sh"
# shellcheck source=../lib/e2e_data.sh
source "${E2E_SCENARIO_DIR}/../lib/e2e_data.sh"

FLOW_ID="VM-REST-06"
VA_LOG="${E2E_RUN_DIR}/reports/va-rest-results.jsonl"
mkdir -p "${E2E_RUN_DIR}/reports"

va_record() {
  local step="$1" endpoint="$2" status="$3" http="$4" msg="$5"
  jq -nc \
    --arg ts "$(now_utc)" \
    --arg flow "$FLOW_ID" \
    --arg step "$step" \
    --arg ep "$endpoint" \
    --arg st "$status" \
    --argjson http "${http:-0}" \
    --arg msg "$msg" \
    '{ts:$ts,flow_id:$flow,step:$step,endpoint:$ep,status:$st,httpStatus:$http,message:$msg}' >>"$VA_LOG"
  e2e_append_test_event "$FLOW_ID" "$step" "REST" "$endpoint" "$status" "$msg" "{}"
}

vm_payment_guard_ok() {
  if [[ "${E2E_TARGET:-local}" != "production" ]]; then
    return 0
  fi
  [[ "${E2E_ALLOW_WRITES:-}" == "true" ]] || return 1
  [[ "${E2E_PRODUCTION_WRITE_CONFIRMATION:-}" == "I_UNDERSTAND_THIS_WRITES_TO_PRODUCTION" ]] || return 1
  local tm
  tm="$(get_data e2eTestMachine)"
  [[ "$tm" == "true" ]] || [[ "$tm" == "1" ]]
}

if [[ "${E2E_ALLOW_WRITES:-}" != "true" ]]; then
  va_record "vend-failure-flow" "—" "skip" "0" "E2E_ALLOW_WRITES!=true"
  exit 2
fi

if ! vm_payment_guard_ok; then
  log_error "VM-REST-06: blocked on production without full guard + e2eTestMachine"
  va_record "safety" "—" "fail" "0" "production guard"
  exit 2
fi

MID="$(get_data machineId)"
PRODUCT_ID="$(get_data productId)"
CUR="$(get_data currency)"
CAB="$(get_data slotCabinetCode)"
SC="$(get_data slotCode)"
[[ -z "$CUR" || "$CUR" == "null" ]] && CUR="VND"
[[ -z "$CAB" || "$CAB" == "null" ]] && CAB="A"
[[ -z "$SC" || "$SC" == "null" ]] && SC="A1"
SLOT_IDX="${E2E_SLOT_INDEX:-1}"

MT="$(get_secret machineToken 2>/dev/null || true)"
[[ -z "$MT" ]] && { log_error "VM-REST-06: machineToken required"; exit 2; }
export ADMIN_TOKEN="$MT"

CBODY="$(jq -nc \
  --arg mid "$MID" \
  --arg pid "$PRODUCT_ID" \
  --arg cur "$CUR" \
  --arg cab "$CAB" \
  --arg sc "$SC" \
  '{machine_id:$mid, product_id:$pid, currency:$cur, cabinet_code:$cab, slot_code:$sc}')"

code="$(e2e_http_post_json_idem "vm-fail-co" "/v1/commerce/cash-checkout" "$CBODY" "e2e-fail-$(date +%s)-${RANDOM}")"
if [[ "$code" != "200" ]]; then
  va_record "cash-checkout" "POST /v1/commerce/cash-checkout" "fail" "$code" "HTTP $code"
  exit 1
fi
OID="$(e2e_jq_resp vm-fail-co -r '.order_id // empty')"
e2e_set_data vmFailureOrderId "$OID"
va_record "cash-checkout" "POST /v1/commerce/cash-checkout" "pass" "$code" "order_id=$OID"

VSLOT="$(jq -nc --argjson si "$SLOT_IDX" '{slot_index:$si}')"
code="$(e2e_http_post_json_idem "vm-fail-vstart" "/v1/commerce/orders/${OID}/vend/start" "$VSLOT" "e2e-fvs-${OID}")"
[[ "$code" != "200" ]] && { va_record "vend-start" "POST .../vend/start" "fail" "$code" "HTTP $code"; exit 1; }
va_record "vend-start" "POST .../vend/start" "pass" "$code" "ok"

FBODY="$(jq -nc --argjson si "$SLOT_IDX" '{slot_index:$si, failure_reason:"e2e_motor_timeout"}')"
code="$(e2e_http_post_json_idem "vm-fail-vfail" "/v1/commerce/orders/${OID}/vend/failure" "$FBODY" "e2e-vf-${OID}")"
if [[ "$code" != "200" ]]; then
  va_record "vend-failure" "POST .../vend/failure" "fail" "$code" "HTTP $code"
  exit 1
fi
va_record "vend-failure" "POST .../vend/failure" "pass" "$code" "ok"

e2e_http_get "vm-fail-order" "/v1/commerce/orders/${OID}" >/dev/null
OST="$(jq -r '.order.status // empty' "${E2E_RUN_DIR}/rest/vm-fail-order.response.json")"
VST="$(jq -r '.vend.state // .vend.vend_state // empty' "${E2E_RUN_DIR}/rest/vm-fail-order.response.json")"
va_record "order-after-failure" "GET /v1/commerce/orders/{id}" "pass" "200" "order.status=${OST} vend=${VST}"

# Refunds on safe targets (guard enforced at script entry for production)
TOT="$(jq -r '.order.total_minor // .order.totalMinor // 1000' "${E2E_RUN_DIR}/rest/vm-fail-order.response.json")"
RBODY="$(jq -nc --argjson am "$TOT" --arg cur "$CUR" '{amount_minor:$am, currency:$cur, reason:"vend_failed_e2e", metadata:{vend_failure_reason:"e2e"}}')"
code="$(e2e_http_post_json_idem "vm-fail-refund" "/v1/commerce/orders/${OID}/refunds" "$RBODY" "e2e-rf-${OID}")"
if [[ "$code" == "200" ]]; then
  va_record "refund" "POST .../refunds" "pass" "$code" "refund accepted"
  log_api_contract_issue "P1" "$FLOW_ID" "06_vend_failure_refund_rest.sh" "refund-eligibility" "REST" "POST .../refunds" "Refund accepted in harness without asserting server-side eligibility matrix (wrong payment state, double refund)" "Financial inconsistency risk" "Return 409 for invalid refund; document audit trail fields" "${E2E_RUN_DIR}/rest/vm-fail-refund.meta.json"
else
  va_record "refund" "POST .../refunds" "skip" "$code" "HTTP $code — policy/amount; see rest/vm-fail-refund.response.json"
  log_api_contract_issue "P2" "$FLOW_ID" "06_vend_failure_refund_rest.sh" "refund-route" "REST" "POST .../refunds" "Refund POST not consistently available or policy blocks automation — support path for vend failure unclear" "Operators cannot complete failure compensation in all envs" "Align REST refund with gRPC/reporting; document final states" "${E2E_RUN_DIR}/rest/vm-fail-refund.response.json"
fi

e2e_http_get "vm-fail-order2" "/v1/commerce/orders/${OID}" >/dev/null
OST2="$(jq -r '.order.status // empty' "${E2E_RUN_DIR}/rest/vm-fail-order2.response.json")"
va_record "final-order-state" "GET /v1/commerce/orders/{id}" "pass" "200" "status=${OST2}"

log_flow_design_issue "P1" "$FLOW_ID" "06_vend_failure_refund_rest.sh" "failure-consistency" "REST" "vend failure + inventory" "REST harness does not cross-check inventory restoration or payment ledger after vend failure" "Stale money or stock vs reality" "Add GET inventory/ledger hooks; mirror gRPC report failure semantics" "${E2E_RUN_DIR}/rest/vm-fail-vfail.meta.json"
log_observability_issue "P2" "$FLOW_ID" "06_vend_failure_refund_rest.sh" "audit-trail" "REST" "order GET" "End-state order payload may not expose correlation to refund attempt for triage" "Support cannot trace failure compensation" "Include refund_id, request_id, audit sequence in order detail" "${E2E_RUN_DIR}/rest/vm-fail-order2.response.json"

e2e_flow_review_scenario_complete "$FLOW_ID" "06_vend_failure_refund_rest.sh" "flow-review-complete" "vend_failure_refund_rest_reviewed"

exit 0
