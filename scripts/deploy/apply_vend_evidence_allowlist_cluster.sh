#!/usr/bin/env bash
# Apply vend evidence allowlist env on both production app nodes and restart api (rolling).
set -Eeuo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SHARED="${SCRIPT_DIR}/../../deployments/prod/shared/scripts"
# shellcheck source=../../deployments/prod/shared/scripts/lib_release.sh
source "${SHARED}/lib_release.sh"

require_cmd ssh

SSH_USER="${PRODUCTION_SSH_USER:-${PROD_SSH_USER:-${SSH_USER:-}}}"
SSH_PORT="${PRODUCTION_SSH_PORT:-${PROD_SSH_PORT:-${SSH_PORT:-22}}}"
DEPLOY_ROOT="${PRODUCTION_DEPLOY_ROOT:-${PROD_DEPLOY_PATH:-/opt/avf-vending-api}}"
ENV_PATH="${APP_NODE_ENV_FILE:-${DEPLOY_ROOT}/deployments/prod/app-node/.env.app-node}"
COMPOSE_DIR="${DEPLOY_ROOT}/deployments/prod/app-node"

APP_NODE_HOSTS_RAW="${APP_NODE_HOSTS:-}"
if [[ -z "${APP_NODE_HOSTS_RAW}" ]]; then
	for host in "${APP_NODE_A_HOST:-}" "${PRODUCTION_APP_NODE_A_HOST:-}" "${PROD_APP_A_HOST:-}" "${APP_A_HOST:-}" \
		"${APP_NODE_B_HOST:-}" "${PRODUCTION_APP_NODE_B_HOST:-}" "${PROD_APP_B_HOST:-}" "${APP_B_HOST:-}"; do
		[[ -n "${host}" ]] && APP_NODE_HOSTS_RAW+=" ${host}"
	done
fi
APP_NODE_HOSTS_RAW="${APP_NODE_HOSTS_RAW//,/ }"
read -r -a APP_NODE_HOSTS <<<"${APP_NODE_HOSTS_RAW}"
[[ "${#APP_NODE_HOSTS[@]}" -ge 1 ]] || fail "set APP_NODE_HOSTS or APP_NODE_A_HOST/APP_NODE_B_HOST"

apply_remote() {
	local host="$1"
	local target
	target="$(ssh_target "${host}")"
	note "apply vend evidence allowlist on ${host}"
	ssh -p "${SSH_PORT}" "${target}" env \
		"APP_NODE_ENV_FILE=${ENV_PATH}" \
		"COMMERCE_REQUIRE_VEND_HARDWARE_EVIDENCE_MACHINE_IDS=${COMMERCE_REQUIRE_VEND_HARDWARE_EVIDENCE_MACHINE_IDS:-019e702c-11c6-7ab0-89c7-5eb32f0b12cb}" \
		bash "${DEPLOY_ROOT}/deployments/prod/shared/scripts/apply_vend_evidence_allowlist_app_node_env.sh" "${ENV_PATH}"
	note "restart api on ${host}"
	ssh -p "${SSH_PORT}" "${target}" "cd '${COMPOSE_DIR}' && docker compose --env-file .env.app-node -f docker-compose.app-node.yml restart api"
}

for host in "${APP_NODE_HOSTS[@]}"; do
	[[ -n "${host}" ]] || continue
	apply_remote "${host}"
done

note "apply_vend_evidence_allowlist_cluster: ok (${#APP_NODE_HOSTS[@]} node(s))"
