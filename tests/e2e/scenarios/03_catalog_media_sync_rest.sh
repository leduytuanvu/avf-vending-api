#!/usr/bin/env bash
# shellcheck shell=bash
# VM-REST-03: Sale catalog + optional media URL checks (machine JWT). Field protocol: gRPC catalog deltas + MQTT.

set +e
set -u

E2E_SCENARIO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=../lib/e2e_common.sh
source "${E2E_SCENARIO_DIR}/../lib/e2e_common.sh"
# shellcheck source=../lib/e2e_http.sh
source "${E2E_SCENARIO_DIR}/../lib/e2e_http.sh"
# shellcheck source=../lib/e2e_data.sh
source "${E2E_SCENARIO_DIR}/../lib/e2e_data.sh"

FLOW_ID="VM-REST-03"
SCENARIO_ID="$(basename "${BASH_SOURCE[0]}")"
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

MID="$(get_data machineId)"
PRODUCT_ID="$(get_data productId)"
SLOT="$(get_data slotCode)"
[[ -z "$SLOT" || "$SLOT" == "null" ]] && SLOT="A1"

MT="$(get_secret machineToken 2>/dev/null || true)"
if [[ -z "$MT" ]]; then
  log_error "VM-REST-03: machineToken missing — run 02 or reuse secrets.private.json"
  exit 2
fi
export ADMIN_TOKEN="$MT"

path="/v1/machines/${MID}/sale-catalog?include_unavailable=true&include_images=true"
code="$(e2e_http_get "vm-sale-catalog" "$path")"
if [[ "$code" != "200" ]]; then
  va_record "sale-catalog" "GET /v1/machines/{id}/sale-catalog" "fail" "$code" "HTTP $code"
  exit 1
fi

hit=""
IMPROV_CATALOG=0
[[ -n "$PRODUCT_ID" && "$PRODUCT_ID" != "null" ]] && hit="$(jq -r --arg p "$PRODUCT_ID" --arg sc "$SLOT" '
  [.items[]? | select(.productId==$p and .slotCode==$sc)] | length
' "${E2E_RUN_DIR}/rest/vm-sale-catalog.response.json")"

img_chk="$(jq -r '[.items[]? | select(.image.displayUrl? // .image.thumbUrl?)] | length' "${E2E_RUN_DIR}/rest/vm-sale-catalog.response.json")"

if [[ -n "$PRODUCT_ID" && "$PRODUCT_ID" != "null" ]] && [[ "${hit:-0}" -ge 1 ]]; then
  va_record "sale-catalog-a1-product" "GET .../sale-catalog" "pass" "$code" "slot ${SLOT} has productId ${PRODUCT_ID}"
else
  va_record "sale-catalog-a1-product" "GET .../sale-catalog" "skip" "$code" "expected product ${PRODUCT_ID:-?} on $SLOT — check planogram/publish"
  log_response_shape_issue "P2" "$FLOW_ID" "$SCENARIO_ID" "sale-catalog-slot" "REST" "GET /v1/machines/{id}/sale-catalog" "Expected product not found on configured slot — deterministic slot assignment not reflected in sale-catalog" "Vending UI/API may miss price/stock for app" "Ensure publish projects slot bindings; stable slot_code keys in response" "${E2E_RUN_DIR}/rest/vm-sale-catalog.response.json"
  IMPROV_CATALOG=1
fi

if [[ "${img_chk:-0}" -gt 0 ]]; then
  va_record "sale-catalog-media-urls" "GET .../sale-catalog" "pass" "$code" "image URLs present on ${img_chk} row(s) (no download performed)"
else
  va_record "sale-catalog-media-urls" "GET .../sale-catalog" "skip" "$code" "no image.displayUrl in response (optional)"
  log_missing_field_issue "P2" "$FLOW_ID" "$SCENARIO_ID" "sale-catalog-media" "REST" "GET .../sale-catalog?include_images=true" "Sale-catalog rows lack image URL (and checksum/fingerprint not asserted in this response)" "Kiosk cannot show product media" "Return thumb/display URL + checksum or media manifest link per OpenAPI" "${E2E_RUN_DIR}/rest/vm-sale-catalog.response.json"
  IMPROV_CATALOG=1
fi

if [[ "${IMPROV_CATALOG:-0}" -eq 1 ]]; then
  log_unnecessary_complexity_issue "P2" "$FLOW_ID" "$SCENARIO_ID" "catalog-call-depth" "REST" "sale-catalog" "Machine sale screen may require multiple hops (bootstrap + catalog + media) vs a single aggregated vending snapshot" "Harder automation and app cold-start" "Offer composite snapshot RPC or REST with embedded media metadata" "${E2E_RUN_DIR}/rest/vm-sale-catalog.meta.json"
else
  log_no_improvement_findings "$FLOW_ID" "$SCENARIO_ID" "flow-review-complete"
fi

e2e_flow_review_scenario_complete "$FLOW_ID" "$SCENARIO_ID" "flow-review-complete" "catalog_media_sync_reviewed"

exit 0
