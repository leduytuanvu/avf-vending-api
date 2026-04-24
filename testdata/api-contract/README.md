# API contract fixtures (kiosk / device)

This directory indexes **contract artifacts** for client teams. **Authoritative MQTT samples** for critical telemetry live under **`testdata/telemetry/`** (see each `valid_*.json` and invalid cases).

## Layout

| Path | Purpose |
| --- | --- |
| `../telemetry/*.json` | Vend, payment, cash, inventory, heartbeat, command-ack, duplicate replay, invalid identity |
| `../../docs/api/examples/device-offline-replay-samples.md` | Human-readable replay checklist |
| `../../docs/api/mqtt-contract.md` | Topic layout, envelope fields, dedupe rules |
| `../../docs/api/kiosk-app-implementation-checklist.md` | Android kiosk HTTP + MQTT order checklist |
| `../../docs/api/examples/kiosk-implementation-payloads.md` | HTTP idempotency table |

## Example shapes (documentation only)

### Durable MQTT outbox row (Room / SQLite)

```json
{
  "local_id": "01ARZ3NDEKTSV4RRFFQ69G5FAV",
  "topic": "{prefix}/{machineId}/events/vend",
  "wire_json": {},
  "dedupe_key": "machine-local-sale:3fa85f64-5717-4562-b3fc-2c963f66afa6:000042",
  "retry_count": 0,
  "next_attempt_at_unix_ms": 1713955200000,
  "last_error_code": null
}
```

### HTTP headers for commerce POST

```json
{
  "Authorization": "Bearer <access_jwt>",
  "Idempotency-Key": "3fa85f64-5717-4562-b3fc-2c963f66afa6:sale:000042",
  "Content-Type": "application/json",
  "X-Request-ID": "optional-client-trace-id",
  "X-Correlation-ID": "optional-correlation-id"
}
```

## Adding new fixtures

1. Add JSON under `testdata/telemetry/` for MQTT-shaped payloads **or** new `.md` / `.json` under this directory when plan mode allows (avoid secrets).
2. Reference new files from `docs/api/examples/` so integrators discover them.
3. Extend `go run ./tools/telemetry-contract` samples if applicable.
