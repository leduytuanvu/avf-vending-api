# Audit trail operations

## When audit write fails (production)

- **Critical mutations** wired with `RecordCritical` / `RecordCriticalTx` will **fail the request** (or roll back the transaction) if the audit row cannot be inserted, except where `AUDIT_CRITICAL_FAIL_OPEN` is explicitly enabled in non-production environments for pool-level `RecordCritical` only.
- Symptoms: HTTP **500** with `audit_failed` on payment webhook paths, or transaction errors on activation claim/refresh if the database rejects `audit_events` inserts.

## Verification

1. Confirm migrations applied: `audit_events` has `machine_id`, `site_id`, and `actor_type` allows `payment_provider`.
2. List recent events: `GET /v1/admin/organizations/{organizationId}/audit-events?limit=20` (with an operator JWT that has `audit.read`).
3. Filter by machine: `?machineId={uuid}`.
4. Spot-check payment webhook rejections: `action` = `payment.webhook.rejected`, `actorType` = `payment_provider`.

## Safety

- Never log raw `before_json` / `after_json` / `metadata` from support tools into public tickets.
- Do not disable RBAC or org scoping on audit list/get endpoints for debugging.
