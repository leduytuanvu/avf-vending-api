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
  # Git Bash + Windows jq can yield CRLF; strip CR to keep topics valid UTF-8.
  mid="${mid//$'\r'/}"
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
  # Windows `.env` files may carry CRLF; strip carriage returns to avoid malformed MQTT topics.
  raw="${raw//$'\r'/}"
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

  local _had_errexit=0
  case "$-" in *e*) _had_errexit=1 ;; esac

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
  [[ "${_had_errexit}" -eq 1 ]] && set -e

  jq -nc \
    --arg topic "$topic" \
    --argjson exitCode "$ec" \
    '{topic:$topic,exitCode:$exitCode}' >"${dir}/${output_name}.meta.json"

  # Documented: docs/api/mqtt-contract.md § "Phase 7 local smoke: Mosquitto client exit codes".
  # Do not mask real broker/auth failures: exit 27 only if logs look like a client quirk, not a disconnect error.
  if [[ "$ec" -eq 0 ]]; then
    return 0
  fi
  if [[ "$ec" -eq 27 ]] && [[ -f "${logf}" ]]; then
    if grep -qiE 'error|unable to connect|connection refused|not authorised|not authorized|denied' "${logf}"; then
      return "$ec"
    fi
    return 0
  fi
  return "$ec"
}

# Subscribe for one message or timeout. ec 0 = message, 27 = timeout (mosquitto often = connected).
e2e_mqtt_subscribe_once() {
  local topic="$1"
  local timeout_sec="$2"
  local output_name="$3"
  require_cmd mosquitto_sub

  local _had_errexit=0
  case "$-" in *e*) _had_errexit=1 ;; esac

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
  [[ "${_had_errexit}" -eq 1 ]] && set -e

  jq -nc \
    --arg topic "$topic" \
    --argjson timeout "$timeout_sec" \
    --argjson exitCode "$ec" \
    '{topic:$topic,timeoutSec:$timeout,exitCode:$exitCode}' >"${dir}/${output_name}.meta.json"

  return "$ec"
}

# Start mosquitto_sub in the current shell (must not use command substitution around this).
# Writes background PID into the variable named by the 4th argument.
e2e_mqtt_subscribe_background_pid() {
  local topic="$1"
  local timeout_sec="$2"
  local logf="$3"
  local pid_var="$4"
  require_cmd mosquitto_sub
  mkdir -p "$(dirname "$logf")"
  local -a args=()
  e2e_mqtt_build_client_args args
  args+=(-t "$topic" -C 1 -W "$timeout_sec" -q 1)
  mosquitto_sub "${args[@]}" >"${logf}" 2>&1 &
  printf -v "$pid_var" '%s' "$!"
}

e2e_mqtt_subscribe_accept_connect() {
  local topic="$1"
  local timeout_sec="$2"
  local output_name="$3"
  e2e_mqtt_subscribe_once "$topic" "$timeout_sec" "$output_name"
  local ec=$?
  # Documented: docs/api/mqtt-contract.md § "Phase 7 local smoke: Mosquitto client exit codes".
  # 0 = received message; 27 = common timeout while session still up (Windows/Git Bash); 5 = no connection on some builds.
  if [[ "$ec" -eq 0 ]] || [[ "$ec" -eq 27 ]]; then
    return 0
  fi
  return "$ec"
}

# After `wait` on a background `mosquitto_sub -C 1`: exit 0 means clean receipt; 27 with an empty log means timeout.
# Exit 27 with a non-empty first line is treated as success only for Phase 7 (see mqtt-contract.md).
e2e_mqtt_sub_join_payload_ok() {
  local wait_ec="$1"
  local logf="$2"
  if [[ "$wait_ec" -eq 0 ]]; then
    return 0
  fi
  if [[ "$wait_ec" -eq 27 ]] && [[ -f "$logf" ]]; then
    local line
    line="$(head -n1 "$logf" 2>/dev/null || true)"
    line="${line//$'\r'/}"
    if [[ -n "${line// /}" ]]; then
      return 0
    fi
  fi
  return 1
}
