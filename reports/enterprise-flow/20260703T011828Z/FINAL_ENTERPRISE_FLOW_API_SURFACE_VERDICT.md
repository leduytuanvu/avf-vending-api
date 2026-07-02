# Final Enterprise Flow API Surface Verdict

**Timestamp:** 20260703T011828Z  
**Verdict:** `ENTERPRISE_FLOW_PARTIAL_BACKEND_READY_APP_FOLLOWUP_REQUIRED`

## Summary

Backend enterprise flow hardening is implemented for lifecycle accountability, device reattach, runtime session admin reads, ops overview, unified timeline, offline event aliases, surface validators, and schema migration `00016`.

## Surface counts

| Surface | Before | After |
|---------|--------|-------|
| REST paths | 268 | **274** |
| REST operations | 329 | **335** |
| gRPC services | 23 | **23** |
| gRPC RPCs | 88 | **88** |
| MQTT enterprise publish rel paths | 12 | **12** |

## Implemented

- Machine JWT blocked from `/v1/admin/*` (existing + preserved)
- Lifecycle POST routes require `reason`; standard mutation response; audit + optional attribution
- `POST /v1/admin/machines/{machineId}/reattach-device` with User JWT + policy
- Runtime session admin APIs (current/history/revoke) — no token hash exposure
- `GET /v1/admin/machines/{machineId}/ops-overview`
- `GET /v1/admin/machines/{machineId}/timeline/unified`
- Activation claim schema extended for accountability fields
- `machine_sessions` documented as canonical; refresh rotation preserved
- Operator `ended_reason` normalization helper
- Offline replay enterprise event type aliases
- API surface validators under `tools/enterprise_flow/` — **passing**

## Tests

- `go test ./internal/app/fleet/...` — pass
- `go test ./internal/domain/operator/...` — pass
- `go test ./internal/app/activation/...` — pass
- `go test ./internal/platform/mqtt/...` — pass
- `go test ./internal/grpcserver/... -run TestMapOfflineEventAlias` — pass
- `go test ./internal/httpserver/...` — pass
- Full DB integration/e2e — not executed in this session

## Known limitations

1. Nine payment ops + one media alias routes remain in `accepted_surface_exceptions.json` pending OpenAPI stubs
2. Postman production-full suite not regenerated
3. Unified timeline SQL merges audit/activation/attribution/sessions; legacy admin_ops timeline (commands/commerce) remains on existing `/timeline` endpoint
4. Public activation claim HTTP/gRPC not yet extended to accept optional `ClaimContext` on all paths

## App follow-up

- Treat backend `active` ≠ device activated; require local refresh token or activation/recovery after reinstall
- Send `reason`, `operator_session_id`, `correlation_id` on technician admin calls
- Use offline alias event types in durable outbox
