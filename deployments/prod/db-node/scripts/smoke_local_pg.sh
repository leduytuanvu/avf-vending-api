#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
# shellcheck disable=SC1091
source "${ROOT}/.env.db-node"
export PGPASSWORD="${POSTGRES_PASSWORD}"
echo "direct :5432"
psql "postgres://${POSTGRES_USER}@127.0.0.1:5432/${POSTGRES_DB}?sslmode=disable" -c "select current_database();"
echo "pgbouncer :6432"
psql "postgres://${POSTGRES_USER}@127.0.0.1:6432/${POSTGRES_DB}?sslmode=disable" -c "select current_database();"
echo "extensions"
psql "postgres://${POSTGRES_USER}@127.0.0.1:5432/${POSTGRES_DB}?sslmode=disable" -c "select nspname, extname from pg_extension e join pg_namespace n on n.oid=e.extnamespace;"
