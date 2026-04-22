#!/usr/bin/env bash
set -Eeuo pipefail

NODE_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SHARED_ROOT="$(cd "${NODE_ROOT}/../shared" && pwd)"
# shellcheck source=../../shared/scripts/lib_release.sh
source "${SHARED_ROOT}/scripts/lib_release.sh"

ENV_FILE="${NODE_ROOT}/.env.data-node"
COMPOSE_FILE="${NODE_ROOT}/docker-compose.data-node.yml"
COMPOSE=(docker compose --env-file "${ENV_FILE}" -f "${COMPOSE_FILE}")
WAIT_SECS="${DATA_NODE_HEALTH_WAIT_SECS:-180}"
POLL_SECS="${DATA_NODE_HEALTH_POLL_SECS:-5}"
failures=0

require_file "${ENV_FILE}"
require_file "${COMPOSE_FILE}"

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

note "data-node container readiness"
wait_for_service "$("${COMPOSE[@]}" ps -q nats)"
wait_for_service "$("${COMPOSE[@]}" ps -q emqx)"

note "data-node explicit health checks"
check "nats /healthz returns 200" "${COMPOSE[@]}" exec -T nats sh -c 'wget -qO- http://127.0.0.1:8222/healthz >/dev/null'
check "emqx /api/v5/status returns 200" "${COMPOSE[@]}" exec -T emqx bash -lc 'exec 3<>/dev/tcp/127.0.0.1/18083; printf %b "GET /api/v5/status HTTP/1.1\r\nHost: localhost\r\nConnection: close\r\n\r\n" >&3; grep -Fq "emqx is running" <&3'

if (( failures > 0 )); then
	echo "healthcheck_data_node: FAIL (${failures} checks failed)" >&2
	exit 1
fi

echo "healthcheck_data_node: PASS"
