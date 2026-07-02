#!/usr/bin/env bash
# Apply EMQX management API env from process environment to app-node .env.app-node.
# Never prints EMQX_API_SECRET. Intended for CI SSH remote apply or operator bootstrap.
#
# Usage: apply_emqx_management_app_node_env.sh [path/to/.env.app-node]
set -euo pipefail

ENV_FILE="${1:-${APP_NODE_ENV_FILE:-}}"
if [[ -z "${ENV_FILE}" ]]; then
	SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
	ENV_FILE="${SCRIPT_DIR}/../../app-node/.env.app-node"
fi

if [[ ! -f "${ENV_FILE}" ]]; then
	echo "error: env file not found: ${ENV_FILE}" >&2
	echo "hint: copy deployments/prod/app-node/.env.app-node.example to .env.app-node first" >&2
	exit 1
fi

set_env_kv() {
	local key="$1"
	local value="$2"
	if grep -q "^${key}=" "${ENV_FILE}"; then
		sed -i "s|^${key}=.*|${key}=${value}|" "${ENV_FILE}"
	else
		printf '%s=%s\n' "${key}" "${value}" >>"${ENV_FILE}"
	fi
}

EMQX_MANAGEMENT_URL="${EMQX_MANAGEMENT_URL:-http://187.127.99.153:18083}"
EMQX_API_KEY="${EMQX_API_KEY:-}"
EMQX_API_SECRET="${EMQX_API_SECRET:-}"

[[ -n "${EMQX_MANAGEMENT_URL}" ]] || {
	echo "error: EMQX_MANAGEMENT_URL must be non-empty" >&2
	exit 1
}
[[ -n "${EMQX_API_KEY}" ]] || {
	echo "error: EMQX_API_KEY must be non-empty" >&2
	exit 1
}
[[ -n "${EMQX_API_SECRET}" ]] || {
	echo "error: EMQX_API_SECRET must be non-empty" >&2
	exit 1
}

set_env_kv "EMQX_MANAGEMENT_URL" "${EMQX_MANAGEMENT_URL}"
set_env_kv "EMQX_API_KEY" "${EMQX_API_KEY}"
set_env_kv "EMQX_API_SECRET" "${EMQX_API_SECRET}"

echo "apply_emqx_management_app_node_env: ok (EMQX_MANAGEMENT_URL=${EMQX_MANAGEMENT_URL}, EMQX_API_KEY_PRESENT=true, EMQX_API_SECRET_PRESENT=true)"
