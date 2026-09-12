#!/usr/bin/env bash
# Read-only EMQX audit: machine users, retained messages, connected clients.
# Usage: bash scripts/ops/emqx-audit.sh
set -Eeuo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
DEPLOY_ROOT="${AVF_DEPLOY_ROOT:-${ROOT}}"

fail() {
	echo "emqx-audit: error: $*" >&2
	exit 1
}

note() {
	echo "emqx-audit: $*"
}

ENVF="${DEPLOY_ROOT}/deployments/prod/.env.production"
if [[ ! -f "${ENVF}" && -f "${DEPLOY_ROOT}/deployments/prod/data-node/.env.data-node" ]]; then
	ENVF="${DEPLOY_ROOT}/deployments/prod/data-node/.env.data-node"
fi
[[ -f "${ENVF}" ]] || fail "missing EMQX env file"

read_env() {
	local key="$1"
	local line
	line="$(grep -E "^${key}=" "${ENVF}" 2>/dev/null | tail -n1 || true)"
	[[ -n "${line}" ]] || fail "${key} not set in ${ENVF}"
	line="${line#"${key}="}"
	line="${line%$'\r'}"
	if [[ "${line}" == \"*\" ]]; then
		line="${line#\"}"
		line="${line%\"}"
	fi
	printf '%s' "${line}"
}

MQTT_USERNAME="$(read_env MQTT_USERNAME)"
EMQX_API_KEY="${EMQX_API_KEY:-$(read_env EMQX_API_KEY)}"
EMQX_API_SECRET="${EMQX_API_SECRET:-$(read_env EMQX_API_SECRET)}"
EMQX_BASE="${EMQX_MANAGEMENT_URL:-http://127.0.0.1:18083}"
EMQX_BASE="${EMQX_BASE%/}"
TOPIC_PREFIX="${MQTT_TOPIC_PREFIX:-avf/devices}"

command -v jq >/dev/null 2>&1 || fail "jq is required"
command -v curl >/dev/null 2>&1 || fail "curl is required"

AUTH_PATH="authentication/password_based%3Abuilt_in_database/users"
API="${EMQX_BASE}/api/v5"

emqx_curl() {
	curl -fsS --max-time 30 -u "${EMQX_API_KEY}:${EMQX_API_SECRET}" "$@"
}

note "management=${EMQX_BASE} service_user=${MQTT_USERNAME} topic_prefix=${TOPIC_PREFIX}"

if ! emqx_curl "${API}/status" >/dev/null 2>&1; then
	fail "EMQX management API not reachable at ${EMQX_BASE}"
fi

users_json="$(emqx_curl "${API}/${AUTH_PATH}?limit=1000")"
machine_users=0
service_users=0
while IFS= read -r user_id; do
	[[ -n "${user_id}" ]] || continue
	if [[ "${user_id}" == "${MQTT_USERNAME}" || "${user_id}" == "avf_app" || "${user_id}" == "avf-mqtt-api" || "${user_id}" == "avf-mqtt-ingest" ]]; then
		service_users=$((service_users + 1))
		note "user keep service=${user_id}"
	else
		machine_users=$((machine_users + 1))
		note "user machine=${user_id}"
	fi
done < <(jq -r '.data[].user_id // empty' <<<"${users_json}")

retained_count=0
if retained_json="$(emqx_curl "${API}/mqtt/retainer/messages?limit=1000" 2>/dev/null)"; then
	retained_count="$(jq -r '[.data[] | select(.topic | startswith("$SYS/") | not)] | length' <<<"${retained_json}" 2>/dev/null || echo 0)"
	note "retained_messages=${retained_count}"
else
	note "retained_messages=UNKNOWN (API endpoint unavailable)"
fi

client_count=0
if clients_json="$(emqx_curl "${API}/clients?limit=1000" 2>/dev/null)"; then
	client_count="$(jq -r '.data | length' <<<"${clients_json}" 2>/dev/null || echo 0)"
	note "connected_clients=${client_count}"
else
	note "connected_clients=UNKNOWN"
fi

note "summary machine_users=${machine_users} service_users=${service_users} retained=${retained_count} clients=${client_count}"

audit_fail=0
[[ "${machine_users}" -eq 0 ]] || audit_fail=1
[[ "${retained_count}" -eq 0 ]] || audit_fail=1

if [[ "${audit_fail}" -ne 0 ]]; then
	note "audit: FAIL"
	exit 1
fi

note "audit: PASS"
