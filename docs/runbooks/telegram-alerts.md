# Telegram incident / error reporting (API)

## Bots

| Source | Bot env | Purpose |
|--------|---------|---------|
| `app` | `TELEGRAM_APP_BOT_TOKEN` (legacy fallback: `TELEGRAM_BOT_TOKEN`) | Machine/App incidents via durable outbox |
| `server` | `TELEGRAM_SERVER_BOT_TOKEN` | API/worker/process errors (never uses legacy APP token) |

Shared chat: `TELEGRAM_ALERT_CHAT_ID`.

## Flags

- `TELEGRAM_ALERTS_ENABLED` — master switch
- `TELEGRAM_ALERTS_REQUIRED` — when true, unconfigured delivery **must not ACK** outbox messages
- `TELEGRAM_INCIDENT_REPEAT_MODE=every|aggregate` — default `every` (each new occurrence alerts)
- `TELEGRAM_INCIDENT_COOLDOWN` — used only in `aggregate` mode via `last_alerted_at`

## App path

gRPC `PushCriticalEvent` / `SubmitTelemetryBatch` (canonical) / MQTT JetStream `handleIncident` / HTTP `POST /machines/{id}/incidents`
→ `ProjectMachineIncident` → `machine_incident_occurrences` + grouped `machine_incidents`
→ `notification.telegram` outbox (`source=app`) → worker `TelegramDispatcher` → APP bot

Occurrence identity is App `event_id` (string, e.g. `incident_anr:…`). Transport duplicates share `(machine_id, occurrence_id)`. Fingerprint groups rows; it is never the occurrence key.

MQTT is a mirror path only — gRPC alone is sufficient for App incidents.

## Persistence / sqlc

`ProjectMachineIncident` and occurrence retention prune use **intentional multi-statement raw SQL** inside transactions (occurrence insert → group update → outbox enqueue). They are **not** sqlc one-shot queries. Do not re-add unused `db/queries/machine_incident_occurrences.sql` unless the store is migrated to generated APIs end-to-end.

Schema: migration `00020_machine_incident_occurrences.sql` + canonical `db/schema/01_platform.sql`.

## Server path

- Unexpected HTTP 5xx (observability middleware) → `ServerErrorReporter`
- Unexpected gRPC Internal/Unknown/DataLoss/Unavailable + recovered panic → `ServerErrorReporter`
- Process terminal exit (`api`/`worker`/`mqtt-ingest`/`reconciler`/`temporal-worker`) → bounded report before nonzero exit

Durable outbox `source=server` when DB works; else bounded emergency direct SERVER bot send.

App-origin `incident_*` telemetry success paths are **not** SERVER pages.

## External semantics

Telegram delivery is at-least-once. Send-before-ACK races can produce a rare duplicate external message for the same occurrence.

## Manual smoke (safe)

Configure tokens/chat in the environment (never commit secrets). Trigger a non-destructive `[TEST]` alert via operator tooling or a staging-only test occurrence. Do not use live payment/vend flows solely to test Telegram.
