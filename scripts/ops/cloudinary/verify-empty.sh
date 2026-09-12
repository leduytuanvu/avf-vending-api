#!/usr/bin/env bash
# Independent post-wipe verification (read-only Admin API).
# Usage: bash scripts/ops/cloudinary/verify-empty.sh [--env-file PATH]
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
		echo "usage: verify-empty.sh [--env-file PATH]"
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
cloudinary_note "verify-empty cloud_name=${CLOUDINARY_CLOUD_NAME}"
cloudinary_ops verify
