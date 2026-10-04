#!/usr/bin/env bash
# Read managed REDIS_URL from app-node A (live api container or sealed .env) and store in GitHub Environment secret.
set -Eeuo pipefail

fail() { echo "bootstrap_production_redis_url_secret: error: $*" >&2; exit 1; }
note() { echo "bootstrap_production_redis_url_secret: $*"; }

PRODUCTION_DEPLOY_ROOT="${PRODUCTION_DEPLOY_ROOT:-/opt/avf-vending-api}"
GITHUB_REPOSITORY="${GITHUB_REPOSITORY:-}"
PRIMARY_ENV="${PRODUCTION_DEPLOY_ROOT}/deployments/prod/app-node/.env.app-node"

[[ -n "${GITHUB_REPOSITORY}" ]] || fail "GITHUB_REPOSITORY is not set"

strip_env_quotes() {
	local v="$1"
	v="${v%\"}"
	v="${v#\"}"
	v="${v%\'}"
	v="${v#\'}"
	printf '%s' "${v}"
}

is_placeholder_value() {
	local value="${1-}"
	[[ -z "${value}" ]] && return 0
	case "${value}" in
	*"CHANGE_ME"* | *"REPLACE_ME"* | *"example.com"* | *"example.invalid"* | *"placeholder"*)
		return 0
		;;
	esac
	return 1
}

resolve_redis_url_from_api_container() {
	local name url
	while read -r name; do
		[[ "${name}" == *api* ]] || continue
		url="$(docker exec "${name}" printenv REDIS_URL 2>/dev/null | tr -d '\r' || true)"
		if [[ -n "${url}" ]] && ! is_placeholder_value "${url}"; then
			printf '%s' "${url}"
			return 0
		fi
	done < <(docker ps --format '{{.Names}}' 2>/dev/null || true)
	return 0
}

resolve_redis_url_from_env_files() {
	local f line url
	for f in "${PRIMARY_ENV}" "${PRIMARY_ENV}".bak* \
		"${PRODUCTION_DEPLOY_ROOT}/deployments/prod/.env.production" \
		"${PRODUCTION_DEPLOY_ROOT}/.env.production"; do
		[[ -f "${f}" ]] || continue
		while IFS= read -r line; do
			[[ "${line}" =~ ^REDIS_URL= ]] || continue
			url="$(strip_env_quotes "${line#REDIS_URL=}")"
			if [[ -n "${url}" ]] && ! is_placeholder_value "${url}"; then
				printf '%s' "${url}"
				return 0
			fi
		done < <(grep -E '^REDIS_URL=' "${f}" 2>/dev/null || true)
	done
	return 0
}

redis_url="$(resolve_redis_url_from_api_container)"
if [[ -z "${redis_url}" ]]; then
	redis_url="$(resolve_redis_url_from_env_files)"
fi
[[ -n "${redis_url}" ]] || fail "REDIS_URL not found on app-node A (api container and env files)"

if ! command -v gh >/dev/null 2>&1; then
	fail "gh CLI is required on the self-hosted runner"
fi

gh secret set PRODUCTION_REDIS_URL --env production --body "${redis_url}" --repo "${GITHUB_REPOSITORY}"
note "updated GitHub Environment secret PRODUCTION_REDIS_URL (value not logged)"
