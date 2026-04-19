#!/usr/bin/env bash
# Thin wrapper: re-applies image tags from .env.production via release.sh (pull + migrate + stack).
# Does not git pull, compile Go, or docker build application images on the server.
set -Eeuo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
RELEASE_SH="${ROOT}/scripts/release.sh"
ENV_FILE="${ROOT}/.env.production"

fail() {
	echo "update_prod: error: $*" >&2
	exit 1
}

read_env_value() {
	local key="$1"
	local line
	line="$(grep -E "^${key}=" "${ENV_FILE}" 2>/dev/null | tail -n1 || true)"
	if [[ -z "${line}" ]]; then
		printf ''
		return 0
	fi
	line="${line#"${key}="}"
	line="${line%$'\r'}"
	if [[ "${line}" == \"*\" ]]; then
		line="${line#\"}"
		line="${line%\"}"
	fi
	printf '%s' "${line}"
}

if [[ "${GIT_PULL:-}" == "1" ]]; then
	echo "update_prod: note: GIT_PULL=1 is ignored; production updates are image-only (see release.sh)." >&2
fi

[[ -f "${ENV_FILE}" ]] || fail "missing ${ENV_FILE}"
[[ -f "${RELEASE_SH}" ]] || fail "missing ${RELEASE_SH}"

app_tag="$(read_env_value APP_IMAGE_TAG)"
if [[ -z "${app_tag}" ]]; then
	app_tag="$(read_env_value IMAGE_TAG)"
fi
[[ -n "${app_tag}" ]] || fail "set APP_IMAGE_TAG or legacy IMAGE_TAG in .env.production"

goose_tag="$(read_env_value GOOSE_IMAGE_TAG)"
if [[ -z "${goose_tag}" ]]; then
	goose_tag="$(read_env_value IMAGE_TAG)"
fi
if [[ -z "${goose_tag}" ]]; then
	goose_tag="${app_tag}"
fi

echo "update_prod: redeploying APP_IMAGE_TAG=${app_tag} GOOSE_IMAGE_TAG=${goose_tag} via release.sh (no source build)"
if [[ "${app_tag}" == "${goose_tag}" ]]; then
	exec bash "${RELEASE_SH}" deploy "${app_tag}"
fi
exec bash "${RELEASE_SH}" deploy "${app_tag}" "${goose_tag}"
