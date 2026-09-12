#!/usr/bin/env bash
# Cloudinary full asset wipe with confirmations and evidence.
#
# Usage:
#   --inventory-only     Full inventory only (no delete)
#   --dry-run            Show what would be deleted
#   --verify-only        Post-wipe verification only
#   --delete-live-assets Requires CONFIRM_CLOUDINARY_LIVE_DELETE
#   --delete-empty-folders  Remove empty folders after asset delete
#
# Optional: --env-file PATH
set -Eeuo pipefail

# shellcheck source=lib_cloudinary.sh
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib_cloudinary.sh"

MODE="wipe"
ENV_FILE=""
DRY_RUN=0
DELETE_FOLDERS=0

while [[ $# -gt 0 ]]; do
	case "$1" in
	--inventory-only)
		MODE="inventory"
		shift
		;;
	--dry-run)
		DRY_RUN=1
		shift
		;;
	--verify-only)
		MODE="verify"
		shift
		;;
	--delete-live-assets)
		MODE="wipe"
		shift
		;;
	--delete-empty-folders)
		DELETE_FOLDERS=1
		shift
		;;
	--env-file)
		ENV_FILE="$2"
		shift 2
		;;
	-h | --help)
		cat <<EOF
usage: wipe.sh [options]

  --inventory-only       Read-only full inventory
  --dry-run              Wipe dry-run (no API deletes)
  --verify-only          Independent empty verification
  --delete-live-assets   Delete all assets (requires CONFIRM_CLOUDINARY_LIVE_DELETE)
  --delete-empty-folders Delete empty folders after wipe
  --env-file PATH        Load Cloudinary credentials from env file
EOF
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

case "${MODE}" in
inventory)
	cloudinary_note "mode=inventory-only cloud_name=${CLOUDINARY_CLOUD_NAME}"
	cloudinary_ops inventory
	;;
verify)
	cloudinary_note "mode=verify-only cloud_name=${CLOUDINARY_CLOUD_NAME}"
	cloudinary_ops verify
	;;
wipe)
	if [[ "${DRY_RUN}" -eq 0 ]]; then
		cloudinary_confirm_live_delete
	fi
	cloudinary_note "mode=wipe dry_run=${DRY_RUN} delete_folders=${DELETE_FOLDERS} cloud_name=${CLOUDINARY_CLOUD_NAME}"
	args=(wipe)
	[[ "${DRY_RUN}" -eq 1 ]] && args+=(--dry-run)
	[[ "${DELETE_FOLDERS}" -eq 1 ]] && args+=(--delete-folders)
	cloudinary_ops "${args[@]}"
	;;
esac
