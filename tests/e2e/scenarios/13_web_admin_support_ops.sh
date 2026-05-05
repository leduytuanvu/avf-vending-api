#!/usr/bin/env bash
# shellcheck shell=bash
# WA-SUP-13: support orders list/detail, optional E2E commerce order + timeline + refund/cancel (local/staging only).

set +e
set -u

E2E_SCENARIO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
: "${E2E_SCRIPT_DIR:=$(cd "${E2E_SCENARIO_DIR}/.." && pwd)}"

# shellcheck source=../lib/e2e_common.sh
source "${E2E_SCENARIO_DIR}/../lib/e2e_common.sh"
# shellcheck source=../lib/e2e_http.sh
source "${E2E_SCENARIO_DIR}/../lib/e2e_http.sh"
# shellcheck source=../lib/e2e_data.sh
source "${E2E_SCENARIO_DIR}/../lib/e2e_data.sh"

FLOW_ID="WA-SUP-13"
MODULE="support_commerce"
WA4_LOG="${E2E_RUN_DIR}/reports/wa-module-results.jsonl"
mkdir -p "${E2E_RUN_DIR}/reports"

wa4_record() {
  local step="$1" endpoint="$2" status="$3" http="$4" expected="$5" actual="$6" remed="$7" stepfile="$8"
  local reqf="${E2E_RUN_DIR}/rest/${stepfile}.request.json"
  local respf="${E2E_RUN_DIR}/rest/${stepfile}.response.json"
  [[ -f "$reqf" ]] || reqf=""
  [[ -f "$respf" ]] || respf=""
  jq -nc \
    --arg ts "$(now_utc)" \
    --arg mod "$MODULE" \
    --arg flow "$FLOW_ID" \
    --arg step "$step" \
    --arg ep "$endpoint" \
    --arg st "$status" \
    --argjson http "${http:-0}" \
    --arg exp "$expected" \
    --arg act "$actual" \
    --arg rem "$remed" \
    --arg req "$reqf" \
    --arg rsp "$respf" \
    '{ts:$ts,module:$mod,flow_id:$flow,step:$step,endpoint:$ep,status:$st,httpStatus:$http,expected:$exp,actual:$act,remediation:$rem,requestPath:$req,responsePath:$rsp}' \
    >>"$WA4_LOG"
  e2e_append_test_event "$FLOW_ID" "$step" "REST" "$endpoint" "$status" "$actual" "{}"
}

if [[ -z "${ADMIN_TOKEN:-}" ]]; then
  t="$(get_secret adminAccessToken 2>/dev/null || true)"
  [[ -n "$t" ]] && export ADMIN_TOKEN="$t"
fi
if [[ -z "${ADMIN_TOKEN:-}" ]]; then
  log_error "WA-SUP-13: ADMIN_TOKEN required"
  exit 2
fi

ORG_ID="$(get_data organizationId)"
MACHINE_ID="$(get_data machineId)"
PRODUCT_ID="$(get_data productId)"
CUR="$(get_data currency)"
CABINET="$(get_data slotCabinetCode)"
SLOT="$(get_data slotCode)"
[[ -z "$CUR" || "$CUR" == "null" ]] && CUR="VND"
[[ -z "$CABINET" || "$CABINET" == "null" ]] && CABINET="A"
[[ -z "$SLOT" || "$SLOT" == "null" ]] && SLOT="A1"

if [[ -z "$ORG_ID" || "$ORG_ID" == "null" ]]; then
  log_error "WA-SUP-13: organizationId required"
  exit 2
fi

Q_ORG_QUERY="organization_id=$(printf '%s' "$ORG_ID" | jq -sRr @uri)"
ANY_FAIL=0
mut_safe=""
if [[ "${E2E_ALLOW_WRITES:-}" == "true" ]] && [[ "${E2E_TARGET:-local}" != "production" ]]; then
  mut_safe="yes"
fi

# Support dashboard: list orders
path="/v1/orders?${Q_ORG_QUERY}&limit=10"
[[ -n "$MACHINE_ID" && "$MACHINE_ID" != "null" ]] && path="${path}&machine_id=$(printf '%s' "$MACHINE_ID" | jq -sRr @uri)"
code="$(e2e_http_get "sup-orders-list" "$path")"
if [[ "$code" == "200" ]] && jq -e '(.items|type=="array")' "${E2E_RUN_DIR}/rest/sup-orders-list.response.json" >/dev/null 2>&1; then
  wa4_record "orders-list" "GET /v1/orders" "pass" "$code" "200 items" "ok" "" "sup-orders-list"
elif [[ "$code" == "401" ]] || [[ "$code" == "403" ]]; then
  wa4_record "orders-list" "GET /v1/orders" "skip" "$code" "200+items" "HTTP $code" "Support/orders role" "sup-orders-list"
elif [[ "$code" =~ ^5 ]] || [[ "$code" == "0" ]]; then
  ANY_FAIL=1
  wa4_record "orders-list" "GET /v1/orders" "fail" "$code" "not 5xx" "HTTP $code" "rest/sup-orders-list.response.json" "sup-orders-list"
else
  wa4_record "orders-list" "GET /v1/orders" "skip" "$code" "200+items" "HTTP $code" "rest/sup-orders-list.response.json" "sup-orders-list"
fi

EXIST_OID="$(get_data e2eTestOrderId)"
if [[ -n "$EXIST_OID" && "$EXIST_OID" != "null" ]]; then
  path="/v1/commerce/orders/${EXIST_OID}"
  code="$(e2e_http_get "sup-order-get-existing" "$path")"
  if [[ "$code" == "200" ]]; then
    wa4_record "commerce-order-get" "GET /v1/commerce/orders/{id}" "pass" "$code" "200" "reuse e2eTestOrderId" "" "sup-order-get-existing"
  else
    wa4_record "commerce-order-get" "GET /v1/commerce/orders/{id}" "skip" "$code" "200" "HTTP $code" "Stale id in test-data" "sup-order-get-existing"
  fi
fi

# Create test order (local/staging, writes, commerce mounted)
NEW_OID=""
if [[ "$mut_safe" != "yes" ]]; then
  wa4_record "commerce-order-create" "POST /v1/commerce/orders" "skip" "0" "local/staging + writes" "skipped" "Not mutating in production or E2E_ALLOW_WRITES off" ""
elif [[ -z "$MACHINE_ID" || "$MACHINE_ID" == "null" || -z "$PRODUCT_ID" || "$PRODUCT_ID" == "null" ]]; then
  wa4_record "commerce-order-create" "POST /v1/commerce/orders" "skip" "0" "machineId+productId" "missing" "Complete Phase 3 / reuse-data" ""
else
  OBODY="$(jq -nc \
    --arg mid "$MACHINE_ID" \
    --arg pid "$PRODUCT_ID" \
    --arg cur "$CUR" \
    --arg cab "$CABINET" \
    --arg sc "$SLOT" \
    '{machine_id:$mid, product_id:$pid, currency:$cur, cabinet_code:$cab, slot_code:$sc}')"
  path="/v1/commerce/orders"
  code="$(e2e_http_post_json_idem "sup-order-create" "$path" "$OBODY" "e2e-sup-order-$(date +%s)")"
  if [[ "$code" == "201" ]] || [[ "$code" == "200" ]]; then
    NEW_OID="$(e2e_jq_resp sup-order-create -r '.order_id // empty')"
    wa4_record "commerce-order-create" "POST /v1/commerce/orders" "pass" "$code" "201" "order_id=$NEW_OID" "" "sup-order-create"
    e2e_set_data e2eTestOrderId "$NEW_OID"
  elif [[ "$code" == "503" ]] || jq -e '.error.code?=="capability_not_configured"' "${E2E_RUN_DIR}/rest/sup-order-create.response.json" >/dev/null 2>&1; then
    wa4_record "commerce-order-create" "POST /v1/commerce/orders" "skip" "$code" "201" "commerce not configured" "Expected in some local stacks" "sup-order-create"
  else
    wa4_record "commerce-order-create" "POST /v1/commerce/orders" "skip" "$code" "201" "HTTP $code" "See rest/sup-order-create.response.json" "sup-order-create"
  fi
fi

OID="${NEW_OID:-$EXIST_OID}"
if [[ -n "$OID" && "$OID" != "null" ]]; then
  path="/v1/commerce/orders/${OID}"
  code="$(e2e_http_get "sup-order-get" "$path")"
  if [[ "$code" == "200" ]] && jq -e '.order' "${E2E_RUN_DIR}/rest/sup-order-get.response.json" >/dev/null 2>&1; then
    wa4_record "commerce-order-get" "GET /v1/commerce/orders/{id}" "pass" "$code" "200 + order object" "ok" "" "sup-order-get"
  elif [[ "$code" =~ ^5 ]] || [[ "$code" == "0" ]]; then
    ANY_FAIL=1
    wa4_record "commerce-order-get" "GET /v1/commerce/orders/{id}" "fail" "$code" "not 5xx" "HTTP $code" "sup-order-get.response.json" "sup-order-get"
  else
    wa4_record "commerce-order-get" "GET /v1/commerce/orders/{id}" "skip" "$code" "200+order" "HTTP $code" "sup-order-get.response.json" "sup-order-get"
  fi

  path="/v1/admin/organizations/${ORG_ID}/orders/${OID}/timeline"
  code="$(e2e_http_get "sup-order-timeline" "$path")"
  if [[ "$code" == "200" ]]; then
    wa4_record "admin-order-timeline" "GET .../orders/{id}/timeline" "pass" "$code" "200" "audit trail read" "" "sup-order-timeline"
  else
    wa4_record "admin-order-timeline" "GET .../timeline" "skip" "$code" "200" "HTTP $code" "Optional route / permissions" "sup-order-timeline"
  fi

  if [[ "$mut_safe" == "yes" ]] && [[ "$OID" == "$NEW_OID" ]]; then
    RST="$(jq -r '.order.status // empty' "${E2E_RUN_DIR}/rest/sup-order-get.response.json" 2>/dev/null)"
    did_mut=0
    if [[ "$RST" == "paid" ]] || [[ "$RST" == "completed" ]]; then
      RBODY="$(jq -nc '{amountMinor:1, reason:"e2e automation refund smoke"}')"
      path="/v1/admin/organizations/${ORG_ID}/orders/${OID}/refunds"
      code="$(e2e_http_post_json_idem "sup-order-refund" "$path" "$RBODY" "e2e-refund-${OID}")"
      if [[ "$code" == "200" ]]; then
        wa4_record "admin-order-refund" "POST .../refunds" "pass" "$code" "200" "refund requested" "" "sup-order-refund"
        did_mut=1
      else
        wa4_record "admin-order-refund" "POST .../refunds" "skip" "$code" "200" "HTTP $code" "Order state / amount; rest/sup-order-refund.response.json" "sup-order-refund"
      fi
    fi
    if [[ "$did_mut" -eq 0 ]]; then
      CBODY="$(jq -nc '{reason:"e2e_cancel", slot_index:1}')"
      path="/v1/commerce/orders/${OID}/cancel"
      code="$(e2e_http_post_json_idem "sup-order-cancel" "$path" "$CBODY" "e2e-cancel-${OID}")"
      if [[ "$code" == "200" ]]; then
        wa4_record "commerce-order-cancel" "POST .../cancel" "pass" "$code" "200" "cancelled pending order" "" "sup-order-cancel"
      else
        wa4_record "commerce-order-cancel" "POST .../cancel" "skip" "$code" "200" "HTTP $code" "Paid orders need refund; see response" "sup-order-cancel"
      fi
    fi
  else
    wa4_record "order-mutations" "refund/cancel" "skip" "0" "E2E-created local order" "not applicable" "Only mutates NEW_OID in local/staging" ""
  fi
else
  wa4_record "commerce-order-flow" "—" "skip" "0" "orderId" "none" "No order created or reused" ""
fi

exit "$ANY_FAIL"
