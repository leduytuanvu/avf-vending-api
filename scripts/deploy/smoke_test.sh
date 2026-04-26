#!/usr/bin/env bash
# Post-deploy read-only HTTP smoke (tools/smoke_test.py).
#
# Required env:
#   BASE_URL          e.g. https://api.example.com
#   ENVIRONMENT_NAME  e.g. staging | production
# Optional (reported as skip with reason if unset; optional failures still fail the run):
#   SMOKE_AUTH_TOKEN + SMOKE_AUTH_READ_PATH  (Bearer GET for a read-only admin route)
#   SMOKE_CHECK_DB_PATH, SMOKE_CHECK_REDIS_PATH, SMOKE_CHECK_HEARTBEAT_PATH, SMOKE_CHECK_PAYMENT_MOCK_PATH
#   SMOKE_CHECK_DB_BODY_SUBSTRING, SMOKE_CHECK_REDIS_BODY_SUBSTRING, etc.
#   SMOKE_CHECK_MQTT_URL (full http(s) URL, e.g. EMQX status if reachable from the runner)
# General:
#   SMOKE_MAX_ATTEMPTS, SMOKE_BACKOFF_BASE_SEC, SMOKE_REQUEST_TIMEOUT_SEC, SMOKE_WARN_LATENCY_MS
#   SMOKE_REPORT_PATH  override JSON output (else smoke-reports/smoke-test.json)
set -Eeuo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "${ROOT}"

python_exec=""
for c in python3 python; do
  if command -v "${c}" >/dev/null 2>&1; then
    if "${c}" -c "import sys" 2>/dev/null; then
      python_exec="${c}"
      break
    fi
  fi
done
if [[ -z "${python_exec}" ]]; then
  echo "smoke_test.sh: error: python3 required" >&2
  exit 1
fi

report_args=()
if [[ -n "${SMOKE_REPORT_PATH:-}" ]]; then
  report_args=(--report "${SMOKE_REPORT_PATH}")
fi

exec "${python_exec}" "${ROOT}/tools/smoke_test.py" "${report_args[@]}"
