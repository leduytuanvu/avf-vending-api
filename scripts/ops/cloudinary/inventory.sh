#!/usr/bin/env bash
# Full paginated Cloudinary inventory (read-only).
# Usage: bash scripts/ops/cloudinary/inventory.sh [--env-file PATH]
set -Eeuo pipefail

# shellcheck source=lib_cloudinary.sh
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib_cloudinary.sh"

ENV_FILE=""
while [[ $# -gt 0 ]]; do
	case "$1" in
	--env-file)
		ENV_FILE="$2"
		shift 2
		;;
	-h | --help)
		echo "usage: inventory.sh [--env-file PATH]"
		exit 0
		;;
	*)
		cloudinary_fail "unknown argument: $1"
		;;
	esac
done

cloudinary_ensure_evidence_dir
if [[ -n "${ENV_FILE}" ]]; then
	cloudinary_load_env_file "${ENV_FILE}"
fi
cloudinary_require_credentials
cloudinary_note "inventory cloud_name=${CLOUDINARY_CLOUD_NAME}"
cloudinary_ops inventory
