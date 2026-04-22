#!/usr/bin/env bash
set -Eeuo pipefail

SHARED_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
# shellcheck source=./lib_release.sh
source "${SHARED_ROOT}/scripts/lib_release.sh"

require_cmd ssh

REMOTE_ROOT="${PRODUCTION_DEPLOY_ROOT:-/opt/avf-vending-api}"
REMOTE_DIR="${APP_NODE_REMOTE_DIR:-${REMOTE_ROOT}/deployments/prod/app-node}"
TEMPORAL_ENABLED="$(normalize_bool "${APP_NODE_ENABLE_TEMPORAL_PROFILE:-0}")"

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

note "verify app-node health across ${#APP_NODE_HOSTS[@]} host(s)"

for host in "${APP_NODE_HOSTS[@]}"; do
	target="$(ssh_target "${host}")"
	note "healthcheck ${host}"
	run_remote_script "${target}" "${REMOTE_DIR}" "scripts/healthcheck_app_node.sh" "" "" "0" "${TEMPORAL_ENABLED}"
done

echo "healthcheck_app_node: PASS"
