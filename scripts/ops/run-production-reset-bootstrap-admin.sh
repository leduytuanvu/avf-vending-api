#!/usr/bin/env bash
# Orchestrate production reset (PostgreSQL wipe all auth + bootstrap admin + Redis/media/EMQX).
# Design-only companion to docs/runbooks/production-reset-bootstrap-admin.md.
#
# Usage:
#   CONFIRM_PRODUCTION_RESET=I_UNDERSTAND_THIS_WIPES_PRODUCTION \
#   bash scripts/ops/run-production-reset-bootstrap-admin.sh all
#
# Phases: preflight | postgres | redis | media | emqx | bootstrap | verify | all
set -Eeuo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
DEPLOY_ROOT="${AVF_DEPLOY_ROOT:-${ROOT}}"
OPS="${ROOT}/scripts/ops"
SHARED_SCRIPTS="${DEPLOY_ROOT}/deployments/prod/shared/scripts"
APP_COMPOSE="${DEPLOY_ROOT}/deployments/prod/app-node/docker-compose.app-node.yml"
APP_ENV="${DEPLOY_ROOT}/deployments/prod/app-node/.env.app-node"
POSTGRES_TOOLS_IMAGE="${POSTGRES_TOOLS_IMAGE:-postgres:17-alpine}"

fail() {
	echo "run-production-reset-bootstrap-admin: error: $*" >&2
	exit 1
}

note() {
	echo "run-production-reset-bootstrap-admin: $*"
}

require_confirm() {
	[[ "${CONFIRM_PRODUCTION_RESET:-}" == "I_UNDERSTAND_THIS_WIPES_PRODUCTION" ]] \
		|| fail "set CONFIRM_PRODUCTION_RESET=I_UNDERSTAND_THIS_WIPES_PRODUCTION"
}

load_app_env() {
	[[ -f "${APP_ENV}" ]] || fail "missing ${APP_ENV}"
	set -a
	# shellcheck disable=SC1090
	source "${APP_ENV}"
	set +a
	if [[ -z "${DATABASE_URL:-}" ]]; then
		DATABASE_URL="$(grep -E '^DATABASE_URL=' "${APP_ENV}" | tail -n1 | cut -d= -f2- | tr -d '\r' | sed -e 's/^"//' -e 's/"$//')"
		export DATABASE_URL
	fi
	[[ -n "${DATABASE_URL:-}" ]] || fail "DATABASE_URL is empty"
}

psql_exec() {
	local sql_file="$1"
	docker run --rm \
		--env-file "${APP_ENV}" \
		-e "DATABASE_URL=${DATABASE_URL}" \
		-v "${OPS}:/ops:ro" \
		"${POSTGRES_TOOLS_IMAGE}" \
		psql "${DATABASE_URL}" -v ON_ERROR_STOP=1 -f "/ops/$(basename "${sql_file}")"
}

psql_query() {
	local query="$1"
	docker run --rm \
		--env-file "${APP_ENV}" \
		-e "DATABASE_URL=${DATABASE_URL}" \
		"${POSTGRES_TOOLS_IMAGE}" \
		psql "${DATABASE_URL}" -v ON_ERROR_STOP=1 -Atqc "${query}"
}

compose_app() {
	docker compose --env-file "${APP_ENV}" -f "${APP_COMPOSE}" "$@"
}

phase_preflight() {
	require_confirm
	load_app_env
	note "Gate 1–2 — preflight backup + stop workers"
	timestamp="$(date -u +"%Y%m%dT%H%M%SZ")"
	backup_path="/var/backups/avf/prod/avf-prod-db-${timestamp}.dump"
	mkdir -p /var/backups/avf/prod
	if [[ -x "${SHARED_SCRIPTS}/backup_managed_postgres.sh" ]]; then
		bash "${SHARED_SCRIPTS}/backup_managed_postgres.sh" "${backup_path}" execute
	else
		note "backup_managed_postgres.sh not found — create Supabase snapshot manually"
	fi
	compose_app stop worker mqtt-ingest reconciler || true
}

phase_postgres() {
	require_confirm
	load_app_env
	note "Gate 4–7 — dry run then reset SQL"
	psql_exec "${OPS}/production-purge-dry-run.sql"
	docker run --rm \
		--env-file "${APP_ENV}" \
		-e "DATABASE_URL=${DATABASE_URL}" \
		-v "${OPS}:/ops:ro" \
		"${POSTGRES_TOOLS_IMAGE}" \
		psql "${DATABASE_URL}" -v ON_ERROR_STOP=1 \
		-c "SET avf.confirm_production_reset='I_UNDERSTAND_THIS_WIPES_PRODUCTION';" \
		-f /ops/production-reset-bootstrap-admin.sql
	local machines accounts
	machines="$(psql_query 'SELECT count(*) FROM machines;')"
	accounts="$(psql_query 'SELECT count(*) FROM platform_auth_accounts;')"
	[[ "${machines}" == "0" ]] || fail "expected 0 machines, got ${machines}"
	[[ "${accounts}" == "0" ]] || fail "expected 0 auth accounts before bootstrap, got ${accounts}"
	note "PostgreSQL reset verified"
}

phase_bootstrap() {
	require_confirm
	load_app_env
	[[ -n "${BOOTSTRAP_ADMIN_USERNAME:-}" ]] || fail "BOOTSTRAP_ADMIN_USERNAME required"
	[[ -n "${BOOTSTRAP_ADMIN_PASSWORD:-}" ]] || fail "BOOTSTRAP_ADMIN_PASSWORD required"
	note "Gate 9 — bootstrap admin"
	compose_app run --rm \
		-e "BOOTSTRAP_ADMIN_USERNAME=${BOOTSTRAP_ADMIN_USERNAME}" \
		-e "BOOTSTRAP_ADMIN_PASSWORD=${BOOTSTRAP_ADMIN_PASSWORD}" \
		api /app/bootstrap-admin
	accounts="$(psql_query 'SELECT count(*) FROM platform_auth_accounts;')"
	[[ "${accounts}" == "1" ]] || fail "expected 1 auth account after bootstrap, got ${accounts}"
}

phase_redis() {
	require_confirm
	load_app_env
	[[ -n "${REDIS_URL:-}" ]] || fail "REDIS_URL is empty"
	note "Redis FLUSHDB"
	if command -v redis-cli >/dev/null 2>&1; then
		redis-cli -u "${REDIS_URL}" FLUSHDB
	else
		docker run --rm redis:7-alpine redis-cli -u "${REDIS_URL}" FLUSHDB
	fi
}

phase_media() {
	require_confirm
	load_app_env
	note "Media purge — same as run-production-purge.sh phase_media"
	bash "${OPS}/run-production-purge.sh" media || true
}

phase_emqx() {
	require_confirm
	bash "${OPS}/run-production-purge.sh" emqx
}

phase_verify() {
	require_confirm
	load_app_env
	note "Gate 10–11 — verify counts"
	psql_exec "${OPS}/production-purge-dry-run.sql"
	curl -fsS "https://${API_DOMAIN:-api.ldtv.dev}/health/live" >/dev/null || true
	note "Manual: POST /v1/auth/login with username and verify platform_admin RBAC"
}

usage() {
	cat <<EOF
usage: CONFIRM_PRODUCTION_RESET=I_UNDERSTAND_THIS_WIPES_PRODUCTION \\
       BOOTSTRAP_ADMIN_USERNAME=admin BOOTSTRAP_ADMIN_PASSWORD=... \\
       bash scripts/ops/run-production-reset-bootstrap-admin.sh <phase>

phases: preflight | postgres | redis | media | emqx | bootstrap | verify | all
EOF
}

main() {
	local phase="${1:-all}"
	case "${phase}" in
	preflight) phase_preflight ;;
	postgres) phase_postgres ;;
	redis) phase_redis ;;
	media) phase_media ;;
	emqx) phase_emqx ;;
	bootstrap) phase_bootstrap ;;
	verify) phase_verify ;;
	all)
		phase_preflight
		phase_postgres
		phase_redis
		phase_media
		note "Skipping emqx on app-node — run phase emqx on data-node"
		phase_bootstrap
		phase_verify
		compose_app start worker mqtt-ingest reconciler || true
		;;
	*)
		usage
		fail "unknown phase: ${phase}"
		;;
	esac
}

main "$@"
