#!/usr/bin/env bash
# Phase 6 — production destructive E2E *preparation* (read-only). No cash, dispense, or inventory mutation.
#
# Usage:
#   E2E_ENV_FILE=tests/e2e/.env.production.destructive.local \
#     bash scripts/e2e/production-destructive-e2e-preflight.sh
#
# Optional aliases (mapped to canonical names):
#   E2E_ENV=production E2E_BASE_URL=... E2E_TEST_MACHINE_ID=... E2E_TEST_SLOT_CODE=...
#
# Real destructive run (later phase) requires E2E_PRODUCTION_DESTRUCTIVE=true — see production-destructive-e2e-gate.sh

set -Eeuo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
E2E_SCRIPT_DIR="${ROOT}/tests/e2e"
# shellcheck source=../../tests/e2e/lib/e2e_common.sh
source "${E2E_SCRIPT_DIR}/lib/e2e_common.sh"
# shellcheck source=../../tests/e2e/lib/e2e_production_destructive_aliases.sh
source "${E2E_SCRIPT_DIR}/lib/e2e_production_destructive_aliases.sh"
# shellcheck source=lib/common.sh
source "${ROOT}/scripts/e2e/lib/common.sh"

e2e_strict_mode
export E2E_ALLOW_WRITES=false
unset E2E_RUN_TS E2E_RUN_DIR E2E_OUTPUT_DIR || true
CANARY_ENV="${ROOT}/tests/e2e/production/.env.production.e2e.local"
load_env "${E2E_ENV_FILE:-${ROOT}/tests/e2e/.env.production.destructive.local}"
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
  : "${MACHINE_ACCESS_TOKEN:=${E2E_PROD_MACHINE_ACCESS_TOKEN:-${MACHINE_ACCESS_TOKEN:-}}}"
  : "${TEST_MACHINE_ID:=${E2E_PROD_MACHINE_ID:-${TEST_MACHINE_ID:-}}}"
fi
e2e_apply_production_destructive_aliases

export E2E_ALLOW_WRITES=false
export E2E_ALLOW_DESTRUCTIVE=false
export E2E_ALLOW_REAL_DISPENSE=false
export E2E_ALLOW_REAL_PAYMENT=false
export E2E_ALLOW_REAL_MACHINE_COMMANDS=false
export E2E_EXPECT_REAL_DISPENSE=false

e2e_refuse_irreversible_flags_for_preflight

e2e_require_cmd curl jq grpcurl git
e2e_init_run_dir "production-destructive-e2e-preflight"

BASE_URL="${BASE_URL:-https://api.ldtv.dev}"
BASE_URL="${BASE_URL%/}"
export BASE_URL

if [[ -n "${E2E_PROD_GRPC_TARGET:-}" ]]; then
  export GRPC_ADDR="${E2E_PROD_GRPC_TARGET}"
elif [[ "${GRPC_ADDR:-}" == *"api.ldtv.dev"* ]]; then
  export GRPC_ADDR="machine-api.ldtv.dev:443"
else
  export GRPC_ADDR="${GRPC_ADDR:-machine-api.ldtv.dev:443}"
fi

: >"${E2E_RUN_DIR}/probes.tsv"
FAILURES=0
BLOCKERS=()

pass_probe() { e2e_record_probe "$1" "PASS" "${2:-}" "${3:-0}"; echo "PASS  $1 ${2:+($2)}"; }
fail_probe() { e2e_record_probe "$1" "FAIL" "${2:-}" "${3:-0}"; echo "FAIL  $1 ${2:-}"; FAILURES=$((FAILURES + 1)); BLOCKERS+=("$1: $2"); }
skip_probe() { e2e_record_probe "$1" "SKIP" "${2:-}" "0"; echo "SKIP  $1 ${2:-}"; }
warn_probe() { e2e_record_probe "$1" "WARN" "${2:-}" "0"; echo "WARN  $1 ${2:-}"; }

echo "== Phase 6 production destructive E2E preflight (read-only) =="
echo "run_dir=${E2E_RUN_DIR}"
echo "base_url=${BASE_URL}"

# --- Gitignore / secrets hygiene ---
local_env="${E2E_ENV_FILE:-${ROOT}/tests/e2e/.env.production.destructive.local}"
if [[ -f "$local_env" ]]; then
  if git -C "$ROOT" check-ignore -q "$local_env" 2>/dev/null; then
    pass_probe "config.gitignore_local_env" "ignored=$(basename "$local_env")"
  else
    fail_probe "config.gitignore_local_env" "${local_env} is not gitignored"
  fi
  if git -C "$ROOT" ls-files --error-unmatch "$local_env" >/dev/null 2>&1; then
    fail_probe "config.local_env_not_tracked" "${local_env} is tracked by git"
  else
    pass_probe "config.local_env_not_tracked" "ok"
  fi
else
  warn_probe "config.local_env_present" "missing ${local_env} — copy from .env.production.destructive.example"
fi

if [[ -f "${ROOT}/tests/e2e/lib/e2e_production_destructive_aliases.sh" ]]; then
  pass_probe "config.safety_gate_library" "e2e_production_destructive_aliases.sh present"
else
  fail_probe "config.safety_gate_library" "missing alias/gate library"
fi

if [[ "${E2E_PRODUCTION_DESTRUCTIVE:-false}" == "true" ]]; then
  warn_probe "config.destructive_flag_in_env" "E2E_PRODUCTION_DESTRUCTIVE=true in env — OK for future run, preflight stays read-only"
else
  pass_probe "config.destructive_flag" "E2E_PRODUCTION_DESTRUCTIVE not set (preflight mode)"
fi

# --- Required inputs ---
: "${TEST_MACHINE_ID:=${MACHINE_ID:-}}"
: "${MACHINE_ACCESS_TOKEN:=${MACHINE_TOKEN:-}}"
: "${TEST_SLOT_CODE:=${E2E_TEST_SLOT_CODE:-A1}}"
: "${TEST_PRODUCT_ID:=${E2E_TEST_PRODUCT_ID:-}}"
: "${TEST_PAYMENT_METHOD:=${E2E_PAYMENT_METHOD:-cash}}"
: "${SLOT_INDEX:=${E2E_SLOT_INDEX:-0}}"

for var in TEST_MACHINE_ID TEST_SLOT_CODE MACHINE_ACCESS_TOKEN; do
  if [[ -z "${!var:-}" ]]; then
    fail_probe "config.required_${var,,}" "unset — fill MACHINE_ID/MACHINE_TOKEN or TEST_* in ${local_env}"
  else
    pass_probe "config.required_${var,,}" "set (value redacted)"
  fi
done

if [[ -z "${TEST_PRODUCT_ID:-}" ]]; then
  warn_probe "config.test_product_id" "TEST_PRODUCT_ID unset — will resolve from planogram slot ${TEST_SLOT_CODE} if gRPC succeeds"
else
  pass_probe "config.test_product_id" "set (value redacted)"
fi

if [[ "${TEST_PAYMENT_METHOD,,}" != "cash" ]]; then
  warn_probe "config.payment_method" "expected cash for cash-only pilot, got ${TEST_PAYMENT_METHOD}"
else
  pass_probe "config.payment_method" "cash"
fi

# --- HTTP health ---
for path in /health/live /health/ready /version; do
  url="${BASE_URL}${path}"
  read -r code lat < <(e2e_curl_get "http${path//\//-}" "$url" "")
  if [[ "$code" == "200" ]]; then
    pass_probe "http${path}" "status=${code}" "$lat"
  else
    fail_probe "http${path}" "expected 200 got ${code}" "$lat"
  fi
done

ADMIN_TOK=""
if ADMIN_TOK="$(e2e_admin_token 2>/dev/null)"; then
  pass_probe "admin.auth" "token acquired"
else
  fail_probe "admin.auth" "ADMIN_TOKEN or ADMIN_EMAIL+ADMIN_PASSWORD required"
fi

if [[ -n "$ADMIN_TOK" ]]; then
  machine_doc="${E2E_RUN_DIR}/raw/admin-machine.body"
  read -r mcode _ < <(e2e_curl_get "admin-machine" "${BASE_URL}/v1/admin/machines/${TEST_MACHINE_ID}" "$ADMIN_TOK")
  if [[ "$mcode" == "200" ]]; then
    status="$(jq -r '.status // empty' "$machine_doc" | tr '[:upper:]' '[:lower:]')"
    name="$(jq -r '.name // .code // empty' "$machine_doc")"
    case "$status" in
      active|online|offline) pass_probe "machine.status" "status=${status} name=${name}" ;;
      *) fail_probe "machine.status" "ineligible status=${status}" ;;
    esac
    last_seen="$(jq -r '.lastSeenAt // .last_seen_at // .lastHeartbeatAt // empty' "$machine_doc")"
    if [[ -n "$last_seen" && "$last_seen" != "null" ]]; then
      pass_probe "machine.last_seen" "last_seen=${last_seen}"
    else
      warn_probe "machine.last_seen" "no heartbeat field — verify board connectivity manually"
    fi
  else
    fail_probe "admin.machine_detail" "http=${mcode}"
  fi

  inv_doc="${E2E_RUN_DIR}/raw/admin-inventory.body"
  read -r icode _ < <(e2e_curl_get "admin-inventory" "${BASE_URL}/v1/admin/machines/${TEST_MACHINE_ID}/inventory" "$ADMIN_TOK")
  if [[ "$icode" != "200" ]]; then
    fail_probe "admin.inventory" "http=${icode}"
  else
    pass_probe "admin.inventory" "http=${icode}"
  fi
fi

# --- gRPC machine sync (bootstrap / planogram / catalog) ---
export TEST_MACHINE_ID TEST_SLOT_CODE MACHINE_ACCESS_TOKEN
export GRPC_ADDR="${GRPC_ADDR:-machine-api.ldtv.dev:443}"
meta="$(jq -nc --arg mid "${TEST_MACHINE_ID}" --arg rid "phase6-preflight-${E2E_RUN_TS}" '{machineId:$mid,requestId:$rid}')"

if e2e_grpc_call "avf.machine.v1.MachineBootstrapService/GetBootstrap" \
  "$(jq -nc --argjson meta "$meta" '{meta:$meta}')" "grpc-bootstrap" machine ""; then
  gb_resp="${E2E_RUN_DIR}/raw/grpc-bootstrap.response.json"
  cash_ok="$(jq -r '.paymentMethods.cashEnabled // .payment_methods.cash_enabled // false' "$gb_resp")"
  if [[ "$cash_ok" == "true" ]]; then
    pass_probe "sync.bootstrap_cash" "cashEnabled=true"
  else
    fail_probe "sync.bootstrap_cash" "cash not enabled for machine"
  fi
else
  fail_probe "sync.bootstrap_grpc" "GetBootstrap failed"
fi

if e2e_grpc_call "avf.machine.v1.MachineInventoryService/GetPlanogram" \
  "$(jq -nc --arg mid "${TEST_MACHINE_ID}" '{machineId:$mid}')" "grpc-planogram" machine ""; then
  slot_product="$(jq -r --arg sc "${TEST_SLOT_CODE}" '
    (.slots // [])[] |
    select((.slotCode // .slot_code // "") == $sc) |
    (.productId // .product_id // "")
  ' "${E2E_RUN_DIR}/raw/grpc-planogram.response.json" | head -n1)"
  plano_source="grpc.planogram"
else
  gb_resp="${E2E_RUN_DIR}/raw/grpc-bootstrap.response.json"
  if [[ -f "$gb_resp" ]]; then
    slot_product="$(jq -r --arg sc "${TEST_SLOT_CODE}" '
      (.topology.cabinets // [])[] |
      (.slots // [])[] |
      select((.slotCode // .slot_code // "") == $sc) |
      (.productId // .product_id // "")
    ' "$gb_resp" | head -n1)"
    if [[ -n "$slot_product" && "$slot_product" != "null" ]]; then
      pass_probe "sync.planogram_grpc" "GetPlanogram blocked — using bootstrap topology fallback"
      plano_source="bootstrap.topology"
    else
      fail_probe "sync.planogram_grpc" "GetPlanogram failed and bootstrap topology empty for slot=${TEST_SLOT_CODE}"
      slot_product=""
      plano_source=""
    fi
  else
    fail_probe "sync.planogram_grpc" "GetPlanogram failed"
    slot_product=""
    plano_source=""
  fi
fi

if [[ -n "$plano_source" ]]; then
  if [[ -z "${TEST_PRODUCT_ID:-}" && -n "$slot_product" && "$slot_product" != "null" ]]; then
    export TEST_PRODUCT_ID="$slot_product"
    pass_probe "config.test_product_id_resolved" "product=${TEST_PRODUCT_ID} from planogram"
  fi
  if [[ -n "${TEST_PRODUCT_ID:-}" && "$slot_product" == "${TEST_PRODUCT_ID}" ]]; then
    pass_probe "sync.planogram_mapping" "slot=${TEST_SLOT_CODE} product=${slot_product}"
  elif [[ -n "${TEST_PRODUCT_ID:-}" ]]; then
    fail_probe "sync.planogram_mapping" "slot=${TEST_SLOT_CODE} product=${slot_product:-missing} expected=${TEST_PRODUCT_ID}"
  else
    fail_probe "sync.planogram_mapping" "slot=${TEST_SLOT_CODE} product=${slot_product:-missing}"
  fi
fi

if [[ -n "${TEST_PRODUCT_ID:-}" ]]; then
  if e2e_grpc_call "avf.machine.v1.MachineCatalogService/GetCatalogSnapshot" \
    "$(jq -nc --arg mid "${TEST_MACHINE_ID}" --argjson meta "$meta" '{machineId:$mid,includeUnavailable:false,meta:$meta}')" \
    "grpc-catalog" machine ""; then
    cat_resp="${E2E_RUN_DIR}/raw/grpc-catalog.response.json"
    price_found="$(jq -r --arg pid "${TEST_PRODUCT_ID}" --arg sc "${TEST_SLOT_CODE}" '
      (.snapshot.items // [])[] |
      select((.productId // .product_id // "") == $pid) |
      select((.slotCode // .slot_code // "") == $sc or $sc == "") |
      (.priceMinor // .price_minor // empty)
    ' "$cat_resp" | head -n1)"
    if [[ -n "$price_found" && "$price_found" != "null" ]]; then
      pass_probe "sync.catalog_product" "product=${TEST_PRODUCT_ID} priceMinor=${price_found}"
    else
      fail_probe "sync.catalog_product" "product/slot not in GetCatalogSnapshot"
    fi
  else
    fail_probe "sync.catalog_grpc" "GetCatalogSnapshot failed"
  fi
fi

if [[ -n "${TEST_PRODUCT_ID:-}" && -n "$ADMIN_TOK" ]]; then
  prod_doc="${E2E_RUN_DIR}/raw/admin-product.body"
  read -r pcode _ < <(e2e_curl_get "admin-product" "${BASE_URL}/v1/admin/products/${TEST_PRODUCT_ID}" "$ADMIN_TOK")
  if [[ "$pcode" == "200" ]]; then
    active="$(jq -r '.active // .isActive // true' "$prod_doc")"
    price="$(jq -r '.priceMinor // .price_minor // empty' "$prod_doc")"
    has_image="$(jq -r '[.imageUrl, .image_url, .mediaUrl, .media_url] | map(select(. != null and . != "")) | length' "$prod_doc")"
    if [[ "$active" == "true" || "$active" == "1" ]]; then
      pass_probe "product.active" "active=true"
    else
      fail_probe "product.active" "product not active"
    fi
    if [[ -n "$price" && "$price" != "null" ]]; then
      pass_probe "product.price" "priceMinor=${price}"
    elif [[ -f "${E2E_RUN_DIR}/raw/grpc-catalog.response.json" ]]; then
      pass_probe "product.price" "priceMinor from catalog sync (admin product has slot price only)"
    else
      fail_probe "product.price" "missing priceMinor"
    fi
    if [[ "$has_image" -gt 0 ]]; then
      pass_probe "product.image" "image url present"
    else
      warn_probe "product.image" "no image URL on product record — app may show fallback"
    fi
  else
    fail_probe "admin.product_detail" "http=${pcode}"
  fi
fi

# --- Inventory qty (after bootstrap/catalog so fallbacks exist) ---
inv_doc="${E2E_RUN_DIR}/raw/admin-inventory.body"
qty=""
if [[ -f "$inv_doc" ]]; then
  qty="$(jq -r --arg pid "${TEST_PRODUCT_ID:-}" --arg sc "$TEST_SLOT_CODE" '
    (.items // .slots // .inventory // [])[] |
    select(
      ($pid != "" and ((.productId // .product_id // "") == $pid)) or
      ((.slotCode // .slot_code // "") == $sc)
    ) |
    (.totalQuantity // .quantity // .qty // .stockOnHand // 0)
  ' "$inv_doc" | head -n1)"
fi
if [[ -z "$qty" || "$qty" == "null" || "$qty" -le 0 ]]; then
  gb_resp="${E2E_RUN_DIR}/raw/grpc-bootstrap.response.json"
  if [[ -f "$gb_resp" ]]; then
    qty="$(jq -r --arg sc "$TEST_SLOT_CODE" '
      (.topology.cabinets // [])[] | (.slots // [])[] |
      select((.slotCode // .slot_code // "") == $sc) |
      (.maxQuantity // .quantity // .qty // 0)
    ' "$gb_resp" | head -n1)"
  fi
fi
if [[ -z "$qty" || "$qty" == "null" || "$qty" -le 0 ]] && [[ -f "${E2E_RUN_DIR}/raw/grpc-catalog.response.json" ]]; then
  qty="$(jq -r --arg pid "${TEST_PRODUCT_ID:-}" --arg sc "$TEST_SLOT_CODE" '
    (.snapshot.items // [])[] |
    select((.productId // .product_id // "") == $pid) |
    select((.slotCode // .slot_code // "") == $sc or $sc == "") |
    (.quantity // .qty // .maxQuantity // 1)
  ' "${E2E_RUN_DIR}/raw/grpc-catalog.response.json" | head -n1)"
fi
if [[ -n "$qty" && "$qty" != "null" && "$qty" -gt 0 ]]; then
  pass_probe "inventory.slot_qty" "slot=${TEST_SLOT_CODE} qty=${qty}"
else
  fail_probe "inventory.slot_qty" "slot=${TEST_SLOT_CODE} product=${TEST_PRODUCT_ID:-unknown} qty=${qty:-missing}"
fi

# --- Report ---
verdict="PASS"
[[ "$FAILURES" -gt 0 ]] && verdict="BLOCKED"

{
  echo "PHASE6_PRODUCTION_DESTRUCTIVE_E2E_PREFLIGHT"
  echo "timestamp=${E2E_RUN_TS}"
  echo "verdict=${verdict}"
  echo "failures=${FAILURES}"
  echo "base_url=${BASE_URL}"
  echo "test_machine_id=${TEST_MACHINE_ID}"
  echo "test_slot_code=${TEST_SLOT_CODE}"
  echo "test_product_id=${TEST_PRODUCT_ID}"
  echo "test_payment_method=${TEST_PAYMENT_METHOD}"
  echo "machine_family=${PRODUCTION_E2E_MACHINE_FAMILY:-TCN}"
  echo "destructive_gate_required=E2E_PRODUCTION_DESTRUCTIVE=true"
  echo "real_dispense_blocked=true"
  echo ""
  echo "## Blockers"
  if [[ "${#BLOCKERS[@]}" -eq 0 ]]; then
    echo "- none"
  else
    for b in "${BLOCKERS[@]}"; do
      echo "- ${b}"
    done
  fi
} >"${E2E_RUN_DIR}/READINESS.txt"

echo ""
echo "== Preflight verdict: ${verdict} (failures=${FAILURES}) =="
echo "Artifacts: ${E2E_RUN_DIR}"

[[ "$FAILURES" -eq 0 ]] || exit 2
