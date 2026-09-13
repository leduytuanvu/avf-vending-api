# Runbook: Bật thanh toán online thật trên production

## Trạng thái đã kiểm tra

`GET https://api.ldtv.dev/version` trước cutover thường là `payment_mode=cash_only` khi image chưa có WiredLiveProvider.

## Điều kiện GO

1. Image API đã đăng ký WiredLiveProvider (MoMo / ZaloPay / VietQR / VNPay / ShopeePay).
2. VPS `.env.app-node` có credentials: `MOMO_*`, `ZALOPAY_*`, `VNP_*`, `SHOPEEPAY_*`.
3. Deploy dùng `apply_live_payment_app_node_env.sh` với allowlist `momo,zalopay,vietqr,shopeepay`.
4. APK máy mở MoMo/Zalo/VietQR/VNPay và không khóa Pay chỉ CASH.

## Verify

```bash
curl -sS https://api.ldtv.dev/version | jq .payment_runtime
```

Kỳ vọng: `payment_mode=live_psp`, `card_qr_sessions_available=true`.

Revert: `apply_cash_only_payment_app_node_env.sh` + restart `api`.

## Recover-deploy interaction

The workflow [production-sshd-recover-and-deploy.yml](../../.github/workflows/production-sshd-recover-and-deploy.yml) runs **`apply_cash_only_payment_app_node_env.sh`** before a full recover rollout. That step forces `PAYMENT_ENV=cash_only` on the app node and will hide QR on devices until live/sandbox payment env is re-applied.

After any recover deploy that needs QR:

1. Re-run `apply_live_payment_app_node_env.sh` with the intended provider allowlist.
2. Verify `GET /version` → `payment_mode=live_psp` (or sandbox equivalent).
3. Re-enable machine QR payment methods in admin if needed.
4. On device: bootstrap refresh; confirm `PAYMENT_QR_UI_GATE deploymentSupported=true`.
