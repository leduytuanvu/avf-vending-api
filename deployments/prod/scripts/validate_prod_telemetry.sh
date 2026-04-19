#!/usr/bin/env bash
# Validates production telemetry-related env and that prod compose renders.
# Exit 1: missing env file, compose config failure, production telemetry rules violated, or required services missing.
# Optional: STRICT_METRICS_WARNINGS=1 treats METRICS_ENABLED!=true as a hard failure (default is stderr warning only).
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
ENV_FILE_INPUT="${1:-$ROOT/.env.production}"

resolve_env_file() {
	local p="$1"
	if [[ "${p}" == /* ]]; then
		printf '%s' "${p}"
		return
	fi
	if [[ -f "${p}" ]]; then
		printf '%s' "$(cd "$(dirname "${p}")" && pwd)/$(basename "${p}")"
		return
	fi
	if [[ -f "${ROOT}/${p}" ]]; then
		printf '%s/%s' "${ROOT}" "${p}"
		return
	fi
	printf ''
}

ENV_FILE="$(resolve_env_file "${ENV_FILE_INPUT}")"
if [[ -z "${ENV_FILE}" ]]; then
	echo "validate_prod_telemetry: error: env file not found: ${ENV_FILE_INPUT}" >&2
	exit 1
fi

fail() {
	echo "validate_prod_telemetry: error: $*" >&2
	exit 1
}

warn() {
	echo "validate_prod_telemetry: warning: $*" >&2
}

read_value() {
	local key="$1"
	local line
	if [[ ! -f "${ENV_FILE}" ]]; then
		fail "env file not found: ${ENV_FILE}"
	fi
	line="$(grep -E "^${key}=" "${ENV_FILE}" 2>/dev/null | tail -n1 || true)"
	if [[ -z "${line}" ]]; then
		printf ''
		return
	fi
	line="${line#"${key}="}"
	line="${line%$'\r'}"
	if [[ "${line}" == \"*\" ]]; then
		line="${line#\"}"
		line="${line%\"}"
	fi
	printf '%s' "${line}"
}

is_truthy_legacy() {
	local v
	v="$(printf '%s' "$1" | tr '[:upper:]' '[:lower:]' | tr -d '[:space:]')"
	[[ "${v}" == "true" || "${v}" == "1" || "${v}" == "yes" ]]
}

note() {
	echo "==> $*"
}

if [[ ! -f "${ENV_FILE}" ]]; then
	fail "missing env file: ${ENV_FILE} (copy from .env.production.example)"
fi

APP_ENV_RAW="$(read_value APP_ENV)"
APP_ENV_LC="$(printf '%s' "${APP_ENV_RAW}" | tr '[:upper:]' '[:lower:]' | tr -d '[:space:]')"

if [[ "${APP_ENV_LC}" == "production" ]]; then
	NATS_URL_VAL="$(read_value NATS_URL)"
	NATS_TRIM="$(printf '%s' "${NATS_URL_VAL}" | tr -d '[:space:]')"
	if [[ -z "${NATS_TRIM}" ]]; then
		fail "APP_ENV=production requires non-empty NATS_URL"
	fi
	LEGACY="$(read_value TELEMETRY_LEGACY_POSTGRES_INGEST)"
	if is_truthy_legacy "${LEGACY}"; then
		fail "APP_ENV=production forbids TELEMETRY_LEGACY_POSTGRES_INGEST=true (use JetStream path only)"
	fi
fi

cd "${ROOT}"
COMPOSE=(docker compose --env-file "${ENV_FILE}" -f docker-compose.prod.yml)

note "docker compose config"
if ! "${COMPOSE[@]}" config >/dev/null; then
	fail "docker compose config failed (fix .env.production / compose interpolation)"
fi

note "required telemetry-path services present"
services="$("${COMPOSE[@]}" config --services)"
for s in nats emqx api worker mqtt-ingest postgres; do
	if ! printf '%s\n' "${services}" | grep -qx "${s}"; then
		fail "rendered compose is missing required service '${s}'"
	fi
done

METRICS="$(read_value METRICS_ENABLED | tr '[:upper:]' '[:lower:]' | tr -d '[:space:]')"
if [[ "${METRICS}" != "true" ]]; then
	msg="METRICS_ENABLED is not true — fleet overload signals (avf_telemetry_*, avf_mqtt_ingest_*) will be unavailable from /metrics"
	if [[ "${STRICT_METRICS_WARNINGS:-0}" == "1" ]]; then
		fail "${msg} (unset STRICT_METRICS_WARNINGS or set METRICS_ENABLED=true)"
	fi
	warn "${msg}"
fi

echo "validate_prod_telemetry: PASS (${ENV_FILE})"
