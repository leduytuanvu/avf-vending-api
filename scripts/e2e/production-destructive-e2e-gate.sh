#!/usr/bin/env bash
# Safety gate for future production destructive E2E (real machine / dispense).
# Source after load_env — does NOT run any tests.
#
# Required for real destructive run:
#   E2E_PRODUCTION_DESTRUCTIVE=true
#   E2E_TARGET=production
#   E2E_ALLOW_WRITES=true
#   E2E_PRODUCTION_WRITE_CONFIRMATION=I_UNDERSTAND_THIS_WRITES_TO_PRODUCTION
#
# Required for real dispense (later):
#   E2E_ALLOW_REAL_DISPENSE=true  (or E2E_EXPECT_REAL_DISPENSE=true)
#   PRODUCTION_LIVE_TEST_CONFIRMATION=I_UNDERSTAND_THIS_CAN_VEND_REAL_PRODUCT

set -Eeuo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
# shellcheck source=../../tests/e2e/lib/e2e_production_destructive_aliases.sh
source "${ROOT}/tests/e2e/lib/e2e_production_destructive_aliases.sh"

e2e_apply_production_destructive_aliases
e2e_require_production_destructive_safety_gate

if [[ "${1:-}" == "--require-dispense" ]]; then
  e2e_require_real_dispense_gate
  echo "OK: production destructive + real dispense gates satisfied (no actions executed)."
else
  echo "OK: production destructive gate satisfied (dispense still blocked until --require-dispense)."
fi
