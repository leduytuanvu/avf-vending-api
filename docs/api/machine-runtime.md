# Machine runtime HTTP writes

These paths are part of the **control/setup plane** under `/v1`. They are not the primary high-volume device runtime path; MQTT remains the preferred runtime transport for continuous telemetry and command/ack traffic.

## POST `/v1/machines/{machineId}/check-ins`

- **Purpose**: machine control-plane check-in with latest machine/runtime metadata.
- **Auth**: Bearer JWT plus machine access on `machineId`.
- **Body**: current machine-reported fields such as versions, identifiers, timezone-relevant timestamps, and optional machine inventory/config hints. See OpenAPI examples in `docs/swagger/swagger.json`.
- **Success**: returns the persisted check-in snapshot with RFC3339/RFC3339Nano timestamp fields.
- **Notes**:
  - intended for bounded control-plane sync, not high-volume telemetry streaming
  - request/response timestamps are timezone-aware

## POST `/v1/machines/{machineId}/config-applies`

- **Purpose**: acknowledge that a machine applied a published configuration version.
- **Auth**: Bearer JWT plus machine access on `machineId`.
- **Body**: applied config version plus `applied_at` and related machine metadata.
- **Success**: returns the recorded apply acknowledgement.
- **Notes**:
  - this is the HTTP-side acknowledgement path for setup/control-plane config application
  - runtime telemetry and command lifecycles should still prefer MQTT where available

## Contract source

- OpenAPI annotations: `internal/httpserver/swagger_operations.go`
- Handler implementation: `internal/httpserver/machine_runtime_http.go`
- Generated spec: `docs/swagger/swagger.json`

## Offline queue and replay posture

These HTTP writes are part of the control/setup plane, but devices that use them during degraded connectivity should follow the same offline queue rules as the MQTT runtime path:

- persist the offline queue durably before attempting network send
- include stable replay identity on every offline event:
  - `machine_id`
  - `event_id` or `boot_id` + `seq_no`
  - `emitted_at`
  - `event_type`
  - `idempotency_key`
- apply deterministic initial replay jitter of `0-300` seconds from stable `machine_id`
- replay at `1-5` events/sec per machine
- use batch sizes of `20-50`
- use exponential backoff with jitter on failures

Critical machine-runtime writes must be retried until **application-level** acknowledgement, not merely transport success. **MQTT QoS 1 `PUBACK` is not a business ACK** for financial or inventory-critical telemetry; see [mqtt-contract.md](./mqtt-contract.md#application-level-ack-durable-device-outbox-and-business-durability-p0-clarity) for durable device outbox removal rules, HTTP `vend-results` semantics, and the current **P0 gap** (no first-party device reconcile API for arbitrary MQTT telemetry idempotency keys). Stale heartbeat-style liveness signals may be compacted or dropped when a newer equivalent state is already queued.
