#!/usr/bin/env bash
set -Eeuo pipefail

SHARED_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PROD_ROOT="$(cd "${SHARED_ROOT}/.." && pwd)"
REPO_ROOT="$(cd "${PROD_ROOT}/../.." && pwd)"
OUTPUT_PATH="${1:-${REPO_ROOT}/nightly-reports/runtime-assets.sha256}"

fail() {
	echo "error: $*" >&2
	exit 1
}

require_file() {
	local path="$1"
	[[ -f "${path}" ]] || fail "required file not found: ${path}"
}

require_cmd() {
	local cmd="$1"
	command -v "${cmd}" >/dev/null 2>&1 || fail "required command not found: ${cmd}"
}

require_cmd bash
require_cmd sha256sum

asset_files=(
	"${PROD_ROOT}/README.md"
	"${PROD_ROOT}/app-node/docker-compose.app-node.yml"
	"${PROD_ROOT}/app-node/.env.app-node.example"
	"${PROD_ROOT}/data-node/docker-compose.data-node.yml"
	"${PROD_ROOT}/data-node/.env.data-node.example"
	"${PROD_ROOT}/shared/Caddyfile"
	"${PROD_ROOT}/emqx/base.hocon"
)

for path in "${asset_files[@]}"; do
	require_file "${path}"
done

for path in "${SHARED_ROOT}/scripts/"*.sh; do
	require_file "${path}"
	bash -n "${path}"
done

if command -v docker >/dev/null 2>&1; then
	docker compose --env-file "${PROD_ROOT}/app-node/.env.app-node.example" -f "${PROD_ROOT}/app-node/docker-compose.app-node.yml" config >/dev/null
	docker compose --env-file "${PROD_ROOT}/app-node/.env.app-node.example" -f "${PROD_ROOT}/app-node/docker-compose.app-node.yml" --profile migration config >/dev/null
	docker compose --env-file "${PROD_ROOT}/app-node/.env.app-node.example" -f "${PROD_ROOT}/app-node/docker-compose.app-node.yml" --profile temporal config >/dev/null
	docker compose --env-file "${PROD_ROOT}/data-node/.env.data-node.example" -f "${PROD_ROOT}/data-node/docker-compose.data-node.yml" config >/dev/null
fi

mkdir -p "$(dirname "${OUTPUT_PATH}")"
: > "${OUTPUT_PATH}"

for path in "${asset_files[@]}" "${SHARED_ROOT}/scripts/"*.sh; do
	sha256sum "${path}" >> "${OUTPUT_PATH}"
done

echo "verify_runtime_assets: PASS (${OUTPUT_PATH})"
