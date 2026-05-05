#!/usr/bin/env bash
# shellcheck shell=bash
# Phase 7: MQTT connect, telemetry, command receive + ACK (mosquitto clients).

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib/e2e_common.sh
source "${SCRIPT_DIR}/lib/e2e_common.sh"
e2e_strict_mode

e2e_print_help() {
  cat <<EOF
Usage: ./tests/e2e/run-mqtt-local.sh [options]

Phase 7 MQTT contract smoke (see docs/api/mqtt-contract.md).

Environment (tests/e2e/.env):
  MQTT_HOST, MQTT_PORT (default port 1883)
  MQTT_USERNAME, MQTT_PASSWORD
  MQTT_CLIENT_ID (default auto)
  MQTT_USE_TLS=false, MQTT_CA_CERT (optional)
  MQTT_MACHINE_ID (or machineId in test-data)
  MQTT_TOPIC_PREFIX (default avf/devices)
  MQTT_TOPIC_LAYOUT=legacy|enterprise (default legacy)
  Overrides: MQTT_TOPIC_TELEMETRY, MQTT_TOPIC_COMMANDS, MQTT_TOPIC_COMMAND_ACK, MQTT_TOPIC_EVENTS
  E2E_TARGET=production + e2eTestMachine + E2E_MQTT_COMMAND_TEST_ACK=I_UNDERSTAND_MQTT_COMMAND_TEST_ACK for command test

  --fresh-data / --reuse-data PATH / -h

Artifacts: .e2e-runs/run-*/mqtt/*.log, *.publish.json, reports/mqtt-contract-results.jsonl, mqtt-contract-summary.md
EOF
}

e2e_capture_inherited_data_flags
e2e_parse_common_args "$@"
load_env
e2e_restore_inherited_data_flags_if_needed

: "${MQTT_USE_TLS:=false}"
: "${MQTT_CLIENT_ID:=e2e-mqtt-$(date +%s)-${RANDOM}}"
export MQTT_HOST MQTT_PORT MQTT_USERNAME MQTT_PASSWORD MQTT_CLIENT_ID MQTT_USE_TLS MQTT_CA_CERT
export MQTT_TOPIC_PREFIX MQTT_TOPIC_LAYOUT MQTT_MACHINE_ID
export MQTT_TOPIC_TELEMETRY MQTT_TOPIC_COMMANDS MQTT_TOPIC_COMMAND_ACK MQTT_TOPIC_EVENTS
export E2E_TARGET

require_cmd jq bash

new_run_dir
e2e_write_run_meta "run-mqtt-local"

# shellcheck source=lib/e2e_data.sh
source "${SCRIPT_DIR}/lib/e2e_data.sh"
if [[ "${E2E_IN_PARENT:-0}" != "1" ]]; then
  e2e_data_initialize
fi

[[ "${E2E_IN_PARENT:-0}" == "1" ]] || cleanup_trap_register

start_step "mqtt-local-suite"
ec=0

if ! command -v mosquitto_pub >/dev/null 2>&1 || ! command -v mosquitto_sub >/dev/null 2>&1; then
  e2e_skip "mosquitto_pub / mosquitto_sub not installed"
  end_step skipped "mosquitto missing"
  [[ "${E2E_IN_PARENT:-0}" == "1" ]] && exit 0
  exit 0
fi

# shellcheck source=lib/e2e_mqtt.sh
source "${SCRIPT_DIR}/lib/e2e_mqtt.sh"

if ! e2e_mqtt_tcp_open; then
  log_error "MQTT broker not reachable at ${MQTT_HOST}:${MQTT_PORT:-1883}. Start mosquitto/EMQX locally; see docs/testing/e2e-troubleshooting.md#mqtt-phase-7-broker-unreachable."
  end_step failed "MQTT broker unreachable"
  ec=1
  [[ "${E2E_IN_PARENT:-0}" == "1" ]] && exit "${ec}"
  # shellcheck source=lib/e2e_report.sh
  source "${SCRIPT_DIR}/lib/e2e_report.sh"
  e2e_finalize_reports "${ec}"
  exit "${ec}"
fi

run_mqtt_scenario() {
  local path="$1"
  if [[ ! -f "$path" ]]; then
    log_error "missing scenario ${path}"
    return 1
  fi
  set +e
  bash "$path"
  local c=$?
  set -e
  if [[ "$c" -ne 0 ]]; then
    log_error "$(basename "$path") exited ${c}"
    return "$c"
  fi
  return 0
}

mqtt_scenarios=(
  "${SCRIPT_DIR}/scenarios/30_mqtt_connect.sh"
  "${SCRIPT_DIR}/scenarios/31_mqtt_telemetry_publish.sh"
  "${SCRIPT_DIR}/scenarios/32_mqtt_command_ack.sh"
)

for scen in "${mqtt_scenarios[@]}"; do
  if ! run_mqtt_scenario "$scen"; then
    ec=1
  fi
done

if [[ "$ec" -eq 0 ]]; then
  end_step passed "MQTT Phase 7 scenarios completed"
else
  end_step failed "MQTT scenarios exit ${ec}"
fi

mqtt_write_contract_summary() {
  local out="${E2E_RUN_DIR}/reports/mqtt-contract-summary.md"
  local jl="${E2E_RUN_DIR}/reports/mqtt-contract-results.jsonl"
  mkdir -p "${E2E_RUN_DIR}/reports"
  {
    echo "# MQTT contract summary (Phase 7)"
    echo
    echo "Generated: $(now_utc)"
    echo
    if [[ ! -f "$jl" ]] || [[ ! -s "$jl" ]]; then
      echo "_(no mqtt-contract-results.jsonl)_"
    else
      jq -s '
        {
          pass: (map(select(.status=="pass")) | length),
          fail: (map(select(.status=="fail")) | length),
          skip: (map(select(.status=="skip")) | length)
        }
      ' "$jl" | jq -r '
        "| Result | Count |",
        "|--------|-------|",
        "| pass | \(.pass) |",
        "| fail | \(.fail) |",
        "| skip | \(.skip) |"
      '
      echo
      echo "## By flow"
      echo
      jq -s -r '
        group_by(.flow_id)[]
        | ("### " + (.[0].flow_id) + "\n\n"
          + "| step | topic | status | note |\n"
          + "|------|-------|--------|------|\n"
          + (map("| \(.step) | `\(.topic)` | **\(.status)** | \(.message) |\n") | join("")))
      ' "$jl"
    fi
  } >"${out}"
}

mqtt_write_contract_summary

if [[ "${E2E_IN_PARENT:-0}" != "1" ]]; then
  # shellcheck source=lib/e2e_report.sh
  source "${SCRIPT_DIR}/lib/e2e_report.sh"
  e2e_finalize_reports "${ec}"
  sm="${E2E_RUN_DIR}/reports/summary.md"
  if [[ -f "$sm" ]] && [[ -f "${E2E_RUN_DIR}/reports/mqtt-contract-summary.md" ]]; then
    {
      echo ""
      echo "## MQTT contract (Phase 7)"
      echo ""
      echo "See \`reports/mqtt-contract-summary.md\` and \`reports/mqtt-contract-results.jsonl\`; artifacts under \`mqtt/\`."
      echo ""
      cat "${E2E_RUN_DIR}/reports/mqtt-contract-summary.md"
      echo ""
    } >>"$sm"
  fi
fi

[[ "${E2E_IN_PARENT:-0}" == "1" ]] && exit "${ec}"
exit "${ec}"
