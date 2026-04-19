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

tag="${1:-}"
if [[ -z "${tag}" ]]; then
	if ! tag="$(read_env_tag APP_IMAGE_TAG)" || [[ -z "${tag}" ]]; then
		echo "deploy_prod: usage: $0 <tag>" >&2
		echo "deploy_prod:   or set APP_IMAGE_TAG in .env.production and run $0 with no arguments (delegates to release.sh deploy)." >&2
		exit 1
	fi
	echo "deploy_prod: no <tag> argument; using APP_IMAGE_TAG from .env.production: ${tag}"
fi

exec bash "${RELEASE_SH}" deploy "${tag}"
