#!/usr/bin/env bash
# Thin wrapper: re-applies the image tag already in .env.production via release.sh (pull + migrate + stack).
# Does not git pull, compile Go, or docker build application images on the server.
set -Eeuo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
RELEASE_SH="${ROOT}/scripts/release.sh"
ENV_FILE="${ROOT}/.env.production"

fail() {
	echo "update_prod: error: $*" >&2
	exit 1
}

if [[ "${GIT_PULL:-}" == "1" ]]; then
	echo "update_prod: note: GIT_PULL=1 is ignored; production updates are image-only (see release.sh)." >&2
fi

[[ -f "${ENV_FILE}" ]] || fail "missing ${ENV_FILE}"
[[ -f "${RELEASE_SH}" ]] || fail "missing ${RELEASE_SH}"

line="$(grep -E "^APP_IMAGE_TAG=" "${ENV_FILE}" 2>/dev/null | tail -n1 || true)"
[[ -n "${line}" ]] || fail "APP_IMAGE_TAG must be set in .env.production"
tag="${line#APP_IMAGE_TAG=}"
tag="${tag%$'\r'}"
if [[ "${tag}" == \"*\" ]]; then
	tag="${tag#\"}"
	tag="${tag%\"}"
fi
[[ -n "${tag}" ]] || fail "APP_IMAGE_TAG is empty in .env.production"

echo "update_prod: redeploying APP_IMAGE_TAG=${tag} via release.sh (no source build)"
exec bash "${RELEASE_SH}" deploy "${tag}"
