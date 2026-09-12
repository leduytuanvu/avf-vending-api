#!/usr/bin/env bash
# Cloudinary Admin API helpers. Never logs API secrets or CLOUDINARY_URL.
set -Eeuo pipefail

CLOUDINARY_OPS_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CLOUDINARY_OPS_ROOT="$(cd "${CLOUDINARY_OPS_DIR}/../../.." && pwd)"
CLOUDINARY_EVIDENCE_DIR="${CLOUDINARY_EVIDENCE_DIR:-${CLOUDINARY_OPS_ROOT}/.cloudinary-wipe-evidence}"

RESOURCE_TYPES=(image video raw)
DELIVERY_TYPES=(upload private authenticated)

cloudinary_fail() {
	echo "cloudinary: error: $*" >&2
	exit 1
}

cloudinary_note() {
	echo "cloudinary: $*"
}

cloudinary_ensure_evidence_dir() {
	mkdir -p "${CLOUDINARY_EVIDENCE_DIR}"
}

cloudinary_load_env_file() {
	local env_file="$1"
	[[ -f "${env_file}" ]] || cloudinary_fail "env file not found: ${env_file}"
	set -a
	# shellcheck disable=SC1090
	source "${env_file}"
	set +a
}

cloudinary_require_credentials() {
	[[ -n "${CLOUDINARY_CLOUD_NAME:-}" ]] || cloudinary_fail "CLOUDINARY_CLOUD_NAME is required"
	[[ -n "${CLOUDINARY_API_KEY:-}" ]] || cloudinary_fail "CLOUDINARY_API_KEY is required"
	[[ -n "${CLOUDINARY_API_SECRET:-}" ]] || cloudinary_fail "CLOUDINARY_API_SECRET is required"
}

cloudinary_py() {
	if command -v python3 >/dev/null 2>&1; then
		python3 "$@"
	elif command -v python >/dev/null 2>&1; then
		python "$@"
	else
		cloudinary_fail "python3 required"
	fi
}

# Run cloudinary_ops.py with current env (credentials must be set).
cloudinary_ops() {
	cloudinary_require_credentials
	cloudinary_py "${CLOUDINARY_OPS_DIR}/cloudinary_ops.py" "$@"
}

cloudinary_load_production_env() {
	local deploy_root="${AVF_DEPLOY_ROOT:-/opt/avf-vending-api}"
	cloudinary_load_env_file "${deploy_root}/deployments/prod/app-node/.env.app-node"
}

cloudinary_load_staging_env() {
	local deploy_root="${AVF_DEPLOY_ROOT:-/opt/avf-vending-api}"
	local f="${deploy_root}/deployments/staging/.env.staging"
	if [[ -f "${f}" ]]; then
		cloudinary_load_env_file "${f}"
		return 0
	fi
	cloudinary_fail "staging env not found: ${f}"
}

cloudinary_confirm_live_delete() {
	local cloud="${CLOUDINARY_CLOUD_NAME}"
	local expected="DELETE-ALL-CLOUDINARY-ASSETS-${cloud}"
	[[ "${CONFIRM_CLOUDINARY_LIVE_DELETE:-}" == "${expected}" ]] \
		|| cloudinary_fail "set CONFIRM_CLOUDINARY_LIVE_DELETE=${expected}"
}

cloudinary_confirm_backup_delete() {
	local cloud="${CLOUDINARY_CLOUD_NAME}"
	local expected="DELETE-ALL-CLOUDINARY-BACKUPS-IRREVERSIBLY-${cloud}"
	[[ "${CONFIRM_CLOUDINARY_BACKUP_DELETE:-}" == "${expected}" ]] \
		|| cloudinary_fail "set CONFIRM_CLOUDINARY_BACKUP_DELETE=${expected}"
}
