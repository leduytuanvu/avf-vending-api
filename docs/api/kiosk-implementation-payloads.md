# Kiosk HTTP payloads & idempotency

> **Scope:** Narrative pointer for kiosk HTTP idempotency. For route tables and payload reference, see [examples/kiosk-implementation-payloads.md](examples/kiosk-implementation-payloads.md).

Kiosk apps must send a stable **`Idempotency-Key`** (or **`X-Idempotency-Key`**) on every mutating commerce route and retry with the **same key** after timeouts. Route-level requirements, vend/inventory coupling, and field notes live in the examples doc above; OpenAPI field schemas are in `docs/swagger/swagger.json` (`make swagger`).
