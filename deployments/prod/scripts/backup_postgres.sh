#!/usr/bin/env bash
# Logical backup of the `avf_vending` database (gzip SQL). Requires a running postgres service.
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

for cmd in docker gzip wc; do
	command -v "${cmd}" >/dev/null 2>&1 || fail "required command not found on PATH: ${cmd}"
done

BACK_ROOT="$(read_env PROD_BACKUP_DIR)"
mkdir -p "${BACK_ROOT}"
BACK_ROOT_ABS="$(cd "${BACK_ROOT}" && pwd -P)"

ts="$(date -u +%Y%m%dT%H%M%SZ)"
out="${BACK_ROOT_ABS}/avf_vending_${ts}.sql.gz"
tmp="${out}.tmp"
COMPOSE=(docker compose --env-file .env.production -f docker-compose.prod.yml)

cleanup() {
	rm -f "${tmp}"
}
trap cleanup EXIT

log "backup destination directory: ${BACK_ROOT_ABS}"
log "backup output file: ${out}"

log "preflight: validate compose config"
"${COMPOSE[@]}" config >/dev/null

log "preflight: validate postgres readiness"
"${COMPOSE[@]}" exec -T postgres sh -c 'pg_isready -U "$POSTGRES_USER" -d avf_vending >/dev/null' || fail "postgres is not ready inside the prod compose stack"

if [[ "${DRY_RUN:-0}" == "1" ]]; then
	log "DRY_RUN=1 set; preflight passed and no backup was written"
	exit 0
fi

log "writing compressed logical backup"
"${COMPOSE[@]}" exec -T postgres \
	sh -c 'pg_dump --if-exists --clean -U "$POSTGRES_USER" avf_vending' | gzip -c >"${tmp}"

[[ -s "${tmp}" ]] || fail "backup file is empty: ${tmp}"
gzip -t "${tmp}" || fail "gzip integrity check failed for ${tmp}"
mv -f "${tmp}" "${out}"

size_bytes="$(wc -c <"${out}" | tr -d ' ')"
log "backup complete: ${out} (${size_bytes} bytes)"
