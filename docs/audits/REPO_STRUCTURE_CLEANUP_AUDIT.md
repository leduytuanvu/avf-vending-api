# Repository Structure Cleanup Audit

**Date:** 2026-05-20  
**Branch inspected:** working tree (production runs from `main`)  
**Scope:** Read-only inspection — no files modified, moved, or deleted during this audit.  
**Commands run:** `git status --short`, `git ls-files` (1,523 tracked files), directory inventory, `git grep` (TODO/FIXME/legacy/secret patterns), `go list ./...` (107 packages), `go test ./...` (PASS).

---

## Executive summary

The repository is a **Go 1.25 modular monolith** with seven production binaries, a well-structured `internal/` tree, and mature CI/deploy wiring. The **authoritative DB migration source of truth** is `migrations/*.sql` (4 goose files after production reset). sqlc reads `db/schema/01_platform.sql` + `db/queries/*.sql`.

Primary cleanup opportunities are **stale committed artifacts**, **local-only run output not gitignored**, **duplicate observability/Postman/doc trees**, and **orphan placeholder packages** — not core application code. All tests pass today.

| Metric | Value |
|--------|-------|
| Tracked files | 1,523 |
| Go packages (`go list ./...`) | 107 |
| Migration files (source of truth) | 4 |
| sqlc query files | 52 |
| `internal/app/*` domains | 41 |
| Untracked / dirty working-tree items | 14 paths (see §1) |

---

## 1. Current repository map

### 1.1 Top-level folders

| Path | Tracked files (approx.) | Role |
|------|-------------------------|------|
| `.github/` | 20 | GitHub Actions, CODEOWNERS, dependabot |
| `cmd/` | 7 | Service entrypoints (one `main.go` per binary) |
| `db/` | 53 | sqlc schema (`db/schema/`) + queries (`db/queries/`) |
| `deployments/` | 114 | Docker Compose, prod/staging VPS layouts, observability stacks |
| `docs/` | 185 | Architecture, API, runbooks, testing, swagger, postman (canonical) |
| `internal/` | ~750 | Application, platform, transport, generated db/grpc |
| `migration-evidence/` | 1 | Committed CI migration report JSON (stale — see §5) |
| `migrations/` | 4 | **Production goose migrations (DO NOT MOVE)** |
| `deployments/docker/observability/` | 28 | Local/dev observability configs + runbook markdown |
| `pkg/` | 1 | Placeholder package (`doc.go` only) |
| `postman/` | 61 tracked (+ large untracked tree) | Full production Postman suite + generators |
| `proto/` | 66 | Protobuf sources + committed `*.pb.go` stubs |
| `reports/` | 12 tracked (+ untracked audit dirs) | Phase/feature audit reports |
| `scripts/` | ~100 tracked (+ untracked `audit/`, `deployments/docker/observability/`) | CI, deploy, test, release, load scripts |
| `testdata/` | 14 | Telemetry JSON fixtures, api-contract readme |
| `tests/` | 83 | Python unit tests + bash E2E harness under `tests/e2e/` |
| `tools/` | ~30 | OpenAPI/Postman generators, loadtest, governance verifiers |

**Local-only / gitignored (present on disk, not tracked):**

| Path | Files (approx.) | Notes |
|------|-----------------|-------|
| `.e2e-runs/` | ~18,722 | E2E harness output — gitignored |
| `.test-runs/` | ~223 | PowerShell test output — gitignored |
| `bin/` | 6 | Compiled binaries — gitignored |
| `ci-reports/` | 1 | Local CI mirror output — gitignored |
| `.go-tmp/` | — | Go temp — not in `.gitignore` (needs review) |
| `tmp/` | 0 | Empty |
| `.cursor-api-e2e.log` | 20,518 lines | Untracked Cursor log |
| `.cursor-worker-e2e.log` | — | Untracked |
| `.cursor-test-file-list.txt` | — | Untracked |
| `.env` | 1,054 bytes | **Local secrets — gitignored, must not commit** |
| `repomix-output.xml` | — | gitignored repomix dump |

### 1.2 `cmd/` binaries

| Binary | Path | Purpose |
|--------|------|---------|
| REST API | `cmd/api/main.go` | HTTP `/v1`, optional gRPC listeners, Swagger mount |
| Worker | `cmd/worker/main.go` | Outbox/reliability/retention background ticks |
| MQTT ingest | `cmd/mqtt-ingest/main.go` | MQTT subscriber → Postgres |
| Reconciler | `cmd/reconciler/main.go` | Commerce/payment reconciliation |
| Temporal worker | `cmd/temporal-worker/main.go` | Workflow compensation/review |
| Outbox replay | `cmd/outbox-replay/main.go` | Operational outbox replay CLI |
| CLI | `cmd/cli/main.go` | `-validate-config`, `-version` |

Build targets: `Makefile` (`make build`, `make run-api`, per-service targets).

### 1.3 `internal/` packages (summary)

```
internal/
├── apierr/              # HTTP error envelope
├── app/                 # 41 domain packages (activation, auth, commerce, device, …)
├── bootstrap/           # Process wiring, graceful shutdown
├── config/              # Env-backed configuration + validation
├── domain/              # Domain types/ports (cash, commerce, fleet, operator, …)
├── e2e/correctness/     # Integration test scenarios (Postgres-required)
├── gen/
│   ├── avfinternalv1/   # Generated internal gRPC stubs (buf output)
│   └── db/              # sqlc-generated query code (~52 files)
├── grpcserver/          # Machine + internal gRPC servers
├── httpserver/          # Chi REST router, admin/operator/commerce routes
├── middleware/          # Request ID, etc.
├── migrations/          # capacity_migration_test.go only (not SQL migrations)
├── modules/postgres/    # Primary OLTP repository implementations
├── observability/       # Logging, tracing, Prometheus helpers
├── platform/            # auth, db, mqtt, nats, redis, objectstore, payments, temporal, …
├── repository/cash/     # Placeholder (doc.go only)
├── service/cash/        # Placeholder (doc.go only)
├── testfixtures/        # Integration seed helpers
└── version/             # Build version injection
```

### 1.4 Migrations

**Source of truth (production):** `migrations/`

| File | Purpose |
|------|---------|
| `00001_placeholder.sql` | Goose baseline placeholder |
| `00002_platform_schema.sql` | Squashed platform DDL |
| `00003_seed_dev.sql` | Dev seed data |
| `00004_product_media_offline_cache.sql` | Product media / offline cache |

**sqlc schema (not goose):** `db/schema/01_platform.sql` — referenced by `sqlc.yaml`, must stay aligned with `00002_platform_schema.sql`.

**Not migrations:** `internal/migrations/capacity_migration_test.go` — test that reads `migrations/00002_platform_schema.sql`.

### 1.5 Deployment files

```
deployments/
├── docker/                    # Local dependency stack (Postgres, Redis, NATS, EMQX, MinIO)
├── staging/                   # Staging VPS compose + scripts
├── prod/
│   ├── app-node/              # 2-VPS stateless app stack (primary production path)
│   ├── data-node/             # 2-VPS data/broker node
│   ├── observability/         # Production Grafana/Prometheus/Loki/Promtail
│   ├── legacy/                # Documented legacy single-host rollback path
│   ├── docker-compose.prod.yml
│   ├── docker-compose.observability.yml
│   ├── Dockerfile / Dockerfile.goose
│   └── scripts/               # release.sh, backup, smoke, rollback, …
└── loadtest/                  # Load-test env example
```

### 1.6 Scripts (grouped)

| Group | Path | CI/deploy usage |
|-------|------|-----------------|
| CI gates | `scripts/ci/*` | `make ci-gates`, `.github/workflows/ci.yml` |
| Deploy | `scripts/deploy/*`, `deployments/*/scripts/*` | Staging/prod deploy workflows |
| Release/security | `scripts/deploy/release/*`, `scripts/deploy/security/*` | build-push, security-release |
| DB | `scripts/db/*` | Backup evidence, migration preflight |
| Test/E2E | `scripts/test/*`, `tests/e2e/*` | Local + canary E2E |
| Load | `scripts/test/load/*`, `scripts/test/loadtest/*` | Makefile loadtest targets |
| Local (Windows) | `scripts/local/*.ps1` | Developer ergonomics |
| Smoke | `scripts/deploy/smoke/*`, `scripts/smoke_*.sh` | Post-deploy smoke |
| Top-level | `scripts/api-contract-check.sh`, `verify_enterprise_release.sh`, … | Makefile wrappers |

**Untracked (working tree):**
- `scripts/audit/single_scope_inventory.py`
- `scripts/db/verify_product_media_migration.sh`

### 1.7 Docs

```
docs/
├── api/                 # REST/gRPC/MQTT contract docs, examples
├── architecture/        # Current + target architecture
├── ci-cd/               # staging-production-gate.md (1 file)
├── cicd/                # CI_CD_FINAL_AUDIT.md (1 file) — naming split
├── contracts/           # JSON schemas, deployment-secrets-contract.yml
├── deployment/          # Environment matrix
├── local/               # Local gRPC testing
├── observability/       # Production metrics doc
├── operations/          # Release, backup, governance checklists
├── postman/             # **Canonical** generated Postman collections (CI-checked)
├── runbooks/            # ~70 operational runbooks
├── security/            # Supply chain pinning
├── swagger/             # Generated OpenAPI 3 + docs.go (CI-checked)
├── testing/             # E2E, Postman production guides
└── vi/                  # Vietnamese API guide
```

### 1.8 Tests

| Location | Type |
|----------|------|
| `internal/**/*_test.go` | Unit + integration (Postgres when `TEST_DATABASE_URL` set) |
| `internal/e2e/correctness/` | Named integration scenarios (`TestP06_*`, etc.) |
| `tests/e2e/` | Bash scenario harness (REST/gRPC/MQTT/full flows) |
| `tests/*.py` | Python tests for scripts/contracts |
| `tools/**/*_test.go` | Tool unit tests |

`go test ./...` — **PASS** (2026-05-20).

### 1.9 Generated artifacts (committed by design)

| Artifact | Generator | CI check |
|----------|-----------|----------|
| `internal/gen/db/*.sql.go` | sqlc (`make sqlc-check`) | `make api-contract-check` |
| `internal/gen/avfinternalv1/*.pb.go` | buf (`proto/buf.gen.avfinternal.yaml`) | `make proto-check` |
| `proto/**/*.pb.go` | buf (`proto/buf.gen.yaml`) | `make proto-check` |
| `docs/swagger/swagger.json`, `docs.go` | `tools/build_openapi.py` | `make swagger-check` |
| `docs/postman/*.json` | `tools/build_postman_collection.py` | `make postman-check` |

### 1.10 OpenAPI / Postman assets

| Location | Role | Canonical? |
|----------|------|------------|
| `docs/swagger/` | Embedded OpenAPI 3 served at `/swagger/doc.json` | **Yes (CI)** |
| `docs/postman/` | Generated REST Postman collection + env files | **Yes (CI)** |
| `postman/suites/full-production-suite/` | Full REST+gRPC+MQTT production verification suite | Supplementary (large) |
| `postman/collections/` (untracked) | Expanded collection tree with `.resources/` sidecars | **Untracked — do not commit blindly** |
| `postman/environments/`, `postman/specs/` (untracked) | Local Postman workspace exports | Untracked |
| `postman/suites/full-production-suite/*.zip` (untracked) | Packaged suite archives | Untracked generated zips |

---

## 2. Folder classification

| Folder | Classification | Rationale |
|--------|----------------|-----------|
| `cmd/` | **PRODUCTION CRITICAL** | All runtime binaries |
| `internal/` | **PRODUCTION CRITICAL** | Core application code |
| `migrations/` | **PRODUCTION CRITICAL** | Production DB source of truth — **immutable in cleanup** |
| `db/` | **PRODUCTION CRITICAL** | sqlc inputs tied to schema |
| `proto/` | **PRODUCTION CRITICAL** | gRPC contracts + generated stubs |
| `deployments/` | **PRODUCTION CRITICAL** | Docker, prod/staging topology |
| `.github/` | **PRODUCTION CRITICAL** | CI/CD pipelines |
| `scripts/ci/`, `scripts/deploy/`, `scripts/deploy/release/`, `scripts/deploy/security/` | **PRODUCTION CRITICAL** | Referenced by workflows and Makefile |
| `Makefile`, `go.mod`, `go.sum`, `sqlc.yaml` | **PRODUCTION CRITICAL** | Build/CI entrypoints |
| `docs/swagger/`, `docs/postman/` | **KEEP** (generated but CI-gated) | Contract artifacts |
| `docs/runbooks/`, `docs/api/`, `docs/architecture/` | **KEEP** | Operational knowledge |
| `tests/e2e/`, `internal/e2e/` | **KEEP** | Test harness |
| `testdata/` | **KEEP** | Fixture data |
| `tools/` | **KEEP** | Generators used by Makefile/CI |
| `postman/suites/full-production-suite/` | **KEEP** (with reorg) | Production verification assets; large but referenced |
| `deployments/docker/observability/` | **MOVE CANDIDATE** | Overlaps `deployments/prod/observability/` and `deployments/docker/` |
| `reports/` | **MOVE CANDIDATE** | Point-in-time audit outputs → `docs/audits/` or archive |
| `migration-evidence/` | **DELETE CANDIDATE** | Stale committed CI output |
| `pkg/` | **NEEDS REVIEW** | Empty placeholder — no imports |
| `internal/repository/cash/`, `internal/service/cash/` | **NEEDS REVIEW** | doc.go placeholders only |
| `bin/`, `ci-reports/`, `.e2e-runs/`, `.test-runs/` | **GENERATED / ARTIFACT** | gitignored local output |
| `.go-tmp/`, `tmp/` | **GENERATED / ARTIFACT** | `.go-tmp` not gitignored |
| `.cursor-*` logs | **DELETE CANDIDATE** (local) | IDE session logs |
| `repomix-output.xml` | **DELETE CANDIDATE** (local) | Code dump — gitignored |
| `secret-vars-scan.txt` | **DELETE CANDIDATE** | Local grep scan committed by mistake |
| `vending_schema.sql` | **DELETE CANDIDATE** | Stale schema commentary file |
| `docs/cicd/` + `docs/cicd/` | **NEEDS REVIEW** | Split naming for CI docs |
| `deployments/prod/legacy/` | **KEEP** | Documented rollback path — referenced by CI legacy-prod-asset-contract |
| `deployments/docker/docker-compose.yml.bak.local` | **DELETE CANDIDATE** | Tracked backup copy |
| `postman/collections/` (untracked) | **NEEDS REVIEW** | May duplicate `docs/postman/` + full suite |
| `scripts/test/load/` vs `scripts/test/loadtest/` | **NEEDS REVIEW** | Parallel load-test script trees |
| `proto/avf/v1/skeleton.*` | **NEEDS REVIEW** | Legacy skeleton proto — minimal references |

---

## 3. Delete candidates

### 3.1 `vending_schema.sql` (root)

| Field | Detail |
|-------|--------|
| **Reason unused** | Header states it mirrors `db/schema/01_platform.sql` but references **obsolete** migration filenames (`00005`–`00011`) that no longer exist after squashing to `00002_platform_schema.sql`. |
| **grep evidence** | Only self-reference in file header; `sqlc.yaml` points to `db/schema/01_platform.sql`, not this file. |
| **CI/deploy check** | Not referenced in `.github/workflows/*`, `Makefile`, or `scripts/ci/verify_migrations.sh`. |
| **Risk if deleted** | **Low** — may confuse developers who bookmarked it. |
| **Recommendation** | **Delete** (or replace with a one-line README pointer to `db/schema/` + `migrations/`). |

### 3.2 `migration-evidence/migration-safety-report.json`

| Field | Detail |
|-------|--------|
| **Reason unused** | Committed CI report lists **40+ migration files** (`00004_device_mqtt_ingest.sql` …) that are **not present** in current `migrations/` (only 4 files). `"blocked": true`, `"exit_code": 1`. |
| **grep evidence** | Referenced in `docs/runbooks/migration-safety.md`, `scripts/deploy/migration_preflight.sh`, `tools/verify_migrations.py` as an **example output path**, not as input. CI writes to `ci-reports/migration-safety-report.json` (gitignored). |
| **CI/deploy check** | `.github/workflows/ci.yml:164` writes to `ci-reports/`, not this path. |
| **Risk if deleted** | **Low** — stale misleading evidence. |
| **Recommendation** | **Delete** from repo; add `migration-evidence/` to `.gitignore` if directory kept for local use. |

### 3.3 `secret-vars-scan.txt` (root)

| Field | Detail |
|-------|--------|
| **Reason unused** | One-off ripgrep output of env var names across the repo (1036 lines). Not consumed by any script. |
| **grep evidence** | `git grep -l "secret-vars-scan"` → no references except `.gitignore` does **not** list it (file is tracked). |
| **CI/deploy check** | Not referenced in workflows or Makefile. |
| **Risk if deleted** | **None**. |
| **Recommendation** | **Delete**; add pattern to `.gitignore` (`*secret-vars-scan*`). |

### 3.4 `deployments/docker/docker-compose.yml.bak.local`

| Field | Detail |
|-------|--------|
| **Reason unused** | Local backup of compose file; no script references. |
| **grep evidence** | `git grep -l "docker-compose.yml.bak"` → no matches. |
| **CI/deploy check** | `ci.yml` validates `deployments/docker/docker-compose.yml`, not `.bak.local`. |
| **Risk if deleted** | **Low** — developer may lose local diff reference. |
| **Recommendation** | **Delete** from repo; keep local backups outside git. |

### 3.5 `repomix-output.xml` (local, gitignored)

| Field | Detail |
|-------|--------|
| **Reason unused** | Full-repo XML dump for LLM context. |
| **grep evidence** | Listed in `.gitignore:11,61`. |
| **Risk if deleted** | **None** — regenerable. |
| **Recommendation** | **Delete locally**; already correctly gitignored. |

### 3.6 `.cursor-api-e2e.log`, `.cursor-worker-e2e.log`, `.cursor-test-file-list.txt`

| Field | Detail |
|-------|--------|
| **Reason unused** | Cursor IDE session logs (20k+ lines). Untracked. |
| **grep evidence** | No references. |
| **Risk if deleted** | **None**. |
| **Recommendation** | **Delete locally**; add `.cursor-*.log` to `.gitignore`. |

### 3.7 `.e2e-runs/` (~18,722 files, gitignored)

| Field | Detail |
|-------|--------|
| **Reason unused** | Accumulated E2E harness output on disk. |
| **grep evidence** | `.gitignore:47`; referenced in `tests/e2e/README.md` as output dir. |
| **Risk if deleted** | **None** for repo; local evidence loss only. |
| **Recommendation** | **Delete locally** (P0 safe cleanup); already gitignored. |

### 3.8 `postman/suites/full-production-suite/avf_full_100_postman_suite.zip`, `avf_full_postman_suite.zip` (untracked)

| Field | Detail |
|-------|--------|
| **Reason unused** | Packaged archives duplicating JSON sources already in tree. |
| **grep evidence** | Untracked (`git status ??`). |
| **Risk if deleted** | **Low** — regenerate from suite scripts. |
| **Recommendation** | **Do not commit**; delete locally or gitignore `postman/**/*.zip`. |

### 3.9 `proto/avf/v1/skeleton.proto` + `skeleton.pb.go` + `skeleton_grpc.pb.go`

| Field | Detail |
|-------|--------|
| **Reason appears unused** | Skeleton/placeholder package; only referenced in `postman/suites/full-production-suite/manifest.json`. |
| **grep evidence** | No Go imports of `avf/v1/skeleton` in application code (`go list` includes package but no server mounts it). |
| **CI/deploy check** | Included in buf generation; removing requires proto-check update. |
| **Risk if deleted** | **Medium** — may break buf breaking-change baseline or manifest completeness counts. |
| **Recommendation** | **Needs manual review** before delete; defer to P2. |

### 3.10 `internal/repository/cash/`, `internal/service/cash/`

| Field | Detail |
|-------|--------|
| **Reason unused** | `doc.go` placeholders only; cash logic lives in `internal/modules/postgres` + `internal/gen/db/cash.sql.go`. |
| **grep evidence** | `git grep` for imports → **no matches**. |
| **Risk if deleted** | **Low** — reserved for future layering. |
| **Recommendation** | **Keep for now** (P2 architecture decision) or delete empty packages in P2 refactor. |

### 3.11 `pkg/` (root)

| Field | Detail |
|-------|--------|
| **Reason unused** | Single `doc.go` stating "reserved for external consumers"; zero implementation. |
| **grep evidence** | `go list` includes package; no external imports in repo. |
| **Risk if deleted** | **Low** — conventional Go layout placeholder. |
| **Recommendation** | **Keep** until a public API surface is defined (P2). |

---

## 4. Move candidates

### 4.1 `deployments/docker/observability/` → consolidate under `deployments/` or `docs/operations/`

| Field | Detail |
|-------|--------|
| **Current** | `deployments/docker/observability/` (Grafana/Loki/Prometheus/OTel configs + `METRICS.md`, `RUNBOOK.md`, …) |
| **Proposed** | `deployments/local/observability/` for configs; move markdown to `docs/operations/` or merge into `docs/runbooks/` |
| **Reason** | Duplicates dashboards with `deployments/prod/observability/grafana/` (similar but not identical files). `deployments/docker/docker-compose.yml` references `deployments/docker/observability/` paths per grep. |
| **References** | `deployments/docker/docker-compose.yml`, `deployments/docker/README.md`, `deployments/docker/observability/METRICS.md`, `README.md` (links `deployments/docker/observability/ANALYTICS_CLICKHOUSE.md`) |
| **Import/path updates** | Compose volume mounts, README links, runbook cross-refs |
| **Risk** | **Medium** — breaks local `docker compose` observability profile if paths wrong |
| **Tests required** | `docker compose -f deployments/docker/docker-compose.yml config`; local metrics scrape smoke |

### 4.2 `docs/postman/` → `postman/collections/` (canonical REST)

| Field | Detail |
|-------|--------|
| **Current** | CI-canonical Postman JSON in `docs/postman/` |
| **Proposed** | `postman/collections/` + `postman/environments/` with `docs/postman/` as symlink or README pointer |
| **Reason** | Untracked `postman/collections/` already exists; operators expect Postman under `postman/` |
| **References** | `Makefile` (`postman-check` diffs `docs/postman/`), `tools/build_postman_collection.py`, `scripts/check_postman_artifacts.sh`, 15+ doc/runbook links |
| **Import/path updates** | Makefile, CI workflow steps, all `docs/postman` links, `tests/e2e/postman/run-newman.sh` |
| **Risk** | **High** — breaks `make postman-check` and CI until all paths updated atomically |
| **Tests required** | `make postman-check`, `tests/e2e/postman/run-newman.sh` |

### 4.3 `reports/` → `docs/audits/` or `docs/reports/`

| Field | Detail |
|-------|--------|
| **Current** | `docs/reports/product-media-offline-cache/`, `docs/reports/test/`, untracked `reports/final-*-audit/` |
| **Proposed** | `docs/audits/<feature>/` |
| **Reason** | Audit outputs are documentation, not runtime assets |
| **References** | Cross-links from `docs/reports/product-media-offline-cache/*.md` to each other; some README links |
| **Risk** | **Low** — link rot in markdown only |
| **Tests required** | Markdown link check (optional) |

### 4.4 `docs/cicd/` + `docs/cicd/` → single `docs/cicd/`

| Field | Detail |
|-------|--------|
| **Current** | Two folders differing by hyphen |
| **Proposed** | `docs/cicd/` containing both files |
| **Reason** | Naming inconsistency (`ci-cd` vs `cicd`) |
| **References** | `README.md` links `docs/cicd/CI_CD_FINAL_AUDIT.md`; `docs/cicd/staging-production-gate.md` |
| **Risk** | **Low** |
| **Tests required** | Link grep |

### 4.5 `scripts/test/load/` + `scripts/test/loadtest/` → `scripts/test/load/`

| Field | Detail |
|-------|--------|
| **Current** | Two parallel load-test script trees |
| **Proposed** | Single `scripts/test/load/` with subdirs `k6/`, `shell/` |
| **Reason** | `Makefile` uses `tools/loadtest` for Go load tool but both script dirs for shell/k6 |
| **References** | `Makefile` loadtest targets, `docs/testing/load-test.md` |
| **Risk** | **Medium** — workflow/script path breaks |
| **Tests required** | `make loadtest-small` (if env available) |

### 4.6 `tools/generate_release_manifest.py` vs `scripts/deploy/release/generate_release_manifest.sh`

| Field | Detail |
|-------|--------|
| **Current** | Python tool exists; shell wrapper is what CI/release uses |
| **Proposed** | Keep shell wrapper in `scripts/deploy/release/`; ensure Python lives in `tools/` only |
| **Reason** | Minor duplication; shell script is canonical per grep |
| **Risk** | **Low** |
| **Recommendation** | Document canonical entrypoint in `scripts/deploy/release/README` (P1) |

### 4.7 Untracked `scripts/audit/`, `scripts/deployments/docker/observability/` → `scripts/ci/` or `tools/`

| Field | Detail |
|-------|--------|
| **Files** | `single_scope_inventory.py`, `verify_product_media_migration.sh` |
| **Proposed** | `tools/audit/` or `scripts/ci/` after review |
| **Risk** | **Low** — currently untracked |
| **Recommendation** | Review content, then commit to appropriate tree or discard |

---

## 5. Duplicate / stale artifacts

### 5.1 Migration folder duplication (not duplicate SQL trees)

| Item | Status |
|------|--------|
| `migrations/*.sql` | **Authoritative** for goose/production |
| `db/schema/01_platform.sql` | **Authoritative** for sqlc — must mirror squashed migration content |
| `vending_schema.sql` | **Stale** commentary — references removed migration numbers |
| `migration-evidence/migration-safety-report.json` | **Stale** — lists 40+ files; repo has 4 |
| `internal/migrations/` | **Not SQL** — Go test only |

**Action:** Do not create second migration folders. Keep `db/schema` + `migrations/` in sync via existing `scripts/ci/verify_migrations.sh`.

### 5.2 Backup / temp files

| Path | Status |
|------|--------|
| `deployments/docker/docker-compose.yml.bak.local` | **Tracked backup — delete candidate** |
| `*.bak`, `*.old`, `*.tmp`, `*.orig` elsewhere | **None found** in tracked tree |

### 5.3 Old reports / logs

| Path | Notes |
|------|-------|
| `docs/reports/product-media-offline-cache/*` | Phase audit trail — some references outdated migration count |
| `docs/reports/final-gate-audit/`, `docs/reports/final-single-scope-audit/` | **Untracked** local audit outputs |
| `.cursor-api-e2e.log` | 20,518-line local log |
| `.e2e-runs/*` | 18k+ local E2E artifacts |

### 5.4 Duplicate observability configs

| Location A | Location B | Relationship |
|------------|------------|--------------|
| `deployments/docker/observability/grafana/provisioning/dashboards/json/*.json` | `deployments/prod/observability/grafana/.../*.json` | **Similar dashboards, not byte-identical** (fc shows diffs) |
| `deployments/docker/observability/prometheus/prometheus.yml` | `deployments/prod/observability/prometheus/prometheus.yml` | Parallel configs for local vs prod |
| `deployments/docker/observability/loki/config.yml` | `deployments/prod/observability/loki/config.yml` | Parallel |

**Recommendation:** Treat `deployments/prod/observability/` as production source; `deployments/docker/observability/` as local dev mirror — document relationship, don't delete either without compose audit.

### 5.5 Duplicate Postman / OpenAPI

| Item | Issue |
|------|-------|
| `docs/postman/` vs `postman/suites/full-production-suite/` | **Different purposes** — CI REST collection vs full REST+gRPC+MQTT suite |
| `postman/suites/full-production-suite/grpc/proto/` | **Duplicate** of `proto/` tree for Postman import convenience |
| Untracked `postman/collections/` | Large expanded tree — likely Postman Desktop export; **do not commit without audit** |
| `docs/testing/05_PRODUCTION_TEST_EXECUTION_ORDER.md` vs `docs/testing/05_PRODUCTION_TEST_EXECUTION_ORDER.md` | **Duplicate doc** |

### 5.6 Duplicate scripts

| Scripts | Notes |
|---------|-------|
| `scripts/test/load/` vs `scripts/test/loadtest/` | Overlapping load-test runners |
| `deployments/prod/app-node/scripts/healthcheck_app_node.sh` vs `deployments/prod/shared/scripts/healthcheck_app_node.sh` | Intentional shared copies for node layouts |
| `tools/check_postman_artifacts.py` vs `scripts/check_postman_artifacts.sh` | Shell wrapper → Python (OK) |

### 5.7 Generated artifacts that should be gitignored (but are intentionally committed)

These **must stay committed** per current CI design:

- `docs/swagger/*`
- `docs/postman/*`
- `internal/gen/db/*`
- `internal/gen/avfinternalv1/*`
- `proto/**/*.pb.go`

**Should be gitignored (already or proposed):**

- `ci-reports/`, `bin/`, `.e2e-runs/`, `.test-runs/`, `repomix-output.xml`
- **Proposed additions:** `.cursor-*.log`, `.go-tmp/`, `postman/**/*.zip`, `migration-evidence/*.json`

---

## 6. Secret risk audit

**Rule:** No secret values printed below — paths and risk levels only.

| Path | Finding | Risk |
|------|---------|------|
| `.env` | Present locally (1,054 bytes), **gitignored** | **HIGH (local only)** — contains real dev credentials; never commit |
| `.env.example`, `.env.local.example`, `.env.staging.example`, `.env.production.example` | Placeholder `DATABASE_URL=postgres://postgres:postgres@...` | **LOW** — documented examples; gitleaks allowlisted |
| `deployments/prod/.env.production.example`, `deployments/staging/.env.staging.example`, node examples | `${PRODUCTION_DATABASE_URL}` substitution placeholders | **LOW** |
| `deployments/docker/docker-compose.yml` | Local dev Postgres/MinIO defaults | **LOW** — gitleaks allowlisted |
| `deployments/prod/emqx/certs/`, `deployments/staging/emqx/certs/` | `.gitignore` for `*.pem`, `*.key`; README only in repo | **LOW** |
| `deployments/docker/observability/private/` | gitignored | **LOW** if stays ignored |
| `secret-vars-scan.txt` | Tracked grep dump of env var **names** and example lines from `.env.example` | **LOW-MEDIUM** — unnecessary exposure surface; delete |
| `.github/workflows/*` | `${{ secrets.* }}` references only — no literal secrets | **LOW** |
| `docs/postman/*.json` | Checked by `tools/check_postman_artifacts.py` for secret-like content | **LOW** if CI gate passes |
| `postman/suites/full-production-suite/*.json` | Production test templates — may contain `{{variable}}` placeholders | **LOW** — review before external share |
| `tests/e2e/.env.example` | Example tokens/URLs | **LOW** |
| `.gitleaks.toml` | Allowlists for example files + docker compose | **GOOD** — active secret scanning config |
| `.cursor-api-e2e.log` | May contain request URLs/tokens from E2E runs | **MEDIUM (local)** — delete + gitignore |
| DB dumps | `deployments/prod/**/*.sql.gz`, `*.dump` gitignored | **LOW** in repo |
| JWT/RSA keys | No `BEGIN RSA` or `BEGIN OPENSSH` in tracked source (grep clean on code) | **LOW** |

**Immediate actions (no code changes in this phase):**
1. Confirm `.env` never staged (`git check-ignore -v .env` → OK).
2. Delete `secret-vars-scan.txt` in Phase 1.
3. Add `.cursor-*.log` to `.gitignore` in Phase 1.

---

## 7. Proposed final repo structure

Do **not** implement in this phase. Target layout after P1/P2:

```
avf-vending-api/
├── cmd/                          # 7 binaries (unchanged)
├── internal/                     # app + platform + gen (unchanged)
├── pkg/                          # future public API surface (or remove if unused)
├── proto/                        # buf sources + committed pb.go
├── db/
│   ├── schema/                   # sqlc DDL mirror of squashed migration
│   └── queries/                  # sqlc queries
├── migrations/                   # goose SQL — PRODUCTION SOURCE OF TRUTH (untouched)
├── deployments/
│   ├── docker/                   # local stack
│   ├── staging/
│   └── prod/                     # app-node, data-node, observability, legacy
├── scripts/
│   ├── ci/                       # CI gates (unchanged)
│   ├── deploy/                   # deploy/smoke wrappers
│   ├── release/                  # release evidence
│   ├── security/                 # security verdict
│   ├── test/                     # coverage/audit scripts
│   ├── load/                     # merged load + loadtest shell scripts
│   └── local/                    # Windows PS helpers
├── tools/                        # Python/Go generators (openapi, postman, governance)
├── tests/
│   ├── e2e/                      # bash E2E harness
│   └── *.py                      # script contract tests
├── testdata/                     # JSON fixtures
├── docs/
│   ├── api/
│   ├── architecture/
│   ├── audits/                   # ← consolidated from reports/ + audit outputs
│   ├── cicd/                     # ← merged ci-cd + cicd
│   ├── operations/
│   ├── runbooks/
│   ├── swagger/                  # generated OpenAPI (CI)
│   └── testing/
├── postman/
│   ├── collections/              # ← canonical REST (from docs/postman)
│   ├── environments/
│   └── full-production-suite/    # REST+gRPC+MQTT production suite
├── api/                          # OPTIONAL alias/symlink docs only → docs/swagger
├── Makefile
├── go.mod
├── sqlc.yaml
└── README.md
```

**Explicit non-goals for structure change:**
- Do not move `migrations/**`
- Do not move `.github/workflows/**`, `deployments/**` prod scripts, Dockerfiles
- Do not split `internal/app` monolith in P1

---

## 8. P0 / P1 / P2 cleanup plan

### P0 — Safe cleanup only (no production path changes)

| # | Action | Risk |
|---|--------|------|
| 1 | Delete local `.e2e-runs/`, `.test-runs/` contents (gitignored) | None |
| 2 | Delete local `.cursor-*.log`, `.cursor-test-file-list.txt` | None |
| 3 | Delete local `repomix-output.xml` if present | None |
| 4 | Delete untracked `postman/suites/full-production-suite/*.zip` | None |
| 5 | **Do not commit** untracked `postman/collections/` until deduped vs `docs/postman/` | — |
| 6 | Verify `.env` remains gitignored before any bulk add | Critical |

**Tracked-file deletes (require PR, still low risk):**
| # | File | Rationale |
|---|------|-----------|
| 7 | `secret-vars-scan.txt` | Accidental scan output |
| 8 | `vending_schema.sql` | Stale, misleading |
| 9 | `migration-evidence/migration-safety-report.json` | Stale CI artifact |
| 10 | `deployments/docker/docker-compose.yml.bak.local` | Local backup in git |

**Tests after P0 tracked deletes:** `make ci-gates` (or `make check-migrations` + `go test ./...`).

### P1 — Docs / scripts / generated-artifacts organization

| # | Action |
|---|--------|
| 1 | Extend `.gitignore`: `.cursor-*.log`, `.go-tmp/`, `postman/**/*.zip`, `migration-evidence/*.json` |
| 2 | Move `reports/*` → `docs/audits/` with redirect README at old paths |
| 3 | Merge `docs/cicd/` into `docs/cicd/` |
| 4 | Document canonical script entrypoints (`scripts/deploy/release/README`, load test index) |
| 5 | Consolidate duplicate `05_PRODUCTION_TEST_EXECUTION_ORDER.md` (keep one canonical copy) |
| 6 | Review untracked `scripts/audit/`, `scripts/deployments/docker/observability/` — commit to `tools/` or discard |
| 7 | Add `docs/audits/README.md` index (this file becomes first entry) |
| 8 | Refresh stale references in `docs/reports/product-media-offline-cache/` that cite old migration counts |

**Tests:** `make ci-gates`, `make verify-workflows`, markdown link spot-check.

### P2 — Deeper architecture refactor (proposal only)

| # | Proposal |
|---|----------|
| 1 | Merge `deployments/docker/observability/` observability into `deployments/docker/observability/` with compose path updates |
| 2 | Relocate `docs/postman/` → `postman/collections/` atomically with Makefile/CI updates |
| 3 | Remove or implement `internal/repository/cash`, `internal/service/cash`, `pkg/` |
| 4 | Evaluate retiring `proto/avf/v1/skeleton` from buf generation |
| 5 | Merge `scripts/test/load/` + `scripts/test/loadtest/` |
| 6 | Deduplicate `postman/suites/full-production-suite/grpc/proto/` — generate from root `proto/` at build time |
| 7 | Consider gitignoring `postman/suites/full-production-suite/grpc/proto/` copies |
| 8 | Strangler cleanup: document deprecation timeline for `deployments/prod/legacy/` single-host path |

**Tests for P2:** Full `make ci-full`, E2E harness, staging smoke, proto-check, postman-check.

---

## Appendix A — Working tree status (audit snapshot)

```
 M docs/runbooks/README.md
?? .cursor-api-e2e.log
?? .cursor-test-file-list.txt
?? .cursor-worker-e2e.log
?? docs/runbooks/product-media-offline-cache-production-migration.md
?? postman/collections/
?? postman/environments/
?? postman/suites/full-production-suite/avf_full_100_postman_suite.zip
?? postman/suites/full-production-suite/avf_full_postman_suite.zip
?? postman/specs/
?? docs/reports/final-gate-audit/
?? docs/reports/final-single-scope-audit/
?? docs/reports/product-media-offline-cache/server-migration-verification-20260519.md
?? docs/reports/product-media-offline-cache/server-migration-verification-template.md
?? scripts/audit/
?? scripts/deployments/docker/observability/
```

---

## Appendix B — Must not touch (hard constraints)

| Path / asset | Reason |
|--------------|--------|
| `migrations/**` | Production DB source of truth |
| `.github/workflows/**` | CI/CD pipelines |
| `deployments/**` (except `.bak.local` delete candidate) | Production/staging deployment |
| `Dockerfile*`, `docker-compose*` (prod/staging) | Image build and orchestration |
| Production env templates (`*.example` under deployments) | Operator contract |
| `docs/swagger/**`, OpenAPI generation pipeline | API contract + Postman generation |
| `docs/postman/**` (until P2 atomic move) | CI `make postman-check` |
| `scripts/ci/**`, `scripts/deploy/**`, `scripts/deploy/release/**` used by workflows | CI/CD scripts |
| `internal/gen/db/**` | sqlc output — regenerate, don't hand-edit |
| `proto/**` (sources + pb.go) | gRPC contract |
| `Makefile`, `go.mod`, `sqlc.yaml` | Build/CI entrypoints |
| Local `.env` | Real credentials — never commit or print |

---

## Appendix C — `go list ./...` package inventory

107 packages including: 7 `cmd/*`, 41 `internal/app/*`, `internal/bootstrap`, `internal/config`, 8 `internal/domain/*`, `internal/e2e/correctness`, `internal/gen/avfinternalv1`, `internal/gen/db`, `internal/grpcserver`, `internal/httpserver`, `internal/modules/postgres`, 15+ `internal/platform/*`, `docs/swagger`, `proto/avf/machine/v1`, `proto/avf/internal/v1`, `proto/avf/v1`, `tools/loadtest`, `tools/dev-bcrypt`, `tools/telemetry-contract`.

---

*End of audit. No repository files were modified except creation of this document.*
