# Production payment provider status

This document is the operator and Android handoff reference for **which payment methods are safe to expose** in production.

## Chosen path: **Path B — explicit cash-only production pilot**

No live PSP (`stripe`, `momo`, `zalopay`, `vnpay`) is wired for outbound `CreatePaymentSession` in this release. Production **must** run `PAYMENT_ENV=cash_only` until Path A (real PSP) is implemented, registered as `WiredLiveProvider`, and field-tested.

**Android rule:** show cash when payment_methods.cash_enabled=true. Show QR/card only when payment_methods.qr_card_enabled=true.

**Recommended live allowlist (examples + apply_live_payment_app_node_env.sh):**

`
PAYMENT_ENV=live
COMMERCE_PAYMENT_PROVIDER=momo
COMMERCE_PAYMENT_PROVIDERS=momo,zalopay,vietqr,shopeepay
`

Fill MOMO_* / ZALOPAY_* / VNP_* / SHOPEEPAY_* from the secret store. To revert to cash pilot, use pply_cash_only_payment_app_node_env.sh.


## Current production posture (Path B — explicit cash-only)

AVF production pilot runs with **cash-only** vending unless a **wired live PSP** is registered and validated.

| Setting | Cash-only pilot (current) | Live QR/card (future) |
|---------|---------------------------|------------------------|
| `PAYMENT_ENV` | `cash_only` | `live` |
| `COMMERCE_PAYMENT_PROVIDER` | **unset** | Wired adapter key (not a placeholder) |
| QR/card in Android | **Hidden** (`qr_card_enabled=false`) | Shown when `qr_card_enabled=true` |
| Cash in Android | **Shown** when `cash_enabled=true` | Per machine feature flag |

Config load **fails** if:

- `APP_ENV=production` + `PAYMENT_ENV=live` + placeholder key (`stripe`, `momo`, `zalopay`, `vnpay`)
- `APP_ENV=production` + `PAYMENT_ENV=cash_only` + `COMMERCE_PAYMENT_PROVIDER` is set

## Provider registry

| Key family | Type | Outbound `CreatePaymentSession` | Production card/QR |
|------------|------|----------------------------------|--------------------|
| `mock`, `sandbox`, `test`, `psp_fixture`, `dev`, `psp_grpc_int` | Sandbox | Yes (deterministic) | **Forbidden** |
| `cash` | Cash ledger | No (use `ConfirmCashPayment`) | Cash checkout only |
| `stripe`, `momo`, `zalopay`, `vnpay` | Placeholder shell | **No** (`provider_unavailable`) | **Forbidden** at config load when `PAYMENT_ENV=live` |
| Custom registered `WiredLiveProvider` | Live adapter | Yes | Allowed when `PAYMENT_ENV=live` |

Placeholders verify inbound webhooks (HMAC) but **must not** be used for paid QR/card sales.

## Android runtime signals

### Bootstrap (`GetBootstrap`)

`GetBootstrapResponse.payment_methods`:

| Field | Meaning |
|-------|---------|
| `cash_enabled` | Show cash checkout |
| `qr_card_enabled` | Show QR/card checkout |
| `payment_mode` | `cash_only` \| `live_psp` \| `sandbox` |
| `card_qr_provider_key` | Registry key when enabled; empty when disabled |
| `card_qr_provider_status` | `unavailable` \| `wired` \| `sandbox` \| `placeholder` |
| `qr_card_unavailable_reason` | Stable reason code, e.g. `provider_unavailable` |

**Android rule:** hide QR/card UI unless `qr_card_enabled == true`.

### Commerce gRPC

| RPC | When QR/card disabled |
|-----|------------------------|
| `CreatePaymentSession` | `FailedPrecondition` / `provider_unavailable` |
| `ConfirmCashPayment` | `FailedPrecondition` / `cash_payment_disabled` when `cash_enabled=false` |

### Ops visibility

| Endpoint | Field |
|----------|-------|
| `GET /version` | `payment_runtime` object |
| `GET /v1/admin/payment/providers` | `wired`, `session_available`, `provider_status` per key |

## Machine-local overrides

Feature flags on the machine (via `RuntimeHints.feature_flags`):

| Flag | Default | Effect |
|------|---------|--------|
| `commerce.cash_enabled` | `true` in cash-only | Set `false` to disable cash on a machine |
| `commerce.qr_card_enabled` | follows deployment | Cannot enable QR/card when deployment has no wired PSP |

## Enabling live QR/card (Path A checklist)

1. Implement `WiredLiveProvider` for the pilot PSP (`CreatePaymentSession`, webhook verify, cancel/refund/reconcile as required).
2. `Register` the adapter in `NewRegistry` or bootstrap wiring (replacing the placeholder entry).
3. Set `PAYMENT_ENV=live` and `COMMERCE_PAYMENT_PROVIDER=<wired-key>`.
4. Verify config load, `/version.payment_runtime.card_qr_sessions_available=true`, bootstrap `qr_card_enabled=true`.
5. Run commerce integration tests and field QA before fleet rollout.

## Related docs

- [Machine gRPC production contract](../api/machine-grpc-production-contract.md)
- [Payment webhooks](../api/payment.md)
- [Backend production app contract audit](../archive/audits/audit/BACKEND_PRODUCTION_APP_CONTRACT_AUDIT.md)

## Contract tests

| Test area | Location |
|-----------|----------|
| Cash-only capability flags | `internal/platform/payments/production_payment_safety_test.go` |
| Placeholder session blocked | `internal/platform/payments/production_payment_safety_test.go`, `registry_resolve_test.go` |
| Production config rejects placeholders | `internal/config/deployment_env_test.go` |
| gRPC `CreatePaymentSession` → `provider_unavailable` | `internal/grpcserver/machine_commerce_cash_only_test.go` |
| Bootstrap `payment_methods` | `internal/grpcserver/machine_commerce_cash_only_test.go`, `machine_payment_runtime_test.go` |
| `/version.payment_runtime` | `internal/observability/version_payment_test.go` |

Run:

```bash
go test ./internal/platform/payments/...
go test ./internal/app/commerce/...
go test ./internal/config/... -run Production
go test ./internal/grpcserver/... -run CashOnly
go test ./...
```
