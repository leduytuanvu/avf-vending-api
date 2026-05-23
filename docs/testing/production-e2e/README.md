# Production E2E harness

Manifest-driven production release verification. **Single source of truth:** [`tests/e2e/production/e2e-manifest.yaml`](../../tests/e2e/production/e2e-manifest.yaml).

## Quick start

```bash
# Contract validation (no network) + Postman generation
bash tests/e2e/production/run_production_e2e.sh --mode contract --dry-run

# REST-only contract
bash tests/e2e/production/run_production_e2e.sh --mode contract --dry-run --suite rest

# Public preflight against production (no admin credentials)
E2E_PROD_BASE_URL=https://api.ldtv.dev bash tests/e2e/production/run_production_e2e.sh --mode preflight --suite rest

# Full REST live run (requires .env.production.e2e.local or E2E_PROD_* CI secrets)
cp tests/e2e/production/.env.production.e2e.example tests/e2e/production/.env.production.e2e.local
# fill ADMIN_EMAIL, ADMIN_PASSWORD, COMMERCE_PAYMENT_WEBHOOK_SECRET
bash tests/e2e/production/run_production_e2e.sh --mode live --suite rest
```

PowerShell: `.\tests\e2e\production\run_production_e2e.ps1 -Mode contract -DryRun -Suite grpc`

### gRPC suite

Machine JWT comes from REST activation claim (`REST-MACHINE-004`); gRPC flows never use admin JWT.

```bash
# Contract (manifest + proto compile check)
bash tests/e2e/production/run_production_e2e.sh --mode contract --suite grpc

# Live (REST machine setup + full gRPC manifest)
E2E_PROD_GRPC_TARGET=api.ldtv.dev:443 \
  bash tests/e2e/production/run_production_e2e.sh --mode live --suite grpc
```

Manifest: `tests/e2e/production/e2e-manifest-grpc.yaml` (16 flows + handlers for commerce/media/offline).

### MQTT suite

Uses `E2E_PROD_MQTT_HOST`, `E2E_PROD_MQTT_USERNAME`, `E2E_PROD_MQTT_PASSWORD` (password never logged). Machine ID comes from REST activation in the same run only.

```bash
bash tests/e2e/production/run_production_e2e.sh --mode contract --suite mqtt
E2E_PROD_MQTT_HOST=mqtt.ldtv.dev \
  bash tests/e2e/production/run_production_e2e.sh --mode live --suite mqtt
```

Manifest: `tests/e2e/production/e2e-manifest-mqtt.yaml` (connect, command pipeline, telemetry, readback, negatives).

After a successful live REST run, the harness **regenerates Postman from the manifest**, runs structural parity validation, then Newman against production using runtime secrets injected into `.e2e-runs/production/<runId>/postman/runtime.postman_environment.json` (tracked env file has placeholders only).

**Postman import:** [postman-import.md](postman-import.md)

### REST route matrix (100% OpenAPI coverage)

Every production `method+path` from `https://api.ldtv.dev/swagger/doc.json` is mapped in:

- [`tests/e2e/production/generated/rest-route-matrix.json`](../../tests/e2e/production/generated/rest-route-matrix.json)
- [`docs/testing/production-e2e/rest-route-coverage.md`](rest-route-coverage.md)

Regenerate from live swagger + validate Postman parity:

```bash
bash tests/e2e/production/run_production_e2e.sh --mode route-matrix --suite rest --fetch-swagger
```

Coverage kinds: `success` (main manifest E2E flows), `readonly_smoke`, `auth_negative`, `permission_negative`, `documented_skip` (with reason in [`rest-route-overrides.yaml`](../../tests/e2e/production/rest-route-overrides.yaml)).

Auto-generated auth/readonly probes: [`tests/e2e/production/e2e-manifest-rest-coverage.yaml`](../../tests/e2e/production/e2e-manifest-rest-coverage.yaml) — do not hand-edit; rerun the generator.

Contract mode for `--suite rest` runs the matrix pipeline and fails on uncovered routes or undocumented skips. **Postman parity** (main manifest only) is enforced by `validate_postman_shell_parity.py` and CI `scripts/ci/verify_production_postman_parity.sh`.

## Artifacts

| Path | Purpose |
|------|---------|
| `.e2e-runs/production/<runId>/raw/` | Every request/response (REST, gRPC, MQTT) |
| `.e2e-runs/production/<runId>/RESULT.md` | Redacted evidence markdown |
| `postman/production/avf-production-e2e.postman_collection.json` | Generated REST parity (do not hand-edit) |
| `tests/e2e/production/generated/rest-route-matrix.json` | Production route coverage matrix |
| `tests/e2e/production/e2e-manifest-rest-coverage.yaml` | Auto-generated readonly/auth-negative REST probes |

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
