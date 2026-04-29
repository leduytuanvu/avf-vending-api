# Outbox manual replay (`cmd/outbox-replay`)

Use when authenticated **admin HTTP** replay is unavailable (bastion job, break-glass automation). The binary loads **`DATABASE_URL`** (via `internal/config`) and performs **only** the operations below.

## Safety

- **`replay-dlq`** resets Postgres **dead-letter quarantine** so `cmd/worker` can publish again. This can **re-execute side effects** on downstream consumers if JetStream dedupe is bypassed or misconfigured. **Require** `-confirm-poison-replay`.
- **`requeue`** clears **lease / backoff** on an **unpublished** row (`pending`, `failed`, or expired `publishing` lease). It does **not** change financial OLTP tables. Interactive **yes** confirmation is required unless `-yes`.
- **`list`** is read-only JSON to stdout.

Prefer **`POST /v1/admin/system/outbox/{eventId}/replay`** with audit when the API is reachable ([outbox.md](./outbox.md)).

## Commands

```bash
# Unpublished rows created in the window [after, before), optional status filter.
go run ./cmd/outbox-replay list \
  -after 2026-04-28T00:00:00Z \
  -before 2026-04-29T00:00:00Z \
  -status failed \
  -limit 50

# Clear backoff/lease on one row (prompts unless -yes).
go run ./cmd/outbox-replay requeue -id 12345 -note "cleared after NATS incident" -yes

# After reviewing payload + idempotency, reset DLQ quarantine.
go run ./cmd/outbox-replay replay-dlq -id 12345 -confirm-poison-replay
```

## Related

- [outbox.md](./outbox.md) — transactional model, worker, HTTP replay
- [outbox-dlq-debug.md](./outbox-dlq-debug.md) — metrics and broker triage
