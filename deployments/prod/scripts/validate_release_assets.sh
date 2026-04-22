#!/usr/bin/env bash
# Validate legacy single-host production release assets before deploy touches containers.
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

legacy_banner() {
	cat >&2 <<'EOF'
================================================================
LEGACY SINGLE-HOST PRODUCTION PATH
NOT THE PRIMARY 2-VPS RELEASE PATH
This validation only covers legacy single-host release assets.
Set ALLOW_LEGACY_SINGLE_HOST=1 to proceed intentionally.
================================================================
EOF
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

require_bash_syntax() {
	local path="$1"
	require_file "${path}"
	bash -n "${path}" || fail "bash syntax check failed: ${path}"
}

require_compose_mount() {
	local needle="$1"
	if ! grep -F -- "${needle}" "${COMPOSE_FILE}" >/dev/null 2>&1; then
		fail "compose file is missing required mount: ${needle}"
	fi
}

require_writable_backup_dir() {
	local backup_dir="$1"
	local env_file="$2"
	local backup_dir_resolved backup_dir_abs probe_file

	[[ -n "${backup_dir}" ]] || fail "PROD_BACKUP_DIR is not set in ${env_file}"
	if [[ "${backup_dir}" == /* ]]; then
		backup_dir_resolved="${backup_dir}"
	else
		backup_dir_resolved="${ROOT}/${backup_dir}"
	fi
	mkdir -p "${backup_dir_resolved}" || fail "could not create PROD_BACKUP_DIR: ${backup_dir_resolved}"
	backup_dir_abs="$(cd "${backup_dir_resolved}" && pwd -P)"
	probe_file="${backup_dir_abs}/.legacy-backup-write-test.$$"
	: >"${probe_file}" || fail "backup directory is not writable: ${backup_dir_abs}"
	rm -f "${probe_file}"
	note "validated backup directory writability (${backup_dir_abs})"
}

main() {
	local env_file api_key api_secret backup_dir

	legacy_banner
	[[ "${ALLOW_LEGACY_SINGLE_HOST:-0}" == "1" ]] || fail "refusing to validate legacy single-host release assets without ALLOW_LEGACY_SINGLE_HOST=1"

	env_file="$(resolve_env_file "${ENV_FILE_INPUT}")"
	if [[ -z "${env_file}" ]]; then
		fail "env file not found: ${ENV_FILE_INPUT}"
	fi

	require_file "${env_file}"
	require_file "${COMPOSE_FILE}"
	require_file "${ROOT}/emqx/base.hocon"
	require_file "${ROOT}/scripts/emqx_bootstrap.sh"
	require_bash_syntax "${ROOT}/scripts/deploy_prod.sh"
	require_bash_syntax "${ROOT}/scripts/update_prod.sh"
	require_bash_syntax "${ROOT}/scripts/release.sh"
	require_bash_syntax "${ROOT}/scripts/backup_postgres.sh"
	require_bash_syntax "${ROOT}/scripts/restore_postgres.sh"
	require_bash_syntax "${ROOT}/scripts/rollback_prod.sh"
	require_bash_syntax "${ROOT}/scripts/healthcheck_prod.sh"
	require_bash_syntax "${ROOT}/scripts/test_release_assets.sh"

	backup_dir="$(read_env_file "${env_file}" PROD_BACKUP_DIR || true)"
	[[ -n "${backup_dir}" ]] || fail "missing PROD_BACKUP_DIR in ${env_file}"
	require_writable_backup_dir "${backup_dir}" "${env_file}"

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

	note "validated LEGACY SINGLE-HOST production release assets (${env_file})"
	echo "validate_release_assets: PASS"
}

main "$@"
