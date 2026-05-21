#!/usr/bin/env bash
# Apply Cloudinary media upload env from process environment to app-node .env.app-node.
# Never prints CLOUDINARY_API_SECRET. Intended for CI SSH remote apply or operator bootstrap.
#
# Usage: apply_cloudinary_app_node_env.sh [path/to/.env.app-node]
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
	}
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

MEDIA_PROVIDER="${MEDIA_PROVIDER:-cloudinary}"
MEDIA_UPLOAD_ENABLED="${MEDIA_UPLOAD_ENABLED:-true}"
CLOUDINARY_FOLDER="${CLOUDINARY_FOLDER:-avf-vending/products}"
MEDIA_MAX_IMAGE_SIZE_MB="${MEDIA_MAX_IMAGE_SIZE_MB:-5}"
MEDIA_ALLOWED_IMAGE_TYPES="${MEDIA_ALLOWED_IMAGE_TYPES:-image/jpeg,image/png,image/webp,image/gif}"

if norm_bool "${MEDIA_UPLOAD_ENABLED}"; then
	[[ -n "${CLOUDINARY_CLOUD_NAME:-}" ]] || {
		echo "error: MEDIA_UPLOAD_ENABLED=true requires CLOUDINARY_CLOUD_NAME" >&2
		exit 1
	}
	[[ -n "${CLOUDINARY_API_KEY:-}" ]] || {
		echo "error: MEDIA_UPLOAD_ENABLED=true requires CLOUDINARY_API_KEY" >&2
		exit 1
	}
	[[ -n "${CLOUDINARY_API_SECRET:-}" ]] || {
		echo "error: MEDIA_UPLOAD_ENABLED=true requires CLOUDINARY_API_SECRET (server-side secret; not logged)" >&2
		exit 1
	}
fi

set_env_kv "MEDIA_PROVIDER" "${MEDIA_PROVIDER}"
set_env_kv "MEDIA_UPLOAD_ENABLED" "$(norm_bool "${MEDIA_UPLOAD_ENABLED}" && printf true || printf false)"
set_env_kv "CLOUDINARY_CLOUD_NAME" "${CLOUDINARY_CLOUD_NAME:-}"
set_env_kv "CLOUDINARY_API_KEY" "${CLOUDINARY_API_KEY:-}"
set_env_kv "CLOUDINARY_API_SECRET" "${CLOUDINARY_API_SECRET:-}"
set_env_kv "CLOUDINARY_FOLDER" "${CLOUDINARY_FOLDER}"
set_env_kv "MEDIA_MAX_IMAGE_SIZE_MB" "${MEDIA_MAX_IMAGE_SIZE_MB}"
set_env_kv "MEDIA_ALLOWED_IMAGE_TYPES" "${MEDIA_ALLOWED_IMAGE_TYPES}"

echo "apply_cloudinary_app_node_env: ok (cloud_name=${CLOUDINARY_CLOUD_NAME:-<unset>}, api_secret=[REDACTED])"
