#!/usr/bin/env bash
# Mint machine access token for production E2E (activation claim). Outputs export lines to stdout.
set -Eeuo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
E2E_SCRIPT_DIR="${ROOT}/tests/e2e"
# shellcheck source=../../tests/e2e/lib/e2e_common.sh
source "${E2E_SCRIPT_DIR}/lib/e2e_common.sh"
# shellcheck source=lib/common.sh
source "${ROOT}/scripts/e2e/lib/common.sh"

CANARY_ENV="${ROOT}/tests/e2e/production/.env.production.e2e.local"
_env_file="${E2E_ENV_FILE:-${ROOT}/tests/e2e/.env.production.destructive.local}"
if [[ -f "${_env_file}" ]]; then
  _env_tmp="$(mktemp)"
  tr -d '\r' <"${_env_file}" >"${_env_tmp}"
  set -a
  # shellcheck disable=SC1090
  source "${_env_tmp}" || true
  set +a
  rm -f "${_env_tmp}"
fi
if [[ -f "$CANARY_ENV" ]]; then
  _env_tmp="$(mktemp)"
  tr -d '\r' <"$CANARY_ENV" >"$_env_tmp"
  set -a
  # shellcheck disable=SC1090
  source "$_env_tmp"
  set +a
  rm -f "$_env_tmp"
  : "${BASE_URL:=${E2E_PROD_BASE_URL:-}}"
  : "${ADMIN_EMAIL:=${E2E_PROD_ADMIN_EMAIL:-${ADMIN_EMAIL:-}}}"
  : "${ADMIN_PASSWORD:=${E2E_PROD_ADMIN_PASSWORD:-${ADMIN_PASSWORD:-}}}"
fi

export BASE_URL="${BASE_URL:-https://api.ldtv.dev}"
export GRPC_ADDR="${E2E_PROD_GRPC_TARGET:-machine-api.ldtv.dev:443}"
TEST_MACHINE_ID="${1:-${E2E_TEST_MACHINE_ID:-${TEST_MACHINE_ID:-${MACHINE_ID:-}}}}"
[[ -n "$TEST_MACHINE_ID" ]] || { echo "FATAL: TEST_MACHINE_ID required" >&2; exit 2; }

E2E_RUN_DIR="${E2E_RUN_DIR:-$(mktemp -d)}"
export E2E_RUN_DIR E2E_RUN_TS="${E2E_RUN_TS:-$(date -u +%Y%m%dT%H%M%SZ)}"
mkdir -p "${E2E_RUN_DIR}/raw"

ADMIN_TOK="$(e2e_admin_token)" || { echo "FATAL: admin login failed" >&2; exit 2; }

act_body_tmp="${E2E_RUN_DIR}/raw/.activation-code-create.raw.json"
http_code="$(curl -sS -o "$act_body_tmp" -w '%{http_code}' -X POST \
  -H "Content-Type: application/json" \
  -H "Accept: application/json" \
  -H "Authorization: Bearer ${ADMIN_TOK}" \
  -H "Idempotency-Key: mint-token-${E2E_RUN_TS}" \
  -d "$(jq -nc --arg n "mint-token ${E2E_RUN_TS}" '{expiresInMinutes:60,maxUses:1,notes:$n}')" \
  "${BASE_URL%/}/v1/admin/machines/${TEST_MACHINE_ID}/activation-codes")"
[[ "$http_code" == "201" || "$http_code" == "200" ]] || { echo "FATAL: activation code http=${http_code}" >&2; exit 2; }
act_code="$(jq -r '.activationCode // empty' "$act_body_tmp")"
[[ -n "$act_code" ]] || { echo "FATAL: activationCode missing" >&2; exit 2; }
e2e_write_redacted_activation_code_create_summary "$act_body_tmp" "$http_code"
rm -f "$act_body_tmp"

claim_req_tmp="${E2E_RUN_DIR}/raw/.activation-claim.request.raw.json"
jq -nc \
  --arg code "$act_code" \
  --arg sn "mint-token-${E2E_RUN_TS}" \
  '{activationCode:$code,deviceFingerprint:{androidId:$sn,serialNumber:$sn,manufacturer:"avf",model:"e2e-mint",packageName:"dev.avf.e2e",versionName:"1.0.0",versionCode:1}}' \
  >"$claim_req_tmp"
e2e_write_redacted_activation_claim_request_summary "$claim_req_tmp"

MACHINE_ACCESS_TOKEN=""
if e2e_grpc_call "avf.machine.v1.MachineActivationService/ClaimActivation" "$(cat "$claim_req_tmp")" "activation-claim-mint" none ""; then
  MACHINE_ACCESS_TOKEN="$(jq -r '.accessToken // .machineToken // empty' "${E2E_RUN_DIR}/raw/activation-claim-mint.response.json")"
  e2e_redact_tokens_in_json_file "${E2E_RUN_DIR}/raw/activation-claim-mint.response.json"
else
  http_code="$(curl -sS -o "${E2E_RUN_DIR}/raw/activation-claim-rest.json" -w '%{http_code}' -X POST \
    -H "Content-Type: application/json" \
    -d @"$claim_req_tmp" \
    "${BASE_URL%/}/v1/setup/activation-codes/claim")"
  [[ "$http_code" == "200" ]] || { echo "FATAL: claim failed" >&2; exit 2; }
  MACHINE_ACCESS_TOKEN="$(jq -r '.machineToken // .accessToken // empty' "${E2E_RUN_DIR}/raw/activation-claim-rest.json")"
  e2e_redact_tokens_in_json_file "${E2E_RUN_DIR}/raw/activation-claim-rest.json"
fi
rm -f "$claim_req_tmp"
[[ -n "$MACHINE_ACCESS_TOKEN" ]] || { echo "FATAL: machine token missing" >&2; exit 2; }

printf 'export TEST_MACHINE_ID=%q\n' "$TEST_MACHINE_ID"
printf 'export MACHINE_ID=%q\n' "$TEST_MACHINE_ID"
printf 'export MACHINE_ACCESS_TOKEN=%q\n' "$MACHINE_ACCESS_TOKEN"
printf 'export MACHINE_TOKEN=%q\n' "$MACHINE_ACCESS_TOKEN"
