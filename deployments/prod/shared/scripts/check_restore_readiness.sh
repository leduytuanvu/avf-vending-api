#!/usr/bin/env bash
set -Eeuo pipefail

SHARED_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
# shellcheck source=./lib_release.sh
source "${SHARED_ROOT}/scripts/lib_release.sh"

ENV_FILE_PATH="${1:-${APP_NODE_ENV_FILE_PATH:-${SHARED_ROOT}/../../app-node/.env.app-node.example}}"
DUMP_PATH="${2:-}"
REPORT_PATH="${CHECK_RESTORE_READINESS_REPORT_PATH:-}"

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

database_url_kind="configured"
connectivity_checked="false"
dump_validated="false"
restore_helper_validated="syntax-only"
readiness_verdict="pass"

if is_placeholder_value "${DATABASE_URL}"; then
	note "placeholder DATABASE_URL detected; skipping live connectivity checks"
	database_url_kind="placeholder"
	readiness_verdict="not-configured"
else
	note "DATABASE_URL is non-placeholder; this script still performs no destructive database action by default"
	if [[ "${CHECK_DATABASE_CONNECTIVITY:-0}" == "1" ]]; then
		require_cmd pg_isready
		pg_isready -d "${DATABASE_URL}" -t 5 >/dev/null || fail "pg_isready failed for DATABASE_URL"
		note "pg_isready passed"
		connectivity_checked="true"
	fi
fi

BACKUP_MANAGED_POSTGRES_REPORT_PATH="" \
	bash "${SHARED_ROOT}/scripts/backup_managed_postgres.sh" "${SHARED_ROOT}/../backups/readiness-placeholder.dump" validate >/dev/null

if [[ -n "${DUMP_PATH}" ]]; then
	require_file "${DUMP_PATH}"
	pg_restore --list "${DUMP_PATH}" >/dev/null
	note "dump file validated with pg_restore --list"
	dump_validated="true"
	restore_helper_validated="dump-inventory-validated"
fi

if [[ -n "${REPORT_PATH}" ]]; then
	{
		printf '{\n'
		printf '  "control_scope": "restore-readiness-only",\n'
		printf '  "control_status": "readiness-only",\n'
		printf '  "restore_drill_executed": false,\n'
		printf '  "env_file_path": "%s",\n' "$(json_escape "${ENV_FILE_PATH}")"
		printf '  "database_url_kind": "%s",\n' "${database_url_kind}"
		printf '  "connectivity_checked": %s,\n' "${connectivity_checked}"
		printf '  "dump_supplied": %s,\n' "$([[ -n "${DUMP_PATH}" ]] && printf 'true' || printf 'false')"
		printf '  "dump_validated": %s,\n' "${dump_validated}"
		printf '  "restore_helper_validation": "%s",\n' "${restore_helper_validated}"
		printf '  "validated": ["pg_dump availability","pg_restore availability","backup/restore helper syntax"],\n'
		printf '  "summary": "%s",\n' "$(json_escape "$(
			if [[ "${readiness_verdict}" == "not-configured" ]]; then
				printf 'restore readiness remained readiness-only because DATABASE_URL is placeholder or missing live credentials'
			else
				printf 'restore readiness validated local tooling and helper contracts only'
			fi
		)")"
		printf '  "verdict": "%s"\n' "${readiness_verdict}"
		printf '}\n'
	} >"${REPORT_PATH}"
fi

if [[ "${readiness_verdict}" == "not-configured" ]]; then
	echo "check_restore_readiness: NOT-CONFIGURED"
else
	echo "check_restore_readiness: READINESS-PASS"
fi
