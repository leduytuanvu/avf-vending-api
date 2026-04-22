#!/usr/bin/env bash
set -Eeuo pipefail

SHARED_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
# shellcheck source=./lib_release.sh
source "${SHARED_ROOT}/scripts/lib_release.sh"

require_cmd ssh

REMOTE_ROOT="${PRODUCTION_DEPLOY_ROOT:-/opt/avf-vending-api}"
REMOTE_DIR="${APP_NODE_REMOTE_DIR:-${REMOTE_ROOT}/deployments/prod/app-node}"
TEMPORAL_ENABLED="$(normalize_bool "${APP_NODE_ENABLE_TEMPORAL_PROFILE:-0}")"
RUN_MIGRATION_ON_FIRST_NODE="$(normalize_bool "${RUN_MIGRATION_ON_FIRST_NODE:-1}")"

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

note "rolling app-node release across ${#APP_NODE_HOSTS[@]} host(s)"

for idx in "${!APP_NODE_HOSTS[@]}"; do
	host="${APP_NODE_HOSTS[$idx]}"
	target="$(ssh_target "${host}")"
	migrate_flag="0"
	if [[ "${idx}" == "0" && "${RUN_MIGRATION_ON_FIRST_NODE}" == "1" ]]; then
		migrate_flag="1"
	fi
	note "release ${host} (migration=${migrate_flag})"
	run_remote_script "${target}" "${REMOTE_DIR}" "scripts/release_app_node.sh" "${1-}" "${2-}" "${migrate_flag}" "${TEMPORAL_ENABLED}"
	note "verify ${host}"
	run_remote_script "${target}" "${REMOTE_DIR}" "scripts/healthcheck_app_node.sh" "" "" "0" "${TEMPORAL_ENABLED}"
done

echo "release_app_cluster: PASS"
