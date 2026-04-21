#!/usr/bin/env bash
# Logical backup of the configured production database (gzip SQL). Requires a running postgres service.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"
umask 077

log() {
	echo "==> $*"
}

fail() {
	echo "error: $*" >&2
	exit 1
}

if [[ ! -f .env.production ]]; then
	fail "missing .env.production"
fi

read_env() {
	local key="$1"
	local line
	line="$(grep -E "^${key}=" .env.production | tail -n1 || true)"
	if [[ -z "${line}" ]]; then
		fail "${key} not set in .env.production"
	fi
	line="${line#"${key}="}"
	line="${line%$'\r'}"
	if [[ "${line}" == \"*\" ]]; then
		line="${line#\"}"
		line="${line%\"}"
	fi
	printf '%s' "${line}"
}

read_env_optional() {
	local key="$1"
	local line
	line="$(grep -E "^${key}=" .env.production | tail -n1 || true)"
	if [[ -z "${line}" ]]; then
		printf ''
		return 0
	fi
	line="${line#"${key}="}"
	line="${line%$'\r'}"
	if [[ "${line}" == \"*\" ]]; then
		line="${line#\"}"
		line="${line%\"}"
	fi
	printf '%s' "${line}"
}

resolve_database_name() {
	local db_name db_url
	db_name="$(read_env_optional POSTGRES_DB)"
	if [[ -n "${db_name}" ]]; then
		printf '%s' "${db_name}"
		return 0
	fi

	db_url="$(read_env_optional DATABASE_URL)"
	[[ -n "${db_url}" ]] || fail "POSTGRES_DB or DATABASE_URL must be set in .env.production"

	db_name="${db_url##*/}"
	db_name="${db_name%%\?*}"
	[[ -n "${db_name}" ]] || fail "could not derive database name from DATABASE_URL"
	printf '%s' "${db_name}"
}

for cmd in docker gzip wc; do
	command -v "${cmd}" >/dev/null 2>&1 || fail "required command not found on PATH: ${cmd}"
done

BACK_ROOT="$(read_env PROD_BACKUP_DIR)"
mkdir -p "${BACK_ROOT}"
BACK_ROOT_ABS="$(cd "${BACK_ROOT}" && pwd -P)"
DB_NAME="$(resolve_database_name)"

ts="$(date -u +%Y%m%dT%H%M%SZ)"
out="${BACK_ROOT_ABS}/${DB_NAME}_${ts}.sql.gz"
tmp="${out}.tmp"
COMPOSE=(docker compose --env-file .env.production -f docker-compose.prod.yml)
BACKUP_READY_WAIT_SECS="${BACKUP_POSTGRES_READY_WAIT_SECS:-90}"
BACKUP_READY_POLL_SECS="${BACKUP_POSTGRES_READY_POLL_SECS:-3}"

wait_for_postgres_ready() {
	local waited=0
	while (( waited <= BACKUP_READY_WAIT_SECS )); do
		if "${COMPOSE[@]}" exec -T postgres sh -c 'pg_isready -U "$POSTGRES_USER" -d "$1" >/dev/null' sh "${DB_NAME}"; then
			return 0
		fi
		sleep "${BACKUP_READY_POLL_SECS}"
		waited=$((waited + BACKUP_READY_POLL_SECS))
	done
	return 1
}

cleanup() {
	rm -f "${tmp}"
}
trap cleanup EXIT

log "backup destination directory: ${BACK_ROOT_ABS}"
log "backup output file: ${out}"
log "configured database name: ${DB_NAME}"
log "postgres readiness wait budget: ${BACKUP_READY_WAIT_SECS}s"

log "preflight: validate compose config"
"${COMPOSE[@]}" config >/dev/null

log "preflight: validate postgres readiness"
wait_for_postgres_ready || fail "postgres is not ready inside the prod compose stack for database ${DB_NAME} after ${BACKUP_READY_WAIT_SECS}s"

if [[ "${DRY_RUN:-0}" == "1" ]]; then
	log "DRY_RUN=1 set; preflight passed and no backup was written"
	exit 0
fi

log "writing compressed logical backup"
"${COMPOSE[@]}" exec -T postgres \
	sh -c 'pg_dump --if-exists --clean -U "$POSTGRES_USER" "$1"' sh "${DB_NAME}" | gzip -c >"${tmp}"

[[ -s "${tmp}" ]] || fail "backup file is empty: ${tmp}"
gzip -t "${tmp}" || fail "gzip integrity check failed for ${tmp}"
mv -f "${tmp}" "${out}"

size_bytes="$(wc -c <"${out}" | tr -d ' ')"
log "backup complete: ${out} (${size_bytes} bytes)"
