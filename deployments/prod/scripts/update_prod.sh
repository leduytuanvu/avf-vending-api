#!/usr/bin/env bash
# Legacy single-host wrapper: re-applies image refs from .env.production via release.sh.
# Does not git pull, compile Go, or docker build application images on the server.
set -Eeuo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
RELEASE_SH="${ROOT}/scripts/release.sh"
ENV_FILE="${ROOT}/.env.production"

fail() {
	echo "update_prod: error: $*" >&2
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
	[[ "${ALLOW_LEGACY_SINGLE_HOST:-0}" == "1" ]] || fail "refusing to run legacy single-host update path without ALLOW_LEGACY_SINGLE_HOST=1"
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

require_legacy_ack
[[ -f "${ENV_FILE}" ]] || fail "missing ${ENV_FILE}"
[[ -f "${RELEASE_SH}" ]] || fail "missing ${RELEASE_SH}"

app_ref="$(read_env_value APP_IMAGE_REF)"
if [[ -z "${app_ref}" ]]; then
	app_ref="$(read_env_value APP_IMAGE_TAG)"
fi
if [[ -z "${app_ref}" ]]; then
	app_ref="$(read_env_value IMAGE_TAG)"
fi
[[ -n "${app_ref}" ]] || fail "set APP_IMAGE_REF or legacy APP_IMAGE_TAG / IMAGE_TAG in .env.production"

goose_ref="$(read_env_value GOOSE_IMAGE_REF)"
if [[ -z "${goose_ref}" ]]; then
	goose_ref="$(read_env_value GOOSE_IMAGE_TAG)"
fi
if [[ -z "${goose_ref}" ]]; then
	goose_ref="$(read_env_value IMAGE_TAG)"
fi
if [[ -z "${goose_ref}" ]]; then
	goose_ref="${app_ref}"
fi

echo "update_prod: redeploying APP_IMAGE_REF=${app_ref} GOOSE_IMAGE_REF=${goose_ref} via release.sh (no source build)"
if [[ "${app_ref}" == "${goose_ref}" ]]; then
	exec bash "${RELEASE_SH}" deploy "${app_ref}"
fi
exec bash "${RELEASE_SH}" deploy "${app_ref}" "${goose_ref}"
