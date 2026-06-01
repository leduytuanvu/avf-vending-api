#!/usr/bin/env bash
# Production destructive E2E gate + Phase 7 runner entry.
#
# Gate only:
#   bash scripts/e2e/production-destructive-e2e-gate.sh [--require-dispense]
#
# Full Phase 7 cash-only E2E (real machine):
#   bash scripts/e2e/production-destructive-e2e-run.sh --execute

set -Eeuo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
# shellcheck source=../../tests/e2e/lib/e2e_common.sh
source "${ROOT}/tests/e2e/lib/e2e_common.sh"
# shellcheck source=../../tests/e2e/lib/e2e_production_destructive_aliases.sh
source "${ROOT}/tests/e2e/lib/e2e_production_destructive_aliases.sh"

export E2E_PRODUCTION_DESTRUCTIVE="${E2E_PRODUCTION_DESTRUCTIVE:-false}"
load_env "${E2E_ENV_FILE:-${ROOT}/tests/e2e/.env.production.destructive.local}"
e2e_apply_production_destructive_aliases
e2e_require_production_destructive_safety_gate

if [[ "${1:-}" == "--require-dispense" ]]; then
  e2e_require_real_dispense_gate
  echo "OK: destructive + real dispense gates satisfied."
  exit 0
fi

if [[ "${1:-}" == "--execute" ]]; then
  APP_ROOT="${APP_ROOT:-${ROOT}/../avf-vending-app}"
  exec bash "${APP_ROOT}/scripts/testing/run-production-cash-only-e2e.sh"
fi

echo "OK: destructive safety gates satisfied."
echo "Phase 7: run avf-vending-app/scripts/testing/run-production-cash-only-e2e.sh (after Phase 6 preflight PASS)."
exit 0
