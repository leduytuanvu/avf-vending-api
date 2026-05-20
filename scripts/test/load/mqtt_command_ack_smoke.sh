#!/usr/bin/env bash
# Optional MQTT command dispatch/ACK smoke. Defaults to DRY_RUN=true; requires mosquitto clients only when executing.
set -euo pipefail

DRY_RUN="${DRY_RUN:-true}"
EXECUTE_LOAD_TEST="${EXECUTE_LOAD_TEST:-false}"
LOAD_TEST_ENV="${LOAD_TEST_ENV:-local}"
CONFIRM_PROD_LOAD_TEST="${CONFIRM_PROD_LOAD_TEST:-false}"
MQTT_BROKER_URL="${MQTT_BROKER_URL:-tcp://localhost:1883}"
MQTT_TOPIC_PREFIX="${MQTT_TOPIC_PREFIX:-avf/devices}"
MQTT_TOPIC_LAYOUT="${MQTT_TOPIC_LAYOUT:-legacy}" # legacy | enterprise
MACHINE_ID="${MACHINE_ID:-00000000-0000-0000-0000-000000000001}"
COMMAND_ID="${COMMAND_ID:-load-command-$(date +%s)}"
MQTT_USERNAME="${MQTT_USERNAME:-}"
MQTT_PASSWORD="${MQTT_PASSWORD:-}"
MQTT_CA_FILE="${MQTT_CA_FILE:-}"

truthy() {
	case "$(printf '%s' "$1" | tr '[:upper:]' '[:lower:]' | tr -d '[:space:]')" in
	true | 1 | yes) return 0 ;;
	*) return 1 ;;
	esac
}

if [[ "${LOAD_TEST_ENV}" == "production" ]] && ! truthy "${CONFIRM_PROD_LOAD_TEST}"; then
	echo "mqtt_command_ack_smoke: refusing production run without CONFIRM_PROD_LOAD_TEST=true" >&2
	exit 1
fi
if truthy "${EXECUTE_LOAD_TEST}"; then
	DRY_RUN=false
fi

prefix="${MQTT_TOPIC_PREFIX%/}"
if [[ "${MQTT_TOPIC_LAYOUT}" == "enterprise" ]]; then
	command_topic="${prefix}/machines/${MACHINE_ID}/commands"
	ack_topic="${prefix}/machines/${MACHINE_ID}/commands/ack"
else
	command_topic="${prefix}/${MACHINE_ID}/commands"
	ack_topic="${prefix}/${MACHINE_ID}/commands/ack"
fi

echo "mqtt_command_ack_smoke plan:"
echo "  env=${LOAD_TEST_ENV} broker=${MQTT_BROKER_URL} machine=${MACHINE_ID} dry_run=${DRY_RUN}"
echo "  publish topic=${command_topic}"
echo "  ack topic=${ack_topic}"
echo "  metrics=command ACK latency, avf_mqtt_publish_duration_seconds, mqtt-ingest dispatch counters"

if truthy "${DRY_RUN}"; then
	exit 0
fi

command -v mosquitto_pub >/dev/null 2>&1 || { echo "mqtt_command_ack_smoke: mosquitto_pub required" >&2; exit 1; }
command -v mosquitto_sub >/dev/null 2>&1 || { echo "mqtt_command_ack_smoke: mosquitto_sub required" >&2; exit 1; }

host_port="${MQTT_BROKER_URL#*://}"
host="${host_port%%:*}"
port="${host_port##*:}"
[[ "${host}" != "${port}" ]] || port=1883
args=(-h "${host}" -p "${port}" -q 1)
if [[ -n "${MQTT_USERNAME}" ]]; then args+=(-u "${MQTT_USERNAME}"); fi
if [[ -n "${MQTT_PASSWORD}" ]]; then args+=(-P "${MQTT_PASSWORD}"); fi
if [[ -n "${MQTT_CA_FILE}" ]]; then args+=(--cafile "${MQTT_CA_FILE}"); fi

tmp="$(mktemp)"
trap 'rm -f "${tmp}"' EXIT
timeout "${ACK_LISTEN_SECONDS:-10}" mosquitto_sub "${args[@]}" -t "${ack_topic}" -C 1 >"${tmp}" &
sub_pid=$!
sleep 1
payload="{\"command_id\":\"${COMMAND_ID}\",\"type\":\"diagnostic.ping\",\"payload\":{},\"sent_at\":\"$(date -u +%FT%TZ)\"}"
mosquitto_pub "${args[@]}" -t "${command_topic}" -m "${payload}"
wait "${sub_pid}" || {
	echo "mqtt_command_ack_smoke: no ACK observed within timeout; this is expected if no device simulator is subscribed" >&2
	exit 2
}
echo "mqtt_command_ack_smoke: observed ACK payload:"
sed 's/./&/g' "${tmp}"
