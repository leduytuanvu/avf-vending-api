# Repository Structure P2 Refactor Proposal

**Date:** 2026-05-20  
**Status:** Proposal only — **do not implement from this document without a dedicated PR plan**  
**Scope:** Deeper architecture refactor for the AVF Vending API modular monolith  
**Baseline:** Post–Phase 5 cleanup (`docs/audits/DEEP_REPO_CLEANUP_AUDIT.md`, `docs/architecture/current-architecture.md`)

---

## Executive summary

The repository is a **Go 1.25 modular monolith** with seven production binaries, **41 flat packages** under `internal/app/*`, a **monolithic Postgres adapter** in `internal/modules/postgres`, and **thin-in-theory / thick-in-practice** transport layers (`internal/httpserver` ~99 Go files, `internal/grpcserver` ~40 files). Business logic is mostly in `internal/app/*`, but boundaries are **organizational rather than enforced**: cross-domain imports, shared sqlc queries, and bootstrap wiring create hidden coupling.

**P2 goal:** Introduce **explicit bounded contexts** and **stable ports** so the monolith can evolve (clearer ownership, safer refactors, optional future extraction) **without** changing runtime behavior, DB schema, or public contracts in the first PRs.

**Non-goals for P2:**

- Splitting into microservices or separate repos
- Changing `go.mod` module path (`github.com/avf/avf-vending-api`)
- Moving `migrations/**` or altering goose production history
- Hand-editing generated OpenAPI / Postman artifacts

---

## 1. Current module boundaries

### 1.1 Process (`cmd/*`) boundaries

| Binary | Primary role | Key wiring | Shared dependencies |
|--------|--------------|------------|---------------------|
| `cmd/api` | HTTP `/v1`, optional `avf.machine.v1` + `avf.internal.v1` gRPC | `internal/bootstrap/api.go` → `internal/httpserver`, `internal/grpcserver` | Postgres, Redis, MQTT publisher, NATS (required in prod posture), object store, Temporal client (optional) |
| `cmd/mqtt-ingest` | MQTT subscriber → ingest / JetStream buffer | `internal/bootstrap` ingest path | Postgres, NATS, MQTT |
| `cmd/worker` | Outbox dispatch, reliability scans, telemetry consumers, retention | `internal/bootstrap` worker path | Postgres, NATS, optional ClickHouse mirror |
| `cmd/reconciler` | Commerce/payment reconciliation ticks | `internal/bootstrap/reconciler.go` | Postgres, optional PSP probe + NATS refund enqueue |
| `cmd/temporal-worker` | Temporal workflows (payment timeout, vend failure, refund, manual review) | `internal/bootstrap/temporal_worker.go` | Postgres, Temporal, NATS (activities) |
| `cmd/outbox-replay` | Break-glass outbox replay CLI | Direct app/postgres | Postgres |
| `cmd/cli` | `-validate-config`, `-version` | `internal/config` | None |

**Observation:** Each binary reuses `internal/bootstrap.BuildRuntime` patterns and the same `internal/modules/postgres` repository surface. Process boundaries exist at **deployment** level, not at **Go package import** level.

### 1.2 Application layer (`internal/app/*`)

Today there are **41 sibling packages** with no enforced hierarchy:

| Cluster (informal) | Current packages | Transport entrypoints |
|--------------------|------------------|------------------------|
| **Auth / admin identity** | `auth`, `operator`, `provisioning`, `setupapp`, `activation`, `devicecerts` | `httpserver/*_auth*.go`, `activation_http.go`, `setup_http.go` |
| **Audit / compliance** | `audit`, `anomalies`, `retention`, `adminops` | `admin_*_http.go`, `/v1/admin/ops/*`, `/v1/admin/system/outbox` |
| **Catalog / media / sale surface** | `catalogadmin`, `salecatalog`, `mediaadmin`, `assortmentapp`, `pricingengine`, `planogram`, `artifacts` | Admin catalog HTTP, machine catalog gRPC, sale catalog HTTP/gRPC |
| **Machine runtime** | `machineruntime`, `machineidempotency`, `device`, `sellreadiness` | `machine_*_grpc.go`, deprecated machine REST, MQTT command path |
| **Commerce / payments** | `commerce`, `commerceadmin`, `payments`, `finance`, `workfloworch` | Commerce HTTP/gRPC, webhooks, Temporal |
| **Inventory / fleet / refill** | `inventoryadmin`, `inventoryapp`, `fleet`, `fleetadmin`, `rollout` | Admin inventory/fleet HTTP, machine inventory gRPC |
| **Telemetry** | `telemetryapp`, `reliability` | MQTT ingest, worker consumers, machine telemetry gRPC |
| **OTA / config** | `otaadmin`, `featureflags` | Admin OTA HTTP, runtime read models |
| **Async / platform ops** | `background`, `outbox`, `reporting`, `reports` | Worker, reconciler, admin reporting HTTP |
| **Composition** | `api` (HTTP application facade), `listscope` | `httpserver.NewHTTPServer` deps |

**Observation:** Naming mixes `*admin`, `*app`, and bare domain names without a consistent port/adapter split. `internal/app/api` acts as a **facade** but many handlers still reach into postgres modules or cross-app packages directly.

### 1.3 Domain layer (`internal/domain/*`)

Nine packages today: `cash`, `commerce`, `compliance`, `device`, `fleet`, `operator`, `org`, `reliability`, `retail`.

**Coverage is partial.** Most business rules live in `internal/app/*`; `internal/domain/*` holds types and some pure logic. There is **no** domain package per bounded context (e.g. no `catalog`, `telemetry`, `payments` domain root).

**Removed placeholders (2026-06 cleanup, Phase 1):**

- ~~`internal/repository/cash/doc.go`~~ — deleted (doc-only; cash settlement uses `internal/modules/postgres` + admin HTTP)
- ~~`internal/service/cash/doc.go`~~ — deleted (same)
- ~~`pkg/doc.go`~~ — deleted (zero imports; no external public library yet)

Reintroduce `pkg/` or context-specific `service`/`repository` packages only when implementing the P2 port/adapter split (PR 4), not as empty placeholders.

### 1.4 Infrastructure and adapters

| Layer | Path | Role today |
|-------|------|------------|
| **Postgres OLTP** | `internal/modules/postgres` (~50+ files), `internal/gen/db`, `db/queries/*.sql` | Single repository implementation; query files grouped by SQL name, not by bounded context package |
| **Platform** | `internal/platform/{auth,db,mqtt,nats,redis,objectstore,payments,temporal,clickhouse,telemetry,ratelimit,rbac,refunds}` | Shared drivers and cross-cutting clients |
| **HTTP adapter** | `internal/httpserver` | Chi router, ~99 Go files, OpenAPI annotations via `swagger_operations.go` |
| **gRPC adapter** | `internal/grpcserver` | Machine + internal query services, replay ledger, interceptors |
| **Observability** | `internal/observability`, `internal/platform/observability/productionmetrics` | Tracing, metrics, dashboard artifact tests |
| **Config** | `internal/config` | Large env surface; deployment coupling (`deployment_env.go`, feature flags) |
| **Bootstrap** | `internal/bootstrap` | Per-binary composition root; highest fan-in file set |

### 1.5 Contract and generated artifacts

| Artifact | Location | Generator / gate |
|----------|----------|------------------|
| OpenAPI 3.0 | `docs/swagger/swagger.json`, `docs/swagger/docs.go` (Go embed) | `make swagger`, `make swagger-check` |
| Postman CI inventory | `postman/collections/`, `postman/environments/` | `make postman-check` |
| Production Postman suite | `postman/suites/full-production-suite/` | `generate_full_postman_suite.py` |
| Protobuf | `proto/avf/machine/v1`, `proto/avf/internal/v1`, `proto/avf/v1` | `make proto-check` |
| sqlc | `db/schema/01_platform.sql`, `db/queries/`, `internal/gen/db` | `make sqlc-check` |
| Migrations (SoR) | `migrations/*.sql` (4 goose files) | `scripts/ci/verify_migrations.sh` |

### 1.6 Current boundary weaknesses (repo-specific)

1. **Flat `internal/app/*`** — easy cross-imports (e.g. commerce ↔ device ↔ fleet) with no compile-time boundary.
2. **Monolithic `internal/modules/postgres`** — one package implements all sqlc-backed repositories; refactors touch wide blast radius.
3. **Fat transport layers** — HTTP and gRPC handlers carry orchestration, validation, and mapping; duplicate paths for machine REST (deprecated), gRPC, and admin REST.
4. **Bootstrap fan-in** — `internal/bootstrap/api.go` imports many app packages directly; adding a domain requires editing composition roots.
5. **Duplicate proto tree** — `postman/suites/full-production-suite/grpc/proto/` copies root `proto/` (drift risk; noted in cleanup audit).
6. **Legacy compatibility** — `MACHINE_REST_LEGACY_ENABLED`, deprecated OpenAPI machine routes, dual outbox admin paths (`/v1/admin/ops/*` vs `/v1/admin/system/outbox`).

---

## 2. Proposed enterprise package boundaries

Principles (from `docs/architecture/enterprise-target-model.md`):

- **Admin REST** → User JWT + RBAC (`internal/platform/auth`)
- **Machine runtime** → Machine JWT on `avf.machine.v1`
- **Internal gRPC** → loopback read/query only (`avf.internal.v1`, service JWT)
- **MQTT** → device ingress + backend→device commands
- **Mutations** → idempotent + auditable where required
- **Postgres** → system of record; Redis/NATS/MQTT are infrastructure, not domain truth

### 2.1 Recommended bounded contexts

Each row is a **target ownership boundary** (package + docs + tests), not a separate service.

| Bounded context | Owns (behavior) | Current packages (primary) | sqlc query files (representative) | Public surfaces |
|-----------------|-----------------|----------------------------|-----------------------------------|-----------------|
| **Auth & admin identity** | User JWT, RBAC, operator sessions, login/refresh, machine token issuance hooks | `auth`, `operator`, `provisioning`, `setupapp`, `activation`, `devicecerts`; platform: `auth`, `rbac` | `auth.sql`, `auth_admin.sql`, `machine_auth.sql`, `machine_runtime_auth.sql`, `provisioning.sql` | Admin REST auth routes; machine activation/setup |
| **Audit & compliance** | Enterprise audit rows, anomaly signals, retention policy execution | `audit`, `anomalies`, `retention`, `adminops` | `enterprise_audit.sql`, `operational_anomalies.sql`, `retention.sql`, `compliance.sql` | Admin audit reads; outbox/DLQ ops |
| **Catalog, products, media, offline cache** | Product/assortment admin, sale catalog projection, media metadata + object keys, offline cache invalidation | `catalogadmin`, `salecatalog`, `mediaadmin`, `assortmentapp`, `pricingengine`, `planogram`, `artifacts` | `catalog.sql`, `catalog_admin.sql`, `catalog_writes.sql`, `runtime_catalog.sql`, `media_admin.sql`, `pricing_runtime.sql`, `promotion_admin.sql` | Admin catalog REST; machine catalog gRPC; HTTPS media URLs |
| **Machine runtime** | Cabinet state, vend session orchestration, machine idempotency ledger, replay, sell readiness | `machineruntime`, `machineidempotency`, `device`, `sellreadiness` | `machine_runtime.sql`, `machine_idempotency.sql`, `machine_offline.sql`, `device.sql`, `device_commands.sql` | `avf.machine.v1` commerce/runtime/inventory; deprecated machine REST |
| **MQTT ingest & commands** | Inbound telemetry/status ingest, command ledger, ACK correlation, topic layout | `device` (inbound/outbound), platform `mqtt`; ingest binary | `device_commands.sql`, `messaging.sql`, `critical_telemetry.sql` | MQTT topics; optional JetStream buffer |
| **Telemetry & observability pipeline** | Telemetry normalization, retention, worker projections, metrics | `telemetryapp`, `reliability`; worker metrics packages | `telemetry_retention.sql`, `critical_telemetry.sql` | MQTT → ingest → worker; machine telemetry gRPC |
| **Payments & commerce** | Orders, payment sessions, webhooks (HMAC), vend lifecycle, refunds, reconciliation | `commerce`, `commerceadmin`, `payments`, `finance`, `workfloworch`; platform `payments`, `refunds` | `commerce.sql`, `commerce_admin.sql`, `commerce_timelines_refunds.sql`, `payment_admin.sql`, `payment_reconciliation.sql`, `finance_daily_close.sql` | REST commerce + webhooks; machine commerce gRPC; reconciler; Temporal |
| **Orders / vend sessions** | (Sub-domain of commerce — keep nested) Order state machine, vend start/success/failure | `commerce`, `machineruntime` (handoff) | `commerce.sql`, `machine_runtime.sql` | HTTP + gRPC commerce RPCs |
| **Inventory & refill** | Slot inventory, adjustments, refill workflows, planogram alignment | `inventoryadmin`, `inventoryapp`, `planogram` | `inventory.sql`, `inventory_admin.sql`, `planogram.sql`, `topology.sql` | Admin inventory REST; machine inventory gRPC |
| **Fleet, rollout, config, OTA** | Machine registry, sites, rollout campaigns, feature flags, OTA metadata | `fleet`, `fleetadmin`, `rollout`, `featureflags`, `otaadmin` | `fleet.sql`, `fleet_admin.sql`, `fleet_lifecycle.sql`, `rollout.sql`, `feature_flags.sql`, `ota_admin.sql`, `device_ota_diagnostics.sql` | Admin fleet/OTA REST; runtime flag reads on machine surface |
| **Worker, outbox, reconciler** | Transactional outbox, JetStream publish/DLQ, reliability scans, commerce reconciliation | `background`, `outbox`, `reliability`; reconciler bootstrap | `outbox.sql`, `reliability.sql`, `payment_reconciliation.sql`, `admin_outbox_ops.sql` | Worker/reconciler processes; admin outbox HTTP |
| **Reporting** | Read-only aggregates / exports | `reporting`, `reports` | `reporting.sql`, `reports.sql` | `/v1/reports/*`, `/v1/admin/reports/*` |
| **Shared platform / kernel** | DB pool, Redis, NATS, object store, config validation, error envelope, tracing, rate limits | `internal/platform/*`, `internal/config`, `internal/apierr`, `internal/observability`, `internal/bootstrap`, `internal/middleware`, `internal/version` | N/A (no domain tables) | Used by all binaries |

### 2.2 Cross-context interaction rules (target)

```
Transport (httpserver, grpcserver, mqtt consumer)
    → Application service (per context: internal/<context>/app)
        → Domain (pure rules: internal/<context>/domain)
        → Ports (interfaces: internal/<context>/ports)
            → Adapters (postgres, mqtt, nats, objectstore: internal/<context>/adapter/postgres)
```

**Allowed:**

- Context A → **Shared kernel** (`platform`, `apierr`, `config` types)
- Context A → Context B via **explicit port interface** defined in B (or shared `internal/contracts` for read-only DTOs)
- All contexts → `internal/gen/db` **only through their own adapter** subpackage

**Forbidden after P2:**

- `catalogadmin` importing `commerce` concrete types (use port)
- HTTP handlers importing `internal/modules/postgres` directly
- New code in `internal/repository/*` or `internal/service/*` unless part of an approved split

### 2.3 Shared kernel vs domain-specific platform

| Keep in shared kernel | Keep context-local |
|-----------------------|-------------------|
| `platform/auth` (JWT, RBAC middleware) | Commerce-specific webhook signature validators |
| `platform/db`, connection lifecycle | Commerce repository adapter |
| `platform/mqtt`, `platform/nats` clients | Topic/command mapping owned by machine runtime + MQTT ingest contexts |
| `platform/objectstore` client | Media key conventions owned by catalog/media context |
| `config` loading + validation | Feature-specific config structs colocated with context (embedded in root config) |

---

## 3. Proposed final folder structure

**Target:** Modular monolith with **vertical slices** under `internal/`, preserving `cmd/*` binaries and `migrations/` at repo root.

```
avf-vending-api/
├── cmd/                          # unchanged binary entrypoints
├── migrations/                   # goose SoR (never split)
├── db/                           # sqlc schema + queries (may split by context later; single sqlc.yaml initially)
├── proto/                        # source of truth for gRPC
├── docs/swagger/                 # OpenAPI embed (Go import path fixed)
├── postman/                      # CI + production inventory
├── deployments/                  # compose, prod/staging (unchanged role)
├── scripts/                      # CI, deploy, test wrappers
├── tools/                        # generators (openapi, postman, loadtest)
├── tests/e2e/                    # Newman/bash harness
│
└── internal/
    ├── kernel/                   # NEW: renamed/shared infra (optional rename from platform/)
    │   ├── config/
    │   ├── auth/                 # JWT, RBAC, machine URL access
    │   ├── db/
    │   ├── mqtt/
    │   ├── nats/
    │   ├── redis/
    │   ├── objectstore/
    │   ├── observability/
    │   └── apierr/
    │
    ├── transport/                # NEW: thin adapters only
    │   ├── http/                 # from httpserver (split by admin/machine/webhook mount)
    │   ├── grpc/                 # from grpcserver (machine + internal)
    │   └── mqtt/               # ingest subscriber wiring (from mqtt-ingest bootstrap)
    │
    ├── composition/              # NEW: from bootstrap (per-binary wiring)
    │   ├── api/
    │   ├── worker/
    │   ├── mqtt_ingest/
    │   ├── reconciler/
    │   └── temporal_worker/
    │
    ├── catalog/                  # bounded context example
    │   ├── app/
    │   ├── domain/
    │   ├── ports/
    │   └── adapter/
    │       └── postgres/
    │
    ├── commerce/
    ├── machine/
    ├── inventory/
    ├── fleet/
    ├── telemetry/
    ├── audit/
    ├── authadmin/                # admin auth (distinct from kernel auth primitives)
    ├── async/                    # outbox, worker jobs, reconciler use cases
    └── reporting/
    │
    ├── gen/                      # unchanged: db, avfinternalv1
    └── e2e/                      # integration tests (correctness)
```

**Phased adoption:** Do **not** big-bang rename `internal/platform` → `internal/kernel` in PR 1. Introduce `internal/<context>/` packages **alongside** existing `internal/app/*` and migrate callers incrementally (strangler inside the monolith).

**sqlc strategy (later PR):** Split `db/queries/*.sql` into subfolders only when `sqlc.yaml` supports multi-schema packages cleanly; until then, keep one `internal/gen/db` but **wrap** with per-context adapter types.

**Proto strategy:** Delete `postman/suites/full-production-suite/grpc/proto/` copies; generate suite from root `proto/` in CI (audit item P2.6).

---

## 4. Migration strategy (small PRs)

### PR 1 — Package boundary docs (zero runtime change)

**Deliverables:**

- Add `docs/architecture/bounded-contexts.md` (context map, ownership, allowed dependencies)
- Add `docs/architecture/package-import-rules.md` (forbidden imports, diagram)
- Add CODEOWNERS entries per bounded context (optional)
- Annotate this proposal with ADR links

**Touches:** `docs/**` only  
**Risk:** Low  
**Gate:** `make ci-gates` doc link check; no code change

---

### PR 2 — Move pure helper packages

**Deliverables:**

- ~~Consolidate or remove placeholders: `pkg/`, `internal/repository/cash`, `internal/service/cash`~~ — **done (2026-06 Phase 1 cleanup)**; next PR 2 work is pure helpers + port interfaces
- Extract **pure** utilities with no postgres/MQTT imports from fat handlers into:
  - `internal/kernel/httputil` (or keep in `internal/apierr` / small `internal/kernel/ptr` if warranted)
  - Domain pure functions from `internal/app/*` into matching `internal/<context>/domain` **without moving stateful services yet**
- Introduce empty `internal/<context>/ports` interfaces mirroring existing postgres calls (interface-only; implementations still delegate to `internal/modules/postgres`)

**Touches:** Low fan-in helpers, tests co-located  
**Risk:** Low–medium (import path churn)  
**Gate:** `go test ./...`, `go vet ./...`, `make api-contract-check`

---

### PR 3 — Split domain packages (strangler)

**Deliverables (incremental, one context per sub-PR recommended):**

1. **Catalog/media** — move `catalogadmin`, `salecatalog`, `mediaadmin`, … → `internal/catalog/app`
2. **Commerce/payments** — move `commerce`, `payments`, … → `internal/commerce/app`
3. **Machine runtime** — move `machineruntime`, `machineidempotency`, … → `internal/machine/app`
4. Continue for inventory, fleet, telemetry, audit, async

**Mechanics:**

- `git mv` + update imports via mechanical refactor
- Keep **type aliases** in old `internal/app/<name>` packages temporarily (`type X = catalogapp.X`) to avoid mega-PR

**Risk:** Medium (wide import graph)  
**Gate:** Full test suite + `internal/e2e/correctness` with `TEST_DATABASE_URL`

---

### PR 4 — Stabilize interfaces (ports & adapters)

**Deliverables:**

- Split `internal/modules/postgres` into `internal/<context>/adapter/postgres` packages
- HTTP/gRPC handlers depend on **ports** only; wire concrete adapters in `internal/composition/*`
- Reduce `internal/bootstrap/api.go` fan-in by context-specific `composition/catalog/wire.go` pattern
- Add `go:generate` or lint script (`scripts/ci/check_import_boundaries.go`) enforcing:
  - `internal/transport/http` must not import `internal/gen/db`
  - `internal/catalog/domain` must not import `internal/commerce`

**Risk:** Medium–high (bootstrap wiring, test doubles)  
**Gate:** `make api-contract-check`, `make proto-check`, integration tests, Postman artifact check (no regeneration drift)

---

### PR 5 — Remove legacy compatibility

**Deliverables (product + eng sign-off required):**

- Retire deprecated machine REST routes (`MACHINE_REST_LEGACY_ENABLED` removal timeline)
- Consolidate admin outbox paths (`/v1/admin/ops/*` → `/v1/admin/system/outbox` only) with deprecation window
- Remove duplicate proto under Postman suite; single generator source
- Remove type aliases from old `internal/app/*` shims
- Evaluate `proto/avf/v1/skeleton` retirement (audit P2.4)

**Risk:** High (external clients, production smoke)  
**Gate:** Full production Postman suite validation, staging smoke, field pilot checklist

---

## 5. Risk analysis

| Risk | Description | Mitigation |
|------|-------------|------------|
| **Import cycles** | Moving packages can create `commerce` ↔ `machine` ↔ `inventory` cycles when extracting ports late | Introduce interfaces in **consumer** package; use `internal/contracts` read DTOs; run `go test` + custom import linter each PR |
| **Broken migrations** | Accidental edit/move of `migrations/**` or desync of `db/schema/01_platform.sql` vs goose | **Do not move** migrations; PR checklist includes `scripts/ci/verify_migrations.sh`; single schema mirror update process |
| **Broken OpenAPI generation** | `docs/swagger` embed import path; `swagger_operations.go` ↔ `tools/build_openapi.py` registry drift | Never move embed without Go import update; always run `make swagger-check`; keep route doc registry in one file |
| **Broken Postman tests** | CI collections generated from OpenAPI + route inventory; full suite under `postman/suites/` | Run `make postman-check` and `python tools/check_postman_artifacts.py`; Newman E2E after path changes |
| **Broken deployment scripts** | `deployments/prod/scripts/*`, compose volume paths, env var names tied to process layout | No binary renames in P2; run `validate_release_assets.sh`, compose config, staging smoke |
| **Broken CI/CD** | Workflows grep fixed paths (`docs/operations/*` stubs, script locations) | Update workflow contract test (`scripts/ci/verify_workflow_contracts.sh`) in same PR as moves |
| **Hidden runtime config coupling** | `internal/config` flags gate NATS/MQTT/gRPC/Temporal together; bootstrap assumes optional wiring order | Document config dependency graph per context; add `cmd/cli -validate-config` tests when splitting bootstrap |
| **sqlc blast radius** | Single `internal/gen/db` means query renames affect all contexts | Adapter layer per context; avoid renaming SQL until adapters stable |
| **Temporal / workflow coupling** | `workfloworch` spans commerce + payments + manual review | Keep workflow definitions in commerce context ports; temporal-worker composition PR isolated |
| **Production inventory regression** | 325+ REST / gRPC / MQTT Postman production suite | Run `postman/suites/full-production-suite/validate_generated_assets.py` before merge |

---

## 6. Test plan

Run from repository root after **each PR** in the migration sequence.

### 6.1 Required (every PR)

| Step | Command | Pass criteria |
|------|---------|---------------|
| Format | `gofmt -l .` (empty) or `make fmt-check` | No diff |
| Unit + package tests | `go test ./...` | Exit 0 |
| Vet | `go vet ./...` | Exit 0 |
| Module sanity | `go mod tidy` + no unintended `go.sum` churn | Clean diff review |
| Short CI parity | `go test ./... -short` | Exit 0 |

### 6.2 Contract gates (PR 2+)

| Step | Command | Pass criteria |
|------|---------|---------------|
| API contract | `make api-contract-check` | sqlc + swagger + postman + proto + machine gRPC docs |
| Postman offline validation | `python tools/check_postman_artifacts.py` | OK |
| Workflow contracts | `make verify-workflows` | actionlint + script path contracts |

### 6.3 Integration tests (PR 3+)

| Step | Command | Prerequisites |
|------|---------|---------------|
| Postgres integration | `TEST_DATABASE_URL=... go test ./internal/modules/postgres/...` | Docker postgres + migrations |
| E2E correctness | `make test-e2e-local` or `go test ./internal/e2e/correctness/...` | `TEST_DATABASE_URL`, migrations applied |
| gRPC integration | `go test ./internal/grpcserver/...` | Same DB for integration-tagged tests |

### 6.4 Postman / Newman smoke (PR 4+, staging)

| Step | Command | Notes |
|------|---------|-------|
| Local REST Newman | `tests/e2e/postman/run-newman.sh` | Requires API on `POSTMAN_ENV` target |
| Full production suite generator | `python postman/suites/full-production-suite/validate_generated_assets.py` | Offline asset validation |
| Import smoke | Manual or documented import of `postman/collections/avf-vending-api.postman_collection.json` | Guard scripts + production mutation flags |

### 6.5 Docker & deploy dry-run (PR 4+)

| Step | Command | Pass criteria |
|------|---------|---------------|
| Go build all binaries | `go build ./cmd/...` | Exit 0 |
| Docker compose config | `docker compose -f deployments/docker/docker-compose.yml config` | Valid YAML |
| Local compose smoke | `make dev-up && make dev-migrate && make dev-test` (or documented equivalent) | Health endpoints pass |
| Prod compose config | `docker compose --env-file deployments/prod/.env.production.example -f deployments/prod/docker-compose.prod.yml config` | No errors |
| Release asset validation | `bash deployments/prod/scripts/validate_release_assets.sh deployments/prod/.env.production.example` | CI parity |
| Production dry-run | `make production-preflight` / `production-validate-env` when env available | Ops sign-off; **no prod mutations** |

### 6.6 Suggested CI matrix addition (future)

- **Import boundary job:** run custom linter script failing on forbidden edges (`transport → gen/db`, `domain → adapter`)
- **Per-context test job:** optional sharded `go test ./internal/catalog/...` for faster feedback after PR 3

---

## Appendix A — Recommended next PR sequence (summary)

| Order | PR | Summary | Est. blast radius |
|-------|-----|---------|-------------------|
| **1** | Package boundary docs | `bounded-contexts.md`, import rules, ADR index | Docs only |
| **2** | Pure helpers + port interfaces | Remove placeholders; add ports; no adapter moves | Low |
| **3a–3g** | Split domain packages (one context per sub-PR) | Catalog → commerce → machine → inventory → fleet → telemetry → audit/async | Medium each |
| **4a–4c** | Stabilize interfaces | Postgres adapter split; composition wiring; import linter | High |
| **5** | Legacy removal | Deprecated machine REST, outbox alias paths, proto dedup, app shims | High (external) |

**Start with PR 1 immediately.** Schedule PR 3a (catalog/media) first among splits—it has the clearest product boundary and supports offline cache work already documented in repo reports.

---

## Appendix B — References

- [Current architecture (as built)](../architecture/current-architecture.md)
- [Enterprise target model](../architecture/enterprise-target-model.md)
- [Transport boundary](../architecture/transport-boundary.md)
- [P0/P1/P2 implementation roadmap](../architecture/p0-p1-p2-implementation-roadmap.md)
- [Deep repository cleanup audit](../archive/cleanup/DEEP_REPO_CLEANUP_AUDIT.md)
- [API contract checks](../api/api-contract-checks.md)
