# Production E2E harness

Manifest-driven production release verification. **Single source of truth:** [`tests/e2e/production/e2e-manifest.yaml`](../../tests/e2e/production/e2e-manifest.yaml).

## Quick start

```bash
# Contract validation (no network) + Postman generation
bash tests/e2e/production/run_production_e2e.sh --mode contract --dry-run

# Live run (requires .env.production.e2e.local or E2E_PROD_* CI secrets)
cp tests/e2e/production/.env.production.e2e.example tests/e2e/production/.env.production.e2e.local
bash tests/e2e/production/run_production_e2e.sh --mode live
```

PowerShell: `.\tests\e2e\production\run_production_e2e.ps1 -Mode contract -DryRun`

## Artifacts

| Path | Purpose |
|------|---------|
| `.e2e-runs/production/<runId>/raw/` | Every request/response (REST, gRPC, MQTT) |
| `.e2e-runs/production/<runId>/RESULT.md` | Redacted evidence markdown |
| `postman/production/avf-production-e2e.postman_collection.json` | Generated REST parity (do not hand-edit) |

## Protocol coverage

| Protocol | Runner | Postman |
|----------|--------|---------|
| REST | `lib/rest.sh` + Newman after live REST phase | Generated from manifest |
| gRPC | `lib/grpc.sh` (grpcurl) | Separate evidence section only |
| MQTT | `lib/mqtt.sh` (mosquitto_pub) | Separate evidence section only |

## Secrets

Never commit `.env.production.e2e.local`. GitHub Actions uses `E2E_PROD_*` secrets mapped in the manifest.

## Cleanup

All created entities use prefix `E2E-PROD-<runId>`. Cleanup must **never** delete non-prefixed production data.

See also: [`RESULT_TEMPLATE.md`](RESULT_TEMPLATE.md), [`docs/testing/05_PRODUCTION_TEST_EXECUTION_ORDER.md`](../05_PRODUCTION_TEST_EXECUTION_ORDER.md).
