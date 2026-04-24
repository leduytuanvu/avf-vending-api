# HTTP API roadmap (planned-only)

Capabilities listed here are **not** mounted on the public Chi router in this repository revision (or are explicitly future phases). They must **not** appear as `paths` in `docs/swagger/swagger.json` until the route exists in `internal/httpserver/server.go` and related handlers.

Shipped surfaces (including admin **catalog CRUD**, **cash settlement** close/start, **sale catalog**, **commerce cancel/refund**, **telemetry reconcile** device path, **activation claim**) are documented in OpenAPI and **`docs/api/api-surface-audit.md`** — they do **not** belong in this table.

| Area | Intended surface | Notes |
|------|------------------|--------|
| **Device activation (alternate)** | Non-claim activation flows (e.g. alternate `POST /v1/activation/...` shapes) | Today: `POST /v1/setup/activation-codes/claim` + admin provisioning. |
| **Runtime catalog variants** | Additional kiosk catalog modes beyond `GET /v1/machines/{machineId}/sale-catalog` | Split layouts, multi-language bundles, etc. |
| **HTTP telemetry operator ACK** | Human-driven reconcile/ack of pipeline events purely over HTTP | Today: device reconcile + JetStream pipeline. |
| **Third-party PSP webhooks (non-AVF HMAC)** | Alternate verification modes (provider-specific signatures) | Today: `COMMERCE_PAYMENT_WEBHOOK_VERIFICATION=avf_hmac`. |

When an item is implemented, add the route to the server, extend `internal/httpserver/swagger_operations.go`, run `make swagger`, and **remove** the row here.
