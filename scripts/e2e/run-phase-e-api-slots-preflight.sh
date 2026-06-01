#!/usr/bin/env bash
# API read-only preflight for TCN slots 1-10 (Phase E). No cash, dispense, or inventory mutation.
set -Eeuo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT"

ARTIFACT_SUB="${1:-api-slots-preflight}"
ARTIFACT_ROOT="${2:-}"
if [[ -n "$ARTIFACT_ROOT" ]]; then
  export E2E_RUN_DIR="${ARTIFACT_ROOT}/${ARTIFACT_SUB}"
  mkdir -p "${E2E_RUN_DIR}/raw"
fi

if [[ -z "${E2E_TEST_MACHINE_ID:-}" && -z "${TEST_MACHINE_ID:-}" ]]; then
  echo "FAIL: E2E_TEST_MACHINE_ID (or TEST_MACHINE_ID) must be set in env — no hardcoded machine UUID default." >&2
  exit 2
fi
eval "$(bash scripts/e2e/mint-production-machine-token.sh "${E2E_TEST_MACHINE_ID:-${TEST_MACHINE_ID}}")"

export E2E_ALLOW_WRITES=false
export E2E_ALLOW_REAL_DISPENSE=false
export E2E_EXPECT_REAL_DISPENSE=false
export E2E_ALLOW_REAL_PAYMENT=false
export E2E_ALLOW_REAL_MACHINE_COMMANDS=false

E2E_SCRIPT_DIR="${ROOT}/tests/e2e"
# shellcheck source=../../tests/e2e/lib/e2e_common.sh
source "${E2E_SCRIPT_DIR}/lib/e2e_common.sh"
# shellcheck source=../../tests/e2e/lib/e2e_production_destructive_aliases.sh
source "${E2E_SCRIPT_DIR}/lib/e2e_production_destructive_aliases.sh"
# shellcheck source=lib/common.sh
source "${ROOT}/scripts/e2e/lib/common.sh"

e2e_strict_mode
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
  : "${BASE_URL:=${E2E_PROD_BASE_URL:-}}"
  : "${GRPC_ADDR:=${E2E_PROD_GRPC_TARGET:-${GRPC_ADDR:-}}}"
  : "${ADMIN_EMAIL:=${E2E_PROD_ADMIN_EMAIL:-}}"
  : "${ADMIN_PASSWORD:=${E2E_PROD_ADMIN_PASSWORD:-}}"
fi
e2e_apply_production_destructive_aliases

if [[ -z "${E2E_RUN_DIR:-}" ]]; then
  e2e_init_run_dir "production-destructive-e2e-slots-preflight"
fi

BASE_URL="${BASE_URL:-https://api.ldtv.dev}"
BASE_URL="${BASE_URL%/}"
export BASE_URL
export GRPC_ADDR="${GRPC_ADDR:-machine-api.ldtv.dev:443}"
export TEST_MACHINE_ID="${TEST_MACHINE_ID:-${E2E_TEST_MACHINE_ID:-}}"

SLOT_RANGE="${E2E_TEST_SLOT_RANGE:-1-10}"
SLOT_CABINET="${E2E_TEST_SLOT_CABINET:-A}"
SLOT_START=1
SLOT_END=10
if [[ "$SLOT_RANGE" =~ ^([0-9]+)-([0-9]+)$ ]]; then
  SLOT_START="${BASH_REMATCH[1]}"
  SLOT_END="${BASH_REMATCH[2]}"
fi

: >"${E2E_RUN_DIR}/probes.tsv"
: >"${E2E_RUN_DIR}/slots.tsv"
printf 'slot_code\tslot_index\tproduct_id\tproduct_name\tprice_minor\timage_status\tinventory_before\tapi_status\tdetail\n' >>"${E2E_RUN_DIR}/slots.tsv"

FAILURES=0
pass_probe() { e2e_record_probe "$1" "PASS" "${2:-}" "${3:-0}"; }
fail_probe() { e2e_record_probe "$1" "FAIL" "${2:-}" "${3:-0}"; FAILURES=$((FAILURES + 1)); }
warn_probe() { e2e_record_probe "$1" "WARN" "${2:-}" "0"; }

echo "== Phase E API slots preflight (read-only) range=${SLOT_CABINET}${SLOT_START}-${SLOT_END} =="

ADMIN_TOK=""
if ADMIN_TOK="$(e2e_admin_token 2>/dev/null)"; then
  pass_probe "admin.auth" "token acquired"
else
  fail_probe "admin.auth" "ADMIN credentials required"
fi

meta="$(jq -nc --arg mid "${TEST_MACHINE_ID}" --arg rid "phase-e-slots-${E2E_RUN_TS:-$(date -u +%Y%m%dT%H%M%SZ)}" '{machineId:$mid,requestId:$rid}')"
export TEST_MACHINE_ID MACHINE_ACCESS_TOKEN

if e2e_grpc_call "avf.machine.v1.MachineBootstrapService/GetBootstrap" \
  "$(jq -nc --argjson meta "$meta" '{meta:$meta}')" "grpc-bootstrap" machine ""; then
  pass_probe "sync.bootstrap_grpc" "GetBootstrap ok"
else
  fail_probe "sync.bootstrap_grpc" "GetBootstrap failed"
fi

if e2e_grpc_call "avf.machine.v1.MachineCatalogService/GetCatalogSnapshot" \
  "$(jq -nc --arg mid "${TEST_MACHINE_ID}" --argjson meta "$meta" '{machineId:$mid,includeUnavailable:false,meta:$meta}')" \
  "grpc-catalog" machine ""; then
  pass_probe "sync.catalog_grpc" "GetCatalogSnapshot ok"
else
  fail_probe "sync.catalog_grpc" "GetCatalogSnapshot failed"
fi

e2e_grpc_call "avf.machine.v1.MachineInventoryService/GetPlanogram" \
  "$(jq -nc --arg mid "${TEST_MACHINE_ID}" '{machineId:$mid}')" "grpc-planogram" machine "" || true

gb="${E2E_RUN_DIR}/raw/grpc-bootstrap.response.json"
cat="${E2E_RUN_DIR}/raw/grpc-catalog.response.json"
plano="${E2E_RUN_DIR}/raw/grpc-planogram.response.json"
inv_doc="${E2E_RUN_DIR}/raw/admin-inventory.body"

if [[ -n "$ADMIN_TOK" ]]; then
  read -r _ _ < <(e2e_curl_get "admin-inventory" "${BASE_URL}/v1/admin/machines/${TEST_MACHINE_ID}/inventory" "$ADMIN_TOK")
fi

for idx in $(seq "$SLOT_START" "$SLOT_END"); do
  slot_code="${SLOT_CABINET}${idx}"
  probe_prefix="slot.${slot_code}"

  product_id="$(jq -r --arg sc "$slot_code" '
    (.topology.cabinets // [])[] | (.slots // [])[] |
    select((.slotCode // .slot_code // "") == $sc) |
    (.productId // .product_id // "")
  ' "$gb" 2>/dev/null | head -n1)"

  if [[ -z "$product_id" || "$product_id" == "null" ]]; then
    product_id="$(jq -r --arg sc "$slot_code" '
      (.slots // [])[] |
      select((.slotCode // .slot_code // "") == $sc) |
      (.productId // .product_id // "")
    ' "$plano" 2>/dev/null | head -n1)"
  fi

  product_name="$(jq -r --arg sc "$slot_code" '
    (.topology.cabinets // [])[] | (.slots // [])[] |
    select((.slotCode // .slot_code // "") == $sc) |
    (.productName // .product_name // "")
  ' "$gb" 2>/dev/null | head -n1)"

  price_minor="$(jq -r --arg sc "$slot_code" --arg pid "$product_id" '
    (.snapshot.items // [])[] |
    select((.slotCode // .slot_code // "") == $sc or ($pid != "" and (.productId // .product_id // "") == $pid)) |
    (.priceMinor // .price_minor // empty)
  ' "$cat" 2>/dev/null | head -n1)"

  if [[ -z "$price_minor" || "$price_minor" == "null" ]]; then
    price_minor="$(jq -r --arg sc "$slot_code" '
      (.topology.cabinets // [])[] | (.slots // [])[] |
      select((.slotCode // .slot_code // "") == $sc) |
      (.priceMinor // .price_minor // empty)
    ' "$gb" 2>/dev/null | head -n1)"
  fi

  qty="$(jq -r --arg sc "$slot_code" --arg pid "$product_id" '
    (.topology.cabinets // [])[] | (.slots // [])[] |
    select((.slotCode // .slot_code // "") == $sc) |
    (.maxQuantity // .quantity // .qty // 0)
  ' "$gb" 2>/dev/null | head -n1)"
  if [[ -z "$qty" || "$qty" == "null" || "$qty" -le 0 ]] && [[ -f "$inv_doc" ]]; then
    qty="$(jq -r --arg sc "$slot_code" --arg pid "$product_id" '
      (.items // .slots // .inventory // [])[] |
      select((.slotCode // .slot_code // "") == $sc or ($pid != "" and ((.productId // .product_id // "") == $pid))) |
      (.totalQuantity // .quantity // .qty // .stockOnHand // 0)
    ' "$inv_doc" 2>/dev/null | head -n1)"
  fi
  if [[ -z "$qty" || "$qty" == "null" || "$qty" -le 0 ]]; then
    qty="$(jq -r --arg sc "$slot_code" --arg pid "$product_id" '
      (.snapshot.items // [])[] |
      select((.slotCode // .slot_code // "") == $sc or ($pid != "" and (.productId // .product_id // "") == $pid)) |
      (.quantity // .qty // .maxQuantity // 0)
    ' "$cat" 2>/dev/null | head -n1)"
  fi

  image_status="missing"
  if [[ -n "$product_id" && "$product_id" != "null" && -n "$ADMIN_TOK" ]]; then
    read -r pcode _ < <(e2e_curl_get "admin-product-${slot_code}" "${BASE_URL}/v1/admin/products/${product_id}" "$ADMIN_TOK")
    prod_doc="${E2E_RUN_DIR}/raw/admin-product-${slot_code}.body"
    if [[ "$pcode" == "200" && -f "$prod_doc" ]]; then
      has_image="$(jq -r '[.imageUrl, .image_url, .mediaUrl, .media_url] | map(select(. != null and . != "")) | length' "$prod_doc")"
      primary_ready="$(jq -r --arg pid "$product_id" '
        (.catalog.products // [])[]? | select((.productId // .product_id // "") == $pid) | (.primaryMediaReady // false)
      ' "$gb" 2>/dev/null | head -n1)"
      if [[ "$has_image" -gt 0 || "$primary_ready" == "true" ]]; then
        image_status="ok"
      else
        image_status="fallback"
      fi
    fi
  fi

  slot_fail=0
  detail_parts=()
  if [[ -z "$product_id" || "$product_id" == "null" ]]; then
    slot_fail=1
    detail_parts+=("missing product mapping")
  fi
  if [[ -z "$price_minor" || "$price_minor" == "null" || "$price_minor" -le 0 ]]; then
    slot_fail=1
    detail_parts+=("missing price")
  fi
  if [[ -z "$qty" || "$qty" == "null" || "$qty" -le 0 ]]; then
    slot_fail=1
    detail_parts+=("inventory<=0")
  fi

  api_status="PASS"
  if [[ "$slot_fail" -ne 0 ]]; then
    api_status="FAIL"
    fail_probe "${probe_prefix}.api" "$(IFS='; '; echo "${detail_parts[*]}")"
    FAILURES=$((FAILURES + 1))
  else
    pass_probe "${probe_prefix}.api" "product=${product_id} qty=${qty} price=${price_minor}"
  fi

  detail="$(IFS='; '; echo "${detail_parts[*]:-ok}")"
  printf '%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n' \
    "$slot_code" "$idx" "${product_id:--}" "${product_name:--}" "${price_minor:--}" \
    "$image_status" "${qty:-0}" "$api_status" "$detail" >>"${E2E_RUN_DIR}/slots.tsv"
done

verdict="PASS"
[[ "$FAILURES" -gt 0 ]] && verdict="BLOCKED"

{
  echo "PHASE_E_API_SLOTS_PREFLIGHT"
  echo "timestamp=${E2E_RUN_TS:-unknown}"
  echo "verdict=${verdict}"
  echo "failures=${FAILURES}"
  echo "slot_range=${SLOT_CABINET}${SLOT_START}-${SLOT_END}"
  echo "test_machine_id=${TEST_MACHINE_ID}"
} >"${E2E_RUN_DIR}/READINESS.txt"

echo ""
echo "== API slots preflight verdict: ${verdict} (failures=${FAILURES}) =="
echo "Artifacts: ${E2E_RUN_DIR}"

[[ "$FAILURES" -eq 0 ]] || exit 2
