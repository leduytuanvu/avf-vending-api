#!/usr/bin/env bash
# Gate 4 final evidence canary — mint token + isolated subcase orders.
set -Eeuo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
REPORT_ROOT="${GATE4_REPORT_ROOT:-${ROOT}/../reports}"
LATEST_TS="$(cat "${REPORT_ROOT}/.latest_ts" 2>/dev/null || date -u +%Y%m%dT%H%M%SZ)"
RUN_DIR="${E2E_RUN_DIR:-${REPORT_ROOT}/${LATEST_TS}/gap4-canary-final-$(date -u +%Y%m%dT%H%M%SZ)}"
export E2E_RUN_DIR="$RUN_DIR"
export E2E_CANARY_LABEL="gate4-canary-final"
export GRPC_MAX_TIME="${GRPC_MAX_TIME:-120}"
export TEST_MACHINE_ID="${TEST_MACHINE_ID:-019e702c-11c6-7ab0-89c7-5eb32f0b12cb}"
mkdir -p "$RUN_DIR"
cd "$ROOT"
eval "$(./scripts/e2e/mint-production-machine-token.sh "$TEST_MACHINE_ID")"
export MACHINE_ACCESS_TOKEN
export BASE_URL="${BASE_URL:-https://api.ldtv.dev}"
exec ./scripts/e2e/production-canary-vend-evidence-grpc.sh
