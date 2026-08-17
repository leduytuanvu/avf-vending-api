#!/usr/bin/env bash
# Merge LIVE PSP secrets from a drop-in file into .env.app-node without printing values.
# Usage:
#   1) Create /root/psp.secrets.env with MOMO_*/ZALOPAY_*/VNP_* (live values only)
#   2) merge_psp_secrets_app_node_env.sh [/path/to/.env.app-node] [/path/to/psp.secrets.env]
set -euo pipefail

ENV_FILE="${1:-${APP_NODE_ENV_FILE:-}}"
SECRETS_FILE="${2:-${PSP_SECRETS_FILE:-/root/psp.secrets.env}}"

if [[ -z "${ENV_FILE}" ]]; then
	SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
	ENV_FILE="${SCRIPT_DIR}/../../app-node/.env.app-node"
fi

if [[ ! -f "${ENV_FILE}" ]]; then
	echo "error: env file not found: ${ENV_FILE}" >&2
	exit 1
fi

if [[ ! -f "${SECRETS_FILE}" ]]; then
	echo "error: secrets file not found: ${SECRETS_FILE}" >&2
	echo "Create it with LIVE MOMO_*/ZALOPAY_*/VNP_* keys (chmod 600), then re-run." >&2
	exit 1
fi

set_env_kv() {
	local key="$1"
	local value="$2"
	if grep -q "^${key}=" "${ENV_FILE}"; then
		# Use awk to avoid sed delimiter issues with secret values.
		awk -v k="${key}" -v v="${value}" '
			BEGIN { done=0 }
			index($0, k "=") == 1 { print k "=" v; done=1; next }
			{ print }
			END { if (!done) print k "=" v }
		' "${ENV_FILE}" >"${ENV_FILE}.tmp"
		mv "${ENV_FILE}.tmp" "${ENV_FILE}"
	else
		printf '%s=%s\n' "${key}" "${value}" >>"${ENV_FILE}"
	fi
}

merged=0
skipped=0
while IFS= read -r line || [[ -n "${line}" ]]; do
	# skip comments/blank
	[[ -z "${line}" || "${line}" =~ ^[[:space:]]*# ]] && continue
	[[ "${line}" != *"="* ]] && continue
	key="${line%%=*}"
	value="${line#*=}"
	key="$(echo "${key}" | tr -d '[:space:]')"
	case "${key}" in
	MOMO_* | TFO_MOMO_* | ZALOPAY_* | VNP_* | VPN_*)
		if [[ -z "${value}" ]]; then
			echo "skip empty ${key}"
			skipped=$((skipped + 1))
			continue
		fi
		set_env_kv "${key}" "${value}"
		echo "merged ${key} (len=${#value})"
		merged=$((merged + 1))
		;;
	*)
		echo "skip non-psp key ${key}"
		skipped=$((skipped + 1))
		;;
	esac
done <"${SECRETS_FILE}"

chmod 600 "${ENV_FILE}" || true
echo "merge_psp_secrets_app_node_env: merged=${merged} skipped=${skipped}"
echo "Restart app-node API containers after merge, then verify: curl -sS https://api.ldtv.dev/version | jq .payment_runtime"
