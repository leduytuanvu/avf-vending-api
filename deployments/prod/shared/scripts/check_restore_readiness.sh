#!/usr/bin/env bash
set -Eeuo pipefail

SHARED_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
# shellcheck source=./lib_release.sh
source "${SHARED_ROOT}/scripts/lib_release.sh"

ENV_FILE_PATH="${1:-${APP_NODE_ENV_FILE_PATH:-${SHARED_ROOT}/../../app-node/.env.app-node.example}}"
DUMP_PATH="${2:-}"

require_cmd bash
require_cmd pg_dump
require_cmd pg_restore

load_env_file "${ENV_FILE_PATH}"

[[ -n "${DATABASE_URL:-}" ]] || fail "DATABASE_URL is required in ${ENV_FILE_PATH}"
require_file "${SHARED_ROOT}/scripts/backup_managed_postgres.sh"
require_file "${SHARED_ROOT}/scripts/restore_managed_postgres.sh"

bash -n "${SHARED_ROOT}/scripts/backup_managed_postgres.sh"
bash -n "${SHARED_ROOT}/scripts/restore_managed_postgres.sh"

note "restore-readiness contract checks"
echo "pg_dump version: $(pg_dump --version)"
echo "pg_restore version: $(pg_restore --version)"

if [[ "${DATABASE_URL}" == *"CHANGE_ME"* ]]; then
	note "placeholder DATABASE_URL detected; skipping live connectivity checks"
else
	note "DATABASE_URL is non-placeholder; this script still performs no destructive database action by default"
	if [[ "${CHECK_DATABASE_CONNECTIVITY:-0}" == "1" ]]; then
		require_cmd pg_isready
		pg_isready -d "${DATABASE_URL}" -t 5 >/dev/null || fail "pg_isready failed for DATABASE_URL"
		note "pg_isready passed"
	fi
fi

if [[ -n "${DUMP_PATH}" ]]; then
	require_file "${DUMP_PATH}"
	pg_restore --list "${DUMP_PATH}" >/dev/null
	note "dump file validated with pg_restore --list"
fi

echo "check_restore_readiness: PASS"
