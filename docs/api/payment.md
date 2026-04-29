# Payments (admin finance & PSP evidence)

Org-scoped routes require a Bearer admin JWT. Tenant users may only access their own `organizationId`; platform admins may access any org id in the path.

## Machine gRPC vs HTTP payment sessions

- **Machine gRPC** (`CreatePaymentSession`): Backend-owned PSP session. The device must not supply trusted **`provider_reference`**, **`payment_url`**, **`checkout_url`**, **`qr_payload`**, or provider session identifiers — the registry adapter generates those after the **`payments`** row exists. The response exposes **`qr_payload_or_url`** for display; webhook confirmation remains on **REST** with HMAC (`/v1/commerce/orders/{orderId}/payments/{paymentId}/webhooks`).
- **HTTP** `POST /v1/commerce/orders/{orderId}/payment-session`: Documented kiosk contract may still include optional `outbox_payload_json` for fan-out; treat any client-supplied PSP hints as **non-authoritative** until superseded by a future hardening pass.

## Kiosk payment session (`POST /v1/commerce/orders/{orderId}/payment-session`)

Machine/kiosk callers create or replay a PSP payment plus durable outbox row. Successful responses include a **stable contract** for the vending app:

- **`sale_id`**: order id (same as URL `orderId`).
- **`session_id`**: primary `vend_sessions.id` for checkout (see `GET /v1/commerce/orders/{orderId}` with `slot_index` query; payment-session uses the same default `slot_index=0`).
- **`payment_id`**, **`amount_minor`**, **`currency`**, **`provider`**
- **`status`**: kiosk-oriented lifecycle (`pending`, `authorized`, `paid`, …) derived from `payment_state`
- **`payment_state`**: authoritative DB `payments.state`
- **`qr_url`**, **`payment_url`**, **`checkout_url`**: optional PSP hints parsed from JSON `outbox_payload_json` (`qr_url`, `payment_url`, `checkout_url`, camelCase aliases)
- **`expires_at`**: optional RFC3339 from `expires_at`, `expiration_time`, `session_expires_at`, unix numeric, or relative `expires_in` (seconds) / Go duration string
- **`idempotency_key`** / **`request_id`**: both echo the **`Idempotency-Key`** header value used for the write scope
- **`outbox_event_id`**, **`replay`**

Webhook capture transitions the payment/order to paid; **payment success does not auto-complete vending** (`vend_sessions.state` advances only via explicit vend machine flows).

## Raw webhook storage (`payment_provider_events`)

Public machine/webhook ingress persists rows when HMAC policy allows (`/v1/commerce/orders/{orderId}/payments/{paymentId}/webhooks`). Stored fields include:

- `provider`, `provider_ref`, `webhook_event_id` (idempotency keys)
- `received_at`, `validation_status`, `signature_valid`
- `payload` (JSON redacted via `compliance.SanitizeJSONBytes` at insert)
- `applied_at`, `ingress_status`, `ingress_error` (processing outcome)
- `organization_id` (denormalized from the order for admin filtering)

Duplicate `webhook_event_id` or `provider_ref` replays do not double-apply payment state (same transaction semantics as before).

## Provider settlements (`payment_provider_settlements`)

Imported PSP settlement reports are keyed by `(organization_id, provider, provider_settlement_id)`. Reconciliation compares **gross_amount_minor** to the sum of internal **payments** referenced by `transaction_refs` (matched via `payment_attempts.provider_reference`).

- **Matched** imports are marked `reconciled`.
- **Mismatch** opens a `commerce_reconciliation_cases` row with `case_type = settlement_amount_mismatch` and `correlation_key = settlement:{provider}:{provider_settlement_id}`.

## Disputes (`payment_disputes`)

Foundation table for chargebacks/disputes with optional links to `payments` and `orders`. Terminal resolutions: `won`, `lost`, `closed` (HTTP accepts those statuses and updates `resolved_at` / `resolution_note`).

## Admin HTTP

| Method | Path | Permission | Notes |
|--------|------|------------|--------|
| GET | `/v1/admin/organizations/{organizationId}/payments/reconciliation` | `commerce:read` or `payment:read` | Query: **`stale_after_seconds`** _or_ **`stale_hours`** (lookahead for paid-not-completed; default stale window applies when omitted), **`limit`** (caps at 500). JSON: stale paid orders stuck in **`paid`**/**`vending`**, PSP payload capture vs **`created`/`authorized`** payment rows, **`captured`** rows without webhook evidence (`payment_provider_events` HMAC/`unsigned_development`), and **applied** webhook rows whose **`provider_amount_minor` / currency** disagree with the **`payments`** ledger (drift / manual repair triage; **`cash`** excluded). |
| GET | `/v1/admin/organizations/{organizationId}/payments/webhook-events` | `commerce:read` or `payment:read` | Pagination: `limit`, `offset` |
| GET | `/v1/admin/organizations/{organizationId}/payments/settlements` | same | |
| POST | `/v1/admin/organizations/{organizationId}/payments/settlements/import` | `payment:refund` | JSON body: `provider`, `settlements[]` |
| GET | `/v1/admin/organizations/{organizationId}/payments/disputes` | same as GET lists | |
| POST | `/v1/admin/organizations/{organizationId}/payments/disputes/{disputeId}/resolve` | `payment:refund` | JSON: `status`, `note`; writes enterprise audit |
| GET | `/v1/admin/organizations/{organizationId}/payments/export` | same | Query: `from`, `to` (RFC3339); CSV download |

Settlement import and dispute resolution emit **critical** audit events (`payment.settlement.imported`, `payment.dispute.resolved`) when enterprise audit is configured.

## Related docs

- [`payment-webhook-security.md`](payment-webhook-security.md)
- [`../runbooks/payment-reconciliation.md`](../runbooks/payment-reconciliation.md)
