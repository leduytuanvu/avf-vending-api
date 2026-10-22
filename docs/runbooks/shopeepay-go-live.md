# ShopeePay go-live (avf-vending-api)

Native IPN: `https://api.ldtv.dev/v1/commerce/webhooks/shopeepay`  
Legacy alias (only if `ENABLE_LEGACY_PAYMENT_HTTP=true`): `/payment-service/payment/shopeepay/callback`

Do not commit `SHOPEEPAY_SECRET_KEY`. Copy values from the old secret store (`avf-backend-prod`).

## 1. ShopeePay portal

Set merchant notify URL to `https://api.ldtv.dev/v1/commerce/webhooks/shopeepay` (AVF and TFO tenants).

## 2. App-node env (VPS A and B)

Required when `COMMERCE_PAYMENT_PROVIDERS` includes `shopeepay`:

```
PAYMENT_ENV=live
COMMERCE_PAYMENT_PROVIDERS=momo,zalopay,vietqr,shopeepay
SHOPEEPAY_END_POINT=https://api.wallet.airpay.vn
SHOPEEPAY_CLIENT_ID=
SHOPEEPAY_SECRET_KEY=
SHOPEEPAY_MERCHANT_EXT_ID=
SHOPEEPAY_STORE_EXT_ID=AVF000079
SHOPEEPAY_CALLBACK_URL=https://api.ldtv.dev/v1/commerce/webhooks/shopeepay
SHOPEEPAY_CALLBACK_IP_WHITELIST=103.78.79.131,103.78.79.132,103.78.79.218,103.78.79.223,125.212.198.129,125.212.198.130,203.116.161.65,203.116.161.66,203.116.124.9,203.162.56.17,143.92.76.1,143.92.76.2,143.92.76.35,147.136.138.163,147.136.138.164
SHOPEEPAY_TRUST_PROXY=true
```

TFO machines need `TFO_SHOPEEPAY_*` including `TFO_SHOPEEPAY_CALLBACK_URL`. Production boot fails if `shopeepay` is allowlisted without credentials or callback URL.

`store_ext_id` at runtime is the machine code (`AVF*` / `TFO*`). Create QR is rejected unless the code is in the embedded SPP allowlist (or `SHOPEEPAY_STORE_EXT_ID_ALLOWLIST` / `_FILE`).

Then recreate `api`, `worker`, and `reconciler`.

## 3. Enable methods (allowlisted machines only)

On an app-node with `DATABASE_URL` or a running API container:

```bash
DRY_RUN=1 bash scripts/ops/enable_shopeepay_allowlist.sh
bash scripts/ops/enable_shopeepay_allowlist.sh
```

Do not enable ShopeePay on machines missing from `deployments/prod/app-node/shopeepay/store_ext_id_allowlist.txt`.

## 4. Kiosk APK

Install a build that un-hides ShopeePay (`PRODUCTION_QR_PROVIDER_ORDER`, visibility, `StorefrontSellability`). Bootstrap refresh after methods are enabled.

Kiosk shows ShopeePay only when: env wired + method enabled on that machine + new APK.

## 5. Smoke (one allowlisted machine)

Suggested: `AVF000079` or `AVF000195` if present in the allowlist.

1. `GET https://api.ldtv.dev/version` → `payment_mode=live_psp`.
2. Start a ShopeePay QR session on the kiosk (`CREATE_PAYMENT_SESSION_PHASES`).
3. Scan / pay. IPN log: `PSP_WEBHOOK_RECEIVED` provider=shopeepay, HTTP 200 `{"errcode":0}`.
4. Kiosk `GetPaymentStatus` → captured → vend.

Failed signature → 401; forbidden IP → 403; unknown order after valid HMAC → 503 (SPP retry).
