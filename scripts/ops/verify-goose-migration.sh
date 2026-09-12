#!/usr/bin/env bash
# Report Goose migration state for the target DATABASE_URL.
# Usage: DATABASE_URL=... bash scripts/ops/verify-goose-migration.sh
set -Eeuo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
MIGRATIONS="${ROOT}/migrations"
POSTGRES_TOOLS_IMAGE="${POSTGRES_TOOLS_IMAGE:-postgres:17-alpine}"

fail() {
	echo "verify-goose-migration: error: $*" >&2
	exit 1
}

[[ -n "${DATABASE_URL:-}" ]] || fail "DATABASE_URL is required"

strip_pgbouncer_url() {
	echo "$1" | sed -E 's/[?&]default_query_exec_mode=[^&]*//g; s/[?&]pgbouncer=[^&]*//g; s/\?&/?/g; s/\?$//'
}

DB_URL="$(strip_pgbouncer_url "${DATABASE_URL}")"
export DATABASE_URL="${DB_URL}"

source_count="$(find "${MIGRATIONS}" -maxdepth 1 -name '*.sql' 2>/dev/null | wc -l | tr -d ' ')"
source_latest="$(find "${MIGRATIONS}" -maxdepth 1 -name '*.sql' 2>/dev/null | sed 's/.*\///' | sort | tail -1 | sed 's/_.*//' | sed 's/^0*//')"

echo "=== verify-goose-migration ==="
echo "source_migration_files=${source_count}"
echo "source_latest_version=${source_latest}"

if command -v goose >/dev/null 2>&1; then
	goose -dir "${MIGRATIONS}" postgres "${DB_URL}" version 2>/dev/null || true
	goose -dir "${MIGRATIONS}" postgres "${DB_URL}" status 2>/dev/null || true
else
	docker run --rm \
		-e "DATABASE_URL=${DB_URL}" \
		-v "${MIGRATIONS}:/migrations:ro" \
		"${POSTGRES_TOOLS_IMAGE}" \
		sh -c 'apk add --no-cache go >/dev/null 2>&1 || true; \
		  if command -v goose >/dev/null 2>&1; then \
		    goose -dir /migrations postgres "$DATABASE_URL" version; \
		  else \
		    psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -Atqc \
		      "SELECT version_id, is_applied FROM goose_db_version ORDER BY version_id DESC LIMIT 5;"; \
		  fi'
fi

echo "--- goose_db_version (top 5) ---"
if command -v psql >/dev/null 2>&1; then
	psql "${DB_URL}" -v ON_ERROR_STOP=1 -c \
		"SELECT version_id, is_applied, tstamp FROM goose_db_version ORDER BY version_id DESC LIMIT 5;"
else
	docker run --rm -e "DATABASE_URL=${DB_URL}" "${POSTGRES_TOOLS_IMAGE}" \
		psql "${DB_URL}" -v ON_ERROR_STOP=1 -c \
		"SELECT version_id, is_applied, tstamp FROM goose_db_version ORDER BY version_id DESC LIMIT 5;"
fi
