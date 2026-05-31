#!/usr/bin/env bash
# shellcheck shell=bash
# Deterministic unit tests for canary live-sale verdict and guard logic (no network).

set -Eeuo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
# shellcheck source=../lib/canary-live-sale-verdict.sh
source "${ROOT}/scripts/e2e/lib/canary-live-sale-verdict.sh"

failures=0

assert_eq() {
  local label="$1"
  local got="$2"
  local want="$3"
  if [[ "$got" != "$want" ]]; then
    echo "FAIL  ${label}: got=${got} want=${want}" >&2
    failures=$((failures + 1))
  else
    echo "PASS  ${label}"
  fi
}

assert_fail() {
  local label="$1"
  shift
  if "$@" >/dev/null 2>&1; then
    echo "FAIL  ${label}: expected failure" >&2
    failures=$((failures + 1))
  else
    echo "PASS  ${label}"
  fi
}

assert_ok() {
  local label="$1"
  shift
  if "$@" >/dev/null 2>&1; then
    echo "PASS  ${label}"
  else
    echo "FAIL  ${label}: expected success" >&2
    failures=$((failures + 1))
  fi
}

run_verdict_case() {
  local name="$1"
  local probe_failures="$2"
  local backend_only="$3"
  local simulate="$4"
  local hardware_done="$5"
  local final_status="$6"
  local inv_state="$7"
  local want_family="$8"
  local want_readiness="$9"
  local want_exit="${10}"

  e2e_compute_canary_live_sale_verdicts "$probe_failures" "$backend_only" "$simulate" "$hardware_done" "$final_status" "$inv_state"
  assert_eq "${name}.family" "${E2E_FAMILY_CANARY_VERDICT}" "$want_family"
  assert_eq "${name}.readiness" "${E2E_READINESS_VERDICT}" "$want_readiness"
  assert_eq "${name}.exit" "${E2E_CANARY_EXIT_CODE}" "$want_exit"
}

assert_fail "simulate_production_refused" e2e_canary_refuse_simulated_on_production "https://api.ldtv.dev" true false production
assert_ok "simulate_production_dry_run_allowed" e2e_canary_refuse_simulated_on_production "https://api.ldtv.dev" true true production
assert_ok "simulate_local_allowed" e2e_canary_refuse_simulated_on_production "http://localhost:8080" true false development

assert_fail "cash_only_blocks_qr" e2e_canary_payment_method_allowed qr cash_only true false
assert_fail "cash_only_blocks_card" e2e_canary_payment_method_allowed card cash_only true false
assert_ok "cash_only_allows_cash" e2e_canary_payment_method_allowed cash cash_only true false
assert_fail "live_psp_requires_qr_enabled" e2e_canary_payment_method_allowed qr live_psp true false
assert_ok "live_psp_allows_qr" e2e_canary_payment_method_allowed qr live_psp true true

assert_eq "inventory_delta_pass" "$(e2e_canary_inventory_delta_state 5 4 completed)" "pass"
assert_eq "inventory_delta_fail" "$(e2e_canary_inventory_delta_state 5 5 completed)" "fail"
assert_eq "inventory_delta_skip" "$(e2e_canary_inventory_delta_state 5 null completed)" "skip"

run_verdict_case "simulated_backend_dry_run" 0 true true 0 completed pass FAIL BACKEND-ONLY-NO-MARKET-GO 0
run_verdict_case "simulated_cannot_market_pass" 0 false true 0 completed pass FAIL BACKEND-ONLY-NO-MARKET-GO 0
run_verdict_case "real_market_pass" 0 false false 1 completed pass PASS NO-FLEET-GO 0
run_verdict_case "missing_inventory_delta" 0 false false 1 completed skip FAIL NO-GO 1
run_verdict_case "missing_hardware" 0 false false 0 completed pass FAIL NO-GO 1
run_verdict_case "probe_failures" 2 false false 1 completed pass FAIL NO-GO 1

# Simulated success must never emit fleet GO or family market PASS.
e2e_compute_canary_live_sale_verdicts 0 true true 0 completed pass
if [[ "${E2E_READINESS_VERDICT}" == "GO" || "${E2E_FAMILY_CANARY_VERDICT}" == "PASS" ]]; then
  echo "FAIL  simulated_hardware_cannot_fleet_go" >&2
  failures=$((failures + 1))
else
  echo "PASS  simulated_hardware_cannot_fleet_go"
fi

echo ""
if [[ "$failures" -gt 0 ]]; then
  echo "${failures} canary live-sale test(s) failed" >&2
  exit 1
fi
echo "All canary live-sale verdict tests passed."
