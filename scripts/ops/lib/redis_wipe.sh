#!/usr/bin/env bash
# Destructive Redis wipe helpers (PR2). Sources redis_resolve.sh for config.
set -Eeuo pipefail

REDIS_WIPE_LIB_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=redis_resolve.sh
source "${REDIS_WIPE_LIB_DIR}/redis_resolve.sh"

redis_wipe_plan() {
	redis_resolve_config
	if [[ "${REDIS_RESOLVED_CONFIGURED}" != "true" ]]; then
		echo "redis_wipe: not configured"
		return 0
	fi
	local dedicated="${REDIS_DEDICATED_INSTANCE:-0}"
	local prefix="${REDIS_RESOLVED_KEY_PREFIX}"
	local avf_keys
	avf_keys="$(redis_avf_key_count 2>/dev/null || echo "?")"
	local dbsize
	dbsize="$(redis_dbsize 2>/dev/null || echo "?")"
	echo "redis_wipe: plan dbsize=${dbsize} avf_prefixed_keys=${avf_keys} prefix=${prefix} dedicated=${dedicated}"
}

redis_wipe_execute() {
	local dry_run="${1:-1}"
	redis_resolve_config
	[[ "${REDIS_RESOLVED_CONFIGURED}" == "true" ]] || return 0

	local prefix="${REDIS_RESOLVED_KEY_PREFIX}"
	local dedicated="${REDIS_DEDICATED_INSTANCE:-0}"

	if [[ "${dry_run}" -eq 1 ]]; then
		redis_wipe_plan
		return 0
	fi

	if [[ "${dedicated}" == "1" || "${dedicated}" == "true" ]]; then
		redis_cli_exec FLUSHDB
		return $?
	fi

	# Prefix-scoped UNLINK (default for shared instances)
	local key count=0
	while IFS= read -r key; do
		[[ -n "${key}" ]] || continue
		redis_cli_exec UNLINK "${key}" >/dev/null
		count=$((count + 1))
	done < <(redis_cli_exec --scan --pattern "${prefix}*" 2>/dev/null || true)
	echo "redis_wipe: unlinked ${count} keys matching ${prefix}*"
}
