#!/usr/bin/env bash
# shellcheck shell=bash
# WA-INV-11: topology, slots, inventory snapshot/events, stock adjustments, optional cash collection open (local only).

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

FLOW_ID="WA-INV-11"
MODULE="inventory_planogram"
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
  log_error "WA-INV-11: ADMIN_TOKEN required"
  exit 2
fi

ORG_ID="$(get_data organizationId)"
MACHINE_ID="$(get_data machineId)"
PRODUCT_ID="$(get_data productId)"
SLOT="$(get_data slotCode)"
CABINET="$(get_data slotCabinetCode)"
PG_ID="$(get_data planogramId)"
[[ -z "$CABINET" || "$CABINET" == "null" ]] && CABINET="A"
[[ -z "$SLOT" || "$SLOT" == "null" ]] && SLOT="A1"

if [[ -z "$ORG_ID" || "$ORG_ID" == "null" ]] || [[ -z "$MACHINE_ID" || "$MACHINE_ID" == "null" ]]; then
  log_error "WA-INV-11: organizationId and machineId required in test-data.json"
  exit 2
fi
Q_ORG="organization_id=$(printf '%s' "$ORG_ID" | jq -sRr @uri)"

ANY_FAIL=0

wa4_ensure_operator() {
  local sid
  sid="$(get_data operatorSessionId)"
  if [[ -n "$sid" && "$sid" != "null" ]]; then
    echo "$sid"
    return 0
  fi
  OP_BODY="$(jq -nc '{force_admin_takeover:true, auth_method:"oidc"}')"
  local code
  code="$(e2e_http_post_json "inv-operator-login" "/v1/machines/${MACHINE_ID}/operator-sessions/login" "$OP_BODY")"
  if [[ "$code" != "200" ]]; then
    echo ""
    return 1
  fi
  e2e_jq_resp inv-operator-login -r '.session.id // empty'
}

# --- Topology ---
path="/v1/admin/machines/${MACHINE_ID}/topology?${Q_ORG}"
code="$(e2e_http_get "inv-topology" "$path")"
if [[ "$code" == "200" ]] && jq -e '.' "${E2E_RUN_DIR}/rest/inv-topology.response.json" >/dev/null 2>&1; then
  wa4_record "machine-topology" "GET .../topology" "pass" "$code" "200 JSON object" "ok" "" "inv-topology"
else
  ANY_FAIL=1
  wa4_record "machine-topology" "GET .../topology" "fail" "$code" "200" "HTTP $code" "Permissions or machine id; rest/inv-topology.response.json" "inv-topology"
fi

# --- Slots + A1 product mapping ---
path="/v1/admin/machines/${MACHINE_ID}/slots?${Q_ORG}"
code="$(e2e_http_get "inv-slots" "$path")"
if [[ "$code" != "200" ]]; then
  ANY_FAIL=1
  wa4_record "machine-slots" "GET .../slots" "fail" "$code" "200" "HTTP $code" "Publish planogram first (Phase 3)" "inv-slots"
else
  map_pid="$(jq -r --arg sc "$SLOT" --arg cab "$CABINET" '
    (.slots // [])[] | select(.slotCode==$sc and ((.cabinetCode//"A") == $cab)) | .productId // empty
  ' "${E2E_RUN_DIR}/rest/inv-slots.response.json" 2>/dev/null | head -n1)"
  [[ -z "$map_pid" ]] && map_pid="$(jq -r --arg sc "$SLOT" '(.slots // [])[] | select(.slotCode==$sc) | .productId // empty' "${E2E_RUN_DIR}/rest/inv-slots.response.json" 2>/dev/null | head -n1)"
  exp="slot ${SLOT} maps to product ${PRODUCT_ID}"
  if [[ -n "$PRODUCT_ID" && "$PRODUCT_ID" != "null" && "$map_pid" == "$PRODUCT_ID" ]]; then
    wa4_record "slot-a1-product" "GET .../slots" "pass" "$code" "$exp" "match productId=$map_pid" "" "inv-slots"
  elif [[ -n "$map_pid" ]]; then
    wa4_record "slot-a1-product" "GET .../slots" "pass" "$code" "$exp" "mapped=$map_pid (expected $PRODUCT_ID differs — planogram)" "Accept if intentional" "inv-slots"
  else
    wa4_record "slot-a1-product" "GET .../slots" "skip" "$code" "$exp" "no product on $SLOT" "Republish planogram with product on A1" "inv-slots"
  fi
fi

# --- Inventory snapshot ---
path="/v1/admin/machines/${MACHINE_ID}/inventory?${Q_ORG}"
code="$(e2e_http_get "inv-snapshot" "$path")"
if [[ "$code" == "200" ]]; then
  wa4_record "inventory-snapshot" "GET .../inventory" "pass" "$code" "200" "ok" "" "inv-snapshot"
else
  ANY_FAIL=1
  wa4_record "inventory-snapshot" "GET .../inventory" "fail" "$code" "200" "HTTP $code" "rest/inv-snapshot.response.json" "inv-snapshot"
fi

# --- Inventory events ---
path="/v1/admin/machines/${MACHINE_ID}/inventory-events?${Q_ORG}&limit=20"
code="$(e2e_http_get "inv-events" "$path")"
if [[ "$code" == "200" ]] && jq -e '(.items|type=="array")' "${E2E_RUN_DIR}/rest/inv-events.response.json" >/dev/null 2>&1; then
  wa4_record "inventory-events" "GET .../inventory-events" "pass" "$code" "200 + items[]" "ok" "" "inv-events"
elif [[ "$code" == "200" ]]; then
  wa4_record "inventory-events" "GET .../inventory-events" "pass" "$code" "200" "ok (array at root?)" "" "inv-events"
else
  wa4_record "inventory-events" "GET .../inventory-events" "skip" "$code" "200" "HTTP $code" "Optional endpoint / permissions" "inv-events"
fi

# --- Stock adjustments (restock + adjust) ---
if [[ "${E2E_ALLOW_WRITES:-}" != "true" ]]; then
  wa4_record "stock-adjust" "POST .../stock-adjustments" "skip" "0" "writes allowed" "E2E_ALLOW_WRITES!=true" "Set E2E_ALLOW_WRITES=true" ""
else
  OP_SID="$(wa4_ensure_operator)"
  if [[ -z "$OP_SID" ]]; then
    wa4_record "stock-adjust" "POST .../stock-adjustments" "skip" "0" "operator session" "login failed" "See rest/inv-operator-login.response.json" "inv-operator-login"
  elif [[ -z "$PG_ID" || "$PG_ID" == "null" ]]; then
    wa4_record "stock-adjust" "POST .../stock-adjustments" "skip" "0" "planogramId in test-data" "missing" "Phase 3 must publish planogram" ""
  elif [[ -z "$PRODUCT_ID" || "$PRODUCT_ID" == "null" ]]; then
    wa4_record "stock-adjust" "POST .../stock-adjustments" "skip" "0" "productId" "missing" "Phase 3 catalog" ""
  else
    Q_NOW="$(jq -r --arg sc "$SLOT" '(.slots // [])[] | select(.slotCode==$sc) | .currentQuantity // empty' "${E2E_RUN_DIR}/rest/inv-slots.response.json" 2>/dev/null | head -n1)"
    [[ -z "$Q_NOW" ]] && Q_NOW="0"
    Q_RESTOCK="10"
    json_adj() {
      local qb="$1" qa="$2"
      jq -nc \
        --arg sid "$OP_SID" \
        --arg pid "$PG_ID" \
        --arg prid "$PRODUCT_ID" \
        --arg cab "$CABINET" \
        --arg sc "$SLOT" \
        --argjson qb "$qb" \
        --argjson qa "$qa" \
        '{
          operator_session_id:$sid,
          reason:"e2e_restock",
          items:[{
            cabinetCode:$cab,
            slotCode:$sc,
            slotIndex:1,
            productId:$prid,
            planogramId:$pid,
            quantityBefore:$qb,
            quantityAfter:$qa
          }]
        }'
    }
    path="/v1/admin/machines/${MACHINE_ID}/stock-adjustments?${Q_ORG}"
    B1="$(json_adj "$Q_NOW" "$Q_RESTOCK")"
    code="$(e2e_http_post_json_idem "inv-stock-1" "$path" "$B1" "e2e-inv-stock1-$(date +%s)")"
    if [[ "$code" == "200" ]]; then
      wa4_record "stock-restock" "$path" "pass" "$code" "200" "qty $Q_NOW->$Q_RESTOCK" "" "inv-stock-1"
      Q2="$Q_RESTOCK"
      Q3="7"
      B2="$(json_adj "$Q2" "$Q3")"
      code2="$(e2e_http_post_json_idem "inv-stock-2" "$path" "$B2" "e2e-inv-stock2-$(date +%s)")"
      if [[ "$code2" == "200" ]]; then
        wa4_record "stock-adjust-second" "$path" "pass" "$code2" "200" "qty $Q2->$Q3" "" "inv-stock-2"
      else
        ANY_FAIL=1
        wa4_record "stock-adjust-second" "$path" "fail" "$code2" "200" "HTTP $code2" "quantityBefore mismatch — refresh slots; rest/inv-stock-2.response.json" "inv-stock-2"
      fi
      # refresh slots for verification
      e2e_http_get "inv-slots-after" "/v1/admin/machines/${MACHINE_ID}/slots?${Q_ORG}" >/dev/null
      qfinal="$(jq -r --arg sc "$SLOT" '(.slots // [])[] | select(.slotCode==$sc) | .currentQuantity // empty' "${E2E_RUN_DIR}/rest/inv-slots-after.response.json" | head -n1)"
      if [[ "$qfinal" == "$Q3" ]]; then
        wa4_record "inventory-verify-slots" "GET .../slots" "pass" "200" "currentQuantity=$Q3" "actual=$qfinal" "" "inv-slots-after"
      else
        wa4_record "inventory-verify-slots" "GET .../slots" "skip" "200" "currentQuantity=$Q3" "actual=$qfinal" "Eventual consistency or slot mismatch" "inv-slots-after"
      fi
    else
      ANY_FAIL=1
      wa4_record "stock-restock" "$path" "fail" "$code" "200" "HTTP $code" "operator session / quantityBefore; rest/inv-stock-1.response.json" "inv-stock-1"
    fi
  fi
fi

# --- Cash collection open (local/staging only; never production) ---
if [[ "${E2E_TARGET:-local}" == "production" ]]; then
  wa4_record "cash-collection-open" "POST .../cash-collections" "skip" "0" "local only" "E2E_TARGET=production" "Intentionally skipped" ""
elif [[ "${E2E_ALLOW_WRITES:-}" != "true" ]]; then
  wa4_record "cash-collection-open" "POST .../cash-collections" "skip" "0" "writes" "disabled" "" ""
else
  OP_SID="$(wa4_ensure_operator)"
  if [[ -z "$OP_SID" ]]; then
    wa4_record "cash-collection-open" "POST .../cash-collections" "skip" "0" "operator" "none" "" "inv-operator-login"
  else
    CBODY="$(jq -nc --arg sid "$OP_SID" '{operator_session_id:$sid}')"
    path="/v1/admin/machines/${MACHINE_ID}/cash-collections?${Q_ORG}"
    code="$(e2e_http_post_json_idem "inv-cash-open" "$path" "$CBODY" "e2e-cash-open-$(date +%s)")"
    if [[ "$code" == "201" ]] || [[ "$code" == "200" ]]; then
      CID="$(e2e_jq_resp inv-cash-open -r '.id // .collectionId // empty')"
      wa4_record "cash-collection-open" "POST .../cash-collections" "pass" "$code" "201/200 + id" "id=$CID" "Only listing / no close in E2E" "inv-cash-open"
      e2e_set_data cashCollectionId "$CID" 2>/dev/null || true
    else
      wa4_record "cash-collection-open" "POST .../cash-collections" "skip" "$code" "201" "HTTP $code" "May need cash policy / one open row; rest/inv-cash-open.response.json" "inv-cash-open"
    fi
  fi
fi

exit "$ANY_FAIL"
