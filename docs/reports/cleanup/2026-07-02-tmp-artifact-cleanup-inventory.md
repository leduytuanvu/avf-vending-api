# 2026-07-02 tmp / local artifact cleanup inventory

**Branch:** `cleanup/local-artifacts-and-tmp-files`  
**Commit SHA:** `73a06c3beeeba4cf82209499b9a8315cd6ea75cf`  
**Date:** 2026-07-02

## Candidate classification

| Path | Type | Git status | Tracked? | Ignored? | References found | Risk | Proposed action | Reason |
| ---- | ---- | ---------- | -------- | -------- | ---------------- | ---- | --------------- | ------ |
| `.tmp/` | Deploy/scratch directory | untracked | No | Yes (`*.tmp`) | Docs archive audits only | LOW | DELETE_LOCAL_ONLY | Deploy candidate, trivy scratch, helper scripts — regenerable local pipeline output |
| `.tmp-sec-verdict/` | Security verdict scratch | untracked | No | Yes (`.tmp-*`) | None in scripts | LOW | DELETE_LOCAL_ONLY | Local security verdict JSON from deploy pipeline |
| `.test-runs/` | Go/PowerShell test output | untracked | No | Yes (`.test-runs/`) | `scripts/local/run-full-go-tests.ps1`, `show-latest-test-status.ps1` | LOW | DELETE_LOCAL_ONLY | Documented local-only; scripts recreate on next run |
| `.e2e-runs/` | E2E/Newman run artifacts | untracked | No | Yes (`.e2e-runs/`) | `tests/e2e/README.md`, `scripts/ci/verify_governance_protection_window.sh` (expects dir ignored, not contents) | LOW | DELETE_LOCAL_ONLY | ~15k files; runtime evidence only |
| `migration-evidence/migration-safety-report.json` | Migration safety report | untracked | No | Yes (`migration-evidence/*.json`) | `scripts/deploy/migration_preflight.sh`, `.github/workflows/deploy-develop.yml` (artifact upload path) | LOW | DELETE_LOCAL_ONLY | Regenerable via `verify_migrations.sh --report`; **keep `migration-evidence/` directory** |
| `bin/` | Build output | absent | No | Yes (`/bin/`) | Makefile `build` target | LOW | N/A (absent) | Not on disk |
| `dist/` | Build output | absent | No | Yes (`/dist/`) | repomix.config.json | LOW | N/A (absent) | Not on disk |
| `coverage/` | Coverage reports | absent | No | Yes (`coverage/`) | `scripts/test/run-mqtt-full-coverage.sh` | LOW | N/A (absent) | Not on disk |
| `ci-reports/` | Local CI reports | absent | No | Yes (`ci-reports/`) | None | LOW | N/A (absent) | Not on disk |
| `security-reports/` | Security scan reports | absent | No | Yes (`security-reports/`) | None | LOW | N/A (absent) | Not on disk |
| `repomix-output*.xml` | Repomix export | absent | No | Yes | `repomix.config.json`, `docs/operations/repomix-generation-guide.md` | LOW | N/A (absent) | Not on disk |
| `*.log` under `.e2e-runs/` | E2E logs | untracked | No | Yes (`*.log`) | E2E docs (runtime layout) | LOW | DELETE_LOCAL_ONLY | Removed with parent `.e2e-runs/` |
| `newman-report.json` / `newman-junit.xml` under `.e2e-runs/` | Newman outputs | untracked | No | Yes (`.e2e-runs/**/`) | E2E postman docs | LOW | DELETE_LOCAL_ONLY | Removed with parent `.e2e-runs/` |
| `migration-evidence/` (directory) | CI artifact path | N/A | No (dir untracked) | Partial (`*.json` only) | deploy-develop workflow | MEDIUM | KEEP | Workflow uploads `migration-evidence/migration-safety-report.json`; directory must exist |
| `cmd/`, `internal/`, `migrations/`, `postman/production/`, etc. | Source / CI assets | tracked | Yes | No | CI, deploy, tests | HIGH | KEEP | Protected by cleanup safety rules |

## Mandatory tracked-file checks

```text
git ls-files .tmp                    → (empty)
git ls-files .tmp-sec-verdict        → (empty)
git ls-files .test-runs              → (empty)
git ls-files .e2e-runs               → (empty)
git ls-files migration-evidence/migration-safety-report.json → (empty)
git ls-files bin dist coverage ci-reports security-reports repomix-output*.xml → (empty)
```

## Reference check summary (`rg`)

- `.test-runs/` — referenced in local PowerShell helpers; outputs are regenerable.
- `.e2e-runs/` — extensively documented as gitignored runtime dir; governance script verifies `.gitignore` entry only.
- `migration-evidence` — referenced for report output path; JSON is regenerable, directory kept.
- `.tmp/` — historical audit docs only; no CI script requires persisted content.

## Dry-run deletion list

Top-level targets (children included):

1. `.tmp/`
2. `.tmp-sec-verdict/`
3. `.test-runs/`
4. `.e2e-runs/`
5. `migration-evidence/migration-safety-report.json` (file only)
6. `tests/e2e/.e2e-runs/` (~7139 items, ~98 MB — gitignored nested e2e harness output)
7. `repomix-output-avf-vending-api.xml` (~15.5 MB — gitignored Repomix export)
