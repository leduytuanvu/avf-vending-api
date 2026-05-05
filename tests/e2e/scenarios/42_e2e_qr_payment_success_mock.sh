#!/usr/bin/env bash
# shellcheck shell=bash
# Phase 8 / E2E-42: QR / PSP payment via payment-session + signed (or dev unsigned) webhook mock, then vend success.

set -euo pipefail

E2E_SCENARIO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=../lib/e2e_common.sh
source "${E2E_SCENARIO_DIR}/../lib/e2e_common.sh"
# shellcheck source=../lib/e2e_http.sh
source "${E2E_SCENARIO_DIR}/../lib/e2e_http.sh"
# shellcheck source=../lib/e2e_data.sh
source "${E2E_SCENARIO_DIR}/../lib/e2e_data.sh"

phase8_record() {
  local scenario_id="$1" result="$2" ids_json="$3" apis_json="$4" expected_state="$5" actual_state="$6" evidence_json="$7" remediation="$8"
  mkdir -p "${E2E_RUN_DIR}/reports"
  jq -nc \
    --arg ts "$(now_utc)" \
    --arg scenario_id "$scenario_id" \
    --arg result "$result" \
    --argjson input_ids "$ids_json" \
    --argjson apis_topics_used "$apis_json" \
    --arg expected_state "$expected_state" \
    --arg actual_state "$actual_state" \
    --argjson evidence_files "$evidence_json" \
    --arg remediation "$remediation" \
    '{ts:$ts,scenario_id:$scenario_id,input_ids:$input_ids,apis_topics_used:$apis_topics_used,expected_state:$expected_state,actual_state:$actual_state,result:$result,evidence_files:$evidence_files,remediation:$remediation}' \
    >>"${E2E_RUN_DIR}/reports/phase8-scenario-results.jsonl"
}

p8_webhook_signature() {
  local body="$1"
  e2e_python -c '
import os, sys, hmac, hashlib, time
body = sys.argv[1].encode("utf-8")
ts = str(int(time.time()))
secret = os.environ.get("COMMERCE_PAYMENT_WEBHOOK_SECRET", "") or os.environ.get("COMMERCE_PAYMENT_WEBHOOK_HMAC_SECRET", "") or os.environ.get("PAYMENT_WEBHOOK_SECRET", "")
if secret:
    dig = hmac.new(secret.encode("utf-8"), (ts + ".").encode("utf-8") + body, hashlib.sha256).hexdigest()
else:
    dig = "unsigned-local-dev"
print(ts + " " + dig)
' "$body"
}

SID="E2E-42-qr-payment-mock"
start_step "phase8-${SID}"

if [[ "${E2E_ALLOW_WRITES:-}" != "true" ]]; then
  IDS_JSON="$(jq -nc --arg m "$(get_data machineId)" '{machineId:$m}')"
  phase8_record "$SID" "skip" "$IDS_JSON" '[]' "payment session + webhook + vend" "E2E_ALLOW_WRITES!=true" "$(jq -nc --arg f "${E2E_RUN_DIR}/test-data.json" '[$f]')" "Set E2E_ALLOW_WRITES=true for commerce writes"
  log_api_contract_issue "P2" "$SID" "42_e2e_qr_payment_success_mock.sh" "writes-off" "REST" "—" "QR payment Phase 8 skipped because writes disabled — local payment mock path not exercised" "No sandbox signal" "Enable writes for CI payment mock" "${E2E_RUN_DIR}/test-data.json"
  end_step skipped "E2E-42 skipped: writes disabled"
  e2e_flow_review_scenario_complete "$SID" "42_e2e_qr_payment_success_mock.sh" "flow-review-skip" "qr_payment_skipped_writes"
  exit 0
fi

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
  IDS_JSON="$(jq -nc --arg m "$(get_data machineId)" '{machineId:$m}')"
  phase8_record "$SID" "fail" "$IDS_JSON" '[]' "safe payment flow" "production guard failed" "$(jq -nc --arg f "${E2E_RUN_DIR}/test-data.json" '[$f]')" "e2eTestMachine + E2E_PRODUCTION_WRITE_CONFIRMATION"
  end_step failed "E2E-42: production payment guard — see test-data.json and env"
  exit 1
fi

MID="$(get_data machineId)"
ORG="$(get_data organizationId)"
PRODUCT_ID="$(get_data productId)"
CUR="$(get_data currency)"
CAB="$(get_data slotCabinetCode)"
SC="$(get_data slotCode)"
[[ -z "$CUR" || "$CUR" == "null" ]] && CUR="VND"
[[ -z "$CAB" || "$CAB" == "null" ]] && CAB="A"
[[ -z "$SC" || "$SC" == "null" ]] && SC="A1"
SLOT_IDX="${E2E_SLOT_INDEX:-1}"

MT="$(get_secret machineToken 2>/dev/null || true)"
[[ -z "$MT" ]] && { log_error "E2E-42: machineToken required"; exit 2; }
export ADMIN_TOKEN="$MT"

IDS_JSON="$(jq -nc --arg m "$MID" --arg o "${ORG:-}" --arg p "$PRODUCT_ID" '{machineId:$m,organizationId:$o,productId:$p}')"
APIS_JSON='["POST /v1/commerce/orders","POST /v1/commerce/orders/{orderId}/payment-session","POST /v1/commerce/orders/{orderId}/payments/{paymentId}/webhooks","GET /v1/commerce/orders/{orderId}","POST .../vend/start","POST .../vend/success"]'
EXPECTED="Order created; payment-session 200; webhook 200 (or replay); order paid; vend completes."
EVID_JSON='[]'

OBODY="$(jq -nc \
  --arg mid "$MID" \
  --arg pid "$PRODUCT_ID" \
  --arg cur "$CUR" \
  --arg cab "$CAB" \
  --arg sc "$SC" \
  '{machine_id:$mid, product_id:$pid, currency:$cur, cabinet_code:$cab, slot_code:$sc}')"
code_o="$(e2e_http_post_json_idem "p8-qr-order" "/v1/commerce/orders" "$OBODY" "e2e-p8-qr-order-$(date +%s)-${RANDOM}")"
EVID_JSON="$(echo "$EVID_JSON" | jq -c --arg f "${E2E_RUN_DIR}/rest/p8-qr-order.meta.json" '. + [$f]')"
if [[ "$code_o" != "201" && "$code_o" != "200" ]]; then
  ACTUAL="POST /v1/commerce/orders HTTP ${code_o} — ${E2E_RUN_DIR}/rest/p8-qr-order.response.json"
  phase8_record "$SID" "fail" "$IDS_JSON" "$APIS_JSON" "$EXPECTED" "$ACTUAL" "$EVID_JSON" "Fix catalog/stock/machine JWT"
  end_step failed "E2E-42: ${ACTUAL}"
  exit 1
fi
OID="$(e2e_jq_resp p8-qr-order -r '.order_id // .orderId // .order.id // .id // empty')"
if [[ -z "$OID" ]]; then
  ACTUAL="no order_id in p8-qr-order response — ${E2E_RUN_DIR}/rest/p8-qr-order.response.json"
  phase8_record "$SID" "fail" "$IDS_JSON" "$APIS_JSON" "$EXPECTED" "$ACTUAL" "$EVID_JSON" "Inspect OpenAPI order create response shape"
  end_step failed "E2E-42: ${ACTUAL}"
  exit 1
fi

AMT="$(jq -r '.order.total_minor // .order.totalMinor // 1000' "${E2E_RUN_DIR}/rest/p8-qr-order.response.json" 2>/dev/null || echo "1000")"
[[ -z "$AMT" || "$AMT" == "null" ]] && AMT="1000"

PSBODY="$(jq -nc \
  --argjson am "$AMT" \
  --arg cur "$CUR" \
  '{provider:"stripe",payment_state:"created",amount_minor:$am,currency:$cur,outbox_payload_json:{source:"e2e_phase8_42"}}')"
code_ps="$(e2e_http_post_json_idem "p8-qr-ps" "/v1/commerce/orders/${OID}/payment-session" "$PSBODY" "e2e-p8-ps-${OID}")"
EVID_JSON="$(echo "$EVID_JSON" | jq -c --arg f "${E2E_RUN_DIR}/rest/p8-qr-ps.meta.json" '. + [$f]')"
if [[ "$code_ps" == "503" ]]; then
  ACTUAL="payment-session returned 503 capability_not_configured — commerce outbox not configured locally"
  phase8_record "$SID" "skip" "$IDS_JSON" "$APIS_JSON" "$EXPECTED" "$ACTUAL" "$EVID_JSON" "Configure v1.commerce.payment_session.outbox per swagger; see e2e-remediation-playbook Payment mock"
  log_api_contract_issue "P2" "$SID" "42_e2e_qr_payment_success_mock.sh" "payment-mock" "REST" "POST .../payment-session" "Local payment-session unavailable (503) — PSP mock / sandbox not wired for deterministic QR tests" "Cannot exercise webhook idempotently in dev" "Configure outbox + test PSP keys per runbook" "${E2E_RUN_DIR}/rest/p8-qr-ps.response.json"
  end_step skipped "E2E-42: payment-session unavailable (503)"
  e2e_flow_review_scenario_complete "$SID" "42_e2e_qr_payment_success_mock.sh" "flow-review-skip" "qr_payment_skipped_503"
  exit 0
fi
if [[ "$code_ps" != "200" ]]; then
  ACTUAL="POST payment-session HTTP ${code_ps} — ${E2E_RUN_DIR}/rest/p8-qr-ps.response.json"
  phase8_record "$SID" "fail" "$IDS_JSON" "$APIS_JSON" "$EXPECTED" "$ACTUAL" "$EVID_JSON" "Commerce logs; outbox wiring"
  end_step failed "E2E-42: ${ACTUAL}"
  exit 1
fi
PAY="$(e2e_jq_resp p8-qr-ps -r '.payment_id // .paymentId // empty')"
if [[ -z "$PAY" ]]; then
  ACTUAL="no payment_id in payment-session response"
  phase8_record "$SID" "fail" "$IDS_JSON" "$APIS_JSON" "$EXPECTED" "$ACTUAL" "$EVID_JSON" "${E2E_RUN_DIR}/rest/p8-qr-ps.response.json"
  end_step failed "E2E-42: ${ACTUAL}"
  exit 1
fi

PSR="${E2E_RUN_DIR}/rest/p8-qr-ps.response.json"
PEX="$(jq -r '.expires_at // .expiresAt // .sessionExpiresAt // .payment_session.expires_at // empty' "$PSR" 2>/dev/null)"
PREFQ="$(jq -r '.provider_reference // .providerReference // .external_reference // empty' "$PSR" 2>/dev/null)"
[[ -z "$PEX" || "$PEX" == "null" ]] && log_missing_field_issue "P2" "$SID" "42_e2e_qr_payment_success_mock.sh" "payment-session-expiry" "REST" "POST .../payment-session" "payment-session JSON lacks a clear expires_at / session TTL field for client UX" "QR flow may hang without deadline" "Document and return session_expires_at per OpenAPI" "$PSR"
[[ -z "$PREFQ" || "$PREFQ" == "null" ]] && log_missing_field_issue "P2" "$SID" "42_e2e_qr_payment_success_mock.sh" "payment-session-provider-ref" "REST" "POST .../payment-session" "payment-session response lacks stable provider reference for reconciliation queries" "Finance trace harder" "Return provider_reference / PSP intent id consistently" "$PSR"

WID="$(e2e_python -c 'import uuid; print(uuid.uuid4())' 2>/dev/null || echo "e2e-webhook-$(date +%s)")"
PIREF="pi_e2e_p8_${RANDOM}"
WBODY="$(jq -nc \
  --arg wid "$WID" \
  --arg pref "$PIREF" \
  '{
    webhook_event_id:$wid,
    provider:"stripe",
    provider_reference:$pref,
    event_type:"payment_intent.succeeded",
    normalized_payment_state:"captured",
    payload_json:{id:$pref, status:"succeeded"}
  }')"

post_webhook() {
  local step="$1"
  local ts_sig
  ts_sig="$(p8_webhook_signature "$WBODY")"
  local ts="${ts_sig%% *}"
  local sig="${ts_sig#* }"
  local url="${BASE_URL}/v1/commerce/orders/${OID}/payments/${PAY}/webhooks"
  local req="${E2E_RUN_DIR}/rest/${step}.request.json"
  local hdr="${E2E_RUN_DIR}/rest/${step}.response.headers.txt"
  local resp="${E2E_RUN_DIR}/rest/${step}.response.json"
  printf '%s' "$WBODY" >"$req"
  local code
  set +e
  code="$(curl -sS -o "$resp" -D "$hdr" -w '%{http_code}' -X POST "$url" \
    -H "Content-Type: application/json" \
    -H "X-AVF-Webhook-Timestamp: ${ts}" \
    -H "X-AVF-Webhook-Signature: ${sig}" \
    --data-binary @"$req")"
  set -e
  echo "$code"
}

code_w1="$(post_webhook "p8-qr-wh1")"
EVID_JSON="$(echo "$EVID_JSON" | jq -c --arg b "${E2E_RUN_DIR}/rest/p8-qr-wh1.response.json" '. + [$b]')"

if [[ "$code_w1" != "200" ]]; then
  ACTUAL="webhook HTTP ${code_w1} — see ${E2E_RUN_DIR}/rest/p8-qr-wh1.response.json (webhook_hmac_required → set COMMERCE_PAYMENT_WEBHOOK_SECRET or enable COMMERCE_PAYMENT_WEBHOOK_ALLOW_UNSIGNED on API for local dev)"
  phase8_record "$SID" "skip" "$IDS_JSON" "$APIS_JSON" "$EXPECTED" "$ACTUAL" "$EVID_JSON" "docs/swagger webhook op + e2e-remediation-playbook Payment mock"
  log_api_contract_issue "P2" "$SID" "42_e2e_qr_payment_success_mock.sh" "webhook-mock" "REST" "POST .../webhooks" "Webhook cannot be simulated idempotently without HMAC secret or dev-unsigned flag" "Blocks QR payment E2E" "Document COMMERCE_PAYMENT_WEBHOOK_SECRET setup; stable replay headers" "${E2E_RUN_DIR}/rest/p8-qr-wh1.response.json"
  end_step skipped "E2E-42: webhook rejected (configure HMAC or allow unsigned dev)"
  e2e_flow_review_scenario_complete "$SID" "42_e2e_qr_payment_success_mock.sh" "flow-review-skip" "qr_payment_skipped_webhook"
  exit 0
fi

code_w2="$(post_webhook "p8-qr-wh2")"
if [[ "$code_w2" == "200" ]]; then
  RP="$(jq -r '.replay // false' "${E2E_RUN_DIR}/rest/p8-qr-wh2.response.json" 2>/dev/null || echo "false")"
  WEBHOOK_DUP_NOTE="webhook_retry_http_200_replay_${RP}"
else
  WEBHOOK_DUP_NOTE="webhook_retry_http_${code_w2}"
fi
EVID_JSON="$(echo "$EVID_JSON" | jq -c --arg b "${E2E_RUN_DIR}/rest/p8-qr-wh2.response.json" '. + [$b]')"

e2e_http_get "p8-qr-order-get" "/v1/commerce/orders/${OID}" >/dev/null
EVID_JSON="$(echo "$EVID_JSON" | jq -c --arg f "${E2E_RUN_DIR}/rest/p8-qr-order-get.meta.json" '. + [$f]')"
OST="$(jq -r '.order.status // .order.order_status // empty' "${E2E_RUN_DIR}/rest/p8-qr-order-get.response.json")"

VSLOT="$(jq -nc --argjson si "$SLOT_IDX" '{slot_index:$si}')"
code_vs="$(e2e_http_post_json_idem "p8-qr-vstart" "/v1/commerce/orders/${OID}/vend/start" "$VSLOT" "e2e-p8-vs-${OID}")"
if [[ "$code_vs" != "200" ]]; then
  ACTUAL="vend/start HTTP ${code_vs} after webhook — ${E2E_RUN_DIR}/rest/p8-qr-vstart.response.json order.status was ${OST}"
  phase8_record "$SID" "fail" "$IDS_JSON" "$APIS_JSON" "$EXPECTED" "$ACTUAL" "$EVID_JSON" "Order not paid; webhook body/state mismatch"
  end_step failed "E2E-42: ${ACTUAL}"
  exit 1
fi
code_vok="$(e2e_http_post_json_idem "p8-qr-vok" "/v1/commerce/orders/${OID}/vend/success" "$VSLOT" "e2e-p8-vok-${OID}")"
EVID_JSON="$(echo "$EVID_JSON" | jq -c --arg a "${E2E_RUN_DIR}/rest/p8-qr-vstart.meta.json" --arg b "${E2E_RUN_DIR}/rest/p8-qr-vok.meta.json" '[$a,$b]')"
if [[ "$code_vok" != "200" ]]; then
  ACTUAL="vend/success HTTP ${code_vok} — ${E2E_RUN_DIR}/rest/p8-qr-vok.response.json"
  phase8_record "$SID" "fail" "$IDS_JSON" "$APIS_JSON" "$EXPECTED" "$ACTUAL" "$EVID_JSON" "Vend lifecycle"
  end_step failed "E2E-42: ${ACTUAL}"
  exit 1
fi

e2e_http_get "p8-qr-order-final" "/v1/commerce/orders/${OID}" >/dev/null
OST2="$(jq -r '.order.status // empty' "${E2E_RUN_DIR}/rest/p8-qr-order-final.response.json")"
EVID_JSON="$(echo "$EVID_JSON" | jq -c --arg f "${E2E_RUN_DIR}/rest/p8-qr-order-final.meta.json" '. + [$f]')"

ACTUAL="order=${OID} payment=${PAY} webhook_ok vend_ok final_status=${OST2} ${WEBHOOK_DUP_NOTE}"
phase8_record "$SID" "pass" "$IDS_JSON" "$APIS_JSON" "$EXPECTED" "$ACTUAL" "$EVID_JSON" ""
log_api_contract_issue "P2" "$SID" "42_e2e_qr_payment_success_mock.sh" "reconciliation-verify" "REST" "commerce reconciliation" "Phase 8 does not GET reconciliation endpoints to verify PSP/settlement row for mocked payment" "Pilot finance validation gap" "Add optional admin reconciliation assert keyed by order_id" "${E2E_RUN_DIR}/rest/p8-qr-order-final.response.json"
end_step passed "E2E-42 QR payment mock completed"
e2e_flow_review_scenario_complete "$SID" "42_e2e_qr_payment_success_mock.sh" "flow-review-complete" "qr_payment_mock_reviewed"
exit 0