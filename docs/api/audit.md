# Enterprise audit events

## Model

Append-only `audit_events` rows include: `id`, `organization_id`, `actor_type` (`user`, `machine`, `system`, `payment_provider`, `webhook`, `service`, …), optional `actor_id`, `action`, `resource_type`, optional `resource_id`, optional `machine_id`, optional `site_id`, `request_id`, `trace_id`, optional `ip_address` / `user_agent`, optional `before_json` / `after_json`, `metadata` (JSON), `outcome` (`success` / `failure`), `occurred_at`, `created_at`.

Secrets must never appear in JSON fields; the audit service applies normalization and key-based redaction (e.g. token, password, HMAC-related keys).

## Admin REST

- `GET /v1/admin/audit/events` — legacy/global-style URL; org scope from JWT or `organization_id` query for platform admins.
- `GET /v1/admin/organizations/{organizationId}/audit-events` — list with filters: `action`, `actorId` / `actor_id`, `actorType` / `actor_type`, `outcome`, `resourceType` / `resource_type`, `resourceId` / `resource_id`, `machineId` / `machine_id`, `from`, `to`, `limit`, `offset`.
- `GET /v1/admin/organizations/{organizationId}/audit-events/{auditEventId}` — fetch one row (404 if missing or outside org).

Requires **audit.read** (`audit:read`). Cross-organization access is denied at the HTTP scope layer.

## Critical writes

Security, payment webhook, activation **claim** / token **refresh**, and other critical mutations use **fail-closed** audit (`RecordCritical` / `RecordCriticalTx`) where required; optional dev-only `AUDIT_CRITICAL_FAIL_OPEN` does not apply to `RecordCriticalTx`.

Payment PSP webhooks attribute `actor_type` **`payment_provider`** for rejection / idempotency / reconciliation-case events.

OpenAPI: see `V1EnterpriseAuditEvent` and audit operations in `docs/swagger/swagger.json`.
