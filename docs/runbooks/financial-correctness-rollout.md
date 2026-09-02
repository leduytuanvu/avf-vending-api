# Financial correctness rollout

Staged rollout for winner arbitration, server-side cash evidence, and admin money view.

## Stage 1 — Schema only

Deploy migration `00026_financial_correctness.sql`. No behaviour change.

## Stage 2 — API (dual-read)

- Deploy API with `COMMERCE_WINNER_ARBITRATION_ENABLED=false` in production initially.
- Legacy cash confirms tolerated (`consent_source=unknown` + reconciliation case).
- New proto fields accepted on `ConfirmCashPayment` and `CreatePaymentSession`.

## Stage 3 — Enable arbitration

Set `COMMERCE_WINNER_ARBITRATION_ENABLED=true` after divergence logs are clean.

## Stage 4 — Device APK

Roll out storefront build with:

- Cash wallet consent gate (`explicit_confirm`)
- Attempt-sequenced QR idempotency keys + server cancel
- Durable payment-screen cash credits (Room + `rawRecordHex` dedup)
- Full cash evidence on `ConfirmCashPayment`

## Stage 5 — Web admin

Deploy web with order money view (`GET /v1/admin/orders/{orderId}/money`) and `GET /v1/payments/{paymentId}`.

## Stage 6 — Reconciler alerting

Enable `cmd/reconciler` financial correctness tick (unclaimed captures, gross mismatch, change liability). Wire alerts on `commerce_reconciliation_cases_total` for new case types.

## Rollback

- Arbitration: flip `COMMERCE_WINNER_ARBITRATION_ENABLED` off.
- Migrations are additive; older binaries continue to run.
