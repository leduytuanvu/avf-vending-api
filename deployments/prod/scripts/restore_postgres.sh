#!/usr/bin/env bash
# Restore from a gzip SQL dump produced by backup_postgres.sh.
# This is disruptive and destructive: it replaces schema/data in avf_vending.
set -euo pipefail

INVOCATION_PWD="$(pwd -P)"
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

usage() {
	echo "usage: $0 [--preflight] [--yes] path/to/avf_vending_TIMESTAMP.sql.gz" >&2
}

log() {
	echo "==> $*"
}

fail() {
	echo "error: $*" >&2
	exit 1
}

PREFLIGHT_ONLY=0
CONFIRMED=0

while [[ $# -gt 0 ]]; do
	case "$1" in
		--preflight)
			PREFLIGHT_ONLY=1
			shift
			;;
		--yes)
			CONFIRMED=1
			shift
			;;
		-h|--help)
			usage
			exit 0
			;;
		--)
			shift
			break
			;;
		-*)
			usage
			fail "unknown flag: $1"
			;;
		*)
			break
			;;
	esac
done

if [[ $# -ne 1 ]]; then
	usage
	fail "exactly one dump path is required"
fi

if [[ ! -f .env.production ]]; then
	fail "missing .env.production"
fi

dump_input="$1"
case "${dump_input}" in
	/*)
		dump_abs="${dump_input}"
		;;
	*)
		if [[ -f "${dump_input}" ]]; then
			dump_abs="$(cd "$(dirname "${dump_input}")" && pwd -P)/$(basename "${dump_input}")"
		elif [[ -f "${INVOCATION_PWD}/${dump_input}" ]]; then
			dump_abs="$(cd "${INVOCATION_PWD}/$(dirname "${dump_input}")" && pwd -P)/$(basename "${dump_input}")"
		else
			fail "dump file not found: ${dump_input}"
		fi
		;;
esac

[[ -f "${dump_abs}" ]] || fail "resolved dump path does not exist: ${dump_abs}"

for cmd in docker gunzip gzip wc; do
	command -v "${cmd}" >/dev/null 2>&1 || fail "required command not found on PATH: ${cmd}"
done

COMPOSE=(docker compose --env-file .env.production -f docker-compose.prod.yml)
WRITER_SERVICES=(api worker mqtt-ingest reconciler caddy)
LONG_LIVED_SERVICES=(postgres nats emqx api worker mqtt-ingest reconciler caddy)
restore_started=0

warn_restore_failure() {
	if (( restore_started )); then
		echo "restore_postgres: restore failed after writers were stopped; inspect postgres and restart services manually when safe" >&2
	fi
}
trap warn_restore_failure ERR

log "restore source file: ${dump_abs}"
log "preflight: validate compose config"
"${COMPOSE[@]}" config >/dev/null

log "preflight: validate gzip integrity"
gzip -t "${dump_abs}" || fail "gzip integrity check failed for ${dump_abs}"

dump_size_bytes="$(wc -c <"${dump_abs}" | tr -d ' ')"
log "dump size: ${dump_size_bytes} bytes"

log "preflight: validate postgres readiness"
"${COMPOSE[@]}" exec -T postgres sh -c 'pg_isready -U "$POSTGRES_USER" -d avf_vending >/dev/null' || fail "postgres is not ready inside the prod compose stack"

if (( PREFLIGHT_ONLY )); then
	log "preflight only; no services were stopped and no data was modified"
	exit 0
fi

if (( ! CONFIRMED )); then
	fail "destructive restore requires explicit confirmation; rerun with --yes or use --preflight first"
fi

echo "restore_postgres: WARNING — this will replace schema/data in avf_vending using ${dump_abs}" >&2
echo "restore_postgres: WARNING — image rollback does not undo database state; restore is the data recovery path" >&2

log "stopping writers (api / worker / mqtt-ingest / reconciler / caddy)"
restore_started=1
"${COMPOSE[@]}" stop "${WRITER_SERVICES[@]}" >/dev/null 2>&1 || true

log "restoring schema/data from ${dump_abs}"
gunzip -c "${dump_abs}" | "${COMPOSE[@]}" exec -T postgres \
	sh -c 'psql -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d avf_vending'

log "starting long-lived stack"
"${COMPOSE[@]}" up -d --remove-orphans "${LONG_LIVED_SERVICES[@]}"

trap - ERR
log "restore complete — verify application health and migration/schema expectations before reopening traffic"
