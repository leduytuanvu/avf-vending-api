# Postman enterprise runtime validation

**Audit ID:** `20260526T180000Z-runtime-validation`  
**Branch:** `postman/production-enterprise-project`  
**SHA:** `28732e2d`  
**Production `git_sha`:** `52a076e340a15a69dad7787cad54d7e3000fcafe` (from `/version`)

## Structural checkers (pre-runtime)

| Checker | Result |
|---------|--------|
| `check_enterprise_api_coverage.py` | `ENTERPRISE_COVERAGE_OK` |
| `check_enterprise_postman_completeness.py` | `ENTERPRISE_POSTMAN_COMPLETE_OK` |

## Production endpoint probes

| Probe | Result |
|-------|--------|
| `GET /health/live` | 200 |
| `GET /health/ready` | 200 |
| `GET /version` | OK |
| gRPC TLS `machine-api.ldtv.dev:443` (ALPN h2) | OK |
| MQTT TLS `mqtt.ldtv.dev:8883` | OK |

## Local credentials

- Source: `tests/e2e/production/.env.production.e2e.local` (gitignored)
- Postman env: `postman/production-enterprise/AVF_PRODUCTION_ENTERPRISE_LOCAL.postman_environment.json` (gitignored via `*LOCAL*`)
- Sync script: `postman/production-enterprise/sync_local_postman_env.py` → `SYNC_LOCAL_ENV_OK`
- Write gate: `allowGatedWrites=true`, `confirmProductionWrites=I_UNDERSTAND_THIS_WRITES_TO_PRODUCTION`

## Newman — enterprise REST collection (290 requests)

**Command:** `newman run postman/production-enterprise/AVF_PRODUCTION_ENTERPRISE_REST.postman_collection.json` with LOCAL env.

| Metric | Value |
|--------|------:|
| Requests executed | 290 |
| Assertions total | 282 |
| Assertions failed | **121** |
| Report | `.e2e-runs/production/enterprise-postman-newman-report.json` |
| CLI log | `.e2e-runs/production/enterprise-postman-newman-cli.log` |

**Result:** `NEWMAN_ENTERPRISE_COLLECTION_FAILED`

### Failure classification (no secrets)

| Category | Count | Verdict |
|----------|------:|---------|
| `ONLINE_PAYMENT_EXCLUDED` prerequest throw (folder 97/98 — expected guarded) | ~93 | Expected guarded skip — **collection bug**: throws error instead of skipping in Newman |
| Missing captured state (`productId`, `machineId`, `machineToken`, …) | ~71 | **Environment / execution order**: blind sequential run without E2E `state.json` |
| Auth-negative / optional 404–503 | ~33 | Expected for route-matrix / disabled contract probes |
| Other assertion mismatch | ~24 | Mixed |

**Conclusion:** Enterprise collection is **not** designed for one-shot Newman over all folders without harness state. Import works; login/health folders pass; full blind run fails.

## Newman — canonical E2E harness (`newman-no-online-payment`)

| Field | Value |
|-------|-------|
| RUN_ID | `20260526T175000Z-runtime-newman` |
| Collection | `postman/production` runtime collection from manifest + synced state |
| Result | `PRODUCTION_E2E_NO_ONLINE_PAYMENT_100_PERCENT_PASS` |
| Newman report | `.e2e-runs/production/20260526T175000Z-runtime-newman/postman/newman-report.json` |
| Pass / fail | 36 / 0 (manifest-eligible flows) |

**Result:** `NEWMAN_CANONICAL_E2E_PASS`

## gRPC runtime

| Field | Value |
|-------|-------|
| Suite | `grpc-inventory-media-cash-no-online-payment` |
| RUN_ID | `20260526T174000Z-runtime-grpc` |
| Result | `PRODUCTION_E2E_NO_ONLINE_PAYMENT_100_PERCENT_PASS` |
| Evidence | `docs/testing/production-e2e/RESULTS_20260526T174000Z-runtime-grpc.md` |

**Note:** First attempt reused stale RUN_ID `20260525T192300Z-1196-5901` → REST 409/500 + gRPC token errors (**production data collision**). Fresh RUN_ID passed.

## MQTT runtime

| Field | Value |
|-------|-------|
| Suite | `mqtt-command-telemetry-no-online-payment` |
| RUN_ID | `20260526T173000Z-runtime-mqtt` |
| Result | `PRODUCTION_E2E_NO_ONLINE_PAYMENT_100_PERCENT_PASS` |
| Cleanup | pass |
| Evidence | `docs/testing/production-e2e/RESULTS_20260526T173000Z-runtime-mqtt.md` |

## Full E2E (`all-no-online-payment`)

| Field | Value |
|-------|-------|
| RUN_ID | `20260526T180000Z-runtime-full` |
| Result | **INTERRUPTED** (harness stopped during REST-COV execution ~flow index 75) |
| Log | `.e2e-runs/production/runtime-full-e2e.log` |

No final `RESULTS_*.md` or cleanup attestation for this run.

## Postman import parity

| Path | Automation (E2E shell + state) | Enterprise blind Newman |
|------|-------------------------------|-------------------------|
| Manifest ~47 REST flows | PASS | N/A (not same collection) |
| Enterprise 290 REST items | N/A | FAIL (121 assertion failures) |

**Automation-pass / import-fail mismatch:** **YES** — canonical E2E Newman passes; enterprise full-collection Newman fails without harness state and counts payment-guard throws as failures.

## Cleanup

| Run | Cleanup attestation |
|-----|---------------------|
| gRPC `20260526T174000Z-runtime-grpc` | pass |
| MQTT `20260526T173000Z-runtime-mqtt` | pass |
| Newman `20260526T175000Z-runtime-newman` | pass |
| Full (interrupted) | not completed |

## Backend deploy / production deploy

| Item | Value |
|------|-------|
| Backend deploy required | **NO** (failures are harness-order, Newman execution model, or expected guards — not proven API regression on fresh RUN_ID) |
| Production deploy triggered | **NO** |

## Recommended follow-ups

1. Add `newman run` profile for enterprise: manifest-order folders only + synced env from `sync_local_postman_env.py` + E2E state export (or document “use E2E `newman-no-online-payment` suite”).
2. Change folder 97 prerequest to `postman.setNextRequest(null)` skip instead of `throw` for Newman compatibility.
3. Re-run `all-no-online-payment` to completion when a long window is available (~60–90 min).

## Final verdict (this session)

`POSTMAN_ENTERPRISE_RUNTIME_FAILED`

Structural coverage remains PASS; targeted production gRPC/MQTT/canonical Newman PASS; enterprise blind Newman and full E2E completion did not pass in this session.
