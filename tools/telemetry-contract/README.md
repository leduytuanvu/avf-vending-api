# telemetry-contract

Lightweight checks for device telemetry JSON fixtures under `testdata/telemetry/` (valid JSON plus **critical identity** hints: `dedupe_key` / `event_id` / `boot_id`+`seq_no`, and `dedupe_key` on command ack fixtures).

## Validate JSON syntax

From repo root:

```bash
go run ./tools/telemetry-contract
```

Or with Python only:

```bash
for f in testdata/telemetry/*.json; do python -m json.tool "$f" >/dev/null && echo OK "$f"; done
```

## Tests

Contract behavior is enforced by `go test` in `internal/platform/mqtt` (`offline_replay_contract_test.go`) and `internal/app/telemetryapp` (`offline_replay_contract_test.go`).
