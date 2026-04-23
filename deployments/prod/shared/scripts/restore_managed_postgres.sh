#!/usr/bin/env bash
set -Eeuo pipefail

SHARED_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
# shellcheck source=./lib_release.sh
source "${SHARED_ROOT}/scripts/lib_release.sh"

ENV_FILE_PATH="${APP_NODE_ENV_FILE_PATH:-${SHARED_ROOT}/../app-node/.env.app-node}"
DUMP_PATH="${1:-}"
RESTORE_MODE="${2:-validate}"
REPORT_PATH="${RESTORE_MANAGED_POSTGRES_REPORT_PATH:-}"

[[ -n "${DUMP_PATH}" ]] || fail "usage: restore_managed_postgres.sh <dump-file> [validate|execute|drill]"

require_cmd bash
require_cmd pg_restore
require_cmd psql
require_cmd python3
load_env_file "${ENV_FILE_PATH}"

[[ -n "${DATABASE_URL:-}" ]] || fail "DATABASE_URL is required"
require_file "${DUMP_PATH}"

if is_placeholder_value "${DATABASE_URL}"; then
	database_url_kind="placeholder"
else
	database_url_kind="configured"
fi

restore_target_identifier="${RESTORE_TARGET_IDENTIFIER:-}"
if [[ -z "${restore_target_identifier}" && "${database_url_kind}" == "configured" ]]; then
	restore_target_identifier="$(postgres_identifier_from_url "${DATABASE_URL}")"
fi
backup_source_identifier="${RESTORE_BACKUP_SOURCE_IDENTIFIER:-}"

note "validate managed PostgreSQL dump compatibility"
pg_restore --list "${DUMP_PATH}" >/dev/null

if [[ "${RESTORE_MODE}" == "validate" ]]; then
	validation_verdict="pass"
	if [[ "${database_url_kind}" != "configured" ]]; then
		validation_verdict="not-configured"
	fi
	if [[ -n "${REPORT_PATH}" ]]; then
		{
			printf '{\n'
			printf '  "control_scope": "restore-readiness-only",\n'
			printf '  "control_status": "readiness-only",\n'
			printf '  "restore_drill_executed": false,\n'
			printf '  "mode": "validate",\n'
			printf '  "env_file_path": "%s",\n' "$(json_escape "${ENV_FILE_PATH}")"
			printf '  "dump_path": "%s",\n' "$(json_escape "${DUMP_PATH}")"
			printf '  "database_url_kind": "%s",\n' "${database_url_kind}"
			printf '  "restore_target_identifier": "%s",\n' "$(json_escape "${restore_target_identifier}")"
			printf '  "validated": ["pg_restore availability","dump inventory via pg_restore --list"],\n'
			printf '  "summary": "%s",\n' "$(json_escape "$(
				if [[ "${validation_verdict}" == "not-configured" ]]; then
					printf 'restore readiness remained readiness-only because DATABASE_URL is placeholder or missing live credentials'
				else
					printf 'restore readiness validated tooling and dump inventory only'
				fi
			)")"
			printf '  "verdict": "%s"\n' "${validation_verdict}"
			printf '}\n'
		} >"${REPORT_PATH}"
	fi
	echo "restore_managed_postgres: validation only"
	echo "restore_managed_postgres: this script does not perform a restore unless you pass 'execute' or 'drill'"
	echo "restore_managed_postgres: provider snapshots, point-in-time restore, and cross-environment approval remain external operational controls"
	exit 0
fi

case "${RESTORE_MODE}" in
execute)
	[[ "${CONFIRM_MANAGED_RESTORE:-}" == "managed-production-restore" ]] || fail "set CONFIRM_MANAGED_RESTORE=managed-production-restore to execute a restore"
	;;
drill)
	[[ "${database_url_kind}" == "configured" ]] || fail "drill mode requires a non-placeholder disposable restore target DATABASE_URL"
	[[ "${RESTORE_DRILL_CONFIRMATION:-}" == "disposable-restore-drill" ]] || fail "set RESTORE_DRILL_CONFIRMATION=disposable-restore-drill to run a restore drill"
	[[ "$(normalize_bool "${RESTORE_TARGET_DISPOSABLE:-0}")" == "1" ]] || fail "set RESTORE_TARGET_DISPOSABLE=1 to confirm the restore target is disposable"
	[[ -n "${restore_target_identifier}" ]] || fail "set RESTORE_TARGET_IDENTIFIER or provide a parseable DATABASE_URL for the disposable restore target"
	;;
*)
	fail "usage: restore_managed_postgres.sh <dump-file> [validate|execute|drill]"
	;;
esac

note "executing client-side restore against DATABASE_URL"
if [[ "${RESTORE_MODE}" == "drill" ]]; then
	note "restore drill mode: target must be a disposable restore environment only"
else
	note "this is not a provider snapshot restore and assumes the target role has sufficient DDL/data privileges"
fi
pg_restore \
	--clean \
	--if-exists \
	--no-owner \
	--no-privileges \
	--single-transaction \
	--exit-on-error \
	--dbname="${DATABASE_URL}" \
	"${DUMP_PATH}"

verification_steps='["pg_restore availability","dump inventory via pg_restore --list","client-side pg_restore execution"]'
summary="client-side restore executed"

if [[ "${RESTORE_MODE}" == "drill" ]]; then
	target_database_name="$(psql "${DATABASE_URL}" -Atqc 'select current_database()')"
	[[ -n "${target_database_name}" ]] || fail "restore drill verification failed to resolve current_database()"
	non_system_table_count="$(psql "${DATABASE_URL}" -Atqc "select count(*) from information_schema.tables where table_schema not in ('pg_catalog','information_schema')")"
	dump_table_count="$(pg_restore --list "${DUMP_PATH}" | python3 -c 'import sys; print(sum(1 for line in sys.stdin if "; TABLE " in line))')"
	if [[ "${dump_table_count}" =~ ^[0-9]+$ ]] && [[ "${non_system_table_count}" =~ ^[0-9]+$ ]] && ((dump_table_count > 0)) && ((non_system_table_count == 0)); then
		fail "restore drill verification found zero restored non-system tables on the disposable target"
	fi
	verification_steps='["pg_restore availability","dump inventory via pg_restore --list","client-side pg_restore execution","target connectivity via psql","current_database() verification","non-system table inventory check"]'
	summary="real restore drill executed against the configured disposable target and verified with read-only post-restore checks"
fi

if [[ -n "${REPORT_PATH}" ]]; then
	{
		printf '{\n'
		printf '  "control_scope": "%s",\n' "$(
			if [[ "${RESTORE_MODE}" == "drill" ]]; then
				printf 'restore-drill'
			else
				printf 'restore-execution'
			fi
		)"
		printf '  "control_status": "%s",\n' "$(
			if [[ "${RESTORE_MODE}" == "drill" ]]; then
				printf 'met'
			else
				printf 'manual-restore-executed'
			fi
		)"
		printf '  "restore_drill_executed": %s,\n' "$([[ "${RESTORE_MODE}" == "drill" ]] && printf 'true' || printf 'false')"
		printf '  "mode": "%s",\n' "${RESTORE_MODE}"
		printf '  "env_file_path": "%s",\n' "$(json_escape "${ENV_FILE_PATH}")"
		printf '  "dump_path": "%s",\n' "$(json_escape "${DUMP_PATH}")"
		printf '  "database_url_kind": "%s",\n' "${database_url_kind}"
		printf '  "backup_source_identifier": "%s",\n' "$(json_escape "${backup_source_identifier}")"
		printf '  "restore_target_identifier": "%s",\n' "$(json_escape "${restore_target_identifier}")"
		if [[ "${RESTORE_MODE}" == "drill" ]]; then
			printf '  "target_database_name": "%s",\n' "$(json_escape "${target_database_name}")"
			printf '  "non_system_table_count": %s,\n' "${non_system_table_count}"
			printf '  "dump_table_count": %s,\n' "${dump_table_count}"
		fi
		printf '  "verification_steps_executed": %s,\n' "${verification_steps}"
		printf '  "summary": "%s",\n' "$(json_escape "${summary}")"
		printf '  "verdict": "pass"\n'
		printf '}\n'
	} >"${REPORT_PATH}"
fi

echo "restore_managed_postgres: PASS"
