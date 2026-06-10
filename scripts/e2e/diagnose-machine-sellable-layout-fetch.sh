#!/usr/bin/env bash
# Fetch production machine topology + catalog evidence for diagnose-machine-sellable-layout.ps1
set -Eeuo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT"

MACHINE_ID="${1:-}"
ARTIFACT_DIR="${2:-}"
if [[ -z "$MACHINE_ID" || -z "$ARTIFACT_DIR" ]]; then
  echo "usage: diagnose-machine-sellable-layout-fetch.sh <machine_id> <artifact_dir>" >&2
  exit 2
fi

mkdir -p "${ARTIFACT_DIR}/raw"
export E2E_RUN_DIR="${ARTIFACT_DIR}"
export E2E_RUN_TS="${E2E_RUN_TS:-$(date -u +%Y%m%dT%H%M%SZ)}"
export E2E_TEST_MACHINE_ID="${MACHINE_ID}"
export TEST_MACHINE_ID="${MACHINE_ID}"

eval "$(bash scripts/e2e/mint-production-machine-token.sh "${MACHINE_ID}")"
export E2E_ALLOW_WRITES=false
export E2E_ALLOW_REAL_DISPENSE=false
export E2E_EXPECT_REAL_DISPENSE=false
export E2E_ALLOW_REAL_PAYMENT=false
export E2E_ALLOW_REAL_MACHINE_COMMANDS=false

E2E_SCRIPT_DIR="${ROOT}/tests/e2e"
source "${E2E_SCRIPT_DIR}/lib/e2e_common.sh"
source "${E2E_SCRIPT_DIR}/lib/e2e_production_destructive_aliases.sh"
source "${ROOT}/scripts/e2e/lib/common.sh"

e2e_strict_mode
load_env "${E2E_ENV_FILE:-${ROOT}/tests/e2e/.env.production.destructive.local}"
e2e_apply_production_destructive_aliases

BASE_URL="${BASE_URL:-https://api.ldtv.dev}"
BASE_URL="${BASE_URL%/}"
export BASE_URL
export GRPC_ADDR="${GRPC_ADDR:-machine-api.ldtv.dev:443}"

meta="$(jq -nc --arg mid "${MACHINE_ID}" --arg rid "diag-layout-${E2E_RUN_TS}" '{machineId:$mid,requestId:$rid}')"

curl -sS --connect-timeout 8 --max-time 20 "${BASE_URL}/version" >"${ARTIFACT_DIR}/raw/version.json" || true
curl -sS --connect-timeout 8 --max-time 20 "${BASE_URL}/health/live" >"${ARTIFACT_DIR}/raw/health-live.txt" || true
curl -sS --connect-timeout 8 --max-time 20 "${BASE_URL}/health/ready" >"${ARTIFACT_DIR}/raw/health-ready.txt" || true

ADMIN_TOK=""
if ADMIN_TOK="$(e2e_admin_token 2>/dev/null)"; then
  e2e_curl_get "admin-machine" "${BASE_URL}/v1/admin/machines/${MACHINE_ID}" "$ADMIN_TOK" >/dev/null || true
  e2e_curl_get "admin-inventory" "${BASE_URL}/v1/admin/machines/${MACHINE_ID}/inventory" "$ADMIN_TOK" >/dev/null || true
  e2e_curl_get "admin-planograms" "${BASE_URL}/v1/admin/planograms?machineId=${MACHINE_ID}&limit=5" "$ADMIN_TOK" >/dev/null || true
fi

export TEST_MACHINE_ID MACHINE_ACCESS_TOKEN

e2e_grpc_call "avf.machine.v1.MachineBootstrapService/GetBootstrap" \
  "$(jq -nc --argjson meta "$meta" '{meta:$meta}')" "grpc-bootstrap" machine "" || true

e2e_grpc_call "avf.machine.v1.MachineCatalogService/GetCatalogSnapshot" \
  "$(jq -nc --arg mid "${MACHINE_ID}" --argjson meta "$meta" '{machineId:$mid,includeUnavailable:false,includeImages:true,meta:$meta}')" \
  "grpc-catalog-sellable" machine "" || true

e2e_grpc_call "avf.machine.v1.MachineCatalogService/GetCatalogSnapshot" \
  "$(jq -nc --arg mid "${MACHINE_ID}" --argjson meta "$meta" '{machineId:$mid,includeUnavailable:true,includeImages:true,meta:$meta}')" \
  "grpc-catalog-all" machine "" || true

e2e_grpc_call "avf.machine.v1.MachineInventoryService/GetPlanogram" \
  "$(jq -nc --arg mid "${MACHINE_ID}" '{machineId:$mid}')" "grpc-planogram" machine "" || true

e2e_grpc_call "avf.machine.v1.MachineInventoryService/GetInventorySnapshot" \
  "$(jq -nc --argjson meta "$meta" '{meta:$meta}')" "grpc-inventory" machine "" || true

if [[ -n "${MACHINE_ACCESS_TOKEN:-}" ]]; then
  curl -sS -o "${ARTIFACT_DIR}/raw/rest-sale-catalog.body" -w '%{http_code}' \
    -H "Accept: application/json" \
    -H "Authorization: Bearer ${MACHINE_ACCESS_TOKEN}" \
    -H "x-machine-id: ${MACHINE_ID}" \
    "${BASE_URL}/v1/machines/${MACHINE_ID}/sale-catalog?include_images=true" \
    >"${ARTIFACT_DIR}/raw/rest-sale-catalog.meta" 2>/dev/null || true
fi

echo "FETCH_OK artifact_dir=${ARTIFACT_DIR}"
