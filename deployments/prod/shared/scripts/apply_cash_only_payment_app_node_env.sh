#!/usr/bin/env bash
# Ensure production app-node env uses explicit cash-only payment mode (required for pilot rollout).
# Usage: apply_cash_only_payment_app_node_env.sh [path/to/.env.app-node]
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

unset_env_kv() {
	local key="$1"
	if grep -q "^${key}=" "${ENV_FILE}"; then
		sed -i "/^${key}=/d" "${ENV_FILE}"
	fi
}

PAYMENT_ENV="${PAYMENT_ENV:-cash_only}"
set_env_kv "PAYMENT_ENV" "${PAYMENT_ENV}"
unset_env_kv "COMMERCE_PAYMENT_PROVIDER"

echo "apply_cash_only_payment_app_node_env: ok (PAYMENT_ENV=${PAYMENT_ENV}, COMMERCE_PAYMENT_PROVIDER unset)"
