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
