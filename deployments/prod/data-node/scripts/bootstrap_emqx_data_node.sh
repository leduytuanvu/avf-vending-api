#!/usr/bin/env bash
set -Eeuo pipefail

NODE_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SHARED_ROOT="$(cd "${NODE_ROOT}/../shared" && pwd)"
# shellcheck source=../../shared/scripts/lib_release.sh
source "${SHARED_ROOT}/scripts/lib_release.sh"

ENV_FILE="${NODE_ROOT}/.env.data-node"
COMPOSE_FILE="${NODE_ROOT}/docker-compose.data-node.yml"
COMPOSE=(docker compose --env-file "${ENV_FILE}" -f "${COMPOSE_FILE}")

require_file "${ENV_FILE}"
require_file "${COMPOSE_FILE}"
require_cmd jq

MQTT_USERNAME="$(read_env_value MQTT_USERNAME)"
MQTT_PASSWORD="$(read_env_value MQTT_PASSWORD)"
EMQX_API_KEY="${EMQX_API_KEY:-$(read_env_value EMQX_API_KEY)}"
EMQX_API_SECRET="${EMQX_API_SECRET:-$(read_env_value EMQX_API_SECRET)}"

BODY="$(jq -cn --arg user "${MQTT_USERNAME}" --arg password "${MQTT_PASSWORD}" '{user_id:$user,password:$password}')"
BASE="http://127.0.0.1:18083/api/v5"
AUTH_PATH="authentication/password_based%3Abuilt_in_database/users"
tmp="$(mktemp)"

code="$(
	curl -sS -o "${tmp}" -w "%{http_code}" \
		-u "${EMQX_API_KEY}:${EMQX_API_SECRET}" \
		-X POST "${BASE}/${AUTH_PATH}" \
		-H "Content-Type: application/json" \
		-d "${BODY}" || true
)"

if [[ "${code}" == "200" || "${code}" == "201" ]]; then
	echo "bootstrap_emqx_data_node: MQTT user created"
	rm -f "${tmp}"
	exit 0
fi

if [[ "${code}" == "409" ]]; then
	echo "bootstrap_emqx_data_node: MQTT user already exists"
	rm -f "${tmp}"
	exit 0
fi

if [[ -s "${tmp}" ]]; then
	cat "${tmp}" >&2
fi
rm -f "${tmp}"
fail "EMQX MQTT user bootstrap failed (HTTP ${code})"
