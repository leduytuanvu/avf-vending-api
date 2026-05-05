#!/usr/bin/env bash
# shellcheck shell=bash
# Vending app QA: optional REST-equivalent scenarios (Phase 5). Field production apps use gRPC/MQTT — see docs.

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib/e2e_common.sh
source "${SCRIPT_DIR}/lib/e2e_common.sh"
e2e_strict_mode

e2e_print_help() {
  cat <<EOF
Usage: ./tests/e2e/run-vending-app-flows.sh [options]

  --rest-equivalent   Run machine REST mirror flows (02, 03, 04, 06, 08).
                      Default without this flag: legacy vending_app_flows.sh if present, else skip.

  Common:
    --fresh-data          Empty run test-data.json
    --reuse-data PATH     Copy prior capture (typically with secrets.private.json containing machineToken
                          or E2E_ACTIVATION_CODE in env for a fresh claim)

  Requires: API at BASE_URL, bash, curl, jq, python3.
  Commerce writes need E2E_ALLOW_WRITES=true.
  Production payment/refund paths require E2E_TARGET=production + E2E_ALLOW_WRITES=true +
  E2E_PRODUCTION_WRITE_CONFIRMATION=I_UNDERSTAND_THIS_WRITES_TO_PRODUCTION and test-data e2eTestMachine=true.

EOF
}

argv=()
REST_EQ="false"
for arg in "$@"; do
  case "$arg" in
    --rest-equivalent) REST_EQ="true" ;;
    *) argv+=("$arg") ;;
  esac
done

e2e_capture_inherited_data_flags
e2e_parse_common_args "${argv[@]}"
load_env
e2e_restore_inherited_data_flags_if_needed

export BASE_URL GRPC_ADDR E2E_TARGET E2E_ALLOW_WRITES E2E_PRODUCTION_WRITE_CONFIRMATION
export E2E_REUSE_DATA E2E_DATA_FILE E2E_CLI_FRESH_DATA
export E2E_ACTIVATION_CODE E2E_SKIP_ACTIVATION_CLAIM E2E_DEVICE_FINGERPRINT_SERIAL E2E_SLOT_INDEX

require_cmd jq curl
e2e_require_python

new_run_dir
e2e_write_run_meta "run-vending-app-flows"

# shellcheck source=lib/e2e_data.sh
source "${SCRIPT_DIR}/lib/e2e_data.sh"
if [[ "${E2E_IN_PARENT:-0}" != "1" ]]; then
  e2e_data_initialize
fi

[[ "${E2E_IN_PARENT:-0}" == "1" ]] || cleanup_trap_register

start_step "vending-app-flows"
ec=0

run_bash_scenario() {
  local path="$1"
  if [[ ! -f "$path" ]]; then
    log_error "scenario missing: ${path}"
    return 1
  fi
  set +e
  bash "$path"
  local c=$?
  set -e
  if [[ "$c" -ne 0 ]]; then
    log_error "$(basename "$path") exited $c"
    return "$c"
  fi
  return 0
}

if [[ "$REST_EQ" == "true" ]]; then
  va_scenarios=(
    "${SCRIPT_DIR}/scenarios/02_machine_activation_bootstrap_rest.sh"
    "${SCRIPT_DIR}/scenarios/03_catalog_media_sync_rest.sh"
    "${SCRIPT_DIR}/scenarios/04_cash_sale_success_rest.sh"
    "${SCRIPT_DIR}/scenarios/06_vend_failure_refund_rest.sh"
    "${SCRIPT_DIR}/scenarios/08_offline_replay_rest.sh"
  )
  for scen in "${va_scenarios[@]}"; do
    if ! run_bash_scenario "$scen"; then
      ec=1
    fi
  done
  if [[ "$ec" -eq 0 ]]; then
    end_step passed "vending app REST-equivalent completed"
  else
    end_step failed "vending app REST-equivalent exit ${ec}"
  fi
else
  SCENARIO="${SCRIPT_DIR}/scenarios/vending_app_flows.sh"
  if [[ -f "$SCENARIO" ]]; then
    set +e
    # shellcheck disable=SC1090
    source "$SCENARIO"
    ec=$?
    set -e
    if [[ "$ec" -eq 0 ]]; then
      end_step passed "vending app flows completed"
    else
      end_step failed "vending app flows exit ${ec}"
    fi
  else
    e2e_skip "vending app: pass --rest-equivalent or add scenarios/vending_app_flows.sh"
    ec=0
    end_step skipped "vending app placeholder"
  fi
fi

va_append_summary() {
  local sm="$1"
  local vf="${E2E_RUN_DIR}/reports/va-rest-results.jsonl"
  [[ -f "$vf" ]] && [[ -s "$vf" ]] || return 0
  {
    echo ""
    echo "## Vending app REST-equivalent results"
    echo ""
    echo "Structured log: \`reports/va-rest-results.jsonl\`. **Note:** production field apps use **gRPC + MQTT**; REST here is **lab/QA** coverage only."
    echo ""
    jq -s -r '
      group_by(.flow_id)[]
      | ("### " + (.[0].flow_id) + "\n\n"
        + "| step | endpoint | status | HTTP | note |\n"
        + "|------|----------|--------|------|------|\n"
        + (map("| \(.step) | `\(.endpoint)` | **\(.status)** | \(.httpStatus) | \(.message) |\n") | join("")))
    ' "$vf"
    echo ""
  } >>"$sm"
}

if [[ "${E2E_IN_PARENT:-0}" != "1" ]]; then
  # shellcheck source=lib/e2e_report.sh
  source "${SCRIPT_DIR}/lib/e2e_report.sh"
  e2e_finalize_reports "${ec}"
  fr=$?
  [[ "${fr}" -ne 0 ]] && ec="${fr}"
  sm="${E2E_RUN_DIR}/reports/summary.md"
  [[ -f "$sm" ]] && [[ "$REST_EQ" == "true" ]] && va_append_summary "$sm"
fi

[[ "${E2E_IN_PARENT:-0}" == "1" ]] && exit "${ec}"
exit "${ec}"
