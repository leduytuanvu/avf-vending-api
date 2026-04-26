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
bash "${SHARED_ROOT}/scripts/bootstrap_prereqs.sh" app-node
registry_login_optional

snapshot_revision previous
set_env_value "APP_IMAGE_REF" "${APP_IMAGE_REF_NEW}" 
set_env_value "GOOSE_IMAGE_REF" "${GOOSE_IMAGE_REF_NEW}"
compose_config_or_fail

PHASE="drain"
if [[ -f "${SHARED_ROOT}/scripts/traffic_drain_hook.sh" ]]; then
	note "traffic drain hook (TRAFFIC_DRAIN_MODE=${TRAFFIC_DRAIN_MODE:-none})"
	bash "${SHARED_ROOT}/scripts/traffic_drain_hook.sh"
else
	note "traffic_drain_hook.sh missing; using caddy stop only (record as no external drain)"
fi
note "drain app-node traffic by stopping caddy"
"${COMPOSE[@]}" stop caddy >/dev/null 2>&1 || true

PHASE="pull"
note "pull app-node images"
"${COMPOSE[@]}" pull "${PULL_SERVICES[@]}"

if [[ "${RUN_MIGRATION}" == "1" ]]; then
	PHASE="migrate"
	note "run one-shot migration profile"
	REPO_ROOT="$(cd "${NODE_ROOT}/../../.." && pwd)"
	set -a
	# shellcheck source=/dev/null
	source "${ENV_FILE}"
	set +a
	export APP_ENV="${APP_ENV:-production}"
	if [[ "${GITHUB_ACTIONS:-}" != "true" ]]; then
		if [[ "${CONFIRM_PRODUCTION_MIGRATION:-}" != "true" ]]; then
			echo "error: set CONFIRM_PRODUCTION_MIGRATION=true to run app-node migration from a shell" >&2
			exit 1
		fi
	fi
	if ! bash "${REPO_ROOT}/scripts/verify_database_environment.sh"; then
		echo "error: verify_database_environment.sh failed" >&2
		exit 1
	fi
	"${COMPOSE[@]}" --profile migration run --rm migrate
fi

PHASE="restart"
note "restart app workloads"
"${COMPOSE[@]}" up -d --remove-orphans --force-recreate "${SERVICES[@]}"

PHASE="verify-app"
APP_NODE_CHECK_CADDY="0" APP_NODE_ENABLE_TEMPORAL_PROFILE="${TEMPORAL_ENABLED}" bash "${NODE_ROOT}/scripts/healthcheck_app_node.sh"

PHASE="resume"
note "resume app-node traffic by starting caddy"
"${COMPOSE[@]}" up -d --remove-orphans caddy

PHASE="verify-caddy"
APP_NODE_CHECK_CADDY="1" APP_NODE_ENABLE_TEMPORAL_PROFILE="${TEMPORAL_ENABLED}" bash "${NODE_ROOT}/scripts/healthcheck_app_node.sh"

PHASE="persist"
snapshot_revision current
record_image_state "${APP_IMAGE_REF_NEW}" "${GOOSE_IMAGE_REF_NEW}" "${APP_IMAGE_REF_OLD}" "${GOOSE_IMAGE_REF_OLD}"
printf '%s\tdeploy\tapp=%s\tgoose=%s\n' "$(date -u +"%Y-%m-%dT%H:%M:%SZ")" "${APP_IMAGE_REF_NEW}" "${GOOSE_IMAGE_REF_NEW}" >>"${STATE_DIR}/history.log"

echo "release_app_node: PASS"
