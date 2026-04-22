#!/usr/bin/env bash
set -Eeuo pipefail

NODE_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SHARED_ROOT="$(cd "${NODE_ROOT}/../shared" && pwd)"
# shellcheck source=../../shared/scripts/lib_release.sh
source "${SHARED_ROOT}/scripts/lib_release.sh"

ENV_FILE="${NODE_ROOT}/.env.app-node"
COMPOSE_FILE="${NODE_ROOT}/docker-compose.app-node.yml"
COMPOSE=(docker compose --env-file "${ENV_FILE}" -f "${COMPOSE_FILE}")
init_state_dir

require_file "${ENV_FILE}"
require_file "${COMPOSE_FILE}"

WAIT_SECS="${APP_NODE_HEALTH_WAIT_SECS:-180}"
POLL_SECS="${APP_NODE_HEALTH_POLL_SECS:-5}"
TEMPORAL_ENABLED="${APP_NODE_ENABLE_TEMPORAL_PROFILE:-0}"
failures=0

pass() {
	echo "PASS: $1"
}

check() {
	local label="$1"
	shift
	if "$@" >/dev/null 2>&1; then
		pass "${label}"
	else
		echo "FAIL: ${label}" >&2
		failures=$((failures + 1))
	fi
}

wait_for_service() {
	local container="$1"
	if [[ -z "${container}" ]]; then
		echo "FAIL: missing container id for monitored service" >&2
		failures=$((failures + 1))
		return 0
	fi
	if wait_for_container_ready "${container}" "${WAIT_SECS}" "${POLL_SECS}"; then
		pass "container ${container} ready"
	else
		echo "FAIL: container ${container} not ready" >&2
		failures=$((failures + 1))
	fi
}

note "app-node container readiness"
wait_for_service "$("${COMPOSE[@]}" ps -q api)"
wait_for_service "$("${COMPOSE[@]}" ps -q worker)"
wait_for_service "$("${COMPOSE[@]}" ps -q reconciler)"
wait_for_service "$("${COMPOSE[@]}" ps -q mqtt-ingest)"
wait_for_service "$("${COMPOSE[@]}" ps -q caddy)"

if [[ "${TEMPORAL_ENABLED}" == "1" ]]; then
	wait_for_service "$("${COMPOSE[@]}" --profile temporal ps -q temporal-worker)"
fi

note "managed dependency reachability"
if bash "${SHARED_ROOT}/scripts/check_managed_services.sh" "${ENV_FILE}"; then
	pass "managed services reachable from app node"
else
	echo "FAIL: managed services check failed" >&2
	failures=$((failures + 1))
fi

note "app-node explicit HTTP health checks"
check "api /health/live returns 200" "${COMPOSE[@]}" exec -T api sh -c 'curl -fsS http://127.0.0.1:8080/health/live | grep -qx ok'
check "api /health/ready returns 200" "${COMPOSE[@]}" exec -T api sh -c 'curl -fsS http://127.0.0.1:8080/health/ready | grep -qx ok'
check "api /version returns 200" "${COMPOSE[@]}" exec -T api sh -c 'curl -fsS http://127.0.0.1:8080/version | grep -q "\"version\""'

check "worker /health/ready returns 200" "${COMPOSE[@]}" exec -T worker sh -c 'addr="${WORKER_METRICS_LISTEN:-127.0.0.1:9091}"; case "$addr" in :*) addr="127.0.0.1${addr}";; esac; curl -fsS "http://${addr}/health/ready" | grep -qx ok'
check "reconciler /health/ready returns 200" "${COMPOSE[@]}" exec -T reconciler sh -c 'addr="${RECONCILER_METRICS_LISTEN:-127.0.0.1:9092}"; case "$addr" in :*) addr="127.0.0.1${addr}";; esac; curl -fsS "http://${addr}/health/ready" | grep -qx ok'
check "mqtt-ingest /health/ready returns 200" "${COMPOSE[@]}" exec -T mqtt-ingest sh -c 'addr="${MQTT_INGEST_METRICS_LISTEN:-127.0.0.1:9093}"; case "$addr" in :*) addr="127.0.0.1${addr}";; esac; curl -fsS "http://${addr}/health/ready" | grep -qx ok'
check "caddy upstream healthcheck returns 200" "${COMPOSE[@]}" exec -T caddy sh -c 'wget -qO- http://api:8080/health/live | grep -qx ok'

if [[ "${TEMPORAL_ENABLED}" == "1" ]]; then
	check "temporal-worker /health/ready returns 200" "${COMPOSE[@]}" --profile temporal exec -T temporal-worker sh -c 'addr="${TEMPORAL_WORKER_METRICS_LISTEN:-127.0.0.1:9094}"; case "$addr" in :*) addr="127.0.0.1${addr}";; esac; curl -fsS "http://${addr}/health/ready" | grep -qx ok'
fi

if (( failures > 0 )); then
	echo "healthcheck_app_node: FAIL (${failures} checks failed)" >&2
	exit 1
fi

echo "healthcheck_app_node: PASS"
