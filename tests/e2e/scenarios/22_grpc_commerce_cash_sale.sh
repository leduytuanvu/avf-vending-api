#!/usr/bin/env bash
# shellcheck shell=bash
# GRPC-22: Machine commerce — cash path, vend success, vend failure probe; GetOrderStatus.

set +e
set -u

E2E_SCENARIO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=../lib/e2e_common.sh
source "${E2E_SCENARIO_DIR}/../lib/e2e_common.sh"
# shellcheck source=../lib/e2e_data.sh
source "${E2E_SCENARIO_DIR}/../lib/e2e_data.sh"
# shellcheck source=../lib/e2e_grpc.sh
source "${E2E_SCENARIO_DIR}/../lib/e2e_grpc.sh"

FLOW_ID="GRPC-22"
ec=0

ORG="$(get_data organizationId)"
MID="$(get_data machineId)"
MT="$(get_secret machineToken 2>/dev/null || true)"
PRODUCT="$(get_data productId)"
CUR="$(get_data currency)"
CAB="$(get_data slotCabinetCode)"
SC="$(get_data slotCode)"
SI="${E2E_SLOT_INDEX:-1}"

[[ -z "${MT:-}" ]] && { log_error "GRPC-22: machine token required"; exit 2; }
[[ -z "${PRODUCT:-}" || "${PRODUCT}" == "null" ]] && { log_error "GRPC-22: productId required in test-data"; exit 2; }

[[ -z "${CUR:-}" || "${CUR}" == "null" ]] && CUR="VND"
[[ -z "${CAB:-}" || "${CAB}" == "null" ]] && CAB="A"
[[ -z "${SC:-}" || "${SC}" == "null" ]] && SC="A1"

export MACHINE_TOKEN="$MT"
export MACHINE_ID="$MID"

META="$(jq -nc --arg o "$ORG" --arg m "$MID" --arg rid "g22-$(date +%s)" \
  '{organizationId:$o, machineId:$m, requestId:$rid}')"

grpc_contract_skip "$FLOW_ID" "create-payment-session-psp" MachineCommerceService CreatePaymentSession \
  "skipped_in_favor_of_confirm_cash_payment_for_contract_run"

SCEN="22_grpc_commerce_cash_sale.sh"
if [[ "${E2E_ALLOW_WRITES:-}" != "true" ]]; then
  grpc_contract_skip "$FLOW_ID" "create-order-gate" MachineCommerceService CreateOrder "E2E_ALLOW_WRITES_not_true"
  grpc_contract_skip "$FLOW_ID" "confirm-cash-gate" MachineCommerceService ConfirmCashPayment "E2E_ALLOW_WRITES_not_true"
  grpc_contract_skip "$FLOW_ID" "start-vend-gate" MachineCommerceService StartVend "E2E_ALLOW_WRITES_not_true"
  grpc_contract_skip "$FLOW_ID" "confirm-vend-gate" MachineCommerceService ConfirmVendSuccess "E2E_ALLOW_WRITES_not_true"
  grpc_contract_skip "$FLOW_ID" "report-failure-gate" MachineCommerceService ReportVendFailure "E2E_ALLOW_WRITES_not_true"
  OID="$(get_data vmCashSuccessOrderId)"
  [[ -z "${OID:-}" || "${OID}" == "null" ]] && OID="$(get_data grpcLastOrderId)"
  if [[ -n "${OID:-}" ]]; then
    ST_BODY="$(jq -nc --arg oid "$OID" --argjson si "$SI" '{orderId:$oid, slotIndex:$si}')"
    grpc_contract_try "$FLOW_ID" "get-order-status-readonly" MachineCommerceService GetOrderStatus "$ST_BODY" "g22-status-ro" "" || true
  else
    grpc_contract_skip "$FLOW_ID" "get-order-status-readonly" MachineCommerceService GetOrderStatus "no_order_id_in_test_data"
  fi
  log_data_setup_issue "P3" "$FLOW_ID" "$SCEN" "grpc-commerce-writes-gate" "gRPC" "MachineCommerceService/*" "gRPC commerce write RPCs skipped — E2E_ALLOW_WRITES not true (readonly GetOrderStatus path only)" "Reduced money/idempotency coverage in this run" "Run lab with E2E_ALLOW_WRITES=true; keep production guards" "${E2E_RUN_DIR}/test-data.json"
  e2e_flow_review_scenario_complete "$FLOW_ID" "$SCEN" "flow-review-readonly" "grpc_commerce_readonly"
  exit 0
fi

TS="$(date +%s)"
CO_BODY="$(jq -nc \
  --arg ik "g22-co-${TS}" \
  --arg ce "g22-ce-${TS}" \
  --arg m "$MID" \
  --arg p "$PRODUCT" \
  --arg cab "$CAB" \
  --arg sc "$SC" \
  --argjson si "$SI" \
  --arg cur "$CUR" \
  --argjson meta "$META" \
  '{context:{idempotencyKey:$ik, clientEventId:$ce}, machineId:$m, productId:$p,
    slot:{cabinetCode:$cab, slotCode:$sc, slotIndex:$si}, currency:$cur, meta:$meta}')"
grpc_contract_try "$FLOW_ID" "create-order" MachineCommerceService CreateOrder "$CO_BODY" "g22-create-order" "g22-co-${TS}" || ec=1

OID="$(jq -r '.orderId // empty' "${E2E_RUN_DIR}/grpc/g22-create-order.response.json")"
[[ -n "$OID" ]] && e2e_set_data grpcLastOrderId "$OID"

TS2="$(date +%s)"
CC_BODY="$(jq -nc --arg ik "g22-cc-${TS2}" --arg ce "g22-ce-cc-${TS2}" --arg oid "$OID" \
  '{context:{idempotencyKey:$ik, clientEventId:$ce}, orderId:$oid}')"
grpc_contract_try "$FLOW_ID" "confirm-cash-payment" MachineCommerceService ConfirmCashPayment "$CC_BODY" "g22-cash" "g22-cc-${TS2}" || ec=1

GO_BODY="$(jq -nc --arg oid "$OID" --argjson si "$SI" '{orderId:$oid, slotIndex:$si}')"
grpc_contract_try "$FLOW_ID" "get-order" MachineCommerceService GetOrder "$GO_BODY" "g22-get-order" "" || ec=1

TS3="$(date +%s)"
SV_BODY="$(jq -nc --arg ik "g22-sv-${TS3}" --arg ce "g22-ce-sv-${TS3}" --arg oid "$OID" --argjson si "$SI" \
  '{context:{idempotencyKey:$ik, clientEventId:$ce}, orderId:$oid, slotIndex:$si}')"
grpc_contract_try "$FLOW_ID" "start-vend" MachineCommerceService StartVend "$SV_BODY" "g22-vend-start" "g22-sv-${TS3}" || ec=1

TS4="$(date +%s)"
VS_BODY="$(jq -nc --arg ik "g22-vs-${TS4}" --arg ce "g22-ce-vs-${TS4}" --arg oid "$OID" --argjson si "$SI" \
  '{context:{idempotencyKey:$ik, clientEventId:$ce}, orderId:$oid, slotIndex:$si}')"
grpc_contract_try "$FLOW_ID" "confirm-vend-success" MachineCommerceService ConfirmVendSuccess "$VS_BODY" "g22-vend-ok" "g22-vs-${TS4}" || ec=1

ST_BODY="$(jq -nc --arg oid "$OID" --argjson si "$SI" '{orderId:$oid, slotIndex:$si}')"
grpc_contract_try "$FLOW_ID" "get-order-status-success" MachineCommerceService GetOrderStatus "$ST_BODY" "g22-status-ok" "" || ec=1

if [[ "${ec}" -eq 0 ]]; then
TS5="$(date +%s)"
CO2_BODY="$(jq -nc \
  --arg ik "g22-co2-${TS5}" \
  --arg ce "g22-ce2-${TS5}" \
  --arg m "$MID" \
  --arg p "$PRODUCT" \
  --arg cab "$CAB" \
  --arg sc "$SC" \
  --argjson si "$SI" \
  --arg cur "$CUR" \
  --argjson meta "$META" \
  '{context:{idempotencyKey:$ik, clientEventId:$ce}, machineId:$m, productId:$p,
    slot:{cabinetCode:$cab, slotCode:$sc, slotIndex:$si}, currency:$cur, meta:$meta}')"
grpc_contract_try "$FLOW_ID" "create-order-fail-path" MachineCommerceService CreateOrder "$CO2_BODY" "g22-create-order-fail" "g22-co2-${TS5}" || ec=1

OID2="$(jq -r '.orderId // empty' "${E2E_RUN_DIR}/grpc/g22-create-order-fail.response.json")"
[[ -n "$OID2" ]] && e2e_set_data grpcFailOrderId "$OID2"

TS6="$(date +%s)"
CC2_BODY="$(jq -nc --arg ik "g22-cc2-${TS6}" --arg ce "g22-ce-cc2-${TS6}" --arg oid "$OID2" \
  '{context:{idempotencyKey:$ik, clientEventId:$ce}, orderId:$oid}')"
grpc_contract_try "$FLOW_ID" "confirm-cash-fail-path" MachineCommerceService ConfirmCashPayment "$CC2_BODY" "g22-cash-fail" "g22-cc2-${TS6}" || ec=1

TS7="$(date +%s)"
SV2_BODY="$(jq -nc --arg ik "g22-sv2-${TS7}" --arg ce "g22-ce-sv2-${TS7}" --arg oid "$OID2" --argjson si "$SI" \
  '{context:{idempotencyKey:$ik, clientEventId:$ce}, orderId:$oid, slotIndex:$si}')"
grpc_contract_try "$FLOW_ID" "start-vend-fail-path" MachineCommerceService StartVend "$SV2_BODY" "g22-vend-start-fail" "g22-sv2-${TS7}" || ec=1

TS8="$(date +%s)"
VF_BODY="$(jq -nc --arg ik "g22-vf-${TS8}" --arg ce "g22-ce-vf-${TS8}" --arg oid "$OID2" --argjson si "$SI" \
  '{context:{idempotencyKey:$ik, clientEventId:$ce}, orderId:$oid, slotIndex:$si, failureReason:"e2e_grpc_simulated_fault"}')"
grpc_contract_try "$FLOW_ID" "report-vend-failure" MachineCommerceService ReportVendFailure "$VF_BODY" "g22-vend-fail" "g22-vf-${TS8}" || ec=1

ST2_BODY="$(jq -nc --arg oid "$OID2" --argjson si "$SI" '{orderId:$oid, slotIndex:$si}')"
grpc_contract_try "$FLOW_ID" "get-order-status-after-failure" MachineCommerceService GetOrderStatus "$ST2_BODY" "g22-status-fail" "" || ec=1
else
  grpc_contract_skip "$FLOW_ID" "vend-failure-probe" MachineCommerceService ReportVendFailure "skipped_success_path_incomplete"
fi

log_rest_grpc_issue "P2" "$FLOW_ID" "$SCEN" "rest-vs-grpc" "mixed" "VM-REST-04 vs gRPC" "Cash/vend lifecycle is duplicated across REST QA harness and gRPC — responses and ordering may drift" "Split-brain for operators" "Single source contract table; parity tests" "${E2E_RUN_DIR}/grpc/g22-create-order.response.json"
if [[ "${GRPC_USE_REFLECTION:-false}" != "true" ]]; then
  log_docs_issue "P2" "$FLOW_ID" "$SCEN" "grpc-entry" "gRPC" "${GRPC_ADDR:-}" "Document grpcurl invocation without reflection for commerce methods" "Field integrators blocked" "Dev guide: GRPC_PROTO_ROOT + GRPC_ADDR" "${E2E_RUN_DIR}/grpc/g22-create-order.meta.json"
fi
if [[ "${ec}" -eq 0 ]]; then
  e2e_flow_review_scenario_complete "$FLOW_ID" "$SCEN" "flow-review-complete" "grpc_commerce_ok"
else
  log_api_contract_issue "P2" "$FLOW_ID" "$SCEN" "grpc-commerce-incomplete" "gRPC" "MachineCommerceService" "One or more gRPC commerce steps failed — idempotency/retry semantics for CreateOrder/ConfirmCash/Vend not fully exercised" "Automation gap vs REST VM harness" "Align error surfaces; document replay keys" "${E2E_RUN_DIR}/grpc/g22-create-order.meta.json"
fi

exit "${ec}"