#!/usr/bin/env bash
# Idempotent: create built-in MQTT user for app clients (requires EMQX management API on loopback).
# Authenticates with EMQX_API_KEY / EMQX_API_SECRET (HTTP Basic), not dashboard credentials.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

ENVF=".env.production"
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
EMQX_API_KEY="${EMQX_API_KEY:-}"
EMQX_API_SECRET="${EMQX_API_SECRET:-}"

if [[ -n "${EMQX_API_KEY}" || -n "${EMQX_API_SECRET}" ]]; then
	if [[ -z "${EMQX_API_KEY}" || -z "${EMQX_API_SECRET}" ]]; then
		echo "error: EMQX_API_KEY / EMQX_API_SECRET must both be set together in environment or ${ENVF}" >&2
		exit 1
	fi
else
	EMQX_API_KEY="$(read_env EMQX_API_KEY)"
	EMQX_API_SECRET="$(read_env EMQX_API_SECRET)"
fi

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
	echo "emqx_bootstrap: failed to reach EMQX management API at ${BASE} (curl HTTP code=${code:-empty})" >&2
	echo "emqx_bootstrap: ensure avf-prod-emqx is running, port 18083 is published to 127.0.0.1, and the configured EMQX API key still exists in EMQX" >&2
	if [[ -s "${tmp}" ]]; then
		cat "${tmp}" >&2
	fi
	rm -f "${tmp}"
	exit 1
fi

if [[ "${code}" == "200" || "${code}" == "201" ]]; then
	echo "emqx_bootstrap: MQTT user created"
	rm -f "${tmp}"
	exit 0
fi

if [[ "${code}" == "409" ]]; then
	echo "emqx_bootstrap: MQTT user already exists (409) — idempotent ok"
	rm -f "${tmp}"
	exit 0
fi

if [[ "${code}" == "401" ]]; then
	echo "emqx_bootstrap: EMQX management API auth failed (HTTP 401 BAD_API_KEY_OR_SECRET)" >&2
	if [[ -s "${tmp}" ]]; then
		cat "${tmp}" >&2
	fi
	rm -f "${tmp}"
	echo "hint: verify EMQX_API_KEY / EMQX_API_SECRET match a pre-provisioned EMQX REST API key, verify that key still exists in EMQX, and verify dashboard credentials are not being used for /api/v5/*." >&2
	exit 1
fi

echo "emqx_bootstrap: create MQTT user failed (HTTP ${code})" >&2
if [[ -s "${tmp}" ]]; then
	cat "${tmp}" >&2
fi
rm -f "${tmp}"
echo "hint: verify EMQX_API_KEY / EMQX_API_SECRET match a pre-provisioned EMQX REST API key with permission to call /api/v5/*; dashboard credentials are UI-only and are not used for this API." >&2
exit 1
