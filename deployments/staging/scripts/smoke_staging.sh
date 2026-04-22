#!/usr/bin/env bash
set -Eeuo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT}"

fail() {
	echo "error: $*" >&2
	exit 1
}

if [[ ! -f .env.staging ]]; then
	fail "missing .env.staging"
fi

read_env() {
	local key="$1"
	local line
	line="$(grep -E "^${key}=" .env.staging | tail -n1 || true)"
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

API_DOMAIN="${STAGING_SMOKE_API_DOMAIN:-$(read_env API_DOMAIN)}"
BASE_URL="${STAGING_SMOKE_BASE_URL:-https://${API_DOMAIN}}"
HTTP_SMOKE="${ROOT}/../prod/shared/scripts/smoke_http.sh"

[[ -n "${API_DOMAIN}" ]] || fail "missing API_DOMAIN"
[[ -f "${HTTP_SMOKE}" ]] || fail "missing shared smoke helper: ${HTTP_SMOKE}"

bash "${HTTP_SMOKE}" "staging public /health/live" "${BASE_URL}/health/live" '^ok$'
bash "${HTTP_SMOKE}" "staging public /health/ready" "${BASE_URL}/health/ready" '^ok$'
bash "${HTTP_SMOKE}" "staging public /version" "${BASE_URL}/version" '"version"[[:space:]]*:'

echo "smoke_staging: PASS"
