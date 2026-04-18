#!/usr/bin/env bash
# Restore from a gzip SQL dump produced by backup_postgres.sh.
# This is disruptive: stop API traffic before restore to avoid concurrent writes.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

if [[ $# -ne 1 ]]; then
	echo "usage: $0 path/to/avf_vending_TIMESTAMP.sql.gz" >&2
	exit 1
fi

if [[ ! -f .env.production ]]; then
	echo "error: missing .env.production" >&2
	exit 1
fi

dump="$1"
if [[ ! -f "${dump}" ]]; then
	echo "error: dump file not found: ${dump}" >&2
	exit 1
fi

COMPOSE=(docker compose --env-file .env.production -f docker-compose.prod.yml)
LONG_LIVED_SERVICES=(postgres nats emqx api worker mqtt-ingest reconciler caddy)

echo "restore_postgres: stopping writers (api / worker / mqtt-ingest / reconciler / caddy)"
"${COMPOSE[@]}" stop api worker mqtt-ingest reconciler caddy >/dev/null 2>&1 || true

echo "restore_postgres: recreating schema from ${dump}"
gunzip -c "${dump}" | "${COMPOSE[@]}" exec -T postgres \
	sh -c 'psql -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d avf_vending'

echo "restore_postgres: starting stack"
"${COMPOSE[@]}" up -d --remove-orphans "${LONG_LIVED_SERVICES[@]}"

echo "restore_postgres: done — verify migrations/version if restoring across releases"
