# Full System Verification Inventory

Generated: 2026-05-20 (branch `chore/final-full-system-verification-uuidv7-postman-tests`)

## Repository snapshot

| Item | Value |
|------|-------|
| Branch | `chore/final-full-system-verification-uuidv7-postman-tests` |
| OpenAPI path keys | **266** (`docs/swagger/swagger.json`) |
| OpenAPI operations (`operationId`) | **327** |
| Goose migrations | **5** (`migrations/*.sql`) |
| gRPC services | **21** (14 machine + 7 internal) |
| gRPC RPC methods | **80** (69 machine + 11 internal) |
| MQTT inbound patterns | **12** legacy / **13** enterprise |
| Postman collections (committed) | **4** |
| Postman environments (committed) | **5** |
| Docker compose services | **10** (3 core + 2 broker + 4 observability + 1 temporal-ui) |

---

## REST modules / routes

Source: `docs/swagger/swagger.json`, `internal/httpserver/`, `internal/app/api/`.

| Module area | Example paths | Auth |
|-------------|---------------|------|
| Health / version | `/health/live`, `/health/ready`, `/version` | Public |
| Auth | `/v1/auth/login`, `/refresh`, `/logout`, `/sessions` | Mixed |
| Admin catalog | `/v1/admin/categories`, `/brands`, `/tags`, `/products`, `/media` | Admin JWT + RBAC |
| Admin fleet | `/v1/admin/sites`, `/regions`, `/machines`, provisioning, activation | Admin JWT + RBAC |
| Admin inventory | `/v1/admin/planograms`, `/assortments`, `/slots`, refill | Admin JWT + RBAC |
| Commerce / orders | `/v1/admin/orders`, refunds, reconciliation | Admin JWT + RBAC |
| Machine REST bridge | `/v1/machines/{id}/…` (deprecated where gRPC preferred) | Machine JWT |
| Payments webhooks | `/v1/payments/webhooks/…` | Provider HMAC |
| Audit / reporting | `/v1/admin/audit-events`, reporting aggregates | Admin JWT + RBAC |
| OTA / config rollout | `/v1/admin/ota/…`, machine config rollouts | Admin JWT + RBAC |
| Finance | `/v1/admin/finance/…`, daily close | Admin JWT + RBAC |
| Telemetry (admin) | `/v1/admin/telemetry/…` | Admin JWT + RBAC |

Contract gate: `python tools/openapi_verify_release.py` — **PASS** (327 operations, Bearer on protected `/v1` routes).

---

## gRPC services / methods

Proto root: `proto/avf/machine/v1/` (field app), `proto/avf/internal/v1/` (internal query).

**Machine services (14):** `MachineTokenService`, `MachineActivationService`, `MachineInventoryService`, `MachineCatalogService`, `MachineBootstrapService`, `MachineTelemetryService`, `MachineOperatorService`, `MachineOfflineSyncService`, `MachineMediaService`, `MachineCommerceService`, `MachineSaleService`, `MachineCommandService`, `MachineAuthService`.

**Internal services (7):** reporting, payment, machine, telemetry, commerce, inventory, catalog query services.

Codegen: `make proto-generate` / `buf generate` — **PASS** (no drift on `internal/gen/`).

---

## MQTT topics / handlers

Implementation: `internal/platform/mqtt/topics.go`, `subscriber.go`, `router.go`.  
Ingest: `cmd/mqtt-ingest/main.go`.

| Relative tail | Purpose |
|---------------|---------|
| `telemetry`, `telemetry/snapshot`, `telemetry/incident` | Machine telemetry ingest |
| `presence`, `state/heartbeat` | Liveness |
| `events/vend`, `events/cash`, `events/inventory` | Domain events |
| `commands/dispatch`, `commands/ack`, `commands/receipt` | Command delivery + ACK |
| `shadow/desired`, `shadow/reported` | Device shadow |

Layouts: `legacy` (default) vs `enterprise` (`MQTT_TOPIC_LAYOUT=enterprise`).

---

## Database migrations

| File | Purpose |
|------|---------|
| `00001_placeholder.sql` | Goose baseline placeholder |
| `00002_platform_schema.sql` | Full platform DDL (v4 defaults historically) |
| `00003_seed_dev.sql` | Dev seed (admin, site, machine, products) |
| `00004_product_media_offline_cache.sql` | Media/offline cache tables |
| `00005_uuid_v7_defaults.sql` | `uuid_generate_v7()` + ALTER DEFAULT on 91 `id` columns |

Offline safety: `bash scripts/ci/verify_migrations.sh` — **PASS** (5 files, 0 findings).

---

## Test commands

| Command | Purpose |
|---------|---------|
| `go test ./...` | All unit + integration (integration skips without `TEST_DATABASE_URL`) |
| `go test ./... -short` | CI short mode |
| `make test-e2e-local` | P06 correctness + offline sync (requires DB, 45m) |
| `make ci-gates` | fmt, vet, placeholders, wiring, migrations, uuid-v7, api-contract-check |
| `bash scripts/local/verify-full-system.sh` | Full local verification wrapper |
| `bash scripts/audit/verify-uuid-v7.sh` | UUID v7 static audit |
| `bash scripts/test/run-full-backend-test-audit.sh` | REST/gRPC/MQTT inventory + reports |

---

## Postman generator / artifacts

| Path | Role |
|------|------|
| `tools/build_postman_collection.py` | Primary collection + env generator from OpenAPI |
| `postman/collections/avf-vending-api.postman_collection.json` | Main REST collection |
| `postman/environments/avf-local.postman_environment.json` | Local env template |
| `postman/suites/full-production-suite/` | Full parity suite (325+ REST requests) |
| `tools/check_postman_artifacts.py` | JSON validation + safety heuristics |

---

## Workers / background

| Binary | Role |
|--------|------|
| `cmd/worker` | Outbox publisher, background jobs |
| `cmd/mqtt-ingest` | MQTT → Postgres/NATS pipeline |
| `cmd/reconciler` | Payment/commerce reconciliation |
| `cmd/temporal-worker` | Workflow orchestration (experimental profile) |
| `cmd/migrate` | Embedded goose runner (`validate`, `up`, `version`) |

---

## External dependencies

| Dependency | Local | Production |
|------------|-------|------------|
| PostgreSQL | Docker `avf-postgres:15432` | Managed (Supabase/etc.) |
| Redis | Docker `6379` | Managed |
| NATS JetStream | Docker `4222` | Managed |
| MQTT (EMQX) | Profile `broker` `:1883` | External TLS broker |
| MinIO | Profile `broker` | S3-compatible object store |
| Payment PSP | **Not containerized** — webhook + probe URLs | Real provider secrets |
| ClickHouse / Temporal | Profile `experimental` only | Optional |

---

## Gaps fixed in this pass

1. **Migration 00005 goose split bug** — plpgsql blocks wrapped with `-- +goose StatementBegin/End` (was failing on fresh DB).
2. **UUID integration test** — unique region code per run; correct `uuid.Version(7)` assertion.
3. **`scripts/audit/verify-uuid-v7.sh`** — audit entrypoint delegating to CI check.
4. **`scripts/local/verify-full-system.sh`** — orchestrated verification with honest skip/fail summary.
5. **Postman collection** — `uuid7()` prerequest + `resource_uuid` variable for client-supplied IDs.

## Remaining gaps (documented, not blockers for merge if accepted)

| Gap | Mitigation |
|-----|------------|
| `rg` not on Windows PATH | CI runs `check-production-placeholders`; install ripgrep locally |
| MQTT live broker tests | `VERIFY_WITH_BROKER=1` + EMQX profile |
| gRPC live grpcurl suite | `VERIFY_WITH_GRPC=1` + running API |
| Newman CLI smoke | `VERIFY_WITH_NEWMAN=1` + `npm i -g newman` |
| 45m E2E correctness | `VERIFY_DESTRUCTIVE=1` + `TEST_DATABASE_URL` |
| Real PSP / hardware vend | Manual/external — see E2E report |
