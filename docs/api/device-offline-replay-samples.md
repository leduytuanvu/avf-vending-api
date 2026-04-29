# Device Offline Replay Samples

`MachineOfflineSyncService.SyncOfflineEvents` accepts events in strict `offline_sequence` order per machine. Each event payload is JSON using the same proto JSON field names as the owning machine gRPC request.

Online and offline-dispatched mutations share the same **PostgreSQL mutation idempotency ledger** when `MACHINE_GRPC_REQUIRE_IDEMPOTENCY` is enabled (default in production wiring): stable `idempotency_key`, canonical payload hash, stored response replay, and `FailedPrecondition` / `idempotency_payload_mismatch` on hash conflict. See [`machine-grpc.md`](machine-grpc.md) § Idempotency ledger.

Example batch:

```json
{
  "meta": {
    "organizationId": "00000000-0000-0000-0000-000000000001",
    "machineId": "00000000-0000-0000-0000-000000000002",
    "requestId": "sync-2026-04-29-001",
    "idempotencyKey": "sync-2026-04-29-001"
  },
  "events": [
    {
      "meta": {
        "requestId": "offline-event-41",
        "clientEventId": "offline-event-41",
        "idempotencyKey": "order-41",
        "offlineSequence": "41",
        "occurredAt": "2026-04-29T01:00:00Z"
      },
      "eventType": "commerce.create_order",
      "payload": {
        "context": {
          "idempotencyKey": "order-41",
          "clientEventId": "offline-event-41",
          "clientCreatedAt": "2026-04-29T01:00:00Z"
        },
        "productId": "00000000-0000-0000-0000-000000000010",
        "currency": "USD",
        "slot": {
          "slotIndex": 3
        }
      }
    }
  ]
}
```

Supported `eventType` values:

- `commerce.create_order`
- `commerce.create_payment_session`
- `commerce.confirm_cash_payment`
- `commerce.start_vend`
- `commerce.confirm_vend_success`
- `commerce.confirm_vend_failure`
- `commerce.cancel_order`
- `inventory.report_delta`
- `inventory.adjustment`
- `inventory.restock`
- `telemetry.batch`
- `telemetry.critical`


See also **`docs/api/machine-grpc.md`** (**Idempotency ledger** section) — native online gRPC mutations use the Postgres-backed **`machine_idempotency_keys`** ledger with hashed payloads (excluding volatile **`request_id`** hints), orthogonal to MQTT/JetStream dedupe.



- **`offline_sequence`** must equal `last_sequence + 1` from the server cursor for that machine stream (otherwise **`Aborted`** — retry after sending missing events).
- **`clientEventId`** is optional but recommended; when set, it must be unique per machine for the lifetime of the offline journal so duplicate uploads resolve as **`REPLAYED`** instead of double-applying commerce/inventory effects.
