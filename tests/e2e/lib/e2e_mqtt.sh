#!/usr/bin/env bash
# shellcheck shell=bash
# MQTT helpers (mosquitto clients). Requires e2e_common.sh, MQTT_HOST, MQTT_PORT, E2E_RUN_DIR.

e2e_mqtt_log_dir() {
  echo "${E2E_RUN_DIR}/mqtt"
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
    '{host:$host,port:$port,topic:$topic,payload:$payload}' >"${pub_json}"

  local -a args=(-h "${MQTT_HOST}" -p "${MQTT_PORT:-1883}" -t "$topic" -m "$payload")
  if [[ -n "${MQTT_USERNAME:-}" ]]; then
    args+=(-u "${MQTT_USERNAME}" -P "${MQTT_PASSWORD:-}")
  fi

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

e2e_mqtt_subscribe_once() {
  local topic="$1"
  local timeout_sec="$2"
  local output_name="$3"
  require_cmd mosquitto_sub

  local dir
  dir="$(e2e_mqtt_log_dir)"
  mkdir -p "$dir"

  local logf="${dir}/${output_name}.subscribe.log"
  local -a args=(-h "${MQTT_HOST}" -p "${MQTT_PORT:-1883}" -t "$topic" -C 1 -W "$timeout_sec")
  if [[ -n "${MQTT_USERNAME:-}" ]]; then
    args+=(-u "${MQTT_USERNAME}" -P "${MQTT_PASSWORD:-}")
  fi

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
