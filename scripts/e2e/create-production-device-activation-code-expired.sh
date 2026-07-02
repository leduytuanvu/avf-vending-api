#!/usr/bin/env bash
# Mint an activation code that expires quickly, wait past expiry, export ACTIVATION_CODE_EXPIRED.
set -Eeuo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
TEST_MACHINE_ID="${1:-${E2E_TEST_MACHINE_ID:-${TEST_MACHINE_ID:-}}}"
[[ -n "$TEST_MACHINE_ID" ]] || { echo "FATAL: TEST_MACHINE_ID required" >&2; exit 2; }

export EXPIRES_IN_MINUTES=1
export E2E_RUN_TS="expired-$(date -u +%Y%m%dT%H%M%SZ)"
out="$("$ROOT/scripts/e2e/create-production-device-activation-code.sh" "$TEST_MACHINE_ID")"
code="$(printf '%s\n' "$out" | sed -n 's/^ACTIVATION_CODE=//p')"
[[ -n "$code" ]] || { echo "FATAL: failed to mint short-lived code" >&2; exit 2; }

echo "Waiting 90s for code expiry..." >&2
sleep 90
printf 'ACTIVATION_CODE_EXPIRED=%s\n' "$code"
