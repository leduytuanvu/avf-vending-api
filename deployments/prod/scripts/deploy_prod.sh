#!/usr/bin/env bash
# Legacy single-host wrapper: delegates to release.sh
# (image-only; no source build on VPS).
set -Eeuo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
RELEASE_SH="${ROOT}/scripts/release.sh"
ENV_FILE="${ROOT}/.env.production"

fail() {
	echo "deploy_prod: error: $*" >&2
	exit 1
}

legacy_banner() {
	cat >&2 <<'EOF'
================================================================
LEGACY SINGLE-HOST PRODUCTION PATH
NOT THE PRIMARY 2-VPS RELEASE PATH
This wrapper exists only for legacy/rollback/reference use.
Set ALLOW_LEGACY_SINGLE_HOST=1 to proceed intentionally.
================================================================
EOF
}

require_legacy_ack() {
	legacy_banner
	[[ "${ALLOW_LEGACY_SINGLE_HOST:-0}" == "1" ]] || fail "refusing to run legacy single-host deploy path without ALLOW_LEGACY_SINGLE_HOST=1"
}

read_env_value() {
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

require_legacy_ack
[[ -f "${ENV_FILE}" ]] || fail "missing ${ENV_FILE} (copy from .env.production.example)"
[[ -f "${RELEASE_SH}" ]] || fail "missing ${RELEASE_SH}"

app_ref="${1:-}"
goose_ref="${2:-}"

if [[ -z "${app_ref}" ]]; then
	app_ref="$(read_env_value APP_IMAGE_REF 2>/dev/null || true)"
	if [[ -z "${app_ref}" ]]; then
		app_ref="$(read_env_value APP_IMAGE_TAG 2>/dev/null || true)"
	fi
	if [[ -z "${app_ref}" ]]; then
		app_ref="$(read_env_value IMAGE_TAG 2>/dev/null || true)"
	fi
	if [[ -z "${app_ref}" ]]; then
		echo "deploy_prod: usage: $0 [app_image_ref [goose_image_ref]]" >&2
		echo "deploy_prod:   or set APP_IMAGE_REF in .env.production (GOOSE_IMAGE_REF optional)." >&2
		exit 1
	fi
	echo "deploy_prod: no <app_image_ref> argument; using resolved app image selection from .env.production: ${app_ref}"
fi

if [[ -z "${goose_ref}" ]]; then
	goose_ref="$(read_env_value GOOSE_IMAGE_REF 2>/dev/null || true)"
	if [[ -z "${goose_ref}" ]]; then
		goose_ref="$(read_env_value GOOSE_IMAGE_TAG 2>/dev/null || true)"
	fi
	if [[ -z "${goose_ref}" ]]; then
		goose_ref="$(read_env_value IMAGE_TAG 2>/dev/null || true)"
	fi
	if [[ -z "${goose_ref}" ]]; then
		goose_ref="${app_ref}"
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

if [[ "${app_ref}" == "${goose_ref}" ]]; then
	exec bash "${RELEASE_SH}" deploy "${app_ref}"
fi
exec bash "${RELEASE_SH}" deploy "${app_ref}" "${goose_ref}"
