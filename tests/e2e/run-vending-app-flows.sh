#!/usr/bin/env bash
# shellcheck shell=bash
# Vending app (machine) flow scenarios — REST/gRPC/MQTT mix lives in scenarios.

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib/e2e_common.sh
source "${SCRIPT_DIR}/lib/e2e_common.sh"
e2e_strict_mode

e2e_print_help() {
  cat <<EOF
Usage: ./tests/e2e/run-vending-app-flows.sh [options]

  --fresh-data / --reuse-data PATH / -h
EOF
}

e2e_capture_inherited_data_flags
e2e_parse_common_args "$@"
load_env
e2e_restore_inherited_data_flags_if_needed

require_cmd jq curl

new_run_dir
e2e_write_run_meta "run-vending-app-flows"

# shellcheck source=lib/e2e_data.sh
source "${SCRIPT_DIR}/lib/e2e_data.sh"
if [[ "${E2E_IN_PARENT:-0}" != "1" ]]; then
  e2e_data_initialize
fi

[[ "${E2E_IN_PARENT:-0}" == "1" ]] || cleanup_trap_register

start_step "vending-app-flows"
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
    [[ "${E2E_IN_PARENT:-0}" == "1" ]] && exit 1
    exit 1
  fi
else
  e2e_skip "vending app scenarios not implemented (${SCENARIO} missing)"
  end_step skipped "vending app placeholder"
fi

[[ "${E2E_IN_PARENT:-0}" == "1" ]] && exit 0
exit 0
