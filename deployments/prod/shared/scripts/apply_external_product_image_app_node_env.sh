#!/usr/bin/env bash
# Apply external product image URL import env to app-node .env.app-node.
#
# Usage: apply_external_product_image_app_node_env.sh [path/to/.env.app-node]
set -euo pipefail

ENV_FILE="${1:-${APP_NODE_ENV_FILE:-}}"
if [[ -z "${ENV_FILE}" ]]; then
	SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
	ENV_FILE="${SCRIPT_DIR}/../../app-node/.env.app-node"
fi

if [[ ! -f "${ENV_FILE}" ]]; then
	echo "error: env file not found: ${ENV_FILE}" >&2
	echo "hint: copy deployments/prod/app-node/.env.app-node.example to .env.app-node first" >&2
	exit 1
fi

norm_bool() {
	case "${1:-}" in
	1 | true | TRUE | yes | YES | on | ON) return 0 ;;
	*) return 1 ;;
	esac
}

set_env_kv() {
	local key="$1"
	local value="$2"
	if grep -q "^${key}=" "${ENV_FILE}"; then
		sed -i "s|^${key}=.*|${key}=${value}|" "${ENV_FILE}"
	else
		printf '%s=%s\n' "${key}" "${value}" >>"${ENV_FILE}"
	fi
}

PRODUCT_IMAGE_EXTERNAL_URLS_ENABLED="${PRODUCT_IMAGE_EXTERNAL_URLS_ENABLED:-true}"
PRODUCT_IMAGE_EXTERNAL_URL_REQUIRE_HTTPS="${PRODUCT_IMAGE_EXTERNAL_URL_REQUIRE_HTTPS:-true}"
PRODUCT_IMAGE_EXTERNAL_URL_ALLOWED_HOSTS="${PRODUCT_IMAGE_EXTERNAL_URL_ALLOWED_HOSTS:-cdn.pixabay.com,res.cloudinary.com,images.unsplash.com,adm.avf.vn}"

set_env_kv "PRODUCT_IMAGE_EXTERNAL_URLS_ENABLED" "$(norm_bool "${PRODUCT_IMAGE_EXTERNAL_URLS_ENABLED}" && printf true || printf false)"
set_env_kv "PRODUCT_IMAGE_EXTERNAL_URL_REQUIRE_HTTPS" "$(norm_bool "${PRODUCT_IMAGE_EXTERNAL_URL_REQUIRE_HTTPS}" && printf true || printf false)"
set_env_kv "PRODUCT_IMAGE_EXTERNAL_URL_ALLOWED_HOSTS" "${PRODUCT_IMAGE_EXTERNAL_URL_ALLOWED_HOSTS}"

echo "apply_external_product_image_app_node_env: ok (enabled=$(norm_bool "${PRODUCT_IMAGE_EXTERNAL_URLS_ENABLED}" && printf true || printf false))"
