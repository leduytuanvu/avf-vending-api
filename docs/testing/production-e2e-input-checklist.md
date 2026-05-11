# Production E2E input checklist

Use before running destructive production E2E. Audit-only references: [`production-e2e-missing-inputs.md`](production-e2e-missing-inputs.md), [`production-e2e-input-audit.json`](production-e2e-input-audit.json).

## Environment & Git

- [ ] `tests/e2e/.env.production.destructive.local` exists and is **not** staged in git
- [ ] `.e2e-runs/` is **not** staged
- [ ] `tests/e2e/.env.production.destructive.local` listed in `.gitignore`
- [ ] No tokens/passwords/private keys in `git diff --cached`

## Destructive confirmations

- [ ] `E2E_PRODUCTION_WRITE_CONFIRMATION=I_UNDERSTAND_THIS_WRITES_TO_PRODUCTION`
- [ ] `E2E_PRODUCTION_DESTRUCTIVE_CONFIRMATION=I_UNDERSTAND_DB_WILL_BE_RESET_AFTER_TEST`
- [ ] `E2E_ALLOW_DESTRUCTIVE=true` and `E2E_ALLOW_DESTRUCTIVE_CLEANUP=true` intentional
- [ ] `E2E_ALLOW_REAL_PAYMENT` / `E2E_ALLOW_REAL_DISPENSE` / `E2E_ALLOW_REAL_MACHINE_COMMANDS` / `E2E_ALLOW_EXTERNAL_NOTIFICATIONS` reviewed (default **false**)

## Reachability

- [ ] `BASE_URL=https://api.ldtv.dev`
- [ ] `GET /health/live` passes (200)
- [ ] `GET /health/ready` passes (200)
- [ ] `GET /version` passes (200)

## Admin auth

- [ ] `ADMIN_TOKEN` **set**, **or**
- [ ] `ADMIN_EMAIL`, `ADMIN_PASSWORD`, and **`E2E_ORGANIZATION_ID`** set for `POST /v1/auth/login`
- [ ] If `ADMIN_TOKEN` only: `E2E_ORGANIZATION_ID` or reuse `test-data.json` with `organizationId`

## Test namespace (non-secret)

- [ ] `E2E_TEST_ORG_NAME`, `E2E_TEST_SITE_NAME`, `E2E_TEST_MACHINE_SERIAL`, `E2E_TEST_PRODUCT_PREFIX`, `E2E_TEST_SLOT_CODE` set
- [ ] `E2E_PAYMENT_MODE` matches intent (`mock_or_sandbox_only` unless ops approves real payment flag)

## Machine / gRPC

- [ ] `MACHINE_ID` set when required by scenarios
- [ ] `MACHINE_TOKEN` **or** `E2E_ACTIVATION_CODE` available
- [ ] `GRPC_ADDR` correct for prod (`api.ldtv.dev:443`)
- [ ] `GRPC_PROTO_ROOT` points at repo `proto` tree (reflection off)
- [ ] **TLS:** confirm grpcurl/harness can speak to `:443` (not only `-plaintext`)

## MQTT

- [ ] `MQTT_HOST` set (e.g. `mqtt.ldtv.dev` per runbook)
- [ ] `MQTT_PORT` / `MQTT_USE_TLS` match broker
- [ ] `MQTT_USERNAME` / `MQTT_PASSWORD` if broker requires auth
- [ ] Optional: `MQTT_CA_CERT`, client cert/key if mTLS

## Payment / webhooks

- [ ] `E2E_ALLOW_REAL_PAYMENT` intentionally **true** only if PSP capture is explicitly allowed
- [ ] For signed webhook E2E: `COMMERCE_PAYMENT_WEBHOOK_SECRET` or HMAC alias available in **shell env** (never commit)

## Media / object storage (optional)

- [ ] `E2E_MEDIA_TEST_FILE` points at a local fixture (e.g. `tests/e2e/fixtures/test-product-image.png`) if testing uploads
- [ ] `E2E_ALLOW_OBJECT_STORAGE_WRITE` aligned with risk

## Database (optional)

- [ ] `DATABASE_URL` only if operator-added **direct DB** tooling; not required for standard shell E2E

## After fill

- [ ] Rerun: `E2E_ENV_FILE=tests/e2e/.env.production.destructive.local ./tests/e2e/run-flow-review.sh --static-only`
- [ ] Then read-only smoke: `E2E_ALLOW_WRITES=false ./tests/e2e/run-rest-local.sh --readonly`
