#!/usr/bin/env bash
# Run full public-schema row-count audit for development, staging, or production.
#
# Usage:
#   bash scripts/ops/run-table-data-audit.sh --environment development
#   bash scripts/ops/run-table-data-audit.sh --environment staging
#   bash scripts/ops/run-table-data-audit.sh --environment production
set -Eeuo pipefail

# Git Bash on Windows rewrites container paths (e.g. /tmp -> host Temp); disable for docker CLI.
case "$(uname -s 2>/dev/null)" in
MINGW* | MSYS*) export MSYS_NO_PATHCONV=1 ;;
esac

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
OPS="${ROOT}/scripts/ops"
LIB="${OPS}/lib"
# shellcheck source=lib/python3_shim.sh
source "${LIB}/python3_shim.sh"
AUDIT_SQL="${OPS}/audit-all-table-rowcounts.sql"
POSTGRES_TOOLS_IMAGE="${POSTGRES_TOOLS_IMAGE:-postgres:17-alpine}"
EVIDENCE_DIR="${ROOT}/.db-destroy-evidence"

ENVIRONMENT=""
SUMMARY_ONLY=0

fail() {
	echo "run-table-data-audit: error: $*" >&2
	exit 1
}

note() {
	echo "run-table-data-audit: $*"
}

usage() {
	cat <<EOF
usage: bash scripts/ops/run-table-data-audit.sh --environment ENV [options]

  development | staging | production

Options:
  --summary-only      Reserved for future use (audit always lists nonempty tables)
EOF
}

ops_prepare_python3_shim "${ROOT}" || fail "unable to prepare python3 shim for verify_database_environment"

database_url_for_docker_exec() {
	local url="$1"
	if docker ps --format '{{.Names}}' 2>/dev/null | grep -qx avf-postgres; then
		echo "${url}" | sed -E 's/@(127\.0\.0\.1|localhost):15432/@127.0.0.1:5432/'
	else
		echo "${url}"
	fi
}

probe_database_url() {
	local candidate="$1"
	local exec_url
	if docker ps --format '{{.Names}}' 2>/dev/null | grep -qx avf-postgres; then
		exec_url="$(database_url_for_docker_exec "${candidate}")"
		if docker exec avf-postgres psql "${exec_url}" -v ON_ERROR_STOP=1 -Atqc "SELECT 1" >/dev/null 2>&1; then
			echo "${candidate}"
			return 0
		fi
	fi
	if command -v psql >/dev/null 2>&1; then
		if psql "${candidate}" -v ON_ERROR_STOP=1 -Atqc "SELECT 1" >/dev/null 2>&1; then
			echo "${candidate}"
			return 0
		fi
	fi
	if command -v docker >/dev/null 2>&1; then
		if docker run --rm "${POSTGRES_TOOLS_IMAGE}" psql "${candidate}" -v ON_ERROR_STOP=1 -Atqc "SELECT 1" >/dev/null 2>&1; then
			echo "${candidate}"
			return 0
		fi
	fi
	return 1
}

resolve_dev_database_url() {
	if [[ -n "${DATABASE_URL:-}" ]]; then
		echo "${DATABASE_URL}"
		return 0
	fi
	local url
	for url in \
		"postgres://postgres:postgres@127.0.0.1:15432/avf_vending?sslmode=disable" \
		"postgres://postgres:postgres@localhost:15432/avf_vending?sslmode=disable" \
		"postgres://postgres:postgres@localhost:5432/avf_vending?sslmode=disable"; do
		if probe_database_url "${url}"; then
			echo "${url}"
			return 0
		fi
	done
	fail "could not reach development database; set DATABASE_URL"
}

load_environment() {
	case "${ENVIRONMENT}" in
	development)
		export APP_ENV=development
		export PAYMENT_ENV="${PAYMENT_ENV:-sandbox}"
		DATABASE_URL="$(resolve_dev_database_url)"
		export DATABASE_URL
		ENV_FILE=""
		;;
	staging)
		export APP_ENV=staging
		export PAYMENT_ENV="${PAYMENT_ENV:-sandbox}"
		ENV_FILE="${ROOT}/deployments/staging/.env.staging"
		if [[ -n "${STAGING_DATABASE_URL:-}" ]]; then
			DATABASE_URL="${STAGING_DATABASE_URL}"
			export DATABASE_URL
		elif [[ -f "${ENV_FILE}" ]]; then
			set -a
			# shellcheck disable=SC1090
			source "${ENV_FILE}"
			set +a
		else
			fail "missing ${ENV_FILE} and STAGING_DATABASE_URL — run on staging VPS or export STAGING_DATABASE_URL"
		fi
		[[ -n "${DATABASE_URL:-}" ]] || fail "DATABASE_URL is empty"
		;;
	production)
		export APP_ENV=production
		ENV_FILE="${ROOT}/deployments/prod/app-node/.env.app-node"
		if [[ -n "${PRODUCTION_DATABASE_URL:-}" ]]; then
			DATABASE_URL="${PRODUCTION_DATABASE_URL}"
			export DATABASE_URL
		elif [[ -f "${ENV_FILE}" ]]; then
			set -a
			# shellcheck disable=SC1090
			source "${ENV_FILE}"
			set +a
			if [[ -z "${DATABASE_URL:-}" ]]; then
				DATABASE_URL="$(grep -E '^DATABASE_URL=' "${ENV_FILE}" | tail -n1 | cut -d= -f2- | tr -d '\r' | sed -e 's/^"//' -e 's/"$//')"
				export DATABASE_URL
			fi
		else
			fail "missing ${ENV_FILE} and PRODUCTION_DATABASE_URL — run on app-node or export PRODUCTION_DATABASE_URL"
		fi
		[[ -n "${DATABASE_URL:-}" ]] || fail "DATABASE_URL is empty"
		;;
	*)
		fail "unsupported environment: ${ENVIRONMENT}"
		;;
	esac
}

docker_cp_ops_to_postgres() {
	local ops_mount="$1"
	local ops_host_path
	docker exec avf-postgres mkdir -p "${ops_mount}"
	if ops_host_path="$(cd "${OPS}" && pwd -W 2>/dev/null)"; then
		MSYS_NO_PATHCONV=1 docker cp "${ops_host_path}/." "avf-postgres:${ops_mount}/" >/dev/null
	else
		docker cp "${OPS}/." "avf-postgres:${ops_mount}/" >/dev/null
	fi
}

psql_exec_file() {
	local sql_file="$1"
	local exec_url
	local ops_mount="/tmp/avf-ops"
	if [[ "${ENVIRONMENT}" == "development" ]] && docker ps --format '{{.Names}}' 2>/dev/null | grep -qx avf-postgres; then
		exec_url="$(database_url_for_docker_exec "${DATABASE_URL}")"
		docker_cp_ops_to_postgres "${ops_mount}"
		docker exec avf-postgres psql "${exec_url}" -v ON_ERROR_STOP=1 \
			-f "${ops_mount}/$(basename "${sql_file}")"
		return 0
	fi
	if command -v psql >/dev/null 2>&1; then
		(cd "${OPS}" && psql "${DATABASE_URL}" -v ON_ERROR_STOP=1 -f "$(basename "${sql_file}")")
		return 0
	fi
	if [[ -n "${ENV_FILE:-}" && -f "${ENV_FILE}" ]]; then
		docker run --rm \
			--env-file "${ENV_FILE}" \
			-e "DATABASE_URL=${DATABASE_URL}" \
			-v "${OPS}:/ops:ro" \
			"${POSTGRES_TOOLS_IMAGE}" \
			psql "${DATABASE_URL}" -v ON_ERROR_STOP=1 -f "/ops/$(basename "${sql_file}")"
		return 0
	fi
	docker run --rm \
		-e "DATABASE_URL=${DATABASE_URL}" \
		-v "${OPS}:/ops:ro" \
		"${POSTGRES_TOOLS_IMAGE}" \
		psql "${DATABASE_URL}" -v ON_ERROR_STOP=1 -f "/ops/$(basename "${sql_file}")"
}

while [[ $# -gt 0 ]]; do
	case "$1" in
	--environment)
		ENVIRONMENT="${2:-}"
		shift 2
		;;
	--summary-only)
		SUMMARY_ONLY=1
		shift
		;;
	-h | --help)
		usage
		exit 0
		;;
	*)
		fail "unknown argument: $1"
		;;
	esac
done

[[ -n "${ENVIRONMENT}" ]] || fail "--environment is required"
[[ -f "${AUDIT_SQL}" ]] || fail "missing ${AUDIT_SQL}"

mkdir -p "${EVIDENCE_DIR}"
EVIDENCE_FILE="${EVIDENCE_DIR}/table-audit-${ENVIRONMENT}-$(date -u +%Y%m%dT%H%M%SZ).log"

load_environment
export APP_ENV
export DATABASE_URL
export PAYMENT_ENV="${PAYMENT_ENV:-sandbox}"
bash "${ROOT}/scripts/verify_database_environment.sh"

note "environment=${ENVIRONMENT} audit=${AUDIT_SQL}"
note "evidence=${EVIDENCE_FILE}"

{
	echo "environment=${ENVIRONMENT}"
	echo "ts=$(date -u +%Y-%m-%dT%H:%M:%SZ)"
	psql_exec_file "${AUDIT_SQL}"
} 2>&1 | tee "${EVIDENCE_FILE}"

note "Done. Evidence: ${EVIDENCE_FILE}"
