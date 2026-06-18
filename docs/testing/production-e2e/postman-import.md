# Production E2E — Postman import guide

The Postman collection and environment are **generated from** [`tests/e2e/production/e2e-manifest.yaml`](../../../tests/e2e/production/e2e-manifest.yaml) only. They mirror the shell REST harness that runs in production E2E live mode. After a successful REST run, the harness regenerates these files, validates structural parity, runs Newman, and records SHA-256 checksums in `RESULTS_<runId>.md`.

## Files

| File | Purpose |
|------|---------|
| `postman/production/avf-production-e2e.postman_collection.json` | All main-manifest REST requests (method, path, body, auth, assertions) |
| `postman/production/avf-production-e2e.postman_environment.json` | **Placeholder values only** — safe to commit and share |

Do not hand-edit the collection. Change [`e2e-manifest.yaml`](../../../tests/e2e/production/e2e-manifest.yaml) and regenerate:

```bash
python postman/production/generate_postman_from_manifest.py
python tests/e2e/production/scripts/validate_postman_shell_parity.py
```

## Import into Postman

1. Open Postman → **Import** → select both JSON files under `postman/production/`.
2. Select the **AVF Production E2E** environment in the top-right dropdown.
3. Set **secret / runtime variables** (current value). Non-secret defaults are pre-filled where safe.

### Required variables (secrets)

| Variable | Description |
|----------|-------------|
| `baseUrl` | Production API base, e.g. `https://api.ldtv.dev` |
| `adminEmail` | E2E admin service account email |
| `adminPassword` | E2E admin password |
| `webhookSecret` | Commerce payment webhook HMAC secret (for signed webhook requests) |

### Populated by the harness or prior requests

| Variable | Description |
|----------|-------------|
| `accessToken` | Admin JWT (set by login request or paste from harness) |
| `machineAccessToken` | Machine JWT after activation claim |
| `machineId` | E2E machine UUID |
| `activationCode` | One-time activation code |
| `runId` / `runPrefix` | E2E resource prefix (`E2E-PROD-<runId>`) |
| `categoryId`, `brandId`, `tagId`, `mediaId`, `productId`, `siteId` | Catalog entities |
| `orderId`, `paymentId`, `commandId` | Commerce entities |
| `operatorSessionId`, `planogramId`, `planogramRevision` | Planogram flows |
| `webhookEventId`, `mediaSha256` | Webhook / media fixtures |

### Gated writes (production safety)

Live mutating requests require:

| Variable | Value |
|----------|-------|
| `allowGatedWrites` | `true` |
| `confirmProductionWrites` | `I_UNDERSTAND_THIS_WRITES_TO_PRODUCTION` |

The shell harness sets these automatically in CI/runtime; set them manually in Postman before running write flows.

## Run with Newman (same as CI / E2E harness)

```bash
# After filling secrets in a local runtime env file (gitignored):
newman run postman/production/avf-production-e2e.postman_collection.json \
  -e .e2e-runs/production/<runId>/postman/runtime.postman_environment.json \
  --reporters cli,json \
  --reporter-json-export .e2e-runs/production/<runId>/postman/newman-report.json \
  --bail
```

The production E2E runner builds `runtime.postman_environment.json` from harness state + `E2E_PROD_*` secrets without writing secrets into the tracked environment file.

## Parity guarantee

CI runs `scripts/ci/verify_production_postman_parity.sh`, which:

1. Regenerates Postman from the manifest
2. Fails if `git diff postman/production/` is non-empty (drift)
3. Runs `validate_postman_shell_parity.py` (method, path, body, query, headers, auth, status, assertions)
4. Optionally runs Newman against production when `RUN_PRODUCTION_NEWMAN=1` and `E2E_PROD_ADMIN_*` are set

A user importing the generated files and filling **only secret variables** should get the same REST outcomes as the shell harness and Newman.

## Not in Postman

- **gRPC** and **MQTT** flows (see manifest gRPC/MQTT sections and `run_production_e2e.sh --suite grpc|mqtt`)
- **`negative` phase** REST flows (auth/webhook negative tests — shell only per manifest `postman.exclude_phases`)
- **Auto-generated route coverage** (`e2e-manifest-rest-coverage.yaml`) — shell route-matrix only

## Troubleshooting

| Symptom | Fix |
|---------|-----|
| `Gated write blocked` | Set `allowGatedWrites=true` and `confirmProductionWrites=I_UNDERSTAND_THIS_WRITES_TO_PRODUCTION` |
| 401 on admin routes | Set `accessToken` or run `REST-AUTH-001` login first with valid `adminEmail` / `adminPassword` |
| Webhook 401 | Set `webhookSecret`; collection pre-request scripts compute HMAC headers |
| Variable `{{machineAccessToken}}` empty | Complete machine activation flows or paste token from harness `state.json` |
