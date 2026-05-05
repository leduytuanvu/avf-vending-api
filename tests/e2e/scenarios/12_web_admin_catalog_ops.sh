#!/usr/bin/env bash
# shellcheck shell=bash
# WA-CAT-12: catalog CRUD-ish reads, safe metadata patch, taxonomies, price book row, sale-catalog check.

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

# PATCH with idempotency (OpenAPI requires Idempotency-Key on product patch).
e2e_http_patch_json_idem() { _e2e_http_json_mutate PATCH "$1" "$2" "$3" "yes" "$4"; }

FLOW_ID="WA-CAT-12"
MODULE="catalog_pricing"
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
  log_error "WA-CAT-12: ADMIN_TOKEN required"
  exit 2
fi

ORG_ID="$(get_data organizationId)"
MACHINE_ID="$(get_data machineId)"
PRODUCT_ID="$(get_data productId)"

if [[ -z "$ORG_ID" || "$ORG_ID" == "null" ]]; then
  log_error "WA-CAT-12: organizationId required"
  exit 2
fi
Q_ORG="organization_id=$(printf '%s' "$ORG_ID" | jq -sRr @uri)"

ANY_FAIL=0

# --- List products ---
path="/v1/admin/products?${Q_ORG}&limit=50"
code="$(e2e_http_get "cat-products-list" "$path")"
found=""
if [[ "$code" == "200" ]]; then
  if [[ -n "$PRODUCT_ID" && "$PRODUCT_ID" != "null" ]]; then
    found="$(jq -r --arg p "$PRODUCT_ID" '[.items[]? | .id] | index($p) | if . == null then "no" else "yes" end' "${E2E_RUN_DIR}/rest/cat-products-list.response.json" 2>/dev/null)"
  fi
  wa4_record "products-list" "GET /v1/admin/products" "pass" "$code" "200 + items" "product_in_page=${found:-na}" "" "cat-products-list"
elif [[ "$code" == "401" ]] || [[ "$code" == "403" ]]; then
  wa4_record "products-list" "GET /v1/admin/products" "skip" "$code" "200" "HTTP $code" "Catalog role" "cat-products-list"
elif [[ "$code" =~ ^5 ]] || [[ "$code" == "0" ]]; then
  ANY_FAIL=1
  wa4_record "products-list" "GET /v1/admin/products" "fail" "$code" "not 5xx" "HTTP $code" "rest/cat-products-list.response.json" "cat-products-list"
else
  wa4_record "products-list" "GET /v1/admin/products" "skip" "$code" "200" "HTTP $code" "rest/cat-products-list.response.json" "cat-products-list"
fi

# --- Get product ---
if [[ -n "$PRODUCT_ID" && "$PRODUCT_ID" != "null" ]]; then
  path="/v1/admin/products/${PRODUCT_ID}?${Q_ORG}"
  code="$(e2e_http_get "cat-product-get" "$path")"
  if [[ "$code" == "200" ]]; then
    wa4_record "product-get" "GET /v1/admin/products/{id}" "pass" "$code" "200" "ok" "" "cat-product-get"
  else
    ANY_FAIL=1
    wa4_record "product-get" "GET /v1/admin/products/{id}" "fail" "$code" "200" "HTTP $code" "cat-product-get.response.json" "cat-product-get"
  fi

  # --- Safe metadata patch (attrs.e2eAutomation) ---
  if [[ "${E2E_ALLOW_WRITES:-}" == "true" ]] && [[ "$code" == "200" ]]; then
    attrs_json="$(e2e_jq_resp cat-product-get -c '.attrs // {}')" || attrs_json="{}"
    [[ -z "$attrs_json" ]] && attrs_json='{}'
    meta_ts="$(date +%s)"
    PPATCH="$(jq -nc --argjson a "$attrs_json" --arg ts "$meta_ts" '{attrs: ($a + {e2eAutomation: ("phase4-"+$ts)})}')"
    path="/v1/admin/products/${PRODUCT_ID}?${Q_ORG}"
    pcode="$(e2e_http_patch_json_idem "cat-product-patch" "$path" "$PPATCH" "e2e-cat-p-${meta_ts}")"
    if [[ "$pcode" == "200" ]]; then
      wa4_record "product-patch-metadata" "PATCH /v1/admin/products/{id}" "pass" "$pcode" "200" "attrs merged" "" "cat-product-patch"
    else
      ANY_FAIL=1
      wa4_record "product-patch-metadata" "PATCH .../products/{id}" "fail" "$pcode" "200" "HTTP $pcode" "Idempotency / attrs schema; rest/cat-product-patch.response.json" "cat-product-patch"
    fi
  else
    wa4_record "product-patch-metadata" "PATCH .../products/{id}" "skip" "0" "writes + GET 200" "skipped" "E2E_ALLOW_WRITES=true" ""
  fi
else
  wa4_record "product-get" "—" "skip" "0" "productId" "missing" "Phase 3 setup" ""
fi

# --- Categories / tags / brands ---
for tuple in \
  "categories|/v1/admin/categories?${Q_ORG}&limit=20|cat-categories" \
  "tags|/v1/admin/tags?${Q_ORG}&limit=20|cat-tags" \
  "brands|/v1/admin/brands?${Q_ORG}&limit=20|cat-brands"; do
  IFS='|' read -r label pfx stepn <<<"$tuple"
  code="$(e2e_http_get "$stepn" "$pfx")"
  if [[ "$code" == "200" ]]; then
    wa4_record "list-${label}" "GET ${pfx%%\?*}" "pass" "$code" "200" "ok" "" "$stepn"
  elif [[ "$code" == "404" ]]; then
    wa4_record "list-${label}" "GET" "skip" "404" "200" "not mounted" "" "$stepn"
  else
    wa4_record "list-${label}" "GET" "skip" "$code" "200" "HTTP $code" "optional ${label}" "$stepn"
  fi
done

# --- Price book: list + patch item + verify ---
PB_ID=""
path="/v1/admin/price-books?${Q_ORG}&limit=20"
code="$(e2e_http_get "cat-pricebooks-list" "$path")"
if [[ "$code" == "200" ]]; then
  PB_ID="$(jq -r '(.items // [])[] | select(.isDefault==true) | .id // empty' "${E2E_RUN_DIR}/rest/cat-pricebooks-list.response.json" | head -n1)"
  [[ -z "$PB_ID" ]] && PB_ID="$(jq -r '(.items // [])[0].id // empty' "${E2E_RUN_DIR}/rest/cat-pricebooks-list.response.json")"
  wa4_record "price-books-list" "GET /v1/admin/price-books" "pass" "$code" "200" "priceBookId=${PB_ID:-none}" "" "cat-pricebooks-list"
else
  wa4_record "price-books-list" "GET /v1/admin/price-books" "skip" "$code" "200" "HTTP $code" "commerce pricing optional" "cat-pricebooks-list"
fi

if [[ -n "$PB_ID" && -n "$PRODUCT_ID" && "$PRODUCT_ID" != "null" ]] && [[ "${E2E_ALLOW_WRITES:-}" == "true" ]]; then
  path="/v1/admin/price-books/${PB_ID}/items?${Q_ORG}"
  e2e_http_get "cat-pb-items" "$path" >/dev/null
  curp="$(jq -r --arg pr "$PRODUCT_ID" '(.items // [])[] | select(.productId==$pr) | .unitPriceMinor // empty' "${E2E_RUN_DIR}/rest/cat-pb-items.response.json" | head -n1)"
  [[ -z "$curp" ]] && curp="1000"
  newp=$((curp + 1))
  PP="$(jq -nc --argjson up "$newp" '{unitPriceMinor:$up}')"
  path="/v1/admin/price-books/${PB_ID}/items/${PRODUCT_ID}?${Q_ORG}"
  pcode="$(e2e_http_patch_json_idem "cat-pb-patch" "$path" "$PP" "e2e-pb-${PRODUCT_ID}-$(date +%s)")"
  if [[ "$pcode" == "200" ]]; then
    e2e_http_get "cat-pb-items2" "/v1/admin/price-books/${PB_ID}/items?${Q_ORG}" >/dev/null
    v="$(jq -r --arg pr "$PRODUCT_ID" '(.items // [])[] | select(.productId==$pr) | .unitPriceMinor // empty' "${E2E_RUN_DIR}/rest/cat-pb-items2.response.json" | head -n1)"
    if [[ "$v" == "$newp" ]]; then
      wa4_record "price-book-item-patch" "PATCH .../items/{productId}" "pass" "$pcode" "unitPriceMinor=$newp" "verified" "" "cat-pb-patch"
    else
      ANY_FAIL=1
      wa4_record "price-book-item-patch" "PATCH .../items/{productId}" "fail" "$pcode" "price=$newp" "listed=$v" "refresh items list" "cat-pb-patch"
    fi
  else
    wa4_record "price-book-item-patch" "PATCH .../items/{productId}" "skip" "$pcode" "200" "HTTP $pcode" "Product may need to be in book via PUT items" "cat-pb-patch"
  fi
else
  wa4_record "price-book-item-patch" "—" "skip" "0" "priceBook + product + writes" "skipped" "" ""
fi

# --- Sale catalog (kiosk) ---
if [[ -n "$MACHINE_ID" && "$MACHINE_ID" != "null" ]]; then
  path="/v1/machines/${MACHINE_ID}/sale-catalog?include_unavailable=true&include_images=false"
  code="$(e2e_http_get "cat-sale-catalog" "$path")"
  if [[ "$code" == "200" && -n "$PRODUCT_ID" && "$PRODUCT_ID" != "null" ]]; then
    hit="$(jq -r --arg p "$PRODUCT_ID" '[.items[]? | select(.productId==$p)] | length' "${E2E_RUN_DIR}/rest/cat-sale-catalog.response.json")"
    if [[ "${hit:-0}" -ge 1 ]]; then
      wa4_record "sale-catalog-contains-product" "GET .../sale-catalog" "pass" "$code" "items contain productId" "count=$hit" "" "cat-sale-catalog"
    else
      wa4_record "sale-catalog-contains-product" "GET .../sale-catalog" "skip" "$code" "product in catalog" "not listed (unavailable?)" "Check publish + price + stock" "cat-sale-catalog"
    fi
  elif [[ "$code" == "200" ]]; then
    wa4_record "sale-catalog-contains-product" "GET .../sale-catalog" "pass" "$code" "200" "ok (no productId to match)" "" "cat-sale-catalog"
  else
    wa4_record "sale-catalog-contains-product" "GET .../sale-catalog" "skip" "$code" "200" "HTTP $code" "Bearer / machine scope" "cat-sale-catalog"
  fi
else
  wa4_record "sale-catalog-contains-product" "—" "skip" "0" "machineId" "missing" "" ""
fi

log_flow_design_issue "P2" "$FLOW_ID" "12_web_admin_catalog_ops.sh" "sale-catalog-determinism" "REST" "GET /v1/machines/{id}/sale-catalog" "Product visibility on sale-catalog depends on publish/price/stock — harness only checks containment, not full vending-app field set" "Kiosk gaps for price/media/qty" "Document required sale-catalog fields; fail CI when missing" "${E2E_RUN_DIR}/rest/cat-sale-catalog.response.json"
log_cleanup_issue "P3" "$FLOW_ID" "12_web_admin_catalog_ops.sh" "catalog-cleanup" "REST" "products/brands" "Test catalog entities may accumulate SKUs without documented bulk purge" "Org clutter" "Add archive SKUs and doc operator cleanup" "${E2E_RUN_DIR}/reports/wa-module-results.jsonl"
if [[ "$ANY_FAIL" -eq 0 ]]; then
  e2e_flow_review_scenario_complete "$FLOW_ID" "12_web_admin_catalog_ops.sh" "flow-review-complete" "wa_catalog_ops_reviewed"
fi

exit "$ANY_FAIL"
