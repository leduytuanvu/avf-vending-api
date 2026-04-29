# MQTT command stuck / expiry

Use when machines stop acknowledging dispatched commands or receipts pile up.

## Quick triage (stuck command)

1. **Classify** using `machine_command_attempts.status`: `pending` (not yet published), `sent` (awaiting valid ACK), `ack_timeout` / `expired` (SLA passed), `failed` / `nack` (terminal failure).
2. **Correlate** ACK path: receipt must match **`command_id`** from `command_ledger` for the same **`machine_id`** and **`sequence`**; wrong `command_id` or wrong machine → rejected with **`avf_mqtt_command_ack_rejected_total{reason="command_id_mismatch"}`** (or `unknown_sequence`) and enterprise audit when applicable.
3. **Late success ACK**: after `ack_deadline_at` or ledger `timeout_at`, success ACK is **rejected** (`ledger timeout` / `ack deadline exceeded` strings in errors). Operator creates a **new** command if policy allows; do not “patch” ledger rows without a runbook.
4. **Duplicate ACK**: same **`dedupe_key`** → idempotent replay (`ReceiptReplay=true`); safe for device resends.
5. **Broker ACL**: if publish works from API but device never receives, check EMQX ACL and device subscription prefix; if cross-machine gossip appears, treat as **security incident** and verify `acl.conf`.

## What runs where

- **Ack deadlines**: `machine_command_attempts` rows in `sent` with `ack_deadline_at < now()` move to `ack_timeout` (`ApplyMQTTCommandAckTimeouts`). The **API** runs this before each dispatch; **cmd/mqtt-ingest** runs the same sweep on a **30s ticker** so machines that only use MQTT (no API traffic) still pick up SLA transitions without waiting for workers.
- **Ledger expiry**: `sent` attempts whose `command_ledger.timeout_at < now()` move to `expired`.
- **Dispatch ceiling**: new `machine_command_attempts` rows are refused when count ≥ `command_ledger.max_dispatch_attempts` (`ErrMQTTMaxDispatchAttemptsExceeded`).

## Queries

- Latest attempt per command: `GetLatestMachineCommandAttemptByCommandID` / `machine_command_attempts` where `command_id = …`.
- Receipts: `device_command_receipts` by `machine_id` + `sequence`.

## Metrics

- `avf_mqtt_command_ack_deadline_exceeded_total`
- `avf_mqtt_command_attempts_expired_total`
- `avf_mqtt_command_ack_rejected_total{reason="command_id_mismatch"}` (and related reasons)
- `avf_mqtt_command_dispatch_refused_total{reason="max_dispatch_attempts"}`

## Config

Production rejects plain `tcp://` / `mqtt://` to non-localhost brokers; localhost TCP remains valid for dev.

## Prometheus signals (canonical)

- `commands_dispatched_total`, `commands_ack_latency_seconds`, `commands_acked_total`, `commands_failed_total`, `commands_expired_total`
- `command_retry_total{reason=…}` — includes dispatch refused when max attempts reached (see `mqttprom` + `productionmetrics`).

## Production alerts (see `alerts.yml`)

| Alert | Threshold (summary) |
| ----- | ------------------- |
| **AVFCommandACKTimeoutSpike** | `avf_mqtt_command_ack_timeout_total` rate > **0.1/s** over **10m** on **`avf_api_metrics`** |
| **AVFMQTTCommandDispatchRefusedElevated** | `avf_mqtt_command_dispatch_refused_total` rate > **0.05/s** over **10m** |

**Runbooks:** `docs/runbooks/observability-alerts.md#command-ack-timeout-spike` and `#mqtt-command-dispatch-refused-elevated`.

Legacy `avf_mqtt_*` series may still mirror these; prefer stable names in [`docs/observability/production-metrics.md`](../observability/production-metrics.md).
