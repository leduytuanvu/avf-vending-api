# Production release readiness

This runbook defines **what must pass** before calling a release **pilot-safe** vs **fleet-scale ready**, and how **storm evidence** blocks scale-up without proof.

## P2 field documentation pack (100–1000 machines)

**Do not** confuse static CI verify with field proof. For **go-live evidence** and reproducible operator steps, use this table as the **single intake path**:

| Artifact | Path | Owner (typical) |
| -------- | ---- | --------------- |
| Normative production integration contract | [`../architecture/production-final-contract.md`](../architecture/production-final-contract.md) | TPM / all client teams |
| Field test matrix (**Case ID … Owner** columns) | [`../testing/field-test-cases.md`](../testing/field-test-cases.md) | Field lead |
| Android kiosk implementation checklist | [`../api/kiosk-app-implementation-checklist.md`](../api/kiosk-app-implementation-checklist.md) | Android lead |
| Kiosk narrative flow | [`../api/kiosk-app-flow.md`](../api/kiosk-app-flow.md) | Android + QA |
| **Post-deploy machine** widen | [`../operations/field-rollout-checklist.md`](../operations/field-rollout-checklist.md) | Field / SRE |
| **Pre-deploy GitHub** checklist | [`../operations/production-release-checklist.md`](../operations/production-release-checklist.md) | Release manager |
| Automated **GET-only** prod smoke | [`../operations/production-smoke-tests.md`](../operations/production-smoke-tests.md) | CI/CD — **not** a substitute for PSP/vend hardware tests |
| Mutating smoke **commands** | [`field-smoke-tests.md`](field-smoke-tests.md) | Operator (staging/pilot) |

**Topology:** Production **2-VPS rolling** + **managed** PostgreSQL / Redis / MQTT / object storage (**not** single-host Compose primary) — **[`production-2-vps.md`](production-2-vps.md)**, **[`production-cutover-rollback.md`](production-cutover-rollback.md)**. Legacy Compose is **rehearsal / emergency** only.

## Databases and environment separation (non-negotiable)

- **Production** must use a **dedicated** production database (`PRODUCTION_DATABASE_URL` / your vault); it must **never** be the same connection string or host as **staging** (`STAGING_DATABASE_URL`). Run `bash scripts/verify_database_environment.sh` with `APP_ENV=production` before one-off or scripted migrations; `deployments/prod/scripts/release.sh` and app-node `release_app_node.sh` do this before Goose when migrations run.
- **Promotion:** Prefer deploying a **digest that already passed** staging and your security/verify gates, not a fresh image of the same commit without staging validation, when your org’s workflow enforces that contract.
- Full narrative: [environment-strategy.md](./environment-strategy.md) and [docs/deployment/environments.md](../deployment/environments.md).

## Command: static enterprise verification

From the repository root:

```bash
make verify-enterprise-release
```

Equivalent:

```bash
bash scripts/verify_enterprise_release.sh
```

The command exits **non-zero** on any failed phase.

### What it runs (phase order)

1. **`go test ./...`** — full test suite (set `TEST_DATABASE_URL` when Postgres-backed integration tests should run).
2. **`make swagger`** then **`make swagger-check`** — regenerate OpenAPI and fail if `docs/swagger/` is out of date.
3. **`bash -n`** — every `*.sh` under `scripts/` and `deployments/`.
4. **Docker Compose config** — offline `docker compose … config` using **example** env files (`deployments/prod/app-node/.env.app-node.example`, data-node example, legacy prod example when present). Set `VERIFY_ENTERPRISE_SKIP_DOCKER=1` only for partial local runs (not a scale sign-off).
5. **OpenAPI release checks** — `tools/openapi_verify_release.py`: production + local dev servers first/second, **required P0 paths** present, no **planned-only** paths in Swagger, JSON **POST/PUT/PATCH** bodies include examples, **Bearer** on protected `/v1` routes (except login, refresh, activation claim, PSP webhook), **2xx + error** response examples, no secret-like **examples**.
6. **Stale P0 docs** — `scripts/check_stale_p0_docs.sh` rejects docs that claim P0 HTTP surfaces are still unmerged/unmounted (see script for exclusions).
7. **Secret heuristics** — deployment `*.example` / `.env.*.example` files; plus `docs/` and `testdata/` (excludes generated `docs/swagger/swagger.json` from JWT-pattern scan). Blocks obvious live keys and JWT-shaped blobs in prose.
8. **YAML parse (optional)** — if `python3` + PyYAML available, parses `deployments/**/*.yml|yaml`.

Phases print `===` headers for CI logs.

### Windows / policy notes

- Requires **bash** (Git Bash / WSL). **Make** optional if `python3` is on PATH (script falls back to `python3 tools/build_openapi.py` + `git diff` for Swagger drift).
- Some Windows **Application Control** policies block `go test` temp binaries — run `make verify-enterprise-release` on **Linux CI** (`ubuntu-latest`) if local policy blocks tests; see [`.github/workflows/enterprise-release-verify.yml`](../../.github/workflows/enterprise-release-verify.yml).

### Optional skips (debug / laptop only)

| Variable | Effect |
| -------- | ------ |
| `VERIFY_ENTERPRISE_SKIP_DOCKER=1` | Skip Compose config. |
| `VERIFY_ENTERPRISE_SKIP_YAML=1` | Skip YAML parse. |
| `VERIFY_ENTERPRISE_SKIP_GO=1` | Skip `go test` (**not** a release sign-off). |

### Machine-readable verify result (evidence pack)

For CI or attach to a release ticket:

```bash
bash deployments/prod/scripts/emit_verify_enterprise_result_json.sh ./evidence/verify-result.json
```

---

## Pilot vs scale: policy (non-negotiable wording)

| Tier | Storm evidence | Claim |
| --- | --- | --- |
| **Pilot** | **Not required** for the static repo gate | You may deploy a **pilot** when `make verify-enterprise-release` passes, live smoke checks pass, and required **HTTPApplication** services are wired for your environment (activation, commerce, telemetry store, etc.). Further HTTP enhancements are tracked in [roadmap.md](../api/roadmap.md) (must **not** appear in OpenAPI until implemented). |
| **scale-100** | **Required** — minimum scenario **100×100** PASS | Do **not** claim **100-machine production readiness** without matching storm evidence and monitoring readiness. |
| **scale-500** | **Required** — minimum scenario **500×200** PASS | Same. |
| **scale-1000** | **Required** — minimum scenario **1000×500** PASS | **No claim of 1000-machine readiness without a PASS on 1000×500** (and CI gate / manifest alignment). Partial runs (e.g. only 100×100) are **insufficient** for `scale-1000`. |

Scenario naming and production workflow inputs: [telemetry-production-rollout.md](./telemetry-production-rollout.md#fleet-scale-storm-gate).

---

## Required telemetry storm JSON fields (scale tiers)

When `fleet_scale_target` is **scale-100**, **scale-500**, or **scale-1000**, the storm artifact consumed by CI / `build_release_evidence_pack.sh` must satisfy **`validate_production_scale_storm_evidence.py`** (see `deployments/prod/shared/scripts/validate_production_scale_storm_evidence.py`), including:

| Field | Requirement |
| --- | --- |
| `machine_count` / `events_per_machine` | At least **100×100**, **500×200**, or **1000×500** respectively |
| `critical_expected` / `critical_accepted` | **Present** (ingest accounting); `critical_accepted` is the accepted critical count (see `telemetry_storm_load_test.sh` JSON) |
| `completed_at_utc` | Non-empty ISO-8601 UTC; must be within `STORM_EVIDENCE_MAX_AGE_DAYS` |
| `critical_lost` | **0** |
| `duplicate_critical_effects` | **0** (must be present) |
| `db_pool_result` | **`pass`** |
| `health_result` | **`pass`** |
| `restart_result` | **`pass`** |
| `final_result` | **`pass`** (or suite aggregate per staging script contract) |
| `execute_load_test` | **true** (not a dry-run certification) |
| `dry_run` | **false** |

**Evidence pack:** `deployments/prod/scripts/build_release_evidence_pack.sh` applies the same strict storm validation for non-pilot tiers (configurable `STORM_EVIDENCE_MAX_AGE_DAYS`, default **30** days for ticket assembly).

---

## Rollout guide: 100 / 500 / 1000 machines

1. Complete the **pilot** checklist on the release commit.
2. Run **staging storm** to the minimum dimension for the target tier (100×100, 500×200, 1000×500) — workflows: [`.github/workflows/telemetry-storm-staging.yml`](../../.github/workflows/telemetry-storm-staging.yml).
3. Capture **monitoring readiness** JSON (`check_monitoring_readiness.sh` or equivalent) with `final_result: pass`.
4. Promote via [`.github/workflows/deploy-prod.yml`](../../.github/workflows/deploy-prod.yml) (workflow **Deploy Production**). **Do not** use [`deploy-production.yml`](../../.github/workflows/deploy-production.yml) for rollouts — that file is a no-op **pointer** only. Set manifest `fleet_scale_target` for the tier **and** attach storm evidence for non-pilot.
5. Assemble **`build_release_evidence_pack.sh`** with digest-pinned manifest, verify JSON, monitoring JSON (if not pilot), storm JSON (required for scale tiers), and non-empty `KNOWN_RISKS_PATH`.

Do **not** mark **scale-1000** internally without a **1000×500** PASS and the field table above.

---

## Checklist: pilot deploy

Use when `fleet_scale_target=pilot` (or equivalent org policy).

- [ ] `make verify-enterprise-release` passes on the **release commit** (or CI workflow green).
- [ ] Images are **digest-pinned** per your deploy policy; migrations planned.
- [ ] **Smoke:** `/health/live`, `/health/ready`, `/version` on API; critical `/v1` paths per product.
- [ ] **Secrets:** only placeholders in repo examples; production secrets in orchestrator / vault — not in git.
- [ ] **OpenAPI:** operators know Swagger is documentation-only; `/v1` still requires Bearer (except login/refresh/webhook).
- [ ] **Feature wiring:** confirm activation, sale catalog, telemetry reconcile, and commerce refund/cancel routes are **mounted** (non-nil deps) or intentionally disabled for the pilot slice.
- [ ] **Incident path:** on-call and rollback steps reviewed ([production-rollback.md](./production-rollback.md), [production-2-vps.md](./production-2-vps.md)).

**Storm suite:** optional for pilot; if you attach storm artifacts to internal records, they must still show **`final_result: pass`** if referenced.

---

## Checklist: scale-100

- [ ] All **pilot** items complete for the target release.
- [ ] **Monitoring readiness:** `deployments/prod/scripts/check_monitoring_readiness.sh` → `final_result: pass` (see [production-observability-alerts.md](./production-observability-alerts.md#monitoring-readiness-check)).
- [ ] **Storm evidence:** **`100×100`** scenario PASS; artifact consumable by production workflow (see [telemetry-production-rollout.md](./telemetry-production-rollout.md)).
- [ ] **JetStream / Postgres** sizing reviewed for ~100 devices ([telemetry-jetstream-resilience.md](./telemetry-jetstream-resilience.md), [production-2-vps.md](./production-2-vps.md)).
- [ ] **Deploy manifest** `fleet_scale_target` (or org equivalent) matches **scale-100** evidence strength.

---

## Checklist: scale-500

- [ ] All **scale-100** items satisfied for the new target.
- [ ] **Storm evidence:** **`500×200`** scenario PASS (minimum machine/event counts per [fleet-scale storm gate](./telemetry-production-rollout.md#fleet-scale-storm-gate)).
- [ ] Capacity review: connection limits, pool sizes, `TELEMETRY_STREAM_MAX_BYTES`, worker concurrency — updated with evidence.
- [ ] **Second app node / split topology** (if used): `validate_production_deploy_inputs.sh` and pooler headroom verified.

---

## Checklist: scale-1000

- [ ] All **scale-500** items satisfied.
- [ ] **Storm evidence:** **`1000×500`** scenario PASS — **mandatory**; weaker scenarios alone **do not** authorize **scale-1000**.
- [ ] **No silent claim:** internal comms and manifests must not say “1000 ready” without this artifact set.
- [ ] **Observability:** alerts and dashboards exercised under load; reconnect-storm runbook understood ([telemetry-production-rollout.md](./telemetry-production-rollout.md#offline-replay-storm-prevention)).

---

## Required storm evidence (summary)

| `fleet_scale_target` | Minimum `machine_count` | Minimum `events_per_machine` | Typical artifact |
| --- | --- | --- | --- |
| `pilot` | _(none required)_ | _(none required)_ | — |
| `scale-100` | 100 | 100 | `telemetry-storm-result-100x100.json` (or suite summary) |
| `scale-500` | 500 | 200 | `telemetry-storm-result-500x200.json` |
| `scale-1000` | 1000 | 500 | `telemetry-storm-result-1000x500.json` |

**How to produce:** staging suite `deployments/prod/scripts/run_staging_telemetry_storm_suite.sh` or GitHub Actions [`.github/workflows/telemetry-storm-staging.yml`](../../.github/workflows/telemetry-storm-staging.yml). Details: [telemetry-production-rollout.md](./telemetry-production-rollout.md#staging-storm-evidence-suite-scale-100--500--1000).

---

## Production workflow options (choose explicitly)

Document **which** path the org uses on the release ticket.

| Option | When to use | Evidence |
| --- | --- | --- |
| **A. `deploy-prod.yml` only (Deploy Production)** | Primary and **only** GitHub deploy/rollback; digest-pinned images; `fleet_scale_target` in manifest. **`deploy-production.yml` is a legacy pointer and does not deploy.** | Successful workflow run + **`production-deployment-manifest.json`**; for scale tiers, storm artifact per [telemetry-production-rollout.md](./telemetry-production-rollout.md#production-ci-fleet-scale-gate-deploy-prodyml) |
| **B. Manual / out-of-band deploy** | Bastion-only or customer CI | Same **logical** gates: static verify JSON, monitoring JSON, storm JSON (if not pilot), manifest, known risks — assembled via `deployments/prod/scripts/build_release_evidence_pack.sh` |
| **C. Legacy single-host compose** | Rollback or rehearsal only | `deployments/prod/docker-compose.prod.yml` + `.env.production.example` validation in `verify-enterprise-release`; not the default 2-VPS story |

**Static CI gate:** [`.github/workflows/enterprise-release-verify.yml`](../../.github/workflows/enterprise-release-verify.yml) — `scripts/verify_enterprise_release.sh` (same as `make verify-enterprise-release`).

---

## Known risks (pilot and scale)

| Risk | Pilot impact | Scale impact | Mitigation |
| --- | --- | --- | --- |
| Planned-only HTTP surfaces documented but not mounted | Confusion if conflated with prod | Wrong client assumptions | [roadmap.md](../api/roadmap.md) only; OpenAPI must not advertise unmounted routes |
| Machine-scoped JWT scope | Wrong token → 403 / data isolation depends on middleware | Same at volume | Tenant-bound machine auth hardening; see [api-surface-security.md](./api-surface-security.md) |
| MQTT / JetStream lag | Pilot may hide backlog | Fleet outage risk | Metrics + storm evidence; [telemetry-jetstream-resilience.md](./telemetry-jetstream-resilience.md) |
| Commerce / webhook HMAC | Misconfig → payment errors | Financial + support load | Secret rotation, clock skew limits |
| Postgres pool exhaustion | Low pilot concurrency | Worker/API brownouts | Pool sizing; [production-2-vps.md](./production-2-vps.md) |

---

## Sign-off (release ticket)

- [ ] `make verify-enterprise-release` green on the **exact** `SOURCE_COMMIT_SHA`.
- [ ] Digest-pinned `app_image_ref` / `goose_image_ref` recorded in manifest.
- [ ] For **scale-100 / 500 / 1000**: storm JSON meets **Required telemetry storm JSON fields** and tier dimensions.
- [ ] `KNOWN_RISKS_PATH` reviewed by owning engineer + manager (names/dates on the ticket).
- [ ] Rollback path documented ([production-rollback.md](./production-rollback.md)).

---

## Enterprise release evidence pack

After gates pass, assemble the evidence pack (see earlier sections in this file for env vars):

```bash
bash deployments/prod/scripts/build_release_evidence_pack.sh
```

Prerequisites: `emit_verify_enterprise_result_json.sh`, monitoring + storm JSONs when not pilot, manifest, non-empty `KNOWN_RISKS_PATH`. Optional: `FLEET_SCALE_TARGET` / `EXPECTED_FLEET_SCALE_TARGET` must match manifest `fleet_scale_target`.

---

## CI

- **Static verify:** [`.github/workflows/enterprise-release-verify.yml`](../../.github/workflows/enterprise-release-verify.yml) runs `scripts/verify_enterprise_release.sh` on `main` PRs/pushes and `workflow_dispatch`.
- **Storm (staging):** [`.github/workflows/telemetry-storm-staging.yml`](../../.github/workflows/telemetry-storm-staging.yml) — manual; produces `telemetry-storm-result` for production scale gate.

---

## Related

- [environment-strategy.md](./environment-strategy.md) — local, staging, production separation and DSN policy.
- [telemetry-production-rollout.md](./telemetry-production-rollout.md) — storm suite, fleet gate, tuning.
- [kiosk-app-flow.md](../api/kiosk-app-flow.md) — end-to-end client journey.
- [api-surface-audit.md](../api/api-surface-audit.md) — endpoint matrix.

---

## Do not

- Commit real credentials into `*.example` files.
- Treat **static verify alone** as proof of fleet behavior — storm + monitoring evidence are required above **pilot**.
- Claim **scale-1000** without **1000×500** PASS.
