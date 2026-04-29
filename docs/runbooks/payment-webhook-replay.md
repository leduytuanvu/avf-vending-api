# Payment webhook replay and signature incidents

Use when alerts fire on **`payment_webhook_rejections_total`**, **`payment_webhook_amount_currency_mismatch_total`**, or PSP delivery looks replayed or malicious.

## What the alert means

| Alert | Signal |
| ----- | ------ |
| **AVFPaymentWebhookHMACRejectionsSpike** | Broad HMAC / 401-ish **`reason`** labels on rejections |
| **AVFPaymentWebhookInvalidSignatureBurst** | **`reason="webhook_hmac_invalid"`** — signature bytes do not verify |
| **AVFPaymentWebhookTimestampSkewBurst** | **`reason="webhook_timestamp_skew"`** — `X-AVF-Webhook-Timestamp` outside allowed skew |
| **AVFPaymentWebhookAmountCurrencyMismatch** | Canonical counter **`payment_webhook_amount_currency_mismatch_total`** — body disagrees with trusted DB row |

**Thresholds:** See `description` on each rule in `deployments/prod/observability/prometheus/alerts.yml`.

## How to query

**Metrics (API job `avf_api_metrics`):**

```promql
sum(rate(payment_webhook_rejections_total[5m])) by (reason)
sum(rate(payment_webhook_amount_currency_mismatch_total[30m]))
sum(rate(avf_commerce_payment_webhook_requests_total[5m])) by (result)
```

**Logs:** Filter HTTP logs on route for payment webhook; join with **`correlation_id`** / **`request_id`** from response middleware.

**DB:** `commerce_reconciliation_cases` for `webhook_amount_currency_mismatch`, `webhook_provider_mismatch`, duplicate provider events.

## Immediate mitigation

1. **Invalid signature burst:** Confirm `COMMERCE_PAYMENT_WEBHOOK_SECRET` / per-provider JSON secrets match PSP console; ensure edge does not strip `X-AVF-Webhook-Signature` or `X-AVF-Webhook-Timestamp`.
2. **Timestamp skew:** NTP on API nodes; widen skew only temporarily and document risk.
3. **Amount mismatch:** Do not replay client-trusted amounts; verify kiosk `CreatePaymentSession` vs webhook payload; resolve via reconciliation UI — see **`docs/runbooks/payment-reconciliation.md`**.

## Safe manual recovery

- Re-open PSP dashboard to confirm event idempotency; use admin APIs to dismiss or resolve cases after root cause is fixed.
- For duplicate PSP deliveries, rely on idempotent webhook apply — **do not** double-post manual captures.

## Escalation

- P0 sustained signature + capture anomalies: page security + commerce on-call.
- Provider-wide outage: incident commander per **`docs/runbooks/production-day-2-incidents.md`**.

## Invalid signature spike

See **AVFPaymentWebhookInvalidSignatureBurst** — treat as possible secret rotation miss or attack; rotate secrets only after confirming legitimate traffic pattern.

## Timestamp skew

See **AVFPaymentWebhookTimestampSkewBurst** — correlate with provider support if clocks cannot be fixed client-side.
