# Payment / vend reconciliation runbook

## Data model

- **Canonical queue:** `commerce_reconciliation_cases` (operator workflow). Compatibility view **`payment_reconciliation_cases`** exposes `created_at` / `updated_at` aliases (`first_detected_at` / `last_detected_at`).
- **Order timeline:** `order_timelines` — append-only events (`commerce.reconciliation.case_resolved`, `commerce.refund.requested`, …).
- **Refund review:** `refund_requests` — durable rows for admin-initiated refunds, linked to ledger `refunds.refunds` via `refund_id` after `CreateRefund`.

## Admin APIs (`/v1/admin/organizations/{organizationId}/…`)

| Endpoint | Purpose |
|----------|---------|
| `GET …/commerce/reconciliation` | List cases |
| `GET …/commerce/reconciliation/{caseId}` | Case detail |
| `POST …/commerce/reconciliation/{caseId}/resolve` | Resolve / dismiss / escalate (JSON `status`) |
| `POST …/commerce/reconciliation/{caseId}/ignore` | Convenience **ignored** resolution |
| `POST …/commerce/reconciliation/{caseId}/request-refund` | Refund from case (ledger + `refund_requests`) |
| `GET …/orders/{orderId}/timeline` | Order timeline |
| `GET …/refunds`, `GET …/refunds/{refundId}` | Refund request registry |
| `POST …/orders/{orderId}/refunds` | Org-scoped refund + durable row (**Idempotency-Key** required) |
| `GET …/payments/webhook-events` | Raw PSP webhook / ingress audit rows (P1.2) |
| `GET …/payments/reconciliation` | Operational drift (stale **`paid`**/**`vending`**, PSP **`captured`** vs local **`created`/`authorized`**, **`captured`** lacking ingress evidence, **applied webhook amount/currency** vs **`payments`** row — **`cash`** excluded). **Read-only SELECT probes** — no automatic correction of `payments` / orders. |

### Operator-only listing (no API)

For break-glass DB access, `GET …/payments/reconciliation` remains the supported contract. The **`ListPaymentReconciliationDrift`** service method issues **SELECTs only** (no `UPDATE` on financial tables). Settlement import and case resolution are **separate explicit** admin actions.
| `GET …/payments/settlements` | Imported provider settlement reports |
| `POST …/payments/settlements/import` | Idempotent settlement upsert + amount reconciliation (opens **`settlement_amount_mismatch`** cases on mismatch) |
| `GET …/payments/disputes` | Dispute / chargeback foundation rows |
| `POST …/payments/disputes/{disputeId}/resolve` | Terminal dispute resolution (`won` / `lost` / `closed`) + audit |
| `GET …/payments/export` | CSV finance export for payments in org + `from`/`to` window |

### Settlement mismatch cases

- New `commerce_reconciliation_cases.case_type`: **`settlement_amount_mismatch`**.
- Deduped while open via **`correlation_key`** = `settlement:{provider}:{provider_settlement_id}` (see migration `00069_p12_payment_settlement_disputes_export.sql`).
- Uses the same operator queue UX as `GET …/commerce/reconciliation`.

## Operations notes

- Resolution updates reconciliation status in Postgres **and** appends an **order timeline** row when `order_id` is present (same transaction).
- Background reconciler continues to open/update cases; concurrent upserts rely on the partial unique index on open/reviewing/escalated rows (see migrations).
- Webhook mismatch / duplicate detection paths remain in [`internal/httpserver/commerce_webhook_public.go`](../../internal/httpserver/commerce_webhook_public.go) and commerce apply logic.

See also: [`payment.md` summary (P1.2 admin routes)](../api/payment.md), [`payment-webhook-security.md`](../api/payment-webhook-security.md), [`commerce-refund-cancel-implementation-handoff.md`](../api/commerce-refund-cancel-implementation-handoff.md).

## Prometheus signals (canonical)

- **`reconciliation_cases_open_total`** — gauge refreshed by the reconciler (open/reviewing/escalated counts).
- **`refunds_requested_total{channel=…}`**, **`refunds_failed_total{reason=…}`** — refund workflow volume.
- **`payment_webhooks_total`** / **`payment_webhook_rejections_total`** — webhook-side mismatches vs accepted traffic.

[`docs/observability/production-metrics.md`](../observability/production-metrics.md).
