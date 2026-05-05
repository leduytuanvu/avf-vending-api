#!/usr/bin/env bash
# shellcheck shell=bash
# Full local orchestration: preflight → REST → admin → vending → gRPC → MQTT → report.

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
  set +e
  E2E_IN_PARENT=1 \
    E2E_RUN_DIR="${E2E_RUN_DIR}" \
    E2E_REUSE_DATA="${E2E_REUSE_DATA}" \
    E2E_DATA_FILE="${E2E_DATA_FILE:-}" \
    E2E_CLI_FRESH_DATA="${E2E_CLI_FRESH_DATA}" \
    bash "${script}" "${E2E_EXTRA_ARGS[@]+"${E2E_EXTRA_ARGS[@]}"}"
  local ec=$?
  set -e
  if [[ "$ec" -ne 0 ]]; then
    OVERALL=1
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
invoke_child "${SCRIPT_DIR}/run-web-admin-flows.sh"
invoke_child "${SCRIPT_DIR}/run-vending-app-flows.sh"
invoke_child "${SCRIPT_DIR}/run-grpc-local.sh"
invoke_child "${SCRIPT_DIR}/run-mqtt-local.sh"

# shellcheck source=lib/e2e_report.sh
source "${SCRIPT_DIR}/lib/e2e_report.sh"
e2e_finalize_reports "${OVERALL}"

exit "${OVERALL}"
