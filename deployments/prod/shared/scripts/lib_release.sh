#!/usr/bin/env bash

fail() {
	echo "error: $*" >&2
	exit 1
}

note() {
	echo "==> $*"
}

warn() {
	echo "warn: $*" >&2
}

normalize_bool() {
	local value="${1:-}"
	case "${value}" in
	1 | true | TRUE | yes | YES | on | ON)
		printf '1'
		;;
	*)
		printf '0'
		;;
	esac
}

require_cmd() {
	local cmd="$1"
	command -v "${cmd}" >/dev/null 2>&1 || fail "required command not found: ${cmd}"
}

require_file() {
	local path="$1"
	[[ -f "${path}" ]] || fail "required file not found: ${path}"
}

require_dir() {
	local path="$1"
	[[ -d "${path}" ]] || fail "required directory not found: ${path}"
}

load_env_file() {
	local path="$1"
	require_file "${path}"
	# shellcheck disable=SC1090
	set -a
	source "${path}"
	set +a
}

read_env_value() {
	local key="$1"
	local default="${2-__missing__}"
	local line
	line="$(grep -E "^${key}=" "${ENV_FILE}" 2>/dev/null | tail -n1 || true)"
	if [[ -z "${line}" ]]; then
		if [[ "${default}" == "__missing__" ]]; then
			fail "missing ${key} in ${ENV_FILE}"
		fi
		printf '%s' "${default}"
		return 0
	fi
	line="${line#"${key}="}"
	line="${line%$'\r'}"
	if [[ "${line}" == \"*\" ]]; then
		line="${line#\"}"
		line="${line%\"}"
	fi
	printf '%s' "${line}"
}

set_env_value() {
	local key="$1"
	local value="$2"
	if grep -q "^${key}=" "${ENV_FILE}"; then
		sed -i "s|^${key}=.*|${key}=${value}|" "${ENV_FILE}"
	else
		printf '%s=%s\n' "${key}" "${value}" >>"${ENV_FILE}"
	fi
}

init_state_dir() {
	STATE_DIR="${NODE_ROOT}/.deploy"
	mkdir -p "${STATE_DIR}"
}

snapshot_revision() {
	local label="$1"
	local snapshot_dir="${STATE_DIR}/${label}"
	mkdir -p "${snapshot_dir}"
	cp "${ENV_FILE}" "${snapshot_dir}/$(basename "${ENV_FILE}")"
	cp "${COMPOSE_FILE}" "${snapshot_dir}/$(basename "${COMPOSE_FILE}")"
}

restore_revision() {
	local label="$1"
	local snapshot_dir="${STATE_DIR}/${label}"
	local env_name compose_name
	env_name="$(basename "${ENV_FILE}")"
	compose_name="$(basename "${COMPOSE_FILE}")"
	require_file "${snapshot_dir}/${env_name}"
	require_file "${snapshot_dir}/${compose_name}"
	cp "${snapshot_dir}/${env_name}" "${ENV_FILE}"
	cp "${snapshot_dir}/${compose_name}" "${COMPOSE_FILE}"
}

record_image_state() {
	local current_app="${1-}"
	local current_goose="${2-}"
	local previous_app="${3-}"
	local previous_goose="${4-}"
	if [[ -n "${previous_app}" ]]; then
		printf '%s\n' "${previous_app}" >"${STATE_DIR}/previous_app_image_ref"
	fi
	if [[ -n "${previous_goose}" ]]; then
		printf '%s\n' "${previous_goose}" >"${STATE_DIR}/previous_goose_image_ref"
	fi
	if [[ -n "${current_app}" ]]; then
		printf '%s\n' "${current_app}" >"${STATE_DIR}/current_app_image_ref"
	fi
	if [[ -n "${current_goose}" ]]; then
		printf '%s\n' "${current_goose}" >"${STATE_DIR}/current_goose_image_ref"
	fi
}

resolve_image_ref() {
	local key="$1"
	local arg="${2-}"
	if [[ -n "${arg}" ]]; then
		printf '%s' "${arg}"
		return 0
	fi
	read_env_value "${key}"
}

registry_login_optional() {
	if [[ -z "${GHCR_PULL_USERNAME:-}" && -z "${GHCR_PULL_TOKEN:-}" ]]; then
		return 0
	fi
	[[ -n "${GHCR_PULL_USERNAME:-}" && -n "${GHCR_PULL_TOKEN:-}" ]] || fail "set GHCR_PULL_USERNAME and GHCR_PULL_TOKEN together"
	note "docker login ghcr.io"
	printf '%s' "${GHCR_PULL_TOKEN}" | docker login ghcr.io -u "${GHCR_PULL_USERNAME}" --password-stdin >/dev/null
}

compose_config_or_fail() {
	note "docker compose config"
	"${COMPOSE[@]}" config >/dev/null || fail "docker compose config failed"
}

container_state() {
	local name="$1"
	docker inspect -f '{{.State.Status}}' "${name}" 2>/dev/null || echo "missing"
}

container_health() {
	local name="$1"
	docker inspect -f '{{if .State.Health}}{{.State.Health.Status}}{{else}}none{{end}}' "${name}" 2>/dev/null || echo "missing"
}

wait_for_container_ready() {
	local name="$1"
	local wait_secs="${2:-180}"
	local poll_secs="${3:-5}"
	local start_ts now_ts elapsed state health
	start_ts="$(date +%s)"
	while true; do
		state="$(container_state "${name}")"
		health="$(container_health "${name}")"
		if [[ "${state}" == "running" && ( "${health}" == "healthy" || "${health}" == "none" ) ]]; then
			return 0
		fi
		now_ts="$(date +%s)"
		elapsed=$((now_ts - start_ts))
		if [[ "${elapsed}" -ge "${wait_secs}" ]]; then
			echo "container ${name} not ready: state=${state} health=${health}" >&2
			docker logs --tail 100 "${name}" 2>&1 | sed 's/^/  /' >&2 || true
			return 1
		fi
		sleep "${poll_secs}"
	done
}

run_remote_script() {
	local host="$1"
	local remote_dir="$2"
	local script_rel="$3"
	local app_ref="${4-}"
	local goose_ref="${5-}"
	local migrate_flag="${6-0}"
	local temporal_flag="${7-0}"
	local extra_env="${8-}"
	local remote_cmd
	remote_cmd="cd '${remote_dir}' && RUN_MIGRATION='${migrate_flag}' APP_NODE_ENABLE_TEMPORAL_PROFILE='${temporal_flag}' ${extra_env} bash '${script_rel}'"
	if [[ -n "${app_ref}" ]]; then
		remote_cmd+=" '${app_ref}'"
	fi
	if [[ -n "${goose_ref}" ]]; then
		remote_cmd+=" '${goose_ref}'"
	fi
	ssh ${SSH_OPTS:-} "${host}" "${remote_cmd}"
}

ssh_target() {
	local host="$1"
	if [[ -n "${SSH_USER:-}" ]]; then
		printf '%s@%s' "${SSH_USER}" "${host}"
	else
		printf '%s' "${host}"
	fi
}
