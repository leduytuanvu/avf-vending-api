#!/usr/bin/env bash
# Create a one-time activation code for the production test machine (device claim, not server-side mint).
set -Eeuo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
E2E_SCRIPT_DIR="${ROOT}/tests/e2e"
# shellcheck source=../../tests/e2e/lib/e2e_common.sh
source "${E2E_SCRIPT_DIR}/lib/e2e_common.sh"
# shellcheck source=lib/common.sh
source "${ROOT}/scripts/e2e/lib/common.sh"

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
  : "${ADMIN_EMAIL:=${E2E_PROD_ADMIN_EMAIL:-${ADMIN_EMAIL:-}}}"
  : "${ADMIN_PASSWORD:=${E2E_PROD_ADMIN_PASSWORD:-${ADMIN_PASSWORD:-}}}"
fi

export BASE_URL="${BASE_URL:-https://api.ldtv.dev}"
TEST_MACHINE_ID="${1:-${E2E_TEST_MACHINE_ID:-${TEST_MACHINE_ID:-${MACHINE_ID:-}}}}"
[[ -n "$TEST_MACHINE_ID" ]] || { echo "FATAL: TEST_MACHINE_ID required" >&2; exit 2; }

E2E_RUN_DIR="${E2E_RUN_DIR:-$(mktemp -d)}"
export E2E_RUN_DIR E2E_RUN_TS="${E2E_RUN_TS:-$(date -u +%Y%m%dT%H%M%SZ)}"
mkdir -p "${E2E_RUN_DIR}/raw"

ADMIN_TOK="$(e2e_admin_token)" || { echo "FATAL: admin login failed" >&2; exit 2; }

act_body="${E2E_RUN_DIR}/raw/activation-code-create-device.json"
http_code="$(curl -sS -o "$act_body" -w '%{http_code}' -X POST \
  -H "Content-Type: application/json" \
  -H "Accept: application/json" \
  -H "Authorization: Bearer ${ADMIN_TOK}" \
  -H "Idempotency-Key: device-activation-${E2E_RUN_TS}" \
  -d "$(jq -nc --arg n "phase-d-device ${E2E_RUN_TS}" '{expiresInMinutes:60,maxUses:1,notes:$n}')" \
  "${BASE_URL%/}/v1/admin/machines/${TEST_MACHINE_ID}/activation-codes")"
[[ "$http_code" == "201" || "$http_code" == "200" ]] || { echo "FATAL: activation code http=${http_code}" >&2; exit 2; }
act_code="$(jq -r '.activationCode // empty' "$act_body")"
[[ -n "$act_code" ]] || { echo "FATAL: activationCode missing" >&2; exit 2; }

printf 'ACTIVATION_CODE=%s\n' "$act_code"
