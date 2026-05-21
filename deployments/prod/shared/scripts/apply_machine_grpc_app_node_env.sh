#!/usr/bin/env bash
# Apply machine gRPC edge env defaults to app-node .env.app-node (Caddy + api container).
# Usage: apply_machine_grpc_app_node_env.sh [path/to/.env.app-node]
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

set_env_kv() {
	local key="$1"
	local value="$2"
	if grep -q "^${key}=" "${ENV_FILE}"; then
		sed -i "s|^${key}=.*|${key}=${value}|" "${ENV_FILE}"
	else
		printf '%s=%s\n' "${key}" "${value}" >>"${ENV_FILE}"
	fi
}

MACHINE_GRPC_DOMAIN="${MACHINE_GRPC_DOMAIN:-machine-api.ldtv.dev}"
UPSTREAM_GRPC="${UPSTREAM_GRPC:-h2c://api:9090}"
GRPC_ADDR="${GRPC_ADDR:-:9090}"
GRPC_BEHIND_TLS_PROXY="${GRPC_BEHIND_TLS_PROXY:-true}"
GRPC_PUBLIC_BASE_URL="${GRPC_PUBLIC_BASE_URL:-grpcs://${MACHINE_GRPC_DOMAIN}:443}"

set_env_kv "MACHINE_GRPC_DOMAIN" "${MACHINE_GRPC_DOMAIN}"
set_env_kv "UPSTREAM_GRPC" "${UPSTREAM_GRPC}"
set_env_kv "MACHINE_GRPC_ENABLED" "${MACHINE_GRPC_ENABLED:-true}"
set_env_kv "GRPC_ADDR" "${GRPC_ADDR}"
set_env_kv "GRPC_BEHIND_TLS_PROXY" "${GRPC_BEHIND_TLS_PROXY}"
set_env_kv "GRPC_PUBLIC_BASE_URL" "${GRPC_PUBLIC_BASE_URL}"

echo "apply_machine_grpc_app_node_env: ok (domain=${MACHINE_GRPC_DOMAIN})"
