#!/usr/bin/env bash
set -Eeuo pipefail

SHARED_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
# shellcheck source=./lib_release.sh
source "${SHARED_ROOT}/scripts/lib_release.sh"

ENV_FILE_PATH="${APP_NODE_ENV_FILE_PATH:-${SHARED_ROOT}/../../app-node/.env.app-node}"
OUTPUT_PATH="${1:-}"
BACKUP_MODE="${2:-execute}"
REPORT_PATH="${BACKUP_MANAGED_POSTGRES_REPORT_PATH:-}"

require_cmd bash
require_cmd pg_dump
load_env_file "${ENV_FILE_PATH}"

[[ -n "${DATABASE_URL:-}" ]] || fail "DATABASE_URL is required"

timestamp="$(date -u +"%Y%m%dT%H%M%SZ")"
if [[ -z "${OUTPUT_PATH}" ]]; then
	OUTPUT_PATH="${MANAGED_POSTGRES_BACKUP_DIR:-${SHARED_ROOT}/../backups}/managed-postgres-${timestamp}.dump"
fi

mkdir -p "$(dirname "${OUTPUT_PATH}")"

note "client-side managed PostgreSQL backup"
note "this script uses pg_dump against DATABASE_URL; it does not trigger provider snapshots or backups"
if is_placeholder_value "${DATABASE_URL}"; then
	database_url_kind="placeholder"
else
	database_url_kind="configured"
fi

case "${BACKUP_MODE}" in
execute)
	pg_dump \
		--format=custom \
		--compress=9 \
		--no-owner \
		--no-privileges \
		--file "${OUTPUT_PATH}" \
		"${DATABASE_URL}"

	sha256_path=""
	if command -v sha256sum >/dev/null 2>&1; then
		sha256_path="${OUTPUT_PATH}.sha256"
		sha256sum "${OUTPUT_PATH}" >"${sha256_path}"
	fi

	if [[ -n "${REPORT_PATH}" ]]; then
		backup_size_bytes="$(wc -c <"${OUTPUT_PATH}" | tr -d ' ')"
		printf '{\n' >"${REPORT_PATH}"
		printf '  "mode": "execute",\n' >>"${REPORT_PATH}"
		printf '  "env_file_path": "%s",\n' "$(json_escape "${ENV_FILE_PATH}")" >>"${REPORT_PATH}"
		printf '  "output_path": "%s",\n' "$(json_escape "${OUTPUT_PATH}")" >>"${REPORT_PATH}"
		printf '  "sha256_path": "%s",\n' "$(json_escape "${sha256_path}")" >>"${REPORT_PATH}"
		printf '  "database_url_kind": "%s",\n' "${database_url_kind}" >>"${REPORT_PATH}"
		printf '  "backup_size_bytes": %s,\n' "${backup_size_bytes:-0}" >>"${REPORT_PATH}"
		printf '  "verdict": "pass"\n' >>"${REPORT_PATH}"
		printf '}\n' >>"${REPORT_PATH}"
	fi

	echo "backup_managed_postgres: PASS (${OUTPUT_PATH})"
	;;
validate)
	note "backup readiness validation only"
	if is_placeholder_value "${DATABASE_URL}"; then
		note "placeholder DATABASE_URL detected; validating backup tooling and contract only"
		connectivity_checked="false"
	else
		connectivity_checked="false"
		if [[ "${CHECK_DATABASE_CONNECTIVITY:-0}" == "1" ]]; then
			require_cmd pg_isready
			pg_isready -d "${DATABASE_URL}" -t 5 >/dev/null
			connectivity_checked="true"
			note "pg_isready passed for configured DATABASE_URL"
		fi
	fi

	if [[ -n "${REPORT_PATH}" ]]; then
		printf '{\n' >"${REPORT_PATH}"
		printf '  "mode": "validate",\n' >>"${REPORT_PATH}"
		printf '  "env_file_path": "%s",\n' "$(json_escape "${ENV_FILE_PATH}")" >>"${REPORT_PATH}"
		printf '  "output_path": "%s",\n' "$(json_escape "${OUTPUT_PATH}")" >>"${REPORT_PATH}"
		printf '  "database_url_kind": "%s",\n' "${database_url_kind}" >>"${REPORT_PATH}"
		printf '  "connectivity_checked": %s,\n' "${connectivity_checked}" >>"${REPORT_PATH}"
		printf '  "validated": ["pg_dump availability","DATABASE_URL contract"],\n' >>"${REPORT_PATH}"
		printf '  "verdict": "pass"\n' >>"${REPORT_PATH}"
		printf '}\n' >>"${REPORT_PATH}"
	fi

	echo "backup_managed_postgres: validation only (${OUTPUT_PATH})"
	;;
*)
	fail "usage: backup_managed_postgres.sh [output-path] [execute|validate]"
	;;
esac
