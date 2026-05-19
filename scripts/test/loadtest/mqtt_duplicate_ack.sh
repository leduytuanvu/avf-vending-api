#!/usr/bin/env bash
# Publish the same command payload twice (duplicate dispatch). Defaults to DRY_RUN=true.
set -euo pipefail

DRY_RUN="${DRY_RUN:-true}"
EXECUTE_LOAD_TEST="${EXECUTE_LOAD_TEST:-false}"
LOAD_TEST_ENV="${LOAD_TEST_ENV:-local}"
MQTT_BROKER_URL="${MQTT_BROKER_URL:-tcp://localhost:1883}"
MQTT_TOPIC_PREFIX="${MQTT_TOPIC_PREFIX:-avf/devices}"
MQTT_TOPIC_LAYOUT="${MQTT_TOPIC_LAYOUT:-legacy}"
MACHINE_ID="${MACHINE_ID:-00000000-0000-0000-0000-000000000001}"
COMMAND_ID="${COMMAND_ID:-dup-$(date +%s)}"
MQTT_USERNAME="${MQTT_USERNAME:-}"
MQTT_PASSWORD="${MQTT_PASSWORD:-}"
MQTT_CA_FILE="${MQTT_CA_FILE:-}"

truthy() {
	case "$(printf '%s' "$1" | tr '[:upper:]' '[:lower:]')" in
	true | 1 | yes) return 0 ;;
	*) return 1 ;;
	esac
}

if [[ "${LOAD_TEST_ENV}" == "production" ]]; then
	echo "mqtt_duplicate_ack: refusing production — use staging" >&2
	exit 1
fi
if truthy "${EXECUTE_LOAD_TEST}"; then
	DRY_RUN=false
fi

prefix="${MQTT_TOPIC_PREFIX%/}"
if [[ "${MQTT_TOPIC_LAYOUT}" == "enterprise" ]]; then
	command_topic="${prefix}/machines/${MACHINE_ID}/commands"
else
	command_topic="${prefix}/${MACHINE_ID}/commands"
fi

echo "mqtt_duplicate_ack plan: broker=${MQTT_BROKER_URL} topic=${command_topic} dry_run=${DRY_RUN}"

if truthy "${DRY_RUN}"; then
	exit 0
fi

command -v mosquitto_pub >/dev/null 2>&1 || {
	echo "mosquitto_pub required" >&2
	exit 1
}

host_port="${MQTT_BROKER_URL#*://}"
host="${host_port%%:*}"
port="${host_port##*:}"
[[ "${host}" != "${port}" ]] || port=1883
args=(-h "${host}" -p "${port}" -q 1)
if [[ -n "${MQTT_USERNAME}" ]]; then args+=(-u "${MQTT_USERNAME}"); fi
if [[ -n "${MQTT_PASSWORD}" ]]; then args+=(-P "${MQTT_PASSWORD}"); fi
if [[ -n "${MQTT_CA_FILE}" ]]; then args+=(--cafile "${MQTT_CA_FILE}"); fi

payload="{\"command_id\":\"${COMMAND_ID}\",\"type\":\"diagnostic.ping\",\"payload\":{},\"sent_at\":\"$(date -u +%FT%TZ)\"}"
mosquitto_pub "${args[@]}" -t "${command_topic}" -m "${payload}"
mosquitto_pub "${args[@]}" -t "${command_topic}" -m "${payload}"
