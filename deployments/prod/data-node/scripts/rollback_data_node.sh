#!/usr/bin/env bash
set -Eeuo pipefail

NODE_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SHARED_ROOT="$(cd "${NODE_ROOT}/../shared" && pwd)"
# shellcheck source=../../shared/scripts/lib_release.sh
source "${SHARED_ROOT}/scripts/lib_release.sh"

ENV_FILE="${NODE_ROOT}/.env.data-node"
COMPOSE_FILE="${NODE_ROOT}/docker-compose.data-node.yml"
COMPOSE=(docker compose --env-file "${ENV_FILE}" -f "${COMPOSE_FILE}")
PHASE="startup"

trap 'rc=$?; if [[ "${rc}" -ne 0 ]]; then echo "rollback_data_node: failed during ${PHASE}" >&2; fi' EXIT

require_file "${ENV_FILE}"
require_file "${COMPOSE_FILE}"
init_state_dir

PHASE="validate"
note "validate data-node rollback prerequisites"
bash "${SHARED_ROOT}/scripts/bootstrap_prereqs.sh" data-node
restore_revision previous
compose_config_or_fail

PHASE="pull"
note "pull rollback data-node images"
"${COMPOSE[@]}" pull nats emqx

PHASE="deploy"
note "restart rollback data-node services"
"${COMPOSE[@]}" up -d --remove-orphans nats emqx

PHASE="bootstrap-emqx"
note "ensure EMQX MQTT app user exists after rollback"
bash "${NODE_ROOT}/scripts/bootstrap_emqx_data_node.sh"

PHASE="verify"
bash "${NODE_ROOT}/scripts/healthcheck_data_node.sh"

PHASE="persist"
snapshot_revision current
printf '%s\trollback\n' "$(date -u +"%Y-%m-%dT%H:%M:%SZ")" >>"${STATE_DIR}/history.log"

echo "rollback_data_node: PASS"
