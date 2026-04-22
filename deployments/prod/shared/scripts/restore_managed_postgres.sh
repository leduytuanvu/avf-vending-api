#!/usr/bin/env bash
set -Eeuo pipefail

SHARED_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
# shellcheck source=./lib_release.sh
source "${SHARED_ROOT}/scripts/lib_release.sh"

ENV_FILE_PATH="${APP_NODE_ENV_FILE_PATH:-${SHARED_ROOT}/../../app-node/.env.app-node}"
DUMP_PATH="${1:-}"
RESTORE_MODE="${2:-validate}"

[[ -n "${DUMP_PATH}" ]] || fail "usage: restore_managed_postgres.sh <dump-file> [validate|execute]"

require_cmd bash
require_cmd pg_restore
load_env_file "${ENV_FILE_PATH}"

[[ -n "${DATABASE_URL:-}" ]] || fail "DATABASE_URL is required"
require_file "${DUMP_PATH}"

note "validate managed PostgreSQL dump compatibility"
pg_restore --list "${DUMP_PATH}" >/dev/null

if [[ "${RESTORE_MODE}" != "execute" ]]; then
	echo "restore_managed_postgres: validation only"
	echo "restore_managed_postgres: this script does not perform a managed restore unless you pass 'execute'"
	echo "restore_managed_postgres: provider snapshots, point-in-time restore, and cross-environment approval remain external operational controls"
	exit 0
fi

[[ "${CONFIRM_MANAGED_RESTORE:-}" == "managed-production-restore" ]] || fail "set CONFIRM_MANAGED_RESTORE=managed-production-restore to execute a restore"

note "executing client-side restore against DATABASE_URL"
note "this is not a provider snapshot restore and assumes the target role has sufficient DDL/data privileges"
pg_restore \
	--clean \
	--if-exists \
	--no-owner \
	--no-privileges \
	--single-transaction \
	--exit-on-error \
	--dbname="${DATABASE_URL}" \
	"${DUMP_PATH}"

echo "restore_managed_postgres: PASS"
