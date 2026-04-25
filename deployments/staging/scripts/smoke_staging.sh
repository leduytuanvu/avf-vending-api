#!/usr/bin/env bash
# Wrapper: resolves API_DOMAIN from .env.staging on the host, then runs repository smoke.
set -Eeuo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
STAGING_ROOT="$(cd "${HERE}/.." && pwd)"
REPO_ROOT="$(cd "${STAGING_ROOT}/../.." && pwd)"

if [[ -f "${STAGING_ROOT}/.env.staging" ]] && [[ -z "${STAGING_BASE_URL:-}" ]]; then
	d="$(grep -E '^API_DOMAIN=' "${STAGING_ROOT}/.env.staging" 2>/dev/null | tail -1 | cut -d= -f2- | tr -d '\r' | sed 's/^"//;s/"$//')"
	if [[ -n "${d}" ]]; then
		export STAGING_BASE_URL="https://${d}"
	fi
fi

exec bash "${REPO_ROOT}/scripts/smoke_staging.sh" "$@"
