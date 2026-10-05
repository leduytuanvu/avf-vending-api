# Deploy: relaxed offline sequence (API before app)

1. **Migrate** `00039_machine_offline_events_idempotency_lookup.sql` on target Postgres.
2. **Deploy API** with env `MACHINE_OFFLINE_SEQUENCE_GAP_TOLERANT=true` (default). Roll back via `false` without redeploying app.
3. **Smoke**: existing field APKs must still push contiguous sequences; gap-tolerant server accepts them.
4. **Deploy app** (`avf-vending-app` with `OfflineSyncRelaxedMode` + abandoned tombstones).
5. **Verify** on a stuck machine: logcat `OUTBOX_SEQUENCE_RESTAMP_APPLIED`, `OFFLINE_SALE_REPLAY_ACCEPTED`, no repeating `OUTBOX_STREAM_STALLED floor=2`.
