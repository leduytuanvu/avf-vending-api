#!/usr/bin/env bash
set -Eeuo pipefail

NODE_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SHARED_ROOT="$(cd "${NODE_ROOT}/../shared" && pwd)"
# shellcheck source=../../shared/scripts/lib_release.sh
source "${SHARED_ROOT}/scripts/lib_release.sh"

APP_NODE_HOSTS_RAW="${APP_NODE_HOSTS:-}"
[[ -n "${APP_NODE_HOSTS_RAW}" ]] || fail "set APP_NODE_HOSTS to the two app-node SSH targets"
REMOTE_DIR="${APP_NODE_REMOTE_DIR:-/opt/avf-vending-api/deployments/prod/app-node}"
TEMPORAL_ENABLED="${APP_NODE_ENABLE_TEMPORAL_PROFILE:-0}"

require_cmd ssh
APP_NODE_HOSTS_RAW="${APP_NODE_HOSTS_RAW//,/ }"
read -r -a APP_NODE_HOSTS <<<"${APP_NODE_HOSTS_RAW}"
[[ "${#APP_NODE_HOSTS[@]}" -ge 2 ]] || fail "APP_NODE_HOSTS must contain at least two hosts for the 2-VPS rollback"

note "rolling app cluster rollback across ${#APP_NODE_HOSTS[@]} hosts"

for host in "${APP_NODE_HOSTS[@]}"; do
	note "rollback ${host}"
	run_remote_script "${host}" "${REMOTE_DIR}" "scripts/rollback_app_node.sh" "${1-}" "${2-}" "0" "${TEMPORAL_ENABLED}"
done

note "final health verification across app nodes"
for host in "${APP_NODE_HOSTS[@]}"; do
	run_remote_script "${host}" "${REMOTE_DIR}" "scripts/healthcheck_app_node.sh" "" "" "0" "${TEMPORAL_ENABLED}"
done

echo "rollback_app_cluster: PASS"
