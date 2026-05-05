#!/usr/bin/env bash
# shellcheck shell=bash
# MQTT publish/subscribe smoke (mosquitto clients).

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib/e2e_common.sh
source "${SCRIPT_DIR}/lib/e2e_common.sh"
e2e_strict_mode

e2e_print_help() {
  cat <<EOF
Usage: ./tests/e2e/run-mqtt-local.sh [options]

Environment:
  MQTT_HOST, MQTT_PORT, MQTT_USERNAME, MQTT_PASSWORD

  --fresh-data / --reuse-data PATH / -h
EOF
}

e2e_capture_inherited_data_flags
e2e_parse_common_args "$@"
load_env
e2e_restore_inherited_data_flags_if_needed

require_cmd jq

new_run_dir
e2e_write_run_meta "run-mqtt-local"

# shellcheck source=lib/e2e_data.sh
source "${SCRIPT_DIR}/lib/e2e_data.sh"
if [[ "${E2E_IN_PARENT:-0}" != "1" ]]; then
  e2e_data_initialize
fi

[[ "${E2E_IN_PARENT:-0}" == "1" ]] || cleanup_trap_register

start_step "mqtt-local-suite"
if ! command -v mosquitto_pub >/dev/null 2>&1 || ! command -v mosquitto_sub >/dev/null 2>&1; then
  e2e_skip "mosquitto_pub / mosquitto_sub not installed"
  end_step skipped "mosquitto missing"
  [[ "${E2E_IN_PARENT:-0}" == "1" ]] && exit 0
  exit 0
fi

# shellcheck source=lib/e2e_mqtt.sh
source "${SCRIPT_DIR}/lib/e2e_mqtt.sh"

SCENARIO="${SCRIPT_DIR}/scenarios/mqtt_local.sh"
if [[ -f "$SCENARIO" ]]; then
  set +e
  # shellcheck disable=SC1090
  source "$SCENARIO"
  ec=$?
  set -e
  if [[ "$ec" -eq 0 ]]; then
    end_step passed "MQTT scenarios completed"
  else
    end_step failed "MQTT scenario exit ${ec}"
    [[ "${E2E_IN_PARENT:-0}" == "1" ]] && exit 1
    exit 1
  fi
else
  e2e_skip "MQTT scenarios not implemented (${SCENARIO} missing)"
  end_step skipped "MQTT placeholder"
fi

[[ "${E2E_IN_PARENT:-0}" == "1" ]] && exit 0
exit 0
