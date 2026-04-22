#!/usr/bin/env bash
set -Eeuo pipefail

SHARED_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PROD_ROOT="$(cd "${SHARED_ROOT}/.." && pwd)"
REPO_ROOT="$(cd "${PROD_ROOT}/../.." && pwd)"
OUTPUT_PATH="${1:-${REPO_ROOT}/nightly-reports/runtime-assets.sha256}"
REPORT_PATH="${VERIFY_RUNTIME_ASSETS_REPORT_PATH:-}"

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
	compose_validation_status="validated"
else
	compose_validation_status="skipped-no-docker"
fi

mkdir -p "$(dirname "${OUTPUT_PATH}")"
: >"${OUTPUT_PATH}"

for path in "${asset_files[@]}" "${SHARED_ROOT}/scripts/"*.sh; do
	sha256sum "${path}" >>"${OUTPUT_PATH}"
done

if [[ -n "${REPORT_PATH}" ]]; then
	{
		printf '{\n'
		printf '  "sha256_inventory_path": "%s",\n' "$(printf '%s' "${OUTPUT_PATH}" | sed 's/\\/\\\\/g; s/"/\\"/g')"
		printf '  "asset_file_count": %s,\n' "${#asset_files[@]}"
		printf '  "shared_script_count": %s,\n' "$(printf '%s\n' "${SHARED_ROOT}/scripts/"*.sh | wc -l | tr -d ' ')"
		printf '  "compose_validation_status": "%s",\n' "${compose_validation_status}"
		printf '  "validated": ["required runtime asset presence","shared script bash syntax","runtime asset sha256 inventory"],\n'
		printf '  "verdict": "pass"\n'
		printf '}\n'
	} >"${REPORT_PATH}"
fi

echo "verify_runtime_assets: PASS (${OUTPUT_PATH})"
