#!/usr/bin/env bash
# Validate production release assets before deploy touches containers.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
ENV_FILE_INPUT="${1:-${ROOT}/.env.production}"
COMPOSE_FILE="${ROOT}/docker-compose.prod.yml"

fail() {
	echo "validate_release_assets: error: $*" >&2
	exit 1
}

note() {
	echo "==> $*"
}

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

read_env_file() {
	local env_file="$1"
	local key="$2"
	local line
	line="$(grep -E "^${key}=" "${env_file}" 2>/dev/null | tail -n1 || true)"
	if [[ -z "${line}" ]]; then
		return 1
	fi
	line="${line#"${key}="}"
	line="${line%$'\r'}"
	if [[ "${line}" == \"*\" ]]; then
		line="${line#\"}"
		line="${line%\"}"
	fi
	printf '%s' "${line}"
}

require_file() {
	local path="$1"
	[[ -f "${path}" ]] || fail "missing required file: ${path}"
}

require_compose_mount() {
	local needle="$1"
	if ! grep -F -- "${needle}" "${COMPOSE_FILE}" >/dev/null 2>&1; then
		fail "compose file is missing required mount: ${needle}"
	fi
}

main() {
	local env_file api_key api_secret

	env_file="$(resolve_env_file "${ENV_FILE_INPUT}")"
	if [[ -z "${env_file}" ]]; then
		fail "env file not found: ${ENV_FILE_INPUT}"
	fi

	require_file "${env_file}"
	require_file "${COMPOSE_FILE}"
	require_file "${ROOT}/emqx/base.hocon"
	require_file "${ROOT}/scripts/emqx_bootstrap.sh"
	require_file "${ROOT}/scripts/healthcheck_prod.sh"

	if [[ -n "${EMQX_API_KEY:-}" ]] || [[ -n "${EMQX_API_SECRET:-}" ]]; then
		if [[ -z "${EMQX_API_KEY:-}" ]] || [[ -z "${EMQX_API_SECRET:-}" ]]; then
			fail "EMQX_API_KEY and EMQX_API_SECRET must both be set together (environment overrides ${env_file})"
		fi
		api_key="${EMQX_API_KEY}"
		api_secret="${EMQX_API_SECRET}"
	else
		api_key="$(read_env_file "${env_file}" EMQX_API_KEY || true)"
		api_secret="$(read_env_file "${env_file}" EMQX_API_SECRET || true)"
		if [[ -z "${api_key}" ]] || [[ -z "${api_secret}" ]]; then
			fail "missing EMQX_API_KEY / EMQX_API_SECRET (set in environment or ${env_file})"
		fi
	fi

	require_compose_mount "./emqx/base.hocon:/opt/emqx/etc/base.hocon:ro"
	require_compose_mount "./emqx/default_api_key.conf:/opt/emqx/etc/default_api_key.conf:ro"

	note "validated production release assets (${env_file})"
	echo "validate_release_assets: PASS"
}

main "$@"
