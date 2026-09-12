#!/usr/bin/env bash
# Unified multi-environment data wipe: TRUNCATE all business data, preserve schema.
# Does NOT drop databases or run goose down.
#
# Default: dry-run (no mutation). Pass --execute to perform destructive actions.
#
# Usage:
#   bash scripts/ops/run-environment-data-wipe.sh --environment development
#
#   CONFIRM_DEV_DATA_WIPE=AVF-WIPE-DEV-DATA \
#     bash scripts/ops/run-environment-data-wipe.sh --environment development --execute --phase all
#
#   CONFIRM_PRODUCTION_DATA_WIPE=I_UNDERSTAND_THIS_WIPES_PRODUCTION \
#     bash scripts/ops/run-environment-data-wipe.sh --environment production --execute \
#       --allow-production --confirmation "WIPE PRODUCTION avf_vending_prod" --phase all
#
# Phases: preflight | postgres | redis | media | emqx | nats | temporal | verify | all
set -Eeuo pipefail

# Git Bash on Windows rewrites container paths (e.g. /tmp -> host Temp); disable for docker CLI.
case "$(uname -s 2>/dev/null)" in
MINGW* | MSYS*) export MSYS_NO_PATHCONV=1 ;;
esac

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
OPS="${ROOT}/scripts/ops"
LIB="${OPS}/lib"
# shellcheck source=lib/redis_resolve.sh
source "${LIB}/redis_resolve.sh"
# shellcheck source=lib/redis_wipe.sh
source "${LIB}/redis_wipe.sh"
# shellcheck source=lib/ops_evidence.sh
source "${LIB}/ops_evidence.sh"
# shellcheck source=lib/ops_lock.sh
source "${LIB}/ops_lock.sh"
# shellcheck source=lib/python3_shim.sh
source "${LIB}/python3_shim.sh"
DB_SCRIPTS="${ROOT}/scripts/db"
DEPLOY_ROOT="${AVF_DEPLOY_ROOT:-${ROOT}}"
POSTGRES_TOOLS_IMAGE="${POSTGRES_TOOLS_IMAGE:-postgres:17-alpine}"
EVIDENCE_DIR="${ROOT}/.db-destroy-evidence"
RESET_GUARD="I_UNDERSTAND_THIS_WIPES_PRODUCTION"
OPS_EVIDENCE_REPO_ROOT="${ROOT}"

ENVIRONMENT=""
PHASE="all"
EXECUTE=0
DRY_RUN=1
ALLOW_PRODUCTION=0
CONFIRMATION_PHRASE=""
SKIP_BACKUP=0
DB_NAME_FOR_CONFIRM=""

fail() {
	echo "run-environment-data-wipe: error: $*" >&2
	exit 1
}

ops_prepare_python3_shim "${ROOT}" || fail "unable to prepare python3 shim for verify_database_environment"

note() {
	echo "run-environment-data-wipe: $*"
}

usage() {
	cat <<EOF
usage: bash scripts/ops/run-environment-data-wipe.sh --environment ENV [options]

Default is dry-run (plan only). Destructive actions require --execute.

Options:
  --environment ENV      development | staging | production (required)
  --phase PHASE          preflight | postgres | redis | media | emqx | nats | temporal | verify | all
  --execute              Perform destructive actions (default: dry-run only)
  --dry-run              Explicit dry-run (default)
  --allow-production     Required with --execute on production
  --confirmation PHRASE  Required for production execute: WIPE PRODUCTION <database_name>
  --skip-backup          Skip backup gate (development only)
  --version              Print toolchain SHA and exit

Confirm tokens (required with --execute):
  development  CONFIRM_DEV_DATA_WIPE=AVF-WIPE-DEV-DATA
  staging      CONFIRM_STAGING_DATA_WIPE=AVF-WIPE-STAGING-DATA
  production   CONFIRM_PRODUCTION_DATA_WIPE=I_UNDERSTAND_THIS_WIPES_PRODUCTION
EOF
}

while [[ $# -gt 0 ]]; do
	case "$1" in
	--environment)
		ENVIRONMENT="${2:-}"
		shift 2
		;;
	--phase)
		PHASE="${2:-}"
		shift 2
		;;
	--execute)
		EXECUTE=1
		DRY_RUN=0
		shift
		;;
	--dry-run)
		EXECUTE=0
		DRY_RUN=1
		shift
		;;
	--allow-production)
		ALLOW_PRODUCTION=1
		shift
		;;
	--confirmation)
		CONFIRMATION_PHRASE="${2:-}"
		shift 2
		;;
	--skip-backup)
		SKIP_BACKUP=1
		shift
		;;
	--version)
		if [[ -f "${OPS}/TOOLCHAIN_SHA" ]]; then
			cat "${OPS}/TOOLCHAIN_SHA"
		else
			git -C "${ROOT}" rev-parse HEAD 2>/dev/null || echo unknown
		fi
		exit 0
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

case "${ENVIRONMENT}" in
development | staging | production) ;;
*) fail "unsupported environment: ${ENVIRONMENT}" ;;
esac

if [[ "${ENVIRONMENT}" != "development" && "${SKIP_BACKUP}" -eq 1 ]]; then
	fail "--skip-backup is only allowed for development"
fi

EVIDENCE_FILE="${EVIDENCE_DIR}/data-wipe-${ENVIRONMENT}-$(date -u +%Y%m%dT%H%M%SZ).jsonl"
mkdir -p "${EVIDENCE_DIR}"
ops_evidence_init "${EVIDENCE_FILE}"

append_evidence() {
	ops_evidence_append "$1" "${2:-}"
}

_wipe_cleanup() {
	ops_lock_release psql_query 2>/dev/null || true
}
trap _wipe_cleanup EXIT INT TERM

parse_db_name_from_url() {
	local url="$1"
	python3 - "$url" <<'PY' 2>/dev/null || echo ""
import sys
from urllib.parse import urlsplit
u = urlsplit(sys.argv[1])
print((u.path or "").lstrip("/").split("?")[0])
PY
}

require_production_gates() {
	[[ "${EXECUTE}" -eq 1 ]] || return 0
	[[ "${ENVIRONMENT}" == "production" ]] || return 0
	[[ "${ALLOW_PRODUCTION}" -eq 1 ]] || fail "production execute requires --allow-production"
	[[ -n "${CONFIRMATION_PHRASE}" ]] || fail "production execute requires --confirmation \"WIPE PRODUCTION <db_name>\""
	load_environment
	local expected="WIPE PRODUCTION ${DB_NAME_FOR_CONFIRM}"
	[[ "${CONFIRMATION_PHRASE}" == "${expected}" ]] || \
		fail "confirmation phrase must be exactly: ${expected}"
}

database_url_for_docker_exec() {
	local url="$1"
	if docker ps --format '{{.Names}}' 2>/dev/null | grep -qx avf-postgres; then
		echo "${url}" | sed -E 's/@(127\.0\.0\.1|localhost):15432/@127.0.0.1:5432/'
	else
		echo "${url}"
	fi
}

require_confirm() {
	if [[ "${DRY_RUN}" -eq 1 ]]; then
		return 0
	fi
	case "${ENVIRONMENT}" in
	development)
		[[ "${CONFIRM_DEV_DATA_WIPE:-}" == "AVF-WIPE-DEV-DATA" ]] \
			|| fail "set CONFIRM_DEV_DATA_WIPE=AVF-WIPE-DEV-DATA"
		;;
	staging)
		[[ "${CONFIRM_STAGING_DATA_WIPE:-}" == "AVF-WIPE-STAGING-DATA" ]] \
			|| fail "set CONFIRM_STAGING_DATA_WIPE=AVF-WIPE-STAGING-DATA"
		;;
	production)
		[[ "${CONFIRM_PRODUCTION_DATA_WIPE:-}" == "I_UNDERSTAND_THIS_WIPES_PRODUCTION" ]] \
			|| fail "set CONFIRM_PRODUCTION_DATA_WIPE=I_UNDERSTAND_THIS_WIPES_PRODUCTION"
		;;
	esac
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
	fail "could not reach development database on :15432 or :5432; set DATABASE_URL"
}

load_environment() {
	case "${ENVIRONMENT}" in
	development)
		export APP_ENV=development
		export PAYMENT_ENV="${PAYMENT_ENV:-sandbox}"
		DATABASE_URL="$(resolve_dev_database_url)"
		export DATABASE_URL
		REDIS_URL="${REDIS_URL:-redis://127.0.0.1:6379}"
		export REDIS_URL
		ENV_FILE="${ROOT}/.env"
		COMPOSE_FILE="${DEPLOY_ROOT}/deployments/docker/docker-compose.yml"
		SHARED_SCRIPTS=""
		;;
	staging)
		export APP_ENV=staging
		export PAYMENT_ENV="${PAYMENT_ENV:-sandbox}"
		ENV_FILE="${DEPLOY_ROOT}/deployments/staging/.env.staging"
		COMPOSE_FILE="${DEPLOY_ROOT}/deployments/staging/docker-compose.staging.yml"
		SHARED_SCRIPTS=""
		[[ -f "${ENV_FILE}" ]] || fail "missing ${ENV_FILE}"
		set -a
		# shellcheck disable=SC1090
		source "${ENV_FILE}"
		set +a
		[[ -n "${DATABASE_URL:-}" ]] || fail "DATABASE_URL is empty in ${ENV_FILE}"
		;;
	production)
		export APP_ENV=production
		ENV_FILE="${DEPLOY_ROOT}/deployments/prod/app-node/.env.app-node"
		COMPOSE_FILE="${DEPLOY_ROOT}/deployments/prod/app-node/docker-compose.app-node.yml"
		SHARED_SCRIPTS="${DEPLOY_ROOT}/deployments/prod/shared/scripts"
		[[ -f "${ENV_FILE}" ]] || fail "missing ${ENV_FILE}"
		set -a
		# shellcheck disable=SC1090
		source "${ENV_FILE}"
		set +a
		if [[ -z "${DATABASE_URL:-}" ]]; then
			DATABASE_URL="$(grep -E '^DATABASE_URL=' "${ENV_FILE}" | tail -n1 | cut -d= -f2- | tr -d '\r' | sed -e 's/^"//' -e 's/"$//')"
			export DATABASE_URL
		fi
		[[ -n "${DATABASE_URL:-}" ]] || fail "DATABASE_URL is empty"
		;;
	esac

	append_evidence "load_environment" "env=${ENVIRONMENT} database_url_set=true"
}

verify_db_environment() {
	export APP_ENV
	export DATABASE_URL
	export PAYMENT_ENV="${PAYMENT_ENV:-sandbox}"
	bash "${ROOT}/scripts/verify_database_environment.sh"
	DB_NAME_FOR_CONFIRM="$(parse_db_name_from_url "${DATABASE_URL}")"
	[[ -n "${DB_NAME_FOR_CONFIRM}" ]] || fail "could not parse database name from DATABASE_URL"
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
	docker run --rm \
		-e "DATABASE_URL=${DATABASE_URL}" \
		-v "${OPS}:/ops:ro" \
		"${POSTGRES_TOOLS_IMAGE}" \
		psql "${DATABASE_URL}" -v ON_ERROR_STOP=1 -f "/ops/$(basename "${sql_file}")"
}

psql_query() {
	local query="$1"
	local exec_url
	if [[ "${ENVIRONMENT}" == "development" ]] && docker ps --format '{{.Names}}' 2>/dev/null | grep -qx avf-postgres; then
		exec_url="$(database_url_for_docker_exec "${DATABASE_URL}")"
		docker exec avf-postgres psql "${exec_url}" -v ON_ERROR_STOP=1 -Atqc "${query}"
		return 0
	fi
	if command -v psql >/dev/null 2>&1; then
		psql "${DATABASE_URL}" -v ON_ERROR_STOP=1 -Atqc "${query}"
		return 0
	fi
	docker run --rm \
		-e "DATABASE_URL=${DATABASE_URL}" \
		"${POSTGRES_TOOLS_IMAGE}" \
		psql "${DATABASE_URL}" -v ON_ERROR_STOP=1 -Atqc "${query}"
}

phase_preflight() {
	require_confirm
	require_production_gates
	load_environment
	verify_db_environment
	ops_lock_compute_key "${ENVIRONMENT}" "${DB_NAME_FOR_CONFIRM}"
	if [[ "${EXECUTE}" -eq 1 ]]; then
		ops_lock_acquire psql_query || fail "could not acquire advisory lock"
	fi

	note "Phase preflight — backup (if required) + quiesce writers"
	append_evidence "preflight_start" "dry_run=${DRY_RUN} execute=${EXECUTE}"

	if [[ "${DRY_RUN}" -eq 1 ]]; then
		note "[dry-run] would quiesce writers for ${ENVIRONMENT}"
		psql_exec_file "${OPS}/production-purge-dry-run.sql"
		return 0
	fi

	if [[ "${ENVIRONMENT}" == "development" ]]; then
		note "Development: skipping backup"
	elif [[ "${SKIP_BACKUP}" -eq 1 ]]; then
		fail "backup is required for ${ENVIRONMENT}; do not pass --skip-backup"
	else
		local timestamp backup_path
		timestamp="$(date -u +%Y%m%dT%H%M%SZ)"
		backup_path="${BACKUP_PATH:-/var/backups/avf-data-wipe-${ENVIRONMENT}-${timestamp}.dump}"
		mkdir -p "$(dirname "${backup_path}")"

		if [[ -n "${BACKUP_PATH:-}" && -f "${BACKUP_PATH}" ]]; then
			note "Using verified backup at ${BACKUP_PATH}"
			append_evidence "backup_existing" "${BACKUP_PATH}"
		elif [[ "${ENVIRONMENT}" == "production" && -x "${SHARED_SCRIPTS}/backup_managed_postgres.sh" ]]; then
			bash "${SHARED_SCRIPTS}/backup_managed_postgres.sh" "${backup_path}" execute
			append_evidence "backup_created" "${backup_path}"
		else
			note "Creating pg_dump backup at ${backup_path}"
			if [[ -f "${ENV_FILE:-}" ]]; then
				docker run --rm \
					--env-file "${ENV_FILE}" \
					-e "DATABASE_URL=${DATABASE_URL}" \
					-v "$(dirname "${backup_path}"):/backup" \
					"${POSTGRES_TOOLS_IMAGE}" \
					pg_dump "${DATABASE_URL}" --format=custom --no-owner --no-privileges \
					--file "/backup/$(basename "${backup_path}")"
			else
				docker run --rm \
					-e "DATABASE_URL=${DATABASE_URL}" \
					-v "$(dirname "${backup_path}"):/backup" \
					"${POSTGRES_TOOLS_IMAGE}" \
					pg_dump "${DATABASE_URL}" --format=custom --no-owner --no-privileges \
					--file "/backup/$(basename "${backup_path}")"
			fi
			append_evidence "backup_created" "${backup_path}"
		fi
	fi

	bash "${DB_SCRIPTS}/quiesce_writers.sh" --environment "${ENVIRONMENT}"
	append_evidence "writers_quiesced" "${ENVIRONMENT}"
	psql_exec_file "${OPS}/production-purge-dry-run.sql"
}

phase_postgres() {
	require_confirm
	load_environment
	verify_db_environment

	note "Phase postgres — dry-run counts"
	psql_exec_file "${OPS}/production-purge-dry-run.sql"

	if [[ "${DRY_RUN}" -eq 1 ]]; then
		note "[dry-run] would run production-reset-bootstrap-admin.sql"
		return 0
	fi

	note "Phase postgres — TRUNCATE all business data (preserve goose_db_version)"
	if [[ "${ENVIRONMENT}" == "development" ]] && docker ps --format '{{.Names}}' 2>/dev/null | grep -qx avf-postgres; then
		exec_url="$(database_url_for_docker_exec "${DATABASE_URL}")"
		docker exec -i avf-postgres psql "${exec_url}" -v ON_ERROR_STOP=1 \
			-c "SET avf.confirm_production_reset='${RESET_GUARD}';" \
			-f - <"${OPS}/production-reset-bootstrap-admin.sql"
	else
		docker run --rm \
			-e "DATABASE_URL=${DATABASE_URL}" \
			-v "${OPS}:/ops:ro" \
			"${POSTGRES_TOOLS_IMAGE}" \
			psql "${DATABASE_URL}" -v ON_ERROR_STOP=1 \
			-c "SET avf.confirm_production_reset='${RESET_GUARD}';" \
			-f /ops/production-reset-bootstrap-admin.sql
	fi

	psql_exec_file "${OPS}/verify-table-data-empty.sql"
	append_evidence "postgres_wipe_complete" "verified_empty=true"
	note "PostgreSQL wipe verified"
}

phase_redis() {
	require_confirm
	load_environment

	if ! redis_configured; then
		if [[ "${ENVIRONMENT}" == "production" ]]; then
			fail "Redis is required for production but not configured (REDIS_ADDR/REDIS_URL missing)"
		fi
		note "Redis not configured — skipping redis phase"
		return 0
	fi

	if [[ "${ENVIRONMENT}" == "development" ]]; then
		export REDIS_CONTAINER="avf-redis"
	elif [[ "${ENVIRONMENT}" == "production" ]]; then
		export REDIS_CONTAINER="$(docker ps --format '{{.Names}}' 2>/dev/null | grep -i redis | head -1 || true)"
	fi

	local before after
	before="$(redis_dbsize 2>/dev/null || echo unknown)"
	note "Phase redis — pre-flush DBSIZE=${before}"

	if [[ "${DRY_RUN}" -eq 1 ]]; then
		redis_wipe_execute 1
		return 0
	fi

	redis_wipe_execute 0 || fail "Redis wipe failed"
	after="$(redis_dbsize 2>/dev/null || echo ERROR)"
	if [[ "${REDIS_DEDICATED_INSTANCE:-0}" == "1" || "${REDIS_DEDICATED_INSTANCE:-0}" == "true" ]]; then
		[[ "${after}" == "0" ]] || fail "Redis post-flush DBSIZE=${after} (expected 0 on dedicated instance)"
	else
		note "Phase redis — prefix-scoped wipe complete (shared instance; DBSIZE=${after})"
	fi
	append_evidence "redis_wiped" "dbsize_before=${before} dbsize_after=${after}"
	note "Phase redis — complete"
}

phase_media() {
	require_confirm
	load_environment

	if [[ "${DRY_RUN}" -eq 1 ]]; then
		note "[dry-run] would purge Cloudinary/S3 media if configured"
		return 0
	fi

	note "Phase media — Cloudinary + object storage"
	if [[ -n "${CLOUDINARY_CLOUD_NAME:-}" && -n "${CLOUDINARY_API_KEY:-}" && -n "${CLOUDINARY_API_SECRET:-}" ]]; then
		local cld_wipe="${OPS}/cloudinary/wipe.sh"
		if [[ -x "${cld_wipe}" || -f "${cld_wipe}" ]]; then
			export CLOUDINARY_EVIDENCE_DIR="${ROOT}/.cloudinary-wipe-evidence"
			mkdir -p "${CLOUDINARY_EVIDENCE_DIR}"
			if [[ "${DRY_RUN}" -eq 1 ]]; then
				note "[dry-run] would run Cloudinary full wipe for cloud_name=${CLOUDINARY_CLOUD_NAME}"
				bash "${cld_wipe}" --dry-run --env-file "${ENV_FILE:-}"
			else
				note "Cloudinary full wipe cloud_name=${CLOUDINARY_CLOUD_NAME}"
				export CONFIRM_CLOUDINARY_LIVE_DELETE="DELETE-ALL-CLOUDINARY-ASSETS-${CLOUDINARY_CLOUD_NAME}"
				bash "${cld_wipe}" --delete-live-assets --delete-empty-folders --env-file "${ENV_FILE:-}"
				bash "${OPS}/cloudinary/verify-empty.sh" --env-file "${ENV_FILE:-}"
			fi
			append_evidence "cloudinary_purged" "full_wipe cloud=${CLOUDINARY_CLOUD_NAME}"
		else
			fail "missing ${cld_wipe}"
		fi
	else
		note "Cloudinary env not configured — skip"
	fi

	local bucket="${OBJECT_STORAGE_BUCKET:-${S3_BUCKET:-}}"
	if [[ -n "${bucket}" ]]; then
		if command -v aws >/dev/null 2>&1; then
			note "Deleting S3 prefix s3://${bucket}/"
			aws s3 rm "s3://${bucket}/" --recursive
			append_evidence "s3_purged" "${bucket}"
		else
			note "Object storage bucket set but aws CLI missing — purge manually"
		fi
	else
		note "No object storage bucket configured — skip"
	fi
}

phase_emqx() {
	require_confirm
	load_environment

	if [[ "${DRY_RUN}" -eq 1 ]]; then
		note "[dry-run] would purge EMQX machine users if API reachable"
		return 0
	fi

	note "Phase emqx — purge machine MQTT users"
	if ! curl -fsS --max-time 3 "http://127.0.0.1:18083/api/v5/status" >/dev/null 2>&1; then
		if [[ "${ENVIRONMENT}" == "development" ]]; then
			note "EMQX not reachable on 127.0.0.1:18083 — skip (start broker profile if needed)"
			return 0
		fi
		fail "EMQX management API not reachable on 127.0.0.1:18083 — run emqx phase on data-node"
	fi
	if [[ -f "${OPS}/emqx-purge.sh" ]]; then
		bash "${OPS}/emqx-purge.sh"
	elif [[ -x "${OPS}/emqx-audit-and-purge.sh" ]]; then
		bash "${OPS}/emqx-audit-and-purge.sh" --purge
	else
		bash "${OPS}/emqx-purge-machine-users.sh"
	fi
	append_evidence "emqx_purged" "true"
}

phase_nats() {
	require_confirm
	load_environment

	if [[ -z "${NATS_URL:-}" ]]; then
		if [[ "${ENVIRONMENT}" == "production" ]]; then
			note "NATS_URL unset — skip nats phase (verify data-node env)"
			return 0
		fi
		note "NATS_URL unset — skipping nats phase"
		return 0
	fi

	if [[ "${DRY_RUN}" -eq 1 ]]; then
		note "[dry-run] would purge AVF JetStream streams"
		return 0
	fi

	note "Phase nats — purge AVF JetStream streams"
	if [[ -f "${OPS}/nats-purge-streams.sh" ]]; then
		bash "${OPS}/nats-purge-streams.sh"
	else
		bash "${OPS}/nats-purge-avf-streams.sh" --purge
	fi
	append_evidence "nats_purged" "true"
}

phase_temporal() {
	require_confirm
	load_environment

	if [[ "${TEMPORAL_ENABLED:-false}" != "true" && "${TEMPORAL_ENABLED:-false}" != "1" ]]; then
		note "TEMPORAL_ENABLED not true — skipping temporal phase"
		return 0
	fi

	if [[ "${DRY_RUN}" -eq 1 ]]; then
		note "[dry-run] would terminate AVF Temporal workflows"
		return 0
	fi

	fail "Temporal is enabled but automated workflow termination is not implemented — terminate manually before verify"
}

phase_verify() {
	require_confirm
	load_environment
	verify_db_environment

	note "Phase verify — table counts must be zero"
	if [[ "${DRY_RUN}" -eq 1 ]]; then
		psql_exec_file "${OPS}/production-purge-dry-run.sql"
		return 0
	fi

	if [[ "${ENVIRONMENT}" != "production" ]]; then
		psql_exec_file "${OPS}/verify-table-data-empty.sql"
	else
		psql_exec_file "${OPS}/production-purge-dry-run.sql"
		note "production verify uses production-purge-dry-run (full audit via run-clean-slate-audit.sh)"
	fi

	if redis_configured; then
		local rdbsize avf_keys
		rdbsize="$(redis_dbsize 2>/dev/null || echo ERROR)"
		avf_keys="$(redis_avf_key_count 2>/dev/null || echo ERROR)"
		if [[ "${REDIS_DEDICATED_INSTANCE:-0}" == "1" || "${REDIS_DEDICATED_INSTANCE:-0}" == "true" ]]; then
			[[ "${rdbsize}" == "0" ]] || fail "verify: Redis DBSIZE=${rdbsize} (expected 0)"
		else
			[[ "${avf_keys}" == "0" ]] || fail "verify: avf_prefixed_keys=${avf_keys} (expected 0)"
		fi
		append_evidence "redis_verify" "dbsize=${rdbsize} avf_keys=${avf_keys}"
	fi

	local goose_version
	goose_version="$(psql_query "SELECT version_id FROM goose_db_version ORDER BY version_id DESC LIMIT 1;")"
	note "goose_db_version=${goose_version}"
	append_evidence "verify_pass" "goose_version=${goose_version}"

	if [[ "${ENVIRONMENT}" == "production" && -n "${API_DOMAIN:-}" ]]; then
		curl -fsS "https://${API_DOMAIN}/health/live" >/dev/null || note "health/live check failed (workers may be stopped)"
	fi
}

run_phase() {
	case "$1" in
	preflight) phase_preflight ;;
	postgres) phase_postgres ;;
	redis) phase_redis ;;
	media) phase_media ;;
	emqx) phase_emqx ;;
	nats) phase_nats ;;
	temporal) phase_temporal ;;
	verify) phase_verify ;;
	all)
		phase_preflight
		phase_nats || true
		phase_postgres
		phase_redis
		phase_media
		if [[ "${ENVIRONMENT}" == "production" ]]; then
			note "Skipping emqx/nats on app-node — run --phase emqx and --phase nats on data-node if needed"
		else
			phase_emqx || true
			phase_nats || true
		fi
		phase_temporal || true
		phase_verify
		;;
	*)
		usage
		fail "unknown phase: $1"
		;;
	esac
}

note "environment=${ENVIRONMENT} phase=${PHASE} execute=${EXECUTE} dry_run=${DRY_RUN} evidence=${EVIDENCE_FILE}"
append_evidence "run_start" "phase=${PHASE} execute=${EXECUTE} dry_run=${DRY_RUN}"
run_phase "${PHASE}"
append_evidence "run_complete" "phase=${PHASE}"
note "Done. Evidence: ${EVIDENCE_FILE}"
