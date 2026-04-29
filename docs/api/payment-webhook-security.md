# Payment provider webhook security (commerce)

Route: `POST /v1/commerce/orders/{orderId}/payments/{paymentId}/webhooks`

## Verification (AVF HMAC)

The API implements **one** verification profile today: **`COMMERCE_PAYMENT_WEBHOOK_VERIFICATION=avf_hmac`** (default). Other values fail at process startup.

When **`COMMERCE_PAYMENT_WEBHOOK_HMAC_SECRET`** (or legacy `PAYMENT_WEBHOOK_SECRET`) is set:

- **`X-AVF-Webhook-Timestamp`**: Unix seconds.
- **`X-AVF-Webhook-Signature`**: hex-encoded HMAC-SHA256 over `{timestamp}.{rawBody}` (bytes of the timestamp decimal string, ASCII `.`, then the **exact** request body). Optional `sha256=` prefix on the signature.

Skew between timestamp and server time must be within **`COMMERCE_PAYMENT_WEBHOOK_TIMESTAMP_SKEW_SECONDS`** (default **300**; validated between 30 and 86400 seconds at config load). Outside that window the API returns **400** `webhook_timestamp_skew` (invalid or stale timestamp). Invalid HMAC still returns **401** `webhook_auth_failed`.

Missing **`X-AVF-Webhook-Timestamp`** or **`X-AVF-Webhook-Signature`** triggers the same **`VerifyCommerceWebhookHMAC`** auth failure (**401**, `webhook_auth_failed`): both headers must be present. Any mutation of the serialized JSON body after signing also fails verification (**401**).



## Provider binding

The JSON **`provider`** must match the **`payments.provider`** row for **`paymentId`** (case-insensitive). A mismatch returns **403** `webhook_provider_mismatch` so one PSP cannot advance another provider’s payment by guessing IDs.

When **`provider_amount_minor`** or **`currency`** are provided on the webhook body, they must match **`payments.amount_minor`** / **`payments.currency`** or the API returns **409** `webhook_amount_currency_mismatch` and opens/refreshes that reconciliation case.

When **`payments.state`** or **`orders.status`** are already terminal for the attempted transition (for example a **`captured`** webhook after **`payments.state=refunded`**), the API returns **409** `webhook_after_terminal_order` and opens/refreshes the **`webhook_after_terminal_order`** case type.

## Secret rotation

Rotate **`COMMERCE_PAYMENT_WEBHOOK_HMAC_SECRET`** by updating the environment / secret manager and redeploying. Coordinate with the PSP to send signatures with the new secret; keep overlap only as long as the PSP supports dual verification. Old events remain replay-idempotent on stored keys.

## Production vs unsigned callbacks

| `APP_ENV`   | Empty secret | Behavior |
|-------------|--------------|----------|
| production  | default      | **403** `webhook_hmac_required` — unsigned webhooks are rejected. |
| production  | `COMMERCE_PAYMENT_WEBHOOK_UNSAFE_ALLOW_UNSIGNED_PRODUCTION=true` | HMAC skipped (**unsafe**; document exception only when no alternative exists). |
| non-production | default   | **503** `capability_not_configured` unless secret is set. |
| non-production | `COMMERCE_PAYMENT_WEBHOOK_ALLOW_UNSIGNED=true` | HMAC skipped (local / CI only). |

`COMMERCE_PAYMENT_WEBHOOK_ALLOW_UNSIGNED=true` is **rejected** when `APP_ENV=production` (use the unsafe production flag instead).

## Idempotency and replay

Persistence enforces:

- **(`provider`, `provider_reference`)** — unique when `provider_reference` is non-empty (existing behavior).
- **(`provider`, `webhook_event_id`)** — unique when optional JSON field **`webhook_event_id`** is present (migration `00022_payment_provider_webhook_event_id.sql`).

Retries that match the same keys return **200** with **`replay: true`**. Conflicting reuse of **`webhook_event_id`** with a different **`provider_reference`** (or vice versa, when both sides are present and disagree) returns **409** `webhook_idempotency_conflict`.

Duplicate deliveries are also surfaced operationally: replayed provider events create or refresh an open `duplicate_provider_event` row in the admin commerce reconciliation queue. Provider mismatches are rejected, audited as `payment.webhook.rejected`, and create or refresh `webhook_provider_mismatch` review cases.

Admin queue (reads require **`commerce:read`**; mutations require **`refunds:write`**):

- `GET /v1/admin/organizations/{organizationId}/commerce/reconciliation` — filter **`case_type`** via query param **`case_type`**.
- `GET /v1/admin/organizations/{organizationId}/commerce/reconciliation/{caseId}`
- `POST /v1/admin/organizations/{organizationId}/commerce/reconciliation/{caseId}/resolve` — terminal statuses include **`resolved`**, **`dismissed`**, **`ignored`**, **`escalated`**.
- `POST /v1/admin/organizations/{organizationId}/commerce/reconciliation/{caseId}/request-refund` — enqueues a **`refunds`** row via **`CreateRefund`** (idempotency `reconciliation_case_refund:{caseId}`); audit **`commerce.reconciliation.refund_requested`**.

## Further reading

- OpenAPI: `DocOpV1CommercePaymentWebhook` in `internal/httpserver/swagger_operations.go` (regenerate with `make swagger`).
- Runbook: `docs/runbooks/api-surface-security.md` (commerce webhook section).
