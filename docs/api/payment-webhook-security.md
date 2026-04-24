# Payment provider webhook security (commerce)

Route: `POST /v1/commerce/orders/{orderId}/payments/{paymentId}/webhooks`

## Verification (AVF HMAC)

The API implements **one** verification profile today: **`COMMERCE_PAYMENT_WEBHOOK_VERIFICATION=avf_hmac`** (default). Other values fail at process startup.

When **`COMMERCE_PAYMENT_WEBHOOK_HMAC_SECRET`** (or legacy `PAYMENT_WEBHOOK_SECRET`) is set:

- **`X-AVF-Webhook-Timestamp`**: Unix seconds.
- **`X-AVF-Webhook-Signature`**: hex-encoded HMAC-SHA256 over `{timestamp}.{rawBody}` (bytes of the timestamp decimal string, ASCII `.`, then the **exact** request body). Optional `sha256=` prefix on the signature.

Skew between timestamp and server time must be within **`COMMERCE_PAYMENT_WEBHOOK_TIMESTAMP_SKEW_SECONDS`** (default **300**; validated between 30 and 86400 seconds at config load). Outside that window the API returns **400** `webhook_timestamp_skew` (invalid or stale timestamp). Invalid HMAC still returns **401** `webhook_auth_failed`.

Never log the shared secret, raw signature material, or full webhook payloads in production unless redacted.

## Provider binding

The JSON **`provider`** must match the **`payments.provider`** row for **`paymentId`** (case-insensitive). A mismatch returns **403** `webhook_provider_mismatch` so one PSP cannot advance another provider’s payment by guessing IDs.

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

## Further reading

- OpenAPI: `DocOpV1CommercePaymentWebhook` in `internal/httpserver/swagger_operations.go` (regenerate with `make swagger`).
- Runbook: `docs/runbooks/api-surface-security.md` (commerce webhook section).
