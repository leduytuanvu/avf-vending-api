# E2E shell harness (planned)

This directory will hold **optional** shell-driven E2E runs that complement:

- **Go/DB correctness:** [`docs/testing/local-e2e.md`](../../docs/testing/local-e2e.md) (`make test-e2e-local`)
- **Field pilot matrix:** [`docs/testing/field-test-cases.md`](../../docs/testing/field-test-cases.md)

## Documentation

| Doc | Purpose |
|-----|---------|
| [`docs/testing/e2e-flow-coverage.md`](../../docs/testing/e2e-flow-coverage.md) | Flow ↔ protocol matrix |
| [`docs/testing/e2e-local-test-guide.md`](../../docs/testing/e2e-local-test-guide.md) | Prerequisites, `.e2e-runs/` |
| [`docs/testing/e2e-test-data-guide.md`](../../docs/testing/e2e-test-data-guide.md) | Seeds, idempotency |
| [`docs/testing/e2e-troubleshooting.md`](../../docs/testing/e2e-troubleshooting.md) | Common failures |
| [`docs/testing/e2e-remediation-playbook.md`](../../docs/testing/e2e-remediation-playbook.md) | Structured fixes |

## Data templates (examples only)

- `data/seed.local.example.json` — initial fictional seed
- `data/reusable-test-data.example.json` — captured IDs after success
- `data/test-data.schema.json` — JSON Schema for capture file

## Intended commands (not yet implemented)

Run from repository root:

```bash
./tests/e2e/run-all-local.sh
./tests/e2e/run-rest-local.sh
./tests/e2e/run-grpc-local.sh
./tests/e2e/run-mqtt-local.sh
./tests/e2e/run-web-admin-flows.sh
./tests/e2e/run-vending-app-flows.sh
```

Options (convention): `--reuse-data path/to/capture.json`, `--fresh-data`, `--out .e2e-runs/run-xxx`.

Scripts will be added in a follow-up change; this README defines the **contract** for reviewers and CI wiring.
