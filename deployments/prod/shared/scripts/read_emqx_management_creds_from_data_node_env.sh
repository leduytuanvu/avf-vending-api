#!/usr/bin/env bash
# Read EMQX management API credentials from the data-node operator env file.
# Prints shell-safe export lines to stdout (never log this output in CI summaries).
set -euo pipefail

ENV_FILE="${1:-${DATA_NODE_ENV_FILE:-}}"
if [[ -z "${ENV_FILE}" ]]; then
	SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
	ENV_FILE="${SCRIPT_DIR}/../../data-node/.env.data-node"
fi

if [[ ! -f "${ENV_FILE}" ]]; then
	echo "error: data-node env file not found: ${ENV_FILE}" >&2
	exit 1
fi

read_env() {
	local key="$1"
	local line
	line="$(grep -E "^${key}=" "${ENV_FILE}" | tail -n 1 || true)"
	[[ -n "${line}" ]] || return 1
	printf '%s' "${line#*=}"
}

EMQX_API_KEY="$(read_env EMQX_API_KEY || true)"
EMQX_API_SECRET="$(read_env EMQX_API_SECRET || true)"

[[ -n "${EMQX_API_KEY}" ]] || {
	echo "error: EMQX_API_KEY missing in ${ENV_FILE}" >&2
	exit 1
}
[[ -n "${EMQX_API_SECRET}" ]] || {
	echo "error: EMQX_API_SECRET missing in ${ENV_FILE}" >&2
	exit 1
}

printf 'export EMQX_API_KEY=%q\n' "${EMQX_API_KEY}"
printf 'export EMQX_API_SECRET=%q\n' "${EMQX_API_SECRET}"
