#!/usr/bin/env bash
# API preflight helper for Phase D orchestrator.
set -Eeuo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT"
unset E2E_RUN_TS E2E_RUN_DIR E2E_OUTPUT_DIR || true
ARTIFACT_SUB="${1:-api-preflight}"
ARTIFACT_ROOT="${2:-}"
if [[ -n "$ARTIFACT_ROOT" ]]; then
  export E2E_RUN_DIR="${ARTIFACT_ROOT}/${ARTIFACT_SUB}"
  mkdir -p "${E2E_RUN_DIR}/raw"
fi
if [[ -z "${E2E_TEST_MACHINE_ID:-}" && -z "${TEST_MACHINE_ID:-}" ]]; then
  echo "FAIL: E2E_TEST_MACHINE_ID (or TEST_MACHINE_ID) must be set — no hardcoded machine UUID default." >&2
  exit 2
fi
eval "$(bash scripts/e2e/mint-production-machine-token.sh "${E2E_TEST_MACHINE_ID:-${TEST_MACHINE_ID}}")"
export E2E_ALLOW_WRITES=false
export E2E_ALLOW_REAL_DISPENSE=false
export E2E_EXPECT_REAL_DISPENSE=false
export E2E_ALLOW_REAL_PAYMENT=false
export E2E_ALLOW_REAL_MACHINE_COMMANDS=false
bash scripts/e2e/production-destructive-e2e-preflight.sh
