#!/usr/bin/env bash
# Restore from a gzip SQL dump produced by backup_postgres.sh.
# This is disruptive and destructive: it replaces schema/data in the configured production database.
set -euo pipefail

INVOCATION_PWD="$(pwd -P)"
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

ENV_FILE="${RESTORE_ENV_FILE:-${ROOT}/.env.production}"
COMPOSE_FILE="${RESTORE_COMPOSE_FILE:-${ROOT}/docker-compose.prod.yml}"
RESTORE_REPORT_PATH="${RESTORE_REPORT_PATH:-}"

usage() {
	echo "usage: $0 [--preflight] [--yes] path/to/<database>_TIMESTAMP.sql.gz" >&2
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
	-h | --help)
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

if [[ ! -f "${ENV_FILE}" ]]; then
	fail "missing ${ENV_FILE}"
fi

read_env_optional() {
	local key="$1"
	local line
	line="$(grep -E "^${key}=" "${ENV_FILE}" | tail -n1 || true)"
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
	[[ -n "${db_url}" ]] || fail "POSTGRES_DB or DATABASE_URL must be set in ${ENV_FILE}"

	db_name="${db_url##*/}"
	db_name="${db_name%%\?*}"
	[[ -n "${db_name}" ]] || fail "could not derive database name from DATABASE_URL"
	printf '%s' "${db_name}"
}

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

COMPOSE=(docker compose --env-file "${ENV_FILE}" -f "${COMPOSE_FILE}")
IFS=' ' read -r -a WRITER_SERVICES <<<"${RESTORE_WRITER_SERVICES:-api worker mqtt-ingest reconciler caddy}"
IFS=' ' read -r -a LONG_LIVED_SERVICES <<<"${RESTORE_LONG_LIVED_SERVICES:-postgres nats emqx api worker mqtt-ingest reconciler caddy}"
DB_NAME="$(resolve_database_name)"
restore_started=0
restore_completed=0
restore_started_at_utc="$(date -u +"%Y-%m-%dT%H:%M:%SZ")"

json_escape() {
	local value="${1-}"
	value="${value//\\/\\\\}"
	value="${value//\"/\\\"}"
	value="${value//$'\n'/\\n}"
	value="${value//$'\r'/\\r}"
	value="${value//$'\t'/\\t}"
	printf '%s' "${value}"
}

join_by_space() {
	local joined=""
	local item
	for item in "$@"; do
		if [[ -n "${joined}" ]]; then
			joined+=" "
		fi
		joined+="${item}"
	done
	printf '%s' "${joined}"
}

write_restore_report() {
	local verdict="$1"
	local completed_at_utc

	[[ -n "${RESTORE_REPORT_PATH}" ]] || return 0
	completed_at_utc="$(date -u +"%Y-%m-%dT%H:%M:%SZ")"
	mkdir -p "$(dirname "${RESTORE_REPORT_PATH}")"
	cat >"${RESTORE_REPORT_PATH}.tmp.$$" <<EOF
{
  "verdict": "$(json_escape "${verdict}")",
  "preflight_only": ${PREFLIGHT_ONLY},
  "confirmed": ${CONFIRMED},
  "dump_path": "$(json_escape "${dump_abs}")",
  "dump_size_bytes": ${dump_size_bytes},
  "database_name": "$(json_escape "${DB_NAME}")",
  "env_file": "$(json_escape "${ENV_FILE}")",
  "compose_file": "$(json_escape "${COMPOSE_FILE}")",
  "writer_services": "$(json_escape "$(join_by_space "${WRITER_SERVICES[@]}")")",
  "long_lived_services": "$(json_escape "$(join_by_space "${LONG_LIVED_SERVICES[@]}")")",
  "restore_started": ${restore_started},
  "restore_completed": ${restore_completed},
  "started_at_utc": "$(json_escape "${restore_started_at_utc}")",
  "completed_at_utc": "$(json_escape "${completed_at_utc}")"
}
EOF
	mv -f "${RESTORE_REPORT_PATH}.tmp.$$" "${RESTORE_REPORT_PATH}"
}

warn_restore_failure() {
	if ((restore_started)); then
		echo "restore_postgres: restore failed after writers were stopped; inspect postgres and restart services manually when safe" >&2
	fi
}
trap warn_restore_failure ERR

log "restore source file: ${dump_abs}"
log "preflight: validate compose config (${COMPOSE_FILE})"
"${COMPOSE[@]}" config >/dev/null

log "preflight: validate gzip integrity"
gzip -t "${dump_abs}" || fail "gzip integrity check failed for ${dump_abs}"

dump_size_bytes="$(wc -c <"${dump_abs}" | tr -d ' ')"
log "dump size: ${dump_size_bytes} bytes"
log "configured database name: ${DB_NAME}"

log "preflight: validate postgres readiness"
"${COMPOSE[@]}" exec -T postgres sh -c 'pg_isready -U "$POSTGRES_USER" -d "$1" >/dev/null' sh "${DB_NAME}" || fail "postgres is not ready inside the prod compose stack for database ${DB_NAME}"

if ((PREFLIGHT_ONLY)); then
	write_restore_report "preflight_pass"
	log "preflight only; no services were stopped and no data was modified"
	exit 0
fi

if ((!CONFIRMED)); then
	fail "destructive restore requires explicit confirmation; rerun with --yes or use --preflight first"
fi

echo "restore_postgres: WARNING — this will replace schema/data in ${DB_NAME} using ${dump_abs}" >&2
echo "restore_postgres: WARNING — image rollback does not undo database state; restore is the data recovery path" >&2

log "stopping writers (api / worker / mqtt-ingest / reconciler / caddy)"
restore_started=1
"${COMPOSE[@]}" stop "${WRITER_SERVICES[@]}" >/dev/null 2>&1 || true

log "restoring schema/data from ${dump_abs}"
gunzip -c "${dump_abs}" | "${COMPOSE[@]}" exec -T postgres \
	sh -c 'psql -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d "$1"' sh "${DB_NAME}"

log "starting long-lived stack"
"${COMPOSE[@]}" up -d --remove-orphans "${LONG_LIVED_SERVICES[@]}"
restore_completed=1

trap - ERR
write_restore_report "restore_pass"
log "restore complete — verify application health and migration/schema expectations before reopening traffic"
