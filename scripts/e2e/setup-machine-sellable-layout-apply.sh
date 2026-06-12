#!/usr/bin/env bash
# Apply a machine layout JSON to production (existing machine only): topology, products, planogram, stock.
set -Eeuo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT"

LAYOUT_JSON="${1:-}"
if [[ -z "$LAYOUT_JSON" || ! -f "$LAYOUT_JSON" ]]; then
  echo "usage: setup-machine-sellable-layout-apply.sh <layout.json>" >&2
  exit 2
fi
LAYOUT_JSON="$(cd "$(dirname "$LAYOUT_JSON")" && pwd)/$(basename "$LAYOUT_JSON")"

if [[ "${E2E_ALLOW_WRITES:-false}" != "true" ]]; then
  echo "FATAL: E2E_ALLOW_WRITES=true required" >&2
  exit 2
fi

E2E_SCRIPT_DIR="${ROOT}/tests/e2e"
# shellcheck source=../../tests/e2e/lib/e2e_common.sh
source "${E2E_SCRIPT_DIR}/lib/e2e_common.sh"
# shellcheck source=../../tests/e2e/lib/e2e_production_destructive_aliases.sh
source "${E2E_SCRIPT_DIR}/lib/e2e_production_destructive_aliases.sh"
# shellcheck source=lib/common.sh
source "${ROOT}/scripts/e2e/lib/common.sh"

e2e_strict_mode
e2e_require_cmd curl jq python3

export E2E_RUN_TS="${E2E_RUN_TS:-$(date -u +%Y%m%dT%H%M%SZ)}"
export E2E_RUN_DIR="${E2E_RUN_DIR:-${ROOT}/reports/e2e/setup-machine-layout/${E2E_RUN_TS}}"
mkdir -p "${E2E_RUN_DIR}/raw"

READINESS="${E2E_RUN_DIR}/SETUP_READINESS.txt"
FAILURES=()

write_readiness() {
  local verdict="$1"
  {
    echo "VERDICT=${verdict}"
    echo "UTC=$(e2e_now_utc)"
    echo "LAYOUT_JSON=${LAYOUT_JSON}"
    echo "E2E_RUN_DIR=${E2E_RUN_DIR}"
    if [[ ${#FAILURES[@]} -gt 0 ]]; then
      echo "FAILURES:"
      printf '%s\n' "${FAILURES[@]}"
    fi
  } >"$READINESS"
}

fail_step() {
  FAILURES+=("$1")
  echo "FAIL: $1" >&2
}

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

if [[ "${E2E_TARGET:-}" == "production" ]]; then
  if [[ "${E2E_PRODUCTION_WRITE_CONFIRMATION:-}" != "I_UNDERSTAND_THIS_WRITES_TO_PRODUCTION" ]]; then
    fail_step "missing E2E_PRODUCTION_WRITE_CONFIRMATION"
    write_readiness FAIL
    exit 2
  fi
fi

BASE_URL="${BASE_URL:-https://api.ldtv.dev}"
BASE_URL="${BASE_URL%/}"
export BASE_URL

if ! e2e_py "${ROOT}/scripts/e2e/layout_config_schema.py" "$LAYOUT_JSON" >/dev/null 2>"${E2E_RUN_DIR}/raw/layout-validate.err"; then
  fail_step "layout validation failed: $(tr '\n' '; ' <"${E2E_RUN_DIR}/raw/layout-validate.err")"
  write_readiness FAIL
  exit 2
fi

LAYOUT_JSON_NORM="$(mktemp)"
tr -d '\r' <"$LAYOUT_JSON" >"$LAYOUT_JSON_NORM"
LAYOUT_JSON="$LAYOUT_JSON_NORM"
MACHINE_ID="$(jq -r '.machine_id // empty' "$LAYOUT_JSON" | tr -d '\r')"
[[ -n "$MACHINE_ID" ]] || {
  fail_step "machine_id missing in layout"
  write_readiness FAIL
  exit 2
}
export TEST_MACHINE_ID="$MACHINE_ID"

ADMIN_TOK=""
if ! ADMIN_TOK="$(e2e_admin_token)"; then
  auth_detail="admin auth failed"
  if [[ -n "${E2E_ADMIN_AUTH_HTTP_CODE:-}" ]]; then
    auth_detail="${auth_detail} (login http=${E2E_ADMIN_AUTH_HTTP_CODE})"
  elif [[ -z "${ADMIN_EMAIL:-}" || -z "${ADMIN_PASSWORD:-}" ]]; then
    auth_detail="${auth_detail} (set ADMIN_TOKEN or ADMIN_EMAIL/ADMIN_PASSWORD in E2E env)"
  fi
  fail_step "$auth_detail"
  write_readiness FAIL
  exit 2
fi

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

code="$(e2e_curl_get "machine-get" "${BASE_URL}/v1/admin/machines/${MACHINE_ID}")"
if [[ "$code" != "200" ]]; then
  fail_step "machine GET http=${code} (existing machine required; no create)"
  write_readiness FAIL
  exit 2
fi

code="$(e2e_curl_json PATCH machine-active "${BASE_URL}/v1/admin/machines/${MACHINE_ID}" '{"status":"active"}' "e2e-layout-active-${MACHINE_ID}")"
[[ "$code" == "200" ]] || fail_step "machine activate http=${code}"

# --- Category (by slug) ---
CAT_SLUG="$(jq -r '.catalog_defaults.category_slug // empty' "$LAYOUT_JSON")"
CAT_ID=""
if [[ -n "$CAT_SLUG" ]]; then
  code="$(e2e_curl_get "categories-list" "${BASE_URL}/v1/admin/categories?limit=200")"
  if [[ "$code" == "200" ]]; then
    CAT_ID="$(jq -r --arg s "$CAT_SLUG" '(.items // [])[] | select(.slug==$s) | .id' "${E2E_RUN_DIR}/raw/categories-list.body" 2>/dev/null | head -n1)"
  fi
  if [[ -z "$CAT_ID" ]]; then
    cbody="$(jq -nc --arg name "E2E ${CAT_SLUG}" --arg slug "$CAT_SLUG" '{name:$name,slug:$slug,parentId:null,active:true}')"
    code="$(e2e_curl_json POST category-create "${BASE_URL}/v1/admin/categories" "$cbody" "e2e-cat-${CAT_SLUG}")"
    if [[ "$code" == "200" ]]; then
      CAT_ID="$(jq -r '.id // empty' "${E2E_RUN_DIR}/raw/category-create.body")"
    else
      fail_step "category create http=${code}"
    fi
  fi
fi

BRAND_ID=""
BR_SLUG="$(jq -r '.catalog_defaults.brand_slug // empty' "$LAYOUT_JSON")"
if [[ -n "$BR_SLUG" ]]; then
  code="$(e2e_curl_get "brands-list" "${BASE_URL}/v1/admin/brands?limit=200")"
  if [[ "$code" == "200" ]]; then
    BRAND_ID="$(jq -r --arg s "$BR_SLUG" '(.items // [])[] | select(.slug==$s) | .id' "${E2E_RUN_DIR}/raw/brands-list.body" 2>/dev/null | head -n1)"
  fi
  if [[ -z "$BRAND_ID" ]]; then
    bbody="$(jq -nc --arg name "E2E ${BR_SLUG}" --arg slug "$BR_SLUG" '{name:$name,slug:$slug,active:true}')"
    code="$(e2e_curl_json POST brand-create "${BASE_URL}/v1/admin/brands" "$bbody" "e2e-brand-${BR_SLUG}")"
    [[ "$code" == "200" ]] && BRAND_ID="$(jq -r '.id // empty' "${E2E_RUN_DIR}/raw/brand-create.body")"
  fi
fi

# --- Products by SKU (idempotent: list then POST) ---
PRODUCT_IDS_JSON="${E2E_RUN_DIR}/raw/product-ids-by-sku.json"
echo '{}' >"$PRODUCT_IDS_JSON"
code="$(e2e_curl_get "products-list" "${BASE_URL}/v1/admin/products?limit=500")"
EXISTING_PRODUCTS_BODY="${E2E_RUN_DIR}/raw/products-list.body"
while IFS= read -r sku; do
  sku="${sku//$'\r'/}"
  [[ -n "$sku" ]] || continue
  slot_json="$(jq -c --arg sku "$sku" '.slots[] | select((.product.sku // "") == $sku) | .product' "$LAYOUT_JSON" | head -n1)"
  pname="$(echo "$slot_json" | jq -r '.name // empty')"
  [[ -n "$pname" ]] || pname="E2E ${sku}"
  pid=""
  if [[ "$code" == "200" ]]; then
    pid="$(jq -r --arg sku "$sku" '(.items // [])[] | select(.sku==$sku) | .id' "$EXISTING_PRODUCTS_BODY" 2>/dev/null | head -n1)"
  fi
  if [[ -z "$pid" ]]; then
    pbody="$(jq -nc --arg name "$pname" --arg sku "$sku" \
      '{name:$name,sku:$sku,description:"e2e layout seed",active:true,ageRestricted:false,allergenCodes:[]}')"
    if [[ -n "$CAT_ID" ]]; then
      pbody="$(echo "$pbody" | jq --arg cid "$CAT_ID" '. + {categoryId:$cid}')"
    fi
    if [[ -n "$BRAND_ID" ]]; then
      pbody="$(echo "$pbody" | jq --arg bid "$BRAND_ID" '. + {brandId:$bid}')"
    fi
    safe_name="$(echo "$sku" | tr -c 'A-Za-z0-9._-' '_')"
    pcode="$(e2e_curl_json POST "product-${safe_name}" "${BASE_URL}/v1/admin/products" "$pbody" "e2e-layout-prod-${sku}")"
    if [[ "$pcode" != "200" ]]; then
      fail_step "product ${sku} http=${pcode}"
      continue
    fi
    pid="$(jq -r '.id // empty' "${E2E_RUN_DIR}/raw/product-${safe_name}.body")"
  fi
  [[ -n "$pid" ]] || fail_step "product ${sku} missing id"
  tmp="$(mktemp)"
  jq --arg sku "$sku" --arg pid "$pid" '. + {($sku):$pid}' "$PRODUCT_IDS_JSON" >"$tmp" && mv "$tmp" "$PRODUCT_IDS_JSON"
done < <(jq -r '.slots[].product.sku // empty' "$LAYOUT_JSON" | tr -d '\r' | sort -u)

# --- Operator session (admin inventory write path) ---
OP_BODY='{"force_admin_takeover":true,"auth_method":"oidc"}'
code="$(e2e_curl_json POST operator-login "${BASE_URL}/v1/admin/machines/${MACHINE_ID}/operator-sessions/start" "$OP_BODY")"
if [[ "$code" != "200" ]]; then
  fail_step "operator session http=${code}"
  write_readiness FAIL
  exit 2
fi
OP_SID="$(jq -r '.session.id // empty' "${E2E_RUN_DIR}/raw/operator-login.body")"
[[ -n "$OP_SID" ]] || {
  fail_step "operator session id missing"
  write_readiness FAIL
  exit 2
}

# --- Cabinet metadata contract (TCN cash_only — fail before topology PUT) ---
metadata_err="${E2E_RUN_DIR}/raw/layout-metadata-contract.err"
if ! e2e_py -c "
import json, sys
from pathlib import Path
sys.path.insert(0, '${ROOT}/scripts/e2e')
from layout_config_schema import validate_tcn_cash_cabinet_metadata
doc = json.loads(Path('${LAYOUT_JSON}').read_text(encoding='utf-8'))
errors = validate_tcn_cash_cabinet_metadata(doc)
if errors:
    for e in errors:
        print(e, file=sys.stderr)
    sys.exit(2)
print('OK')
" >"${E2E_RUN_DIR}/raw/layout-metadata-contract.out" 2>"$metadata_err"; then
  fail_step "cabinet metadata contract failed: $(tr '\n' '; ' <"$metadata_err" 2>/dev/null || echo unknown)"
  write_readiness FAIL
  exit 2
fi

# --- Topology ---
TOPO_JSON="$(jq -c --arg sid "$OP_SID" '{
  operator_session_id:$sid,
  cabinets:[.cabinets[] | {code:.code, title:(.title // ("Cabinet "+.code)), sortOrder:(.sort_order // 1), metadata:(.metadata // {})}],
  layouts:[.layouts[] | {cabinetCode:.cabinet_code, layoutKey:.layout_key, revision:.revision, layoutSpec:(.layout_spec // {}), status:(.status // "published")}]
}' "$LAYOUT_JSON")"
code="$(e2e_curl_json PUT machine-topology "${BASE_URL}/v1/admin/machines/${MACHINE_ID}/topology" "$TOPO_JSON")"
[[ "$code" == "204" ]] || fail_step "topology PUT http=${code}"

# --- Planogram ---
code="$(e2e_curl_get "planogram-list" "${BASE_URL}/v1/admin/planograms?limit=20")"
PG_ID=""
PG_REV=0
if [[ "$code" == "200" ]]; then
  PG_ID="$(jq -r '(.items // [])[] | select(.status=="published") | .id' "${E2E_RUN_DIR}/raw/planogram-list.body" 2>/dev/null | head -n1)"
  PG_REV="$(jq -r --arg id "$PG_ID" '(.items // [])[] | select(.id==$id) | .revision' "${E2E_RUN_DIR}/raw/planogram-list.body" 2>/dev/null | head -n1)"
fi
[[ -z "$PG_ID" ]] && PG_ID="$(jq -r '(.items // [])[0].id // empty' "${E2E_RUN_DIR}/raw/planogram-list.body" 2>/dev/null)"
[[ -z "$PG_REV" || "$PG_REV" == "null" ]] && PG_REV=0
[[ -n "$PG_ID" ]] || fail_step "no org planogram found"

# Draft items: enabled + sellable slots only
DRAFT_JSON="$(jq -c \
  --arg sid "$OP_SID" \
  --arg pid "$PG_ID" \
  --argjson prev "$PG_REV" \
  --slurpfile pmap "$PRODUCT_IDS_JSON" '
  def layout_for($cc):
    (.layouts[]? | select(.cabinet_code==$cc)) | {key:.layout_key, rev:.revision};
  {
    operator_session_id:$sid,
    planogramId:$pid,
    planogramRevision:$prev,
    syncLegacyReadModel:true,
    items:[
      (.slots // [])[]
      | select(.enabled==true and .sellable==true)
      | (.product.sku) as $sku
      | layout_for(.cabinet_code) as $lay
      | select($lay != null)
      | select($pmap[0][$sku] != null)
      | {
          cabinetCode:.cabinet_code,
          slotCode:.slot_code,
          productId:($pmap[0][$sku]),
          maxQuantity:(.inventory_quantity // 10),
          priceMinor:.price_minor,
          layoutKey:$lay.key,
          layoutRevision:$lay.rev,
          legacySlotIndex:.slot_index,
          metadata:{}
        }
    ]
  }' "$LAYOUT_JSON")"

code="$(e2e_curl_json PUT planogram-draft "${BASE_URL}/v1/admin/machines/${MACHINE_ID}/planograms/draft" "$DRAFT_JSON")"
[[ "$code" == "204" ]] || fail_step "planogram draft http=${code}"

code="$(e2e_curl_json POST planogram-publish "${BASE_URL}/v1/admin/machines/${MACHINE_ID}/planograms/publish" "$DRAFT_JSON" "e2e-layout-pub-${MACHINE_ID}-${E2E_RUN_TS}")"
[[ "$code" == "200" ]] || fail_step "planogram publish http=${code}"

# --- Stock adjustments (quantity from GET slots -> target from layout) ---
code="$(e2e_curl_get "slots-get" "${BASE_URL}/v1/admin/machines/${MACHINE_ID}/slots")"
[[ "$code" == "200" ]] || fail_step "slots GET http=${code}"

STOCK_ITEMS="$(jq -c \
  --arg pid "$PG_ID" \
  --slurpfile pmap "$PRODUCT_IDS_JSON" \
  --slurpfile live "${E2E_RUN_DIR}/raw/slots-get.body" '
  [(.slots // [])[] | select(.enabled==true and .sellable==true) | . as $cfg |
    ((($live[0] // {}).slots // [])[] | select(.slotCode==$cfg.slot_code)) as $row |
    select($pmap[0][$cfg.product.sku] != null)
    | {
      cabinetCode:$cfg.cabinet_code,
      slotCode:$cfg.slot_code,
      slotIndex:$cfg.slot_index,
      productId:($pmap[0][$cfg.product.sku]),
      planogramId:$pid,
      quantityBefore:($row.currentQuantity // 0),
      quantityAfter:($cfg.inventory_quantity // 10)
    }
  ]' "$LAYOUT_JSON")"

if [[ "$(echo "$STOCK_ITEMS" | jq 'length')" -gt 0 ]]; then
  STOCK_JSON="$(jq -nc --arg sid "$OP_SID" --argjson items "$STOCK_ITEMS" '{operator_session_id:$sid,reason:"restock",items:$items}')"
  code="$(e2e_curl_json POST stock-adjust "${BASE_URL}/v1/admin/machines/${MACHINE_ID}/stock-adjustments" "$STOCK_JSON" "e2e-layout-stock-${MACHINE_ID}-${E2E_RUN_TS}")"
  [[ "$code" == "200" ]] || fail_step "stock adjustment http=${code}"
fi

if [[ ${#FAILURES[@]} -gt 0 ]]; then
  write_readiness FAIL
  exit 2
fi

rm -f "${LAYOUT_JSON_NORM:-}" 2>/dev/null || true

write_readiness PASS
echo "SETUP_READINESS=PASS run_dir=${E2E_RUN_DIR}"
exit 0
