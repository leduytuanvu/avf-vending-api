#!/usr/bin/env bash
# Attach catalog images + price-book prices from layout JSON catalog_products section.
set -Eeuo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT"

LAYOUT_JSON="${1:-}"
if [[ -z "$LAYOUT_JSON" || ! -f "$LAYOUT_JSON" ]]; then
  echo "usage: attach_layout_catalog_products.sh <layout.json>" >&2
  exit 2
fi
LAYOUT_JSON="$(cd "$(dirname "$LAYOUT_JSON")" && pwd)/$(basename "$LAYOUT_JSON")"

if [[ "${ATTACH_CATALOG_SKIP_WRITE_GUARD:-false}" != "true" ]]; then
  if [[ "${E2E_ALLOW_WRITES:-false}" != "true" ]]; then
    echo "FATAL: E2E_ALLOW_WRITES=true required" >&2
    exit 2
  fi
fi

if ! jq -e '(.catalog_products // []) | length > 0' "$LAYOUT_JSON" >/dev/null 2>&1; then
  echo "SKIP: layout has no catalog_products"
  exit 0
fi

E2E_SCRIPT_DIR="${ROOT}/tests/e2e"
# shellcheck source=../../tests/e2e/lib/e2e_common.sh
source "${E2E_SCRIPT_DIR}/lib/e2e_common.sh"
# shellcheck source=../../tests/e2e/lib/e2e_production_destructive_aliases.sh
source "${E2E_SCRIPT_DIR}/lib/e2e_production_destructive_aliases.sh"
# shellcheck source=lib/common.sh
source "${ROOT}/scripts/e2e/lib/common.sh"

e2e_strict_mode
e2e_require_cmd curl jq

export E2E_RUN_TS="${E2E_RUN_TS:-$(date -u +%Y%m%dT%H%M%SZ)}"
export E2E_RUN_DIR="${E2E_RUN_DIR:-${ROOT}/reports/e2e/attach-layout-catalog/${E2E_RUN_TS}}"
mkdir -p "${E2E_RUN_DIR}/raw"

FAILURES=()
fail_step() {
  FAILURES+=("$1")
  echo "FAIL: $1" >&2
}

if [[ -z "${ADMIN_TOK:-}" ]]; then
  load_env "${E2E_ENV_FILE:-${ROOT}/tests/e2e/.env.production.destructive.local}"
  CANARY_ENV="${ROOT}/tests/e2e/production/.env.production.e2e.local"
  if [[ -f "$CANARY_ENV" ]]; then
    _env_tmp="$(mktemp)"
    tr -d '\r' <"$CANARY_ENV" >"$_env_tmp"
    set -a
    # shellcheck disable=SC1090
    source "$_env_tmp"
    set +a
    rm -f "$_env_tmp"
    : "${BASE_URL:=${E2E_PROD_BASE_URL:-${BASE_URL:-}}}"
    : "${ADMIN_EMAIL:=${E2E_PROD_ADMIN_EMAIL:-${ADMIN_EMAIL:-}}}"
    : "${ADMIN_PASSWORD:=${E2E_PROD_ADMIN_PASSWORD:-${ADMIN_PASSWORD:-}}}"
    : "${ADMIN_TOKEN:=${E2E_PROD_ADMIN_TOKEN:-${ADMIN_TOKEN:-}}}"
  fi
  e2e_apply_production_destructive_aliases
  BASE_URL="${BASE_URL:-https://api.ldtv.dev}"
  BASE_URL="${BASE_URL%/}"
  export BASE_URL
  if ! ADMIN_TOK="$(e2e_admin_token)"; then
    fail_step "admin auth failed"
    exit 2
  fi
fi

BASE_URL="${BASE_URL:-https://api.ldtv.dev}"
BASE_URL="${BASE_URL%/}"

e2e_curl_get() {
  local name="$1" url="$2"
  local meta="${E2E_RUN_DIR}/raw/${name}.meta"
  local code="000"
  code="$(curl -sS -o "${E2E_RUN_DIR}/raw/${name}.body" -w '%{http_code}' \
    -H "Accept: application/json" \
    -H "Authorization: Bearer ${ADMIN_TOK}" \
    --connect-timeout 8 --max-time 30 \
    "$url" 2>/dev/null)" || code="000"
  printf '%s' "$code" >"$meta"
  echo "$code"
}

e2e_curl_json() {
  local method="$1" name="$2" url="$3" body="$4" idem="${5:-}"
  local out="${E2E_RUN_DIR}/raw/${name}.body"
  local meta="${E2E_RUN_DIR}/raw/${name}.meta"
  local code="000"
  printf '%s' "$body" >"${E2E_RUN_DIR}/raw/${name}.request.json"
  local -a hdr=( -H "Content-Type: application/json" -H "Accept: application/json" -H "Authorization: Bearer ${ADMIN_TOK}" )
  [[ -n "$idem" ]] && hdr+=( -H "Idempotency-Key: ${idem}" )
  code="$(curl -sS -o "$out" -w '%{http_code}' -X "$method" "${hdr[@]}" --connect-timeout 8 --max-time 60 -d "$body" "$url" 2>/dev/null)" || code="000"
  printf '%s' "$code" >"$meta"
  echo "$code"
}

code="$(e2e_curl_get "products-list" "${BASE_URL}/v1/admin/products?limit=500")"
[[ "$code" == "200" ]] || {
  fail_step "products list http=${code}"
  exit 2
}
PRODUCTS_BODY="${E2E_RUN_DIR}/raw/products-list.body"

PB_ID=""
code="$(e2e_curl_get "price-books-list" "${BASE_URL}/v1/admin/price-books?limit=50")"
if [[ "$code" == "200" ]]; then
  PB_ID="$(jq -r '(.items // [])[] | select(.isDefault==true) | .id' "${E2E_RUN_DIR}/raw/price-books-list.body" 2>/dev/null | head -n1)"
  if [[ -z "$PB_ID" ]]; then
    PB_ID="$(jq -r '(.items // [])[] | select(.active==true) | .id' "${E2E_RUN_DIR}/raw/price-books-list.body" 2>/dev/null | head -n1)"
  fi
fi
if [[ -z "$PB_ID" ]]; then
  eff_from="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  pb_body="$(jq -nc --arg eff "$eff_from" '{name:"AVF Layout Default",currency:"VND",effectiveFrom:$eff,isDefault:true,priceBookLevel:"global",priority:0}')"
  code="$(e2e_curl_json POST price-book-create "${BASE_URL}/v1/admin/price-books" "$pb_body" "e2e-layout-pb-${E2E_RUN_TS}")"
  if [[ "$code" == "200" ]]; then
    PB_ID="$(jq -r '.id // empty' "${E2E_RUN_DIR}/raw/price-book-create.body")"
  else
    fail_step "price book create http=${code}"
  fi
fi
[[ -n "$PB_ID" ]] || {
  fail_step "price book id missing"
  exit 2
}

IMG_OK=0
IMG_FAIL=0
PRICE_OK=0
PRICE_FAIL=0

while IFS= read -r row; do
  [[ -n "$row" ]] || continue
  sku="$(echo "$row" | jq -r '.sku // empty')"
  img_url="$(echo "$row" | jq -r '.primary_image_url // empty')"
  price_minor="$(echo "$row" | jq -r '.unit_price_minor // 0')"
  [[ -n "$sku" ]] || continue

  pid="$(jq -r --arg sku "$sku" '(.items // [])[] | select(.sku==$sku) | .id' "$PRODUCTS_BODY" 2>/dev/null | head -n1)"
  if [[ -z "$pid" ]]; then
    fail_step "product sku=${sku} not found in catalog"
    continue
  fi

  if [[ -n "$img_url" ]]; then
    patch_body="$(jq -nc --arg url "$img_url" '{primaryImageUrl:$url}')"
    safe_sku="$(echo "$sku" | tr -c 'A-Za-z0-9._-' '_')"
    pcode="$(e2e_curl_json PATCH "prod-img-${safe_sku}" "${BASE_URL}/v1/admin/products/${pid}" "$patch_body" "e2e-layout-img-${sku}")"
    if [[ "$pcode" == "200" ]]; then
      IMG_OK=$((IMG_OK + 1))
    else
      IMG_FAIL=$((IMG_FAIL + 1))
      fail_step "product ${sku} image patch http=${pcode}"
    fi
  fi

  if [[ "$price_minor" =~ ^[0-9]+$ ]] && [[ "$price_minor" -gt 0 ]]; then
    price_body="$(jq -nc --argjson minor "$price_minor" '{unitPriceMinor:$minor}')"
    safe_sku="$(echo "$sku" | tr -c 'A-Za-z0-9._-' '_')"
    pcode="$(e2e_curl_json PATCH "prod-price-${safe_sku}" "${BASE_URL}/v1/admin/price-books/${PB_ID}/items/${pid}" "$price_body" "e2e-layout-price-${sku}")"
    if [[ "$pcode" == "200" ]]; then
      PRICE_OK=$((PRICE_OK + 1))
    else
      PRICE_FAIL=$((PRICE_FAIL + 1))
      fail_step "product ${sku} price patch http=${pcode}"
    fi
  fi
done < <(jq -c '.catalog_products[]?' "$LAYOUT_JSON")

{
  echo "priceBookId=${PB_ID}"
  echo "images_ok=${IMG_OK}"
  echo "images_failed=${IMG_FAIL}"
  echo "prices_ok=${PRICE_OK}"
  echo "prices_failed=${PRICE_FAIL}"
} >"${E2E_RUN_DIR}/SUMMARY.txt"

if [[ ${#FAILURES[@]} -gt 0 ]]; then
  exit 2
fi

echo "PASS images=${IMG_OK} prices=${PRICE_OK} priceBookId=${PB_ID}"
exit 0
