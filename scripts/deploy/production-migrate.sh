#!/usr/bin/env bash
# Production database backup + goose migration using migrations embedded in APP_IMAGE_REF.
# Safe for managed PostgreSQL (Supabase). Never runs down/reset. Masks DATABASE_URL in logs.
set -Eeuo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
# shellcheck source=../../deployments/prod/shared/scripts/lib_release.sh
source "${REPO_ROOT}/deployments/prod/shared/scripts/lib_release.sh"

COMPOSE_FILE="${COMPOSE_FILE:-}"
COMPOSE_ENV_FILE="${COMPOSE_ENV_FILE:-}"
COMPOSE_PROJECT_NAME="${COMPOSE_PROJECT_NAME:-}"
MIGRATION_LOG_DIR="${MIGRATION_LOG_DIR:-/opt/avf-vending-api/deployments/prod/logs/migrations}"
POSTGRES_TOOLS_IMAGE="${POSTGRES_TOOLS_IMAGE:-postgres:17-alpine}"
ADVISORY_LOCK_ID="${MIGRATION_ADVISORY_LOCK_ID:-90420520260520}"
DRY_VALIDATE="${DRY_VALIDATE:-0}"

# production-migrate.sh exit codes (release_app_node maps migrate failure to deploy exit 41)
readonly EXIT_USAGE=2
readonly EXIT_VERIFY_ENV=10
readonly EXIT_IMAGE_REF=11
readonly EXIT_BACKUP=20
readonly EXIT_MIGRATION=30

usage() {
	cat <<'EOF'
usage: production-migrate.sh [options]

Required environment or flags:
  COMPOSE_FILE          docker-compose.app-node.yml path
  COMPOSE_ENV_FILE      .env.app-node path (DATABASE_URL source)

Optional:
  COMPOSE_PROJECT_NAME  compose project name
  MIGRATION_LOG_DIR     backup and log directory (default: /opt/avf-vending-api/deployments/prod/logs/migrations)
  DATABASE_URL          override (otherwise loaded from COMPOSE_ENV_FILE)
  BACKUP_DATABASE_URL   optional direct Postgres URL for pg_dump only (bypasses session pooler)
  APP_IMAGE_REF         override (otherwise loaded from COMPOSE_ENV_FILE)
  DRY_VALIDATE=1        validate migrations in image only (no DB backup/up)
  --validate-only       same as DRY_VALIDATE=1

Never prints full DATABASE_URL or passwords.
EOF
}

fail() {
	echo "production-migrate: error: $*" >&2
	exit 1
}

fail_with_code() {
	local code="$1"
	shift
	echo "production-migrate: error: $*" >&2
	exit "${code}"
}

require_digest_image_ref() {
	local label="$1"
	local ref="$2"
	[[ -n "${ref}" ]] || fail_with_code "${EXIT_IMAGE_REF}" "${label} is empty"
	[[ "${ref}" == *"@sha256:"* ]] || fail_with_code "${EXIT_IMAGE_REF}" "${label} must be digest-pinned (...@sha256:...): ${ref}"
	[[ "${ref}" != *":latest"* ]] || fail_with_code "${EXIT_IMAGE_REF}" "${label} must not use the latest tag: ${ref}"
}

require_postgres17_backup_image() {
	case "${POSTGRES_TOOLS_IMAGE}" in
	postgres:17 | postgres:17-alpine | postgres:17.*-alpine)
		return 0
		;;
	*)
		fail_with_code "${EXIT_BACKUP}" "POSTGRES_TOOLS_IMAGE must be Postgres 17 compatible (default postgres:17-alpine); got: ${POSTGRES_TOOLS_IMAGE}"
		;;
	esac
}

note() {
	echo "production-migrate: $*"
}

mask_database_url() {
	python3 - "$1" <<'PY'
import sys
from urllib.parse import urlsplit, urlunsplit

raw = sys.argv[1].strip()
if not raw:
    print("(empty)")
    raise SystemExit(0)
u = urlsplit(raw)
user = u.username or ""
host = u.hostname or ""
port = f":{u.port}" if u.port else ""
path = u.path or ""
masked_user = user if user else ""
if u.password:
    masked_user = f"{user}:***" if user else "***"
netloc = host + port
if masked_user:
    netloc = f"{masked_user}@{netloc}"
print(urlunsplit((u.scheme, netloc, path, u.query, u.fragment)))
PY
}

# pg_dump rejects some pooler-only query params (e.g. Supabase simple_protocol on :6543).
sanitize_url_for_pg_tools() {
	local url="$1"
	python3 - "${url}" <<'PY'
import sys
from urllib.parse import parse_qsl, urlencode, urlparse, urlunparse

u = urlparse(sys.argv[1])
drop = {"default_query_exec_mode", "pgbouncer"}
query = [(k, v) for k, v in parse_qsl(u.query, keep_blank_values=True) if k not in drop]
print(urlunparse((u.scheme, u.netloc, u.path, u.params, urlencode(query), u.fragment)))
PY
}

load_env_file() {
	local path="$1"
	[[ -f "${path}" ]] || fail "env file not found: ${path}"
	set -a
	# shellcheck disable=SC1090
	source "${path}"
	set +a
}

read_env_value_from_file() {
	local file="$1"
	local key="$2"
	local line
	line="$(grep -E "^${key}=" "${file}" 2>/dev/null | tail -n1 || true)"
	[[ -n "${line}" ]] || return 1
	line="${line#"${key}="}"
	line="${line%$'\r'}"
	if [[ "${line}" == \"*\" ]]; then
		line="${line#\"}"
		line="${line%\"}"
	fi
	printf '%s' "${line}"
}

find_running_api_container() {
	docker ps --format '{{.Names}}' | grep -E 'api' | head -n1
}

read_database_url_from_running_api_container() {
	local container="$1"
	local val
	[[ -n "${container}" ]] || return 1
	val="$(docker inspect "${container}" --format '{{range .Config.Env}}{{println .}}{{end}}' \
		| grep -E '^DATABASE_URL=' | tail -n1 | cut -d= -f2- | tr -d '\r')"
	[[ -n "${val}" ]] || return 1
	printf '%s' "${val}"
}

resolve_database_url() {
	local container val
	if [[ -n "${DATABASE_URL:-}" ]]; then
		return 0
	fi
	load_env_file "${COMPOSE_ENV_FILE}"
	if [[ -n "${DATABASE_URL:-}" ]]; then
		return 0
	fi
	container="$(find_running_api_container)"
	if val="$(read_database_url_from_running_api_container "${container}")"; then
		DATABASE_URL="${val}"
		export DATABASE_URL
		note "resolved DATABASE_URL from running api container (${container})"
		return 0
	fi
	return 1
}

run_psql() {
	local sql="$1"
	local psql_url
	psql_url="$(sanitize_url_for_pg_tools "${DATABASE_URL}")"
	docker run --rm \
		--env-file "${COMPOSE_ENV_FILE}" \
		-e "DATABASE_URL=${psql_url}" \
		"${POSTGRES_TOOLS_IMAGE}" \
		psql "${psql_url}" -v ON_ERROR_STOP=1 -c "${sql}"
}

run_compose_migrate_cmd() {
	local subcommand="$1"
	"${COMPOSE[@]}" --profile migration run --rm --no-TTY migrate "${subcommand}"
}

validate_image_migrations() {
	note "validate migrations embedded in APP_IMAGE_REF (source: /app/migrations inside digest-pinned app image; not host backup folders)"
	if ! run_compose_migrate_cmd validate; then
		fail_with_code "${EXIT_MIGRATION}" "embedded migration validate failed inside APP_IMAGE_REF"
	fi
}

backup_database() {
	local backup_path="$1"
	local dump_url
	dump_url="$(resolve_backup_database_url)"
	note "backup database to ${backup_path}"
	if [[ "${dump_url}" != "${DATABASE_URL}" ]]; then
		note "using BACKUP_DATABASE_URL for pg_dump (direct/non-pooler connection)"
	else
		note "using DATABASE_URL for pg_dump"
	fi
	mkdir -p "$(dirname "${backup_path}")"
	local attempt=1
	local max_attempts=5
	local err_file
	err_file="$(mktemp)"
	trap 'rm -f "${err_file}"' RETURN
	while [[ "${attempt}" -le "${max_attempts}" ]]; do
		if docker run --rm \
			--env-file "${COMPOSE_ENV_FILE}" \
			-e "DATABASE_URL=${dump_url}" \
			-v "$(dirname "${backup_path}"):/backup" \
			"${POSTGRES_TOOLS_IMAGE}" \
			pg_dump "${dump_url}" -Fc -f "/backup/$(basename "${backup_path}")" 2>"${err_file}"; then
			rm -f "${err_file}"
			[[ -s "${backup_path}" ]] || fail_with_code "${EXIT_BACKUP}" "backup file is empty: ${backup_path}"
			return 0
		fi
		if is_pg_pool_exhausted "${err_file}" && [[ "${attempt}" -lt "${max_attempts}" ]]; then
			note "pg_dump hit pool limit (attempt ${attempt}/${max_attempts}); retrying in $((attempt * 5))s"
			sleep $((attempt * 5))
			attempt=$((attempt + 1))
			continue
		fi
		cat "${err_file}" >&2
		fail_with_code "${EXIT_BACKUP}" "pg_dump failed"
	done
	fail_with_code "${EXIT_BACKUP}" "pg_dump failed after ${max_attempts} attempts"
}

verify_backup() {
	local backup_path="$1"
	note "verify backup artifact"
	if ! docker run --rm \
		-v "$(dirname "${backup_path}"):/backup:ro" \
		"${POSTGRES_TOOLS_IMAGE}" \
		pg_restore -l "/backup/$(basename "${backup_path}")" >/dev/null; then
		fail_with_code "${EXIT_BACKUP}" "pg_restore -l verification failed for ${backup_path}"
	fi
}

LOCK_ACQUIRED=0

acquire_advisory_lock() {
	note "acquire PostgreSQL advisory lock ${ADVISORY_LOCK_ID}"
	run_psql "SELECT pg_advisory_lock(${ADVISORY_LOCK_ID});"
	LOCK_ACQUIRED=1
}

release_advisory_lock() {
	if [[ "${LOCK_ACQUIRED}" != "1" ]]; then
		return 0
	fi
	note "release PostgreSQL advisory lock ${ADVISORY_LOCK_ID}"
	run_psql "SELECT pg_advisory_unlock(${ADVISORY_LOCK_ID});" || true
	LOCK_ACQUIRED=0
}

on_exit() {
	local rc=$?
	release_advisory_lock
	exit "${rc}"
}

while [[ $# -gt 0 ]]; do
	case "$1" in
	--compose-file)
		COMPOSE_FILE="$2"
		shift 2
		;;
	--env-file)
		COMPOSE_ENV_FILE="$2"
		shift 2
		;;
	--project-name)
		COMPOSE_PROJECT_NAME="$2"
		shift 2
		;;
	--validate-only)
		DRY_VALIDATE=1
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

[[ -n "${COMPOSE_FILE}" ]] || fail "COMPOSE_FILE is required"
[[ -n "${COMPOSE_ENV_FILE}" ]] || fail "COMPOSE_ENV_FILE is required"
[[ -f "${COMPOSE_FILE}" ]] || fail "compose file not found: ${COMPOSE_FILE}"
[[ -f "${COMPOSE_ENV_FILE}" ]] || fail "compose env file not found: ${COMPOSE_ENV_FILE}"

COMPOSE=(docker compose --env-file "${COMPOSE_ENV_FILE}" -f "${COMPOSE_FILE}")
if [[ -n "${COMPOSE_PROJECT_NAME}" ]]; then
	COMPOSE+=(--project-name "${COMPOSE_PROJECT_NAME}")
fi

if ! resolve_database_url; then
	fail "DATABASE_URL is empty (not in env, ${COMPOSE_ENV_FILE}, or running api container)"
fi

if [[ -z "${BACKUP_DATABASE_URL:-}" ]]; then
	BACKUP_DATABASE_URL="$(read_env_value_from_file "${COMPOSE_ENV_FILE}" "BACKUP_DATABASE_URL" || true)"
fi

resolve_backup_database_url() {
	local url
	if [[ -n "${BACKUP_DATABASE_URL:-}" ]]; then
		url="${BACKUP_DATABASE_URL}"
	else
		url="${DATABASE_URL}"
	fi
	sanitize_url_for_pg_tools "${url}"
}

is_pg_pool_exhausted() {
	local err_file="$1"
	grep -Eiq 'max clients reached|EMAXCONNSESSION|too many clients' "${err_file}" 2>/dev/null
}

if [[ -z "${APP_IMAGE_REF:-}" ]]; then
	APP_IMAGE_REF="$(read_env_value_from_file "${COMPOSE_ENV_FILE}" "APP_IMAGE_REF" || true)"
fi
[[ -n "${APP_IMAGE_REF:-}" ]] || fail_with_code "${EXIT_IMAGE_REF}" "APP_IMAGE_REF is empty"

if [[ -z "${GOOSE_IMAGE_REF:-}" ]]; then
	GOOSE_IMAGE_REF="$(read_env_value_from_file "${COMPOSE_ENV_FILE}" "GOOSE_IMAGE_REF" || true)"
fi
[[ -n "${GOOSE_IMAGE_REF:-}" ]] || fail_with_code "${EXIT_IMAGE_REF}" "GOOSE_IMAGE_REF is empty"

require_digest_image_ref "APP_IMAGE_REF" "${APP_IMAGE_REF}"
require_digest_image_ref "GOOSE_IMAGE_REF" "${GOOSE_IMAGE_REF}"
require_postgres17_backup_image

if [[ -n "${MIGRATIONS_DIR:-}" && "${MIGRATIONS_DIR}" != "/app/migrations" ]]; then
	fail_with_code "${EXIT_MIGRATION}" "refusing host MIGRATIONS_DIR override in production (${MIGRATIONS_DIR}); migrations must come from APP_IMAGE_REF:/app/migrations"
fi

export APP_ENV="${APP_ENV:-production}"
if [[ "${GITHUB_ACTIONS:-}" != "true" ]]; then
	if [[ "${CONFIRM_PRODUCTION_MIGRATION:-}" != "true" ]]; then
		fail "set CONFIRM_PRODUCTION_MIGRATION=true for manual production migration"
	fi
fi

if ! run_script "${REPO_ROOT}/scripts/verify_database_environment.sh"; then
	fail_with_code "${EXIT_VERIFY_ENV}" "verify_database_environment.sh failed"
fi

note "target database: $(mask_database_url "${DATABASE_URL}")"
note "app image (migration runner + embedded SQL): ${APP_IMAGE_REF}"
note "goose image (digest-pinned deploy artifact; recorded for rollback parity): ${GOOSE_IMAGE_REF}"
note "backup tools image: ${POSTGRES_TOOLS_IMAGE}"

validate_image_migrations

if [[ "${DRY_VALIDATE}" == "1" ]]; then
	note "validate-only: skipping backup and migration up"
	exit 0
fi

timestamp="$(date -u +"%Y%m%dT%H%M%SZ")"
mkdir -p "${MIGRATION_LOG_DIR}"
log_file="${MIGRATION_LOG_DIR}/migrate-${timestamp}.log"
backup_path="${MIGRATION_LOG_DIR}/backup-${timestamp}.dump"

note "migration log: ${log_file}"
exec > >(tee -a "${log_file}") 2>&1

trap on_exit EXIT

backup_database "${backup_path}"
verify_backup "${backup_path}"
acquire_advisory_lock

note "migration status (before)"
run_compose_migrate_cmd status
version_before="$(run_compose_migrate_cmd version 2>/dev/null | tail -n1 || echo "unknown")"
note "goose version before: ${version_before}"

note "apply pending migrations (goose up via /app/migrate in APP_IMAGE_REF)"
if ! run_compose_migrate_cmd up; then
	fail_with_code "${EXIT_MIGRATION}" "goose up failed"
fi

note "migration status (after)"
run_compose_migrate_cmd status
version_after="$(run_compose_migrate_cmd version 2>/dev/null | tail -n1 || echo "unknown")"
note "goose version after: ${version_after}"

note "PASS backup=${backup_path} version_before=${version_before} version_after=${version_after} migration_gate=closed"
