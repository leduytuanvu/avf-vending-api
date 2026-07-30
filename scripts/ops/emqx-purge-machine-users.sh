#!/usr/bin/env bash
# Delete EMQX built-in MQTT users except the service ingest user (MQTT_USERNAME).
# Run on VPS B (data-node) where EMQX management API is on 127.0.0.1:18083.
set -Eeuo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
DEPLOY_ROOT="${AVF_DEPLOY_ROOT:-${ROOT}}"
ENVF="${DEPLOY_ROOT}/deployments/prod/.env.production"

fail() {
	echo "emqx-purge-machine-users: error: $*" >&2
	exit 1
}

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

[[ -f "${ENVF}" ]] || fail "missing ${ENVF}"

MQTT_USERNAME="$(read_env MQTT_USERNAME)"
EMQX_API_KEY="${EMQX_API_KEY:-$(read_env EMQX_API_KEY)}"
EMQX_API_SECRET="${EMQX_API_SECRET:-$(read_env EMQX_API_SECRET)}"
command -v jq >/dev/null 2>&1 || fail "jq is required"

BASE="http://127.0.0.1:18083/api/v5"
AUTH_PATH="authentication/password_based%3Abuilt_in_database/users"

list_json="$(curl -fsS -u "${EMQX_API_KEY}:${EMQX_API_SECRET}" "${BASE}/${AUTH_PATH}?limit=1000")"
deleted=0
kept=0

while IFS= read -r user_id; do
	[[ -n "${user_id}" ]] || continue
	if [[ "${user_id}" == "${MQTT_USERNAME}" ]]; then
		echo "emqx-purge-machine-users: keep service user ${user_id}"
		kept=$((kept + 1))
		continue
	fi
	code="$(curl -sS -o /dev/null -w "%{http_code}" \
		-u "${EMQX_API_KEY}:${EMQX_API_SECRET}" \
		-X DELETE "${BASE}/${AUTH_PATH}/${user_id}" || true)"
	if [[ "${code}" == "204" || "${code}" == "200" ]]; then
		echo "emqx-purge-machine-users: deleted ${user_id}"
		deleted=$((deleted + 1))
	else
		echo "emqx-purge-machine-users: failed to delete ${user_id} (HTTP ${code})" >&2
	fi
done < <(jq -r '.data[].user_id // empty' <<<"${list_json}")

echo "emqx-purge-machine-users: deleted=${deleted} kept=${kept}"
