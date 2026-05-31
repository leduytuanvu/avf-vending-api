#!/usr/bin/env bash
# shellcheck shell=bash
# Verdict helpers for production-readonly-smoke.sh (unit-testable).

readonly E2E_STRICT_CANARY_PROBES=(
  version.payment_runtime
  admin.auth
  grpc.bootstrap
  grpc.bootstrap.payment_methods
  grpc.catalog
  grpc.media_manifest
  grpc.inventory
  grpc.planogram
)

e2e_strict_canary_default_for_base_url() {
  local base_url="${1%/}"
  if [[ "${PRODUCTION_SMOKE_STRICT_CANARY:-}" == "true" ]]; then
    echo true
    return 0
  fi
  if [[ "${PRODUCTION_SMOKE_STRICT_CANARY:-}" == "false" ]]; then
    echo false
    return 0
  fi
  if [[ "$base_url" == "https://api.ldtv.dev" ]]; then
    echo true
    return 0
  fi
  echo false
}

e2e_is_strict_canary_probe() {
  local name="$1"
  local probe
  for probe in "${E2E_STRICT_CANARY_PROBES[@]}"; do
    [[ "$probe" == "$name" ]] && return 0
  done
  return 1
}

e2e_probe_outcome_from_tsv() {
  local probes_tsv="$1"
  local probe_name="$2"
  local line outcome
  line="$(grep -F "${probe_name}|" "$probes_tsv" 2>/dev/null | tail -n 1 || true)"
  if [[ -z "$line" ]]; then
    echo "MISSING"
    return 0
  fi
  IFS='|' read -r _ outcome _ _ <<<"$line"
  echo "${outcome:-MISSING}"
}

e2e_validate_cash_only_payment_runtime() {
  local body_file="$1"
  local strict="${2:-false}"
  local mode cash card_qr reason provider_status

  if [[ ! -f "$body_file" ]]; then
    echo "version body missing"
    return 1
  fi
  if ! jq -e '.payment_runtime // .paymentRuntime' "$body_file" >/dev/null 2>&1; then
    echo "payment_runtime absent"
    return 1
  fi

  mode="$(jq -r '.payment_runtime.payment_mode // .paymentRuntime.paymentMode // empty' "$body_file")"

  if [[ "$strict" == "true" && "$mode" != "cash_only" ]]; then
    echo "strict canary requires payment_mode=cash_only (got ${mode:-empty})"
    return 1
  fi
  if [[ "$mode" == "cash_only" || "$strict" == "true" ]]; then
    if [[ "$mode" != "cash_only" ]]; then
      echo "payment_mode must be cash_only (got ${mode:-empty})"
      return 1
    fi
    if ! jq -e '(.payment_runtime.cash_allowed_by_deployment // .paymentRuntime.cashAllowedByDeployment) == true' "$body_file" >/dev/null 2>&1; then
      echo "cash_allowed_by_deployment must be true"
      return 1
    fi
    if ! jq -e '((.payment_runtime.card_qr_sessions_available // .paymentRuntime.cardQrSessionsAvailable) | not)' "$body_file" >/dev/null 2>&1; then
      echo "card_qr_sessions_available must be false"
      return 1
    fi
    reason="$(jq -r '.payment_runtime.qr_card_unavailable_reason // .paymentRuntime.qrCardUnavailableReason // empty' "$body_file")"
    provider_status="$(jq -r '.payment_runtime.card_qr_provider_status // .paymentRuntime.cardQrProviderStatus // empty' "$body_file")"
    if [[ -z "$reason" && "$provider_status" != "unavailable" && "$provider_status" != "placeholder" ]]; then
      echo "qr_card_unavailable_reason empty and provider status not unavailable/placeholder"
      return 1
    fi
  fi
  return 0
}

# Sets E2E_SMOKE_VERDICT, E2E_READINESS_VERDICT, E2E_EXIT_CODE.
e2e_compute_readonly_smoke_verdicts() {
  local strict_mode="$1"
  local probes_tsv="$2"
  local probe_failures="${3:-0}"
  local cash_only_ok="${4:-0}"

  local strict_skip=0 strict_fail=0 strict_missing=0 probe outcome

  for probe in "${E2E_STRICT_CANARY_PROBES[@]}"; do
    outcome="$(e2e_probe_outcome_from_tsv "$probes_tsv" "$probe")"
    case "$outcome" in
      PASS) ;;
      SKIP) strict_skip=$((strict_skip + 1)) ;;
      FAIL) strict_fail=$((strict_fail + 1)) ;;
      *) strict_missing=$((strict_missing + 1)) ;;
    esac
  done

  if [[ "$probe_failures" -gt 0 || "$strict_fail" -gt 0 ]]; then
    E2E_SMOKE_VERDICT="FAIL"
    E2E_READINESS_VERDICT="NO-GO"
    E2E_EXIT_CODE=1
    return 0
  fi

  if [[ "$strict_mode" == "true" ]]; then
    if [[ "$strict_skip" -gt 0 || "$strict_missing" -gt 0 || "$cash_only_ok" -ne 0 ]]; then
      E2E_SMOKE_VERDICT="FAIL"
      E2E_READINESS_VERDICT="NO-GO"
      E2E_EXIT_CODE=1
      return 0
    fi
    E2E_SMOKE_VERDICT="PASS"
    E2E_READINESS_VERDICT="GO-CANARY-ONLY"
    E2E_EXIT_CODE=0
    return 0
  fi

  if [[ "$strict_skip" -gt 0 || "$strict_missing" -gt 0 ]]; then
    E2E_SMOKE_VERDICT="PASS_DEV_ONLY"
    E2E_READINESS_VERDICT="NO-GO"
    E2E_EXIT_CODE=0
    return 0
  fi

  if [[ "$cash_only_ok" -ne 0 ]]; then
    E2E_SMOKE_VERDICT="PASS"
    E2E_READINESS_VERDICT="NO-GO"
    E2E_EXIT_CODE=0
    return 0
  fi

  E2E_SMOKE_VERDICT="PASS"
  E2E_READINESS_VERDICT="GO-CANARY-ONLY"
  E2E_EXIT_CODE=0
}
