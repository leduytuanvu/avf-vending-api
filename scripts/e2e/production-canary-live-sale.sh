#!/usr/bin/env bash
# shellcheck shell=bash
# Guarded production canary live sale — mutates ONLY a marked canary test machine.
#
# Required:
#   PRODUCTION_LIVE_TEST_CONFIRMATION=I_UNDERSTAND_THIS_CAN_CHARGE_AND_VEND
#   BASE_URL, GRPC_ADDR, MACHINE_ACCESS_TOKEN
#   TEST_MACHINE_ID, TEST_SITE_ID, TEST_SLOT_CODE, TEST_PRODUCT_ID
#   TEST_PRICE_MINOR, TEST_PAYMENT_METHOD (cash for current cash-only pilot)
#   TEST_OPERATOR_NAME, PRODUCTION_CANARY_ROLLBACK_PLAN
#
# Optional:
#   PRODUCTION_E2E_MAX_PRICE_MINOR=50000 (default)
#   PRODUCTION_CANARY_MACHINE_ALLOWLIST=uuid1,uuid2
#   SIMULATE_HARDWARE_VEND=false (default — real hardware required for market PASS)
#   BACKEND_ONLY_DRY_RUN=true (required with SIMULATE on production; never fleet GO)
#   SLOT_INDEX=1, TEST_CURRENCY=VND, APP_ENV=production

set -Eeuo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
# shellcheck source=lib/common.sh
source "${ROOT}/scripts/e2e/lib/common.sh"
# shellcheck source=lib/canary-live-sale-verdict.sh
source "${ROOT}/scripts/e2e/lib/canary-live-sale-verdict.sh"

e2e_require_cmd curl jq grpcurl
e2e_init_run_dir "production-canary-live-sale"

BASE_URL="${BASE_URL:-${PRODUCTION_BASE_URL:-https://api.ldtv.dev}}"
BASE_URL="${BASE_URL%/}"
export BASE_URL
APP_ENV="${APP_ENV:-production}"

PRODUCTION_E2E_MAX_PRICE_MINOR="${PRODUCTION_E2E_MAX_PRICE_MINOR:-50000}"
TEST_CURRENCY="${TEST_CURRENCY:-VND}"
SLOT_INDEX="${SLOT_INDEX:-1}"
SIMULATE_HARDWARE_VEND="${SIMULATE_HARDWARE_VEND:-false}"
BACKEND_ONLY_DRY_RUN="${BACKEND_ONLY_DRY_RUN:-false}"

: >"${E2E_RUN_DIR}/probes.tsv"
ORDER_LOG="${E2E_RUN_DIR}/test-order-ids.log"
: >"$ORDER_LOG"
FAILURES=0
final_status="unknown"
final_vend_state="unknown"
hardware_vend_completed=0
inventory_delta_state="skip"
MARKET_CANARY_MODE="true"

pass_probe() { e2e_record_probe "$1" "PASS" "${2:-}" "${3:-0}"; echo "PASS  $1 ${2:+($2)}"; }
fail_probe() { e2e_record_probe "$1" "FAIL" "${2:-}" "${3:-0}"; echo "FAIL  $1 ${2:-}"; FAILURES=$((FAILURES + 1)); }
skip_probe() { e2e_record_probe "$1" "SKIP" "${2:-}" "0"; echo "SKIP  $1 ${2:-}"; }

log_order() {
  local kind="$1"
  local oid="$2"
  local extra="${3:-}"
  printf '%s %s %s %s\n' "$(e2e_now_utc)" "$kind" "$oid" "$extra" | tee -a "$ORDER_LOG"
}

write_canary_manifest() {
  local manifest="${E2E_RUN_DIR}/CANARY_MANIFEST.json"
  jq -nc \
    --arg generated_at "$(e2e_now_utc)" \
    --arg operator "${TEST_OPERATOR_NAME:-}" \
    --arg machine_id "${TEST_MACHINE_ID:-}" \
    --arg site_id "${TEST_SITE_ID:-}" \
    --arg rollback "${PRODUCTION_CANARY_ROLLBACK_PLAN:-}" \
    --arg simulate "${SIMULATE_HARDWARE_VEND}" \
    --arg backend_only "${BACKEND_ONLY_DRY_RUN}" \
    --arg payment_method "${TEST_PAYMENT_METHOD:-}" \
    --arg order_id "${ORDER_ID:-}" \
    --arg run_dir "${E2E_RUN_DIR}" \
    '{
      generated_at: $generated_at,
      operator_name: $operator,
      canary_machine_id: $machine_id,
      site_id: $site_id,
      rollback_plan: $rollback,
      simulate_hardware_vend: ($simulate == "true"),
      backend_only_dry_run: ($backend_only == "true"),
      real_hardware_required: ($simulate != "true"),
      payment_method: $payment_method,
      order_id: $order_id,
      artifacts: {
        create_order: ($run_dir + "/raw/sale-create-order.response.json"),
        cash_confirmation: ($run_dir + "/raw/sale-cash.response.json"),
        payment_session: ($run_dir + "/raw/sale-payment-session.response.json"),
        start_vend: ($run_dir + "/raw/sale-vstart.response.json"),
        final_order: ($run_dir + "/raw/sale-final-order.response.json"),
        inventory_before: ($run_dir + "/raw/preflight-inventory-before.response.json"),
        inventory_after: ($run_dir + "/raw/preflight-inventory-after.response.json"),
        cleanup: ($run_dir + "/CLEANUP.txt"),
        order_log: ($run_dir + "/test-order-ids.log")
      }
    }' >"$manifest"
  echo "Manifest: ${manifest}"
}

abort_guard() {
  echo "FATAL: $1" >&2
  write_canary_manifest 2>/dev/null || true
  e2e_finalize_report "production-canary-live-sale" "BLOCKED" 2 "NO-GO" "" >/dev/null || true
  exit 2
}

echo "== Production canary live sale =="
echo "base_url=${BASE_URL}"
echo "run_dir=${E2E_RUN_DIR}"
echo "simulate_hardware_vend=${SIMULATE_HARDWARE_VEND}"
echo "backend_only_dry_run=${BACKEND_ONLY_DRY_RUN}"

if [[ "${PRODUCTION_LIVE_TEST_CONFIRMATION:-}" != "I_UNDERSTAND_THIS_CAN_CHARGE_AND_VEND" ]]; then
  abort_guard "PRODUCTION_LIVE_TEST_CONFIRMATION must be exactly I_UNDERSTAND_THIS_CAN_CHARGE_AND_VEND"
fi

if ! simulate_err="$(e2e_canary_refuse_simulated_on_production "$BASE_URL" "$SIMULATE_HARDWARE_VEND" "$BACKEND_ONLY_DRY_RUN" "$APP_ENV" 2>&1)"; then
  abort_guard "$simulate_err"
fi
if [[ "${SIMULATE_HARDWARE_VEND,,}" == "true" ]]; then
  MARKET_CANARY_MODE="false"
  pass_probe "guard.simulate_hardware" "BACKEND_ONLY_DRY_RUN=${BACKEND_ONLY_DRY_RUN} (not market canary)"
else
  pass_probe "guard.simulate_hardware" "SIMULATE_HARDWARE_VEND=false (real hardware required)"
fi

required_vars=(BASE_URL GRPC_ADDR MACHINE_ACCESS_TOKEN TEST_MACHINE_ID TEST_SITE_ID TEST_SLOT_CODE TEST_PRODUCT_ID TEST_PRICE_MINOR TEST_PAYMENT_METHOD TEST_OPERATOR_NAME PRODUCTION_CANARY_ROLLBACK_PLAN)
for v in "${required_vars[@]}"; do
  if [[ -z "${!v:-}" ]]; then
    abort_guard "missing required env: ${v}"
  fi
done

if [[ -z "${PRODUCTION_CANARY_ROLLBACK_PLAN// /}" ]]; then
  abort_guard "PRODUCTION_CANARY_ROLLBACK_PLAN must describe rollback/cleanup steps (non-empty)"
fi
pass_probe "guard.rollback_plan" "documented"

if [[ -z "${TEST_OPERATOR_NAME// /}" ]]; then
  abort_guard "TEST_OPERATOR_NAME is required (non-empty operator attribution)"
fi
pass_probe "guard.operator" "operator=${TEST_OPERATOR_NAME}"

if ! [[ "${TEST_PRICE_MINOR}" =~ ^[0-9]+$ ]]; then
  abort_guard "TEST_PRICE_MINOR must be a positive integer minor units"
fi
if [[ "${TEST_PRICE_MINOR}" -gt "${PRODUCTION_E2E_MAX_PRICE_MINOR}" ]]; then
  abort_guard "TEST_PRICE_MINOR=${TEST_PRICE_MINOR} exceeds PRODUCTION_E2E_MAX_PRICE_MINOR=${PRODUCTION_E2E_MAX_PRICE_MINOR}"
fi

case "${TEST_PAYMENT_METHOD,,}" in
  cash|qr|card) ;;
  *) abort_guard "TEST_PAYMENT_METHOD must be cash, qr, or card" ;;
esac

if e2e_is_production_canary_base_url "$BASE_URL" && [[ "${TEST_PAYMENT_METHOD,,}" != "cash" && "${BACKEND_ONLY_DRY_RUN,,}" != "true" ]]; then
  abort_guard "production cash-only pilot requires TEST_PAYMENT_METHOD=cash (got ${TEST_PAYMENT_METHOD})"
fi

export TEST_MACHINE_ID TEST_SITE_ID MACHINE_ACCESS_TOKEN
export GRPC_ADDR="${GRPC_ADDR:-${GRPC_TARGET:-}}"

ADMIN_TOK=""
if ! ADMIN_TOK="$(e2e_admin_token 2>/dev/null)"; then
  abort_guard "admin auth required to verify canary machine (ADMIN_TOKEN or ADMIN_EMAIL+ADMIN_PASSWORD)"
fi

if ! e2e_machine_is_canary "${TEST_MACHINE_ID}" "$ADMIN_TOK"; then
  abort_guard "refusing non-canary machine TEST_MACHINE_ID=${TEST_MACHINE_ID}"
fi
pass_probe "guard.canary_machine" "machine marked canary or allowlisted"

machine_doc="${E2E_RUN_DIR}/raw/canary-machine-check.json"
site_id="$(jq -r '.siteId // .site_id // empty' "$machine_doc")"
if [[ "$site_id" != "${TEST_SITE_ID}" ]]; then
  abort_guard "TEST_SITE_ID mismatch: machine site=${site_id} env=${TEST_SITE_ID}"
fi
pass_probe "guard.site_match" "site_id=${site_id}"

status="$(jq -r '.status // empty' "$machine_doc" | tr '[:upper:]' '[:lower:]')"
case "$status" in
  active|online|offline) pass_probe "guard.machine_status" "status=${status}" ;;
  *) abort_guard "machine status ${status} not eligible for canary sale" ;;
esac

# Payment deployment mode from /version (read-only) before mutations.
read -r _version_code _version_lat < <(e2e_curl_get "preflight-version" "${BASE_URL}/version" "")
version_pay_mode=""
if [[ "$_version_code" == "200" ]]; then
  version_pay_mode="$(jq -r '.payment_runtime.payment_mode // .paymentRuntime.paymentMode // empty' "${E2E_RUN_DIR}/raw/preflight-version.body")"
  pass_probe "guard.version_payment_runtime" "payment_mode=${version_pay_mode:-unknown}"
else
  fail_probe "guard.version_payment_runtime" "GET /version failed code=${_version_code}"
fi

meta="$(jq -nc --arg mid "${TEST_MACHINE_ID}" --arg rid "e2e-canary-${E2E_RUN_TS}" '{machineId:$mid,requestId:$rid}')"
preflight_body="$(jq -nc --arg mid "${TEST_MACHINE_ID}" --argjson meta "$meta" '{machineId:$mid,includeUnavailable:false,meta:$meta}')"
if ! e2e_grpc_call "avf.machine.v1.MachineCatalogService/GetCatalogSnapshot" "$preflight_body" "preflight-catalog" machine ""; then
  abort_guard "GetCatalogSnapshot failed before sale"
fi
cat_resp="${E2E_RUN_DIR}/raw/preflight-catalog.response.json"
price_found="$(jq -r --arg pid "${TEST_PRODUCT_ID}" --arg sc "${TEST_SLOT_CODE}" '
  (.snapshot.items // [])[] |
  select((.productId // .product_id // "") == $pid) |
  select((.slotCode // .slot_code // "") == $sc or $sc == "") |
  (.priceMinor // .price_minor // empty)
' "$cat_resp" | head -n1)"
if [[ -z "$price_found" || "$price_found" == "null" ]]; then
  abort_guard "TEST_PRODUCT_ID not found in catalog snapshot"
fi
if [[ "$price_found" != "${TEST_PRICE_MINOR}" ]]; then
  abort_guard "TEST_PRICE_MINOR=${TEST_PRICE_MINOR} does not match catalog priceMinor=${price_found}"
fi
pass_probe "guard.catalog_price" "product=${TEST_PRODUCT_ID} priceMinor=${price_found}"

gb_body="$(jq -nc --argjson meta "$meta" '{meta:$meta}')"
if ! e2e_grpc_call "avf.machine.v1.MachineBootstrapService/GetBootstrap" "$gb_body" "preflight-bootstrap" machine ""; then
  abort_guard "GetBootstrap failed before sale"
fi
gb_resp="${E2E_RUN_DIR}/raw/preflight-bootstrap.response.json"
cash_ok="$(jq -r '.paymentMethods.cashEnabled // .payment_methods.cash_enabled // false' "$gb_resp")"
qr_ok="$(jq -r '.paymentMethods.qrCardEnabled // .payment_methods.qr_card_enabled // false' "$gb_resp")"
pay_mode="$(jq -r '.paymentMethods.paymentMode // .payment_methods.payment_mode // empty' "$gb_resp")"
if [[ -z "$pay_mode" && -n "$version_pay_mode" ]]; then
  pay_mode="$version_pay_mode"
fi

if ! pay_err="$(e2e_canary_payment_method_allowed "${TEST_PAYMENT_METHOD}" "$pay_mode" "$cash_ok" "$qr_ok" 2>&1)"; then
  abort_guard "${pay_err} — aborting before commerce mutations"
fi
pass_probe "guard.payment_config" "method=${TEST_PAYMENT_METHOD,,} mode=${pay_mode} cash=${cash_ok} qr_card=${qr_ok}"

inv_before_body="$(jq -nc --argjson meta "$meta" '{meta:$meta}')"
if ! e2e_grpc_call "avf.machine.v1.MachineInventoryService/GetInventorySnapshot" "$inv_before_body" "preflight-inventory-before" machine ""; then
  abort_guard "GetInventorySnapshot failed before sale"
fi
inv_before_qty="$(jq -r --arg sc "${TEST_SLOT_CODE}" --argjson si "${SLOT_INDEX}" '
  (.slots // .snapshot.slots // [])[] |
  select((.slotCode // .slot_code // "") == $sc or (.slotIndex // .slot_index // -1) == $si) |
  (.quantity // .qty // 0)
' "${E2E_RUN_DIR}/raw/preflight-inventory-before.response.json" | head -n1)"
if [[ -z "$inv_before_qty" || "$inv_before_qty" == "null" ]]; then
  abort_guard "inventory snapshot missing slot ${TEST_SLOT_CODE} (slot_index=${SLOT_INDEX})"
fi
pass_probe "guard.inventory_present" "slot=${TEST_SLOT_CODE} qty_before=${inv_before_qty}"

plano_body="$(jq -nc --arg mid "${TEST_MACHINE_ID}" '{machineId:$mid}')"
if ! e2e_grpc_call "avf.machine.v1.MachineInventoryService/GetPlanogram" "$plano_body" "preflight-planogram" machine ""; then
  abort_guard "GetPlanogram failed before sale"
fi
slot_product="$(jq -r --arg sc "${TEST_SLOT_CODE}" --argjson si "${SLOT_INDEX}" --arg pid "${TEST_PRODUCT_ID}" '
  (.slots // [])[] |
  select((.slotCode // .slot_code // "") == $sc or (.slotIndex // .slot_index // -1) == $si) |
  (.productId // .product_id // "")
' "${E2E_RUN_DIR}/raw/preflight-planogram.response.json" | head -n1)"
if [[ -z "$slot_product" || "$slot_product" == "null" ]]; then
  abort_guard "planogram missing slot mapping for ${TEST_SLOT_CODE}"
fi
if [[ "$slot_product" != "${TEST_PRODUCT_ID}" ]]; then
  abort_guard "planogram slot ${TEST_SLOT_CODE} product=${slot_product} != TEST_PRODUCT_ID=${TEST_PRODUCT_ID}"
fi
pass_probe "guard.slot_mapping" "slot=${TEST_SLOT_CODE} product=${slot_product}"

# --- Commerce flow ---
stem="canary-${E2E_RUN_TS}"
ctx="$(jq -nc --arg ik "${stem}-order-ik" --arg ce "${stem}-order-ce" --arg ts "$(e2e_now_utc)" '{idempotencyKey:$ik,clientEventId:$ce,clientCreatedAt:$ts}')"
order_body="$(jq -nc \
  --argjson ctx "$ctx" \
  --argjson meta "$meta" \
  --arg mid "${TEST_MACHINE_ID}" \
  --arg pid "${TEST_PRODUCT_ID}" \
  --arg sc "${TEST_SLOT_CODE}" \
  --argjson si "${SLOT_INDEX}" \
  --arg cur "${TEST_CURRENCY}" \
  '{context:$ctx, machineId:$mid, productId:$pid, slot:{slotCode:$sc, slotIndex:$si}, currency:$cur, meta:$meta}')"

if ! e2e_grpc_call "avf.machine.v1.MachineCommerceService/CreateOrder" "$order_body" "sale-create-order" machine "$(echo "$ctx" | jq -r '.idempotencyKey')"; then
  abort_guard "CreateOrder failed"
fi
ORDER_ID="$(jq -r '.orderId // empty' "${E2E_RUN_DIR}/raw/sale-create-order.response.json")"
[[ -n "$ORDER_ID" ]] || abort_guard "CreateOrder missing orderId"
log_order "create_order" "$ORDER_ID" "operator=${TEST_OPERATOR_NAME} machine=${TEST_MACHINE_ID}"
pass_probe "commerce.create_order" "order_id=${ORDER_ID}"

pay_method="${TEST_PAYMENT_METHOD,,}"
if [[ "$pay_method" == "cash" ]]; then
  ctx_cash="$(jq -nc --arg ik "${stem}-cash-ik" --arg ce "${stem}-cash-ce" --arg ts "$(e2e_now_utc)" '{idempotencyKey:$ik,clientEventId:$ce,clientCreatedAt:$ts}')"
  cash_body="$(jq -nc --argjson ctx "$ctx_cash" --arg oid "$ORDER_ID" '{context:$ctx, orderId:$oid}')"
  if ! e2e_grpc_call "avf.machine.v1.MachineCommerceService/ConfirmCashPayment" "$cash_body" "sale-cash" machine "$(echo "$ctx_cash" | jq -r '.idempotencyKey')"; then
    abort_guard "ConfirmCashPayment failed"
  fi
  pass_probe "commerce.confirm_cash" "order_id=${ORDER_ID}"
else
  ctx_pay="$(jq -nc --arg ik "${stem}-pay-ik" --arg ce "${stem}-pay-ce" --arg ts "$(e2e_now_utc)" '{idempotencyKey:$ik,clientEventId:$ce,clientCreatedAt:$ts}')"
  pay_body="$(jq -nc \
    --argjson ctx "$ctx_pay" \
    --arg oid "$ORDER_ID" \
    --argjson amt "${TEST_PRICE_MINOR}" \
    --arg cur "${TEST_CURRENCY}" \
    '{context:$ctx, orderId:$oid, paymentState:"created", amountMinor:$amt, currency:$cur}')"
  if ! e2e_grpc_call "avf.machine.v1.MachineCommerceService/CreatePaymentSession" "$pay_body" "sale-payment-session" machine "$(echo "$ctx_pay" | jq -r '.idempotencyKey')"; then
    abort_guard "CreatePaymentSession failed — live QR/card may be disabled (cash-only production)"
  fi
  PAYMENT_ID="$(jq -r '.paymentId // empty' "${E2E_RUN_DIR}/raw/sale-payment-session.response.json")"
  [[ -n "$PAYMENT_ID" ]] || abort_guard "CreatePaymentSession missing paymentId"
  log_order "payment_session" "$ORDER_ID" "payment_id=${PAYMENT_ID}"
  pass_probe "commerce.payment_session" "payment_id=${PAYMENT_ID}"
  echo "NOTE: Live QR/card requires webhook capture or operator payment — poll GetOrderStatus manually if not auto-captured." >&2
fi

ctx_vstart="$(jq -nc --arg ik "${stem}-vstart-ik" --arg ce "${stem}-vstart-ce" --arg ts "$(e2e_now_utc)" '{idempotencyKey:$ik,clientEventId:$ce,clientCreatedAt:$ts}')"
vstart_body="$(jq -nc --argjson ctx "$ctx_vstart" --arg oid "$ORDER_ID" --argjson si "${SLOT_INDEX}" '{context:$ctx, orderId:$oid, slotIndex:$si}')"
if ! e2e_grpc_call "avf.machine.v1.MachineCommerceService/StartVend" "$vstart_body" "sale-vstart" machine "$(echo "$ctx_vstart" | jq -r '.idempotencyKey')"; then
  abort_guard "StartVend failed"
fi
pass_probe "commerce.start_vend" "order_id=${ORDER_ID}"

if [[ "${SIMULATE_HARDWARE_VEND,,}" == "true" ]]; then
  ctx_ok="$(jq -nc --arg ik "${stem}-vsuccess-ik" --arg ce "${stem}-vsuccess-ce" --arg ts "$(e2e_now_utc)" '{idempotencyKey:$ik,clientEventId:$ce,clientCreatedAt:$ts}')"
  vs_body="$(jq -nc --argjson ctx "$ctx_ok" --arg oid "$ORDER_ID" --argjson si "${SLOT_INDEX}" '{context:$ctx, orderId:$oid, slotIndex:$si}')"
  if ! e2e_grpc_call "avf.machine.v1.MachineCommerceService/ConfirmVendSuccess" "$vs_body" "sale-vsuccess" machine "$(echo "$ctx_ok" | jq -r '.idempotencyKey')"; then
    abort_guard "ConfirmVendSuccess failed (backend-only dry run)"
  fi
  pass_probe "commerce.vend_backend_simulated" "ConfirmVendSuccess backend-only order_id=${ORDER_ID}"
  hardware_vend_completed=0
else
  echo "Waiting for real hardware vend completion (timeout ${HARDWARE_VEND_WAIT_SEC:-120}s)..." >&2
  deadline=$(( $(date +%s) + ${HARDWARE_VEND_WAIT_SEC:-120} ))
  vend_done=0
  last_poll_resp=""
  while [[ $(date +%s) -lt $deadline ]]; do
    poll_tag="sale-poll-$(date +%s)"
    if e2e_grpc_call "avf.machine.v1.MachineCommerceService/GetOrderStatus" \
      "$(jq -nc --arg oid "$ORDER_ID" --argjson si "${SLOT_INDEX}" '{orderId:$oid,slotIndex:$si}')" \
      "$poll_tag" machine ""; then
      last_poll_resp="${E2E_RUN_DIR}/raw/${poll_tag}.response.json"
      ost="$(jq -r '.orderStatus // .order_status // empty' "$last_poll_resp")"
      vst="$(jq -r '.vendState // .vend_state // empty' "$last_poll_resp")"
      if e2e_canary_order_status_is_market_complete "$ost" && e2e_canary_vend_state_is_hardware_complete "$vst"; then
        vend_done=1
        final_status="$ost"
        final_vend_state="$vst"
        break
      fi
      if [[ "${vst,,}" == "failed" ]]; then
        fail_probe "commerce.vend_hardware" "hardware reported failure vend_state=${vst}"
        vend_done=2
        final_vend_state="$vst"
        break
      fi
    fi
    sleep 5
  done
  if [[ "$vend_done" -eq 0 ]]; then
    fail_probe "commerce.vend_hardware" "timeout waiting for real hardware vend completion"
  elif [[ "$vend_done" -eq 1 ]]; then
    hardware_vend_completed=1
    pass_probe "commerce.vend_hardware" "hardware completed orderStatus=${final_status} vendState=${final_vend_state}"
  elif [[ "$vend_done" -eq 2 ]]; then
    ctx_fail="$(jq -nc --arg ik "${stem}-vfail-ik" --arg ce "${stem}-vfail-ce" --arg ts "$(e2e_now_utc)" '{idempotencyKey:$ik,clientEventId:$ce,clientCreatedAt:$ts}')"
    vfail_body="$(jq -nc --argjson ctx "$ctx_fail" --arg oid "$ORDER_ID" --argjson si "${SLOT_INDEX}" '{context:$ctx, orderId:$oid, slotIndex:$si, failureReason:"e2e_canary_hardware_failure"}')"
    if e2e_grpc_call "avf.machine.v1.MachineCommerceService/ReportVendFailure" "$vfail_body" "sale-vfailure" machine "$(echo "$ctx_fail" | jq -r '.idempotencyKey')"; then
      pass_probe "commerce.report_vend_failure" "order_id=${ORDER_ID}"
    else
      fail_probe "commerce.report_vend_failure" "ReportVendFailure failed"
    fi
  fi
fi

if e2e_grpc_call "avf.machine.v1.MachineCommerceService/GetOrder" \
  "$(jq -nc --arg oid "$ORDER_ID" --argjson si "${SLOT_INDEX}" '{orderId:$oid,slotIndex:$si}')" \
  "sale-final-order" machine ""; then
  final_status="$(jq -r '.orderStatus // .order_status // empty' "${E2E_RUN_DIR}/raw/sale-final-order.response.json")"
  final_vend_state="$(jq -r '.vendState // .vend_state // empty' "${E2E_RUN_DIR}/raw/sale-final-order.response.json")"
  pass_probe "reconcile.get_order" "orderStatus=${final_status} vendState=${final_vend_state}"
  log_order "final_status" "$ORDER_ID" "status=${final_status} vend=${final_vend_state}"
  if [[ "$MARKET_CANARY_MODE" == "true" ]] && ! e2e_canary_order_status_is_market_complete "$final_status"; then
    fail_probe "reconcile.order_completed" "expected completed/success got ${final_status:-empty}"
  elif [[ "$MARKET_CANARY_MODE" == "true" ]]; then
    pass_probe "reconcile.order_completed" "orderStatus=${final_status}"
  fi
else
  fail_probe "reconcile.get_order" "GetOrder failed"
fi

inv_after_qty=""
if e2e_grpc_call "avf.machine.v1.MachineInventoryService/GetInventorySnapshot" "$inv_before_body" "preflight-inventory-after" machine ""; then
  inv_after_qty="$(jq -r --arg sc "${TEST_SLOT_CODE}" --argjson si "${SLOT_INDEX}" '
    (.slots // .snapshot.slots // [])[] |
    select((.slotCode // .slot_code // "") == $sc or (.slotIndex // .slot_index // -1) == $si) |
    (.quantity // .qty // 0)
  ' "${E2E_RUN_DIR}/raw/preflight-inventory-after.response.json" | head -n1)"
  inventory_delta_state="$(e2e_canary_inventory_delta_state "$inv_before_qty" "$inv_after_qty" "$final_status")"
  case "$inventory_delta_state" in
    pass)
      pass_probe "reconcile.inventory_delta" "before=${inv_before_qty} after=${inv_after_qty} decremented"
      ;;
    fail)
      fail_probe "reconcile.inventory_delta" "expected qty drop: before=${inv_before_qty} after=${inv_after_qty}"
      ;;
    skip)
      if [[ "$MARKET_CANARY_MODE" == "true" ]]; then
        fail_probe "reconcile.inventory_delta" "inventory qty unavailable after vend (market canary requires decrement proof)"
      else
        skip_probe "reconcile.inventory_delta" "slot qty not found after vend (backend-only)"
      fi
      inventory_delta_state="skip"
      ;;
  esac
else
  fail_probe "reconcile.inventory_after" "GetInventorySnapshot failed"
  if [[ "$MARKET_CANARY_MODE" == "true" ]]; then
    inventory_delta_state="skip"
  fi
fi

tel_idem="e2e-canary-telemetry-${E2E_RUN_TS}"
tel_ts="$(e2e_now_utc)"
tel_ctx="$(jq -nc --arg ik "$tel_idem" --arg ce "$tel_idem-ce" --arg ts "$tel_ts" '{idempotencyKey:$ik,clientEventId:$ce,clientCreatedAt:$ts}')"
tel_body="$(jq -nc --argjson ctx "$tel_ctx" --argjson meta "$meta" --arg eid "$tel_idem" --arg ts "$tel_ts" --arg oid "${ORDER_ID:-}" \
  '{context:$ctx, meta:$meta, events:[{eventId:$eid,eventType:"production_e2e_canary_sale",occurredAt:$ts,attributes:{order_id:$oid}}]}')"
if e2e_grpc_call "avf.machine.v1.MachineTelemetryService/PushTelemetryBatch" "$tel_body" "sale-telemetry" machine "$tel_idem"; then
  pass_probe "reconcile.telemetry_event" "production_e2e_canary_sale accepted"
else
  fail_probe "reconcile.telemetry_event" "PushTelemetryBatch failed"
fi

{
  echo ""
  echo "== Cleanup / rollback plan =="
  echo "${PRODUCTION_CANARY_ROLLBACK_PLAN}"
  echo ""
  echo "== Cleanup instructions =="
  echo "- Test order IDs logged: ${ORDER_LOG}"
  echo "- Operator: ${TEST_OPERATOR_NAME}"
  echo "- Canary machine: ${TEST_MACHINE_ID}"
  echo "- SIMULATE_HARDWARE_VEND=${SIMULATE_HARDWARE_VEND}"
  echo "- BACKEND_ONLY_DRY_RUN=${BACKEND_ONLY_DRY_RUN}"
  echo "- Review order ${ORDER_ID} in admin commerce UI; refund/cancel if this was an unintended canary sale."
  echo "- Verify inventory on canary machine ${TEST_MACHINE_ID} slot ${TEST_SLOT_CODE}."
  echo "- Archive evidence: ${E2E_RUN_DIR}"
  echo "- Fleet GO requires higher-level matrix (XY + TCN + TCN-HYBRID + bill evidence); this script never emits fleet GO."
} | tee "${E2E_RUN_DIR}/CLEANUP.txt"

write_canary_manifest

e2e_compute_canary_live_sale_verdicts "$FAILURES" "$BACKEND_ONLY_DRY_RUN" "$SIMULATE_HARDWARE_VEND" "$hardware_vend_completed" "$final_status" "$inventory_delta_state"
CANARY_VERDICT="${E2E_FAMILY_CANARY_VERDICT}"
READINESS_VERDICT="${E2E_READINESS_VERDICT}"
EXIT_CODE="${E2E_CANARY_EXIT_CODE}"

{
  echo ""
  echo "== Final =="
  echo "FAMILY_CANARY_VERDICT=${CANARY_VERDICT}"
  echo "READINESS_VERDICT=${READINESS_VERDICT}"
  echo "SIMULATE_HARDWARE_VEND=${SIMULATE_HARDWARE_VEND}"
  echo "BACKEND_ONLY_DRY_RUN=${BACKEND_ONLY_DRY_RUN}"
} | tee "${E2E_RUN_DIR}/READINESS.txt"

e2e_finalize_report "production-canary-live-sale" "$CANARY_VERDICT" "$EXIT_CODE" "$READINESS_VERDICT" ""
exit "$EXIT_CODE"
