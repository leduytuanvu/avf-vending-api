#!/usr/bin/env bash
# shellcheck shell=bash
# Verdict helpers for production-canary-live-sale.sh (unit-testable).

readonly E2E_PRODUCTION_CANARY_BASE_URL="https://api.ldtv.dev"

e2e_is_production_canary_base_url() {
  local base_url="${1%/}"
  [[ "$base_url" == "$E2E_PRODUCTION_CANARY_BASE_URL" ]]
}

e2e_canary_refuse_simulated_on_production() {
  local base_url="$1"
  local simulate_hardware="${2:-false}"
  local backend_only_dry_run="${3:-false}"
  local app_env="${4:-}"

  if [[ "${simulate_hardware,,}" != "true" ]]; then
    return 0
  fi
  if [[ "${backend_only_dry_run,,}" == "true" ]]; then
    return 0
  fi
  if e2e_is_production_canary_base_url "$base_url" || [[ "${app_env,,}" == "production" ]]; then
    echo "SIMULATE_HARDWARE_VEND=true on production requires BACKEND_ONLY_DRY_RUN=true (refusing simulated market canary)"
    return 1
  fi
  return 0
}

e2e_canary_payment_method_allowed() {
  local payment_method="$1"
  local pay_mode="$2"
  local cash_ok="$3"
  local qr_ok="$4"

  payment_method="${payment_method,,}"
  pay_mode="${pay_mode,,}"

  case "$payment_method" in
    cash)
      if [[ "$cash_ok" != "true" ]]; then
        echo "payment config: cash_enabled=false (payment_mode=${pay_mode})"
        return 1
      fi
      ;;
    qr|card)
      if [[ "$pay_mode" == "cash_only" ]]; then
        echo "cash_only production forbids TEST_PAYMENT_METHOD=${payment_method} — use cash only"
        return 1
      fi
      if [[ "$pay_mode" != "live_psp" ]]; then
        echo "qr/card requires bootstrap payment_mode=live_psp (got ${pay_mode:-empty})"
        return 1
      fi
      if [[ "$qr_ok" != "true" ]]; then
        echo "payment config: qr_card_enabled=false (payment_mode=${pay_mode})"
        return 1
      fi
      ;;
    *)
      echo "TEST_PAYMENT_METHOD must be cash, qr, or card"
      return 1
      ;;
  esac
  return 0
}

e2e_canary_order_status_is_market_complete() {
  local status="$1"
  status="${status,,}"
  [[ "$status" == "completed" || "$status" == "success" ]]
}

e2e_canary_vend_state_is_hardware_complete() {
  local vend_state="$1"
  vend_state="${vend_state,,}"
  [[ "$vend_state" == "completed" || "$vend_state" == "success" ]]
}

# inventory_delta_state: pass | fail | skip
e2e_canary_inventory_delta_state() {
  local before="$1"
  local after="$2"
  local final_status="$3"

  if [[ -z "$before" || "$before" == "null" || -z "$after" || "$after" == "null" ]]; then
    echo "skip"
    return 0
  fi
  if e2e_canary_order_status_is_market_complete "$final_status"; then
    if [[ "$after" -lt "$before" ]]; then
      echo "pass"
      return 0
    fi
    echo "fail"
    return 0
  fi
  echo "pass"
}

# Sets E2E_FAMILY_CANARY_VERDICT, E2E_READINESS_VERDICT, E2E_CANARY_EXIT_CODE.
e2e_compute_canary_live_sale_verdicts() {
  local probe_failures="${1:-0}"
  local backend_only_dry_run="${2:-false}"
  local simulate_hardware="${3:-false}"
  local hardware_vend_completed="${4:-0}"
  local final_status="${5:-}"
  local inventory_delta_state="${6:-skip}"

  if [[ "$probe_failures" -gt 0 ]]; then
    E2E_FAMILY_CANARY_VERDICT="FAIL"
    if [[ "${backend_only_dry_run,,}" == "true" ]]; then
      E2E_READINESS_VERDICT="NO-GO"
    else
      E2E_READINESS_VERDICT="NO-GO"
    fi
    E2E_CANARY_EXIT_CODE=1
    return 0
  fi

  if [[ "${backend_only_dry_run,,}" == "true" || "${simulate_hardware,,}" == "true" ]]; then
    E2E_FAMILY_CANARY_VERDICT="FAIL"
    E2E_READINESS_VERDICT="BACKEND-ONLY-NO-MARKET-GO"
    E2E_CANARY_EXIT_CODE=0
    return 0
  fi

  local market_ok=1
  if [[ "$hardware_vend_completed" -ne 1 ]]; then
    market_ok=0
  fi
  if ! e2e_canary_order_status_is_market_complete "$final_status"; then
    market_ok=0
  fi
  if [[ "$inventory_delta_state" != "pass" ]]; then
    market_ok=0
  fi

  if [[ "$market_ok" -eq 1 ]]; then
    E2E_FAMILY_CANARY_VERDICT="PASS"
    E2E_READINESS_VERDICT="NO-FLEET-GO"
    E2E_CANARY_EXIT_CODE=0
    return 0
  fi

  E2E_FAMILY_CANARY_VERDICT="FAIL"
  E2E_READINESS_VERDICT="NO-GO"
  E2E_CANARY_EXIT_CODE=1
}
