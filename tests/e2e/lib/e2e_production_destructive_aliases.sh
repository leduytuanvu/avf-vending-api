#!/usr/bin/env bash
# shellcheck shell=bash
# Maps operator-facing E2E_* aliases to canonical harness names (see .env.production.destructive.example).

e2e_apply_production_destructive_aliases() {
  if [[ -n "${E2E_ENV:-}" && -z "${E2E_TARGET:-}" ]]; then
    export E2E_TARGET="${E2E_ENV}"
  fi
  if [[ -n "${E2E_BASE_URL:-}" ]]; then
    export BASE_URL="${E2E_BASE_URL}"
  fi
  if [[ -n "${E2E_TEST_MACHINE_ID:-}" && -z "${TEST_MACHINE_ID:-}" ]]; then
    export TEST_MACHINE_ID="${E2E_TEST_MACHINE_ID}"
  fi
  if [[ -z "${TEST_MACHINE_ID:-}" && -n "${MACHINE_ID:-}" ]]; then
    export TEST_MACHINE_ID="${MACHINE_ID}"
  fi
  if [[ -z "${MACHINE_ACCESS_TOKEN:-}" && -n "${MACHINE_TOKEN:-}" ]]; then
    export MACHINE_ACCESS_TOKEN="${MACHINE_TOKEN}"
  fi
  if [[ -n "${E2E_TEST_SLOT_CODE:-}" && -z "${TEST_SLOT_CODE:-}" ]]; then
    export TEST_SLOT_CODE="${E2E_TEST_SLOT_CODE}"
  fi
  if [[ -z "${TEST_SLOT_CODE:-}" && -n "${E2E_TEST_SLOT_CODE:-}" ]]; then
    export TEST_SLOT_CODE="${E2E_TEST_SLOT_CODE}"
  fi
  if [[ -n "${E2E_PAYMENT_METHOD:-}" && -z "${TEST_PAYMENT_METHOD:-}" ]]; then
    export TEST_PAYMENT_METHOD="${E2E_PAYMENT_METHOD}"
  fi
  if [[ -n "${E2E_ANDROID_DEVICE_ID:-}" && -z "${ADB_SERIAL:-}" ]]; then
    export ADB_SERIAL="${E2E_ANDROID_DEVICE_ID}"
  fi
  if [[ -n "${E2E_TEST_PRODUCT_ID:-}" && -z "${TEST_PRODUCT_ID:-}" ]]; then
    export TEST_PRODUCT_ID="${E2E_TEST_PRODUCT_ID}"
  fi
  if [[ "${E2E_EXPECT_REAL_DISPENSE:-false}" == "true" ]]; then
    export E2E_ALLOW_REAL_DISPENSE=true
  fi
  if [[ -n "${E2E_MACHINE_TYPE:-}" && -z "${PRODUCTION_E2E_MACHINE_FAMILY:-}" ]]; then
    export PRODUCTION_E2E_MACHINE_FAMILY="${E2E_MACHINE_TYPE,,}"
  fi
}

# Phase 6+ real-machine destructive runner gate (prep scripts must NOT call this).
e2e_require_production_destructive_safety_gate() {
  if [[ "${E2E_PRODUCTION_DESTRUCTIVE:-false}" != "true" ]]; then
    echo "FATAL: production destructive E2E requires E2E_PRODUCTION_DESTRUCTIVE=true" >&2
    exit 2
  fi
  if [[ "${E2E_TARGET:-}" != "production" ]]; then
    echo "FATAL: E2E_TARGET must be production (or set E2E_ENV=production)" >&2
    exit 2
  fi
  if [[ "${E2E_ALLOW_WRITES:-false}" != "true" ]]; then
    echo "FATAL: destructive E2E requires E2E_ALLOW_WRITES=true" >&2
    exit 2
  fi
  if [[ "${E2E_PRODUCTION_WRITE_CONFIRMATION:-}" != "I_UNDERSTAND_THIS_WRITES_TO_PRODUCTION" ]]; then
    echo "FATAL: missing E2E_PRODUCTION_WRITE_CONFIRMATION" >&2
    exit 2
  fi
  if [[ "${E2E_ALLOW_DESTRUCTIVE:-false}" == "true" ]]; then
    if [[ "${E2E_PRODUCTION_DESTRUCTIVE_CONFIRMATION:-}" != "I_UNDERSTAND_DB_WILL_BE_RESET_AFTER_TEST" ]]; then
      echo "FATAL: E2E_ALLOW_DESTRUCTIVE=true requires E2E_PRODUCTION_DESTRUCTIVE_CONFIRMATION" >&2
      exit 2
    fi
  fi
}

e2e_require_real_dispense_gate() {
  if [[ "${E2E_ALLOW_REAL_DISPENSE:-false}" != "true" ]]; then
    echo "FATAL: real dispense requires E2E_ALLOW_REAL_DISPENSE=true (or E2E_EXPECT_REAL_DISPENSE=true)" >&2
    exit 2
  fi
  if [[ "${E2E_PRODUCTION_DESTRUCTIVE:-false}" != "true" ]]; then
    echo "FATAL: real dispense also requires E2E_PRODUCTION_DESTRUCTIVE=true" >&2
    exit 2
  fi
  if [[ "${PRODUCTION_LIVE_TEST_CONFIRMATION:-}" != "I_UNDERSTAND_THIS_CAN_VEND_REAL_PRODUCT" ]] &&
    [[ "${PRODUCTION_LIVE_TEST_CONFIRMATION:-}" != "I_UNDERSTAND_THIS_CAN_CHARGE_AND_VEND" ]]; then
    echo "FATAL: real dispense requires PRODUCTION_LIVE_TEST_CONFIRMATION" >&2
    exit 2
  fi
}

# Preflight-only: refuse if operator accidentally enabled irreversible flags.
e2e_refuse_irreversible_flags_for_preflight() {
  local blocked=0
  for flag in E2E_ALLOW_REAL_DISPENSE E2E_ALLOW_REAL_PAYMENT E2E_ALLOW_REAL_MACHINE_COMMANDS; do
    if [[ "${!flag:-false}" == "true" ]]; then
      echo "REFUSED (preflight): ${flag}=true — Phase 6 is read-only preparation only." >&2
      blocked=1
    fi
  done
  if [[ "${E2E_EXPECT_REAL_DISPENSE:-false}" == "true" ]]; then
    echo "REFUSED (preflight): E2E_EXPECT_REAL_DISPENSE=true — unset for Phase 6 preflight." >&2
    blocked=1
  fi
  if [[ "$blocked" -ne 0 ]]; then
    exit 2
  fi
}
