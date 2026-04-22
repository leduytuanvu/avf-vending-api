#!/usr/bin/env bash
set -Eeuo pipefail

SHARED_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
# shellcheck source=./lib_release.sh
source "${SHARED_ROOT}/scripts/lib_release.sh"

ENV_FILE_PATH="${APP_NODE_ENV_FILE_PATH:-${SHARED_ROOT}/../../app-node/.env.app-node}"
OUTPUT_PATH="${1:-}"

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
pg_dump \
	--format=custom \
	--compress=9 \
	--no-owner \
	--no-privileges \
	--file "${OUTPUT_PATH}" \
	"${DATABASE_URL}"

if command -v sha256sum >/dev/null 2>&1; then
	sha256sum "${OUTPUT_PATH}" > "${OUTPUT_PATH}.sha256"
fi

echo "backup_managed_postgres: PASS (${OUTPUT_PATH})"
