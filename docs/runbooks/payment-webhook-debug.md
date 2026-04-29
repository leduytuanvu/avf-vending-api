# Payment webhook debugging (REST, HMAC, idempotency)

Payment provider callbacks hit the **public REST** webhook route (not gRPC). Verification is **HMAC**-based with timestamp skew and **idempotent** processing on the server.

## Quick checks

1. **Config** — In staging/production, a webhook HMAC secret must be configured (`COMMERCE_PAYMENT_WEBHOOK_SECRET`, `PAYMENT_WEBHOOK_SECRET`, or `COMMERCE_PAYMENT_WEBHOOK_HMAC_SECRET`, or per-provider `COMMERCE_PAYMENT_WEBHOOK_SECRETS_JSON`). Unsigned / unsafe modes are rejected in those environments.
2. **Headers** — Requests must include `X-AVF-Webhook-Timestamp` and `X-AVF-Webhook-Signature` (HMAC-SHA256 over `{timestamp}.{rawBody}`). Skew is bounded by `COMMERCE_PAYMENT_WEBHOOK_REPLAY_WINDOW` (or legacy alias).
3. **Idempotency** — Retries with the same logical event should not double-apply business effects; investigate duplicate delivery via logs and commerce audit/outbox rows rather than disabling verification.

## Metrics

- `avf_commerce_payment_webhook_requests_total{result="ok|error|..."}` — aggregate success vs failure (exact label values as implemented in `internal/httpserver`).

Correlate with HTTP access logs using **`request_id`** / **`trace_id`** (never log signature or body secrets).

## Common failures

| Symptom | Likely cause |
|---------|----------------|
| 401/403 on signature | Clock skew, wrong secret, wrong body canonicalization (middleware reads raw body). |
| 400 validation | Malformed JSON or missing required payment/order fields. |
| 400 / reconciliation case | **`provider_amount_minor`** or **`currency`** in the webhook body disagrees with the persisted **`payments`** row (`webhook_amount_currency_mismatch`). |
| 5xx after auth | Downstream DB or commerce service; check readiness and Postgres, not webhook crypto. |

## Field smoke

Local/staging field smoke can exercise HMAC verification and idempotent replay when a webhook secret is configured:

Git Bash:

```bash
export BASE_URL="http://localhost:8080"
export COMMERCE_PAYMENT_WEBHOOK_SECRET="dev-secret"
bash scripts/smoke/local_field_smoke.sh --evidence-json smoke-reports/payment-webhook-smoke.json
```

PowerShell:

```powershell
$env:BASE_URL = "http://localhost:8080"
$env:COMMERCE_PAYMENT_WEBHOOK_SECRET = "dev-secret"
.\scripts\smoke\local_field_smoke.ps1 -EvidenceJson smoke-reports/payment-webhook-smoke.json
```

The smoke posts the same signed webhook event twice. Both requests must return 200 and the server must not double-apply the event.

## Safe operations

- Rotate secrets using provider-specific keys in `COMMERCE_PAYMENT_WEBHOOK_SECRETS_JSON` where supported.
- Use staging with **sandbox** payment env; production requires **live** `PAYMENT_ENV` per config rules.

## Related

- [production-readiness.md](./production-readiness.md) — webhook config gates.
- OpenAPI: webhook routes under commerce paths in `docs/swagger` / `/swagger/doc.json`.

## Prometheus signals (canonical)

- `payment_webhooks_total{result=…}` — per-outcome webhook handling.
- `payment_webhook_rejections_total{reason=…}` — HMAC, validation, ordering, replay conflict (subset of rejects).
- See [`docs/observability/production-metrics.md`](../observability/production-metrics.md).
