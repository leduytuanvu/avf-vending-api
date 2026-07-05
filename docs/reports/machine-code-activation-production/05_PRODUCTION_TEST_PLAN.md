# Production Test Plan — Machine-Code Activation

Date: 2026-07-06

## Environment (local only — never commit secrets)

Copy `tests/e2e/production/.env.production.e2e.example` → `tests/e2e/production/.env.production.e2e.local`.

Required variables:

| Variable | Value |
|----------|-------|
| `BASE_URL` | `https://api.ldtv.dev` |
| `ADMIN_EMAIL` | operator email (local env) |
| `ADMIN_PASSWORD` | operator password (local env) |
| `E2E_ALLOW_WRITES` | `true` |
| `E2E_PRODUCTION_WRITE_CONFIRMATION` | `I_UNDERSTAND_THIS_WRITES_TO_PRODUCTION` |
| `GRPC_ADDR` | `machine-api.ldtv.dev:443` |
| `MQTT_HOST` | `mqtt.ldtv.dev` |
| `MQTT_PORT` | `8883` |
| `MQTT_USE_TLS` | `true` |
| `RUN_PREFIX` | `MCODE-ACT-PROD-YYYYMMDD-HHMMSS` |

## Safety rules

- Bootstrap isolated test machine only (`bootstrap_test_data.py`)
- Revoke test activation codes after verification
- Redact tokens, MQTT passwords, activation plaintext in reports
- No modifications to real customer machines/orders/inventory

## Test phases

### Phase A — Preflight

1. `GET /health/live`, `/health/ready`, `/version`
2. `POST /v1/auth/login`, `GET /v1/auth/me`
3. Verify production `/version` SHA matches deployed commit

### Phase B — Activation smoke

Run: `python tools/production_full_test/run_machine_code_activation_prod.py`

Covers: create/list/revoke by machineCode, UUID paths, collection body variants, claim smoke.

### Phase C — Full surface suites

```bash
python tools/production_full_test/run_production_full_suite.py --base-url https://api.ldtv.dev --passes 3 --prefix MCODE-ACT-PROD-<UTC>
bash tests/e2e/production/run_production_e2e.sh --mode live --suite all-no-online-payment
```

### Phase D — Board replacement

- Local: `service_attachment_integration_test.go` (DB)
- Production: claim with fingerprint A then B on isolated machine (via activation smoke + bootstrap)

## Pass criteria

| Surface | Required |
|---------|----------|
| REST | `failed=skipped=blocked=not_run=0` (contract-only exceptions count as pass) |
| gRPC | same |
| MQTT | same |
| Activation-by-machineCode | all smoke tests pass |

## Rollback trigger

Any activation regression, secret leak, or production SHA mismatch → stop, rollback deploy, `NO_GO`.
