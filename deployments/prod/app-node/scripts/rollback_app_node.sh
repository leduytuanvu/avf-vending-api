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

trap 'rc=$?; if [[ "${rc}" -ne 0 ]]; then echo "rollback_app_node: failed during ${PHASE}" >&2; fi' EXIT

require_file "${ENV_FILE}"
require_file "${COMPOSE_FILE}"
init_state_dir

TEMPORAL_ENABLED="${APP_NODE_ENABLE_TEMPORAL_PROFILE:-0}"
SERVICES=(api worker reconciler mqtt-ingest)
PULL_SERVICES=(api worker reconciler mqtt-ingest caddy)
if [[ "${TEMPORAL_ENABLED}" == "1" ]]; then
	SERVICES+=(temporal-worker)
	PULL_SERVICES+=(temporal-worker)
fi

APP_IMAGE_REF_OLD="$(read_env_value APP_IMAGE_REF "")"
GOOSE_IMAGE_REF_OLD="$(read_env_value GOOSE_IMAGE_REF "")"

PHASE="validate"
note "validate app-node rollback prerequisites"
bash "${SHARED_ROOT}/scripts/bootstrap_prereqs.sh" app-node
registry_login_optional

if [[ -n "${1-}" ]]; then
	APP_IMAGE_REF_TARGET="$(resolve_image_ref APP_IMAGE_REF "${1-}")"
	GOOSE_IMAGE_REF_TARGET="$(resolve_image_ref GOOSE_IMAGE_REF "${2-}")"
	snapshot_revision previous
	set_env_value "APP_IMAGE_REF" "${APP_IMAGE_REF_TARGET}"
	set_env_value "GOOSE_IMAGE_REF" "${GOOSE_IMAGE_REF_TARGET}"
else
	note "restore previous app-node compose revision snapshot"
	restore_revision previous
	APP_IMAGE_REF_TARGET="$(read_env_value APP_IMAGE_REF "")"
	GOOSE_IMAGE_REF_TARGET="$(read_env_value GOOSE_IMAGE_REF "")"
fi

compose_config_or_fail

PHASE="drain"
note "drain app-node traffic by stopping caddy"
"${COMPOSE[@]}" stop caddy >/dev/null 2>&1 || true

PHASE="pull"
note "pull rollback images"
"${COMPOSE[@]}" pull "${PULL_SERVICES[@]}"

PHASE="restart"
note "restart rollback app workloads"
"${COMPOSE[@]}" up -d --remove-orphans --force-recreate "${SERVICES[@]}"

PHASE="verify-app"
APP_NODE_ENABLE_TEMPORAL_PROFILE="${TEMPORAL_ENABLED}" bash "${NODE_ROOT}/scripts/healthcheck_app_node.sh"

PHASE="resume"
note "resume app-node traffic by starting caddy"
"${COMPOSE[@]}" up -d --remove-orphans caddy

PHASE="verify-caddy"
APP_NODE_ENABLE_TEMPORAL_PROFILE="${TEMPORAL_ENABLED}" bash "${NODE_ROOT}/scripts/healthcheck_app_node.sh"

PHASE="persist"
snapshot_revision current
record_image_state "${APP_IMAGE_REF_TARGET}" "${GOOSE_IMAGE_REF_TARGET}" "${APP_IMAGE_REF_OLD}" "${GOOSE_IMAGE_REF_OLD}"
printf '%s\trollback\tapp=%s\tgoose=%s\n' "$(date -u +"%Y-%m-%dT%H:%M:%SZ")" "${APP_IMAGE_REF_TARGET}" "${GOOSE_IMAGE_REF_TARGET}" >>"${STATE_DIR}/history.log"

echo "rollback_app_node: PASS"
