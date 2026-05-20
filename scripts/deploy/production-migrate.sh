#!/usr/bin/env bash
# Production database backup + goose migration using migrations embedded in APP_IMAGE_REF.
# Safe for managed PostgreSQL (Supabase). Never runs down/reset. Masks DATABASE_URL in logs.
set -Eeuo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"

COMPOSE_FILE="${COMPOSE_FILE:-}"
COMPOSE_ENV_FILE="${COMPOSE_ENV_FILE:-}"
COMPOSE_PROJECT_NAME="${COMPOSE_PROJECT_NAME:-}"
MIGRATION_LOG_DIR="${MIGRATION_LOG_DIR:-/opt/avf-vending-api/deployments/prod/logs/migrations}"
POSTGRES_TOOLS_IMAGE="${POSTGRES_TOOLS_IMAGE:-postgres:17-alpine}"
ADVISORY_LOCK_ID="${MIGRATION_ADVISORY_LOCK_ID:-90420520260520}"
DRY_VALIDATE="${DRY_VALIDATE:-0}"

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

run_psql() {
	local sql="$1"
	docker run --rm \
		--env-file "${COMPOSE_ENV_FILE}" \
		-e "DATABASE_URL=${DATABASE_URL}" \
		"${POSTGRES_TOOLS_IMAGE}" \
		psql "${DATABASE_URL}" -v ON_ERROR_STOP=1 -c "${sql}"
}

run_compose_migrate_cmd() {
	local subcommand="$1"
	"${COMPOSE[@]}" --profile migration run --rm --no-TTY migrate "${subcommand}"
}

validate_image_migrations() {
	note "validate migrations embedded in APP_IMAGE_REF"
	run_compose_migrate_cmd validate
}

backup_database() {
	local backup_path="$1"
	note "backup database to ${backup_path}"
	mkdir -p "$(dirname "${backup_path}")"
	if ! docker run --rm \
		--env-file "${COMPOSE_ENV_FILE}" \
		-e "DATABASE_URL=${DATABASE_URL}" \
		-v "$(dirname "${backup_path}"):/backup" \
		"${POSTGRES_TOOLS_IMAGE}" \
		pg_dump "${DATABASE_URL}" -Fc -f "/backup/$(basename "${backup_path}")"; then
		fail "pg_dump failed"
	fi
	[[ -s "${backup_path}" ]] || fail "backup file is empty: ${backup_path}"
}

verify_backup() {
	local backup_path="$1"
	note "verify backup artifact"
	if ! docker run --rm \
		-v "$(dirname "${backup_path}"):/backup:ro" \
		"${POSTGRES_TOOLS_IMAGE}" \
		pg_restore -l "/backup/$(basename "${backup_path}")" >/dev/null; then
		fail "pg_restore -l verification failed for ${backup_path}"
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

if [[ -z "${DATABASE_URL:-}" ]]; then
	load_env_file "${COMPOSE_ENV_FILE}"
fi
[[ -n "${DATABASE_URL:-}" ]] || fail "DATABASE_URL is empty"

if [[ -z "${APP_IMAGE_REF:-}" ]]; then
	APP_IMAGE_REF="$(read_env_value_from_file "${COMPOSE_ENV_FILE}" "APP_IMAGE_REF" || true)"
fi
[[ -n "${APP_IMAGE_REF:-}" ]] || fail "APP_IMAGE_REF is empty"

export APP_ENV="${APP_ENV:-production}"
if [[ "${GITHUB_ACTIONS:-}" != "true" ]]; then
	if [[ "${CONFIRM_PRODUCTION_MIGRATION:-}" != "true" ]]; then
		fail "set CONFIRM_PRODUCTION_MIGRATION=true for manual production migration"
	fi
fi

if ! bash "${REPO_ROOT}/scripts/verify_database_environment.sh"; then
	fail "verify_database_environment.sh failed"
fi

note "target database: $(mask_database_url "${DATABASE_URL}")"
note "app image: ${APP_IMAGE_REF}"

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
run_compose_migrate_cmd status || true
version_before="$(run_compose_migrate_cmd version 2>/dev/null | tail -n1 || echo "unknown")"
note "goose version before: ${version_before}"

note "apply pending migrations (goose up)"
run_compose_migrate_cmd up

note "migration status (after)"
run_compose_migrate_cmd status || true
version_after="$(run_compose_migrate_cmd version 2>/dev/null | tail -n1 || echo "unknown")"
note "goose version after: ${version_after}"

note "PASS backup=${backup_path} version_before=${version_before} version_after=${version_after}"
