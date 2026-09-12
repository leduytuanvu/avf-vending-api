#!/usr/bin/env bash
# Stop or quiesce AVF writers before database destruction.
# Usage:
#   quiesce_writers.sh --environment development|staging|production [--dry-run]
set -Eeuo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib_db_destroy.sh
source "${SCRIPT_DIR}/lib_db_destroy.sh"

ENVIRONMENT=""
DRY_RUN=0

usage() {
	cat <<'EOF'
usage: quiesce_writers.sh --environment ENV [options]

Options:
  --environment ENV   development | test | staging | production
  --dry-run           Print actions without executing
  -h, --help          Show help

Repopulation blockers (documented; not auto-executed):
  - Revoke machine JWTs / keep API down after destruction
  - Block GitHub deploy/migrate workflows
  - Do not restart Postgres with POSTGRES_DB init
EOF
}

while [[ $# -gt 0 ]]; do
	case "$1" in
	--environment)
		ENVIRONMENT="${2:-}"
		shift 2
		;;
	--dry-run)
		DRY_RUN=1
		shift
		;;
	-h | --help)
		usage
		exit 0
		;;
	*)
		db_destroy_fail "unknown argument: $1"
		;;
	esac
done

[[ -n "${ENVIRONMENT}" ]] || db_destroy_fail "--environment is required"

run_or_echo() {
	if [[ "${DRY_RUN}" -eq 1 ]]; then
		db_destroy_note "[dry-run] $*"
	else
		db_destroy_note "$*"
		eval "$@"
	fi
}

case "${ENVIRONMENT}" in
development | test)
	DC="docker compose -f ${DB_DESTROY_REPO_ROOT}/deployments/docker/docker-compose.yml"
	run_or_echo "${DC} stop api worker mqtt-ingest reconciler 2>/dev/null || true"
	;;
staging)
	STG="${DB_DESTROY_REPO_ROOT}/deployments/staging"
	run_or_echo "cd '${STG}' && docker compose --env-file .env.staging -f docker-compose.staging.yml stop api worker mqtt-ingest reconciler caddy 2>/dev/null || true"
	;;
production)
	APP="${DB_DESTROY_REPO_ROOT}/deployments/prod/app-node"
	run_or_echo "cd '${APP}' && docker compose --env-file .env.app-node -f docker-compose.app-node.yml stop api worker mqtt-ingest reconciler 2>/dev/null || true"
	LEG="${DB_DESTROY_REPO_ROOT}/deployments/prod"
	run_or_echo "cd '${LEG}' && docker compose --env-file .env.production -f docker-compose.prod.yml stop api worker mqtt-ingest reconciler caddy 2>/dev/null || true"
	;;
*)
	db_destroy_fail "unsupported environment: ${ENVIRONMENT}"
	;;
esac

db_destroy_note "repopulation blockers checklist:"
db_destroy_note "  [ ] GitHub production-migrate / deploy workflows held"
db_destroy_note "  [ ] Machine credentials revoked or fleet factory-reset (see APP outbox replay risk)"
db_destroy_note "  [ ] NATS/EMQX consumers stopped if API will not restart"
db_destroy_note "  [ ] Do not run goose up or dev-migrate after destruction"

db_destroy_append_evidence "quiesce" "${ENVIRONMENT}" "OK" "dry_run=${DRY_RUN}"
db_destroy_note "quiesce_writers: OK"
