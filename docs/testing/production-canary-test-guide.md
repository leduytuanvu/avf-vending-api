# Production Canary Test Guide

Production canary tests are intentionally blocked by default. Read-only smoke can run with a production or staging `BASE_URL`; write/destructive canary flows require explicit confirmation and canary-only resources.

## Read-Only Smoke

Allowed probes:

- `GET /health/live`
- `GET /health/ready`
- `GET /version`
- Optional extra read-only paths from `SMOKE_READONLY_PATHS`

Run:

```bash
bash scripts/e2e/production-readonly-smoke.sh
```

See also: [`PRODUCTION_E2E_CANARY_RUNBOOK.md`](PRODUCTION_E2E_CANARY_RUNBOOK.md).

## Canary Write E2E

Required environment:

```bash
ALLOW_PROD_WRITES=true
PROD_WRITE_CONFIRMATION=RUN_DESTRUCTIVE_PRODUCTION_TESTS
CANARY_SCOPE_ID=<canary company scope UUID only>
CANARY_MACHINE_ID=<canary machine only>
CANARY_MACHINE_TOKEN=<canary machine token>
CANARY_SITE_ID=<canary site only>
CANARY_PRODUCT_ID=<canary product only>
CANARY_SLOT_INDEX=<canary slot only>
BASE_URL=<production or staging API URL>
```

Rules:

- Do not use real customer company, site, machine, product, or slot IDs.
- Do not call a real payment provider destructive operation unless the PSP sandbox/canary account is explicitly configured.
- Do not publish commands to non-canary MQTT topics.
- Store evidence under `docs/reports/test/` and `.e2e-runs/`; redact tokens before sharing reports.
- If any required value is absent, the runner reports `BLOCKED`, not `PASS`.

Run:

```bash
bash scripts/e2e/production-canary-live-sale.sh
```

See [`PRODUCTION_E2E_CANARY_RUNBOOK.md`](PRODUCTION_E2E_CANARY_RUNBOOK.md) for full guard documentation.

## Blocked Proof

Provider and hardware proof remains blocked until the following are available:

- PSP sandbox/canary credentials and webhook secret aligned with the server.
- Canary vending hardware or simulator capable of command ACK, no-ACK timeout, dispense success, dispense failure, and offline replay.
- Production/staging URL reachable from the test runner.
