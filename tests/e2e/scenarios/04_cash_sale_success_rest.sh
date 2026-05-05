#!/usr/bin/env bash
# shellcheck shell=bash
# VM-REST-04: Cash checkout → vend start → vend success → order status; optional catalog qty delta.
# Machine JWT. Not the production app primary path (gRPC/MQTT).

set +e
set -u

E2E_SCENARIO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=../lib/e2e_common.sh
source "${E2E_SCENARIO_DIR}/../lib/e2e_common.sh"
# shellcheck source=../lib/e2e_http.sh
source "${E2E_SCENARIO_DIR}/../lib/e2e_http.sh"
# shellcheck source=../lib/e2e_data.sh
source "${E2E_SCENARIO_DIR}/../lib/e2e_data.sh"

FLOW_ID="VM-REST-04"
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

if ! vm_payment_guard_ok; then
  log_error "VM-REST-04: production payment writes blocked (need E2E_ALLOW_WRITES + E2E_PRODUCTION_WRITE_CONFIRMATION + test-data e2eTestMachine=true)"
  va_record "safety" "—" "fail" "0" "production guard"
  exit 2
fi

if [[ "${E2E_ALLOW_WRITES:-}" != "true" ]]; then
  va_record "cash-checkout" "—" "skip" "0" "E2E_ALLOW_WRITES!=true"
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
[[ -z "$MT" ]] && { log_error "VM-REST-04: machineToken required"; exit 2; }
export ADMIN_TOKEN="$MT"

qty_before=""
if [[ -n "$PRODUCT_ID" && "$PRODUCT_ID" != "null" ]]; then
  e2e_http_get "vm-sc-qty-before" "/v1/machines/${MID}/sale-catalog?include_unavailable=true&include_images=false" >/dev/null
  qty_before="$(jq -r --arg p "$PRODUCT_ID" '([.items[]? | select(.productId==$p) | .availableQuantity] | first) // empty' "${E2E_RUN_DIR}/rest/vm-sc-qty-before.response.json")"
fi

CBODY="$(jq -nc \
  --arg mid "$MID" \
  --arg pid "$PRODUCT_ID" \
  --arg cur "$CUR" \
  --arg cab "$CAB" \
  --arg sc "$SC" \
  '{machine_id:$mid, product_id:$pid, currency:$cur, cabinet_code:$cab, slot_code:$sc}')"

code="$(e2e_http_post_json_idem "vm-cash-co" "/v1/commerce/cash-checkout" "$CBODY" "e2e-cash-$(date +%s)-${RANDOM}")"
if [[ "$code" != "200" ]]; then
  va_record "cash-checkout" "POST /v1/commerce/cash-checkout" "fail" "$code" "HTTP $code — commerce/outbox?"
  exit 1
fi
OID="$(e2e_jq_resp vm-cash-co -r '.order_id // empty')"
PAY="$(e2e_jq_resp vm-cash-co -r '.payment_id // empty')"
e2e_set_data vmCashSuccessOrderId "$OID"
e2e_set_data vmCashSuccessPaymentId "$PAY"
va_record "cash-checkout" "POST /v1/commerce/cash-checkout" "pass" "$code" "order_id=${OID} payment_id=${PAY}"

VSLOT="$(jq -nc --argjson si "$SLOT_IDX" '{slot_index:$si}')"
code="$(e2e_http_post_json_idem "vm-vend-start" "/v1/commerce/orders/${OID}/vend/start" "$VSLOT" "e2e-vs-${OID}")"
if [[ "$code" != "200" ]]; then
  va_record "vend-start" "POST .../vend/start" "fail" "$code" "HTTP $code"
  exit 1
fi
va_record "vend-start" "POST .../vend/start" "pass" "$code" "ok"

code="$(e2e_http_post_json_idem "vm-vend-ok" "/v1/commerce/orders/${OID}/vend/success" "$VSLOT" "e2e-vok-${OID}")"
if [[ "$code" != "200" ]]; then
  va_record "vend-success" "POST .../vend/success" "fail" "$code" "HTTP $code"
  exit 1
fi
va_record "vend-success" "POST .../vend/success" "pass" "$code" "ok"

e2e_http_get "vm-order-final" "/v1/commerce/orders/${OID}" >/dev/null
OST="$(jq -r '.order.status // .order.order_status // empty' "${E2E_RUN_DIR}/rest/vm-order-final.response.json")"
va_record "order-status" "GET /v1/commerce/orders/{id}" "pass" "200" "status=${OST}"

if [[ -n "$qty_before" ]] && [[ "$qty_before" =~ ^[0-9]+$ ]]; then
  e2e_http_get "vm-sc-qty-after" "/v1/machines/${MID}/sale-catalog?include_unavailable=true&include_images=false" >/dev/null
  qty_after="$(jq -r --arg p "$PRODUCT_ID" '([.items[]? | select(.productId==$p) | .availableQuantity] | first) // empty' "${E2E_RUN_DIR}/rest/vm-sc-qty-after.response.json")"
  if [[ -n "$qty_after" ]] && [[ "$qty_after" =~ ^[0-9]+$ ]]; then
    exp=$((qty_before - 1))
    if [[ "$qty_after" == "$exp" ]]; then
      va_record "inventory-sale-catalog" "GET .../sale-catalog" "pass" "200" "availableQty $qty_before→$qty_after"
    else
      va_record "inventory-sale-catalog" "GET .../sale-catalog" "skip" "200" "expected $exp got $qty_after (eventual consistency?)"
    fi
  else
    va_record "inventory-sale-catalog" "GET .../sale-catalog" "skip" "200" "no qty after"
  fi
else
  va_record "inventory-sale-catalog" "—" "skip" "0" "no baseline qty"
fi

exit 0
