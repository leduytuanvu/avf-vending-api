#!/usr/bin/env bash
# Ensure production app-node env uses live QR/card payment (MoMo/ZaloPay/VietQR/VNPay/ShopeePay).
# Does not write PSP secrets — fill MOMO_* / ZALOPAY_* / VNP_* / SHOPEEPAY_* separately.
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

PAYMENT_ENV="${PAYMENT_ENV:-live}"
COMMERCE_PAYMENT_PROVIDER="${COMMERCE_PAYMENT_PROVIDER:-momo}"
COMMERCE_PAYMENT_PROVIDERS="${COMMERCE_PAYMENT_PROVIDERS:-momo,zalopay,vietqr,vnpay,shopeepay}"

set_env_kv "PAYMENT_ENV" "${PAYMENT_ENV}"
set_env_kv "COMMERCE_PAYMENT_PROVIDER" "${COMMERCE_PAYMENT_PROVIDER}"
set_env_kv "COMMERCE_PAYMENT_PROVIDERS" "${COMMERCE_PAYMENT_PROVIDERS}"

echo "apply_live_payment_app_node_env: ok (PAYMENT_ENV=${PAYMENT_ENV}, PROVIDER=${COMMERCE_PAYMENT_PROVIDER}, PROVIDERS=${COMMERCE_PAYMENT_PROVIDERS})"
echo "apply_live_payment_app_node_env: ensure MOMO_* / ZALOPAY_* / VNP_* / SHOPEEPAY_* credentials are set before restart"
