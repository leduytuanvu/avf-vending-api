#!/usr/bin/env bash
# Recreate app-node Go services so they pick up EMQX_* from .env.app-node
# without a digest-pinned image deploy.
set -Eeuo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
APP_NODE_ROOT="$(cd "${SCRIPT_DIR}/../../app-node" && pwd)"
# shellcheck source=./lib_release.sh
source "${SCRIPT_DIR}/lib_release.sh"

ENV_FILE="${APP_NODE_ENV_FILE:-${APP_NODE_ROOT}/.env.app-node}"
COMPOSE_FILE="${APP_NODE_COMPOSE_FILE:-${APP_NODE_ROOT}/docker-compose.app-node.yml}"
require_file "${ENV_FILE}"
require_file "${COMPOSE_FILE}"

COMPOSE=(docker compose --env-file "${ENV_FILE}" -f "${COMPOSE_FILE}")
SERVICES=(api worker reconciler mqtt-ingest)
note "recreate ${SERVICES[*]} to load EMQX management env"
"${COMPOSE[@]}" up -d --no-deps --force-recreate "${SERVICES[@]}"
note "reload_app_node_emqx_env: ok"
