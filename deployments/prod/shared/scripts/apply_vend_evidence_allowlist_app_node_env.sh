#!/usr/bin/env bash
# Apply machine-scoped vend hardware evidence allowlist on app-node env (Gate 4 pilot).
# Usage: apply_vend_evidence_allowlist_app_node_env.sh [path/to/.env.app-node]
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

MACHINE_IDS="${COMMERCE_REQUIRE_VEND_HARDWARE_EVIDENCE_MACHINE_IDS:-019e702c-11c6-7ab0-89c7-5eb32f0b12cb}"

set_env_kv() {
	local key="$1"
	local value="$2"
	if grep -q "^${key}=" "${ENV_FILE}"; then
		sed -i "s|^${key}=.*|${key}=${value}|" "${ENV_FILE}"
	else
		printf '%s=%s\n' "${key}" "${value}" >>"${ENV_FILE}"
	fi
}

set_env_kv "COMMERCE_REQUIRE_VEND_HARDWARE_EVIDENCE" "false"
set_env_kv "COMMERCE_REQUIRE_VEND_HARDWARE_EVIDENCE_MACHINE_IDS" "${MACHINE_IDS}"

echo "apply_vend_evidence_allowlist_app_node_env: ok (global=false, machine_ids_count=$(echo "${MACHINE_IDS}" | tr ',' '\n' | grep -c . || true))"
