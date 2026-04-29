# Machine offline runbook

Use this when Admin Web, reports, metrics, or field staff report a machine as offline or stale.

## What "offline" means here

The API stores current machine status and last-seen timestamps in Postgres. Reporting marks machines as offline when status is a problem/terminal state, `last_seen_at` is missing, or `last_seen_at` is older than the report cutoff. MQTT and gRPC auth have their own connection/auth evidence; do not assume one signal explains all failures.

## Quick checks

1. Check `GET /health/ready` for backend dependency readiness.
2. Check Admin machine list/detail for `status`, `last_seen_at`, site, and assigned technician.
3. Check MQTT broker connection for the machine client and ACL topic prefix.
4. Check `cmd/mqtt-ingest` logs/metrics for heartbeat or telemetry ingest errors.
5. Check machine clock, network, TLS certificate validity, and firmware/app version.
6. If machine gRPC is in scope, verify Machine JWT issuer/audience, machine ID match, and credential version.

## Git Bash

```bash
BASE_URL="http://localhost:8080"
TOKEN="<admin bearer token>"
ORG_ID="11111111-1111-1111-1111-111111111111"
MACHINE_ID="55555555-5555-5555-5555-555555555555"

curl -sS "$BASE_URL/v1/admin/machines/$MACHINE_ID?organization_id=$ORG_ID" \
  -H "Authorization: Bearer $TOKEN"
```

## PowerShell

```powershell
$BaseUrl = "http://localhost:8080"
$Token = "<admin bearer token>"
$OrgId = "11111111-1111-1111-1111-111111111111"
$MachineId = "55555555-5555-5555-5555-555555555555"

Invoke-RestMethod -Method Get `
  -Uri "$BaseUrl/v1/admin/machines/$MachineId?organization_id=$OrgId" `
  -Headers @{ Authorization = "Bearer $Token" }
```

## Escalation

- If many machines go offline together, investigate MQTT broker, API readiness, Redis rate limiting, DNS/TLS, and network carrier issues before dispatching technicians.
- If one machine is offline, dispatch a technician with the assignment workflow and preserve non-secret evidence: machine ID, last successful heartbeat, broker client ID, and request IDs.
- Do not mark command ACKs successful manually. Use command/debug and reconciliation paths.

Related: `docs/runbooks/mqtt-command-debug.md`, `docs/api/machine-grpc.md`, `docs/runbooks/field-smoke-tests.md`.

## Prometheus signals (canonical)

- `machine_offline_events_total{result=…}` — volume and outcome of persisted offline batches.
- `machine_offline_replay_failures_total{reason=…}` — dispatch/replay rejects on the API.
- `machine_sync_lag_seconds` — lag between `occurred_at` and server processing.
- `machine_last_seen_age_seconds` — telemetry worker view of staleness vs `last_seen_at`.
- Full list: [`docs/observability/production-metrics.md`](../observability/production-metrics.md).
