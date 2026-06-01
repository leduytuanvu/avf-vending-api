#!/usr/bin/env bash
# API preflight helper for Phase D orchestrator.
set -Eeuo pipefail
ROOT="/d/admin/development/avf/avf-vending-system/avf-vending-api"
cd "$ROOT"
unset E2E_RUN_TS E2E_RUN_DIR E2E_OUTPUT_DIR || true
ARTIFACT_SUB="${1:-api-preflight}"
ARTIFACT_ROOT="${2:-}"
if [[ -n "$ARTIFACT_ROOT" ]]; then
  export E2E_RUN_DIR="${ARTIFACT_ROOT}/${ARTIFACT_SUB}"
  mkdir -p "${E2E_RUN_DIR}/raw"
fi
eval "$(bash scripts/e2e/mint-production-machine-token.sh "${E2E_TEST_MACHINE_ID:-019e702c-11c6-7ab0-89c7-5eb32f0b12cb}")"
export E2E_ALLOW_WRITES=false
export E2E_ALLOW_REAL_DISPENSE=false
export E2E_EXPECT_REAL_DISPENSE=false
export E2E_ALLOW_REAL_PAYMENT=false
export E2E_ALLOW_REAL_MACHINE_COMMANDS=false
bash scripts/e2e/production-destructive-e2e-preflight.sh
