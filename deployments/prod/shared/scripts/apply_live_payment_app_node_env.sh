#!/usr/bin/env bash
# Ensure production app-node env uses live QR/card payment (MoMo / ZaloPay / VietQR / VNPay).
# Does not write PSP secrets — fill MOMO_* / ZALOPAY_* / VNP_* separately with LIVE credentials.
# Usage: apply_live_payment_app_node_env.sh [path/to/.env.app-node]
set -euo pipefail

ENV_FILE="${1:-${APP_NODE_ENV_FILE:-}}"
if [[ -z "${ENV_FILE}" ]]; then
	SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
	ENV_FILE="${SCRIPT_DIR}/../../app-node/.env.app-node"
fi

if [[ ! -f "${ENV_FILE}" ]]; then
	echo "error: env file not found: ${ENV_FILE}" >&2
	exit 1
fi

set_env_kv() {
	local key="$1"
	local value="$2"
	if grep -q "^${key}=" "${ENV_FILE}"; then
		sed -i "s|^${key}=.*|${key}=${value}|" "${ENV_FILE}"
	else
		printf '%s=%s\n' "${key}" "${value}" >>"${ENV_FILE}"
	fi
}

set_env_kv "PAYMENT_ENV" "live"
set_env_kv "COMMERCE_PAYMENT_PROVIDER" "${COMMERCE_PAYMENT_PROVIDER:-momo}"
set_env_kv "COMMERCE_PAYMENT_PROVIDERS" "${COMMERCE_PAYMENT_PROVIDERS:-momo,zalopay,vietqr,vnpay}"

# Prefer new-API IPN URLs when unset or still pointing at legacy Django payment-service paths.
PUBLIC_BASE="${PUBLIC_BASE_URL:-https://api.ldtv.dev}"
PUBLIC_BASE="${PUBLIC_BASE%/}"

ensure_webhook_url() {
	local key="$1"
	local path="$2"
	local desired="${PUBLIC_BASE}${path}"
	if grep -q "^${key}=" "${ENV_FILE}"; then
		local current
		current="$(grep "^${key}=" "${ENV_FILE}" | head -n1 | cut -d= -f2-)"
		if [[ -z "${current}" || "${current}" == *"payment-service"* || "${current}" == *"dev-api.avf.vn"* ]]; then
			set_env_kv "${key}" "${desired}"
		fi
	else
		set_env_kv "${key}" "${desired}"
	fi
}

ensure_webhook_url "MOMO_IPN_URL" "/v1/commerce/webhooks/momo"
ensure_webhook_url "ZALOPAY_CALLBACK_URL" "/v1/commerce/webhooks/zalopay"
ensure_webhook_url "VNP_RETURN_URL" "/v1/commerce/webhooks/vnpay/return"

echo "apply_live_payment_app_node_env: ok (PAYMENT_ENV=live, COMMERCE_PAYMENT_PROVIDER=$(grep '^COMMERCE_PAYMENT_PROVIDER=' "${ENV_FILE}" | cut -d= -f2-), providers=$(grep '^COMMERCE_PAYMENT_PROVIDERS=' "${ENV_FILE}" | cut -d= -f2-))"
echo "apply_live_payment_app_node_env: ensure LIVE MOMO_*/ZALOPAY_*/VNP_* secrets are filled; restart app-node after edit"
