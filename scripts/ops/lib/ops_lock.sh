#!/usr/bin/env bash
# Postgres advisory lock for ops wipe concurrency (keyed by environment + DB identity).
set -Eeuo pipefail

OPS_LOCK_HELD=0
OPS_LOCK_KEY=""

ops_lock_compute_key() {
	local env="$1"
	local db_identity="$2"
	local hash
	hash="$(printf '%s:%s' "${env}" "${db_identity}" | cksum | awk '{print $1}')"
	OPS_LOCK_KEY="${hash}"
}

ops_lock_acquire() {
	local psql_fn="$1"
	local key="${OPS_LOCK_KEY}"
	[[ -n "${key}" ]] || return 1
	if "${psql_fn}" "SELECT pg_try_advisory_lock(${key});" | grep -qx t; then
		OPS_LOCK_HELD=1
		return 0
	fi
	echo "ops_lock: another wipe/destroy may be in progress (advisory lock ${key})" >&2
	return 1
}

ops_lock_release() {
	local psql_fn="$1"
	[[ "${OPS_LOCK_HELD}" -eq 1 ]] || return 0
	"${psql_fn}" "SELECT pg_advisory_unlock(${OPS_LOCK_KEY});" >/dev/null 2>&1 || true
	OPS_LOCK_HELD=0
}
