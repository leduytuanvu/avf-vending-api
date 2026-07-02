# 2026-07-02 deep clean inventory

**Baseline commit:** `a35df0fe`  
**Branch:** `cleanup/repo-deep-clean-docs-reports-scripts`

| Path | Type | Current purpose | References found | Risk | Proposed action | Reason | Verification |
| ---- | ---- | --------------- | ---------------- | ---- | --------------- | ------ | ------------ |
| `docs/reports/postman/` (3 MD) | Report | Postman audit snapshots | Self-only headers | LOW | ARCHIVE → `docs/archive/reports/postman/` | Historical; not CI | `rg docs/reports/postman` |
| `docs/reports/verification/*.md` (16) | Report | Protocol verification outputs | 1 active: POSTMAN_FULL_FLOW_TESTING_GUIDE | MEDIUM | ARCHIVE → `docs/archive/reports/verification/` | Superseded by archive FINALs | Update guide link |
| `docs/reports/verification/README.md` | Index | Active verification index | docs/README, testing/README | LOW | KEEP (rewrite as archive pointer) | Canonical index | link check |
| `docs/audits/final-enterprise-audit.md` | Audit | Canonical readiness | docs/README, audits/README | HIGH | KEEP | Active canonical | — |
| `docs/audits/final-production-gap-plan.md` | Audit | Gap planning | audits/README | MEDIUM | KEEP | Active planning | — |
| `docs/audits/REPO_STRUCTURE_P2_REFACTOR_PROPOSAL.md` | Audit | P2 roadmap | audits/README, prior cleanups | MEDIUM | KEEP | Deferred refactor scope | — |
| `docs/audits/enterprise-api-backend-audit-report.md` | Audit | Historical API audit | openapi-enterprise-upgrade-handoff L181 | MEDIUM | ARCHIVE → `docs/archive/audits/` | Marked historical; update runbook link | `rg enterprise-api-backend-audit` |
| `docs/audits/p0-hardening-report.md` | Audit | P0 evidence | audits/README (historical) | LOW | ARCHIVE → `docs/archive/audits/` | Historical evidence | — |
| `docs/release/evidence/20260531T201748Z-*` | Evidence | Deploy proof snapshot | BACKEND_MARKET_READY_REPORT | MEDIUM | ARCHIVE → `docs/archive/release/evidence/` | Timestamped one-off | Update market-ready links |
| `docs/release/BACKEND_MARKET_READY_REPORT.md` | Release | Market readiness verdict | PRODUCTION_E2E_CANARY_RUNBOOK | MEDIUM | KEEP | Active verdict reference | — |
| `docs/reports/cleanup/2026-07-02-repo-cleanup-{baseline,inventory,plan}.md` | Cleanup | Prior pass working docs | docs/README | LOW | ARCHIVE → `docs/archive/cleanup/2026-07-02/` | Working papers | Update indexes |
| `docs/reports/cleanup/2026-07-02-repo-cleanup-final-report.md` | Cleanup | Prior pass summary | docs/reports/README | LOW | KEEP | Active summary | Fix .gitignore link |
| `docs/reports/cleanup/2026-06-repo-cleanup-manifest.md` | Cleanup | 2026-06 manifest | Multiple READMEs | LOW | KEEP | Historical manifest still linked | — |
| `docs/reports/product-media-offline-cache/` (3) | Report | Final phase gates | reports/README, runbook | MEDIUM | KEEP | Active final evidence | — |
| `scripts/repair/*.ps1` (2) | Script | Field repair | test-metadata-contract.ps1 | MEDIUM | KEEP + README | Active field ops | — |
| `scripts/postman/generate_production_full_suite.py` | Generator | Full suite regen | postman README, docs | HIGH | UPDATE OUT_DIR + names | Path drift vs tracked suite | regen + diff |
| `postman/suites/production-full/avf-vending-production.full.*` | Postman | Tracked full suite | postman/README | HIGH | KEEP | Canonical committed suite | postman-check |
| `deployments/loadtest/env.example` | Config | Load test env template | docs/testing/load-test.md | MEDIUM | KEEP + README | Referenced by load-test guide | — |
| `tests/e2e/production/generated/rest-route-matrix.json` | Fixture | E2E route matrix | run_production_e2e.sh, testing docs | HIGH | KEEP | CI parity artifact | e2e scripts |
| `docs/archive/**` | Archive | Historical store | docs/README | LOW | KEEP | Improve README only | — |

Protected unchanged: `migrations/`, workflows, `deployments/prod|staging|docker/`, `postman/production/`, `postman/collections/`, `postman/environments/`, all Go source and generated code.
