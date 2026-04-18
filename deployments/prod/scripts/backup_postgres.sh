#!/usr/bin/env bash
# Logical backup of the `avf_vending` database (gzip SQL). Requires a running postgres service.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

if [[ ! -f .env.production ]]; then
	echo "error: missing .env.production" >&2
	exit 1
fi

read_env() {
	local key="$1"
	local line
	line="$(grep -E "^${key}=" .env.production | tail -n1 || true)"
	if [[ -z "${line}" ]]; then
		echo "error: ${key} not set in .env.production" >&2
		exit 1
	fi
	line="${line#"${key}="}"
	line="${line%$'\r'}"
	printf '%s' "${line}"
}

BACK_ROOT="$(read_env PROD_BACKUP_DIR)"
mkdir -p "${BACK_ROOT}"

ts="$(date -u +%Y%m%dT%H%M%SZ)"
out="${BACK_ROOT}/avf_vending_${ts}.sql.gz"

COMPOSE=(docker compose --env-file .env.production -f docker-compose.prod.yml)

echo "backup_postgres: writing ${out}"
"${COMPOSE[@]}" exec -T postgres \
	sh -c 'pg_dump --if-exists --clean -U "$POSTGRES_USER" avf_vending' | gzip -c >"${out}"

echo "backup_postgres: done (${out})"
