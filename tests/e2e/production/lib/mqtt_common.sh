#!/usr/bin/env bash
# shellcheck shell=bash
# MQTT topic resolution and mosquitto client helpers for production E2E.

prod_e2e_mqtt_resolve_topics() {
  local mid="${machineId:-${MACHINE_ID:-}}"
  mid="${mid//$'\r'/}"
  [[ -n "$mid" && "$mid" != "null" ]] || {
    echo "prod_e2e_mqtt: machineId required from current run state" >&2
    return 1
  }
  export PROD_E2E_MQTT_MACHINE_ID="$mid"

  if [[ -n "${MQTT_TOPIC_COMMANDS:-}" ]]; then
    export PROD_E2E_MQTT_TOPIC_COMMAND_IN="${MQTT_TOPIC_COMMANDS}"
    export PROD_E2E_MQTT_TOPIC_COMMAND_ACK="${MQTT_TOPIC_COMMAND_ACK:-}"
    export PROD_E2E_MQTT_TOPIC_TELEMETRY="${MQTT_TOPIC_TELEMETRY:-}"
    [[ -n "${PROD_E2E_MQTT_TOPIC_COMMAND_IN}" && -n "${PROD_E2E_MQTT_TOPIC_COMMAND_ACK}" ]] || return 1
    return 0
  fi

  local layout prefix
  layout="$(echo "${MQTT_TOPIC_LAYOUT:-enterprise}" | tr '[:upper:]' '[:lower:]')"
  prefix="${MQTT_TOPIC_PREFIX:-avf/prod}"
  prefix="$(echo "$prefix" | sed 's/^[[:space:]]*//;s/[[:space:]]*$//' | sed 's:/*$::')"
  prefix="${prefix//$'\r'/}"

  if [[ "$layout" == "enterprise" ]]; then
    export PROD_E2E_MQTT_TOPIC_COMMAND_IN="${prefix}/machines/${mid}/commands"
    export PROD_E2E_MQTT_TOPIC_COMMAND_ACK="${prefix}/machines/${mid}/commands/ack"
    export PROD_E2E_MQTT_TOPIC_TELEMETRY="${prefix}/machines/${mid}/telemetry"
    export PROD_E2E_MQTT_TOPIC_HEARTBEAT="${prefix}/machines/${mid}/state/heartbeat"
    export PROD_E2E_MQTT_TOPIC_PRESENCE="${prefix}/machines/${mid}/presence"
    export PROD_E2E_MQTT_TOPIC_TEL_SNAPSHOT="${prefix}/machines/${mid}/telemetry/snapshot"
    export PROD_E2E_MQTT_TOPIC_EVENTS_INV="${prefix}/machines/${mid}/events/inventory"
  else
    export PROD_E2E_MQTT_TOPIC_COMMAND_IN="${prefix}/${mid}/commands/dispatch"
    export PROD_E2E_MQTT_TOPIC_COMMAND_ACK="${prefix}/${mid}/commands/ack"
    export PROD_E2E_MQTT_TOPIC_TELEMETRY="${prefix}/${mid}/telemetry"
    export PROD_E2E_MQTT_TOPIC_HEARTBEAT="${prefix}/${mid}/state/heartbeat"
    export PROD_E2E_MQTT_TOPIC_PRESENCE="${prefix}/${mid}/presence"
    export PROD_E2E_MQTT_TOPIC_TEL_SNAPSHOT="${prefix}/${mid}/telemetry/snapshot"
    export PROD_E2E_MQTT_TOPIC_EVENTS_INV="${prefix}/${mid}/events/inventory"
  fi
  export PROD_E2E_MQTT_TOPIC_LAYOUT="$layout"
  return 0
}

prod_e2e_mqtt_client_args() {
  local -n _out="$1"
  local user pass
  _out=(-h "${MQTT_HOST:-}" -p "${MQTT_PORT:-8883}")
  user="${MQTT_USERNAME:-}"
  pass="${MQTT_PASSWORD:-}"
  if [[ -n "$user" ]]; then
    _out+=(-u "$user" -P "$pass")
  fi
  _out+=(-i "${MQTT_CLIENT_ID:-${PROD_E2E_PREFIX}-mqtt-${RANDOM}}")
  _out+=(-V "${MQTT_PROTOCOL_VERSION:-mqttv311}")
  if [[ "${MQTT_USE_TLS:-true}" == "true" ]]; then
    local ca="${MQTT_CAFILE:-${MQTT_CA_CERT:-}}"
    if [[ -n "$ca" && -f "$ca" ]]; then
      _out+=(--cafile "$ca")
    elif [[ -d /etc/ssl/certs ]]; then
      _out+=(--capath /etc/ssl/certs)
    else
      local -a ca_candidates=(
        "/c/Program Files/Git/usr/ssl/certs/ca-bundle.crt"
        "/c/Program Files/Git/mingw64/ssl/certs/ca-bundle.crt"
        "/usr/ssl/certs/ca-bundle.crt"
      )
      local c
      for c in "${ca_candidates[@]}"; do
        if [[ -f "$c" ]]; then
          _out+=(--cafile "$c")
          break
        fi
      done
    fi
  fi
}

prod_e2e_mqtt_redact_log() {
  local in_file="$1"
  local out_file="$2"
  sed -E \
    -e 's/(-P[[:space:]]+)[^[:space:]]+/\1<redacted>/g' \
    -e 's/("password"[[:space:]]*:[[:space:]]*")[^"]*"/\1<redacted>"/gi' \
    "$in_file" >"$out_file" 2>/dev/null || cp "$in_file" "$out_file"
}

prod_e2e_mqtt_normalize_exit() {
  local ec="$1"
  local logf="$2"
  if [[ "$ec" -eq 0 ]]; then
    return 0
  fi
  if [[ "$ec" -eq 27 && -f "$logf" ]]; then
    if grep -qiE 'error|unable to connect|connection refused|not authorised|not authorized|denied' "$logf"; then
      return "$ec"
    fi
    return 0
  fi
  return "$ec"
}

prod_e2e_mqtt_sub_join_ok() {
  local wait_ec="$1"
  local logf="$2"
  [[ "$wait_ec" -eq 0 ]] && return 0
  if [[ "$wait_ec" -eq 27 && -f "$logf" ]]; then
    local line
    line="$(head -n1 "$logf" 2>/dev/null | tr -d '\r')"
    [[ -n "${line// /}" ]] && return 0
  fi
  return 1
}

prod_e2e_mqtt_now_rfc3339() {
  date -u +%Y-%m-%dT%H:%M:%SZ
}

prod_e2e_mqtt_ensure_clients() {
  if command -v mosquitto_pub >/dev/null 2>&1 && command -v mosquitto_sub >/dev/null 2>&1; then
    return 0
  fi
  local -a candidates=(
    "/c/Program Files/mosquitto"
    "/c/Program Files (x86)/mosquitto"
    "/usr/bin"
    "/usr/local/bin"
  )
  local d
  for d in "${candidates[@]}"; do
    if [[ -x "${d}/mosquitto_pub" && -x "${d}/mosquitto_sub" ]]; then
      export PATH="${d}:${PATH}"
      return 0
    fi
  done
  return 1
}

# 0 = message; 27 = timeout while session up (Windows/Git Bash per mqtt-contract.md).
prod_e2e_mqtt_subscribe_accept_connect() {
  local topic="$1"
  local timeout_sec="$2"
  local evidence_label="$3"
  prod_e2e_mqtt_subscribe_once "$topic" "$timeout_sec" "$evidence_label"
  local rc=$?
  if [[ "$rc" -eq 0 || "$rc" -eq 27 ]]; then
    return 0
  fi
  return "$rc"
}

prod_e2e_mqtt_fail_hint() {
  local flow_id="$1"
  local hint="$2"
  case "$hint" in
    topic) prod_e2e_fail_classify "c" "$flow_id" "MQTT topic naming/layout mismatch — check MQTT_TOPIC_PREFIX and MQTT_TOPIC_LAYOUT" ;;
    auth) prod_e2e_fail_classify "d" "$flow_id" "MQTT auth/TLS failure — check E2E_PROD_MQTT_* credentials and broker ACL" ;;
    qos) prod_e2e_fail_classify "c" "$flow_id" "MQTT QoS/retain mismatch — expected QoS 1 retain false per mqtt-contract.md" ;;
    bridge) prod_e2e_fail_classify "d" "$flow_id" "MQTT backend bridge/ingest not receiving — check mqtt-ingest and EMQX routing" ;;
    payload) prod_e2e_fail_classify "c" "$flow_id" "MQTT payload contract drift — compare testdata/telemetry and docs/api/mqtt-contract.md" ;;
    *) prod_e2e_fail_classify "a" "$flow_id" "$hint" ;;
  esac
}
