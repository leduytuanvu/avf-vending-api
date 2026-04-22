#!/usr/bin/env bash
set -Eeuo pipefail

SHARED_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
# shellcheck source=./lib_release.sh
source "${SHARED_ROOT}/scripts/lib_release.sh"

ENV_FILE_PATH="${APP_NODE_ENV_FILE_PATH:-${SHARED_ROOT}/../../app-node/.env.app-node}"
DUMP_PATH="${1:-}"
RESTORE_MODE="${2:-validate}"
REPORT_PATH="${RESTORE_MANAGED_POSTGRES_REPORT_PATH:-}"

[[ -n "${DUMP_PATH}" ]] || fail "usage: restore_managed_postgres.sh <dump-file> [validate|execute]"

require_cmd bash
require_cmd pg_restore
load_env_file "${ENV_FILE_PATH}"

[[ -n "${DATABASE_URL:-}" ]] || fail "DATABASE_URL is required"
require_file "${DUMP_PATH}"

note "validate managed PostgreSQL dump compatibility"
pg_restore --list "${DUMP_PATH}" >/dev/null

if [[ "${RESTORE_MODE}" != "execute" ]]; then
	if [[ -n "${REPORT_PATH}" ]]; then
		printf '{\n' >"${REPORT_PATH}"
		printf '  "mode": "validate",\n' >>"${REPORT_PATH}"
		printf '  "env_file_path": "%s",\n' "$(json_escape "${ENV_FILE_PATH}")" >>"${REPORT_PATH}"
		printf '  "dump_path": "%s",\n' "$(json_escape "${DUMP_PATH}")" >>"${REPORT_PATH}"
		printf '  "database_url_kind": "%s",\n' "$(
			if is_placeholder_value "${DATABASE_URL}"; then
				printf 'placeholder'
			else
				printf 'configured'
			fi
		)" >>"${REPORT_PATH}"
		printf '  "validated": ["pg_restore availability","dump inventory via pg_restore --list"],\n' >>"${REPORT_PATH}"
		printf '  "verdict": "pass"\n' >>"${REPORT_PATH}"
		printf '}\n' >>"${REPORT_PATH}"
	fi
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

if [[ -n "${REPORT_PATH}" ]]; then
	printf '{\n' >"${REPORT_PATH}"
	printf '  "mode": "execute",\n' >>"${REPORT_PATH}"
	printf '  "env_file_path": "%s",\n' "$(json_escape "${ENV_FILE_PATH}")" >>"${REPORT_PATH}"
	printf '  "dump_path": "%s",\n' "$(json_escape "${DUMP_PATH}")" >>"${REPORT_PATH}"
	printf '  "database_url_kind": "%s",\n' "$(
		if is_placeholder_value "${DATABASE_URL}"; then
			printf 'placeholder'
		else
			printf 'configured'
		fi
	)" >>"${REPORT_PATH}"
	printf '  "validated": ["pg_restore availability","dump inventory via pg_restore --list","client-side pg_restore execution"],\n' >>"${REPORT_PATH}"
	printf '  "verdict": "pass"\n' >>"${REPORT_PATH}"
	printf '}\n' >>"${REPORT_PATH}"
fi

echo "restore_managed_postgres: PASS"
