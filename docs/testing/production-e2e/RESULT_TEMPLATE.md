# Production E2E Result Template

Copy sections into `RESULT.md` after a live run. Replace placeholders; never paste live secrets.

## Run metadata

| Field | Value |
|-------|-------|
| run_id | `<PROD_E2E_RUN_ID>` |
| prefix | `E2E-PROD-<run_id>` |
| base_url | `<BASE_URL>` |
| started_utc | `<ISO8601>` |
| finished_utc | `<ISO8601>` |
| verdict | PASS / FAIL |

## Flow matrix

| id | label | protocol | status | evidence_label |
|----|-------|----------|--------|----------------|
| REST-PREFLIGHT-001 | GET /health/live | rest | | rest-health-live |

## REST example (redacted)

### REST-AUTH-001 — rest-auth-login

- method/path: `POST /v1/auth/login`
- status: `200`
- request (redacted):

```json
{"email":"<redacted>","password":"<redacted>"}
```

- response (redacted):

```json
{"tokens":{"accessToken":"<redacted>"}}
```

## gRPC section (not in Postman)

Document grpcurl evidence paths under `raw/grpc-*`.

## MQTT section (not in Postman)

Document mosquitto evidence paths under `raw/mqtt-*`.

## Postman / Newman

- collection: `postman/production/avf-production-e2e.postman_collection.json`
- report: `.e2e-runs/production/<runId>/postman/newman-report.json`

## Cleanup attestation

- [ ] Only `E2E-PROD-<run_id>` resources were targeted for cleanup
- [ ] No non-E2E production rows were deleted
