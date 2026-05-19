# Phase 5 — MQTT catalog / media refresh trigger

## Goal

Notify machines over MQTT to **refresh catalog and media manifest**. MQTT is **only a trigger**: versions (`catalogVersion`, `mediaManifestVersion`) tell the device **what** to pull next; **binary images, base64, and bulk URL lists must not** appear on MQTT.

## Command flow

1. Backend appends a ledger row with `command_type: catalog.refresh` and dispatches the usual MQTT outbound wire (legacy `…/commands/dispatch` or enterprise `…/machines/{id}/commands`).
2. Inner JSON `payload` uses camelCase metadata: `type`, `catalogVersion`, `mediaManifestVersion`, `reason`.
3. Device performs HTTPS/gRPC (or existing bulk paths) sync; MQTT does not carry artifacts.
4. Device publishes `commands/ack` with top-level correlation plus nested `payload` including `mediaSynced: true` and the same version fields.

## Validation

- **Dispatch**: `ValidateCatalogRefreshDispatchPayload` in `internal/platform/mqtt/catalog_refresh.go` (also enforced in `MQTTCommandDispatcher.DispatchRemoteMQTTCommand`).
- **ACK**: `ValidateCatalogRefreshAckPayload` after `command_id` matches the ledger row for `(machine_id, sequence)` in `ApplyCommandReceiptTransition`. Normalized status must be `acked` (`success` / `ok` / `ack` aliases map to `acked`).

## Tests

- Unit: `internal/platform/mqtt/catalog_refresh_test.go`
- Postgres / command ledger: `internal/e2e/correctness/mqtt_command_integration_test.go` (`TestP06_E2E_MQTTCommand_catalogRefresh_*`)
- Broker smoke: `tests/e2e/scenarios/33_mqtt_catalog_refresh.sh` (included from `tests/e2e/run-mqtt-local.sh`)

## Fixtures

- Sample ACK: `testdata/telemetry/valid_catalog_refresh_command_ack.json`
