# Final production readiness signoff — AVF vending backend

<!-- markdownlint-disable-file MD013 MD060 -->

**Document type:** Principal engineering review record (post implementation phases).

**Date (UTC):** 2026-04-29 (update when signing).

**Repository:** `avf-vending-api`

This signoff records **automated verification** run in the development environment and **honest limits**
of what CI cannot prove (field hardware, live PSP, org approvals).

---

## Verdict

| Item | Status |
|------|--------|
| **Passed (automated)** | See [Passed items](#passed-items-automated--evidence). |
| **Remaining risks** | [Remaining risks](#remaining-risks). |
| **Manual validation still required** | [Required manual validation](#required-manual-validation). |
| **Go-live blocker** | **No** for **code + contract gates** on a branch where **`go test ./...` is green**, **`sqlc` / OpenAPI outputs are committed** (no drift vs `db/queries` and route registry), **workflow YAML parses**, and org gates (**`make ci-gates`**, **`verify-enterprise-release`**) pass. **Yes** if tests fail, generated artifacts are uncommitted while queries/routes changed, YAML is invalid, or **fleet-scale / storm / monitoring evidence** is missing for the claimed target (see [`production-release-readiness.md`](production-release-readiness.md)). |

---

## Passed items (automated + evidence)

| Area | What was verified | Result (this session) |
|------|-------------------|----------------------|
| **Compile** | `go build -o NUL ./...` (Windows) / `go build ./...` | **PASS** |
| **Unit / integration tests** | `go test ./... -count=1` | **PASS** |
| **Go formatting** | `gofmt -l` then `gofmt -w` on drift files | **PASS** after write — see [Files changed summary](#files-changed-summary) |
| **sqlc** | `go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.29.0 generate` (pin matches `Makefile`) | **PASS**; **commit** `internal/gen/db/` |
| **OpenAPI / Swagger** | `python tools/build_openapi.py` then `python tools/openapi_verify_release.py` | **PASS**; **commit** `docs/swagger/swagger.json`, `docs/swagger/docs.go` |
| **Workflow YAML syntax** | `python` + `yaml.safe_load` on `.github/workflows/*.yml` | **PASS** — **17** files |
| **Workflow policy lint** | `actionlint` / `make verify-workflows` | **Not run locally** — rely on CI or install `actionlint` |
| **Merge whitespace** | `git diff --check` | **Inconclusive on dirty trees** — full tree may emit many historical hits; **hygiene files** touched here were rechecked clean with `git diff --check -- <paths>` |
| **Markdown style** | `npx markdownlint-cli` | **Optional** — this file disables strict line/table rules for readability |
| **Secret material scan** | `git grep` patterns (see [commands](#exact-commands-run)) | **No PEM blocks in non-doc paths**; matches are **variable names**, **test messages**, **OpenAPI examples**, or **scripts that detect keys** |

### Production-readiness checklist (mapped to sources)

| # | Topic | Satisfied by (architecture / runbooks / code pointers) |
|---|--------|----------------------------------------------------------|
| 1 | **Admin web:** REST / OpenAPI / User JWT / RBAC; tenant scope; audit on admin mutations | [`../api/admin-rest.md`](../api/admin-rest.md), [`api-surface-security.md`](api-surface-security.md), [`audit.md`](audit.md), [`../api/swagger-openapi-appendix.md`](../api/swagger-openapi-appendix.md) |
| 2 | **Machine app:** native gRPC; Machine JWT; idempotency; offline replay; **no production legacy REST dependency** | [`../architecture/production-final-contract.md`](../architecture/production-final-contract.md), [`../api/machine-grpc.md`](../api/machine-grpc.md), [`../../internal/grpcserver/machine_replay_ledger.go`](../../internal/grpcserver/machine_replay_ledger.go), [`../api/kiosk-app-flow.md`](../api/kiosk-app-flow.md) |
| 3 | **Payment:** backend-owned provider session; **do not trust** client `providerReference` / payment URL / QR from client; webhook HMAC; idempotency; amount validation; reconciliation | [`../api/payment.md`](../api/payment.md), [`../api/payment-webhook-security.md`](../api/payment-webhook-security.md), [`payment-reconciliation.md`](payment-reconciliation.md), [`payment-webhook-debug.md`](payment-webhook-debug.md) |
| 4 | **Pricing:** catalog / order / payment share pricing engine; promotion consistency | Commerce + catalog docs; [`runtime-sale-catalog` handoff](../api/runtime-sale-catalog-implementation-handoff.md); code paths exercised by tests |
| 5 | **MQTT:** TLS in production; topic ACL documented; command ledger; ACK correlation; timeout / retry | [`../api/mqtt-contract.md`](../api/mqtt-contract.md), [`mqtt-command-debug.md`](mqtt-command-debug.md), [`mqtt-command-stuck.md`](mqtt-command-stuck.md), [`production-readiness.md`](production-readiness.md) |
| 6 | **Media:** object storage; variants; HTTPS / signed URLs; hash / version for kiosk cache | [`../architecture/media-sync.md`](../architecture/media-sync.md), [`product-media-cache-invalidation.md`](product-media-cache-invalidation.md) |
| 7 | **Internal:** PostgreSQL SoT; Redis cache/rate/session; NATS·JetStream outbox; DLQ / replay; audit | [`../architecture/transport-boundary.md`](../architecture/transport-boundary.md), [`outbox.md`](outbox.md), [`outbox-dlq-debug.md`](outbox-dlq-debug.md), [`audit.md`](audit.md) |
| 8 | **Deployment:** production secrets contract; no staging/prod mix; no mock PSP in production; no plaintext public MQTT | [`../contracts/deployment-secrets-contract.yml`](../contracts/deployment-secrets-contract.yml), [`../operations/deployment-secrets.md`](../operations/deployment-secrets.md), [`production-release-readiness.md`](production-release-readiness.md) |
| 9 | **Observability:** metrics; alerts; runbooks; field smoke | [`observability-alerts.md`](observability-alerts.md), [`production-observability-alerts.md`](production-observability-alerts.md), [`../operations/deploy-monitoring-slo.md`](../operations/deploy-monitoring-slo.md), [`../runbooks/field-smoke-tests.md`](../runbooks/field-smoke-tests.md), [`../operations/production-smoke-tests.md`](../operations/production-smoke-tests.md) |

---

## Recommended pilot scope

| Scope | When to use | Preconditions |
|-------|-------------|----------------|
| **Lab only** | First integration of new build; broker / TLS / grpcurl / webhook harness | Staging or isolated lab; **`ENABLE_LEGACY_MACHINE_HTTP`** only if explicitly testing migration — **off** for production-parity |
| **10-machine field pilot** | **Default first field gate** after lab green | [`../operations/field-pilot-checklist.md`](../operations/field-pilot-checklist.md), [`../testing/field-test-cases.md`](../testing/field-test-cases.md) material evidence |
| **30-machine pilot** | Same as 10-machine but wider operator load / MQTT fan-out | Payment + MQTT + rollback owners named; no open P0 |
| **100+ rollout** | Fleet tier | [`production-release-readiness.md`](production-release-readiness.md) storm + monitoring artifacts; tranche plan [`../operations/field-rollout-checklist.md`](../operations/field-rollout-checklist.md) |

**Recommendation:** **Do not** skip **lab → 10-machine**; treat **100+** as a separate gate with scale evidence.

---

## Exact commands run

Host: **Windows PowerShell**, repository root `avf-vending-api`.

```powershell
gofmt -l internal cmd
gofmt -w internal/app/commerce/machine_payment_session.go `
  internal/bootstrap/reconciler.go `
  internal/bootstrap/temporal_worker.go `
  internal/grpcserver/machine_offline_p06_integration_test.go `
  internal/observability/observability_artifacts_test.go

go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.29.0 generate
go test ./... -count=1
go build -o NUL ./...

git diff --check
# Optional: scope whitespace check to intentional edits, e.g.:
# git diff --check -- internal/app/commerce/machine_payment_session.go

python tools/build_openapi.py
python tools/openapi_verify_release.py

python -c "import yaml,glob,pathlib; n=0
for p in sorted(glob.glob('.github/workflows/*.yml')):
  yaml.safe_load(pathlib.Path(p).read_text(encoding='utf-8'))
  n+=1
print('workflow yml ok', n)"

git grep -n -E "TODO|FIXME|panic\\(|fmt\.Println|log\.Fatal" -- internal cmd db .github deployments docs

git grep -n -E "BEGIN RSA|BEGIN OPENSSH|BEGIN PRIVATE KEY|PRIVATE KEY|password=|secret=" -- . ":(exclude).git" ":(exclude)vendor" ":(exclude)docs"

npx --yes markdownlint-cli docs/runbooks/final-production-readiness-signoff.md
```

**Linux / macOS equivalents:** use `go build -o /dev/null ./...`; use `make ci-gates` or
`make verify-enterprise-release` per [`Makefile`](../../Makefile).

---

## Files changed summary

| Path | Reason |
|------|--------|
| `internal/app/commerce/machine_payment_session.go` | `gofmt` |
| `internal/bootstrap/reconciler.go` | `gofmt` |
| `internal/bootstrap/temporal_worker.go` | `gofmt` |
| `internal/grpcserver/machine_offline_p06_integration_test.go` | `gofmt` |
| `internal/observability/observability_artifacts_test.go` | `gofmt` |
| `internal/gen/db/*.sql.go`, `models.go`, `querier.go`, … | **`sqlc generate`** — includes new query packages under `db/queries` |
| `docs/swagger/swagger.json`, `docs/swagger/docs.go` | **`tools/build_openapi.py`** |
| `docs/runbooks/final-production-readiness-signoff.md` | This signoff |

---

## Remaining risks

1. **`git diff --check`** on a **large dirty tree** may fail for unrelated whitespace; use a **clean release
   branch** before merge or scope `--check` to the commit diff.
2. **Committed drift:** If `internal/gen/db/` or `docs/swagger/` after generation **differs** from what is
   committed, **`make sqlc-check` / `swagger-check`** fails — **release blocker**.
3. **Observability gaps:** [`production-observability-alerts.md`](production-observability-alerts.md) lists
   **TODO** signals not yet exported — alerts may be incomplete.
4. **Scale claims:** Pilot metrics **≠** fleet storm behavior — follow
   [`production-release-readiness.md`](production-release-readiness.md).
5. **`git grep` noise:** `log.Fatal` / `panic` in **`cmd/*`** are startup guards (expected). `secret=` hits include
   **tests**, **scripts**, and **OpenAPI examples** — human review distinguishes from real leaks.
6. **actionlint:** Not run in this session; treat **CI workflow job** as authoritative when local tooling is
   missing.

---

## Required manual validation

1. **Org / CAB:** production environment reviewers, change record.
2. **Images / supply chain:** digest-pinned deploy artifacts per
   [`../operations/production-release-checklist.md`](../operations/production-release-checklist.md).
3. **Field matrix:** [`../testing/field-test-cases.md`](../testing/field-test-cases.md) + pilot / rollout
   checklists.
4. **PSP:** live or sandbox proof; webhook secret rotation exercised.
5. **MQTT:** broker TLS + ACL + real device ACK path.
6. **DR:** backup id / restore drill references current.
7. **Rollback:** owners and rehearsal per [`production-cutover-rollback.md`](production-cutover-rollback.md).

---

## Sign-off table (fill in at release)

| Role | Name | Date (UTC) | Notes |
|------|------|------------|-------|
| Principal / Staff engineer | | | This document |
| Release manager | | | Deploy workflow + manifest |
| Security / SRE (if applicable) | | | Secrets, alerts, DR |

---

## Related

- [`production-release-readiness.md`](production-release-readiness.md) — pilot vs scale tiers.
- [`../operations/production-release-checklist.md`](../operations/production-release-checklist.md) — deploy governance.
- [`../../Makefile`](../../Makefile) — `ci-gates`, `verify-enterprise-release`, `api-contract-check`.
