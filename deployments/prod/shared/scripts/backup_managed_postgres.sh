#!/usr/bin/env bash
set -Eeuo pipefail

SHARED_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
# shellcheck source=./lib_release.sh
source "${SHARED_ROOT}/scripts/lib_release.sh"

ENV_FILE_PATH="${APP_NODE_ENV_FILE_PATH:-${SHARED_ROOT}/../app-node/.env.app-node}"
REPORT_PATH="${BACKUP_MANAGED_POSTGRES_REPORT_PATH:-}"
OUTPUT_PATH=""
BACKUP_MODE="execute"

usage() {
	echo "usage: backup_managed_postgres.sh [validate|execute] | [output-path] [validate|execute]" >&2
}

if [[ $# -gt 0 ]]; then
	case "$1" in
	validate | execute)
		BACKUP_MODE="$1"
		shift
		;;
	-*)
		usage
		fail "unknown argument: $1"
		;;
	*)
		OUTPUT_PATH="$1"
		shift
		;;
	esac
fi

if [[ $# -gt 0 ]]; then
	case "$1" in
	validate | execute)
		BACKUP_MODE="$1"
		shift
		;;
	*)
		usage
		fail "unsupported backup mode: $1"
		;;
	esac
fi

[[ $# -eq 0 ]] || {
	usage
	fail "too many arguments"
}

require_cmd bash
require_cmd pg_dump
load_env_file "${ENV_FILE_PATH}"

[[ -n "${DATABASE_URL:-}" ]] || fail "DATABASE_URL is required"

note "client-side managed PostgreSQL backup"
note "this script uses pg_dump against DATABASE_URL; it does not trigger provider snapshots or backups"

if is_placeholder_value "${DATABASE_URL}"; then
	database_url_kind="placeholder"
else
	database_url_kind="configured"
fi

backup_source_identifier="${BACKUP_SOURCE_IDENTIFIER:-}"
if [[ -z "${backup_source_identifier}" && "${database_url_kind}" == "configured" ]]; then
	backup_source_identifier="$(postgres_identifier_from_url "${DATABASE_URL}")"
fi

write_report() {
	local control_scope="$1"
	local mode="$2"
	local verdict="$3"
	local control_status="$4"
	local backup_executed="$5"
	local backup_artifact_created="$6"
	local summary="$7"
	local connectivity_checked="${8:-false}"
	local report_output_path="${9:-}"
	local report_sha256_path="${10:-}"
	local backup_size_bytes="${11:-0}"
	local backup_started_at_utc="${12:-}"
	local backup_completed_at_utc="${13:-}"

	[[ -n "${REPORT_PATH}" ]] || return 0

	mkdir -p "$(dirname "${REPORT_PATH}")"
	{
		printf '{\n'
		printf '  "control_scope": "%s",\n' "$(json_escape "${control_scope}")"
		printf '  "mode": "%s",\n' "$(json_escape "${mode}")"
		printf '  "control_status": "%s",\n' "$(json_escape "${control_status}")"
		printf '  "env_file_path": "%s",\n' "$(json_escape "${ENV_FILE_PATH}")"
		printf '  "database_url_kind": "%s",\n' "$(json_escape "${database_url_kind}")"
		printf '  "backup_source_identifier": "%s",\n' "$(json_escape "${backup_source_identifier}")"
		printf '  "backup_executed": %s,\n' "${backup_executed}"
		printf '  "backup_artifact_created": %s,\n' "${backup_artifact_created}"
		printf '  "output_path": "%s",\n' "$(json_escape "${report_output_path}")"
		printf '  "sha256_path": "%s",\n' "$(json_escape "${report_sha256_path}")"
		printf '  "connectivity_checked": %s,\n' "${connectivity_checked}"
		printf '  "backup_size_bytes": %s,\n' "${backup_size_bytes}"
		printf '  "backup_started_at_utc": "%s",\n' "$(json_escape "${backup_started_at_utc}")"
		printf '  "backup_completed_at_utc": "%s",\n' "$(json_escape "${backup_completed_at_utc}")"
		printf '  "validated": ["pg_dump availability","DATABASE_URL contract"],\n'
		printf '  "summary": "%s",\n' "$(json_escape "${summary}")"
		printf '  "verdict": "%s"\n' "$(json_escape "${verdict}")"
		printf '}\n'
	} >"${REPORT_PATH}"
}

case "${BACKUP_MODE}" in
validate)
	note "backup readiness validation only"
	connectivity_checked="false"
	if [[ "${database_url_kind}" == "placeholder" ]]; then
		note "placeholder DATABASE_URL detected; validating backup tooling and contract only"
		write_report \
			"backup-readiness-only" \
			"validate" \
			"not-configured" \
			"readiness-only" \
			"false" \
			"false" \
			"backup readiness remained readiness-only because DATABASE_URL is placeholder or missing live credentials; no backup artifact was created" \
			"${connectivity_checked}"
		echo "backup_managed_postgres: READINESS-ONLY-NOT-CONFIGURED"
		exit 0
	fi

	if [[ "${CHECK_DATABASE_CONNECTIVITY:-0}" == "1" ]]; then
		require_cmd pg_isready
		pg_isready -d "${DATABASE_URL}" -t 5 >/dev/null
		connectivity_checked="true"
		note "pg_isready passed for configured DATABASE_URL"
	fi

	write_report \
		"backup-readiness-only" \
		"validate" \
		"pass" \
		"readiness-only" \
		"false" \
		"false" \
		"backup readiness-only check validated tooling and DATABASE_URL contract; no backup artifact was created" \
		"${connectivity_checked}"
	echo "backup_managed_postgres: READINESS-ONLY-PASS"
	;;
execute)
	backup_started_at_utc="$(utc_timestamp)"
	if [[ "${database_url_kind}" != "configured" ]]; then
		write_report \
			"backup-artifact-generation" \
			"execute" \
			"not-configured" \
			"unmet" \
			"false" \
			"false" \
			"backup execution did not run because DATABASE_URL is placeholder or missing live credentials" \
			"false" \
			"" \
			"" \
			"0" \
			"${backup_started_at_utc}" \
			""
		fail "execute mode requires a non-placeholder DATABASE_URL"
	fi

	if [[ -z "${OUTPUT_PATH}" ]]; then
		timestamp="$(date -u +"%Y%m%dT%H%M%SZ")"
		OUTPUT_PATH="${MANAGED_POSTGRES_BACKUP_DIR:-${SHARED_ROOT}/../backups}/managed-postgres-${timestamp}.dump"
	fi
	mkdir -p "$(dirname "${OUTPUT_PATH}")"

	dump_url="${BACKUP_DATABASE_URL:-${DATABASE_URL}}"
	if [[ -n "${BACKUP_DATABASE_URL:-}" ]]; then
		note "using BACKUP_DATABASE_URL for pg_dump"
	fi

	attempt=1
	max_attempts=5
	err_file="$(mktemp)"
	while [[ "${attempt}" -le "${max_attempts}" ]]; do
		if pg_dump \
			--format=custom \
			--compress=9 \
			--no-owner \
			--no-privileges \
			--file "${OUTPUT_PATH}" \
			"${dump_url}" 2>"${err_file}"; then
			rm -f "${err_file}"
			break
		fi
		if grep -Eiq 'max clients reached|EMAXCONNSESSION|too many clients' "${err_file}" && [[ "${attempt}" -lt "${max_attempts}" ]]; then
			note "pg_dump pool limit (attempt ${attempt}/${max_attempts}); retrying in $((attempt * 5))s"
			sleep $((attempt * 5))
			attempt=$((attempt + 1))
			continue
		fi
		cat "${err_file}" >&2
		rm -f "${err_file}"
		write_report \
			"backup-artifact-generation" \
			"execute" \
			"fail" \
			"unmet" \
			"true" \
			"false" \
			"pg_dump failed; no backup artifact was produced" \
			"false" \
			"${OUTPUT_PATH}" \
			"" \
			"0" \
			"${backup_started_at_utc}" \
			"$(utc_timestamp)"
		fail "pg_dump failed for DATABASE_URL"
	done
	rm -f "${err_file}"
	if [[ ! -s "${OUTPUT_PATH}" ]]; then
		write_report \
			"backup-artifact-generation" \
			"execute" \
			"fail" \
			"unmet" \
			"true" \
			"false" \
			"pg_dump produced an empty backup artifact" \
			"false" \
			"${OUTPUT_PATH}" \
			"" \
			"0" \
			"${backup_started_at_utc}" \
			"$(utc_timestamp)"
		fail "pg_dump produced empty file"
	fi

	sha256_path=""
	if command -v sha256sum >/dev/null 2>&1; then
		sha256_path="${OUTPUT_PATH}.sha256"
		sha256sum "${OUTPUT_PATH}" >"${sha256_path}"
	fi

	backup_size_bytes="$(wc -c <"${OUTPUT_PATH}" | tr -d ' ')"
	write_report \
		"backup-artifact-generation" \
		"execute" \
		"pass" \
		"met" \
		"true" \
		"true" \
		"actual client-side pg_dump completed against the configured backup source and created a backup artifact" \
		"false" \
		"${OUTPUT_PATH}" \
		"${sha256_path}" \
		"${backup_size_bytes:-0}" \
		"${backup_started_at_utc}" \
		"$(utc_timestamp)"
	echo "backup_managed_postgres: EXECUTION-PASS (${OUTPUT_PATH})"
	;;
*)
	usage
	fail "unsupported backup mode: ${BACKUP_MODE}"
	;;
esac
