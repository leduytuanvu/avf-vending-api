#!/usr/bin/env bash
# shellcheck shell=bash
# Deterministic unit tests for readonly-smoke verdict logic (no network).

set -Eeuo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
# shellcheck source=../lib/readonly-smoke-verdict.sh
source "${ROOT}/scripts/e2e/lib/readonly-smoke-verdict.sh"

FIXTURES="${ROOT}/scripts/e2e/tests/fixtures/readonly-smoke-verdict"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

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

run_verdict_case() {
  local name="$1"
  local strict="$2"
  local probe_failures="$3"
  local cash_only_ok="$4"
  local want_smoke="$5"
  local want_readiness="$6"
  local want_exit="$7"
  local probes_tsv="${TMP}/${name}.tsv"
  cp "${FIXTURES}/${name}.probes.tsv" "$probes_tsv"
  e2e_compute_readonly_smoke_verdicts "$strict" "$probes_tsv" "$probe_failures" "$cash_only_ok"
  assert_eq "${name}.smoke" "${E2E_SMOKE_VERDICT}" "$want_smoke"
  assert_eq "${name}.readiness" "${E2E_READINESS_VERDICT}" "$want_readiness"
  assert_eq "${name}.exit" "${E2E_EXIT_CODE}" "$want_exit"
}

assert_eq "strict_default_ldtv" "$(e2e_strict_canary_default_for_base_url 'https://api.ldtv.dev')" "true"
assert_eq "strict_default_local" "$(PRODUCTION_SMOKE_STRICT_CANARY=false e2e_strict_canary_default_for_base_url 'http://localhost:8080')" "false"
assert_eq "strict_default_localhost_ldtv_override" "$(PRODUCTION_SMOKE_STRICT_CANARY=true e2e_strict_canary_default_for_base_url 'http://localhost:8080')" "true"

run_verdict_case "missing-payment-runtime" false 0 1 "PASS_DEV_ONLY" "NO-GO" 0
run_verdict_case "missing-payment-runtime" true 1 1 "FAIL" "NO-GO" 1
run_verdict_case "skipped-strict-probes" false 0 1 "PASS_DEV_ONLY" "NO-GO" 0
run_verdict_case "skipped-strict-probes" true 0 1 "FAIL" "NO-GO" 1
run_verdict_case "cash-only-full-pass" false 0 0 "PASS" "GO-CANARY-ONLY" 0
run_verdict_case "cash-only-full-pass" true 0 0 "PASS" "GO-CANARY-ONLY" 0
run_verdict_case "cash-only-missing-grpc" true 3 1 "FAIL" "NO-GO" 1

cash_body="${FIXTURES}/cash-only-version.json"
bad_body="${FIXTURES}/missing-payment-runtime-version.json"
if e2e_validate_cash_only_payment_runtime "$cash_body" true >/dev/null 2>&1; then
  echo "PASS  cash_only_contract.valid"
else
  echo "FAIL  cash_only_contract.valid" >&2
  failures=$((failures + 1))
fi
if e2e_validate_cash_only_payment_runtime "$bad_body" true >/dev/null 2>&1; then
  echo "FAIL  cash_only_contract.missing_should_fail" >&2
  failures=$((failures + 1))
else
  echo "PASS  cash_only_contract.missing_should_fail"
fi

echo ""
if [[ "$failures" -gt 0 ]]; then
  echo "${failures} verdict test(s) failed" >&2
  exit 1
fi
echo "All readonly-smoke verdict tests passed."
