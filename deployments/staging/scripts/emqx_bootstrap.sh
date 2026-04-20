#!/usr/bin/env bash
# Idempotent: create built-in MQTT user for staging app clients.
# Authenticates with EMQX_API_KEY / EMQX_API_SECRET (HTTP Basic), not dashboard credentials.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

ENVF=".env.staging"
if [[ ! -f "${ENVF}" ]]; then
	echo "error: missing ${ENVF} in ${ROOT}" >&2
	exit 1
fi

read_env() {
	local key="$1"
	local line
	line="$(grep -E "^${key}=" "${ENVF}" | tail -n1 || true)"
	if [[ -z "${line}" ]]; then
		echo "error: ${key} not set in ${ENVF}" >&2
		exit 1
	fi
	line="${line#"${key}="}"
	line="${line%$'\r'}"
	if [[ "${line}" == \"*\" ]]; then
		line="${line#\"}"
		line="${line%\"}"
	fi
	printf '%s' "${line}"
}

MQTT_USERNAME="$(read_env MQTT_USERNAME)"
MQTT_PASSWORD="$(read_env MQTT_PASSWORD)"
EMQX_API_KEY="$(read_env EMQX_API_KEY)"
EMQX_API_SECRET="$(read_env EMQX_API_SECRET)"

if [[ -z "${MQTT_USERNAME}" || -z "${MQTT_PASSWORD}" ]]; then
	echo "error: MQTT_USERNAME / MQTT_PASSWORD must be non-empty" >&2
	exit 1
fi

if [[ -z "${EMQX_API_KEY}" || -z "${EMQX_API_SECRET}" ]]; then
	echo "error: EMQX_API_KEY / EMQX_API_SECRET must be non-empty" >&2
	exit 1
fi

if ! command -v jq >/dev/null 2>&1; then
	echo "error: jq is required for EMQX bootstrap payload generation" >&2
	exit 1
fi

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

if [[ -z "${code}" || "${code}" == "000" ]]; then
	echo "emqx_bootstrap(staging): failed to reach EMQX management API at ${BASE} (curl HTTP code=${code:-empty})" >&2
	echo "emqx_bootstrap(staging): ensure emqx is running, 127.0.0.1:18083 is reachable, and emqx/default_api_key.conf is mounted" >&2
	if [[ -s "${tmp}" ]]; then
		cat "${tmp}" >&2
	fi
	rm -f "${tmp}"
	exit 1
fi

if [[ "${code}" == "200" || "${code}" == "201" ]]; then
	echo "emqx_bootstrap(staging): MQTT user created"
	rm -f "${tmp}"
	exit 0
fi

if [[ "${code}" == "409" ]]; then
	echo "emqx_bootstrap(staging): MQTT user already exists (409) — ok"
	rm -f "${tmp}"
	exit 0
fi

echo "emqx_bootstrap(staging): create MQTT user failed (HTTP ${code})" >&2
if [[ -s "${tmp}" ]]; then
	cat "${tmp}" >&2
fi
rm -f "${tmp}"
echo "hint: verify EMQX_API_KEY / EMQX_API_SECRET match emqx/default_api_key.conf; 401 often means mismatch. Dashboard login credentials are not used for this API." >&2
exit 1
