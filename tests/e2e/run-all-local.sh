#!/usr/bin/env bash
# shellcheck shell=bash
# Full local orchestration: preflight → REST → admin (--full) → vending (REST-equivalent) → gRPC → MQTT → Phase 8 → report.

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib/e2e_common.sh
source "${SCRIPT_DIR}/lib/e2e_common.sh"
e2e_strict_mode

e2e_print_help() {
  e2e_print_help_run_all
}

e2e_capture_inherited_data_flags
e2e_parse_common_args "$@"
load_env
e2e_restore_inherited_data_flags_if_needed

# shellcheck disable=SC2317,SC3046
eval "$(declare -f end_step | sed '1s/end_step/_e2e_original_end_step/')"
end_step() {
  local _st="$1"
  shift || true
  _e2e_original_end_step "$_st" "$@"
  if [[ "$_st" == "failed" ]]; then
    log_error "E2E run directory (failed step): ${E2E_RUN_DIR:-unknown}"
  fi
}

require_cmd jq curl bash python3

new_run_dir
e2e_write_run_meta "run-all-local"

# shellcheck source=lib/e2e_data.sh
source "${SCRIPT_DIR}/lib/e2e_data.sh"
e2e_data_initialize

cleanup_trap_register

# shellcheck source=lib/e2e_http.sh
source "${SCRIPT_DIR}/lib/e2e_http.sh"

OVERALL=0

invoke_child() {
  local script="$1"
  shift
  set +e
  E2E_IN_PARENT=1 \
    E2E_RUN_DIR="${E2E_RUN_DIR}" \
    E2E_REUSE_DATA="${E2E_REUSE_DATA}" \
    E2E_DATA_FILE="${E2E_DATA_FILE:-}" \
    E2E_CLI_FRESH_DATA="${E2E_CLI_FRESH_DATA}" \
    bash "${script}" "$@" ${E2E_EXTRA_ARGS[@]+"${E2E_EXTRA_ARGS[@]}"}
  local ec=$?
  set -e
  if [[ "$ec" -ne 0 ]]; then
    OVERALL=1
    log_error "Child script failed (exit ${ec}): ${script}; artifacts: ${E2E_RUN_DIR}"
  fi
}

start_step "preflight"
SCENARIO_PREFLIGHT="${SCRIPT_DIR}/scenarios/00_preflight.sh"
if [[ -f "$SCENARIO_PREFLIGHT" ]]; then
  set +e
  # shellcheck disable=SC1090
  source "$SCENARIO_PREFLIGHT"
  ec=$?
  set -e
  if [[ "$ec" -eq 0 ]]; then
    end_step passed "preflight scenario completed"
  else
    end_step failed "preflight exit ${ec}"
    OVERALL=1
  fi
else
  e2e_skip "preflight scenario not implemented (${SCENARIO_PREFLIGHT} missing)"
  end_step skipped "preflight placeholder"
fi

invoke_child "${SCRIPT_DIR}/run-rest-local.sh"
invoke_child "${SCRIPT_DIR}/run-web-admin-flows.sh" --full
invoke_child "${SCRIPT_DIR}/run-vending-app-flows.sh" --rest-equivalent
invoke_child "${SCRIPT_DIR}/run-grpc-local.sh"
invoke_child "${SCRIPT_DIR}/run-mqtt-local.sh"

# --- Phase 8: end-to-end narratives + structured reports (40–47) ---
PHASE8_LIST=(
  40_e2e_first_boot.sh
  41_e2e_cash_sale_success.sh
  42_e2e_qr_payment_success_mock.sh
  43_e2e_vend_failure_refund.sh
  44_e2e_offline_replay.sh
  45_e2e_remote_command_ack.sh
  46_e2e_inventory_restock_adjustment.sh
  47_e2e_reporting_audit.sh
)
for _p8 in "${PHASE8_LIST[@]}"; do
  _path="${SCRIPT_DIR}/scenarios/${_p8}"
  if [[ ! -f "$_path" ]]; then
    log_error "Phase 8 scenario missing: ${_path}"
    OVERALL=1
    continue
  fi
  set +e
  E2E_IN_PARENT=1 \
    E2E_RUN_DIR="${E2E_RUN_DIR}" \
    E2E_REUSE_DATA="${E2E_REUSE_DATA}" \
    E2E_DATA_FILE="${E2E_DATA_FILE:-}" \
    E2E_CLI_FRESH_DATA="${E2E_CLI_FRESH_DATA}" \
    bash "${_path}" ${E2E_EXTRA_ARGS[@]+"${E2E_EXTRA_ARGS[@]}"}
  _ec=$?
  set -e
  if [[ "$_ec" -ne 0 ]]; then
    OVERALL=1
    log_error "Phase 8 scenario failed (exit ${_ec}): ${_p8}; artifacts: ${E2E_RUN_DIR}"
  fi
done

# shellcheck source=lib/e2e_report.sh
source "${SCRIPT_DIR}/lib/e2e_report.sh"
e2e_finalize_reports "${OVERALL}"
fr=$?
[[ "${fr}" -ne 0 ]] && OVERALL="${fr}"
exit "${OVERALL}"
