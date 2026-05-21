#!/usr/bin/env bash
# Validate GitHub Actions / deploy-time Cloudinary secrets before production rollout.
# Fails when MEDIA upload is enabled but credentials are incomplete.
#
# Env (from GitHub Actions secrets or operator shell):
#   MEDIA_UPLOAD_ENABLED (default true when PRODUCTION_SYNC_CLOUDINARY_ENV=1)
#   CLOUDINARY_CLOUD_NAME, CLOUDINARY_API_KEY, CLOUDINARY_API_SECRET
#   PRODUCTION_SYNC_CLOUDINARY_ENV — when 1, require full Cloudinary credential set
set -euo pipefail

norm_bool() {
	case "${1:-}" in
	1 | true | TRUE | yes | YES | on | ON) return 0 ;;
	*) return 1 ;;
	esac
}

is_set() {
	[[ -n "$(printf '%s' "${1:-}" | tr -d '[:space:]')" ]]
}

sync_enabled="0"
if norm_bool "${PRODUCTION_SYNC_CLOUDINARY_ENV:-0}"; then
	sync_enabled="1"
fi

has_name="0"
has_key="0"
has_secret="0"
is_set "${CLOUDINARY_CLOUD_NAME:-}" && has_name="1"
is_set "${CLOUDINARY_API_KEY:-}" && has_key="1"
is_set "${CLOUDINARY_API_SECRET:-}" && has_secret="1"

partial=$((has_name + has_key + has_secret))
if [[ "${partial}" -gt 0 && "${partial}" -lt 3 ]]; then
	echo "error: Cloudinary credentials are partially set (need all of CLOUDINARY_CLOUD_NAME, CLOUDINARY_API_KEY, CLOUDINARY_API_SECRET)" >&2
	exit 1
fi

if [[ "${sync_enabled}" == "1" ]]; then
	if [[ "${partial}" -ne 3 ]]; then
		echo "error: PRODUCTION_SYNC_CLOUDINARY_ENV=1 requires all Cloudinary GitHub secrets" >&2
		exit 1
	fi
	if norm_bool "${MEDIA_UPLOAD_ENABLED:-true}"; then
		echo "validate_cloudinary_deploy_secrets: ok (Cloudinary upload enabled; cloud_name=${CLOUDINARY_CLOUD_NAME})"
	else
		echo "validate_cloudinary_deploy_secrets: ok (sync requested but MEDIA_UPLOAD_ENABLED=false)"
	fi
	exit 0
fi

if norm_bool "${MEDIA_UPLOAD_ENABLED:-false}" && [[ "${partial}" -ne 3 ]]; then
	echo "error: MEDIA_UPLOAD_ENABLED=true requires complete Cloudinary credentials" >&2
	exit 1
fi

echo "validate_cloudinary_deploy_secrets: ok (Cloudinary sync not required for this deploy)"
