#!/usr/bin/env bash
# Thin wrapper: historic entrypoint for operators. Delegates to release.sh (image-only; no source build on VPS).
set -Eeuo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
RELEASE_SH="${ROOT}/scripts/release.sh"
ENV_FILE="${ROOT}/.env.production"

fail() {
	echo "deploy_prod: error: $*" >&2
	exit 1
}

read_env_tag() {
	local key="$1"
	local line
	line="$(grep -E "^${key}=" "${ENV_FILE}" 2>/dev/null | tail -n1 || true)"
	if [[ -z "${line}" ]]; then
		return 1
	fi
	line="${line#"${key}="}"
	line="${line%$'\r'}"
	if [[ "${line}" == \"*\" ]]; then
		line="${line#\"}"
		line="${line%\"}"
	fi
	printf '%s' "${line}"
}

[[ -f "${ENV_FILE}" ]] || fail "missing ${ENV_FILE} (copy from .env.production.example)"
[[ -f "${RELEASE_SH}" ]] || fail "missing ${RELEASE_SH}"

tag_app="${1:-}"
tag_goose="${2:-}"

if [[ -z "${tag_app}" ]]; then
	tag_app="$(read_env_tag APP_IMAGE_TAG 2>/dev/null || true)"
	if [[ -z "${tag_app}" ]]; then
		tag_app="$(read_env_tag IMAGE_TAG 2>/dev/null || true)"
	fi
	if [[ -z "${tag_app}" ]]; then
		echo "deploy_prod: usage: $0 [app_image_tag [goose_image_tag]]" >&2
		echo "deploy_prod:   or set APP_IMAGE_TAG or IMAGE_TAG in .env.production (GOOSE_IMAGE_TAG optional)." >&2
		exit 1
	fi
	echo "deploy_prod: no <app_image_tag> argument; using resolved app tag from .env.production: ${tag_app}"
fi

if [[ -z "${tag_goose}" ]]; then
	tag_goose="$(read_env_tag GOOSE_IMAGE_TAG 2>/dev/null || true)"
	if [[ -z "${tag_goose}" ]]; then
		tag_goose="$(read_env_tag IMAGE_TAG 2>/dev/null || true)"
	fi
	if [[ -z "${tag_goose}" ]]; then
		tag_goose="${tag_app}"
	fi
fi

if ! grep -qE '^IMAGE_REGISTRY=' "${ENV_FILE}"; then
	fail ".env.production must define IMAGE_REGISTRY (required for GHCR image pulls)"
fi
if ! grep -qE '^APP_IMAGE_REPOSITORY=' "${ENV_FILE}"; then
	fail ".env.production must define APP_IMAGE_REPOSITORY"
fi
if ! grep -qE '^GOOSE_IMAGE_REPOSITORY=' "${ENV_FILE}"; then
	fail ".env.production must define GOOSE_IMAGE_REPOSITORY"
fi

if [[ "${tag_app}" == "${tag_goose}" ]]; then
	exec bash "${RELEASE_SH}" deploy "${tag_app}"
fi
exec bash "${RELEASE_SH}" deploy "${tag_app}" "${tag_goose}"
