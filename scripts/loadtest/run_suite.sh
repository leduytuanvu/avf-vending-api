#!/usr/bin/env bash
# Run the full staging-style load harness (Go avf-loadtest suite + optional k6 admin).
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "${ROOT}"

export LOAD_TEST_ENV="${LOAD_TEST_ENV:-staging}"
export EXECUTE_LOAD_TEST="${EXECUTE_LOAD_TEST:-false}"

echo "run_suite: LOAD_TEST_ENV=${LOAD_TEST_ENV} EXECUTE_LOAD_TEST=${EXECUTE_LOAD_TEST}"
echo "  step1: avf-loadtest -scenario storm (sequential check-in waves, gRPC phases, MQTT, webhook, admin)"
ARGS=( "$@" )
EXTRA_FLAGS=()
if [[ "$(printf '%s' "${EXECUTE_LOAD_TEST:-false}" | tr '[:upper:]' '[:lower:]')" == "true" ]]; then
  EXTRA_FLAGS+=( -manifest "${LOADTEST_MACHINE_MANIFEST:?set LOADTEST_MACHINE_MANIFEST when EXECUTE_LOAD_TEST=true}" )
fi

go run ./tools/loadtest/cmd/avf-loadtest \
  -scenario storm \
  -http-base "${API_BASE_URL:-http://localhost:8080}" \
  -grpc-addr "${GRPC_ADDR:-localhost:9090}" \
  -metrics-url "${AVF_LOADTEST_METRICS_URL:-}" \
  "${EXTRA_FLAGS[@]}" \
  "${ARGS[@]}"

if command -v k6 >/dev/null 2>&1; then
  echo "step2: k6 admin_smoke.k6.js (optional)"
  k6 run scripts/loadtest/admin_smoke.k6.js
else
  echo "k6 not on PATH — skip REST k6 tier (install from https://k6.io)"
fi
