#!/usr/bin/env bash
# shellcheck shell=bash
# MQTT helpers (mosquitto clients). Requires e2e_common.sh, E2E_RUN_DIR.
# Topic layout follows docs/api/mqtt-contract.md and internal/platform/mqtt/topics.go:
#   legacy:    {prefix}/{machineId}/telemetry, .../commands/dispatch, .../commands/ack
#   enterprise:{prefix}/machines/{machineId}/telemetry, .../commands, .../commands/ack

e2e_mqtt_log_dir() {
  echo "${E2E_RUN_DIR}/mqtt"
}

e2e_mqtt_tcp_open() {
  local host port
  host="${MQTT_HOST%%:*}"
  port="${MQTT_PORT:-1883}"
  [[ -n "$host" ]] || return 1
  if command -v timeout >/dev/null 2>&1; then
    timeout 2 bash -c "echo >/dev/tcp/${host}/${port}" >/dev/null 2>&1
    return $?
  fi
  bash -c "echo >/dev/tcp/${host}/${port}" >/dev/null 2>&1
}

# Resolve concrete topics; env overrides: MQTT_TOPIC_TELEMETRY, MQTT_TOPIC_COMMANDS, MQTT_TOPIC_COMMAND_ACK, MQTT_TOPIC_EVENTS.
# Requires machine UUID from MQTT_MACHINE_ID or test-data machineId (via get_data when sourced after e2e_data).
e2e_mqtt_resolve_topics() {
  local mid="${MQTT_MACHINE_ID:-}"
  if [[ -z "$mid" ]] && declare -F get_data >/dev/null 2>&1; then
    mid="$(get_data machineId 2>/dev/null || true)"
  fi
  [[ "$mid" == "null" ]] && mid=""
  if [[ -z "$mid" ]]; then
    log_error "e2e_mqtt: set MQTT_MACHINE_ID or machineId in test-data"
    return 2
  fi

  export E2E_MQTT_MACHINE_ID="$mid"

  if [[ -n "${MQTT_TOPIC_TELEMETRY:-}" ]]; then
    export E2E_MQTT_TOPIC_TELEMETRY="$MQTT_TOPIC_TELEMETRY"
    export E2E_MQTT_TOPIC_COMMAND_IN="${MQTT_TOPIC_COMMANDS:-}"
    export E2E_MQTT_TOPIC_COMMAND_ACK="${MQTT_TOPIC_COMMAND_ACK:-}"
    export E2E_MQTT_TOPIC_EVENTS="${MQTT_TOPIC_EVENTS:-}"
    [[ -n "${E2E_MQTT_TOPIC_COMMAND_IN:-}" ]] && [[ -n "${E2E_MQTT_TOPIC_COMMAND_ACK:-}" ]] && return 0
    log_error "e2e_mqtt: MQTT_TOPIC_TELEMETRY set but MQTT_TOPIC_COMMANDS or MQTT_TOPIC_COMMAND_ACK missing"
    return 2
  fi

  local layout raw prefix
  layout="$(echo "${MQTT_TOPIC_LAYOUT:-legacy}" | tr '[:upper:]' '[:lower:]')"
  raw="${MQTT_TOPIC_PREFIX:-avf/devices}"
  raw="$(echo "$raw" | sed 's/^[[:space:]]*//;s/[[:space:]]*$//')"
  prefix="${raw%/}"
  if [[ "$layout" == "enterprise" ]]; then
    export E2E_MQTT_TOPIC_TELEMETRY="${prefix}/machines/${mid}/telemetry"
    export E2E_MQTT_TOPIC_COMMAND_IN="${prefix}/machines/${mid}/commands"
    export E2E_MQTT_TOPIC_COMMAND_ACK="${prefix}/machines/${mid}/commands/ack"
    export E2E_MQTT_TOPIC_EVENTS="${prefix}/machines/${mid}/events"
  else
    export E2E_MQTT_TOPIC_TELEMETRY="${prefix}/${mid}/telemetry"
    export E2E_MQTT_TOPIC_COMMAND_IN="${prefix}/${mid}/commands/dispatch"
    export E2E_MQTT_TOPIC_COMMAND_ACK="${prefix}/${mid}/commands/ack"
    export E2E_MQTT_TOPIC_EVENTS="${prefix}/${mid}/events/vend"
  fi
  return 0
}

e2e_mqtt_build_client_args() {
  local -n __mqtt_args="${1}"
  __mqtt_args=(-h "${MQTT_HOST}" -p "${MQTT_PORT:-1883}")
  if [[ -n "${MQTT_USERNAME:-}" ]]; then
    __mqtt_args+=(-u "${MQTT_USERNAME}" -P "${MQTT_PASSWORD:-}")
  fi
  local cid="${MQTT_CLIENT_ID:-e2e-mqtt}-${RANDOM}${RANDOM}"
  __mqtt_args+=(-i "${cid}")
  if [[ "${MQTT_USE_TLS:-false}" == "true" ]]; then
    if [[ -n "${MQTT_CA_CERT:-}" ]] && [[ -f "${MQTT_CA_CERT}" ]]; then
      __mqtt_args+=(--cafile "${MQTT_CA_CERT}")
    else
      log_warn "e2e_mqtt: MQTT_USE_TLS=true but MQTT_CA_CERT missing or not a file — connection may fail"
    fi
  fi
}

mqtt_contract_record() {
  local flow_id="$1"
  local step="$2"
  local topic="$3"
  local status="$4"
  local msg="$5"
  [[ -n "${E2E_RUN_DIR:-}" ]] || return 0
  mkdir -p "${E2E_RUN_DIR}/reports"
  local jl="${E2E_RUN_DIR}/reports/mqtt-contract-results.jsonl"
  jq -nc \
    --arg ts "$(now_utc)" \
    --arg flow_id "$flow_id" \
    --arg step "$step" \
    --arg topic "$topic" \
    --arg status "$status" \
    --arg msg "$msg" \
    '{ts:$ts,flow_id:$flow_id,step:$step,topic:$topic,status:$status,message:$msg}' >>"${jl}"
}

e2e_mqtt_publish() {
  local topic="$1"
  local payload="$2"
  local output_name="$3"
  require_cmd mosquitto_pub

  local dir
  dir="$(e2e_mqtt_log_dir)"
  mkdir -p "$dir"

  local pub_json="${dir}/${output_name}.publish.json"
  jq -nc \
    --arg host "${MQTT_HOST}" \
    --argjson port "${MQTT_PORT:-1883}" \
    --arg topic "$topic" \
    --arg payload "$payload" \
    --arg tls "${MQTT_USE_TLS:-false}" \
    '{host:$host,port:$port,topic:$topic,payload:$payload,tls:$tls}' >"${pub_json}"

  local -a args=()
  e2e_mqtt_build_client_args args
  args+=(-t "$topic" -m "$payload" -q 1)

  local logf="${dir}/${output_name}.publish.log"
  set +e
  mosquitto_pub "${args[@]}" >"${logf}" 2>&1
  local ec=$?
  set -e

  jq -nc \
    --arg topic "$topic" \
    --argjson exitCode "$ec" \
    '{topic:$topic,exitCode:$exitCode}' >"${dir}/${output_name}.meta.json"

  return "$ec"
}

# Subscribe for one message or timeout. ec 0 = message, 27 = timeout (mosquitto often = connected).
e2e_mqtt_subscribe_once() {
  local topic="$1"
  local timeout_sec="$2"
  local output_name="$3"
  require_cmd mosquitto_sub

  local dir
  dir="$(e2e_mqtt_log_dir)"
  mkdir -p "$dir"

  local logf="${dir}/${output_name}.subscribe.log"
  local -a args=()
  e2e_mqtt_build_client_args args
  args+=(-t "$topic" -C 1 -W "$timeout_sec" -q 1)

  set +e
  mosquitto_sub "${args[@]}" >"${logf}" 2>&1
  local ec=$?
  set -e

  jq -nc \
    --arg topic "$topic" \
    --argjson timeout "$timeout_sec" \
    --argjson exitCode "$ec" \
    '{topic:$topic,timeoutSec:$timeout,exitCode:$exitCode}' >"${dir}/${output_name}.meta.json"

  return "$ec"
}

e2e_mqtt_subscribe_accept_connect() {
  local topic="$1"
  local timeout_sec="$2"
  local output_name="$3"
  e2e_mqtt_subscribe_once "$topic" "$timeout_sec" "$output_name"
  local ec=$?
  # 0 received, 27 common timeout (still connected), 5 = no connection on some builds
  if [[ "$ec" -eq 0 ]] || [[ "$ec" -eq 27 ]]; then
    return 0
  fi
  return "$ec"
}
