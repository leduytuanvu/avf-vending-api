#!/usr/bin/env bash
# Cutover helper: dump current DATABASE_URL (Supabase) into local Postgres on this host.
# Run on VPS A after docker compose db-node is healthy.
# Does NOT rewrite app-node DATABASE_URL; do that after pg_restore + goose succeed.
set -Eeuo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DUMP_PATH="${1:-}"
SOURCE_URL="${SOURCE_DATABASE_URL:-${DATABASE_URL:-}}"
TARGET_URL="${TARGET_DATABASE_URL:-${BACKUP_DATABASE_URL:-}}"

[[ -n "${SOURCE_URL}" ]] || { echo "set SOURCE_DATABASE_URL or DATABASE_URL (Supabase dump source)" >&2; exit 1; }
[[ -n "${TARGET_URL}" ]] || { echo "set TARGET_DATABASE_URL or BACKUP_DATABASE_URL (local Postgres :5432)" >&2; exit 1; }

if [[ -z "${DUMP_PATH}" ]]; then
	DUMP_PATH="${ROOT}/backups/managed-postgres-$(date -u +%Y%m%dT%H%M%SZ).dump"
fi
mkdir -p "$(dirname "${DUMP_PATH}")"

echo "dumping source -> ${DUMP_PATH}"
pg_dump --format=custom --compress=9 --no-owner --no-privileges --file "${DUMP_PATH}" "${SOURCE_URL}"

echo "restoring into target (requires CONFIRM_MANAGED_RESTORE=managed-production-restore)"
[[ "${CONFIRM_MANAGED_RESTORE:-}" == "managed-production-restore" ]] || {
	echo "dump complete. To restore: CONFIRM_MANAGED_RESTORE=managed-production-restore TARGET_DATABASE_URL=... $0 ${DUMP_PATH}"
	exit 0
}

pg_restore \
	--clean \
	--if-exists \
	--no-owner \
	--no-privileges \
	--single-transaction \
	--exit-on-error \
	--dbname="${TARGET_URL}" \
	"${DUMP_PATH}"

echo "restore complete. Point both app-node DATABASE_URL at PgBouncer :6432, then goose up, then healthcheck."
