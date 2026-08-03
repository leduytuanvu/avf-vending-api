# Production payment provider status

This document is the operator and Android handoff reference for **which payment methods are safe to expose** in production.

## Chosen path: **Path A — multi-provider live PSP + dual surface**

Live VN PSPs are implemented as `WiredLiveProvider` adapters: **`momo`**, **`zalopay`**, **`vietqr`**, **`vnpay`**, **`shopeepay`**. `stripe` remains an unwired placeholder shell.

Production may run:

| Mode | When |
|------|------|
| `PAYMENT_ENV=live` | **Default in production examples** — wired MoMo / ZaloPay / VietQR (+ optional VNPay/ShopeePay) |
| `PAYMENT_ENV=cash_only` | Explicit cash pilot; QR/card hidden |

**Android rule:** show cash when `payment_methods.cash_enabled=true`. Show QR/card only when `payment_methods.qr_card_enabled=true`.

**Recommended live allowlist (examples + `apply_live_payment_app_node_env.sh`):**

```
PAYMENT_ENV=live
COMMERCE_PAYMENT_PROVIDER=momo
COMMERCE_PAYMENT_PROVIDERS=momo,zalopay,vietqr,vnpay,shopeepay
```

Fill `MOMO_*` / `ZALOPAY_*` (and VietQR) from the secret store. To revert to cash pilot, use `apply_cash_only_payment_app_node_env.sh`.

## Surfaces (Hướng 3)

| Surface | Path | Clients |
|---------|------|---------|
| gRPC | `CreatePaymentSession` + poll `GetOrderStatus` | `avf-vending-app` |
| Legacy HTTP | `/payment-service/payment/*` when `ENABLE_LEGACY_PAYMENT_HTTP=true` | Old machines (domain-only cutover) |
| Native IPN | `/v1/commerce/webhooks/{momo,zalopay,shopeepay}` + `GET .../vnpay/return` | PSP portals |
| Legacy IPN aliases | `/payment-service/payment/momo/callback`, `/callback`, `/shopeepay/callback`, `/vnpay_return` | Same handlers; keep old path on new domain |
| MQTT push | `command_type=payment.captured` after IPN capture | New app (poll remains mandatory fallback) |

## Config

| Env | Role |
|-----|------|
| `PAYMENT_ENV` | `sandbox` \| `live` \| `cash_only` |
| `COMMERCE_PAYMENT_PROVIDER` | Default registry key |
| `COMMERCE_PAYMENT_PROVIDERS` | CSV allowlist for multi-method (e.g. `momo,zalopay,vietqr,vnpay,shopeepay`) |
| `COMMERCE_PAYMENT_PUSH_MQTT` | Enqueue `payment.captured` (default true) |
| `ENABLE_LEGACY_PAYMENT_HTTP` | Mount legacy `/payment-service/payment/*` |
| `LEGACY_PAYMENT_HTTP_ALLOW_IN_PRODUCTION` | Required when legacy payment HTTP is on in production |
| `MOMO_*` / `TFO_MOMO_*` | MoMo credentials |
| `ZALOPAY_*` | ZaloPay credentials |
| `VNP_*` / `VPN_*` | VNPay Merchant QR credentials |
| `SHOPEEPAY_*` / `TFO_SHOPEEPAY_*` | ShopeePay credentials + `SHOPEEPAY_CALLBACK_IP_WHITELIST` |

Config load **fails** if:

- `APP_ENV=production` + `PAYMENT_ENV=live` + placeholder `stripe` or sandbox family keys
- `APP_ENV=production` + `PAYMENT_ENV=live` + live key **without** outbound credentials (unwired)
- `APP_ENV=production` + `PAYMENT_ENV=cash_only` + `COMMERCE_PAYMENT_PROVIDER` / `COMMERCE_PAYMENT_PROVIDERS` set
- Production + legacy payment HTTP without `LEGACY_PAYMENT_HTTP_ALLOW_IN_PRODUCTION=true`

## Provider registry

| Key family | Type | Outbound `CreatePaymentSession` | Production card/QR |
|------------|------|----------------------------------|--------------------|
| `mock`, `sandbox`, … | Sandbox | Yes | **Forbidden** in production |
| `cash` | Cash ledger | No (`ConfirmCashPayment`) | Cash only |
| `stripe` | Placeholder | No | **Forbidden** as live default |
| `momo`, `zalopay`, `vietqr`, `vnpay`, `shopeepay` | WiredLiveProvider | Yes when credentials present | Allowed when `PAYMENT_ENV=live` |

## Payment success notification

1. PSP IPN → `ApplyPaymentProviderWebhook` (`captured`)
2. MQTT command `payment.captured` to machine (best-effort; never fails IPN)
3. App **must** confirm via `GetOrderStatus` / legacy `/query` before vend
4. `GetOrderStatus` may also refresh pending payments via provider query (ZaloPay-style)

Ops: `GET /version` → `payment_runtime.enabled_providers`; `GET /v1/admin/payment/providers`.

## Enabling live QR/card checklist

1. Set PSP credentials for chosen keys
2. `PAYMENT_ENV=live` + `COMMERCE_PAYMENT_PROVIDER` and/or `COMMERCE_PAYMENT_PROVIDERS`
3. Register IPN URLs on PSP portals (canonical or legacy alias on new domain)
4. Verify `/version.payment_runtime.card_qr_sessions_available=true`
5. Field QA: create → IPN/MQTT → poll → vend; legacy create/query if used

## Related docs

- [Payment webhooks](../api/payment.md)
- [MQTT contract](../api/mqtt-contract.md) — `payment.captured` command
- [Machine gRPC](../api/machine-grpc.md)
