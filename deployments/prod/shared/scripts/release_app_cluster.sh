#!/usr/bin/env bash
set -Eeuo pipefail

SHARED_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
# shellcheck source=./lib_release.sh
source "${SHARED_ROOT}/scripts/lib_release.sh"

require_cmd ssh
require_cmd python3

REMOTE_ROOT="${PRODUCTION_DEPLOY_ROOT:-/opt/avf-vending-api}"
REMOTE_DIR="${APP_NODE_REMOTE_DIR:-${REMOTE_ROOT}/deployments/prod/app-node}"
PROD_ROOT="$(cd "${SHARED_ROOT}/.." && pwd)"
TEMPORAL_ENABLED="$(normalize_bool "${APP_NODE_ENABLE_TEMPORAL_PROFILE:-0}")"
RUN_MIGRATION_ON_FIRST_NODE="$(normalize_bool "${RUN_MIGRATION_ON_FIRST_NODE:-1}")"
RUN_SMOKE_AFTER_READY="$(normalize_bool "${APP_NODE_RUN_SMOKE_AFTER_READY:-0}")"
SMOKE_SCRIPT="${APP_NODE_SMOKE_SCRIPT:-${PROD_ROOT}/scripts/smoke_prod.sh}"
SMOKE_JSON_FILE_BASE="${APP_NODE_SMOKE_JSON_FILE:-}"
SMOKE_LABEL_PREFIX="${APP_NODE_SMOKE_LABEL_PREFIX:-app-node}"

EXIT_CODE_DEPLOY_FAILURE=41
EXIT_CODE_READINESS_FAILURE=42
EXIT_CODE_SMOKE_FAILURE=43

if [[ -n "${SSH_PORT:-}" ]]; then
	SSH_OPTS="-p ${SSH_PORT} ${SSH_OPTS:-}"
fi

APP_NODE_HOSTS_RAW="${APP_NODE_HOSTS:-}"
if [[ -z "${APP_NODE_HOSTS_RAW}" ]]; then
	for host in "${APP_NODE_A_HOST:-}" "${APP_NODE_B_HOST:-}"; do
		if [[ -n "${host}" ]]; then
			APP_NODE_HOSTS_RAW+=" ${host}"
		fi
	done
fi

APP_NODE_HOSTS_RAW="${APP_NODE_HOSTS_RAW//,/ }"
read -r -a APP_NODE_HOSTS <<<"${APP_NODE_HOSTS_RAW}"
[[ "${#APP_NODE_HOSTS[@]}" -ge 1 ]] || fail "set APP_NODE_HOSTS or APP_NODE_A_HOST/APP_NODE_B_HOST"

resolve_smoke_file() {
	local host="$1"
	local index="$2"
	if [[ -z "${SMOKE_JSON_FILE_BASE}" ]]; then
		printf ''
		return 0
	fi
	if [[ "${#APP_NODE_HOSTS[@]}" -eq 1 ]]; then
		printf '%s' "${SMOKE_JSON_FILE_BASE}"
		return 0
	fi
	python3 -c 'import pathlib,sys; path = pathlib.Path(sys.argv[1]); stem = f"{path.stem}-{sys.argv[2]}-{sys.argv[3]}"; print(str(path.with_name(stem + path.suffix)))' "${SMOKE_JSON_FILE_BASE}" "${index}" "${host}"
}

run_local_smoke_gate() {
	local host="$1"
	local index="$2"
	local smoke_label="${SMOKE_LABEL_PREFIX}-${host}"
	local smoke_json_file
	smoke_json_file="$(resolve_smoke_file "${host}" "${index}")"

	require_file "${SMOKE_SCRIPT}"
	note "smoke ${host}"
	append_release_evidence "app-node" "${host}" "smoke" "running" "starting post-readiness blackbox smoke"
	if [[ -n "${smoke_json_file}" ]]; then
		mkdir -p "$(dirname "${smoke_json_file}")"
		SMOKE_JSON=1 \
			SMOKE_CONNECT_TO_HOST="${APP_NODE_SMOKE_CONNECT_TO_HOST:-${host}}" \
			SMOKE_CONNECT_TO_PORT="${APP_NODE_SMOKE_CONNECT_TO_PORT:-}" \
			SMOKE_LABEL="${smoke_label}" \
			bash "${SMOKE_SCRIPT}" --json >"${smoke_json_file}"
	else
		SMOKE_JSON=0 \
			SMOKE_CONNECT_TO_HOST="${APP_NODE_SMOKE_CONNECT_TO_HOST:-${host}}" \
			SMOKE_CONNECT_TO_PORT="${APP_NODE_SMOKE_CONNECT_TO_PORT:-}" \
			SMOKE_LABEL="${smoke_label}" \
			bash "${SMOKE_SCRIPT}"
	fi
	append_release_evidence "app-node" "${host}" "smoke" "pass" "post-readiness blackbox smoke passed"
}

note "rolling app-node release across ${#APP_NODE_HOSTS[@]} host(s)"

for idx in "${!APP_NODE_HOSTS[@]}"; do
	host="${APP_NODE_HOSTS[$idx]}"
	target="$(ssh_target "${host}")"
	migrate_flag="0"
	if [[ "${idx}" == "0" && "${RUN_MIGRATION_ON_FIRST_NODE}" == "1" ]]; then
		migrate_flag="1"
	fi
	note "release ${host} (migration=${migrate_flag})"
	append_release_evidence "app-node" "${host}" "deploy" "running" "starting app-node deploy" "{\"migration\":\"${migrate_flag}\"}"
	if ! run_remote_script "${target}" "${REMOTE_DIR}" "scripts/release_app_node.sh" "${1-}" "${2-}" "${migrate_flag}" "${TEMPORAL_ENABLED}"; then
		append_release_evidence "app-node" "${host}" "deploy" "fail" "app-node deploy failed" "{\"migration\":\"${migrate_flag}\"}"
		exit "${EXIT_CODE_DEPLOY_FAILURE}"
	fi
	append_release_evidence "app-node" "${host}" "deploy" "pass" "app-node deploy completed" "{\"migration\":\"${migrate_flag}\"}"
	note "verify ${host}"
	append_release_evidence "app-node" "${host}" "readiness" "running" "starting app-node readiness verification"
	if ! run_remote_script "${target}" "${REMOTE_DIR}" "scripts/healthcheck_app_node.sh" "" "" "0" "${TEMPORAL_ENABLED}"; then
		append_release_evidence "app-node" "${host}" "readiness" "fail" "app-node readiness verification failed"
		exit "${EXIT_CODE_READINESS_FAILURE}"
	fi
	append_release_evidence "app-node" "${host}" "readiness" "pass" "app-node readiness verification passed"

	if [[ "${RUN_SMOKE_AFTER_READY}" == "1" ]]; then
		if ! run_local_smoke_gate "${host}" "${idx}"; then
			append_release_evidence "app-node" "${host}" "smoke" "fail" "post-readiness blackbox smoke failed"
			exit "${EXIT_CODE_SMOKE_FAILURE}"
		fi
	fi
done

echo "release_app_cluster: PASS"
