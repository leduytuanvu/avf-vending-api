#!/usr/bin/env bash
# Remove stale APP_GIT_SHA from app-node env so /version uses link-time embed from the deployed image.
# Usage: apply_clear_stale_app_git_sha_app_node_env.sh [path/to/.env.app-node]
set -euo pipefail

ENV_FILE="${1:-${APP_NODE_ENV_FILE:-}}"
if [[ -z "${ENV_FILE}" ]]; then
	SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
	ENV_FILE="${SCRIPT_DIR}/../../app-node/.env.app-node"
fi

if [[ ! -f "${ENV_FILE}" ]]; then
	echo "error: env file not found: ${ENV_FILE}" >&2
	exit 1
fi

if grep -q "^APP_GIT_SHA=" "${ENV_FILE}"; then
	sed -i '/^APP_GIT_SHA=/d' "${ENV_FILE}"
	echo "apply_clear_stale_app_git_sha_app_node_env: removed stale APP_GIT_SHA override"
else
	echo "apply_clear_stale_app_git_sha_app_node_env: ok (APP_GIT_SHA not set)"
fi
