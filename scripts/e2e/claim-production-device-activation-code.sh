#!/usr/bin/env bash
# Claim (consume) an activation code via production API — for already-used matrix precondition.
set -Eeuo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
E2E_SCRIPT_DIR="${ROOT}/tests/e2e"
# shellcheck source=../../tests/e2e/lib/e2e_common.sh
source "${E2E_SCRIPT_DIR}/lib/e2e_common.sh"
# shellcheck source=lib/common.sh
source "${ROOT}/scripts/e2e/lib/common.sh"

load_env "${E2E_ENV_FILE:-${ROOT}/tests/e2e/.env.production.destructive.local}"
export BASE_URL="${BASE_URL:-https://api.ldtv.dev}"
export GRPC_ADDR="${E2E_PROD_GRPC_TARGET:-machine-api.ldtv.dev:443}"

ACT_CODE="${1:-}"
TEST_MACHINE_ID="${2:-${E2E_TEST_MACHINE_ID:-019e702c-11c6-7ab0-89c7-5eb32f0b12cb}}"
[[ -n "$ACT_CODE" ]] || { echo "FATAL: activation code arg required" >&2; exit 2; }

E2E_RUN_DIR="${E2E_RUN_DIR:-$(mktemp -d)}"
export E2E_RUN_DIR E2E_RUN_TS="${E2E_RUN_TS:-$(date -u +%Y%m%dT%H%M%SZ)}"
mkdir -p "${E2E_RUN_DIR}/raw"

claim_req="${E2E_RUN_DIR}/raw/activation-claim.request.json"
jq -nc \
  --arg code "$ACT_CODE" \
  --arg sn "consume-${E2E_RUN_TS}" \
  '{activationCode:$code,deviceFingerprint:{androidId:$sn,serialNumber:$sn,manufacturer:"avf",model:"e2e-consume",packageName:"dev.avf.e2e",versionName:"1.0.0",versionCode:1}}' \
  >"$claim_req"

if e2e_grpc_call "avf.machine.v1.MachineActivationService/ClaimActivation" "$(cat "$claim_req")" "activation-claim-consume" none ""; then
  echo "CLAIM_OK=grpc"
  exit 0
fi

http_code="$(curl -sS -o "${E2E_RUN_DIR}/raw/activation-claim-rest.json" -w '%{http_code}' -X POST \
  -H "Content-Type: application/json" \
  -d @"$claim_req" \
  "${BASE_URL%/}/v1/setup/activation-codes/claim")"
[[ "$http_code" == "200" ]] || { echo "FATAL: claim failed http=${http_code}" >&2; exit 2; }
echo "CLAIM_OK=rest"
