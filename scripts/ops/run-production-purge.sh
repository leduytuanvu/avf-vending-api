#!/usr/bin/env bash
# Orchestrate production test-data purge (PostgreSQL + Redis + media + EMQX machine users).
# Run on app-node A with live deployments/prod/app-node/.env.app-node.
#
# Usage:
#   CONFIRM_PRODUCTION_PURGE=I_UNDERSTAND_THIS_WIPES_PRODUCTION \
#   bash scripts/ops/run-production-purge.sh all
#
# Phases: preflight | postgres | redis | media | emqx | restart | verify | all
set -Eeuo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
DEPLOY_ROOT="${AVF_DEPLOY_ROOT:-${ROOT}}"
OPS="${ROOT}/scripts/ops"
SHARED_SCRIPTS="${DEPLOY_ROOT}/deployments/prod/shared/scripts"
APP_COMPOSE="${DEPLOY_ROOT}/deployments/prod/app-node/docker-compose.app-node.yml"
APP_ENV="${DEPLOY_ROOT}/deployments/prod/app-node/.env.app-node"
DATA_ENV="${DEPLOY_ROOT}/deployments/prod/.env.production"
POSTGRES_TOOLS_IMAGE="${POSTGRES_TOOLS_IMAGE:-postgres:17-alpine}"

fail() {
	echo "run-production-purge: error: $*" >&2
	exit 1
}

note() {
	echo "run-production-purge: $*"
}

require_confirm() {
	[[ "${CONFIRM_PRODUCTION_PURGE:-}" == "I_UNDERSTAND_THIS_WIPES_PRODUCTION" ]] \
		|| fail "set CONFIRM_PRODUCTION_PURGE=I_UNDERSTAND_THIS_WIPES_PRODUCTION"
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

	note "Phase 0 — preflight backup + admin export + stop workers"

	local timestamp backup_path
	timestamp="$(date -u +"%Y%m%dT%H%M%SZ")"
	backup_path="/var/backups/avf-pre-wipe-${timestamp}.dump"
	mkdir -p /var/backups

	if [[ -x "${SHARED_SCRIPTS}/backup_managed_postgres.sh" ]]; then
		bash "${SHARED_SCRIPTS}/backup_managed_postgres.sh" "${backup_path}" execute
	else
		docker run --rm \
			--env-file "${APP_ENV}" \
			-e "DATABASE_URL=${DATABASE_URL}" \
			-v "/var/backups:/backup" \
			"${POSTGRES_TOOLS_IMAGE}" \
			pg_dump "${DATABASE_URL}" --format=custom --no-owner --no-privileges \
			--file "/backup/avf-pre-wipe-${timestamp}.dump"
	fi

	local keep_admin_id
	keep_admin_id="$(psql_query "SELECT id FROM platform_auth_accounts WHERE lower(email)='admin@avf.com';")"
	[[ -n "${keep_admin_id}" ]] || fail "admin@avf.com not found"
	note "KEEP_ADMIN_ID=${keep_admin_id}"
	note "Supabase snapshot: create manually in Dashboard → Database → Backups before postgres phase"

	compose_app stop worker mqtt-ingest reconciler || true
	note "Stopped worker, mqtt-ingest, reconciler (api left running)"
}

phase_postgres() {
	require_confirm
	load_app_env
	note "Phase 1 — PostgreSQL dry-run"
	psql_exec "${OPS}/production-purge-dry-run.sql"

	note "Phase 1 — PostgreSQL purge"
	docker run --rm \
		--env-file "${APP_ENV}" \
		-e "DATABASE_URL=${DATABASE_URL}" \
		-v "${OPS}:/ops:ro" \
		"${POSTGRES_TOOLS_IMAGE}" \
		psql "${DATABASE_URL}" -v ON_ERROR_STOP=1 \
		-c "SET avf.confirm_production_purge='I_UNDERSTAND_THIS_WIPES_PRODUCTION';" \
		-f /ops/production-purge-keep-admin.sql

	local machines products accounts
	machines="$(psql_query 'SELECT count(*) FROM machines;')"
	products="$(psql_query 'SELECT count(*) FROM products;')"
	accounts="$(psql_query 'SELECT count(*) FROM platform_auth_accounts;')"
	[[ "${machines}" == "0" ]] || fail "expected 0 machines, got ${machines}"
	[[ "${products}" == "0" ]] || fail "expected 0 products, got ${products}"
	[[ "${accounts}" == "1" ]] || fail "expected 1 auth account, got ${accounts}"
	note "PostgreSQL purge verified"
}

phase_redis() {
	require_confirm
	load_app_env
	[[ -n "${REDIS_URL:-}" ]] || fail "REDIS_URL is empty"

	note "Phase 2 — Redis FLUSHDB"
	if command -v redis-cli >/dev/null 2>&1; then
		redis-cli -u "${REDIS_URL}" FLUSHDB
	else
		docker run --rm redis:7-alpine redis-cli -u "${REDIS_URL}" FLUSHDB
	fi
	note "Redis flushed — admin must log in again"
}

phase_media() {
	require_confirm
	load_app_env
	note "Phase 3 — media purge"

	if [[ -n "${CLOUDINARY_CLOUD_NAME:-}" && -n "${CLOUDINARY_API_KEY:-}" && -n "${CLOUDINARY_API_SECRET:-}" ]]; then
		local folder="${CLOUDINARY_FOLDER:-avf-vending/products}"
		note "Deleting Cloudinary folder ${folder}"
		curl -fsS -X DELETE \
			"https://api.cloudinary.com/v1_1/${CLOUDINARY_CLOUD_NAME}/folders/${folder}" \
			-u "${CLOUDINARY_API_KEY}:${CLOUDINARY_API_SECRET}" \
			|| note "Cloudinary folder delete returned non-zero (may already be empty)"
	else
		note "Cloudinary env not configured — skip or purge via Console"
	fi

	if [[ -n "${OBJECT_STORAGE_BUCKET:-}" ]]; then
		if command -v aws >/dev/null 2>&1; then
			note "Deleting S3 prefix s3://${OBJECT_STORAGE_BUCKET}/"
			aws s3 rm "s3://${OBJECT_STORAGE_BUCKET}/" --recursive
		else
			note "OBJECT_STORAGE_BUCKET set but aws CLI missing — purge manually"
		fi
	else
		note "OBJECT_STORAGE_BUCKET not set — S3 skip"
	fi
}

phase_emqx() {
	require_confirm
	note "Phase 4 — EMQX machine MQTT users (run on VPS B / data-node only)"
	if ! curl -fsS --max-time 3 "http://127.0.0.1:18083/api/v5/status" >/dev/null 2>&1; then
		fail "EMQX management API not reachable on 127.0.0.1:18083 — SSH to data-node B and run: bash scripts/ops/emqx-purge-machine-users.sh"
	fi
	bash "${OPS}/emqx-purge-machine-users.sh"
}

phase_restart() {
	require_confirm
	load_app_env
	note "Phase 5 — restart workers"
	compose_app start worker mqtt-ingest reconciler
	compose_app ps
}

phase_verify() {
	require_confirm
	load_app_env
	note "Phase 5 — verify API + DB counts"

	curl -fsS "https://${API_DOMAIN:-api.ldtv.dev}/health/live" >/dev/null
	curl -fsS "https://${API_DOMAIN:-api.ldtv.dev}/version" || true

	psql_exec "${OPS}/production-purge-dry-run.sql"

	if [[ -x "${ROOT}/scripts/e2e/production-readonly-smoke.sh" && -n "${ADMIN_EMAIL:-}" && -n "${ADMIN_PASSWORD:-}" ]]; then
		BASE_URL="https://${API_DOMAIN:-api.ldtv.dev}" \
			bash "${ROOT}/scripts/e2e/production-readonly-smoke.sh"
	fi

	note "Manual: log in at https://admin.ldtv.dev/login and confirm empty /machines, /catalog/products"
}

usage() {
	cat <<EOF
usage: CONFIRM_PRODUCTION_PURGE=I_UNDERSTAND_THIS_WIPES_PRODUCTION \\
       bash scripts/ops/run-production-purge.sh <phase>

phases: preflight | postgres | redis | media | emqx | restart | verify | all | all-stack
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
	restart) phase_restart ;;
	verify) phase_verify ;;
	all)
		phase_preflight
		phase_postgres
		phase_redis
		phase_media
		note "Skipping emqx on app-node — run phase emqx on VPS B (data-node)"
		phase_restart
		phase_verify
		;;
	all-stack)
		phase_preflight
		phase_postgres
		phase_redis
		phase_media
		phase_emqx
		phase_restart
		phase_verify
		;;
	*)
		usage
		fail "unknown phase: ${phase}"
		;;
	esac
}

main "$@"
