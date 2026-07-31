#!/usr/bin/env bash
set -Eeuo pipefail

NODE_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SHARED_ROOT="$(cd "${NODE_ROOT}/../shared" && pwd)"
# shellcheck source=../../shared/scripts/lib_release.sh
source "${SHARED_ROOT}/scripts/lib_release.sh"

ENV_FILE="${NODE_ROOT}/.env.app-node"
COMPOSE_FILE="${NODE_ROOT}/docker-compose.app-node.yml"
COMPOSE=(docker compose --env-file "${ENV_FILE}" -f "${COMPOSE_FILE}")
PHASE="startup"
REPO_ROOT="$(cd "${NODE_ROOT}/../../.." && pwd)"
MIGRATION_LOG_DIR="${PRODUCTION_MIGRATION_LOG_DIR:-/opt/avf-vending-api/deployments/prod/logs/migrations}"

trap 'rc=$?; if [[ "${rc}" -ne 0 ]]; then echo "release_app_node: failed during ${PHASE}" >&2; fi' EXIT

require_file "${ENV_FILE}"
require_file "${COMPOSE_FILE}"
init_state_dir

APP_IMAGE_REF_NEW="$(resolve_image_ref APP_IMAGE_REF "${1-}")"
GOOSE_IMAGE_REF_NEW="$(resolve_image_ref GOOSE_IMAGE_REF "${2-}")"
APP_IMAGE_REF_OLD="$(read_env_value APP_IMAGE_REF "")"
GOOSE_IMAGE_REF_OLD="$(read_env_value GOOSE_IMAGE_REF "")"
TEMPORAL_ENABLED="${APP_NODE_ENABLE_TEMPORAL_PROFILE:-0}"
RUN_MIGRATION="${RUN_MIGRATION:-0}"
SERVICES=(api worker reconciler mqtt-ingest)
PULL_SERVICES=(api worker reconciler mqtt-ingest caddy)

if [[ "${TEMPORAL_ENABLED}" == "1" ]]; then
	SERVICES+=(temporal-worker)
	PULL_SERVICES+=(temporal-worker)
fi

PHASE="validate"
note "validate app-node prerequisites"
run_script "${SHARED_ROOT}/scripts/bootstrap_prereqs.sh" app-node
registry_login_optional

snapshot_revision previous
set_env_value "APP_IMAGE_REF" "${APP_IMAGE_REF_NEW}"
set_env_value "GOOSE_IMAGE_REF" "${GOOSE_IMAGE_REF_NEW}"
compose_config_or_fail

PHASE="pull"
note "pull app-node images (before migration so embedded migrations match digest)"
"${COMPOSE[@]}" pull "${PULL_SERVICES[@]}"

if [[ "${RUN_MIGRATION}" == "1" ]]; then
	PHASE="migrate"
	note "production database backup + migration (before traffic drain; old containers keep serving if migrate fails)"
	export CONFIRM_PRODUCTION_MIGRATION=true
	migrate_args=(--compose-file "${COMPOSE_FILE}" --env-file "${ENV_FILE}")
	if [[ -n "${COMPOSE_PROJECT_NAME:-}" ]]; then
		migrate_args+=(--project-name "${COMPOSE_PROJECT_NAME}")
	fi
	if ! run_script "${REPO_ROOT}/scripts/deploy/production-migrate.sh" "${migrate_args[@]}"; then
		echo "error: production-migrate.sh failed; leaving running containers unchanged" >&2
		note "restoring previous .env.app-node snapshot after failed migration (no traffic drain or app recreate)"
		restore_revision previous
		exit 41
	fi
fi

PHASE="drain"
if [[ -f "${SHARED_ROOT}/scripts/traffic_drain_hook.sh" ]]; then
	note "traffic drain hook (TRAFFIC_DRAIN_MODE=${TRAFFIC_DRAIN_MODE:-none})"
	run_script "${SHARED_ROOT}/scripts/traffic_drain_hook.sh"
else
	note "traffic_drain_hook.sh missing; using caddy stop only (record as no external drain)"
fi
note "drain app-node traffic by stopping caddy"
"${COMPOSE[@]}" stop caddy >/dev/null 2>&1 || true

PHASE="restart"
note "restart app workloads with new image"
"${COMPOSE[@]}" up -d --remove-orphans --force-recreate "${SERVICES[@]}"

PHASE="resume"
note "resume app-node traffic by starting caddy (before verify so a failed smoke gate does not leave edge traffic drained)"
"${COMPOSE[@]}" up -d --remove-orphans caddy

PHASE="verify-app"
APP_NODE_CHECK_CADDY="0" APP_NODE_ENABLE_TEMPORAL_PROFILE="${TEMPORAL_ENABLED}" run_script "${NODE_ROOT}/scripts/healthcheck_app_node.sh"

PHASE="verify-caddy"
APP_NODE_CHECK_CADDY="1" APP_NODE_ENABLE_TEMPORAL_PROFILE="${TEMPORAL_ENABLED}" run_script "${NODE_ROOT}/scripts/healthcheck_app_node.sh"

PHASE="persist"
snapshot_revision current
record_image_state "${APP_IMAGE_REF_NEW}" "${GOOSE_IMAGE_REF_NEW}" "${APP_IMAGE_REF_OLD}" "${GOOSE_IMAGE_REF_OLD}"
printf '%s\tdeploy\tapp=%s\tgoose=%s\tmigration=%s\n' "$(date -u +"%Y-%m-%dT%H:%M:%SZ")" "${APP_IMAGE_REF_NEW}" "${GOOSE_IMAGE_REF_NEW}" "${RUN_MIGRATION}" >>"${STATE_DIR}/history.log"

echo "release_app_node: PASS"
