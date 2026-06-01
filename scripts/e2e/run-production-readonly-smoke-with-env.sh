#!/usr/bin/env bash
# Load production E2E env files then run read-only smoke (Phase 10 matrix helper).
set -Eeuo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
E2E_SCRIPT_DIR="${ROOT}/tests/e2e"
# shellcheck source=../../tests/e2e/lib/e2e_common.sh
source "${E2E_SCRIPT_DIR}/lib/e2e_common.sh"
# shellcheck source=../../tests/e2e/lib/e2e_production_destructive_aliases.sh
source "${E2E_SCRIPT_DIR}/lib/e2e_production_destructive_aliases.sh"

export E2E_ALLOW_WRITES=false
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
: "${TEST_MACHINE_ID:=${MACHINE_ID:-}}"
: "${MACHINE_ACCESS_TOKEN:=${MACHINE_TOKEN:-}}"

export BASE_URL="${BASE_URL:-https://api.ldtv.dev}"
if [[ -n "${E2E_PROD_GRPC_TARGET:-}" ]]; then
  export GRPC_ADDR="${E2E_PROD_GRPC_TARGET}"
elif [[ "${GRPC_ADDR:-}" == *"api.ldtv.dev"* ]]; then
  export GRPC_ADDR="machine-api.ldtv.dev:443"
fi
exec bash "${ROOT}/scripts/e2e/production-readonly-smoke.sh"
